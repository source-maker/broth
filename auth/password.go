package auth

import (
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// PasswordHasher defines the interface for password hashing and verification.
type PasswordHasher interface {
	Hash(password string) (string, error)
	Verify(password, hash string) error
}

// BcryptHasher is a bcrypt-based password hasher.
type BcryptHasher struct {
	cost int
}

// NewBcryptHasher creates a BcryptHasher with the given cost.
// If cost is 0, bcrypt.DefaultCost (10) is used.
func NewBcryptHasher(cost int) *BcryptHasher {
	if cost == 0 {
		cost = bcrypt.DefaultCost
	}
	return &BcryptHasher{cost: cost}
}

// Hash hashes a password using bcrypt.
func (h *BcryptHasher) Hash(password string) (string, error) {
	if len(password) < 8 {
		return "", errors.New("auth: password must be at least 8 characters")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), h.cost)
	if err != nil {
		return "", fmt.Errorf("auth: bcrypt hash: %w", err)
	}
	return string(hash), nil
}

// Verify checks if a password matches the given bcrypt hash.
func (h *BcryptHasher) Verify(password, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

// DefaultPasswordHasher is the default password hasher using bcrypt.
var DefaultPasswordHasher PasswordHasher = NewBcryptHasher(0)
