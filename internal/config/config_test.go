package config

import (
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func clearEnv() {
	for _, k := range []string{
		"OCIS_MCP_OCIS_URL", "OCIS_MCP_AUTH_MODE",
		"OCIS_MCP_OCIS_AUTH_MODE", "OCIS_MCP_OCIS_APP_TOKEN_USER",
		"OCIS_MCP_OCIS_APP_TOKEN_VALUE", "OCIS_MCP_OCIS_BEARER_TOKEN",
		"OCIS_MCP_APP_TOKEN_USER", "OCIS_MCP_APP_TOKEN_VALUE",
		"OCIS_MCP_OIDC_ISSUER", "OCIS_MCP_OIDC_CLIENT_ID", "OCIS_MCP_OIDC_CLIENT_SECRET",
		"OCIS_MCP_OIDC_ACCESS_TOKEN", "OCIS_MCP_EDUCATION_ACCESS_TOKEN",
		"OCIS_MCP_TRANSPORT", "OCIS_MCP_HTTP_ADDR", "OCIS_MCP_HTTP_SECRET", "OCIS_MCP_LOG_LEVEL",
		"OCIS_MCP_INSECURE", "OCIS_MCP_TLS_SKIP_VERIFY", "OCIS_MCP_HTTP_TIMEOUT",
		"OCIS_MCP_HTTP_AUTH", "OCIS_MCP_HTTP_PUBLIC_URL", "OCIS_MCP_HTTP_OIDC_ISSUER",
		"OCIS_MCP_HTTP_OIDC_SCOPES", "OCIS_MCP_HTTP_OIDC_CA_FILE",
	} {
		_ = os.Unsetenv(k)
	}
}

func TestLoadMinimalConfig(t *testing.T) {
	clearEnv()
	_ = os.Setenv("OCIS_MCP_OCIS_URL", "https://ocis.example.com")
	_ = os.Setenv("OCIS_MCP_OCIS_APP_TOKEN_USER", "admin")
	_ = os.Setenv("OCIS_MCP_OCIS_APP_TOKEN_VALUE", "tok")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.AuthMode != "app-token" {
		t.Errorf("AuthMode = %q, want app-token", cfg.AuthMode)
	}
	if cfg.Transport != "stdio" {
		t.Errorf("Transport = %q, want stdio", cfg.Transport)
	}
	if cfg.HTTPAddr != "127.0.0.1:8090" {
		t.Errorf("HTTPAddr = %q, want 127.0.0.1:8090", cfg.HTTPAddr)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want info", cfg.LogLevel)
	}
	if cfg.HTTPTimeout != 30*time.Second {
		t.Errorf("HTTPTimeout = %v, want 30s", cfg.HTTPTimeout)
	}
}

func TestLoadMissingURL(t *testing.T) {
	clearEnv()
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing URL")
	}
}

func TestLoadHTTPWithoutInsecure(t *testing.T) {
	clearEnv()
	_ = os.Setenv("OCIS_MCP_OCIS_URL", "http://insecure.example.com")
	_ = os.Setenv("OCIS_MCP_OCIS_APP_TOKEN_USER", "admin")
	_ = os.Setenv("OCIS_MCP_OCIS_APP_TOKEN_VALUE", "tok")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for HTTP without INSECURE")
	}
}

func TestLoadHTTPWithInsecure(t *testing.T) {
	clearEnv()
	_ = os.Setenv("OCIS_MCP_OCIS_URL", "http://insecure.example.com")
	_ = os.Setenv("OCIS_MCP_OCIS_APP_TOKEN_USER", "admin")
	_ = os.Setenv("OCIS_MCP_OCIS_APP_TOKEN_VALUE", "tok")
	_ = os.Setenv("OCIS_MCP_INSECURE", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if !cfg.Insecure {
		t.Error("expected Insecure=true")
	}
}

func TestLoadInvalidTransport(t *testing.T) {
	clearEnv()
	_ = os.Setenv("OCIS_MCP_OCIS_URL", "https://ocis.example.com")
	_ = os.Setenv("OCIS_MCP_OCIS_APP_TOKEN_USER", "admin")
	_ = os.Setenv("OCIS_MCP_OCIS_APP_TOKEN_VALUE", "tok")
	_ = os.Setenv("OCIS_MCP_TRANSPORT", "grpc")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid transport")
	}
}

