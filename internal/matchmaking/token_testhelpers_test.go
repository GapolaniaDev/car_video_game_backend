package matchmaking

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func mustEncode(t *testing.T, pl tokenPayload) string {
	t.Helper()
	raw, err := json.Marshal(pl)
	if err != nil {
		t.Fatal(err)
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

func signRaw(secret, payload string) string {
	mac := mustHmac(secret, payload)
	return base64.RawURLEncoding.EncodeToString(mac)
}