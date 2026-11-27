package config

import (
	"crypto/tls"
	"fmt"
	"net"
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

	// HTTPSecret is the deprecated shared secret required as `Authorization: Bearer <secret>`
	// on the HTTP transport's /mcp endpoint in "secret" mode.
	HTTPSecret string // OCIS_MCP_HTTP_SECRET

	// Inbound auth for the HTTP transport. HTTPAuth is the resolved mode ("oidc", "secret"
	// or "none"); it is empty when the transport is stdio, which ignores OCIS_MCP_HTTP_*.
	HTTPAuth       string   // OCIS_MCP_HTTP_AUTH (inferred when unset)
	HTTPPublicURL  string   // OCIS_MCP_HTTP_PUBLIC_URL (origin only)
	HTTPOIDCIssuer string   // OCIS_MCP_HTTP_OIDC_ISSUER
	HTTPOIDCScopes []string // OCIS_MCP_HTTP_OIDC_SCOPES (default: ocis:mcp)
	HTTPOIDCCAFile string   // OCIS_MCP_HTTP_OIDC_CA_FILE

	// Warnings collects non-fatal startup notices. They never contain setting values.
	Warnings []string

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
		HTTPSecret:           os.Getenv("OCIS_MCP_HTTP_SECRET"),
		HTTPAuth:             strings.ToLower(strings.TrimSpace(os.Getenv("OCIS_MCP_HTTP_AUTH"))),
		HTTPPublicURL:        strings.TrimSpace(os.Getenv("OCIS_MCP_HTTP_PUBLIC_URL")),
		HTTPOIDCIssuer:       strings.TrimSpace(os.Getenv("OCIS_MCP_HTTP_OIDC_ISSUER")),
		HTTPOIDCCAFile:       os.Getenv("OCIS_MCP_HTTP_OIDC_CA_FILE"),
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

	cfg.HTTPOIDCScopes = parseScopes(os.Getenv("OCIS_MCP_HTTP_OIDC_SCOPES"))

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

	if err := c.resolveHTTPAuth(); err != nil {
		return err
	}

	// Reject ambiguous auth: both app-token and OIDC set without explicit mode
	hasAppToken := c.AppTokenUser != "" && c.AppTokenValue != ""
	hasOIDC := c.OidcAccessToken != ""
	if hasAppToken && hasOIDC && c.AuthMode == "" {
		return fmt.Errorf("both app-token and OIDC credentials are set. Set OCIS_MCP_AUTH_MODE explicitly")
	}

	return nil
}

// parseScopes splits a comma or space separated scope list. An empty list yields the default.
func parseScopes(raw string) []string {
	scopes := strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' })
	if len(scopes) == 0 {
		return []string{"ocis:mcp"}
	}
	return scopes
}

// resolveHTTPAuth validates the inbound HTTP auth settings and fixes HTTPAuth. The stdio
// transport ignores every OCIS_MCP_HTTP_* setting, so nothing is validated for it.
func (c *Config) resolveHTTPAuth() error {
	if c.Transport != "http" {
		c.HTTPAuth = ""
		return nil
	}

	switch c.HTTPAuth {
	case "":
		switch {
		case c.HTTPOIDCIssuer != "" && c.HTTPSecret != "":
			return fmt.Errorf("both OCIS_MCP_HTTP_OIDC_ISSUER and OCIS_MCP_HTTP_SECRET are set. Set OCIS_MCP_HTTP_AUTH explicitly")
		case c.HTTPOIDCIssuer != "":
			c.HTTPAuth = "oidc"
		case c.HTTPSecret != "":
			c.HTTPAuth = "secret"
		default:
			c.HTTPAuth = "none"
		}
	case "oidc", "secret", "none":
		c.warnUnusedHTTPCredentials()
		if c.HTTPAuth != "secret" {
			// The secret must never authenticate in another mode.
			c.HTTPSecret = ""
		}
	default:
		return fmt.Errorf("OCIS_MCP_HTTP_AUTH must be 'oidc', 'secret' or 'none', got %q", c.HTTPAuth)
	}

	switch c.HTTPAuth {
	case "oidc":
		if c.HTTPOIDCIssuer == "" {
			return fmt.Errorf("OCIS_MCP_HTTP_AUTH=oidc requires OCIS_MCP_HTTP_OIDC_ISSUER")
		}
		if c.HTTPPublicURL == "" {
			return fmt.Errorf("OCIS_MCP_HTTP_AUTH=oidc requires OCIS_MCP_HTTP_PUBLIC_URL")
		}
		if err := checkSecureURL("OCIS_MCP_HTTP_OIDC_ISSUER", c.HTTPOIDCIssuer, false); err != nil {
			return err
		}
		if err := checkSecureURL("OCIS_MCP_HTTP_PUBLIC_URL", c.HTTPPublicURL, true); err != nil {
			return err
		}
	case "secret":
		if c.HTTPSecret == "" {
			return fmt.Errorf("OCIS_MCP_HTTP_AUTH=secret requires OCIS_MCP_HTTP_SECRET")
		}
		c.Warnings = append(c.Warnings,
			"OCIS_MCP_HTTP_AUTH=secret is deprecated and will be removed in a future major version; use OCIS_MCP_HTTP_AUTH=oidc")
	case "none":
		if !isLoopbackHost(c.HTTPAddr) {
			return fmt.Errorf(
				"OCIS_MCP_TRANSPORT=http is bound to non-loopback address %q with no authentication: "+
					"set OCIS_MCP_HTTP_AUTH=oidc (or the deprecated secret mode), or bind to a "+
					"loopback address (see README > Securing the HTTP transport)", c.HTTPAddr)
		}
	}
	return nil
}

