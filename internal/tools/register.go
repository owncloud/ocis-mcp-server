package tools

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/owncloud/ocis-mcp-server/internal/client"
	"github.com/owncloud/ocis-mcp-server/internal/config"
)

// RegisterAll registers all MCP tools, resources, and prompts on the server.
func RegisterAll(s *mcp.Server, c *client.Client, cfg *config.Config) {
	// Core tools
	registerUsers(s, c)
	registerGroups(s, c)
	registerSpaces(s, c)
	registerFiles(s, c)
	registerShares(s, c)
	registerSearch(s, c)
	registerNotifications(s, c)
	registerSettings(s, c)
	registerAppTokens(s, c)
	registerAdmin(s, c)
	registerOCM(s, c)
	registerWorkflows(s, c)

	// Education tools (only if configured)
	registerEducation(s, c, cfg.EducationAccessToken != "")

	// MCP Resources and Prompts
	registerResources(s, c)
	registerPrompts(s)
}
