package tools

import (
	"context"
	"fmt"
	"net/url"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/owncloud/ocis-mcp-server/internal/client"
)

// --- Input/Output types ---

type CreateShareInput struct {
	SpaceID    string   `json:"space_id" jsonschema:"Drive/space ID,required"`
	ItemID     string   `json:"item_id" jsonschema:"Item ID to share,required"`
	Recipients []string `json:"recipients" jsonschema:"User or group IDs to invite,required"`
	Roles      []string `json:"roles" jsonschema:"Role IDs to assign (e.g. Viewer or Editor role IDs),required"`
	Expiration string   `json:"expiration,omitempty" jsonschema:"Expiration date (ISO 8601)"`
}

type CreateLinkInput struct {
	SpaceID    string `json:"space_id" jsonschema:"Drive/space ID,required"`
	ItemID     string `json:"item_id" jsonschema:"Item ID,required"`
	Type       string `json:"type,omitempty" jsonschema:"Link type: view, edit, createOnly, blocksDownload (default: view)"`
	Password   string `json:"password,omitempty" jsonschema:"Optional password"`
	Expiration string `json:"expiration,omitempty" jsonschema:"Expiration date (ISO 8601)"`
}

type ListSharesInput struct {
	SpaceID string `json:"space_id" jsonschema:"Drive/space ID,required"`
	ItemID  string `json:"item_id" jsonschema:"Item ID,required"`
}

type UpdateShareInput struct {
	SpaceID      string   `json:"space_id" jsonschema:"Drive/space ID,required"`
	ItemID       string   `json:"item_id" jsonschema:"Item ID,required"`
	PermissionID string   `json:"permission_id" jsonschema:"Permission ID to update,required"`
	Roles        []string `json:"roles,omitempty" jsonschema:"New role IDs"`
}

type UpdateShareExpirationInput struct {
	SpaceID      string `json:"space_id" jsonschema:"Drive/space ID,required"`
	ItemID       string `json:"item_id" jsonschema:"Item ID,required"`
	PermissionID string `json:"permission_id" jsonschema:"Permission ID,required"`
	Expiration   string `json:"expiration,omitempty" jsonschema:"New expiration date (ISO 8601) or empty to remove"`
}

type DeleteShareInput struct {
	SpaceID      string `json:"space_id" jsonschema:"Drive/space ID,required"`
	ItemID       string `json:"item_id" jsonschema:"Item ID,required"`
	PermissionID string `json:"permission_id" jsonschema:"Permission ID to delete,required"`
	Confirm      bool   `json:"confirm" jsonschema:"Must be true to confirm removal"`
}

type ListSharedByMeInput struct {
	Limit  int `json:"limit,omitempty" jsonschema:"Max results (default 50, max 200)"`
	Offset int `json:"offset,omitempty" jsonschema:"Skip results"`
}

type SharedItemsOutput struct {
	Items      []SharedItem `json:"items"`
	TotalCount int          `json:"total_count"`
}

type SharedItem struct {
	ID               string       `json:"id"`
	Name             string       `json:"name"`
	RemoteItem       *RemoteItem  `json:"remoteItem,omitempty"`
	Permissions      []Permission `json:"permissions,omitempty"`
}

type RemoteItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type AcceptShareInput struct {
	SpaceID      string `json:"space_id" jsonschema:"Drive/space ID,required"`
	ItemID       string `json:"item_id" jsonschema:"Item ID,required"`
	PermissionID string `json:"permission_id" jsonschema:"Permission ID to accept,required"`
}

type GetSharingRolesInput struct {
	SpaceID string `json:"space_id" jsonschema:"Drive/space ID,required"`
}

type SharingRolesOutput struct {
	Roles []SharingRole `json:"roles"`
}

// --- Registration ---

func registerShares(s *mcp.Server, c *client.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_create_share",
		Description: "Share a file or folder with users or groups by inviting them with specific roles.",
		Annotations: mutatingAnnotations(),
	}, handleCreateShare(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_create_link",
		Description: "Create a public sharing link for a file or folder.",
		Annotations: mutatingAnnotations(),
	}, handleCreateLink(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_list_shares",
		Description: "List all sharing permissions on a specific file or folder.",
		Annotations: readOnlyAnnotations(),
	}, handleListShares(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_update_share",
		Description: "Update a sharing permission's roles.",
		Annotations: idempotentAnnotations(),
	}, handleUpdateShare(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_update_share_expiration",
		Description: "Set or remove the expiration date on a sharing permission.",
		Annotations: idempotentAnnotations(),
	}, handleUpdateShareExpiration(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_delete_share",
		Description: "Remove a sharing permission. Requires confirm=true.",
		Annotations: destructiveAnnotations(),
	}, handleDeleteShare(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_list_shared_by_me",
		Description: "List all shares created by the current user.",
		Annotations: readOnlyAnnotations(),
	}, handleListSharedByMe(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_list_received_shares",
		Description: "List all shares received by the current user.",
		Annotations: readOnlyAnnotations(),
	}, handleListReceivedShares(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_accept_share",
		Description: "Accept an incoming share.",
		Annotations: idempotentAnnotations(),
	}, handleAcceptShare(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_reject_share",
		Description: "Reject an incoming share. Requires confirm=true.",
		Annotations: destructiveAnnotations(),
	}, handleRejectShare(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_get_sharing_roles",
		Description: "Get available sharing roles for a drive/space.",
		Annotations: readOnlyAnnotations(),
	}, handleGetSharingRoles(c))
}

