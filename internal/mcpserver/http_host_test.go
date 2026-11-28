package mcpserver

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestHTTPOIDCHostRejectionIsDebugLogged pins that a rejected Host is logged at debug
// level with a fixed message and the host value, and that an accepted one logs nothing.
func TestHTTPOIDCHostRejectionIsDebugLogged(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	s, _ := newTestServer(t, "")
	h, err := NewHTTPHandler(s, oidcAuth("https://mcp.example.test"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		host       string
		wantLogged bool
	}{{"mcp.example.test", false}, {"evil.example", true}} {
		buf.Reset()
		req := httptest.NewRequest(http.MethodGet, metadataPath, nil)
		req.Host = tt.host
		h.ServeHTTP(httptest.NewRecorder(), req)
		if logged := buf.Len() > 0; logged != tt.wantLogged {
			t.Fatalf("Host %q: logged = %v, want %v", tt.host, logged, tt.wantLogged)
		}
		if !tt.wantLogged {
			continue
		}
		var rec map[string]any
		if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
			t.Fatalf("log line is not JSON: %v", err)
		}
		if rec["level"] != "DEBUG" || rec["host"] != tt.host || rec["msg"] != "rejected request with a Host header outside the allowlist" {
			t.Errorf("unexpected log record for Host %q", tt.host)
		}
	}
}

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
		{"bare public host, non-default public port (proxy drops the port)", "https://mcp.example.test:8443", "mcp.example.test", http.StatusOK},
		{"public host with a different port than the public URL", "https://mcp.example.test:8443", "mcp.example.test:9999", http.StatusOK},
		{"public host with the default port, non-default public port", "https://mcp.example.test:8443", "mcp.example.test:443", http.StatusOK},
		{"public host on a plain http public URL, any port", "http://mcp.example.test:8080", "mcp.example.test:1", http.StatusOK},
		{"public ipv6 literal host", "https://[2001:db8::1]:8443", "[2001:db8::1]", http.StatusOK},
		{"localhost any port", "https://mcp.example.test", "localhost:1234", http.StatusOK},
		{"127.0.0.1 any port", "https://mcp.example.test", "127.0.0.1:1234", http.StatusOK},
		{"ipv6 loopback any port", "https://mcp.example.test", "[::1]:1234", http.StatusOK},
		{"foreign host", "https://mcp.example.test", "evil.example", http.StatusForbidden},
		{"foreign host with public prefix", "https://mcp.example.test", "mcp.example.test.evil.example", http.StatusForbidden},
		{"foreign host with default port", "https://mcp.example.test", "evil.example:443", http.StatusForbidden},
		{"foreign host on the public port", "https://mcp.example.test:8443", "evil.example:8443", http.StatusForbidden},
		{"public host as a subdomain prefix of a foreign host", "https://mcp.example.test:8443", "h.evil.example", http.StatusForbidden},
		{"foreign subdomain of the public host", "https://mcp.example.test", "x.mcp.example.test", http.StatusForbidden},
		{"foreign host that prefixes the public host", "https://mcp.example.test", "evil.mcp.example.test.", http.StatusForbidden},
		{"trailing dot on the public host", "https://mcp.example.test", "mcp.example.test.", http.StatusForbidden},
		{"trailing dot on the public host with port", "https://mcp.example.test", "mcp.example.test.:443", http.StatusForbidden},
		{"userinfo form", "https://mcp.example.test", "mcp.example.test@evil.example", http.StatusForbidden},
		{"userinfo form with the public host last", "https://mcp.example.test", "evil.example@mcp.example.test", http.StatusForbidden},
		{"non numeric port", "https://mcp.example.test", "mcp.example.test:abc", http.StatusForbidden},
		{"signed port", "https://mcp.example.test", "mcp.example.test:+80", http.StatusForbidden},
		{"port out of range", "https://mcp.example.test", "mcp.example.test:65536", http.StatusForbidden},
		{"empty port", "https://mcp.example.test", "mcp.example.test:", http.StatusForbidden},
		{"bracketed non-ip name", "https://mcp.example.test", "[mcp.example.test]", http.StatusForbidden},
		{"other loopback address is not allowlisted", "https://mcp.example.test", "127.0.0.2:80", http.StatusForbidden},
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
			status, mcpHdr, _ := rawPost(t, srv.URL+"/mcp", hdr, callBody)
			if status != tt.want {
				t.Errorf("/mcp status = %d, want %d", status, tt.want)
			}
			if tt.want == http.StatusForbidden && mcpHdr.Get("X-Frame-Options") != "DENY" {
				t.Errorf("/mcp 403 lacks X-Frame-Options: DENY (got %q)", mcpHdr.Get("X-Frame-Options"))
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
			if got := resp.Header.Get("X-Frame-Options"); got != "DENY" {
				t.Errorf("metadata response X-Frame-Options = %q, want DENY", got)
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
