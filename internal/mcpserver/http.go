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
		nil,
	)
	mux := http.NewServeMux()
	mux.Handle("/mcp", middleware.SecurityHeaders(middleware.RequireBearer(secret)(handler)))
	return mux
}
