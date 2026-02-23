package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/source-maker/broth/httputil"
)

const requestIDHeader = "X-Request-ID"

// RequestID injects a unique request ID into the context and response header.
// If the incoming request already has an X-Request-ID header, it is reused.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(requestIDHeader)
		if id == "" {
			id = generateID()
		}
		ctx := httputil.Set(r.Context(), httputil.RequestID, id)
		w.Header().Set(requestIDHeader, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func generateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
