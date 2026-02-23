// Package account provides user account management.
package account

import (
	"fmt"
	"strings"
	"time"
)

// User represents a user account.
type User struct {
	ID           int64
	Email        string
	Name         string
	PasswordHash string
	Roles        []string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// RegisterInput contains the data needed to register a new user.
type RegisterInput struct {
	Email    string
	Name     string
	Password string
}

// Validate checks the registration input for errors.
func (i RegisterInput) Validate() error {
	if strings.TrimSpace(i.Email) == "" {
		return fmt.Errorf("account: register: email is required")
	}
	if !strings.Contains(i.Email, "@") {
		return fmt.Errorf("account: register: invalid email format")
	}
	if strings.TrimSpace(i.Name) == "" {
		return fmt.Errorf("account: register: name is required")
	}
	if len(i.Password) < 8 {
		return fmt.Errorf("account: register: password must be at least 8 characters")
	}
	return nil
}

// LoginInput contains the data needed to authenticate a user.
type LoginInput struct {
	Email    string
	Password string
}

// Validate checks the login input for errors.
func (i LoginInput) Validate() error {
	if strings.TrimSpace(i.Email) == "" {
		return fmt.Errorf("account: login: email is required")
	}
	if i.Password == "" {
		return fmt.Errorf("account: login: password is required")
	}
	return nil
}