// --- Handlers ---

func permPath(spaceID, itemID string) string {
	return fmt.Sprintf("/graph/v1.0/drives/%s/items/%s",
		url.PathEscape(spaceID), url.PathEscape(itemID))
}

func handleCreateShare(c *client.Client) mcp.ToolHandlerFor[CreateShareInput, InviteOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input CreateShareInput) (*mcp.CallToolResult, InviteOutput, error) {
		if err := client.ValidateID("space_id", input.SpaceID); err != nil {
			return nil, InviteOutput{}, err
		}
		if err := client.ValidateID("item_id", input.ItemID); err != nil {
			return nil, InviteOutput{}, err
		}
		recipients := make([]map[string]any, len(input.Recipients))
		for i, r := range input.Recipients {
			recipients[i] = map[string]any{"objectId": r}
		}
		body := map[string]any{
			"recipients": recipients,
			"roles":      input.Roles,
		}
		if input.Expiration != "" {
			body["expirationDateTime"] = input.Expiration
		}
		result, err := client.PostJSON[InviteOutput](ctx, c, permPath(input.SpaceID, input.ItemID)+"/invite", body)
		if err != nil {
			return nil, InviteOutput{}, err
		}
		return nil, result, nil
	}
}

func handleCreateLink(c *client.Client) mcp.ToolHandlerFor[CreateLinkInput, Permission] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input CreateLinkInput) (*mcp.CallToolResult, Permission, error) {
		if err := client.ValidateID("space_id", input.SpaceID); err != nil {
			return nil, Permission{}, err
		}
		if err := client.ValidateID("item_id", input.ItemID); err != nil {
			return nil, Permission{}, err
		}
		linkType := input.Type
		if linkType == "" {
			linkType = "view"
		}
		body := map[string]any{"type": linkType}
		if input.Password != "" {
			body["password"] = input.Password
		}
		if input.Expiration != "" {
			body["expirationDateTime"] = input.Expiration
		}
		perm, err := client.PostJSON[Permission](ctx, c, permPath(input.SpaceID, input.ItemID)+"/createLink", body)
		if err != nil {
			return nil, Permission{}, err
		}
		return nil, perm, nil
	}
}

func handleListShares(c *client.Client) mcp.ToolHandlerFor[ListSharesInput, ListPermissionsOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input ListSharesInput) (*mcp.CallToolResult, ListPermissionsOutput, error) {
		if err := client.ValidateID("space_id", input.SpaceID); err != nil {
			return nil, ListPermissionsOutput{}, err
		}
		if err := client.ValidateID("item_id", input.ItemID); err != nil {
			return nil, ListPermissionsOutput{}, err
		}
		perms, err := client.ListJSON[Permission](ctx, c, permPath(input.SpaceID, input.ItemID)+"/permissions", nil)
		if err != nil {
			return nil, ListPermissionsOutput{}, err
		}
		return nil, ListPermissionsOutput{Permissions: perms}, nil
	}
}

func handleUpdateShare(c *client.Client) mcp.ToolHandlerFor[UpdateShareInput, Permission] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input UpdateShareInput) (*mcp.CallToolResult, Permission, error) {
		if err := client.ValidateID("space_id", input.SpaceID); err != nil {
			return nil, Permission{}, err
		}
		if err := client.ValidateID("permission_id", input.PermissionID); err != nil {
			return nil, Permission{}, err
		}
		body := map[string]any{}
		if len(input.Roles) > 0 {
			body["roles"] = input.Roles
		}
		path := fmt.Sprintf("%s/permissions/%s", permPath(input.SpaceID, input.ItemID), url.PathEscape(input.PermissionID))
		perm, err := client.PatchJSON[Permission](ctx, c, path, body)
		if err != nil {
			return nil, Permission{}, err
		}
		return nil, perm, nil
	}
}

func handleUpdateShareExpiration(c *client.Client) mcp.ToolHandlerFor[UpdateShareExpirationInput, Permission] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input UpdateShareExpirationInput) (*mcp.CallToolResult, Permission, error) {
		if err := client.ValidateID("space_id", input.SpaceID); err != nil {
			return nil, Permission{}, err
		}
		if err := client.ValidateID("permission_id", input.PermissionID); err != nil {
			return nil, Permission{}, err
		}
		body := map[string]any{}
		if input.Expiration != "" {
			body["expirationDateTime"] = input.Expiration
		} else {
			body["expirationDateTime"] = nil
		}
		path := fmt.Sprintf("%s/permissions/%s", permPath(input.SpaceID, input.ItemID), url.PathEscape(input.PermissionID))
		perm, err := client.PatchJSON[Permission](ctx, c, path, body)
		if err != nil {
			return nil, Permission{}, err
		}
		return nil, perm, nil
	}
}

