// Package protoframing implements a tiny length-prefixed framing for
// Protobuf-over-UDP. Each frame is a 4-byte big-endian unsigned length
// followed by that many bytes of payload.
//
// This matches the wire format Unity will emit/read on the client side.
package protoframing

import (
	"encoding/binary"
	"errors"
	"math"
)

// HeaderSize is the number of leading bytes holding the length prefix.
const HeaderSize = 4

// MaxFrame guards against absurdly large payloads (DoS protection).
// 64 KiB is well above any single Protobuf message we expect (the
// largest is WorldSnapshot with ~32 cars).
const MaxFrame = 64 * 1024

// ErrShortFrame is returned when a buffer contains fewer than 4 bytes
// for the length prefix or when the buffer is shorter than the
// declared length.
var ErrShortFrame = errors.New("protoframing: short frame")

// ErrFrameTooLarge is returned when the length prefix exceeds MaxFrame.
var ErrFrameTooLarge = errors.New("protoframing: frame too large")

// Decode splits the given buffer into the payload bytes and the
// remainder of the buffer (which may be empty for a clean read). It
// does NOT copy the payload — callers MUST consume or copy it before
// the next read reuses the underlying buffer.
func Decode(buf []byte) (payload, rest []byte, err error) {
	if len(buf) < HeaderSize {
		return nil, nil, ErrShortFrame
	}
	l := binary.BigEndian.Uint32(buf[:HeaderSize])
	if l == 0 {
		return nil, buf[HeaderSize:], nil
	}
	if l > MaxFrame {
		return nil, nil, ErrFrameTooLarge
	}
	end := HeaderSize + int(l)
	if len(buf) < end {
		return nil, nil, ErrShortFrame
	}
	return buf[HeaderSize:end], buf[end:], nil
}

// Encode prepends a 4-byte big-endian length prefix to payload. It
// returns a freshly allocated slice; payload is not modified.
func Encode(payload []byte) ([]byte, error) {
	if len(payload) > MaxFrame {
		return nil, ErrFrameTooLarge
	}
	out := make([]byte, HeaderSize+len(payload))
	binary.BigEndian.PutUint32(out[:HeaderSize], uint32(len(payload)))
	copy(out[HeaderSize:], payload)
	return out, nil
}

// Uint32Max is the upper bound used to check the length prefix fits in
// the uint32 header before encoding. We define it as a constant so
// callers don't need math.
const Uint32Max = math.MaxUint32
