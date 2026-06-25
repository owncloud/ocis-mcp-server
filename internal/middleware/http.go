// Package middleware provides HTTP middleware for the MCP server's HTTP transport.
package middleware

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"strings"
)

// SecurityHeaders sets baseline response security headers on every request.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

// RequireBearer returns middleware that authenticates every request against a static
// shared secret: callers must present `Authorization: Bearer <secret>`. Requests without a
// valid token receive 401 and are not forwarded to the wrapped handler.
//
// The comparison is constant-time: both the provided token and the configured secret are
// SHA-256 hashed before comparison, which keeps the check independent of the secret's value
// and length (so it leaks neither via timing).
//
// When secret is empty the middleware is a pass-through and the endpoint is left
// unauthenticated. That is only safe on a trusted loopback bind; the configuration layer
// refuses to start an HTTP transport on a non-loopback address without a secret.
func RequireBearer(secret string) func(http.Handler) http.Handler {
	if secret == "" {
		return func(next http.Handler) http.Handler { return next }
	}
	want := sha256.Sum256([]byte(secret))
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !validBearer(r, want) {
				w.Header().Set("WWW-Authenticate", `Bearer realm="ocis-mcp-server"`)
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// validBearer reports whether the request carries an Authorization: Bearer header whose
// token matches want (a SHA-256 digest of the configured secret).
func validBearer(r *http.Request, want [32]byte) bool {
	const prefix = "Bearer "
	h := r.Header.Get("Authorization")
	if len(h) <= len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return false
	}
	got := sha256.Sum256([]byte(h[len(prefix):]))
	return subtle.ConstantTimeCompare(got[:], want[:]) == 1
}
