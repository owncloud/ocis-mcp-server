package mcpserver

import (
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
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

// NewHTTPHandler returns a handler serving server on /mcp.
//
// Every request is authenticated with a bearer token (no-op when secret is empty;
// config validation forbids that on a non-loopback bind), then security headers are added.
func NewHTTPHandler(server *mcp.Server, secret string) http.Handler {
	handler := mcp.NewStreamableHTTPHandler(
		func(_ *http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{
			// Stateless is required to serve MCP 2026-07-28 requests.
			Stateless: true,
			// Without this a 2026-07-28 client disconnect does not cancel the handler.
			PropagateRequestCancellation: true,
		},
	)
	mux := http.NewServeMux()
	mux.Handle("/mcp", middleware.RequireBearer(secret)(handler))
	// The SDK checks the Host header but not Origin. Reject cross-origin browser
	// requests; clients that send no Origin header are unaffected.
	// SecurityHeaders is outermost so rejections and 404s carry the headers too.
	return middleware.SecurityHeaders(http.NewCrossOriginProtection().Handler(mux))
}
