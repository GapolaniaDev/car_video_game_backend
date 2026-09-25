package protoframing

import (
	"errors"
	"testing"
)

func TestEncodeDecodeRoundTrip(t *testing.T) {
	payload := []byte("hello, protobuf!")
	framed, err := Encode(payload)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if len(framed) != HeaderSize+len(payload) {
		t.Fatalf("len(framed)=%d want %d", len(framed), HeaderSize+len(payload))
	}
	out, rest, err := Decode(framed)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if string(out) != string(payload) {
		t.Fatalf("payload mismatch: %q want %q", out, payload)
	}
	if len(rest) != 0 {
		t.Fatalf("expected no remainder, got %d bytes", len(rest))
	}
}

func TestDecodePreservesRemainder(t *testing.T) {
	framed1, _ := Encode([]byte("first"))
	framed2, _ := Encode([]byte("second"))
	combined := append(append([]byte{}, framed1...), framed2...)
	out1, rest, err := Decode(combined)
	if err != nil {
		t.Fatalf("decode#1: %v", err)
	}
	if string(out1) != "first" {
		t.Fatalf("first decode got %q", out1)
	}
	out2, rest2, err := Decode(rest)
	if err != nil {
		t.Fatalf("decode#2: %v", err)
	}
	if string(out2) != "second" || len(rest2) != 0 {
		t.Fatalf("second decode wrong: %q rest len %d", out2, len(rest2))
	}
}

func TestDecodeShort(t *testing.T) {
	_, _, err := Decode([]byte{0, 0, 0})
	if !errors.Is(err, ErrShortFrame) {
		t.Fatalf("want ErrShortFrame got %v", err)
	}
}

func TestDecodeTooLarge(t *testing.T) {
	// Length = MaxFrame+1 (we encode it as 0x00010040 + 1, i.e. 0x00010040=65537)
	buf := []byte{0x00, 0x01, 0x00, 0x40}
	_, _, err := Decode(buf)
	if !errors.Is(err, ErrFrameTooLarge) {
		t.Fatalf("want ErrFrameTooLarge got %v", err)
	}
}

func TestEncodeTooLarge(t *testing.T) {
	big := make([]byte, MaxFrame+1)
	_, err := Encode(big)
	if !errors.Is(err, ErrFrameTooLarge) {
		t.Fatalf("want ErrFrameTooLarge got %v", err)
	}
}
