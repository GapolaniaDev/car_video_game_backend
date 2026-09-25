package auth

import "testing"

func TestHashPasswordAndVerify_RoundTrip(t *testing.T) {
	hash, err := HashPassword("hunter2hunter2")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hash == "" || hash == "hunter2hunter2" {
		t.Fatal("hash should be non-empty and differ from plaintext")
	}
	if err := VerifyPassword(hash, "hunter2hunter2"); err != nil {
		t.Errorf("verify correct password: %v", err)
	}
}

func TestVerifyPassword_WrongReturnsInvalidCredentials(t *testing.T) {
	hash, err := HashPassword("correct-password")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	err = VerifyPassword(hash, "wrong-password")
	if err != ErrInvalidCredentials {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestHashPassword_Empty(t *testing.T) {
	if _, err := HashPassword(""); err == nil {
		t.Error("expected error for empty password")
	}
}