package config

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all configuration for the MCP server, loaded from environment variables.
type Config struct {
	// oCIS connection
	OcisURL string // OCIS_MCP_OCIS_URL

	// Authentication mode: "app-token" or "oidc" (default: auto-detect)
	AuthMode string // OCIS_MCP_AUTH_MODE

	// App Token auth (preferred for MCP)
	AppTokenUser  string // OCIS_MCP_APP_TOKEN_USER
	AppTokenValue string // OCIS_MCP_APP_TOKEN_VALUE

	// OIDC auth (alternative)
	OidcIssuer       string // OCIS_MCP_OIDC_ISSUER
	OidcClientID     string // OCIS_MCP_OIDC_CLIENT_ID
	OidcClientSecret string // OCIS_MCP_OIDC_CLIENT_SECRET
	OidcAccessToken  string // OCIS_MCP_OIDC_ACCESS_TOKEN

	// Education API (optional)
	EducationAccessToken string // OCIS_MCP_EDUCATION_ACCESS_TOKEN

	// Transport
	Transport string // OCIS_MCP_TRANSPORT ("stdio" | "http", default: "stdio")
	HTTPAddr  string // OCIS_MCP_HTTP_ADDR (default: "127.0.0.1:8090")

	// Logging
	LogLevel string // OCIS_MCP_LOG_LEVEL (default: "info")

	// Security
	Insecure      bool          // OCIS_MCP_INSECURE
	TLSSkipVerify bool          // OCIS_MCP_TLS_SKIP_VERIFY
	HTTPTimeout   time.Duration // OCIS_MCP_HTTP_TIMEOUT (default: 30s)
}

// Load reads configuration from environment variables and validates it.
func Load() (*Config, error) {
	cfg := &Config{
		OcisURL:              os.Getenv("OCIS_MCP_OCIS_URL"),
		AuthMode:             os.Getenv("OCIS_MCP_AUTH_MODE"),
		AppTokenUser:         os.Getenv("OCIS_MCP_APP_TOKEN_USER"),
		AppTokenValue:        os.Getenv("OCIS_MCP_APP_TOKEN_VALUE"),
		OidcIssuer:           os.Getenv("OCIS_MCP_OIDC_ISSUER"),
		OidcClientID:         os.Getenv("OCIS_MCP_OIDC_CLIENT_ID"),
		OidcClientSecret:     os.Getenv("OCIS_MCP_OIDC_CLIENT_SECRET"),
		OidcAccessToken:      os.Getenv("OCIS_MCP_OIDC_ACCESS_TOKEN"),
		EducationAccessToken: os.Getenv("OCIS_MCP_EDUCATION_ACCESS_TOKEN"),
		Transport:            os.Getenv("OCIS_MCP_TRANSPORT"),
		HTTPAddr:             os.Getenv("OCIS_MCP_HTTP_ADDR"),
		LogLevel:             os.Getenv("OCIS_MCP_LOG_LEVEL"),
		Insecure:             envBool("OCIS_MCP_INSECURE"),
		TLSSkipVerify:        envBool("OCIS_MCP_TLS_SKIP_VERIFY"),
	}

	// Defaults
	if cfg.Transport == "" {
		cfg.Transport = "stdio"
	}
	if cfg.HTTPAddr == "" {
		cfg.HTTPAddr = "127.0.0.1:8090"
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}

	timeout := os.Getenv("OCIS_MCP_HTTP_TIMEOUT")
	if timeout == "" {
		cfg.HTTPTimeout = 30 * time.Second
	} else {
		d, err := time.ParseDuration(timeout)
		if err != nil {
			return nil, fmt.Errorf("invalid OCIS_MCP_HTTP_TIMEOUT %q: %w", timeout, err)
		}
		cfg.HTTPTimeout = d
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	cfg.AuthMode = selectAuthMode(cfg)
	return cfg, nil
}

func (c *Config) validate() error {
	if c.OcisURL == "" {
		return fmt.Errorf("OCIS_MCP_OCIS_URL is required")
	}

	u, err := url.Parse(c.OcisURL)
	if err != nil {
		return fmt.Errorf("invalid OCIS_MCP_OCIS_URL %q: %w", c.OcisURL, err)
	}

	if u.Scheme == "http" && !c.Insecure {
		return fmt.Errorf("OCIS_MCP_OCIS_URL uses plaintext HTTP. Set OCIS_MCP_INSECURE=true to allow this (dev only)")
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("OCIS_MCP_OCIS_URL must use http or https scheme, got %q", u.Scheme)
	}

	if c.Transport != "stdio" && c.Transport != "http" {
		return fmt.Errorf("OCIS_MCP_TRANSPORT must be 'stdio' or 'http', got %q", c.Transport)
	}

	// Reject ambiguous auth: both app-token and OIDC set without explicit mode
	hasAppToken := c.AppTokenUser != "" && c.AppTokenValue != ""
	hasOIDC := c.OidcAccessToken != ""
	if hasAppToken && hasOIDC && c.AuthMode == "" {
		return fmt.Errorf("both app-token and OIDC credentials are set. Set OCIS_MCP_AUTH_MODE explicitly")
	}

	return nil
}

func selectAuthMode(cfg *Config) string {
	if cfg.AuthMode != "" {
		return cfg.AuthMode
	}
	if cfg.AppTokenUser != "" && cfg.AppTokenValue != "" {
		return "app-token"
	}
	if cfg.OidcAccessToken != "" {
		return "oidc"
	}
	return "none"
}

// NewHTTPClient creates a configured *http.Client for oCIS API calls.
func (c *Config) NewHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if c.TLSSkipVerify {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec
	}
	return &http.Client{
		Timeout:   c.HTTPTimeout,
		Transport: transport,
	}
}

// OcisBaseURL returns the base URL with trailing slash stripped.
func (c *Config) OcisBaseURL() string {
	return strings.TrimRight(c.OcisURL, "/")
}

func envBool(key string) bool {
	v := os.Getenv(key)
	b, _ := strconv.ParseBool(v)
	return b
}
