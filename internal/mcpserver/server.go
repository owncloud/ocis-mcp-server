// Package mcpserver builds the MCP server and its HTTP transport handler.
package mcpserver

import (
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
		nil,
	)
	tools.RegisterAll(server, c, cfg)
	return server
}
