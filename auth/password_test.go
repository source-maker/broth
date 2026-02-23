package auth

import (
	"testing"
)

func TestBcryptHasher_HashAndVerify(t *testing.T) {
	t.Parallel()

	h := NewBcryptHasher(4) // Low cost for fast tests
	hash, err := h.Hash("password123")
	if err != nil {
		t.Fatalf("Hash error: %v", err)
	}
	if hash == "" {
		t.Fatal("hash should not be empty")
	}
	if hash == "password123" {
		t.Fatal("hash should not be the plaintext password")
	}

	if err := h.Verify("password123", hash); err != nil {
		t.Errorf("Verify correct password: %v", err)
	}
}

func TestBcryptHasher_VerifyWrongPassword(t *testing.T) {
	t.Parallel()

	h := NewBcryptHasher(4)
	hash, _ := h.Hash("password123")

	if err := h.Verify("wrongpassword", hash); err == nil {
		t.Error("expected error for wrong password")
	}
}

func TestBcryptHasher_HashTooShort(t *testing.T) {
	t.Parallel()

	h := NewBcryptHasher(4)
	_, err := h.Hash("short")
	if err == nil {
		t.Fatal("expected error for password < 8 chars")
	}
}

func TestBcryptHasher_HashExactly8Chars(t *testing.T) {
	t.Parallel()

	h := NewBcryptHasher(4)
	hash, err := h.Hash("12345678")
	if err != nil {
		t.Fatalf("Hash 8-char password error: %v", err)
	}
	if err := h.Verify("12345678", hash); err != nil {
		t.Errorf("Verify 8-char password: %v", err)
	}
}

func TestBcryptHasher_DifferentHashesForSamePassword(t *testing.T) {
	t.Parallel()

	h := NewBcryptHasher(4)
	hash1, _ := h.Hash("password123")
	hash2, _ := h.Hash("password123")

	if hash1 == hash2 {
		t.Error("same password should produce different hashes (bcrypt uses salt)")
	}

	// Both should verify
	if err := h.Verify("password123", hash1); err != nil {
		t.Errorf("Verify hash1: %v", err)
	}
	if err := h.Verify("password123", hash2); err != nil {
		t.Errorf("Verify hash2: %v", err)
	}
}

func TestBcryptHasher_DefaultCost(t *testing.T) {
	t.Parallel()

	h := NewBcryptHasher(0) // Should use default cost
	hash, err := h.Hash("password123")
	if err != nil {
		t.Fatalf("Hash error: %v", err)
	}
	if err := h.Verify("password123", hash); err != nil {
		t.Errorf("Verify error: %v", err)
	}
}

func TestDefaultPasswordHasher(t *testing.T) {
	t.Parallel()

	if DefaultPasswordHasher == nil {
		t.Fatal("DefaultPasswordHasher should not be nil")
	}

	hash, err := DefaultPasswordHasher.Hash("password123")
	if err != nil {
		t.Fatalf("Hash error: %v", err)
	}
	if err := DefaultPasswordHasher.Verify("password123", hash); err != nil {
		t.Errorf("Verify error: %v", err)
	}
}
