// Package form provides HTTP form binding and validation.
// Reflection is used here as permitted by ADR-003.
package form

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"
)

// Errors holds field-level validation errors.
type Errors map[string][]string

// HasErrors returns true if there are any validation errors.
func (e Errors) HasErrors() bool {
	return len(e) > 0
}

// Add appends an error message for the given field.
func (e Errors) Add(field, msg string) {
	e[field] = append(e[field], msg)
}

// Bind parses form data from the request and populates dst.
// dst must be a pointer to a struct. Fields are matched by the `form` struct tag.
func Bind(r *http.Request, dst any) error {
	if err := r.ParseForm(); err != nil {
		return fmt.Errorf("form: parse: %w", err)
	}

	v := reflect.ValueOf(dst)
	if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("form: Bind requires a pointer to a struct")
	}
	v = v.Elem()
	t := v.Type()

	for i := range t.NumField() {
		field := t.Field(i)
		tag := field.Tag.Get("form")
		if tag == "" || tag == "-" {
			continue
		}
		val := r.FormValue(tag)
		if fv := v.Field(i); fv.CanSet() && fv.Kind() == reflect.String {
			fv.SetString(val)
		}
	}
	return nil
}

// Validator is implemented by form structs that support validation.
type Validator interface {
	Validate() Errors
}

// Validate calls the Validator interface on v if it implements it.
// Returns nil if v does not implement Validator or if validation passes.
func Validate(v any) Errors {
	if val, ok := v.(Validator); ok {
		errs := val.Validate()
		if errs.HasErrors() {
			return errs
		}
	}
	return nil
}

// TextField describes a text input field for form rendering.
type TextField struct {
	Name        string
	Label       string
	Value       string
	Placeholder string
	Required    bool
	Errors      []string
}

// PasswordField describes a password input field for form rendering.
type PasswordField struct {
	Name        string
	Label       string
	Placeholder string
	Required    bool
	MinLength   int
	Errors      []string
}

// RequiredString validates that a string field is not empty.
func RequiredString(errs Errors, field, value, label string) {
	if strings.TrimSpace(value) == "" {
		errs.Add(field, label+" is required")
	}
}
