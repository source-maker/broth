package account

import "testing"

func TestLoginForm_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		form    LoginForm
		wantErr bool
		field   string
	}{
		{"valid", LoginForm{Email: "a@b.com", Password: "secret"}, false, ""},
		{"missing email", LoginForm{Password: "secret"}, true, "email"},
		{"missing password", LoginForm{Email: "a@b.com"}, true, "password"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			errs := tt.form.Validate()
			if tt.wantErr && !errs.HasErrors() {
				t.Fatal("expected validation errors")
			}
			if !tt.wantErr && errs.HasErrors() {
				t.Fatalf("unexpected errors: %v", errs)
			}
			if tt.field != "" {
				if _, ok := errs[tt.field]; !ok {
					t.Errorf("expected error on field %q", tt.field)
				}
			}
		})
	}
}

func TestRegisterForm_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		form    RegisterForm
		wantErr bool
		field   string
	}{
		{
			"valid",
			RegisterForm{Email: "a@b.com", Name: "Test", Password: "12345678", PasswordConfirm: "12345678"},
			false, "",
		},
		{
			"missing email",
			RegisterForm{Name: "Test", Password: "12345678", PasswordConfirm: "12345678"},
			true, "email",
		},
		{
			"short password",
			RegisterForm{Email: "a@b.com", Name: "Test", Password: "short", PasswordConfirm: "short"},
			true, "password",
		},
		{
			"password mismatch",
			RegisterForm{Email: "a@b.com", Name: "Test", Password: "12345678", PasswordConfirm: "different"},
			true, "password_confirm",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			errs := tt.form.Validate()
			if tt.wantErr && !errs.HasErrors() {
				t.Fatal("expected validation errors")
			}
			if !tt.wantErr && errs.HasErrors() {
				t.Fatalf("unexpected errors: %v", errs)
			}
			if tt.field != "" {
				if _, ok := errs[tt.field]; !ok {
					t.Errorf("expected error on field %q", tt.field)
				}
			}
		})
	}
}
