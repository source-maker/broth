package middleware

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleAuthError_API_EncodesJSON(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	err := errors.New(`bad "token"` + "\n" + `format`)

	handleAuthError(rec, req, ContextAPI, err)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	var body map[string]string
	if decodeErr := json.NewDecoder(rec.Body).Decode(&body); decodeErr != nil {
		t.Fatalf("response body should be valid JSON: %v", decodeErr)
	}
	if body["error"] != err.Error() {
		t.Errorf("error = %q, want %q", body["error"], err.Error())
	}
}
