package auth

import (
	"strings"
	"testing"
	"time"
)

func TestGenerateAndValidateAccessToken(t *testing.T) {
	t.Parallel()

	ts := NewTokenService([]byte("test-secret-key-32-bytes-long!!!"), 15*time.Minute)
	user := &User{
		ID:    42,
		Email: "alice@example.com",
		Roles: []string{"admin", "editor"},
	}

	token, err := ts.GenerateAccessToken(user)
	if err != nil {
		t.Fatalf("GenerateAccessToken error: %v", err)
	}
	if token == "" {
		t.Fatal("token should not be empty")
	}

	claims, err := ts.ValidateAccessToken(token)
	if err != nil {
		t.Fatalf("ValidateAccessToken error: %v", err)
	}

	if claims.UserID != 42 {
		t.Errorf("UserID = %d, want 42", claims.UserID)
	}
	if claims.Email != "alice@example.com" {
		t.Errorf("Email = %q, want %q", claims.Email, "alice@example.com")
	}
	if len(claims.Roles) != 2 || claims.Roles[0] != "admin" {
		t.Errorf("Roles = %v, want [admin editor]", claims.Roles)
	}
}

func TestValidateAccessToken_Expired(t *testing.T) {
	t.Parallel()

	ts := NewTokenService([]byte("test-secret-key-32-bytes-long!!!"), -1*time.Hour)
	user := &User{ID: 1, Email: "test@example.com"}

	token, err := ts.GenerateAccessToken(user)
	if err != nil {
		t.Fatalf("GenerateAccessToken error: %v", err)
	}

	_, err = ts.ValidateAccessToken(token)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestValidateAccessToken_InvalidSignature(t *testing.T) {
	t.Parallel()

	ts1 := NewTokenService([]byte("secret-key-one-32-bytes-long!!!!"), 15*time.Minute)
	ts2 := NewTokenService([]byte("secret-key-two-32-bytes-long!!!!"), 15*time.Minute)

	user := &User{ID: 1, Email: "test@example.com"}
	token, _ := ts1.GenerateAccessToken(user)

	_, err := ts2.ValidateAccessToken(token)
	if err == nil {
		t.Fatal("expected error for invalid signature")
	}
}

func TestValidateAccessToken_Tampered(t *testing.T) {
	t.Parallel()

	ts := NewTokenService([]byte("test-secret-key-32-bytes-long!!!"), 15*time.Minute)
	user := &User{ID: 1, Email: "test@example.com"}

	token, _ := ts.GenerateAccessToken(user)
	tampered := token + "x"

	_, err := ts.ValidateAccessToken(tampered)
	if err == nil {
		t.Fatal("expected error for tampered token")
	}
}

func TestValidateAccessToken_EmptyString(t *testing.T) {
	t.Parallel()

	ts := NewTokenService([]byte("test-secret-key-32-bytes-long!!!"), 15*time.Minute)

	_, err := ts.ValidateAccessToken("")
	if err == nil {
		t.Fatal("expected error for empty token")
	}
}

func TestValidateAccessToken_MalformedJWT(t *testing.T) {
	t.Parallel()

	ts := NewTokenService([]byte("test-secret-key-32-bytes-long!!!"), 15*time.Minute)

	_, err := ts.ValidateAccessToken("not.a.jwt")
	if err == nil {
		t.Fatal("expected error for malformed JWT")
	}
}

func TestGenerateAccessToken_NoRoles(t *testing.T) {
	t.Parallel()

	ts := NewTokenService([]byte("test-secret-key-32-bytes-long!!!"), 15*time.Minute)
	user := &User{ID: 1, Email: "noroles@example.com"}

	token, err := ts.GenerateAccessToken(user)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	claims, err := ts.ValidateAccessToken(token)
	if err != nil {
		t.Fatalf("validate error: %v", err)
	}
	if len(claims.Roles) != 0 {
		t.Errorf("Roles = %v, want empty", claims.Roles)
	}
}

func TestGenerateRefreshToken(t *testing.T) {
	t.Parallel()

	token, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(token) != 64 { // 32 bytes = 64 hex chars
		t.Errorf("token length = %d, want 64", len(token))
	}

	// Uniqueness
	token2, _ := GenerateRefreshToken()
	if token == token2 {
		t.Error("refresh tokens should be unique")
	}
}

func TestTokenContainsExpiry(t *testing.T) {
	t.Parallel()

	ts := NewTokenService([]byte("test-secret-key-32-bytes-long!!!"), 30*time.Minute)
	user := &User{ID: 1, Email: "test@example.com"}

	token, _ := ts.GenerateAccessToken(user)
	claims, _ := ts.ValidateAccessToken(token)

	if claims.ExpiresAt == nil {
		t.Fatal("ExpiresAt should not be nil")
	}
	if claims.IssuedAt == nil {
		t.Fatal("IssuedAt should not be nil")
	}

	// Token should expire approximately 30 minutes from now
	expectedExpiry := time.Now().Add(30 * time.Minute)
	diff := claims.ExpiresAt.Time.Sub(expectedExpiry)
	if diff > 5*time.Second || diff < -5*time.Second {
		t.Errorf("expiry diff from expected = %v, should be within 5s", diff)
	}
}

func TestTokenJWTFormat(t *testing.T) {
	t.Parallel()

	ts := NewTokenService([]byte("test-secret-key-32-bytes-long!!!"), 15*time.Minute)
	user := &User{ID: 1, Email: "test@example.com"}

	token, _ := ts.GenerateAccessToken(user)

	// JWT should have 3 parts separated by dots
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Errorf("JWT parts = %d, want 3", len(parts))
	}
}
