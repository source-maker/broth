package config

import (
	"os"
	"testing"
	"time"
)

func TestMustBind_AllTypes(t *testing.T) {
	t.Setenv("TEST_HOST", "localhost")
	t.Setenv("TEST_PORT", "8080")
	t.Setenv("TEST_DEBUG", "true")
	t.Setenv("TEST_TIMEOUT", "5s")

	type cfg struct {
		Host    string        `env:"TEST_HOST"`
		Port    int           `env:"TEST_PORT"`
		Debug   bool          `env:"TEST_DEBUG"`
		Timeout time.Duration `env:"TEST_TIMEOUT"`
	}

	var c cfg
	MustBind(&c)

	if c.Host != "localhost" {
		t.Errorf("Host = %q, want %q", c.Host, "localhost")
	}
	if c.Port != 8080 {
		t.Errorf("Port = %d, want %d", c.Port, 8080)
	}
	if !c.Debug {
		t.Error("Debug = false, want true")
	}
	if c.Timeout != 5*time.Second {
		t.Errorf("Timeout = %v, want %v", c.Timeout, 5*time.Second)
	}
}

func TestMustBind_Defaults(t *testing.T) {
	type cfg struct {
		Host  string `env:"TEST_DEF_HOST" default:"0.0.0.0"`
		Port  int    `env:"TEST_DEF_PORT" default:"3000"`
		Debug bool   `env:"TEST_DEF_DEBUG" default:"false"`
	}

	os.Unsetenv("TEST_DEF_HOST")
	os.Unsetenv("TEST_DEF_PORT")
	os.Unsetenv("TEST_DEF_DEBUG")

	var c cfg
	MustBind(&c)

	if c.Host != "0.0.0.0" {
		t.Errorf("Host = %q, want %q", c.Host, "0.0.0.0")
	}
	if c.Port != 3000 {
		t.Errorf("Port = %d, want %d", c.Port, 3000)
	}
	if c.Debug {
		t.Error("Debug = true, want false")
	}
}

func TestMustBind_EnvOverridesDefault(t *testing.T) {
	type cfg struct {
		Host string `env:"TEST_OVRD_HOST" default:"fallback"`
	}

	t.Setenv("TEST_OVRD_HOST", "override")

	var c cfg
	MustBind(&c)

	if c.Host != "override" {
		t.Errorf("Host = %q, want %q", c.Host, "override")
	}
}

func TestMustBind_FieldWithoutEnvTag_Skipped(t *testing.T) {
	type cfg struct {
		Internal string
		Host     string `env:"TEST_SKIP_HOST"`
	}

	t.Setenv("TEST_SKIP_HOST", "example.com")

	c := cfg{Internal: "original"}
	MustBind(&c)

	if c.Internal != "original" {
		t.Errorf("Internal = %q, want %q", c.Internal, "original")
	}
	if c.Host != "example.com" {
		t.Errorf("Host = %q, want %q", c.Host, "example.com")
	}
}

func TestMustBind_UnsetOptionalField_ZeroValue(t *testing.T) {
	type cfg struct {
		Port int `env:"TEST_UNSET_PORT"`
	}

	os.Unsetenv("TEST_UNSET_PORT")

	var c cfg
	MustBind(&c)

	if c.Port != 0 {
		t.Errorf("Port = %d, want 0", c.Port)
	}
}

func TestMustBind_DurationDefault(t *testing.T) {
	type cfg struct {
		Timeout time.Duration `env:"TEST_DUR_DEF" default:"30s"`
	}

	os.Unsetenv("TEST_DUR_DEF")

	var c cfg
	MustBind(&c)

	if c.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want %v", c.Timeout, 30*time.Second)
	}
}

func TestMustBind_BoolVariants(t *testing.T) {
	tests := []struct {
		raw  string
		want bool
	}{
		{"true", true},
		{"false", false},
		{"1", true},
		{"0", false},
		{"t", true},
		{"f", false},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			type cfg struct {
				Flag bool `env:"TEST_BOOL_VAR"`
			}
			t.Setenv("TEST_BOOL_VAR", tt.raw)
			var c cfg
			MustBind(&c)
			if c.Flag != tt.want {
				t.Errorf("Flag = %v, want %v for raw=%q", c.Flag, tt.want, tt.raw)
			}
		})
	}
}

func TestMustBind_PanicOnNonPointer(t *testing.T) {
	type cfg struct {
		Host string `env:"X"`
	}
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for non-pointer, got none")
		}
	}()
	MustBind(cfg{})
}

func TestMustBind_PanicOnRequiredMissing(t *testing.T) {
	type cfg struct {
		Secret string `env:"TEST_REQUIRED_SECRET" required:"true"`
	}

	os.Unsetenv("TEST_REQUIRED_SECRET")

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for missing required env var, got none")
		}
	}()

	var c cfg
	MustBind(&c)
}

func TestMustBind_PanicOnUnsupportedType(t *testing.T) {
	type cfg struct {
		Data []string `env:"TEST_UNSUPPORTED"`
	}

	t.Setenv("TEST_UNSUPPORTED", "a,b,c")

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for unsupported type, got none")
		}
	}()

	var c cfg
	MustBind(&c)
}

func TestMustBind_PanicOnInvalidInt(t *testing.T) {
	type cfg struct {
		Port int `env:"TEST_INVALID_INT"`
	}

	t.Setenv("TEST_INVALID_INT", "not-a-number")

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for invalid int, got none")
		}
	}()

	var c cfg
	MustBind(&c)
}

func TestMustBind_RequiredWithDefault_NoEnv(t *testing.T) {
	type cfg struct {
		Host string `env:"TEST_REQ_DEF" required:"true" default:"fallback"`
	}

	os.Unsetenv("TEST_REQ_DEF")

	var c cfg
	MustBind(&c)

	if c.Host != "fallback" {
		t.Errorf("Host = %q, want %q", c.Host, "fallback")
	}
}
