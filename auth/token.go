package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TokenPair holds the access and refresh tokens returned after authentication.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

// TokenClaims holds the claims embedded in an access token.
type TokenClaims struct {
	jwt.RegisteredClaims
	UserID int64    `json:"uid"`
	Email  string   `json:"email"`
	Roles  []string `json:"roles,omitempty"`
}

// TokenService handles JWT token generation and validation.
type TokenService struct {
	secretKey      []byte
	accessLifetime time.Duration
}

// NewTokenService creates a TokenService with the given secret key and access token lifetime.
func NewTokenService(secretKey []byte, accessLifetime time.Duration) *TokenService {
	return &TokenService{
		secretKey:      secretKey,
		accessLifetime: accessLifetime,
	}
}

// GenerateAccessToken creates a signed JWT access token for the given user.
// Uses HMAC-SHA256 for signing.
func (ts *TokenService) GenerateAccessToken(user *User) (string, error) {
	now := time.Now()
	claims := TokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ts.accessLifetime)),
		},
		UserID: user.ID,
		Email:  user.Email,
		Roles:  user.Roles,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(ts.secretKey)
}

// ValidateAccessToken parses and validates a signed access token.
// Verifies HMAC-SHA256 signature and expiration (prevents alg=none attacks).
func (ts *TokenService) ValidateAccessToken(tokenStr string) (*TokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &TokenClaims{},
		func(t *jwt.Token) (any, error) {
			// Verify signing algorithm to prevent alg=none / RS256 confusion attacks
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("auth: unexpected signing method: %v", t.Header["alg"])
			}
			return ts.secretKey, nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("auth: validate token: %w", err)
	}
	claims, ok := token.Claims.(*TokenClaims)
	if !ok || !token.Valid {
		return nil, errors.New("auth: invalid token claims")
	}
	return claims, nil
}

// GenerateRefreshToken creates a cryptographically random refresh token.
func GenerateRefreshToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
