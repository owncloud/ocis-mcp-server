package mcpserver

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
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
		},
	)
	mux.Handle("/mcp", guard(handler))
	// The SDK checks the Host header but not Origin. Reject cross-origin browser
	// requests; clients that send no Origin header are unaffected.
	// SecurityHeaders is outermost so rejections and 404s carry the headers too.
	return middleware.SecurityHeaders(cop.Handler(mux)), nil
}
