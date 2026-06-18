package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
}

func TestRequireBearer_NoSecretIsPassthrough(t *testing.T) {
	h := RequireBearer("")(okHandler())
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/mcp", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("passthrough: want 200, got %d", rr.Code)
	}
}

func TestRequireBearer_ValidToken(t *testing.T) {
	h := RequireBearer("s3cr3t")(okHandler())
	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set("Authorization", "Bearer s3cr3t")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("valid token: want 200, got %d", rr.Code)
	}
}

func TestRequireBearer_CaseInsensitiveScheme(t *testing.T) {
	h := RequireBearer("s3cr3t")(okHandler())
	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set("Authorization", "bearer s3cr3t")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("lowercase scheme: want 200, got %d", rr.Code)
	}
}

func TestRequireBearer_Rejects(t *testing.T) {
	cases := map[string]string{
		"missing header":   "",
		"wrong token":      "Bearer nope",
		"empty token":      "Bearer ",
		"no scheme":        "s3cr3t",
		"basic scheme":     "Basic s3cr3t",
		"prefix of secret": "Bearer s3cr3", // different length must still fail closed
		"superset":         "Bearer s3cr3tx",
	}
	for name, auth := range cases {
		t.Run(name, func(t *testing.T) {
			h := RequireBearer("s3cr3t")(okHandler())
			req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
			if auth != "" {
				req.Header.Set("Authorization", auth)
			}
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)
			if rr.Code != http.StatusUnauthorized {
				t.Fatalf("%s: want 401, got %d", name, rr.Code)
			}
			if rr.Header().Get("WWW-Authenticate") == "" {
				t.Errorf("%s: missing WWW-Authenticate header", name)
			}
		})
	}
}

func TestSecurityHeaders(t *testing.T) {
	h := SecurityHeaders(okHandler())
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/mcp", nil))
	if got := rr.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
	}
	if got := rr.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Errorf("X-Frame-Options = %q, want DENY", got)
	}
}
