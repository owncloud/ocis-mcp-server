package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/owncloud/ocis-mcp-server/internal/config"
)

// newTestConfig returns a config pointing at the given httptest server URL.
func newTestConfig(srvURL string) *config.Config {
	return &config.Config{
		OcisURL:       srvURL,
		AuthMode:      "app-token",
		AppTokenUser:  "admin",
		AppTokenValue: "test-token",
		Transport:     "stdio",
		HTTPAddr:      "127.0.0.1:8090",
		LogLevel:      "info",
		HTTPTimeout:   30 * time.Second,
		Insecure:      true, // httptest uses http://
	}
}

func TestAuthHeaderInjection(t *testing.T) {
	tests := []struct {
		name       string
		authMode   string
		user       string
		tokenValue string
		oidcToken  string
		wantPrefix string
	}{
		{
			name:       "app-token sends Basic auth",
			authMode:   "app-token",
			user:       "admin",
			tokenValue: "secret-token",
			wantPrefix: "Basic ",
		},
		{
			name:       "oidc sends Bearer token",
			authMode:   "oidc",
			oidcToken:  "my-access-token",
			wantPrefix: "Bearer my-access-token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotAuth string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotAuth = r.Header.Get("Authorization")
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			cfg := newTestConfig(srv.URL)
			cfg.AuthMode = tt.authMode
			cfg.AppTokenUser = tt.user
			cfg.AppTokenValue = tt.tokenValue
			cfg.OidcAccessToken = tt.oidcToken
			c := New(cfg)

			req, err := c.NewRequest(context.Background(), http.MethodGet, "/test", nil)
			if err != nil {
				t.Fatalf("NewRequest failed: %v", err)
			}
			resp, err := c.Do(req)
			if err != nil {
				t.Fatalf("Do failed: %v", err)
			}
			resp.Body.Close()

			if !strings.HasPrefix(gotAuth, tt.wantPrefix) {
				t.Errorf("Authorization header = %q, want prefix %q", gotAuth, tt.wantPrefix)
			}
		})
	}
}

func TestErrorMapping(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantMsg    string
	}{
		{
			name:       "401 maps to auth error",
			statusCode: 401,
			wantMsg:    "authentication failed",
		},
		{
			name:       "403 maps to permissions error",
			statusCode: 403,
			wantMsg:    "insufficient permissions",
		},
		{
			name:       "404 maps to not found",
			statusCode: 404,
			wantMsg:    "not found",
		},
		{
			name:       "409 maps to conflict",
			statusCode: 409,
			wantMsg:    "conflict",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(`{}`))
			}))
			defer srv.Close()

			cfg := newTestConfig(srv.URL)
			c := New(cfg)

			req, err := c.NewRequest(context.Background(), http.MethodGet, "/test", nil)
			if err != nil {
				t.Fatalf("NewRequest failed: %v", err)
			}
			_, err = c.DoJSON(req)
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			apiErr, ok := err.(*APIError)
			if !ok {
				t.Fatalf("expected *APIError, got %T: %v", err, err)
			}
			if apiErr.StatusCode != tt.statusCode {
				t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, tt.statusCode)
			}
			if !strings.Contains(apiErr.Message, tt.wantMsg) {
				t.Errorf("Message = %q, want it to contain %q", apiErr.Message, tt.wantMsg)
			}
		})
	}
}

func TestValidatePath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{name: "valid path", path: "/dav/spaces/abc/file.txt", wantErr: false},
		{name: "path with dot-dot", path: "/dav/spaces/../etc/passwd", wantErr: true},
		{name: "null byte", path: "/dav/spaces/abc\x00evil", wantErr: true},
		{name: "root path", path: "/", wantErr: false},
		{name: "single dot is ok", path: "/dav/spaces/./file.txt", wantErr: false},
		{name: "dot-dot in middle", path: "foo/../bar", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}

