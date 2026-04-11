package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/owncloud/ocis-mcp-server/internal/client"
)

func registerResources(s *mcp.Server, c *client.Client) {
	s.AddResource(
		&mcp.Resource{
			URI:         "ocis://capabilities",
			Name:        "oCIS Server Capabilities",
			Description: "Cached server capabilities including supported features and limits",
			MIMEType:    "application/json",
		},
		capabilitiesHandler(c),
	)

	s.AddResource(
		&mcp.Resource{
			URI:         "ocis://version",
			Name:        "oCIS Server Version",
			Description: "oCIS server version string",
			MIMEType:    "application/json",
		},
		versionHandler(c),
	)

	s.AddResource(
		&mcp.Resource{
			URI:         "ocis://sharing-roles",
			Name:        "oCIS Sharing Roles",
			Description: "Available sharing roles and their permission sets",
			MIMEType:    "application/json",
		},
		sharingRolesHandler(c),
	)

	s.AddResource(
		&mcp.Resource{
			URI:         "ocis://drive-types",
			Name:        "oCIS Drive Types",
			Description: "Supported drive types: personal, project, shares, virtual",
			MIMEType:    "application/json",
		},
		driveTypesHandler(),
	)

	s.AddResource(
		&mcp.Resource{
			URI:         "ocis://auth-mode",
			Name:        "Authentication Mode",
			Description: "Current authentication mode and connection info (no credentials)",
			MIMEType:    "application/json",
		},
		authModeHandler(c),
	)
}

func textResource(uri, text string) *mcp.ReadResourceResult {
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{URI: uri, MIMEType: "application/json", Text: text},
		},
	}
}

func capabilitiesHandler(c *client.Client) mcp.ResourceHandler {
	return func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		data, statusCode, err := client.GetRaw(ctx, c, "/ocs/v1.php/cloud/capabilities?format=json")
		if err != nil {
			return nil, fmt.Errorf("fetching capabilities: %w", err)
		}
		if statusCode >= 400 {
			return nil, fmt.Errorf("capabilities endpoint returned HTTP %d", statusCode)
		}
		return textResource(req.Params.URI, string(data)), nil
	}
}

func versionHandler(c *client.Client) mcp.ResourceHandler {
	return func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		data, statusCode, err := client.GetRaw(ctx, c, "/ocs/v1.php/cloud/capabilities?format=json")
		if err != nil {
			return nil, fmt.Errorf("fetching version: %w", err)
		}
		if statusCode >= 400 {
			return nil, fmt.Errorf("capabilities endpoint returned HTTP %d", statusCode)
		}
		var caps struct {
			OCS struct {
				Data struct {
					Version map[string]any `json:"version"`
				} `json:"data"`
			} `json:"ocs"`
		}
		if err := json.Unmarshal(data, &caps); err != nil {
			return nil, fmt.Errorf("parsing version: %w", err)
		}
		verJSON, _ := json.MarshalIndent(caps.OCS.Data.Version, "", "  ")
		return textResource(req.Params.URI, string(verJSON)), nil
	}
}

func sharingRolesHandler(c *client.Client) mcp.ResourceHandler {
	return func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		drives, err := client.ListJSON[Drive](ctx, c, "/graph/v1.0/me/drives", nil)
		if err != nil {
			return nil, fmt.Errorf("listing drives for roles: %w", err)
		}
		if len(drives) == 0 {
			return textResource(req.Params.URI, "[]"), nil
		}
		path := fmt.Sprintf("/graph/v1.0/drives/%s/root/permissions/roles", drives[0].ID)
		roles, err := client.ListJSON[SharingRole](ctx, c, path, nil)
		if err != nil {
			return nil, fmt.Errorf("fetching sharing roles: %w", err)
		}
		rolesJSON, _ := json.MarshalIndent(roles, "", "  ")
		return textResource(req.Params.URI, string(rolesJSON)), nil
	}
}

func driveTypesHandler() mcp.ResourceHandler {
	return func(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		types := []map[string]string{
			{"type": "personal", "description": "User's personal space (one per user)"},
			{"type": "project", "description": "Collaborative project space with shared membership"},
			{"type": "shares", "description": "Virtual space aggregating shares received by the user"},
			{"type": "virtual", "description": "System-managed virtual space"},
		}
		data, _ := json.MarshalIndent(types, "", "  ")
		return textResource(req.Params.URI, string(data)), nil
	}
}

func authModeHandler(c *client.Client) mcp.ResourceHandler {
	return func(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		cfg := c.Config()
		info := map[string]string{
			"auth_mode": cfg.AuthMode,
			"ocis_url":  cfg.OcisBaseURL(),
			"transport": cfg.Transport,
		}
		data, _ := json.MarshalIndent(info, "", "  ")
		return textResource(req.Params.URI, string(data)), nil
	}
}
