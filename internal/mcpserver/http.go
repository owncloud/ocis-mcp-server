package mcpserver

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
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

// hostAllowlist rejects with 403 any request whose Host is not the public host of
// resource (with or without the scheme's default port, or the configured port), or a
// loopback name (localhost, 127.0.0.1, [::1]) on any port. This blocks DNS rebinding
// without depending on the connection's local address.
func hostAllowlist(resource *url.URL) func(http.Handler) http.Handler {
	publicHost := strings.ToLower(resource.Hostname())
	defaultPort := "80"
	if strings.EqualFold(resource.Scheme, "https") {
		defaultPort = "443"
	}
	port := resource.Port()
	if port == "" {
		port = defaultPort
	}
	allowedPorts := map[string]bool{port: true}
	if port == defaultPort {
		allowedPorts[""] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			host, hostPort := r.Host, ""
			if h, p, err := net.SplitHostPort(r.Host); err == nil {
				host, hostPort = h, p
			}
			host = strings.Trim(strings.ToLower(host), "[]")
			switch {
			case host == "localhost", host == "127.0.0.1", host == "::1":
			case host == publicHost && allowedPorts[hostPort]:
			default:
				http.Error(w, "forbidden host", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
