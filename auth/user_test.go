package auth

import (
	"context"
	"testing"
)

func TestSetUserAndGetUser(t *testing.T) {
	t.Parallel()

	user := &User{
		ID:    42,
		Email: "test@example.com",
		Name:  "Test User",
		Roles: []string{"admin", "editor"},
	}

	ctx := context.Background()
	ctx = SetUser(ctx, user)

	got := GetUser(ctx)
	if got == nil {
		t.Fatal("GetUser returned nil, expected a user")
	}
	if got.ID != user.ID {
		t.Errorf("GetUser().ID = %d, want %d", got.ID, user.ID)
	}
	if got.Email != user.Email {
		t.Errorf("GetUser().Email = %q, want %q", got.Email, user.Email)
	}
	if got.Name != user.Name {
		t.Errorf("GetUser().Name = %q, want %q", got.Name, user.Name)
	}
}

func TestGetUser_EmptyContext(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	got := GetUser(ctx)
	if got != nil {
		t.Errorf("GetUser on empty context = %v, want nil", got)
	}
}

func TestMustGetUser(t *testing.T) {
	t.Parallel()

	user := &User{ID: 1, Email: "admin@example.com", Name: "Admin"}
	ctx := SetUser(context.Background(), user)

	got := MustGetUser(ctx)
	if got.ID != user.ID {
		t.Errorf("MustGetUser().ID = %d, want %d", got.ID, user.ID)
	}
}

func TestMustGetUser_PanicsWhenNoUser(t *testing.T) {
	t.Parallel()

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("MustGetUser did not panic on empty context")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("panic value is %T, want string", r)
		}
		if msg != "auth: no user in context" {
			t.Errorf("panic message = %q, want %q", msg, "auth: no user in context")
		}
	}()

	ctx := context.Background()
	MustGetUser(ctx)
}

func TestIsAuthenticated(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ctx  context.Context
		want bool
	}{
		{
			name: "authenticated user in context",
			ctx:  SetUser(context.Background(), &User{ID: 1}),
			want: true,
		},
		{
			name: "no user in context",
			ctx:  context.Background(),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := IsAuthenticated(tt.ctx); got != tt.want {
				t.Errorf("IsAuthenticated() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasRole(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		roles []string
		query string
		want  bool
	}{
		{
			name:  "user has the role",
			roles: []string{"admin", "editor"},
			query: "admin",
			want:  true,
		},
		{
			name:  "user has the role at second position",
			roles: []string{"admin", "editor"},
			query: "editor",
			want:  true,
		},
		{
			name:  "user does not have the role",
			roles: []string{"admin", "editor"},
			query: "viewer",
			want:  false,
		},
		{
			name:  "empty roles slice",
			roles: nil,
			query: "admin",
			want:  false,
		},
		{
			name:  "single role match",
			roles: []string{"superadmin"},
			query: "superadmin",
			want:  true,
		},
		{
			name:  "case sensitive mismatch",
			roles: []string{"Admin"},
			query: "admin",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			u := &User{Roles: tt.roles}
			if got := u.HasRole(tt.query); got != tt.want {
				t.Errorf("HasRole(%q) = %v, want %v", tt.query, got, tt.want)
			}
		})
	}
}

func TestSetUser_OverwritesPreviousUser(t *testing.T) {
	t.Parallel()

	user1 := &User{ID: 1, Email: "first@example.com"}
	user2 := &User{ID: 2, Email: "second@example.com"}

	ctx := SetUser(context.Background(), user1)
	ctx = SetUser(ctx, user2)

	got := GetUser(ctx)
	if got.ID != user2.ID {
		t.Errorf("after overwrite, GetUser().ID = %d, want %d", got.ID, user2.ID)
	}
	if got.Email != user2.Email {
		t.Errorf("after overwrite, GetUser().Email = %q, want %q", got.Email, user2.Email)
	}
}
