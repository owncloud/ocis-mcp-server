package tools

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/owncloud/ocis-mcp-server/internal/client"
)

// OCMProvider represents an Open Cloud Mesh federation provider.
type OCMProvider struct {
	Name     string `json:"name"`
	Domain   string `json:"domain"`
	Endpoint string `json:"endpoint,omitempty"`
}

// OCMShare represents an Open Cloud Mesh share.
type OCMShare struct {
	ID           string       `json:"id"`
	Name         string       `json:"name,omitempty"`
	ResourceID   string       `json:"resourceId,omitempty"`
	ResourceType string       `json:"resourceType,omitempty"`
	Owner        *OCMIdentity `json:"owner,omitempty"`
	Creator      *OCMIdentity `json:"creator,omitempty"`
	Grantee      *OCMIdentity `json:"grantee,omitempty"`
	ShareType    string       `json:"shareType,omitempty"`
	Expiration   string       `json:"expiration,omitempty"`
	CreatedAt    string       `json:"createdAt,omitempty"`
}

// OCMIdentity represents a user identity in an OCM context.
type OCMIdentity struct {
	ID    string `json:"id,omitempty"`
	Idp   string `json:"idp,omitempty"`
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
}

// --- Input/Output types ---

type ListOCMProvidersInput struct{}

type ListOCMProvidersOutput struct {
	Providers []OCMProvider `json:"providers"`
}

type CreateOCMShareInput struct {
	ResourceID   string `json:"resource_id" jsonschema:"ID of the resource to share,required"`
	GranteeID    string `json:"grantee_id" jsonschema:"ID of the remote user to share with,required"`
	GranteeIdp   string `json:"grantee_idp" jsonschema:"Identity provider domain of the remote user,required"`
	GranteeName  string `json:"grantee_name,omitempty" jsonschema:"Display name of the remote user"`
	GranteeEmail string `json:"grantee_email,omitempty" jsonschema:"Email of the remote user"`
	ShareType    string `json:"share_type,omitempty" jsonschema:"Share type (e.g. user)"`
}

type CreateOCMShareOutput struct {
	OCMShare
}

type ListOCMSharesInput struct{}

type ListOCMSharesOutput struct {
	Shares []OCMShare `json:"shares"`
}

type ListOCMReceivedInput struct{}

type ListOCMReceivedOutput struct {
	Shares []OCMShare `json:"shares"`
}

// --- Registration ---

func registerOCM(s *mcp.Server, c *client.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_ocm_list_providers",
		Description: "List known Open Cloud Mesh federation providers. Returns provider names, domains, and endpoints.",
		Annotations: readOnlyAnnotations(),
	}, handleListOCMProviders(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_ocm_create_share",
		Description: "Create a federated share via Open Cloud Mesh to a remote user on another instance.",
		Annotations: mutatingAnnotations(),
	}, handleCreateOCMShare(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_ocm_list_shares",
		Description: "List all outgoing OCM shares created by the current user.",
		Annotations: readOnlyAnnotations(),
	}, handleListOCMShares(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_ocm_list_received",
		Description: "List all incoming OCM shares received from remote users.",
		Annotations: readOnlyAnnotations(),
	}, handleListOCMReceived(c))
}

// --- Handlers ---

func handleListOCMProviders(c *client.Client) mcp.ToolHandlerFor[ListOCMProvidersInput, ListOCMProvidersOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, _ ListOCMProvidersInput) (*mcp.CallToolResult, ListOCMProvidersOutput, error) {
		providers, err := client.GetJSON[[]OCMProvider](ctx, c, "/ocm/providers", nil)
		if err != nil {
			return nil, ListOCMProvidersOutput{}, err
		}
		if providers == nil {
			providers = []OCMProvider{}
		}
		return nil, ListOCMProvidersOutput{Providers: providers}, nil
	}
}

func handleCreateOCMShare(c *client.Client) mcp.ToolHandlerFor[CreateOCMShareInput, CreateOCMShareOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input CreateOCMShareInput) (*mcp.CallToolResult, CreateOCMShareOutput, error) {
		if err := client.ValidateID("resource_id", input.ResourceID); err != nil {
			return nil, CreateOCMShareOutput{}, err
		}
		if err := client.ValidateID("grantee_id", input.GranteeID); err != nil {
			return nil, CreateOCMShareOutput{}, err
		}
		if err := client.ValidateID("grantee_idp", input.GranteeIdp); err != nil {
			return nil, CreateOCMShareOutput{}, fmt.Errorf("grantee_idp is required")
		}

		grantee := map[string]any{
			"id":  input.GranteeID,
			"idp": input.GranteeIdp,
		}
		if input.GranteeName != "" {
			grantee["name"] = input.GranteeName
		}
		if input.GranteeEmail != "" {
			grantee["email"] = input.GranteeEmail
		}

		body := map[string]any{
			"resourceId": input.ResourceID,
			"grantee":    grantee,
		}
		if input.ShareType != "" {
			body["shareType"] = input.ShareType
		}

		share, err := client.PostJSON[OCMShare](ctx, c, "/ocm/shares", body)
		if err != nil {
			return nil, CreateOCMShareOutput{}, err
		}
		return nil, CreateOCMShareOutput{OCMShare: share}, nil
	}
}

func handleListOCMShares(c *client.Client) mcp.ToolHandlerFor[ListOCMSharesInput, ListOCMSharesOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, _ ListOCMSharesInput) (*mcp.CallToolResult, ListOCMSharesOutput, error) {
		shares, err := client.GetJSON[[]OCMShare](ctx, c, "/ocm/shares", nil)
		if err != nil {
			return nil, ListOCMSharesOutput{}, err
		}
		if shares == nil {
			shares = []OCMShare{}
		}
		return nil, ListOCMSharesOutput{Shares: shares}, nil
	}
}

func handleListOCMReceived(c *client.Client) mcp.ToolHandlerFor[ListOCMReceivedInput, ListOCMReceivedOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, _ ListOCMReceivedInput) (*mcp.CallToolResult, ListOCMReceivedOutput, error) {
		shares, err := client.GetJSON[[]OCMShare](ctx, c, "/ocm/received-shares", nil)
		if err != nil {
			return nil, ListOCMReceivedOutput{}, err
		}
		if shares == nil {
			shares = []OCMShare{}
		}
		return nil, ListOCMReceivedOutput{Shares: shares}, nil
	}
}
