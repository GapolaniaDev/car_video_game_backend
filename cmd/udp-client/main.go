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
	mode := flag.String("mode", "ping", "ping | protobuf")
	timeout := flag.Duration("timeout", 3*time.Second, "Read deadline")
	matchID := flag.String("match-id", "", "matchId for protobuf mode")
	playerID := flag.String("player-id", "", "playerId for protobuf mode")
	gameToken := flag.String("game-token", "", "game_token for protobuf mode")
	flag.Parse()

	addr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(*host, fmt.Sprintf("%d", *port)))
	if err != nil {
		fmt.Fprintf(os.Stderr, "udp-client: resolve %s:%d: %v\n", *host, *port, err)
		os.Exit(1)
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "udp-client: dial %s: %v\n", addr, err)
		os.Exit(1)
	}
	defer conn.Close()

	_ = conn.SetReadDeadline(time.Now().Add(*timeout))

	switch strings.ToLower(*mode) {
	case "ping":
		sendPing(conn, addr)
	case "protobuf", "join":
		sendProtobuf(conn, addr, *matchID, *playerID, *gameToken)
	default:
		fmt.Fprintf(os.Stderr, "udp-client: unknown mode %q\n", *mode)
		os.Exit(2)
	}
}

func sendPing(conn *net.UDPConn, addr *net.UDPAddr) {
	if _, err := conn.Write([]byte("PING")); err != nil {
		fmt.Fprintf(os.Stderr, "udp-client: write PING: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Sent: PING → %s\n", addr)
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

func sendProtobuf(conn *net.UDPConn, addr *net.UDPAddr, mid, pid, tok string) {
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
	fmt.Printf("Sent: JoinRaceRequest → %s (match=%s, player=%s)\n", addr, req.MatchId, req.PlayerId)

	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "udp-client: read reply: %v\n", err)
		os.Exit(1)
	}
	out, _, err := protoframing.Decode(buf[:n])
	if err != nil {
		fmt.Fprintf(os.Stderr, "udp-client: decode reply frame: %v\n", err)
		os.Exit(1)
	}
	resp := &gamepb.JoinRaceResponse{}
	if err := proto.Unmarshal(out, resp); err != nil {
		fmt.Fprintf(os.Stderr, "udp-client: unmarshal reply: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Received: ok=%v raceId=%q initialTick=%d error=%q\n", resp.GetOk(), resp.GetRaceId(), resp.GetInitialTick(), resp.GetError())
	if !resp.GetOk() {
		os.Exit(1)
	}
}