func TestLoadAmbiguousAuth(t *testing.T) {
	clearEnv()
	_ = os.Setenv("OCIS_MCP_OCIS_URL", "https://ocis.example.com")
	_ = os.Setenv("OCIS_MCP_OCIS_APP_TOKEN_USER", "admin")
	_ = os.Setenv("OCIS_MCP_OCIS_APP_TOKEN_VALUE", "tok")
	_ = os.Setenv("OCIS_MCP_OCIS_BEARER_TOKEN", "bearer-token")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for ambiguous auth")
	}
}

func TestLoadExplicitAuthMode(t *testing.T) {
	clearEnv()
	_ = os.Setenv("OCIS_MCP_OCIS_URL", "https://ocis.example.com")
	_ = os.Setenv("OCIS_MCP_OCIS_APP_TOKEN_USER", "admin")
	_ = os.Setenv("OCIS_MCP_OCIS_APP_TOKEN_VALUE", "tok")
	_ = os.Setenv("OCIS_MCP_OCIS_BEARER_TOKEN", "bearer-token")
	_ = os.Setenv("OCIS_MCP_OCIS_AUTH_MODE", "static-bearer")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.AuthMode != "static-bearer" {
		t.Errorf("AuthMode = %q, want static-bearer", cfg.AuthMode)
	}
}

func TestLoadBearerAutoDetect(t *testing.T) {
	clearEnv()
	_ = os.Setenv("OCIS_MCP_OCIS_URL", "https://ocis.example.com")
	_ = os.Setenv("OCIS_MCP_OCIS_BEARER_TOKEN", "bearer-token")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.AuthMode != "static-bearer" {
		t.Errorf("AuthMode = %q, want static-bearer", cfg.AuthMode)
	}
}

func TestLoadNoAuth(t *testing.T) {
	clearEnv()
	_ = os.Setenv("OCIS_MCP_OCIS_URL", "https://ocis.example.com")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.AuthMode != "none" {
		t.Errorf("AuthMode = %q, want none", cfg.AuthMode)
	}
}

func TestLoadCustomTimeout(t *testing.T) {
	clearEnv()
	_ = os.Setenv("OCIS_MCP_OCIS_URL", "https://ocis.example.com")
	_ = os.Setenv("OCIS_MCP_HTTP_TIMEOUT", "60s")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.HTTPTimeout != 60*time.Second {
		t.Errorf("HTTPTimeout = %v, want 60s", cfg.HTTPTimeout)
	}
}

func TestLoadInvalidTimeout(t *testing.T) {
	clearEnv()
	_ = os.Setenv("OCIS_MCP_OCIS_URL", "https://ocis.example.com")
	_ = os.Setenv("OCIS_MCP_HTTP_TIMEOUT", "not-a-duration")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid timeout")
	}
}

func TestLoadInvalidURLScheme(t *testing.T) {
	clearEnv()
	_ = os.Setenv("OCIS_MCP_OCIS_URL", "ftp://ocis.example.com")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for ftp scheme")
	}
}

func TestOcisBaseURL(t *testing.T) {
	cfg := &Config{OcisURL: "https://ocis.example.com/"}
	if got := cfg.OcisBaseURL(); got != "https://ocis.example.com" {
		t.Errorf("OcisBaseURL() = %q, want without trailing slash", got)
	}
}

func TestNewHTTPClient(t *testing.T) {
	cfg := &Config{
		HTTPTimeout:   10 * time.Second,
		TLSSkipVerify: true,
	}
	c := cfg.NewHTTPClient()
	if c.Timeout != 10*time.Second {
		t.Errorf("Timeout = %v, want 10s", c.Timeout)
	}
}

