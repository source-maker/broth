package session

import (
	"context"
	"testing"
	"time"
)

func TestFromContext_NoSession(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	s := FromContext(ctx)
	if s != nil {
		t.Errorf("FromContext on empty context = %v, want nil", s)
	}
}

func TestNewContextAndFromContext(t *testing.T) {
	t.Parallel()

	sess := &Session{
		ID:        "sess-abc-123",
		Data:      map[string]any{"theme": "dark"},
		UserID:    42,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(24 * time.Hour),
		IsNew:     false,
	}

	ctx := NewContext(context.Background(), sess)
	got := FromContext(ctx)

	if got == nil {
		t.Fatal("FromContext returned nil after NewContext")
	}
	if got.ID != sess.ID {
		t.Errorf("session ID = %q, want %q", got.ID, sess.ID)
	}
	if got.UserID != sess.UserID {
		t.Errorf("session UserID = %d, want %d", got.UserID, sess.UserID)
	}
}

func TestGetString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data map[string]any
		key  string
		want string
	}{
		{
			name: "existing string key",
			data: map[string]any{"theme": "dark", "lang": "en"},
			key:  "theme",
			want: "dark",
		},
		{
			name: "missing key",
			data: map[string]any{"theme": "dark"},
			key:  "lang",
			want: "",
		},
		{
			name: "nil data map",
			data: nil,
			key:  "theme",
			want: "",
		},
		{
			name: "non-string value returns empty",
			data: map[string]any{"count": 42},
			key:  "count",
			want: "",
		},
		{
			name: "empty string value",
			data: map[string]any{"name": ""},
			key:  "name",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := &Session{Data: tt.data}
			if got := s.GetString(tt.key); got != tt.want {
				t.Errorf("GetString(%q) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}

func TestSet(t *testing.T) {
	t.Parallel()

	t.Run("set on nil data initializes map", func(t *testing.T) {
		t.Parallel()
		s := &Session{}
		if s.Data != nil {
			t.Fatal("precondition: Data should be nil")
		}

		s.Set("key", "value")

		if s.Data == nil {
			t.Fatal("Set did not initialize Data map")
		}
		if got := s.Data["key"]; got != "value" {
			t.Errorf("Data[key] = %v, want %q", got, "value")
		}
	})

	t.Run("set on existing data", func(t *testing.T) {
		t.Parallel()
		s := &Session{Data: map[string]any{"existing": "yes"}}

		s.Set("new_key", 123)

		if got := s.Data["new_key"]; got != 123 {
			t.Errorf("Data[new_key] = %v, want 123", got)
		}
		if got := s.Data["existing"]; got != "yes" {
			t.Errorf("Data[existing] = %v, want %q", got, "yes")
		}
	})

	t.Run("set overwrites existing key", func(t *testing.T) {
		t.Parallel()
		s := &Session{Data: map[string]any{"key": "old"}}

		s.Set("key", "new")

		if got := s.Data["key"]; got != "new" {
			t.Errorf("Data[key] = %v, want %q", got, "new")
		}
	})

	t.Run("set various types", func(t *testing.T) {
		t.Parallel()
		s := &Session{}

		s.Set("str", "hello")
		s.Set("num", 42)
		s.Set("bool", true)

		if got, ok := s.Data["str"].(string); !ok || got != "hello" {
			t.Errorf("Data[str] = %v, want %q", s.Data["str"], "hello")
		}
		if got, ok := s.Data["num"].(int); !ok || got != 42 {
			t.Errorf("Data[num] = %v, want 42", s.Data["num"])
		}
		if got, ok := s.Data["bool"].(bool); !ok || got != true {
			t.Errorf("Data[bool] = %v, want true", s.Data["bool"])
		}
	})
}

func TestNewContext_OverwritesPreviousSession(t *testing.T) {
	t.Parallel()

	sess1 := &Session{ID: "sess-1"}
	sess2 := &Session{ID: "sess-2"}

	ctx := NewContext(context.Background(), sess1)
	ctx = NewContext(ctx, sess2)

	got := FromContext(ctx)
	if got.ID != "sess-2" {
		t.Errorf("after overwrite, session ID = %q, want %q", got.ID, "sess-2")
	}
}

func TestGetString_AfterSet(t *testing.T) {
	t.Parallel()

	s := &Session{}
	s.Set("flash", "Welcome back!")

	got := s.GetString("flash")
	if got != "Welcome back!" {
		t.Errorf("GetString after Set = %q, want %q", got, "Welcome back!")
	}
}

func TestRegenerate(t *testing.T) {
	t.Parallel()

	s := &Session{ID: "old-id"}
	s.Regenerate()

	if s.ID == "old-id" {
		t.Error("Regenerate should change the session ID")
	}
	if s.ID == "" {
		t.Error("Regenerate should set a new non-empty ID")
	}
	if !s.IsRegenerated() {
		t.Error("IsRegenerated should return true")
	}
	if s.OldID() != "old-id" {
		t.Errorf("OldID = %q, want %q", s.OldID(), "old-id")
	}
	if !s.IsModified() {
		t.Error("IsModified should return true after Regenerate")
	}
}

func TestDestroy(t *testing.T) {
	t.Parallel()

	s := &Session{ID: "to-destroy"}
	s.Destroy()

	if !s.IsDestroyed() {
		t.Error("IsDestroyed should return true")
	}
	if !s.IsModified() {
		t.Error("IsModified should return true after Destroy")
	}
}

func TestSetMaxAge(t *testing.T) {
	t.Parallel()

	s := &Session{}
	before := time.Now()
	s.SetMaxAge(3600)
	after := time.Now()

	if s.MaxAge() != 3600 {
		t.Errorf("MaxAge = %d, want 3600", s.MaxAge())
	}
	if s.ExpiresAt.Before(before.Add(3600*time.Second)) || s.ExpiresAt.After(after.Add(3600*time.Second)) {
		t.Error("ExpiresAt should be approximately now + 3600 seconds")
	}
	if !s.IsModified() {
		t.Error("IsModified should return true after SetMaxAge")
	}
}

func TestGenerateID(t *testing.T) {
	t.Parallel()

	id := GenerateID()
	if len(id) != 64 { // 32 bytes * 2 hex chars
		t.Errorf("GenerateID length = %d, want 64", len(id))
	}

	// Uniqueness check
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := GenerateID()
		if ids[id] {
			t.Fatalf("duplicate ID generated: %s", id)
		}
		ids[id] = true
	}
}

func TestSetMarksModified(t *testing.T) {
	t.Parallel()

	s := &Session{}
	if s.IsModified() {
		t.Error("new session should not be modified")
	}
	s.Set("key", "value")
	if !s.IsModified() {
		t.Error("session should be modified after Set")
	}
}
