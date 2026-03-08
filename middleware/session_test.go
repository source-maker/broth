package middleware

import (
	"bufio"
	"crypto/rand"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/source-maker/broth/session"
)

func testCookieStore(t *testing.T) *session.CookieStore {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	store, err := session.NewCookieStore(key)
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func TestSessionMiddleware_CreatesNewSession(t *testing.T) {
	t.Parallel()

	store := testCookieStore(t)
	var gotSession *session.Session

	handler := Session(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSession = session.FromContext(r.Context())
		io.WriteString(w, "ok")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	if gotSession == nil {
		t.Fatal("session should be in context")
	}
	if !gotSession.IsNew {
		t.Error("session should be marked as new")
	}
	if gotSession.ID == "" {
		t.Error("session ID should not be empty")
	}
}

func TestSessionMiddleware_SetsCookie(t *testing.T) {
	t.Parallel()

	store := testCookieStore(t)

	handler := Session(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess := session.FromContext(r.Context())
		sess.Set("key", "value")
		io.WriteString(w, "ok")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	cookies := rec.Result().Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == "_broth_session" {
			found = true
			if !c.HttpOnly {
				t.Error("session cookie should be HttpOnly")
			}
			if c.Value == "" {
				t.Error("session cookie value should not be empty")
			}
		}
	}
	if !found {
		t.Error("session cookie not set")
	}
}

func TestSessionMiddleware_LoadsExistingSession(t *testing.T) {
	t.Parallel()

	store := testCookieStore(t)
	var gotValue string

	handler := Session(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess := session.FromContext(r.Context())
		gotValue = sess.GetString("greeting")
		io.WriteString(w, "ok")
	}))

	// First request: set session data
	rec1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodGet, "/", nil)

	Session(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess := session.FromContext(r.Context())
		sess.Set("greeting", "hello")
		io.WriteString(w, "ok")
	})).ServeHTTP(rec1, req1)

	// Extract cookie from first response
	cookies := rec1.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "_broth_session" {
			sessionCookie = c
		}
	}
	if sessionCookie == nil {
		t.Fatal("no session cookie in first response")
	}

	// Second request: read session data
	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.AddCookie(sessionCookie)
	handler.ServeHTTP(rec2, req2)

	if gotValue != "hello" {
		t.Errorf("session value = %q, want %q", gotValue, "hello")
	}
}

func TestSessionMiddleware_Destroy(t *testing.T) {
	t.Parallel()

	store := testCookieStore(t)

	handler := Session(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess := session.FromContext(r.Context())
		sess.Destroy()
		io.WriteString(w, "ok")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	cookies := rec.Result().Cookies()
	for _, c := range cookies {
		if c.Name == "_broth_session" {
			if c.MaxAge != -1 {
				t.Errorf("destroyed session cookie MaxAge = %d, want -1", c.MaxAge)
			}
			if c.Value != "" {
				t.Errorf("destroyed session cookie value = %q, want empty", c.Value)
			}
			return
		}
	}
	t.Error("expected session cookie to be set (with MaxAge=-1)")
}

func TestSessionMiddleware_Regenerate(t *testing.T) {
	t.Parallel()

	store := testCookieStore(t)
	var originalID, newID string

	// First request: create session
	rec1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodGet, "/", nil)

	Session(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess := session.FromContext(r.Context())
		originalID = sess.ID
		sess.Set("data", "keepme")
		io.WriteString(w, "ok")
	})).ServeHTTP(rec1, req1)

	var sessionCookie *http.Cookie
	for _, c := range rec1.Result().Cookies() {
		if c.Name == "_broth_session" {
			sessionCookie = c
		}
	}
	if sessionCookie == nil {
		t.Fatal("no session cookie in first response")
	}

	// Second request: regenerate session
	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.AddCookie(sessionCookie)

	Session(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess := session.FromContext(r.Context())
		sess.Regenerate()
		newID = sess.ID
		io.WriteString(w, "ok")
	})).ServeHTTP(rec2, req2)

	if newID == originalID {
		t.Error("regenerated session should have a new ID")
	}
	if newID == "" {
		t.Error("regenerated session ID should not be empty")
	}
}