func TestSelectAuthMode(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want string
	}{
		{
			name: "explicit overrides",
			cfg:  Config{AuthMode: "custom"},
			want: "custom",
		},
		{
			name: "infer app-token",
			cfg:  Config{AppTokenUser: "u", AppTokenValue: "v"},
			want: "app-token",
		},
		{
			name: "infer static-bearer",
			cfg:  Config{BearerToken: "tok"},
			want: "static-bearer",
		},
		{
			name: "none when empty",
			cfg:  Config{},
			want: "none",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := selectAuthMode(&tt.cfg)
			if got != tt.want {
				t.Errorf("selectAuthMode() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEnvBool(t *testing.T) {
	tests := []struct {
		val  string
		want bool
	}{
		{"true", true},
		{"false", false},
		{"1", true},
		{"0", false},
		{"", false},
		{"invalid", false},
	}
	for _, tt := range tests {
		t.Run(tt.val, func(t *testing.T) {
			t.Setenv("TEST_BOOL", tt.val)
			if got := envBool("TEST_BOOL"); got != tt.want {
				t.Errorf("envBool(%q) = %v, want %v", tt.val, got, tt.want)
			}
		})
	}
}

func TestLoadHTTPTransport(t *testing.T) {
	clearEnv()
	_ = os.Setenv("OCIS_MCP_OCIS_URL", "https://ocis.example.com")
	_ = os.Setenv("OCIS_MCP_TRANSPORT", "http")
	_ = os.Setenv("OCIS_MCP_HTTP_ADDR", "0.0.0.0:9090")
	// A non-loopback HTTP bind requires an authentication secret.
	_ = os.Setenv("OCIS_MCP_HTTP_SECRET", "test-secret")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Transport != "http" {
		t.Errorf("Transport = %q, want http", cfg.Transport)
	}
	if cfg.HTTPAddr != "0.0.0.0:9090" {
		t.Errorf("HTTPAddr = %q, want 0.0.0.0:9090", cfg.HTTPAddr)
	}
	if !cfg.HTTPAuthEnabled() {
		t.Error("HTTPAuthEnabled() = false, want true")
	}
}

func TestLoadHTTP_NonLoopbackWithoutSecretFails(t *testing.T) {
	addrs := []string{"0.0.0.0:9090", ":9090", "192.0.2.10:9090"}
	for _, addr := range addrs {
		t.Run(addr, func(t *testing.T) {
			clearEnv()
			_ = os.Setenv("OCIS_MCP_OCIS_URL", "https://ocis.example.com")
			_ = os.Setenv("OCIS_MCP_TRANSPORT", "http")
			_ = os.Setenv("OCIS_MCP_HTTP_ADDR", addr)
			if _, err := Load(); err == nil {
				t.Fatalf("expected error for non-loopback http bind %q without OCIS_MCP_HTTP_SECRET", addr)
			}
		})
	}
}

func TestLoadHTTP_LoopbackWithoutSecretAllowed(t *testing.T) {
	for _, addr := range []string{"127.0.0.1:8090", "localhost:8090", "[::1]:8090"} {
		t.Run(addr, func(t *testing.T) {
			clearEnv()
			_ = os.Setenv("OCIS_MCP_OCIS_URL", "https://ocis.example.com")
			_ = os.Setenv("OCIS_MCP_TRANSPORT", "http")
			_ = os.Setenv("OCIS_MCP_HTTP_ADDR", addr)
			cfg, err := Load()
			if err != nil {
				t.Fatalf("loopback bind %q without secret should be allowed, got: %v", addr, err)
			}
			if !cfg.IsLoopbackBind() {
				t.Errorf("IsLoopbackBind() = false for %q, want true", addr)
			}
			if cfg.HTTPAuthEnabled() {
				t.Error("HTTPAuthEnabled() = true, want false (no secret set)")
			}
		})
	}
}

func TestLoadHTTPAuthModes(t *testing.T) {
	const secretVal = "s3cr3t-value-do-not-leak"
	const issuer = "https://idp.example.com/realms/ocis"
	const pub = "https://mcp.example.com"

	tests := []struct {
		name        string
		env         map[string]string
		wantErr     string
		wantMode    string
		wantScopes  []string
		wantWarnSub []string
		wantEnabled bool
		wantNoSec   bool // HTTPSecret must be empty after Load
		wantWarns   int  // expected warning count, -1 to skip
	}{
		{name: "inferred oidc from issuer", env: map[string]string{
			"OCIS_MCP_HTTP_OIDC_ISSUER": issuer, "OCIS_MCP_HTTP_PUBLIC_URL": pub,
			"OCIS_MCP_HTTP_ADDR": "0.0.0.0:8090"},
			wantMode: "oidc", wantScopes: []string{"ocis:mcp"}, wantEnabled: true},
		{name: "inferred secret", env: map[string]string{
			"OCIS_MCP_HTTP_SECRET": secretVal, "OCIS_MCP_HTTP_ADDR": "0.0.0.0:8090"},
			wantMode: "secret", wantEnabled: true, wantWarns: 1,
			wantWarnSub: []string{"deprecated"}},
		{name: "inferred none on loopback", env: map[string]string{},
			wantMode: "none"},
		{name: "issuer and secret without mode", env: map[string]string{
			"OCIS_MCP_HTTP_OIDC_ISSUER": issuer, "OCIS_MCP_HTTP_PUBLIC_URL": pub,
			"OCIS_MCP_HTTP_SECRET": secretVal},
			wantErr: "OCIS_MCP_HTTP_AUTH"},
		{name: "explicit oidc ignores secret and warns", env: map[string]string{
			"OCIS_MCP_HTTP_AUTH": "oidc", "OCIS_MCP_HTTP_OIDC_ISSUER": issuer,
			"OCIS_MCP_HTTP_PUBLIC_URL": pub, "OCIS_MCP_HTTP_SECRET": secretVal},
			wantMode: "oidc", wantScopes: []string{"ocis:mcp"}, wantEnabled: true,
			wantNoSec: true, wantWarns: 1, wantWarnSub: []string{"OCIS_MCP_HTTP_SECRET"}},
		{name: "explicit secret ignores issuer and warns", env: map[string]string{
			"OCIS_MCP_HTTP_AUTH": "secret", "OCIS_MCP_HTTP_OIDC_ISSUER": issuer,
			"OCIS_MCP_HTTP_SECRET": secretVal},
			wantMode: "secret", wantEnabled: true, wantWarns: 2,
			wantWarnSub: []string{"OCIS_MCP_HTTP_OIDC_ISSUER", "deprecated"}},
		{name: "explicit secret with empty secret", env: map[string]string{
			"OCIS_MCP_HTTP_AUTH": "secret"},
			wantErr: "OCIS_MCP_HTTP_SECRET"},
		{name: "explicit none ignores secret and warns", env: map[string]string{
			"OCIS_MCP_HTTP_AUTH": "none", "OCIS_MCP_HTTP_SECRET": secretVal},
			wantMode: "none", wantNoSec: true, wantWarns: 1,
			wantWarnSub: []string{"OCIS_MCP_HTTP_SECRET"}},
		{name: "oidc without issuer", env: map[string]string{
			"OCIS_MCP_HTTP_AUTH": "oidc", "OCIS_MCP_HTTP_PUBLIC_URL": pub},
			wantErr: "OCIS_MCP_HTTP_OIDC_ISSUER"},
		{name: "oidc without public url", env: map[string]string{
			"OCIS_MCP_HTTP_AUTH": "oidc", "OCIS_MCP_HTTP_OIDC_ISSUER": issuer},
			wantErr: "OCIS_MCP_HTTP_PUBLIC_URL"},
		{name: "public url with path", env: map[string]string{
			"OCIS_MCP_HTTP_OIDC_ISSUER": issuer, "OCIS_MCP_HTTP_PUBLIC_URL": pub + "/mcp"},
			wantErr: "OCIS_MCP_HTTP_PUBLIC_URL"},
		{name: "public url with query", env: map[string]string{
			"OCIS_MCP_HTTP_OIDC_ISSUER": issuer, "OCIS_MCP_HTTP_PUBLIC_URL": pub + "?a=b"},
			wantErr: "OCIS_MCP_HTTP_PUBLIC_URL"},
		{name: "public url with fragment", env: map[string]string{
			"OCIS_MCP_HTTP_OIDC_ISSUER": issuer, "OCIS_MCP_HTTP_PUBLIC_URL": pub + "#f"},
			wantErr: "OCIS_MCP_HTTP_PUBLIC_URL"},
		{name: "public url http non-loopback", env: map[string]string{
			"OCIS_MCP_HTTP_OIDC_ISSUER": issuer, "OCIS_MCP_HTTP_PUBLIC_URL": "http://mcp.example.com"},
			wantErr: "OCIS_MCP_HTTP_PUBLIC_URL"},
		{name: "public url http loopback ok", env: map[string]string{
			"OCIS_MCP_HTTP_OIDC_ISSUER": issuer, "OCIS_MCP_HTTP_PUBLIC_URL": "http://127.0.0.1:8090"},
			wantMode: "oidc", wantScopes: []string{"ocis:mcp"}, wantEnabled: true},
		{name: "issuer http non-loopback", env: map[string]string{
			"OCIS_MCP_HTTP_OIDC_ISSUER": "http://idp.example.com", "OCIS_MCP_HTTP_PUBLIC_URL": pub},
			wantErr: "OCIS_MCP_HTTP_OIDC_ISSUER"},
		{name: "issuer http loopback ok", env: map[string]string{
			"OCIS_MCP_HTTP_OIDC_ISSUER": "http://localhost:8080/realms/x", "OCIS_MCP_HTTP_PUBLIC_URL": pub},
			wantMode: "oidc", wantScopes: []string{"ocis:mcp"}, wantEnabled: true},
		{name: "none on non-loopback bind", env: map[string]string{
			"OCIS_MCP_HTTP_AUTH": "none", "OCIS_MCP_HTTP_ADDR": "0.0.0.0:8090"},
			wantErr: "non-loopback"},
		{name: "unknown mode", env: map[string]string{
			"OCIS_MCP_HTTP_AUTH": "basic"},
			wantErr: "OCIS_MCP_HTTP_AUTH"},
		{name: "scopes comma separated", env: map[string]string{
			"OCIS_MCP_HTTP_OIDC_ISSUER": issuer, "OCIS_MCP_HTTP_PUBLIC_URL": pub,
			"OCIS_MCP_HTTP_OIDC_SCOPES": "a:b,c:d"},
			wantMode: "oidc", wantScopes: []string{"a:b", "c:d"}, wantEnabled: true},
		{name: "scopes space separated", env: map[string]string{
			"OCIS_MCP_HTTP_OIDC_ISSUER": issuer, "OCIS_MCP_HTTP_PUBLIC_URL": pub,
			"OCIS_MCP_HTTP_OIDC_SCOPES": "a:b  c:d, e:f"},
			wantMode: "oidc", wantScopes: []string{"a:b", "c:d", "e:f"}, wantEnabled: true},
		{name: "scopes duplicate entry kept as given", env: map[string]string{
			"OCIS_MCP_HTTP_OIDC_ISSUER": issuer, "OCIS_MCP_HTTP_PUBLIC_URL": pub,
			"OCIS_MCP_HTTP_OIDC_SCOPES": "a:b,a:b"},
			wantMode: "oidc", wantScopes: []string{"a:b", "a:b"}, wantEnabled: true},
		{name: "scopes empty entry dropped", env: map[string]string{
			"OCIS_MCP_HTTP_OIDC_ISSUER": issuer, "OCIS_MCP_HTTP_PUBLIC_URL": pub,
			"OCIS_MCP_HTTP_OIDC_SCOPES": "a,,b"},
			wantMode: "oidc", wantScopes: []string{"a", "b"}, wantEnabled: true},
		{name: "issuer with userinfo", env: map[string]string{
			"OCIS_MCP_HTTP_OIDC_ISSUER": "https://user:pw@idp.example.com", "OCIS_MCP_HTTP_PUBLIC_URL": pub},
			wantErr: "OCIS_MCP_HTTP_OIDC_ISSUER"},
		{name: "issuer with fragment", env: map[string]string{
			"OCIS_MCP_HTTP_OIDC_ISSUER": issuer + "#f", "OCIS_MCP_HTTP_PUBLIC_URL": pub},
			wantErr: "OCIS_MCP_HTTP_OIDC_ISSUER"},
		{name: "public url with userinfo", env: map[string]string{
			"OCIS_MCP_HTTP_OIDC_ISSUER": issuer, "OCIS_MCP_HTTP_PUBLIC_URL": "https://user@mcp.example.com"},
			wantErr: "OCIS_MCP_HTTP_PUBLIC_URL"},
		{name: "issuer http localhost.evil.com", env: map[string]string{
			"OCIS_MCP_HTTP_OIDC_ISSUER": "http://localhost.evil.com", "OCIS_MCP_HTTP_PUBLIC_URL": pub},
			wantErr: "OCIS_MCP_HTTP_OIDC_ISSUER"},
		{name: "issuer http localhost@evil.com", env: map[string]string{
			"OCIS_MCP_HTTP_OIDC_ISSUER": "http://localhost@evil.com", "OCIS_MCP_HTTP_PUBLIC_URL": pub},
			wantErr: "OCIS_MCP_HTTP_OIDC_ISSUER"},
		{name: "issuer http upper-case LOCALHOST", env: map[string]string{
			"OCIS_MCP_HTTP_OIDC_ISSUER": "http://LOCALHOST:8080", "OCIS_MCP_HTTP_PUBLIC_URL": pub},
			wantMode: "oidc", wantScopes: []string{"ocis:mcp"}, wantEnabled: true},
		{name: "issuer http ipv6 loopback with port", env: map[string]string{
			"OCIS_MCP_HTTP_OIDC_ISSUER": "http://[::1]:8080", "OCIS_MCP_HTTP_PUBLIC_URL": pub},
			wantMode: "oidc", wantScopes: []string{"ocis:mcp"}, wantEnabled: true},
		{name: "issuer http ipv6 loopback without port", env: map[string]string{
			"OCIS_MCP_HTTP_OIDC_ISSUER": "http://[::1]", "OCIS_MCP_HTTP_PUBLIC_URL": pub},
			wantMode: "oidc", wantScopes: []string{"ocis:mcp"}, wantEnabled: true},
		{name: "stdio ignores bad http settings", env: map[string]string{
			"OCIS_MCP_TRANSPORT": "stdio", "OCIS_MCP_HTTP_AUTH": "bogus",
			"OCIS_MCP_HTTP_OIDC_ISSUER": "http://idp.example.com",
			"OCIS_MCP_HTTP_PUBLIC_URL":  "not a url/path?x", "OCIS_MCP_HTTP_SECRET": secretVal},
			wantMode: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearEnv()
			_ = os.Setenv("OCIS_MCP_OCIS_URL", "https://ocis.example.com")
			if _, ok := tt.env["OCIS_MCP_TRANSPORT"]; !ok {
				_ = os.Setenv("OCIS_MCP_TRANSPORT", "http")
			}
			for k, v := range tt.env {
				_ = os.Setenv(k, v)
			}
			t.Cleanup(clearEnv)

			cfg, err := Load()
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.wantErr)
				}
				if strings.Contains(err.Error(), secretVal) {
					t.Error("error text contains the secret value")
				}
				return
			}
			if err != nil {
				t.Fatalf("Load failed: %v", err)
			}
			if cfg.HTTPAuth != tt.wantMode {
				t.Errorf("HTTPAuth = %q, want %q", cfg.HTTPAuth, tt.wantMode)
			}
			if cfg.HTTPAuthEnabled() != tt.wantEnabled {
				t.Errorf("HTTPAuthEnabled() = %v, want %v", cfg.HTTPAuthEnabled(), tt.wantEnabled)
			}
			if tt.wantScopes != nil && !reflect.DeepEqual(cfg.HTTPOIDCScopes, tt.wantScopes) {
				t.Errorf("HTTPOIDCScopes = %v, want %v", cfg.HTTPOIDCScopes, tt.wantScopes)
			}
			if tt.wantNoSec && cfg.HTTPSecret != "" {
				t.Error("HTTPSecret not cleared for a non-secret mode")
			}
			if len(cfg.Warnings) != tt.wantWarns {
				t.Errorf("got %d warnings %q, want %d", len(cfg.Warnings), cfg.Warnings, tt.wantWarns)
			}
			joined := strings.Join(cfg.Warnings, "\n")
			if strings.Contains(joined, secretVal) {
				t.Error("warning text contains the secret value")
			}
			for _, sub := range tt.wantWarnSub {
				if !strings.Contains(joined, sub) {
					t.Errorf("warnings %q do not mention %q", joined, sub)
				}
			}
		})
	}
}

