package session

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"
)

// CookieStore stores session data encrypted in a cookie using AES-GCM.
// No server-side state is required (lightweight, but limited to ~4KB).
type CookieStore struct {
	gcm cipher.AEAD
}

// NewCookieStore creates a CookieStore with the given 32-byte secret key.
func NewCookieStore(secretKey []byte) (*CookieStore, error) {
	if len(secretKey) != 32 {
		return nil, errors.New("session: secret key must be 32 bytes")
	}
	block, err := aes.NewCipher(secretKey)
	if err != nil {
		return nil, fmt.Errorf("session: new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("session: new gcm: %w", err)
	}
	return &CookieStore{gcm: gcm}, nil
}

// sessionPayload is the JSON-serializable session data.
type sessionPayload struct {
	ID        string         `json:"id"`
	Data      map[string]any `json:"data,omitempty"`
	UserID    int64          `json:"user_id,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	ExpiresAt time.Time      `json:"expires_at,omitempty"`
}

// Encode serializes and encrypts a session into a cookie-safe string.
func (cs *CookieStore) Encode(s *Session) (string, error) {
	p := sessionPayload{
		ID:        s.ID,
		Data:      s.Data,
		UserID:    s.UserID,
		CreatedAt: s.CreatedAt,
		ExpiresAt: s.ExpiresAt,
	}

	plaintext, err := json.Marshal(p)
	if err != nil {
		return "", fmt.Errorf("session: marshal: %w", err)
	}

	nonce := make([]byte, cs.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("session: nonce: %w", err)
	}

	ciphertext := cs.gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.URLEncoding.EncodeToString(ciphertext), nil
}

// Decode decrypts and deserializes a cookie value into a Session.
func (cs *CookieStore) Decode(value string) (*Session, error) {
	ciphertext, err := base64.URLEncoding.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("session: base64 decode: %w", err)
	}

	nonceSize := cs.gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("session: ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := cs.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("session: decrypt: %w", err)
	}

	var p sessionPayload
	if err := json.Unmarshal(plaintext, &p); err != nil {
		return nil, fmt.Errorf("session: unmarshal: %w", err)
	}

	if !p.ExpiresAt.IsZero() && time.Now().After(p.ExpiresAt) {
		return nil, errors.New("session: expired")
	}

	return &Session{
		ID:        p.ID,
		Data:      p.Data,
		UserID:    p.UserID,
		CreatedAt: p.CreatedAt,
		ExpiresAt: p.ExpiresAt,
	}, nil
}
