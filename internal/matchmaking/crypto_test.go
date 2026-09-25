package matchmaking

import (
	"crypto/hmac"
	"crypto/sha256"
)

func mustHmac(secret, msg string) []byte {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(msg))
	return m.Sum(nil)
}