func TestLoadHTTPOIDCSettingsCarried(t *testing.T) {
	clearEnv()
	t.Cleanup(clearEnv)
	_ = os.Setenv("OCIS_MCP_OCIS_URL", "https://ocis.example.com")
	_ = os.Setenv("OCIS_MCP_TRANSPORT", "http")
	_ = os.Setenv("OCIS_MCP_HTTP_OIDC_ISSUER", "https://idp.example.com")
	_ = os.Setenv("OCIS_MCP_HTTP_PUBLIC_URL", "https://mcp.example.com")
	_ = os.Setenv("OCIS_MCP_HTTP_OIDC_CA_FILE", "/etc/ssl/ca.pem")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.HTTPOIDCIssuer != "https://idp.example.com" || cfg.HTTPPublicURL != "https://mcp.example.com" ||
		cfg.HTTPOIDCCAFile != "/etc/ssl/ca.pem" {
		t.Errorf("settings not carried: %+v", cfg)
	}
	if got := cfg.MetadataURL(); got != "https://mcp.example.com/.well-known/oauth-protected-resource/mcp" {
		t.Errorf("MetadataURL() = %q", got)
	}
}

func TestResourceURI(t *testing.T) {
	tests := []struct {
		name string
		pub  string
		want string
	}{
		{"plain", "https://mcp.example.com", "https://mcp.example.com/mcp"},
		{"trailing slash", "https://mcp.example.com/", "https://mcp.example.com/mcp"},
		{"upper-case host and scheme", "HTTPS://MCP.Example.COM", "https://mcp.example.com/mcp"},
		{"default https port", "https://mcp.example.com:443", "https://mcp.example.com/mcp"},
		{"default http port", "http://localhost:80", "http://localhost/mcp"},
		{"non-default port kept", "https://mcp.example.com:8443/", "https://mcp.example.com:8443/mcp"},
		{"ipv6 loopback", "http://[::1]:8090", "http://[::1]:8090/mcp"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Config{HTTPPublicURL: tt.pub}
			if got := c.ResourceURI(); got != tt.want {
				t.Errorf("ResourceURI() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLoadOutboundAliases(t *testing.T) {
	const secret = "s3cr3t-DISTINCTIVE-9f8e7d"
	tests := []struct {
		name         string
		env          map[string]string
		wantMode     string
		wantUser     string
		wantValue    string
		wantBearer   string
		wantWarnSubs [][]string // one entry per expected warning; every substring must appear
	}{
		{name: "new names only, no warnings",
			env: map[string]string{
				"OCIS_MCP_OCIS_APP_TOKEN_USER": "u", "OCIS_MCP_OCIS_APP_TOKEN_VALUE": secret},
			wantMode: "app-token", wantUser: "u", wantValue: secret},
		{name: "old app token user alias",
			env: map[string]string{
				"OCIS_MCP_APP_TOKEN_USER": "u", "OCIS_MCP_OCIS_APP_TOKEN_VALUE": secret},
			wantMode: "app-token", wantUser: "u", wantValue: secret,
			wantWarnSubs: [][]string{{"OCIS_MCP_APP_TOKEN_USER", "OCIS_MCP_OCIS_APP_TOKEN_USER", "deprecated"}}},
		{name: "old app token value alias",
			env: map[string]string{
				"OCIS_MCP_OCIS_APP_TOKEN_USER": "u", "OCIS_MCP_APP_TOKEN_VALUE": secret},
			wantMode: "app-token", wantUser: "u", wantValue: secret,
			wantWarnSubs: [][]string{{"OCIS_MCP_APP_TOKEN_VALUE", "OCIS_MCP_OCIS_APP_TOKEN_VALUE", "deprecated"}}},
		{name: "old bearer alias",
			env:      map[string]string{"OCIS_MCP_OIDC_ACCESS_TOKEN": secret},
			wantMode: "static-bearer", wantBearer: secret,
			wantWarnSubs: [][]string{{"OCIS_MCP_OIDC_ACCESS_TOKEN", "OCIS_MCP_OCIS_BEARER_TOKEN", "deprecated"}}},
		{name: "old auth mode alias",
			env: map[string]string{
				"OCIS_MCP_AUTH_MODE":         "app-token",
				"OCIS_MCP_OCIS_BEARER_TOKEN": secret},
			wantMode: "app-token", wantBearer: secret,
			wantWarnSubs: [][]string{{"OCIS_MCP_AUTH_MODE", "OCIS_MCP_OCIS_AUTH_MODE", "deprecated"}}},
		{name: "legacy oidc mode maps to static-bearer",
			env: map[string]string{
				"OCIS_MCP_AUTH_MODE":         "oidc",
				"OCIS_MCP_OCIS_BEARER_TOKEN": secret},
			wantMode: "static-bearer", wantBearer: secret,
			wantWarnSubs: [][]string{{"OCIS_MCP_AUTH_MODE", "OCIS_MCP_OCIS_AUTH_MODE", "deprecated"}}},
		{name: "new wins over old and old still warns",
			env: map[string]string{
				"OCIS_MCP_OCIS_AUTH_MODE": "none", "OCIS_MCP_AUTH_MODE": "app-token",
				"OCIS_MCP_OCIS_APP_TOKEN_USER": "new-user", "OCIS_MCP_APP_TOKEN_USER": "old-user",
				"OCIS_MCP_OCIS_APP_TOKEN_VALUE": "new-val", "OCIS_MCP_APP_TOKEN_VALUE": "old-val",
				"OCIS_MCP_OCIS_BEARER_TOKEN": "new-bearer", "OCIS_MCP_OIDC_ACCESS_TOKEN": "old-bearer"},
			wantMode: "none", wantUser: "new-user", wantValue: "new-val", wantBearer: "new-bearer",
			wantWarnSubs: [][]string{
				{"OCIS_MCP_APP_TOKEN_USER", "OCIS_MCP_OCIS_APP_TOKEN_USER"},
				{"OCIS_MCP_APP_TOKEN_VALUE", "OCIS_MCP_OCIS_APP_TOKEN_VALUE"},
				{"OCIS_MCP_OIDC_ACCESS_TOKEN", "OCIS_MCP_OCIS_BEARER_TOKEN"},
				{"OCIS_MCP_AUTH_MODE", "OCIS_MCP_OCIS_AUTH_MODE"}}},
		{name: "dead issuer warns and points at the new name",
			env:          map[string]string{"OCIS_MCP_OIDC_ISSUER": "https://idp." + secret + ".example.com"},
			wantMode:     "none",
			wantWarnSubs: [][]string{{"OCIS_MCP_OIDC_ISSUER", "no effect", "OCIS_MCP_HTTP_OIDC_ISSUER"}}},
		{name: "dead client id warns",
			env:          map[string]string{"OCIS_MCP_OIDC_CLIENT_ID": "id-" + secret},
			wantMode:     "none",
			wantWarnSubs: [][]string{{"OCIS_MCP_OIDC_CLIENT_ID", "no effect"}}},
		{name: "dead client secret warns",
			env:          map[string]string{"OCIS_MCP_OIDC_CLIENT_SECRET": secret},
			wantMode:     "none",
			wantWarnSubs: [][]string{{"OCIS_MCP_OIDC_CLIENT_SECRET", "no effect"}}},
		{name: "all three dead variables warn once each",
			env: map[string]string{
				"OCIS_MCP_OIDC_ISSUER": "x", "OCIS_MCP_OIDC_CLIENT_ID": "y", "OCIS_MCP_OIDC_CLIENT_SECRET": secret},
			wantMode: "none",
			wantWarnSubs: [][]string{
				{"OCIS_MCP_OIDC_ISSUER", "no effect"},
				{"OCIS_MCP_OIDC_CLIENT_ID", "no effect"},
				{"OCIS_MCP_OIDC_CLIENT_SECRET", "no effect"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearEnv()
			t.Cleanup(clearEnv)
			t.Setenv("OCIS_MCP_OCIS_URL", "https://ocis.example.com")
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load failed: %v", err)
			}
			if cfg.AuthMode != tt.wantMode {
				t.Errorf("AuthMode = %q, want %q", cfg.AuthMode, tt.wantMode)
			}
			if cfg.AppTokenUser != tt.wantUser || cfg.AppTokenValue != tt.wantValue {
				t.Errorf("app token user/value mismatch")
			}
			if cfg.BearerToken != tt.wantBearer {
				t.Errorf("BearerToken mismatch")
			}
			if len(cfg.Warnings) != len(tt.wantWarnSubs) {
				t.Fatalf("got %d warnings %q, want %d", len(cfg.Warnings), cfg.Warnings, len(tt.wantWarnSubs))
			}
			for i, subs := range tt.wantWarnSubs {
				for _, sub := range subs {
					if !strings.Contains(cfg.Warnings[i], sub) {
						t.Errorf("warning %d = %q, missing %q", i, cfg.Warnings[i], sub)
					}
				}
			}
			// No warning may carry any configured value.
			for _, w := range cfg.Warnings {
				for _, v := range tt.env {
					if len(v) > 3 && strings.Contains(w, v) {
						t.Errorf("warning %q contains a setting value", w)
					}
				}
				if strings.Contains(w, secret) {
					t.Errorf("warning %q contains the secret", w)
				}
			}
		})
	}
}

func TestLoadAmbiguousAuthWithOldNames(t *testing.T) {
	clearEnv()
	t.Cleanup(clearEnv)
	t.Setenv("OCIS_MCP_OCIS_URL", "https://ocis.example.com")
	t.Setenv("OCIS_MCP_APP_TOKEN_USER", "admin")
	t.Setenv("OCIS_MCP_APP_TOKEN_VALUE", "tok")
	t.Setenv("OCIS_MCP_OIDC_ACCESS_TOKEN", "bearer-token")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for ambiguous auth")
	}
	if !strings.Contains(err.Error(), "OCIS_MCP_OCIS_AUTH_MODE") {
		t.Errorf("error %q should name OCIS_MCP_OCIS_AUTH_MODE", err)
	}
}
