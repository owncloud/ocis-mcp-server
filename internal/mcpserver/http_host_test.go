package mcpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestHTTPOIDCHostAllowlist sends requests over a loopback connection, as a reverse proxy
// that keeps the public Host does. The SDK guard is disabled in oidc mode, so the
// allowlist is the only Host check.
func TestHTTPOIDCHostAllowlist(t *testing.T) {
	tests := []struct {
		name   string
		public string
		host   string
		want   int
	}{
		{"public host", "https://mcp.example.test", "mcp.example.test", http.StatusOK},
		{"public host mixed case", "https://mcp.example.test", "MCP.Example.Test", http.StatusOK},
		{"public host with default port", "https://mcp.example.test", "mcp.example.test:443", http.StatusOK},
		{"public host, default port configured", "https://mcp.example.test:443", "mcp.example.test", http.StatusOK},
		{"public host, custom port configured", "https://mcp.example.test:8443", "mcp.example.test:8443", http.StatusOK},
		{"public host, wrong custom port", "https://mcp.example.test:8443", "mcp.example.test:9999", http.StatusForbidden},
		{"localhost any port", "https://mcp.example.test", "localhost:1234", http.StatusOK},
		{"127.0.0.1 any port", "https://mcp.example.test", "127.0.0.1:1234", http.StatusOK},
		{"ipv6 loopback any port", "https://mcp.example.test", "[::1]:1234", http.StatusOK},
		{"foreign host", "https://mcp.example.test", "evil.example", http.StatusForbidden},
		{"foreign host with public prefix", "https://mcp.example.test", "mcp.example.test.evil.example", http.StatusForbidden},
		{"foreign host with default port", "https://mcp.example.test", "evil.example:443", http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, fake := newTestServer(t, "")
			h, err := NewHTTPHandler(s, oidcAuth(tt.public))
			if err != nil {
				t.Fatal(err)
			}
			srv := httptest.NewServer(h)
			t.Cleanup(srv.Close)
			hdr := headers2026("tools/call", "ocis_get_version")
			hdr["Authorization"] = "Bearer " + validToken
			hdr["Host"] = tt.host
			status, _, _ := rawPost(t, srv.URL+"/mcp", hdr, callBody)
			if status != tt.want {
				t.Errorf("/mcp status = %d, want %d", status, tt.want)
			}
			if tt.want == http.StatusForbidden && fake.Hits() != 0 {
				t.Errorf("fake oCIS saw %d requests, want 0", fake.Hits())
			}

			req, err := http.NewRequest(http.MethodGet, srv.URL+metadataPath, nil)
			if err != nil {
				t.Fatal(err)
			}
			req.Host = tt.host
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			_ = resp.Body.Close()
			if resp.StatusCode != tt.want {
				t.Errorf("metadata status = %d, want %d", resp.StatusCode, tt.want)
			}
			if resp.Header.Get("X-Content-Type-Options") != "nosniff" {
				t.Error("response lacks security headers")
			}
		})
	}
}

// TestHTTPNonOIDCKeepsSDKHostGuard pins that none and secret modes keep the SDK default.
func TestHTTPNonOIDCKeepsSDKHostGuard(t *testing.T) {
	for _, mode := range []HTTPAuth{{Mode: AuthModeNone}, {Mode: AuthModeSecret, Secret: secretVal}} {
		t.Run(mode.Mode, func(t *testing.T) {
			s, _ := newTestServer(t, "")
			h, err := NewHTTPHandler(s, mode)
			if err != nil {
				t.Fatal(err)
			}
			srv := httptest.NewServer(h)
			t.Cleanup(srv.Close)
			hdr := headers2026("tools/call", "ocis_get_version")
			hdr["Authorization"] = "Bearer " + secretVal
			hdr["Host"] = "evil.example"
			status, _, _ := rawPost(t, srv.URL+"/mcp", hdr, callBody)
			if status != http.StatusForbidden {
				t.Errorf("status = %d, want 403 from the SDK guard", status)
			}
		})
	}
}
