package config

import (
	"os"
	"testing"
	"time"
)

func clearEnv() {
	for _, k := range []string{
		"OCIS_MCP_OCIS_URL", "OCIS_MCP_AUTH_MODE",
		"OCIS_MCP_APP_TOKEN_USER", "OCIS_MCP_APP_TOKEN_VALUE",
		"OCIS_MCP_OIDC_ISSUER", "OCIS_MCP_OIDC_CLIENT_ID", "OCIS_MCP_OIDC_CLIENT_SECRET",
		"OCIS_MCP_OIDC_ACCESS_TOKEN", "OCIS_MCP_EDUCATION_ACCESS_TOKEN",
		"OCIS_MCP_TRANSPORT", "OCIS_MCP_HTTP_ADDR", "OCIS_MCP_LOG_LEVEL",
		"OCIS_MCP_INSECURE", "OCIS_MCP_TLS_SKIP_VERIFY", "OCIS_MCP_HTTP_TIMEOUT",
	} {
		_ = os.Unsetenv(k)
	}
}

func TestLoadMinimalConfig(t *testing.T) {
	clearEnv()
	_ = os.Setenv("OCIS_MCP_OCIS_URL", "https://ocis.example.com")
	_ = os.Setenv("OCIS_MCP_APP_TOKEN_USER", "admin")
	_ = os.Setenv("OCIS_MCP_APP_TOKEN_VALUE", "tok")

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
	_ = os.Setenv("OCIS_MCP_APP_TOKEN_USER", "admin")
	_ = os.Setenv("OCIS_MCP_APP_TOKEN_VALUE", "tok")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for HTTP without INSECURE")
	}
}

func TestLoadHTTPWithInsecure(t *testing.T) {
	clearEnv()
	_ = os.Setenv("OCIS_MCP_OCIS_URL", "http://insecure.example.com")
	_ = os.Setenv("OCIS_MCP_APP_TOKEN_USER", "admin")
	_ = os.Setenv("OCIS_MCP_APP_TOKEN_VALUE", "tok")
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
	_ = os.Setenv("OCIS_MCP_APP_TOKEN_USER", "admin")
	_ = os.Setenv("OCIS_MCP_APP_TOKEN_VALUE", "tok")
	_ = os.Setenv("OCIS_MCP_TRANSPORT", "grpc")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid transport")
	}
}

func TestLoadAmbiguousAuth(t *testing.T) {
	clearEnv()
	_ = os.Setenv("OCIS_MCP_OCIS_URL", "https://ocis.example.com")
	_ = os.Setenv("OCIS_MCP_APP_TOKEN_USER", "admin")
	_ = os.Setenv("OCIS_MCP_APP_TOKEN_VALUE", "tok")
	_ = os.Setenv("OCIS_MCP_OIDC_ACCESS_TOKEN", "bearer-token")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for ambiguous auth")
	}
}

func TestLoadExplicitAuthMode(t *testing.T) {
	clearEnv()
	_ = os.Setenv("OCIS_MCP_OCIS_URL", "https://ocis.example.com")
	_ = os.Setenv("OCIS_MCP_APP_TOKEN_USER", "admin")
	_ = os.Setenv("OCIS_MCP_APP_TOKEN_VALUE", "tok")
	_ = os.Setenv("OCIS_MCP_OIDC_ACCESS_TOKEN", "bearer-token")
	_ = os.Setenv("OCIS_MCP_AUTH_MODE", "oidc")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.AuthMode != "oidc" {
		t.Errorf("AuthMode = %q, want oidc", cfg.AuthMode)
	}
}

func TestLoadOIDCAutoDetect(t *testing.T) {
	clearEnv()
	_ = os.Setenv("OCIS_MCP_OCIS_URL", "https://ocis.example.com")
	_ = os.Setenv("OCIS_MCP_OIDC_ACCESS_TOKEN", "bearer-token")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.AuthMode != "oidc" {
		t.Errorf("AuthMode = %q, want oidc", cfg.AuthMode)
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
			name: "infer oidc",
			cfg:  Config{OidcAccessToken: "tok"},
			want: "oidc",
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
}
