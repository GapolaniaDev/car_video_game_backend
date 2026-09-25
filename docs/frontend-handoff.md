# Frontend Handoff — Car Video Game Backend

**Audience:** Unity / mobile / web dev integrating with this backend.
**Last updated:** 2026-09-26

This doc tells you everything you need to connect a client to the
multiplayer racing backend, end-to-end. If something here contradicts
the OpenAPI spec, the OpenAPI wins — but please open an issue.

---

## 1. Architecture at a glance

The backend is split into **two cooperating services**, each with a
clear role:

```
┌────────────────────┐                  ┌────────────────────┐
│  racing-platform   │   (REST / JWT)   │   racing-engine    │
│                    │ ───────────────▶ │                    │
│  • auth            │                  │  • real-time sim   │
│  • profile         │  POST            │  • tick scheduler  │
│  • catalog         │  /matchmaking    │  • physics + races │
│  • persistence     │  /join           │  • UDP authoritative│
│  • matchmaking     │                  │  • protobuf wire    │
│   (queue + sign)   │  ◀─────────────  │                    │
│                    │   game_token     │                    │
└────────────────────┘                  └────────────────────┘
        ▲                                       ▲
        │ HTTPS                                 │ UDP (7000)
        │ Bearer JWT                            │ length-prefixed
        │                                       │ protobuf
        │                                       │
   ┌────┴──────────────────────────────────────┴────┐
   │                  Unity client                  │
   └────────────────────────────────────────────────┘
```

### What each piece owns

| Service | Owns | Talks to clients over | Talks to other services over |
|---|---|---|---|
| **`racing-platform`** | Identity, sessions, profiles, car/track catalog, matchmaking queue, token signing | HTTPS (REST/JSON) | PostgreSQL (its own DB) |
| **`racing-engine`** | Real-time race simulation, physics, input validation, world state replication, race lifecycle | UDP + Protocol Buffers | PostgreSQL (read-only for race metadata) |

You always start at `racing-platform` (auth + matchmaking), then open
a UDP socket to `racing-engine`.

### Why two services?

- **Different latency profiles.** Account/profile stuff tolerates a
  200 ms RTT. Racing needs <50 ms and a packet every 16 ms. They can't
  share an event loop or a thread pool.
- **Different network reach.** `racing-platform` is reachable from
  anywhere via the Cloudflare Tunnel (`https://api.gapolaniadev.com`).
  `racing-engine` runs on UDP/7000 on the LAN — the tunnel can't carry
  UDP, and you don't want it on the public internet without DDoS
  protection anyway.
- **Different deploy lifecycles.** `racing-platform` can be scaled
  horizontally behind a load balancer. `racing-engine` is stateful —
  one process per active race. They deploy independently.

---

## 2. Reachability — the most common gotcha

| Service | Where you reach it | From where |
|---|---|---|
| `racing-platform` | `https://api.gapolaniadev.com/api/v1` | **Anywhere** (Cloudflare edge → tunnel → container) |
| `racing-engine` | `<host>:7000/udp` | **Same LAN only**, **or** a VPN / Tailscale / direct public IP + firewall rule |

The Game Server's UDP port `7000` is **not** in the Cloudflare Tunnel.
Cloudflare Tunnel is TCP/HTTP only — UDP is not supported (Cloudflare
Spectrum is the paid option). For dev purposes we keep it on direct
host publish:

```
ports:
  - "7000:7000/udp"   # docker-compose.yml → racing-engine service
```

So if your dev machine isn't on the same LAN as the host running
docker-compose, you have three options, in increasing complexity:

1. **Tailscale / WireGuard** between your dev box and the host. Easiest.
2. **VPN** into the host's network.
3. **Port-forward** `7000/udp` on the host's router + firewall rule
   allowing your IP. Ask the host's owner first.

Whatever you do, do **not** expose `7000/udp` to the open internet
without auth, rate-limiting, and DDoS protection. The server trusts
whatever IP you say you are, and the auth token is only checked on
the very first packet.

---

## 3. `racing-platform` — REST API