// warnUnusedHTTPCredentials adds a warning for each credential setting the explicit mode ignores.
func (c *Config) warnUnusedHTTPCredentials() {
	if c.HTTPAuth != "oidc" && c.HTTPOIDCIssuer != "" {
		c.Warnings = append(c.Warnings,
			fmt.Sprintf("OCIS_MCP_HTTP_OIDC_ISSUER is set but ignored because OCIS_MCP_HTTP_AUTH=%s", c.HTTPAuth))
	}
	if c.HTTPAuth != "secret" && c.HTTPSecret != "" {
		c.Warnings = append(c.Warnings,
			fmt.Sprintf("OCIS_MCP_HTTP_SECRET is set but ignored because OCIS_MCP_HTTP_AUTH=%s", c.HTTPAuth))
	}
}

// checkSecureURL requires https, or plain http for a loopback host. With originOnly the URL
// must carry no path, query or fragment. The URL itself is never echoed in the error.
func checkSecureURL(name, raw string, originOnly bool) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return fmt.Errorf("%s must be an absolute http(s) URL", name)
	}
	switch strings.ToLower(u.Scheme) {
	case "https":
	case "http":
		if !isLoopbackName(u.Hostname()) {
			return fmt.Errorf("%s must use https (plain http is only allowed for a loopback host)", name)
		}
	default:
		return fmt.Errorf("%s must use https (plain http is only allowed for a loopback host)", name)
	}
	if u.User != nil || u.Fragment != "" || strings.Contains(raw, "#") {
		return fmt.Errorf("%s must not contain userinfo or a fragment", name)
	}
	if originOnly {
		if (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.ForceQuery {
			return fmt.Errorf("%s must be an origin (scheme, host, optional port) with no path, query or fragment", name)
		}
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

// HTTPAuthEnabled reports whether the HTTP transport requires a bearer token.
func (c *Config) HTTPAuthEnabled() bool {
	return c.HTTPAuth == "oidc" || c.HTTPAuth == "secret"
}

// ResourceURI is the canonical protected-resource identifier: lowercase scheme and host,
// no default port, no trailing slash, ending in /mcp.
func (c *Config) ResourceURI() string {
	u, err := url.Parse(c.HTTPPublicURL)
	if err != nil {
		return strings.TrimRight(c.HTTPPublicURL, "/") + "/mcp"
	}
	scheme := strings.ToLower(u.Scheme)
	host := strings.ToLower(u.Hostname())
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	port := u.Port()
	if (scheme == "https" && port == "443") || (scheme == "http" && port == "80") {
		port = ""
	}
	if port != "" {
		host += ":" + port
	}
	return scheme + "://" + host + "/mcp"
}

// MetadataURL is where the protected-resource metadata document is served.
func (c *Config) MetadataURL() string {
	return strings.TrimRight(c.HTTPPublicURL, "/") + "/.well-known/oauth-protected-resource/mcp"
}

// IsLoopbackBind reports whether HTTPAddr binds only the loopback interface.
func (c *Config) IsLoopbackBind() bool {
	return isLoopbackHost(c.HTTPAddr)
}

// isLoopbackHost reports whether a "host:port" (or bare host) listen address binds only
// the loopback interface. A bare port (e.g. ":8090") or an unresolved hostname is treated
// as non-loopback (fail safe).
func isLoopbackHost(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	switch host {
	case "":
		return false
	case "localhost":
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback()
}

// isLoopbackName reports whether a bare host name (no port, no brackets) is loopback.
func isLoopbackName(host string) bool {
	host = strings.ToLower(host)
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func envBool(key string) bool {
	v := os.Getenv(key)
	b, _ := strconv.ParseBool(v)
	return b
}
