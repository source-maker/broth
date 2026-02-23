package testutil

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// NewTestServer creates an httptest.Server for the given handler
// and registers cleanup to close it when the test finishes.
func NewTestServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

// AssertStatus fails the test if the response status code does not match expected.
func AssertStatus(t *testing.T, resp *http.Response, expected int) {
	t.Helper()
	if resp.StatusCode != expected {
		t.Errorf("expected status %d, got %d", expected, resp.StatusCode)
	}
}
