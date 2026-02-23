package httputil

import (
	"context"
	"testing"
)

func TestSetAndGet_String(t *testing.T) {
	ctx := context.Background()
	key := CtxKey[string]{Name: "user_id"}

	ctx = Set(ctx, key, "abc-123")

	got, ok := Get(ctx, key)
	if !ok {
		t.Fatal("expected ok=true, got false")
	}
	if got != "abc-123" {
		t.Errorf("value = %q, want %q", got, "abc-123")
	}
}

func TestGet_Missing(t *testing.T) {
	ctx := context.Background()
	key := CtxKey[string]{Name: "missing"}

	got, ok := Get(ctx, key)
	if ok {
		t.Error("expected ok=false for missing key")
	}
	if got != "" {
		t.Errorf("expected zero value, got %q", got)
	}
}

func TestSetAndGet_Int(t *testing.T) {
	ctx := context.Background()
	key := CtxKey[int]{Name: "count"}

	ctx = Set(ctx, key, 42)

	got, ok := Get(ctx, key)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if got != 42 {
		t.Errorf("value = %d, want 42", got)
	}
}

func TestSetAndGet_Bool(t *testing.T) {
	ctx := context.Background()
	key := CtxKey[bool]{Name: "admin"}

	ctx = Set(ctx, key, true)

	got, ok := Get(ctx, key)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if !got {
		t.Error("value = false, want true")
	}
}

func TestDifferentKeysWithSameName_DifferentTypes(t *testing.T) {
	ctx := context.Background()
	strKey := CtxKey[string]{Name: "id"}
	intKey := CtxKey[int]{Name: "id"}

	ctx = Set(ctx, strKey, "hello")
	ctx = Set(ctx, intKey, 99)

	strVal, ok := Get(ctx, strKey)
	if !ok {
		t.Fatal("expected ok=true for string key")
	}
	if strVal != "hello" {
		t.Errorf("string value = %q, want %q", strVal, "hello")
	}

	intVal, ok := Get(ctx, intKey)
	if !ok {
		t.Fatal("expected ok=true for int key")
	}
	if intVal != 99 {
		t.Errorf("int value = %d, want 99", intVal)
	}
}

func TestRequestID_Predefined(t *testing.T) {
	ctx := context.Background()
	ctx = Set(ctx, RequestID, "req-abc-123")

	got, ok := Get(ctx, RequestID)
	if !ok {
		t.Fatal("expected ok=true for RequestID")
	}
	if got != "req-abc-123" {
		t.Errorf("RequestID = %q, want %q", got, "req-abc-123")
	}
}

func TestOverwriteValue(t *testing.T) {
	ctx := context.Background()
	key := CtxKey[string]{Name: "token"}

	ctx = Set(ctx, key, "old")
	ctx = Set(ctx, key, "new")

	got, ok := Get(ctx, key)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if got != "new" {
		t.Errorf("value = %q, want %q", got, "new")
	}
}

func TestGet_ZeroValueOnMissing(t *testing.T) {
	ctx := context.Background()

	intVal, ok := Get(ctx, CtxKey[int]{Name: "x"})
	if ok {
		t.Error("expected ok=false")
	}
	if intVal != 0 {
		t.Errorf("expected 0 for missing int, got %d", intVal)
	}

	boolVal, ok := Get(ctx, CtxKey[bool]{Name: "y"})
	if ok {
		t.Error("expected ok=false")
	}
	if boolVal {
		t.Errorf("expected false for missing bool, got %v", boolVal)
	}
}