func TestSessionMiddleware_CustomCookieName(t *testing.T) {
	t.Parallel()

	store := testCookieStore(t)

	handler := Session(store, SessionConfig{CookieName: "my_session"})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sess := session.FromContext(r.Context())
			sess.Set("x", "y")
			io.WriteString(w, "ok")
		}),
	)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	found := false
	for _, c := range rec.Result().Cookies() {
		if c.Name == "my_session" {
			found = true
		}
	}
	if !found {
		t.Error("expected custom cookie name 'my_session'")
	}
}

func TestSessionMiddleware_NoWriteWhenUnmodified(t *testing.T) {
	t.Parallel()

	store := testCookieStore(t)

	// Create a session first
	rec1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodGet, "/", nil)
	Session(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess := session.FromContext(r.Context())
		sess.Set("init", "true")
		io.WriteString(w, "ok")
	})).ServeHTTP(rec1, req1)

	var sessionCookie *http.Cookie
	for _, c := range rec1.Result().Cookies() {
		if c.Name == "_broth_session" {
			sessionCookie = c
		}
	}
	if sessionCookie == nil {
		t.Fatal("no session cookie in first response")
	}

	// Second request: don't modify session
	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.AddCookie(sessionCookie)

	Session(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read only, no modifications
		_ = session.FromContext(r.Context())
		io.WriteString(w, "ok")
	})).ServeHTTP(rec2, req2)

	// Should not set a new cookie when session is unmodified
	cookies := rec2.Result().Cookies()
	for _, c := range cookies {
		if c.Name == "_broth_session" {
			t.Error("should not set session cookie when session is unmodified")
		}
	}
}

func TestSessionMiddleware_InvalidCookieCreatesNew(t *testing.T) {
	t.Parallel()

	store := testCookieStore(t)
	var gotSession *session.Session

	handler := Session(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSession = session.FromContext(r.Context())
		io.WriteString(w, "ok")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "_broth_session", Value: "garbage-value"})
	handler.ServeHTTP(rec, req)

	if gotSession == nil {
		t.Fatal("session should be in context")
	}
	if !gotSession.IsNew {
		t.Error("invalid cookie should result in a new session")
	}
}

func TestSessionWriterFlushDelegatesAndSavesSession(t *testing.T) {
	t.Parallel()

	store := testCookieStore(t)
	sess := &session.Session{
		ID:        session.GenerateID(),
		Data:      map[string]any{"hello": "world"},
		CreatedAt: time.Now(),
		IsNew:     true,
	}
	rw := &sessionFlushRecorder{ResponseRecorder: httptest.NewRecorder()}
	sw := &sessionWriter{
		ResponseWriter: rw,
		sess:           sess,
		store:          store,
		cfg: SessionConfig{
			CookieName: "_broth_session",
			SameSite:   http.SameSiteLaxMode,
			Path:       "/",
		},
	}

	sw.Flush()

	if !rw.flushed {
		t.Error("Flush should delegate to the wrapped ResponseWriter")
	}
	if !sw.headerWritten {
		t.Error("Flush should mark headers as written after saving session")
	}

	found := false
	for _, c := range rw.Result().Cookies() {
		if c.Name == "_broth_session" {
			found = true
		}
	}
	if !found {
		t.Error("Flush should save the session cookie before flushing")
	}
}

func TestSessionWriterUnwrapReturnsOriginalWriter(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	sw := &sessionWriter{ResponseWriter: rec}

	if got := sw.Unwrap(); got != rec {
		t.Errorf("Unwrap() = %T, want original recorder", got)
	}
}

type sessionFlushRecorder struct {
	*httptest.ResponseRecorder
	flushed bool
}

func (f *sessionFlushRecorder) Flush() {
	f.flushed = true
	f.ResponseRecorder.Flush()
}

func (f *sessionFlushRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return nil, nil, http.ErrNotSupported
}
