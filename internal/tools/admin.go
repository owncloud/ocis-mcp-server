package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/owncloud/ocis-mcp-server/internal/client"
)

// --- Input/Output types ---

type HealthCheckInput struct{}

type HealthCheckOutput struct {
	Healthy bool   `json:"healthy"`
	Status  int    `json:"status"`
	Message string `json:"message"`
}

type GetVersionInput struct{}

type OcisVersion struct {
	Major   int    `json:"major"`
	Minor   int    `json:"minor"`
	Micro   int    `json:"micro"`
	String  string `json:"string"`
	Edition string `json:"edition"`
	Product string `json:"product,omitempty"`
}

type GetVersionOutput struct {
	Version OcisVersion `json:"version"`
}

type GetCapabilitiesInput struct{}

// ocsCapabilitiesResponse wraps the OCS capabilities envelope.
type ocsCapabilitiesResponse struct {
	OCS struct {
		Data struct {
			Version      OcisVersion    `json:"version"`
			Capabilities map[string]any `json:"capabilities"`
		} `json:"data"`
	} `json:"ocs"`
}

type GetCapabilitiesOutput struct {
	Version      OcisVersion    `json:"version"`
	Capabilities map[string]any `json:"capabilities"`
}

type GetConfigInput struct{}

type GetConfigOutput struct {
	AuthMode    string `json:"auth_mode"`
	OcisURL     string `json:"ocis_url"`
	Transport   string `json:"transport"`
	HTTPAddr    string `json:"http_addr,omitempty"`
	TLSVerify   bool   `json:"tls_verify"`
	Insecure    bool   `json:"insecure"`
}

// --- Registration ---

func registerAdmin(s *mcp.Server, c *client.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_health_check",
		Description: "Check whether the oCIS instance is reachable and responding. Returns health status and HTTP status code.",
		Annotations: readOnlyAnnotations(),
	}, handleHealthCheck(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_get_version",
		Description: "Get the oCIS server version including major, minor, micro, edition, and product string.",
		Annotations: readOnlyAnnotations(),
	}, handleGetVersion(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_get_capabilities",
		Description: "Get the full capabilities report from oCIS including supported features, sharing options, and file handling limits.",
		Annotations: readOnlyAnnotations(),
	}, handleGetCapabilities(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_get_config",
		Description: "Get the current MCP server configuration including auth mode, oCIS URL, and transport settings. Credentials are never exposed.",
		Annotations: readOnlyAnnotations(),
	}, handleGetConfig(c))
}

// --- Handlers ---

func handleHealthCheck(c *client.Client) mcp.ToolHandlerFor[HealthCheckInput, HealthCheckOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, _ HealthCheckInput) (*mcp.CallToolResult, HealthCheckOutput, error) {
		_, status, err := client.GetRaw(ctx, c, "/.well-known/openid-configuration")
		if err != nil {
			return nil, HealthCheckOutput{
				Healthy: false,
				Status:  0,
				Message: fmt.Sprintf("connection failed: %v", err),
			}, nil
		}
		healthy := status >= 200 && status < 400
		msg := "oCIS is healthy"
		if !healthy {
			msg = fmt.Sprintf("oCIS returned status %d", status)
		}
		return nil, HealthCheckOutput{
			Healthy: healthy,
			Status:  status,
			Message: msg,
		}, nil
	}
}

func handleGetVersion(c *client.Client) mcp.ToolHandlerFor[GetVersionInput, GetVersionOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, _ GetVersionInput) (*mcp.CallToolResult, GetVersionOutput, error) {
		raw, status, err := client.GetRaw(ctx, c, "/ocs/v1.php/cloud/capabilities?format=json")
		if err != nil {
			return nil, GetVersionOutput{}, err
		}
		if status >= 400 {
			return nil, GetVersionOutput{}, fmt.Errorf("capabilities API returned status %d", status)
		}

		var resp ocsCapabilitiesResponse
		if err := json.Unmarshal(raw, &resp); err != nil {
			return nil, GetVersionOutput{}, fmt.Errorf("decoding capabilities response: %w", err)
		}
		return nil, GetVersionOutput{Version: resp.OCS.Data.Version}, nil
	}
}

func handleGetCapabilities(c *client.Client) mcp.ToolHandlerFor[GetCapabilitiesInput, GetCapabilitiesOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, _ GetCapabilitiesInput) (*mcp.CallToolResult, GetCapabilitiesOutput, error) {
		raw, status, err := client.GetRaw(ctx, c, "/ocs/v1.php/cloud/capabilities?format=json")
		if err != nil {
			return nil, GetCapabilitiesOutput{}, err
		}
		if status >= 400 {
			return nil, GetCapabilitiesOutput{}, fmt.Errorf("capabilities API returned status %d", status)
		}

		var resp ocsCapabilitiesResponse
		if err := json.Unmarshal(raw, &resp); err != nil {
			return nil, GetCapabilitiesOutput{}, fmt.Errorf("decoding capabilities response: %w", err)
		}
		return nil, GetCapabilitiesOutput{
			Version:      resp.OCS.Data.Version,
			Capabilities: resp.OCS.Data.Capabilities,
		}, nil
	}
}

func handleGetConfig(c *client.Client) mcp.ToolHandlerFor[GetConfigInput, GetConfigOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, _ GetConfigInput) (*mcp.CallToolResult, GetConfigOutput, error) {
		cfg := c.Config()
		return nil, GetConfigOutput{
			AuthMode:  cfg.AuthMode,
			OcisURL:   cfg.OcisBaseURL(),
			Transport: cfg.Transport,
			HTTPAddr:  cfg.HTTPAddr,
			TLSVerify: !cfg.TLSSkipVerify,
			Insecure:  cfg.Insecure,
		}, nil
	}
}
