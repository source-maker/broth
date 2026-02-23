package account

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/source-maker/broth/auth"
	"github.com/source-maker/broth/log"
)

// mockRepo is an in-memory repository for testing.
type mockRepo struct {
	users  []*User
	nextID int64
}

func newMockRepo() *mockRepo {
	return &mockRepo{nextID: 1}
}

func (r *mockRepo) Create(_ context.Context, user *User) error {
	user.ID = r.nextID
	r.nextID++
	r.users = append(r.users, user)
	return nil
}

func (r *mockRepo) FindByID(_ context.Context, id int64) (*User, error) {
	for _, u := range r.users {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, nil
}

func (r *mockRepo) FindByEmail(_ context.Context, email string) (*User, error) {
	for _, u := range r.users {
		if u.Email == email {
			return u, nil
		}
	}
	return nil, nil
}

func newTestService() (*Service, *mockRepo) {
	repo := newMockRepo()
	hasher := auth.NewBcryptHasher(4) // Low cost for fast tests
	logger := log.New(slog.LevelError)
	_ = os.Stderr
	svc := NewService(repo, hasher, logger)
	return svc, repo
}

func TestService_Register(t *testing.T) {
	t.Parallel()
	svc, _ := newTestService()

	user, err := svc.Register(context.Background(), RegisterInput{
		Email:    "alice@example.com",
		Name:     "Alice",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if user.ID == 0 {
		t.Error("user ID should be set")
	}
	if user.Email != "alice@example.com" {
		t.Errorf("email = %q, want %q", user.Email, "alice@example.com")
	}
	if user.PasswordHash == "" {
		t.Error("password hash should be set")
	}
	if user.PasswordHash == "password123" {
		t.Error("password hash should not be plaintext")
	}
}

func TestService_Register_DuplicateEmail(t *testing.T) {
	t.Parallel()
	svc, _ := newTestService()

	input := RegisterInput{
		Email:    "bob@example.com",
		Name:     "Bob",
		Password: "password123",
	}

	if _, err := svc.Register(context.Background(), input); err != nil {
		t.Fatalf("first register failed: %v", err)
	}

	_, err := svc.Register(context.Background(), input)
	if err == nil {
		t.Fatal("expected duplicate email error")
	}
}

func TestService_Register_InvalidInput(t *testing.T) {
	t.Parallel()
	svc, _ := newTestService()

	_, err := svc.Register(context.Background(), RegisterInput{
		Email:    "",
		Name:     "Test",
		Password: "password123",
	})
	if err == nil {
		t.Fatal("expected validation error for empty email")
	}
}

func TestService_Authenticate(t *testing.T) {
	t.Parallel()
	svc, _ := newTestService()

	// Register first
	_, err := svc.Register(context.Background(), RegisterInput{
		Email:    "carol@example.com",
		Name:     "Carol",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Authenticate with correct credentials
	user, err := svc.Authenticate(context.Background(), LoginInput{
		Email:    "carol@example.com",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("Authenticate failed: %v", err)
	}
	if user.Email != "carol@example.com" {
		t.Errorf("email = %q, want %q", user.Email, "carol@example.com")
	}
}

func TestService_Authenticate_WrongPassword(t *testing.T) {
	t.Parallel()
	svc, _ := newTestService()

	_, _ = svc.Register(context.Background(), RegisterInput{
		Email:    "dave@example.com",
		Name:     "Dave",
		Password: "password123",
	})

	_, err := svc.Authenticate(context.Background(), LoginInput{
		Email:    "dave@example.com",
		Password: "wrongpassword",
	})
	if err == nil {
		t.Fatal("expected authentication error for wrong password")
	}
}

func TestService_Authenticate_UnknownUser(t *testing.T) {
	t.Parallel()
	svc, _ := newTestService()

	_, err := svc.Authenticate(context.Background(), LoginInput{
		Email:    "unknown@example.com",
		Password: "password123",
	})
	if err == nil {
		t.Fatal("expected authentication error for unknown user")
	}
}

func TestService_FindByID(t *testing.T) {
	t.Parallel()
	svc, _ := newTestService()

	created, _ := svc.Register(context.Background(), RegisterInput{
		Email:    "eve@example.com",
		Name:     "Eve",
		Password: "password123",
	})

	found, err := svc.FindByID(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("FindByID failed: %v", err)
	}
	if found == nil {
		t.Fatal("expected to find user")
	}
	if found.Email != "eve@example.com" {
		t.Errorf("email = %q, want %q", found.Email, "eve@example.com")
	}
}

func TestService_FindByID_NotFound(t *testing.T) {
	t.Parallel()
	svc, _ := newTestService()

	found, err := svc.FindByID(context.Background(), 999)
	if err != nil {
		t.Fatalf("FindByID failed: %v", err)
	}
	if found != nil {
		t.Error("expected nil for non-existent user")
	}
}
