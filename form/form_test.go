package form

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestErrors_AddAndHasErrors(t *testing.T) {
	errs := make(Errors)

	if errs.HasErrors() {
		t.Error("new Errors should have no errors")
	}

	errs.Add("email", "is required")

	if !errs.HasErrors() {
		t.Error("should have errors after Add")
	}
	if got := errs["email"]; len(got) != 1 || got[0] != "is required" {
		t.Errorf("email errors = %v, want [is required]", got)
	}
}

func TestErrors_AddMultipleToSameField(t *testing.T) {
	errs := make(Errors)
	errs.Add("password", "is required")
	errs.Add("password", "must be at least 8 characters")

	if got := len(errs["password"]); got != 2 {
		t.Errorf("password error count = %d, want 2", got)
	}
}

func TestRequiredString_Empty(t *testing.T) {
	errs := make(Errors)
	RequiredString(errs, "name", "", "Name")

	if !errs.HasErrors() {
		t.Error("expected error for empty string")
	}
	if got := errs["name"][0]; got != "Name is required" {
		t.Errorf("error = %q, want %q", got, "Name is required")
	}
}

func TestRequiredString_Whitespace(t *testing.T) {
	errs := make(Errors)
	RequiredString(errs, "name", "   ", "Name")

	if !errs.HasErrors() {
		t.Error("expected error for whitespace-only string")
	}
}

func TestRequiredString_NonEmpty(t *testing.T) {
	errs := make(Errors)
	RequiredString(errs, "name", "Alice", "Name")

	if errs.HasErrors() {
		t.Error("should not have errors for non-empty string")
	}
}

func newPostRequest(t *testing.T, formData url.Values) *http.Request {
	t.Helper()
	body := formData.Encode()
	r := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return r
}

func TestBind_BasicFields(t *testing.T) {
	type loginForm struct {
		Email    string `form:"email"`
		Password string `form:"password"`
	}

	r := newPostRequest(t, url.Values{
		"email":    {"alice@example.com"},
		"password": {"secret123"},
	})

	var f loginForm
	if err := Bind(r, &f); err != nil {
		t.Fatalf("Bind error: %v", err)
	}

	if f.Email != "alice@example.com" {
		t.Errorf("Email = %q, want %q", f.Email, "alice@example.com")
	}
	if f.Password != "secret123" {
		t.Errorf("Password = %q, want %q", f.Password, "secret123")
	}
}

func TestBind_MissingFormValue_EmptyString(t *testing.T) {
	type myForm struct {
		Name string `form:"name"`
	}

	r := newPostRequest(t, url.Values{})

	var f myForm
	if err := Bind(r, &f); err != nil {
		t.Fatalf("Bind error: %v", err)
	}

	if f.Name != "" {
		t.Errorf("Name = %q, want empty string", f.Name)
	}
}

func TestBind_SkipsFieldsWithoutTag(t *testing.T) {
	type myForm struct {
		Internal string
		Email    string `form:"email"`
	}

	r := newPostRequest(t, url.Values{
		"email": {"bob@example.com"},
	})

	f := myForm{Internal: "keep-me"}
	if err := Bind(r, &f); err != nil {
		t.Fatalf("Bind error: %v", err)
	}

	if f.Internal != "keep-me" {
		t.Errorf("Internal = %q, want %q", f.Internal, "keep-me")
	}
	if f.Email != "bob@example.com" {
		t.Errorf("Email = %q, want %q", f.Email, "bob@example.com")
	}
}

func TestBind_SkipsDashTag(t *testing.T) {
	type myForm struct {
		Secret string `form:"-"`
		Name   string `form:"name"`
	}

	r := newPostRequest(t, url.Values{
		"name": {"Alice"},
	})

	f := myForm{Secret: "keep"}
	if err := Bind(r, &f); err != nil {
		t.Fatalf("Bind error: %v", err)
	}

	if f.Secret != "keep" {
		t.Errorf("Secret = %q, want %q", f.Secret, "keep")
	}
}

func TestBind_ErrorOnNonPointer(t *testing.T) {
	type myForm struct {
		Name string `form:"name"`
	}

	r := newPostRequest(t, url.Values{"name": {"x"}})

	err := Bind(r, myForm{})
	if err == nil {
		t.Fatal("expected error for non-pointer, got nil")
	}
}

func TestBind_GET_QueryParams(t *testing.T) {
	type searchForm struct {
		Query string `form:"q"`
	}

	r := httptest.NewRequest(http.MethodGet, "/search?q=golang", nil)

	var f searchForm
	if err := Bind(r, &f); err != nil {
		t.Fatalf("Bind error: %v", err)
	}

	if f.Query != "golang" {
		t.Errorf("Query = %q, want %q", f.Query, "golang")
	}
}

type validatedForm struct {
	Email string
}

func (f *validatedForm) Validate() Errors {
	errs := make(Errors)
	RequiredString(errs, "email", f.Email, "Email")
	return errs
}

func TestValidate_WithErrors(t *testing.T) {
	f := &validatedForm{Email: ""}
	errs := Validate(f)

	if errs == nil {
		t.Fatal("expected errors, got nil")
	}
	if !errs.HasErrors() {
		t.Error("expected HasErrors to be true")
	}
}

func TestValidate_NoErrors(t *testing.T) {
	f := &validatedForm{Email: "alice@example.com"}
	errs := Validate(f)

	if errs != nil {
		t.Errorf("expected nil, got %v", errs)
	}
}

func TestValidate_NonValidator(t *testing.T) {
	type plain struct {
		Name string
	}

	errs := Validate(&plain{Name: "x"})
	if errs != nil {
		t.Errorf("expected nil for non-Validator, got %v", errs)
	}
}
