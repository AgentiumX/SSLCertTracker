package auth

import "testing"

func TestHashAndVerify(t *testing.T) {
	hash, err := HashPassword("hunter2")
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	if hash == "" {
		t.Errorf("expected non-empty hash")
	}
	if hash == "hunter2" {
		t.Errorf("hash must not equal plaintext")
	}
	if !VerifyPassword(hash, "hunter2") {
		t.Errorf("VerifyPassword should accept correct password")
	}
}

func TestVerifyWrongPassword(t *testing.T) {
	hash, err := HashPassword("hunter2")
	if err != nil {
		t.Fatal(err)
	}
	if VerifyPassword(hash, "wrong") {
		t.Errorf("VerifyPassword should reject wrong password")
	}
}

func TestVerifyPassword_InvalidHash(t *testing.T) {
	if VerifyPassword("not-a-valid-bcrypt-hash", "anything") {
		t.Errorf("VerifyPassword should reject invalid hash format")
	}
}

func TestDummyHash_Verifies(t *testing.T) {
	// DummyHash is used in login flow when user not found, to defend against
	// timing attacks. It must be a valid bcrypt hash so VerifyPassword runs
	// the full computation, but should never match any real password.
	if VerifyPassword(DummyHash, "") {
		t.Errorf("DummyHash should not match empty password")
	}
	if VerifyPassword(DummyHash, "any-password") {
		t.Errorf("DummyHash should not match any password")
	}
}
