package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/owncloud/ocis-mcp-server/internal/client"
)

func TestHandleHealthCheck(t *testing.T) {
	tests := []struct {
		name        string
		statusCode  int
		body        string
		wantHealthy bool
		wantErr     bool
	}{
		{
			name:        "healthy server returns 200",
			statusCode:  200,
			body:        `{"issuer":"https://ocis.example.org"}`,
			wantHealthy: true,
		},
		{
			name:        "server returns 204 is healthy",
			statusCode:  204,
			body:        "",
			wantHealthy: true,
		},
		{
			name:        "server returns 500 is unhealthy",
			statusCode:  500,
			body:        "",
			wantHealthy: false,
		},
		{
			name:        "server returns 503 is unhealthy",
			statusCode:  503,
			body:        "",
			wantHealthy: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				if tt.body != "" {
					w.Write([]byte(tt.body))
				}
			}))
			defer srv.Close()

			cfg := newTestConfig(srv.URL)
			c := client.New(cfg)

			handler := handleHealthCheck(c)
			_, output, err := handler(context.Background(), &mcp.CallToolRequest{}, HealthCheckInput{})

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if output.Healthy != tt.wantHealthy {
				t.Errorf("Healthy = %v, want %v", output.Healthy, tt.wantHealthy)
			}
			if output.Status != tt.statusCode {
				t.Errorf("Status = %d, want %d", output.Status, tt.statusCode)
			}
		})
	}
}

func TestHandleGetCapabilities(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantVer    string
		wantErr    bool
	}{
		{
			name:       "success returns capabilities",
			statusCode: 200,
			body: `{
				"ocs": {
					"data": {
						"version": {
							"major": 6,
							"minor": 0,
							"micro": 0,
							"string": "6.0.0",
							"edition": "Community"
						},
						"capabilities": {
							"core": {"status": {"installed": true}},
							"files_sharing": {"api_enabled": true}
						}
					}
				}
			}`,
			wantVer: "6.0.0",
		},
		{
			name:       "server error",
			statusCode: 500,
			body:       `{}`,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			cfg := newTestConfig(srv.URL)
			c := client.New(cfg)

			handler := handleGetCapabilities(c)
			_, output, err := handler(context.Background(), &mcp.CallToolRequest{}, GetCapabilitiesInput{})

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if output.Version.String != tt.wantVer {
				t.Errorf("Version.String = %q, want %q", output.Version.String, tt.wantVer)
			}
			if output.Capabilities == nil {
				t.Error("Capabilities should not be nil")
			}
		})
	}
}

func TestHandleGetVersion(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantMajor  int
		wantErr    bool
	}{
		{
			name:       "success returns version",
			statusCode: 200,
			body: `{
				"ocs": {
					"data": {
						"version": {
							"major": 6,
							"minor": 1,
							"micro": 2,
							"string": "6.1.2",
							"edition": "Enterprise"
						},
						"capabilities": {}
					}
				}
			}`,
			wantMajor: 6,
		},
		{
			name:       "404 returns error",
			statusCode: 404,
			body:       `{}`,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			cfg := newTestConfig(srv.URL)
			c := client.New(cfg)

			handler := handleGetVersion(c)
			_, output, err := handler(context.Background(), &mcp.CallToolRequest{}, GetVersionInput{})

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if output.Version.Major != tt.wantMajor {
				t.Errorf("Version.Major = %d, want %d", output.Version.Major, tt.wantMajor)
			}
		})
	}
}

func TestHandleGetConfig(t *testing.T) {
	tests := []struct {
		name         string
		authMode     string
		wantAuthMode string
		wantURL      string
	}{
		{
			name:         "app-token config",
			authMode:     "app-token",
			wantAuthMode: "app-token",
		},
		{
			name:         "oidc config",
			authMode:     "oidc",
			wantAuthMode: "oidc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// GetConfig does not make HTTP calls, but we still need a server for the client.
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			cfg := newTestConfig(srv.URL)
			cfg.AuthMode = tt.authMode
			c := client.New(cfg)

			handler := handleGetConfig(c)
			_, output, err := handler(context.Background(), &mcp.CallToolRequest{}, GetConfigInput{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if output.AuthMode != tt.wantAuthMode {
				t.Errorf("AuthMode = %q, want %q", output.AuthMode, tt.wantAuthMode)
			}
			if output.OcisURL == "" {
				t.Error("OcisURL should not be empty")
			}
			if output.Transport != "stdio" {
				t.Errorf("Transport = %q, want stdio", output.Transport)
			}
		})
	}
}