### Base URL

```
https://api.gapolaniadev.com/api/v1
```

TLS is terminated at Cloudflare; everything past the edge is plain
HTTP inside the docker network.

### OpenAPI spec

`docs/openapi.yaml` in this repo. Import into Postman / Insomnia /
whatever to get full request/response schemas, examples, and error codes.

### Auth flow

1. **Register** — `POST /auth/register`

   ```json
   {
     "username": "alice",
     "email": "alice@example.com",
     "password": "hunter2hunter2",
     "displayName": "Alice"
   }
   ```

   Returns `{ accessToken, playerId }`.

2. **Login** — `POST /auth/login` (same body minus `displayName`).
3. **Use token** — every protected endpoint needs:

   ```
   Authorization: Bearer eyJhbGciOiJIUzI1NiIs...
   ```

   JWT lifetime: **1 hour**. After expiry, login again.

### Endpoints you'll likely hit

| Method | Path | Purpose | Auth |
|---|---|---|---|
| GET | `/health` | Server status | none |
| POST | `/auth/register` | Create account + get token | none |
| POST | `/auth/login` | Get fresh token | none |
| GET | `/players/me` | Your profile | yes |
| GET | `/cars` | Car catalog (3 cars at MVP) | yes |
| GET | `/tracks` | Track catalog | yes |
| GET | `/garage` | Your owned cars | yes |
| POST | `/matchmaking/join` | **Get a `game_token` + engine address** | yes |

---

## 4. Matchmaking — the bridge between platform and engine

`POST /matchmaking/join` (auth required) returns:

```json
{
  "matchId":        "9f2b1c7e-…",
  "gameServerHost": "192.168.1.42",
  "gameServerPort": 7000,
  "gameToken":      "AbCdEf…long-base64url-string"
}
```

- `gameServerHost` / `gameServerPort` — where to open the UDP socket.
- `gameToken` — short-lived (5 min) HMAC-SHA256 token. Send it on the
  very first UDP message (`JoinRaceRequest.game_token`). The engine
  **consumes** it: one token = one race join, then it's dead.
- If you don't join within 5 min, ask for a new one (POST again).

The matchmaking itself is "immediate" at MVP: as soon as you ask, the
service either slots you into an existing race with <4 players or
spins up a new one. No skill rating, no waiting queue.

⚠️ `gameServerHost` comes from the env var `GAME_SERVER_PUBLIC_HOST`
in `racing-platform`. Make sure the host owner has set it to a
**LAN-reachable IP** (e.g. `192.168.1.42`), not `localhost`, or your
UDP socket will only work from the host machine itself.

---

## 5. `racing-engine` — UDP + Protocol Buffers

### Wire format

Protobuf over UDP, framed as **4-byte big-endian length + payload**.

```
┌──────────────────────┬──────────────────────────────────┐
│ length (uint32 BE)   │ protobuf payload (game.proto)    │
└──────────────────────┴──────────────────────────────────┘
```

Maximum packet size: **1200 bytes** (safe under typical MTU minus
headers; protects against fragmentation).

### Protobuf schema

`protocol/game.proto` in this repo. Generate client bindings:

**Unity (C#):**

```bash
protoc \
  --csharp_out=Assets/Scripts/Generated \
  --proto_path=protocol \
  protocol/game.proto
```

**Web (TypeScript / protobufjs):**

```bash
pbjs -t static-module -w es6 -o game.js   protocol/game.proto
pbts -o game.d.ts game.js
```

**Node / TS / Go:** any `protoc` target works.

### Message sequence

```
   client                              racing-engine
     │                                         │
     │── UDP open to host:7000 ───────────────▶│
     │                                         │
     │── [len][JoinRaceRequest] ──────────────▶│
     │       {match_id, player_id, game_token} │
     │                                         │
     │◀── [len][JoinRaceResponse] ─────────────│
     │       {ok=true, race_id, initial_tick}  │
     │                                         │
     │── [len][PlayerInput] ──────────────────▶│  (every input tick)
     │◀── [len][WorldSnapshot] ───────────────│  (every server tick)
     │                                         │
     │         … race in progress …            │
     │                                         │
     │◀── [len][RaceFinished] ────────────────│
     │       {race_id, results:[…]}            │
```

### First message — `JoinRaceRequest`

```protobuf
message JoinRaceRequest {
  string match_id   = 1;
  string player_id  = 2;
  string game_token = 3;
}
```

Server replies with one of:

```protobuf
message JoinRaceResponse {
  bool   ok           = 1;  // true → proceed
  string error        = 2;  // populated when ok=false
  string race_id      = 3;  // your race id, keep it
  uint64 initial_tick = 4;  // server's current tick when you joined
}
```

### Live loop

- Send `PlayerInput { sequence, throttle, brake, steering }` at your
  input rate (60 Hz typical).
- Receive `WorldSnapshot { tick, cars[] }` at the server's tick rate.
- Each `CarState` carries `player_id, position, rotation, velocity`.
- Sequence numbers are **monotonically increasing per session**; the
  engine discards out-of-order packets.

### Race events

Subscribe to these (they arrive interleaved with `WorldSnapshot`):

- `RaceStarting { start_tick, countdown_ms }`
- `RaceStarted { start_tick }`
- `CheckpointPassed { player_id, checkpoint_index, tick }`
- `LapCompleted { player_id, lap, lap_time_ms }`
- `RaceFinished { race_id, results[] }`
- `PlayerDisconnected { player_id, reason }`

### Errors

If the engine can't decode a packet, the `game_token` is
invalid/expired, or the player isn't registered yet, you'll get a
`JoinRaceResponse { ok=false, error="…" }`. After that, the engine
won't process further packets until you send a fresh
`JoinRaceRequest` with a new `game_token`.

---

## 6. Minimal Unity (C#) skeleton

This is the smallest end-to-end example that talks to both services.
Drop it into a MonoBehaviour and you should see your player join a
race and receive one world snapshot.

```csharp
using System;
using System.Net;
using System.Net.Sockets;
using System.Text;
using System.Threading;
using System.Threading.Tasks;
using Google.Protobuf;
using Racing.Game.V1;
using UnityEngine;

public class RaceClient : MonoBehaviour
{
    [Header("racing-platform")]
    public string platformBaseUrl = "https://api.gapolaniadev.com/api/v1";
    public string username        = "alice";
    public string password        = "hunter2hunter2";

    string _jwt;
    string _playerId;

    UdpClient _udp;
    IPEndPoint _engineEp;
    CancellationTokenSource _cts;

    async void Start()
    {
        // 1) Auth via racing-platform
        _jwt = await HttpPostJson($"{platformBaseUrl}/auth/login",
            $"{{\"username\":\"{username}\",\"password\":\"{password}\",\"email\":\"x@x.local\"}}",
            extractField: "accessToken");

        _playerId = await HttpGet($"{platformBaseUrl}/players/me",
            $"Bearer {_jwt}", extractField: "playerId");

        // 2) Matchmaking via racing-platform → tells us where racing-engine lives
        var mm = await HttpPostJson($"{platformBaseUrl}/matchmaking/join", "{}", $"Bearer {_jwt}");
        var host   = ExtractJsonString(mm, "gameServerHost");
        var port   = int.Parse(ExtractJsonString(mm, "gameServerPort"));
        var match  = ExtractJsonString(mm, "matchId");
        var token  = ExtractJsonString(mm, "gameToken");

        // 3) Open UDP to racing-engine
        _engineEp = new IPEndPoint(IPAddress.Parse(host), port);
        _udp      = new UdpClient();
        _cts      = new CancellationTokenSource();

        // 4) Listen in background
        _ = Task.Run(() => ReceiveLoopAsync(_cts.Token));

        // 5) Send JoinRaceRequest
        var join = new JoinRaceRequest {
            MatchId   = match,
            PlayerId  = _playerId,
            GameToken = token,
        };
        SendProtobuf(join);

        // 6) (Later) send PlayerInput at 60Hz, etc.
    }

    async Task ReceiveLoopAsync(CancellationToken ct)
    {
        while (!ct.IsCancellationRequested) {
            try {
                var result = await _udp.ReceiveAsync();
                // 4-byte BE length prefix | protobuf payload
                var payload = result.Buffer;
                if (payload.Length < 4) continue;
                int len = (payload[0] << 24) | (payload[1] << 16)
                        | (payload[2] <<  8) |  payload[3];
                if (payload.Length < 4 + len) continue;

                var msg = RacingMessage.Parser.ParseFrom(
                    new ArraySegment<byte>(payload, 4, len).ToArray());

                if (msg.JoinRaceResponse != null) {
                    Debug.Log($"joined: ok={msg.JoinRaceResponse.Ok} race={msg.JoinRaceResponse.RaceId}");
                }
                if (msg.WorldSnapshot != null) {
                    Debug.Log($"tick={msg.WorldSnapshot.Tick} cars={msg.WorldSnapshot.Cars.Count}");
                }
                // ... other event handlers ...
            } catch (Exception e) {
                Debug.LogError($"udp recv: {e.Message}");
            }
        }
    }

    void SendProtobuf<T>(T message) where T : IMessage<T> {
        var bytes = message.ToByteArray();
        var framed = new byte[4 + bytes.Length];
        framed[0] = (byte)(bytes.Length >> 24);
        framed[1] = (byte)(bytes.Length >> 16);
        framed[2] = (byte)(bytes.Length >>  8);
        framed[3] = (byte) bytes.Length;
        Array.Copy(bytes, 0, framed, 4, bytes.Length);
        _udp.Send(framed, framed.Length, _engineEp);
    }

    void OnDestroy() {
        _cts?.Cancel();
        _udp?.Close();
    }

    // ... HttpPostJson, HttpGet, ExtractJsonString helpers ...
}
```

(`HttpPostJson` / `HttpGet` are intentionally left out — use
`UnityWebRequest` or any HTTP lib you prefer. The pattern is the same.)

---

## 7. Quick-start checklist

- [ ] Get the repo (or just `docs/openapi.yaml` + `protocol/game.proto`)
- [ ] Import OpenAPI into your HTTP client → exercise `/auth/register`,
      `/players/me` (sanity check)
- [ ] Generate C# / TS / etc bindings from `protocol/game.proto`
- [ ] Implement 4-byte length-prefix framing on top of `UdpClient` (Unity)
      or `dgram` (Node)
- [ ] Wire up the matchmaking flow: REST → POST `/matchmaking/join` →
      open UDP → send `JoinRaceRequest`
- [ ] Throttle input to 60 Hz max; **don't** flood the server
- [ ] Handle disconnects: if you stop hearing `WorldSnapshot` for >2 s,
      assume the server dropped you, re-do matchmaking

---

## 8. What NOT to do

- ❌ Send `PlayerInput` before `JoinRaceResponse.ok=true`. The engine
  drops anything that isn't `JoinRaceRequest` first.
- ❌ Reuse a `game_token`. They're single-use.
- ❌ Hard-code IPs. Always read `gameServerHost` / `gameServerPort` from
  the matchmaking response.
- ❌ Send packets > 1200 bytes. Split or compress.
- ❌ Spam `matchmaking/join`. Rate-limit client-side (1 req/sec max).
- ❌ Expose `racing-engine` UDP port to the open internet without auth
  + rate-limiting + DDoS protection.

---

## 9. Versions in use (2026-09-26)

| Component | Image / tag | Notes |
|---|---|---|
| `racing-platform` | `racing-game-backend/backend-api:dev` | REST, JWT |
| `racing-engine`   | `racing-game-backend/game-server:dev` | UDP, protobuf |
| `cloudflared`     | `cloudflare/cloudflared:2025.11.1` | Tunnel for the platform |
| `protocol/game.proto` | `racing.game.v1` | Frozen field tags |

If the repo has moved on, check `docker-compose.yml` for the pinned
image tags.