func TestValidateID(t *testing.T) {
	tests := []struct {
		name    string
		idName  string
		value   string
		wantErr bool
	}{
		{name: "valid ID", idName: "user_id", value: "abc-123", wantErr: false},
		{name: "empty string", idName: "user_id", value: "", wantErr: true},
		{name: "whitespace only", idName: "user_id", value: "   ", wantErr: true},
		{name: "single char", idName: "space_id", value: "x", wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateID(tt.idName, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateID(%q, %q) error = %v, wantErr %v", tt.idName, tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidateLimit(t *testing.T) {
	tests := []struct {
		name  string
		input int
		want  int
	}{
		{name: "zero clamps to default", input: 0, want: 50},
		{name: "negative clamps to default", input: -10, want: 50},
		{name: "one stays at one", input: 1, want: 1},
		{name: "100 passes through", input: 100, want: 100},
		{name: "200 stays at 200", input: 200, want: 200},
		{name: "201 clamps to 200", input: 201, want: 200},
		{name: "999 clamps to 200", input: 999, want: 200},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateLimit(tt.input)
			if got != tt.want {
				t.Errorf("ValidateLimit(%d) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestRateLimitRetry(t *testing.T) {
	tests := []struct {
		name           string
		retryAfter     string
		failCount      int // how many 429s before success
		wantSuccess    bool
		wantStatusCode int
	}{
		{
			name:           "retry succeeds after one 429",
			retryAfter:     "0",
			failCount:      1,
			wantSuccess:    true,
			wantStatusCode: 200,
		},
		{
			name:           "retry succeeds after two 429s",
			retryAfter:     "0",
			failCount:      2,
			wantSuccess:    true,
			wantStatusCode: 200,
		},
		{
			name:        "exhausted after three 429s",
			retryAfter:  "0",
			failCount:   3,
			wantSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attempt := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if attempt < tt.failCount {
					attempt++
					w.Header().Set("Retry-After", tt.retryAfter)
					w.WriteHeader(http.StatusTooManyRequests)
					return
				}
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"ok":true}`))
			}))
			defer srv.Close()

			cfg := newTestConfig(srv.URL)
			c := New(cfg)

			req, err := c.NewRequest(context.Background(), http.MethodGet, "/test", nil)
			if err != nil {
				t.Fatalf("NewRequest failed: %v", err)
			}
			resp, err := c.Do(req)

			if tt.wantSuccess {
				if err != nil {
					t.Fatalf("expected success, got error: %v", err)
				}
				resp.Body.Close()
				if resp.StatusCode != tt.wantStatusCode {
					t.Errorf("StatusCode = %d, want %d", resp.StatusCode, tt.wantStatusCode)
				}
			} else {
				if err == nil {
					resp.Body.Close()
					t.Fatal("expected error after exhausted retries, got nil")
				}
				apiErr, ok := err.(*APIError)
				if !ok {
					t.Fatalf("expected *APIError, got %T: %v", err, err)
				}
				if apiErr.StatusCode != 429 {
					t.Errorf("StatusCode = %d, want 429", apiErr.StatusCode)
				}
			}
		})
	}
}

func TestParseRetryAfter(t *testing.T) {
	tests := []struct {
		name string
		val  string
		want time.Duration
	}{
		{name: "empty defaults to 2s", val: "", want: 2 * time.Second},
		{name: "numeric seconds", val: "5", want: 5 * time.Second},
		{name: "zero seconds", val: "0", want: 0},
		{name: "invalid defaults to 2s", val: "not-a-number", want: 2 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseRetryAfter(tt.val)
			if got != tt.want {
				t.Errorf("parseRetryAfter(%q) = %v, want %v", tt.val, got, tt.want)
			}
		})
	}
}

func TestBaseURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := New(newTestConfig(srv.URL))
	if got := c.BaseURL(); got != srv.URL {
		t.Errorf("BaseURL() = %q, want %q", got, srv.URL)
	}
}

func TestConfig(t *testing.T) {
	cfg := newTestConfig("http://localhost")
	c := New(cfg)
	if got := c.Config(); got != cfg {
		t.Error("Config() did not return the same config pointer")
	}
}

func TestDoJSONSetsAcceptHeader(t *testing.T) {
	var gotAccept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAccept = r.Header.Get("Accept")
		w.WriteHeader(200)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := New(newTestConfig(srv.URL))
	req, _ := c.NewRequest(context.Background(), http.MethodGet, "/test", nil)
	resp, err := c.DoJSON(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()
	if gotAccept != "application/json" {
		t.Errorf("Accept = %q, want application/json", gotAccept)
	}
}

func TestDoJSONPreservesExistingAcceptHeader(t *testing.T) {
	var gotAccept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAccept = r.Header.Get("Accept")
		w.WriteHeader(200)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := New(newTestConfig(srv.URL))
	req, _ := c.NewRequest(context.Background(), http.MethodGet, "/test", nil)
	req.Header.Set("Accept", "text/xml")
	resp, err := c.DoJSON(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()
	if gotAccept != "text/xml" {
		t.Errorf("Accept = %q, want text/xml", gotAccept)
	}
}