func handleDeleteShare(c *client.Client) mcp.ToolHandlerFor[DeleteShareInput, DeleteOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input DeleteShareInput) (*mcp.CallToolResult, DeleteOutput, error) {
		if !input.Confirm {
			return nil, DeleteOutput{}, fmt.Errorf("this is a destructive operation. Set confirm=true to proceed")
		}
		if err := client.ValidateID("space_id", input.SpaceID); err != nil {
			return nil, DeleteOutput{}, err
		}
		if err := client.ValidateID("permission_id", input.PermissionID); err != nil {
			return nil, DeleteOutput{}, err
		}
		path := fmt.Sprintf("%s/permissions/%s", permPath(input.SpaceID, input.ItemID), url.PathEscape(input.PermissionID))
		if err := client.Delete(ctx, c, path); err != nil {
			return nil, DeleteOutput{}, err
		}
		return nil, DeleteOutput{Success: true, Message: "share removed"}, nil
	}
}

func handleListSharedByMe(c *client.Client) mcp.ToolHandlerFor[ListSharedByMeInput, SharedItemsOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input ListSharedByMeInput) (*mcp.CallToolResult, SharedItemsOutput, error) {
		items, err := client.ListJSON[SharedItem](ctx, c, "/graph/v1.0/me/drive/sharedByMe", nil)
		if err != nil {
			return nil, SharedItemsOutput{}, err
		}
		return nil, SharedItemsOutput{Items: items, TotalCount: len(items)}, nil
	}
}

func handleListReceivedShares(c *client.Client) mcp.ToolHandlerFor[ListSharedByMeInput, SharedItemsOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input ListSharedByMeInput) (*mcp.CallToolResult, SharedItemsOutput, error) {
		items, err := client.ListJSON[SharedItem](ctx, c, "/graph/v1.0/me/drive/sharedWithMe", nil)
		if err != nil {
			return nil, SharedItemsOutput{}, err
		}
		return nil, SharedItemsOutput{Items: items, TotalCount: len(items)}, nil
	}
}

func handleAcceptShare(c *client.Client) mcp.ToolHandlerFor[AcceptShareInput, DeleteOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input AcceptShareInput) (*mcp.CallToolResult, DeleteOutput, error) {
		if err := client.ValidateID("space_id", input.SpaceID); err != nil {
			return nil, DeleteOutput{}, err
		}
		if err := client.ValidateID("permission_id", input.PermissionID); err != nil {
			return nil, DeleteOutput{}, err
		}
		path := fmt.Sprintf("%s/permissions/%s/accept", permPath(input.SpaceID, input.ItemID), url.PathEscape(input.PermissionID))
		_, err := client.PostJSONRaw(ctx, c, path, nil)
		if err != nil {
			return nil, DeleteOutput{}, err
		}
		return nil, DeleteOutput{Success: true, Message: "share accepted"}, nil
	}
}

func handleRejectShare(c *client.Client) mcp.ToolHandlerFor[DeleteShareInput, DeleteOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input DeleteShareInput) (*mcp.CallToolResult, DeleteOutput, error) {
		if !input.Confirm {
			return nil, DeleteOutput{}, fmt.Errorf("this is a destructive operation. Set confirm=true to proceed")
		}
		if err := client.ValidateID("space_id", input.SpaceID); err != nil {
			return nil, DeleteOutput{}, err
		}
		if err := client.ValidateID("permission_id", input.PermissionID); err != nil {
			return nil, DeleteOutput{}, err
		}
		path := fmt.Sprintf("%s/permissions/%s", permPath(input.SpaceID, input.ItemID), url.PathEscape(input.PermissionID))
		if err := client.Delete(ctx, c, path); err != nil {
			return nil, DeleteOutput{}, err
		}
		return nil, DeleteOutput{Success: true, Message: "share rejected"}, nil
	}
}

func handleGetSharingRoles(c *client.Client) mcp.ToolHandlerFor[GetSharingRolesInput, SharingRolesOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input GetSharingRolesInput) (*mcp.CallToolResult, SharingRolesOutput, error) {
		if err := client.ValidateID("space_id", input.SpaceID); err != nil {
			return nil, SharingRolesOutput{}, err
		}
		path := fmt.Sprintf("/graph/v1.0/drives/%s/root/permissions/roles", url.PathEscape(input.SpaceID))
		roles, err := client.ListJSON[SharingRole](ctx, c, path, nil)
		if err != nil {
			return nil, SharingRolesOutput{}, err
		}
		return nil, SharingRolesOutput{Roles: roles}, nil
	}
}
