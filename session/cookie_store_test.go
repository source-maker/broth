package session

import (
	"crypto/rand"
	"testing"
	"time"
)

func testKey(t *testing.T) []byte {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	return key
}

func TestNewCookieStore(t *testing.T) {
	t.Parallel()

	store, err := NewCookieStore(testKey(t))
	if err != nil {
		t.Fatalf("NewCookieStore error: %v", err)
	}
	if store == nil {
		t.Fatal("store is nil")
	}
}

func TestNewCookieStore_InvalidKeyLength(t *testing.T) {
	t.Parallel()

	_, err := NewCookieStore([]byte("short"))
	if err == nil {
		t.Fatal("expected error for short key")
	}
}

func TestCookieStore_EncodeDecode(t *testing.T) {
	t.Parallel()

	store, err := NewCookieStore(testKey(t))
	if err != nil {
		t.Fatal(err)
	}

	original := &Session{
		ID:        "sess-123",
		Data:      map[string]any{"username": "alice", "role": "admin"},
		UserID:    42,
		CreatedAt: time.Now().Truncate(time.Second),
		ExpiresAt: time.Now().Add(time.Hour).Truncate(time.Second),
	}

	encoded, err := store.Encode(original)
	if err != nil {
		t.Fatalf("Encode error: %v", err)
	}
	if encoded == "" {
		t.Fatal("encoded value is empty")
	}

	decoded, err := store.Decode(encoded)
	if err != nil {
		t.Fatalf("Decode error: %v", err)
	}

	if decoded.ID != original.ID {
		t.Errorf("ID = %q, want %q", decoded.ID, original.ID)
	}
	if decoded.UserID != original.UserID {
		t.Errorf("UserID = %d, want %d", decoded.UserID, original.UserID)
	}
	if decoded.Data["username"] != "alice" {
		t.Errorf("Data[username] = %v, want %q", decoded.Data["username"], "alice")
	}
	if decoded.Data["role"] != "admin" {
		t.Errorf("Data[role] = %v, want %q", decoded.Data["role"], "admin")
	}
}

func TestCookieStore_DecodeInvalidValue(t *testing.T) {
	t.Parallel()

	store, err := NewCookieStore(testKey(t))
	if err != nil {
		t.Fatal(err)
	}

	_, err = store.Decode("invalid-base64-!!!")
	if err == nil {
		t.Error("expected error for invalid base64")
	}
}

func TestCookieStore_DecodeTamperedValue(t *testing.T) {
	t.Parallel()

	store, err := NewCookieStore(testKey(t))
	if err != nil {
		t.Fatal(err)
	}

	s := &Session{
		ID:        "sess-456",
		Data:      map[string]any{"key": "value"},
		ExpiresAt: time.Now().Add(time.Hour),
	}
	encoded, err := store.Encode(s)
	if err != nil {
		t.Fatal(err)
	}

	// Tamper with the encoded value
	tampered := encoded[:len(encoded)-2] + "XX"
	_, err = store.Decode(tampered)
	if err == nil {
		t.Error("expected error for tampered value")
	}
}

func TestCookieStore_DecodeWrongKey(t *testing.T) {
	t.Parallel()

	store1, err := NewCookieStore(testKey(t))
	if err != nil {
		t.Fatal(err)
	}
	store2, err := NewCookieStore(testKey(t))
	if err != nil {
		t.Fatal(err)
	}

	s := &Session{
		ID:        "sess-789",
		Data:      map[string]any{"secret": "data"},
		ExpiresAt: time.Now().Add(time.Hour),
	}
	encoded, err := store1.Encode(s)
	if err != nil {
		t.Fatal(err)
	}

	_, err = store2.Decode(encoded)
	if err == nil {
		t.Error("expected error when decoding with different key")
	}
}

func TestCookieStore_DecodeExpired(t *testing.T) {
	t.Parallel()

	store, err := NewCookieStore(testKey(t))
	if err != nil {
		t.Fatal(err)
	}

	s := &Session{
		ID:        "sess-expired",
		Data:      map[string]any{},
		ExpiresAt: time.Now().Add(-time.Hour), // Already expired
	}
	encoded, err := store.Encode(s)
	if err != nil {
		t.Fatal(err)
	}

	_, err = store.Decode(encoded)
	if err == nil {
		t.Error("expected error for expired session")
	}
}

func TestCookieStore_EncodeDecodeEmptyData(t *testing.T) {
	t.Parallel()

	store, err := NewCookieStore(testKey(t))
	if err != nil {
		t.Fatal(err)
	}

	s := &Session{
		ID:        "sess-empty",
		ExpiresAt: time.Now().Add(time.Hour),
	}
	encoded, err := store.Encode(s)
	if err != nil {
		t.Fatalf("Encode error: %v", err)
	}

	decoded, err := store.Decode(encoded)
	if err != nil {
		t.Fatalf("Decode error: %v", err)
	}
	if decoded.ID != "sess-empty" {
		t.Errorf("ID = %q, want %q", decoded.ID, "sess-empty")
	}
}

func TestCookieStore_DecodeShortCiphertext(t *testing.T) {
	t.Parallel()

	store, err := NewCookieStore(testKey(t))
	if err != nil {
		t.Fatal(err)
	}

	// Very short base64 value (too short to contain nonce)
	_, err = store.Decode("YQ==")
	if err == nil {
		t.Error("expected error for short ciphertext")
	}
}

func TestCookieStore_NoExpiryDoesNotExpire(t *testing.T) {
	t.Parallel()

	store, err := NewCookieStore(testKey(t))
	if err != nil {
		t.Fatal(err)
	}

	s := &Session{
		ID:   "sess-no-expiry",
		Data: map[string]any{"key": "val"},
		// ExpiresAt is zero value — should not expire
	}
	encoded, err := store.Encode(s)
	if err != nil {
		t.Fatal(err)
	}

	decoded, err := store.Decode(encoded)
	if err != nil {
		t.Fatalf("should not expire with zero ExpiresAt: %v", err)
	}
	if decoded.ID != "sess-no-expiry" {
		t.Errorf("ID = %q, want %q", decoded.ID, "sess-no-expiry")
	}
}
