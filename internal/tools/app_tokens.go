package tools

import (
	"context"
	"fmt"
	"net/url"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/owncloud/ocis-mcp-server/internal/client"
)

// AppToken represents an application authentication token.
type AppToken struct {
	Token      string `json:"token,omitempty"`
	Label      string `json:"label,omitempty"`
	Expiry     string `json:"expiry,omitempty"`
	CreatedAt  string `json:"created_date,omitempty"`
	LastUsedAt string `json:"last_used_date,omitempty"`
}

// --- Input/Output types ---

type ListAppTokensInput struct{}

type ListAppTokensOutput struct {
	Tokens []AppToken `json:"tokens"`
}

type CreateAppTokenInput struct {
	Label  string `json:"label,omitempty" jsonschema:"Human-readable label for the token"`
	Expiry string `json:"expiry,omitempty" jsonschema:"Token expiry duration (e.g. 72h or 2160h for 90 days)"`
}

type CreateAppTokenOutput struct {
	AppToken
}

type DeleteAppTokenInput struct {
	Token   string `json:"token" jsonschema:"Token value to delete,required"`
	Confirm bool   `json:"confirm" jsonschema:"Must be true to confirm deletion"`
}

// --- Registration ---

func registerAppTokens(s *mcp.Server, c *client.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_list_app_tokens",
		Description: "List all application tokens for the current user. Returns token labels, expiry dates, and usage info (token values are not shown).",
		Annotations: readOnlyAnnotations(),
	}, handleListAppTokens(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_create_app_token",
		Description: "Create a new application authentication token. The token value is only shown once in the response.",
		Annotations: mutatingAnnotations(),
	}, handleCreateAppToken(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_delete_app_token",
		Description: "Delete an application token. Requires confirm=true.",
		Annotations: destructiveAnnotations(),
	}, handleDeleteAppToken(c))
}

// --- Handlers ---

func handleListAppTokens(c *client.Client) mcp.ToolHandlerFor[ListAppTokensInput, ListAppTokensOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, _ ListAppTokensInput) (*mcp.CallToolResult, ListAppTokensOutput, error) {
		tokens, err := client.GetJSON[[]AppToken](ctx, c, "/auth-app/tokens", nil)
		if err != nil {
			return nil, ListAppTokensOutput{}, err
		}
		if tokens == nil {
			tokens = []AppToken{}
		}
		return nil, ListAppTokensOutput{Tokens: tokens}, nil
	}
}

func handleCreateAppToken(c *client.Client) mcp.ToolHandlerFor[CreateAppTokenInput, CreateAppTokenOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input CreateAppTokenInput) (*mcp.CallToolResult, CreateAppTokenOutput, error) {
		q := url.Values{}
		if input.Expiry != "" {
			q.Set("expiry", input.Expiry)
		}
		if input.Label != "" {
			q.Set("label", input.Label)
		}
		path := "/auth-app/tokens"
		if len(q) > 0 {
			path = path + "?" + q.Encode()
		}
		token, err := client.PostJSON[AppToken](ctx, c, path, nil)
		if err != nil {
			return nil, CreateAppTokenOutput{}, err
		}
		return nil, CreateAppTokenOutput{AppToken: token}, nil
	}
}

func handleDeleteAppToken(c *client.Client) mcp.ToolHandlerFor[DeleteAppTokenInput, DeleteOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input DeleteAppTokenInput) (*mcp.CallToolResult, DeleteOutput, error) {
		if !input.Confirm {
			return nil, DeleteOutput{}, fmt.Errorf("this is a destructive operation. Set confirm=true to proceed")
		}
		if err := client.ValidateID("token", input.Token); err != nil {
			return nil, DeleteOutput{}, err
		}
		q := url.Values{}
		q.Set("token", input.Token)
		path := "/auth-app/tokens?" + q.Encode()
		if err := client.Delete(ctx, c, path); err != nil {
			return nil, DeleteOutput{}, err
		}
		return nil, DeleteOutput{Success: true, Message: "app token deleted"}, nil
	}
}
