// Command udp-client is a developer utility for exercising the Game
// Server over UDP.
//
// Modes:
//
//	default (PING)         — sends "PING", expects "PONG" (legacy).
//	protobuf / join        — sends a length-prefixed JoinRaceRequest.
//
// Usage:
//
//	go run ./cmd/udp-client
//	go run ./cmd/udp-client --mode=protobuf --match-id=<uuid> --player-id=<uuid> --game-token=<token>
//	go run ./cmd/udp-client --mode=protobuf --timeout=5s
//
// Exit codes:
//
//	0  expected reply received
//	1  timeout or network error
package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"

	"github.com/gustavo/racing-game-backend/internal/protoframing"
	"github.com/gustavo/racing-game-backend/protocol/gamepb"
)

func main() {
	host := flag.String("host", "localhost", "Game Server host")
	port := flag.Int("port", 7000, "Game Server UDP port")
	mode := flag.String("mode", "ping", "ping | protobuf | join | player")
	timeout := flag.Duration("timeout", 3*time.Second, "Read deadline (non-player modes)")
	driveMs := flag.Int("drive-ms", 12000, "How long the player mode keeps sending PlayerInputs")
	matchID := flag.String("match-id", "", "matchId for protobuf/join/player mode")
	playerID := flag.String("player-id", "", "playerId for protobuf/join/player mode")
	gameToken := flag.String("game-token", "", "game_token for protobuf/join/player mode")
	throttle := flag.Float64("throttle", 1.0, "throttle value the player mode keeps sending")
	steering := flag.Float64("steering", 0.7, "steering value the player mode keeps sending (turn left)")
	flag.Parse()

	addr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(*host, fmt.Sprintf("%d", *port)))
	if err != nil {
		fmt.Fprintf(os.Stderr, "udp-client: resolve %s:%d: %v\n", *host, *port, err)
		os.Exit(1)
	}

	switch strings.ToLower(*mode) {
	case "ping":
		runOnce(addr, func(c *net.UDPConn) { sendPing(c) }, *timeout)
	case "protobuf", "join":
		runOnce(addr, func(c *net.UDPConn) {
			sendJoin(c, *matchID, *playerID, *gameToken)
		}, *timeout)
	case "player":
		runPlayer(addr, *matchID, *playerID, *gameToken, *throttle, *steering, time.Duration(*driveMs)*time.Millisecond)
	default:
		fmt.Fprintf(os.Stderr, "udp-client: unknown mode %q\n", *mode)
		os.Exit(2)
	}
}

func runOnce(addr *net.UDPAddr, fn func(*net.UDPConn), timeout time.Duration) {
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "udp-client: dial %s: %v\n", addr, err)
		os.Exit(1)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(timeout))
	fn(conn)
}

func runPlayer(addr *net.UDPAddr, mid, pid, tok string, throttle, steering float64, driveFor time.Duration) {
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "udp-client: dial %s: %v\n", addr, err)
		os.Exit(1)
	}
	defer conn.Close()

	// 1) Join first.
	joinReply := sendJoin(conn, mid, pid, tok)
	if joinReply == nil {
		fmt.Fprintln(os.Stderr, "udp-client: no Join reply")
		os.Exit(1)
	}
	if !joinReply.GetOk() {
		fmt.Fprintf(os.Stderr, "udp-client: Join rejected: %s\n", joinReply.GetError())
		os.Exit(1)
	}
	fmt.Printf("udp-client: joined raceId=%s initialTick=%d\n", joinReply.GetRaceId(), joinReply.GetInitialTick())

	// 2) Spawn a reader that consumes (and prints) every WorldSnapshot.
	snapshots := 0
	go func() {
		for {
			buf := make([]byte, 4096)
			_ = conn.SetReadDeadline(time.Now().Add(driveFor + 5*time.Second))
			n, err := conn.Read(buf)
			if err != nil {
				return
			}
			payload, _, err := protoframing.Decode(buf[:n])
			if err != nil {
				continue
			}
			snap := &gamepb.WorldSnapshot{}
			if err := proto.Unmarshal(payload, snap); err == nil {
				snapshots++
				if snapshots <= 3 {
					fmt.Printf("udp-client: snapshot tick=%d cars=%d\n", snap.GetTick(), len(snap.GetCars()))
				}
			}
		}
	}()

	// 3) Drive: send PlayerInputs until deadline.
	deadline := time.After(driveFor)
	seq := uint32(1)
	for {
		select {
		case <-deadline:
			fmt.Printf("udp-client: drove for %s, snapshots=%d\n", driveFor, snapshots)
			return
		default:
		}
		in := &gamepb.PlayerInput{Sequence: seq, Throttle: float32(throttle), Steering: float32(steering)}
		bs, err := proto.Marshal(in)
		if err != nil {
			continue
		}
		framed, err := protoframing.Encode(bs)
		if err != nil {
			continue
		}
		_, _ = conn.Write(framed)
		seq++
		time.Sleep(33 * time.Millisecond) // ~30Hz send rate
	}
}

func sendPing(conn *net.UDPConn) {
	if _, err := conn.Write([]byte("PING")); err != nil {
		fmt.Fprintf(os.Stderr, "udp-client: write PING: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Sent: PING → %s\n", conn.RemoteAddr())
	buf := make([]byte, 64)
	n, err := conn.Read(buf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "udp-client: read reply: %v\n", err)
		os.Exit(1)
	}
	reply := string(buf[:n])
	fmt.Printf("Received: %s\n", reply)
	if reply != "PONG" {
		fmt.Fprintf(os.Stderr, "udp-client: unexpected reply %q, want PONG\n", reply)
		os.Exit(1)
	}
}

// sendJoin sends a JoinRaceRequest and returns the decoded reply.
func sendJoin(conn *net.UDPConn, mid, pid, tok string) *gamepb.JoinRaceResponse {
	req := &gamepb.JoinRaceRequest{
		MatchId:   mid,
		PlayerId:  pid,
		GameToken: tok,
	}
	if req.MatchId == "" {
		req.MatchId = uuid.New().String()
	}
	if req.PlayerId == "" {
		req.PlayerId = uuid.New().String()
	}
	payload, err := proto.Marshal(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "udp-client: marshal: %v\n", err)
		os.Exit(1)
	}
	framed, err := protoframing.Encode(payload)
	if err != nil {
		fmt.Fprintf(os.Stderr, "udp-client: frame: %v\n", err)
		os.Exit(1)
	}
	if _, err := conn.Write(framed); err != nil {
		fmt.Fprintf(os.Stderr, "udp-client: write: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Sent: JoinRaceRequest → %s (match=%s, player=%s)\n", conn.RemoteAddr(), req.MatchId, req.PlayerId)

	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "udp-client: read reply: %v\n", err)
		return nil
	}
	out, _, err := protoframing.Decode(buf[:n])
	if err != nil {
		return nil
	}
	resp := &gamepb.JoinRaceResponse{}
	if err := proto.Unmarshal(out, resp); err != nil {
		fmt.Fprintf(os.Stderr, "udp-client: unmarshal reply: %v\n", err)
		return nil
	}
	fmt.Printf("Received: ok=%v raceId=%q initialTick=%d error=%q\n",
		resp.GetOk(), resp.GetRaceId(), resp.GetInitialTick(), resp.GetError())
	return resp
}
