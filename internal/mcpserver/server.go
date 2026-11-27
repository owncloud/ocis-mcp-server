// Package mcpserver builds the MCP server and its HTTP transport handler.
package mcpserver

import (
	"context"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/owncloud/ocis-mcp-server/internal/client"
	"github.com/owncloud/ocis-mcp-server/internal/config"
	"github.com/owncloud/ocis-mcp-server/internal/tools"
)

// New builds the MCP server and registers all tools, resources and prompts.
func New(version string, c *client.Client, cfg *config.Config) *mcp.Server {
	server := mcp.NewServer(
		&mcp.Implementation{
			Name:    "ocis-mcp-server",
			Version: version,
		},
		&mcp.ServerOptions{SetCacheable: setCacheable},
	)
	server.AddReceivingMiddleware(AuditMiddleware(slog.Default()))
	tools.RegisterAll(server, c, cfg)
	return server
}

// Cache hint values, in milliseconds. Listings and discovery change only when
// the server is redeployed; resource bodies reflect live oCIS state and may
// differ per caller, so they get a short, private TTL.
const (
	listTTLMs     = 300000
	resourceTTLMs = 30000
)

// setCacheable fills the 2026-07-28 ttlMs and cacheScope fields. The SDK may
// call it with the server lock held, so it must not call back into the server.
func setCacheable(_ context.Context, req mcp.Request, c *mcp.Cacheable) {
	if _, ok := req.GetParams().(*mcp.ReadResourceParams); ok {
		c.TTLMs, c.CacheScope = resourceTTLMs, "private"
		return
	}
	c.TTLMs, c.CacheScope = listTTLMs, "public"
}
