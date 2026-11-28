package mcpserver

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	sdkauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
	"github.com/owncloud/ocis-mcp-server/internal/auth"
	"github.com/owncloud/ocis-mcp-server/internal/middleware"
)

// NewHTTPServer wraps h in an http.Server with slowloris and idle limits.
//
// ReadTimeout and WriteTimeout stay unset on purpose: responses may be streamed.
func NewHTTPServer(h http.Handler) *http.Server {
	return &http.Server{
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
}

// Inbound authentication modes accepted by NewHTTPHandler.
const (
	AuthModeOIDC   = "oidc"
	AuthModeSecret = "secret"
	AuthModeNone   = "none"
)

// metadataPath is where the protected resource metadata document is served.
const metadataPath = "/.well-known/oauth-protected-resource/mcp"

// HTTPAuth selects and configures inbound authentication for the HTTP transport.
type HTTPAuth struct {
	// Mode is "oidc", "secret" or "none". Any other value (including empty) is rejected.
	Mode string
	// Secret is the shared bearer secret. Used only in "secret" mode, ignored otherwise.
	Secret string
	// PublicURL is the public origin of this server (oidc mode).
	PublicURL string
	// Resource is the canonical resource URI served as "resource" in the metadata (oidc mode).
	Resource string
	// Issuer is the authorization server issuer advertised in the metadata (oidc mode).
	Issuer string
	// MetadataURL is the URL of the metadata document, used in challenges (oidc mode).
	// It must end in the path the handler serves it on.
	MetadataURL string
	// Scopes are the scopes a token must carry (oidc mode).
	Scopes []string
	// Verifier validates bearer tokens (oidc mode).
	Verifier sdkauth.TokenVerifier
}

// NewHTTPHandler returns a handler serving server on /mcp.
//
// It fails closed: a mode that is unknown, or that lacks what it needs to authenticate,
// is an error. Only Mode "none" serves /mcp without authentication. Security headers are
// added to every response, including rejections.
func NewHTTPHandler(server *mcp.Server, a HTTPAuth) (http.Handler, error) {
	cop := http.NewCrossOriginProtection()
	var guard func(http.Handler) http.Handler
	var hostGuard func(http.Handler) http.Handler
	disableSDKHostGuard := false
	mux := http.NewServeMux()

	switch a.Mode {
	case AuthModeNone:
		guard = func(h http.Handler) http.Handler { return h }
	case AuthModeSecret:
		if a.Secret == "" {
			return nil, errors.New("http auth mode secret requires a non-empty secret")
		}
		guard = middleware.RequireBearer(a.Secret)
	case AuthModeOIDC:
		switch {
		case a.Verifier == nil:
			return nil, errors.New("http auth mode oidc requires a token verifier")
		case a.PublicURL == "":
			return nil, errors.New("http auth mode oidc requires a public URL")
		case len(a.Scopes) == 0:
			return nil, errors.New("http auth mode oidc requires at least one scope")
		case a.Resource == "":
			return nil, errors.New("http auth mode oidc requires a resource URI")
		case a.Issuer == "":
			return nil, errors.New("http auth mode oidc requires an issuer")
		}
		mu, err := url.Parse(a.MetadataURL)
		if err != nil || mu.Path != metadataPath {
			return nil, fmt.Errorf("http auth mode oidc requires a metadata URL ending in %s", metadataPath)
		}
		ru, err := url.Parse(a.Resource)
		if err != nil || ru.Scheme == "" || ru.Host == "" {
			return nil, errors.New("http auth mode oidc requires an absolute resource URI")
		}
		if err := cop.AddTrustedOrigin(ru.Scheme + "://" + ru.Host); err != nil {
			return nil, fmt.Errorf("http auth mode oidc: trusted origin: %w", err)
		}
		// Behind a reverse proxy that keeps the public Host, the SDK guard (loopback
		// connection with a non-loopback Host) would reject every request, so it is
		// replaced by an explicit allowlist that covers every route.
		disableSDKHostGuard = true
		hostGuard = hostAllowlist(ru)
		guard = sdkauth.RequireBearerToken(a.Verifier, &sdkauth.RequireBearerTokenOptions{
			ResourceMetadataURL: a.MetadataURL,
			Scopes:              a.Scopes,
			ClockSkew:           auth.ClockSkew,
		})
		mux.Handle("GET "+metadataPath, sdkauth.ProtectedResourceMetadataHandler(&oauthex.ProtectedResourceMetadata{
			Resource:               a.Resource,
			AuthorizationServers:   []string{a.Issuer},
			ScopesSupported:        a.Scopes,
			BearerMethodsSupported: []string{"header"},
		}))
	default:
		return nil, fmt.Errorf("unknown http auth mode %q", a.Mode)
	}

	handler := mcp.NewStreamableHTTPHandler(
		func(_ *http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{
			// Stateless is required to serve MCP 2026-07-28 requests.
			Stateless: true,
			// Without this a 2026-07-28 client disconnect does not cancel the handler.
			PropagateRequestCancellation: true,
			// Set only in oidc mode, where hostGuard checks the Host header instead.
			DisableLocalhostProtection: disableSDKHostGuard,
		},
	)
	mux.Handle("/mcp", guard(handler))
	// The SDK checks the Host header but not Origin. Reject cross-origin browser
	// requests; clients that send no Origin header are unaffected.
	// SecurityHeaders is outermost so rejections and 404s carry the headers too.
	inner := cop.Handler(mux)
	if hostGuard != nil {
		inner = hostGuard(inner)
	}
	return middleware.SecurityHeaders(inner), nil
}

// hostAllowlist rejects with 403 any request whose Host is not the public hostname of
// resource (on any port, or none) or one of localhost, 127.0.0.1 and [::1] on any port.
// This blocks DNS rebinding without depending on the connection's local address.
//
// The port is deliberately not compared: a reverse proxy that forwards `Host: $host`
// drops the port of a non-default public URL, and the port adds no rebinding defence
// because an attacker cannot make a victim's browser send our hostname to another
// name. Every other form is refused: other hostnames (including prefixes, subdomains
// and a trailing dot), userinfo, an empty or non numeric port, and bracketed names that
// are not IP literals.
func hostAllowlist(resource *url.URL) func(http.Handler) http.Handler {
	publicHost := strings.ToLower(resource.Hostname())
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			host, ok := hostnameOf(r.Host)
			if !ok || (host != publicHost && host != "localhost" && host != "127.0.0.1" && host != "::1") {
				slog.Debug("rejected request with a Host header outside the allowlist", "host", r.Host)
				http.Error(w, "forbidden host", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// hostnameOf extracts the lowercased hostname from a Host header value, without
// brackets, and reports whether the value is well formed: a bare name, a name with a
// numeric port, a bare IPv6 literal, or a bracketed IP literal with an optional port.
func hostnameOf(hostHeader string) (string, bool) {
	h := strings.ToLower(hostHeader)
	if strings.HasPrefix(h, "[") {
		end := strings.Index(h, "]")
		if end < 0 {
			return "", false
		}
		name, rest := h[1:end], h[end+1:]
		if net.ParseIP(name) == nil {
			return "", false
		}
		if rest != "" && !validPortSuffix(rest) {
			return "", false
		}
		return name, true
	}
	switch strings.Count(h, ":") {
	case 0:
		return h, h != ""
	case 1:
		i := strings.Index(h, ":")
		if !validPortSuffix(h[i:]) {
			return "", false
		}
		return h[:i], h[:i] != ""
	}
	// More than one colon without brackets: only a bare IPv6 literal is meaningful.
	if net.ParseIP(h) == nil {
		return "", false
	}
	return h, true
}

// validPortSuffix reports whether s is ":" followed by a decimal port in 0..65535.
func validPortSuffix(s string) bool {
	port, ok := strings.CutPrefix(s, ":")
	if !ok || port == "" || len(port) > 5 {
		return false
	}
	for _, c := range port {
		if c < '0' || c > '9' {
			return false
		}
	}
	n, err := strconv.Atoi(port)
	return err == nil && n <= 65535
}
