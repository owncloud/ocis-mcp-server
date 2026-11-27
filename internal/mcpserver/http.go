package mcpserver

import (
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/owncloud/ocis-mcp-server/internal/middleware"
)

// NewHTTPHandler returns a mux serving server on /mcp.
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
	mux.Handle("/mcp", middleware.SecurityHeaders(middleware.RequireBearer(secret)(handler)))
	return mux
}
