package tools

import (
	"context"
	"fmt"
	"net/url"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/owncloud/ocis-mcp-server/internal/client"
)

// Drive represents a LibreGraph drive (space).
type Drive struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	DriveType   string     `json:"driveType"`
	Description string     `json:"description,omitempty"`
	WebURL      string     `json:"webUrl,omitempty"`
	Owner       *Identity  `json:"owner,omitempty"`
	Quota       *DriveQuota `json:"quota,omitempty"`
	Root        *DriveItem  `json:"root,omitempty"`
	CreatedBy   *Identity  `json:"createdBy,omitempty"`
	Special     []DriveItem `json:"special,omitempty"`
}

// Identity represents an identity reference.
type Identity struct {
	User *IdentityUser `json:"user,omitempty"`
}

// IdentityUser is the user part of an identity.
type IdentityUser struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName,omitempty"`
}

// DriveQuota holds quota information.
type DriveQuota struct {
	Total     int64  `json:"total,omitempty"`
	Used      int64  `json:"used,omitempty"`
	Remaining int64  `json:"remaining,omitempty"`
	State     string `json:"state,omitempty"`
}

// DriveItem represents a file or folder in a drive.
type DriveItem struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
	ETag string `json:"eTag,omitempty"`
}

// Permission represents a sharing permission on a drive/item.
type Permission struct {
	ID                   string          `json:"id"`
	Roles                []string        `json:"roles,omitempty"`
	GrantedToV2          *GrantedTo      `json:"grantedToV2,omitempty"`
	Link                 *SharingLink    `json:"link,omitempty"`
	ExpirationDateTime   string          `json:"expirationDateTime,omitempty"`
	HasPassword          bool            `json:"hasPassword,omitempty"`
	CreatedDateTime      string          `json:"createdDateTime,omitempty"`
	Invitation           *Invitation     `json:"invitation,omitempty"`
}

// GrantedTo represents the identity a permission is granted to.
type GrantedTo struct {
	User  *IdentityUser `json:"user,omitempty"`
	Group *IdentityUser `json:"group,omitempty"`
}

// SharingLink represents a sharing link.
type SharingLink struct {
	Type     string `json:"type,omitempty"`
	WebURL   string `json:"webUrl,omitempty"`
	Password bool   `json:"password,omitempty"`
}

// Invitation represents a sharing invitation.
type Invitation struct {
	InvitedBy *Identity `json:"invitedBy,omitempty"`
}

// SharingRole represents a sharing role definition.
type SharingRole struct {
	ID          string   `json:"id"`
	DisplayName string   `json:"displayName"`
	Description string   `json:"description,omitempty"`
	Condition   string   `json:"condition,omitempty"`
	RolePermissions []RolePermission `json:"rolePermissions,omitempty"`
}

// RolePermission describes permissions in a role.
type RolePermission struct {
	Condition   string   `json:"condition,omitempty"`
	AllowedResourceActions []string `json:"allowedResourceActions,omitempty"`
}

// --- Input/Output types ---

type ListSpacesInput struct {
	Filter string `json:"filter,omitempty" jsonschema:"OData filter (e.g. driveType eq 'project')"`
	Limit  int    `json:"limit,omitempty" jsonschema:"Maximum results (default 50, max 200)"`
	Offset int    `json:"offset,omitempty" jsonschema:"Number of results to skip"`
}

type ListSpacesOutput struct {
	Spaces     []Drive `json:"spaces"`
	TotalCount int     `json:"total_count"`
	HasMore    bool    `json:"has_more"`
	NextOffset int     `json:"next_offset"`
}

type GetSpaceInput struct {
	SpaceID string `json:"space_id" jsonschema:"Drive/space ID,required"`
}

type GetSpaceOutput struct {
	Drive
}

type CreateSpaceInput struct {
	Name        string `json:"name" jsonschema:"Space name,required"`
	Description string `json:"description,omitempty" jsonschema:"Space description"`
	Quota       int64  `json:"quota,omitempty" jsonschema:"Quota in bytes"`
}

type UpdateSpaceInput struct {
	SpaceID     string  `json:"space_id" jsonschema:"Space ID to update,required"`
	Name        *string `json:"name,omitempty" jsonschema:"New name"`
	Description *string `json:"description,omitempty" jsonschema:"New description"`
	Quota       *int64  `json:"quota,omitempty" jsonschema:"New quota in bytes"`
}

type DisableSpaceInput struct {
	SpaceID string `json:"space_id" jsonschema:"Space ID to disable,required"`
	Confirm bool   `json:"confirm" jsonschema:"Must be true to confirm"`
}

type DeleteSpaceInput struct {
	SpaceID string `json:"space_id" jsonschema:"Space ID to permanently delete,required"`
	Confirm bool   `json:"confirm" jsonschema:"Must be true to confirm permanent deletion"`
}

type RestoreSpaceInput struct {
	SpaceID string `json:"space_id" jsonschema:"Space ID to restore,required"`
}

type InviteToSpaceInput struct {
	SpaceID    string   `json:"space_id" jsonschema:"Space ID,required"`
	Recipients []string `json:"recipients" jsonschema:"User or group IDs to invite,required"`
	Roles      []string `json:"roles" jsonschema:"Sharing role IDs to assign,required"`
}

type InviteOutput struct {
	Permissions []Permission `json:"permissions"`
}

type CreateSpaceLinkInput struct {
	SpaceID  string `json:"space_id" jsonschema:"Space ID,required"`
	Type     string `json:"type,omitempty" jsonschema:"Link type: view or edit (default: view)"`
	Password string `json:"password,omitempty" jsonschema:"Optional password for the link"`
}

type ListSpacePermissionsInput struct {
	SpaceID string `json:"space_id" jsonschema:"Space ID,required"`
}

type ListPermissionsOutput struct {
	Permissions []Permission `json:"permissions"`
}

type EmptyTrashbinInput struct {
	SpaceID string `json:"space_id" jsonschema:"Space ID whose trashbin to empty,required"`
	Confirm bool   `json:"confirm" jsonschema:"Must be true to confirm"`
}

type SetSpaceSpecialInput struct {
	SpaceID string `json:"space_id" jsonschema:"Space ID,required"`
	Content string `json:"content" jsonschema:"File content (text),required"`
}

// --- Registration ---

func registerSpaces(s *mcp.Server, c *client.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_list_spaces",
		Description: "List all spaces/drives visible to the admin. Use filter to narrow by driveType (personal, project, shares, virtual).",
		Annotations: readOnlyAnnotations(),
	}, handleListSpaces(c, "/graph/v1.0/drives"))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_list_my_spaces",
		Description: "List spaces/drives the current user has access to.",
		Annotations: readOnlyAnnotations(),
	}, handleListSpaces(c, "/graph/v1.0/me/drives"))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_get_space",
		Description: "Get details of a specific space/drive by ID including quota, owner, and root item.",
		Annotations: readOnlyAnnotations(),
	}, handleGetSpace(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_create_space",
		Description: "Create a new project space with a name, optional description, and optional quota.",
		Annotations: mutatingAnnotations(),
	}, handleCreateSpace(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_update_space",
		Description: "Update space name, description, or quota.",
		Annotations: idempotentAnnotations(),
	}, handleUpdateSpace(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_disable_space",
		Description: "Soft-delete (disable) a space. Can be restored later. Requires confirm=true.",
		Annotations: destructiveAnnotations(),
	}, handleDisableSpace(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_delete_space",
		Description: "Permanently delete a disabled space. Cannot be undone. Requires confirm=true.",
		Annotations: destructiveAnnotations(),
	}, handleDeleteSpace(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_restore_space",
		Description: "Restore a previously disabled space.",
		Annotations: idempotentAnnotations(),
	}, handleRestoreSpace(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_invite_to_space",
		Description: "Invite users or groups to a space with specified roles.",
		Annotations: mutatingAnnotations(),
	}, handleInviteToSpace(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_create_space_link",
		Description: "Create a public sharing link for a space.",
		Annotations: mutatingAnnotations(),
	}, handleCreateSpaceLink(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_list_space_permissions",
		Description: "List all permissions (shares) on a space's root.",
		Annotations: readOnlyAnnotations(),
	}, handleListSpacePermissions(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_empty_trashbin",
		Description: "Permanently empty the trashbin of a space. Cannot be undone. Requires confirm=true.",
		Annotations: destructiveAnnotations(),
	}, handleEmptyTrashbin(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_set_space_image",
		Description: "Set the space avatar/image by uploading text content to the .space folder.",
		Annotations: idempotentAnnotations(),
	}, handleSetSpaceImage(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_set_space_readme",
		Description: "Set the space readme/description file.",
		Annotations: idempotentAnnotations(),
	}, handleSetSpaceReadme(c))
}

// --- Handlers ---

func handleListSpaces(c *client.Client, basePath string) mcp.ToolHandlerFor[ListSpacesInput, ListSpacesOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input ListSpacesInput) (*mcp.CallToolResult, ListSpacesOutput, error) {
		limit := client.ValidateLimit(input.Limit)
		q := url.Values{}
		if input.Filter != "" {
			q.Set("$filter", input.Filter)
		}
		q.Set("$top", fmt.Sprintf("%d", limit))
		if input.Offset > 0 {
			q.Set("$skip", fmt.Sprintf("%d", input.Offset))
		}
		drives, err := client.ListJSON[Drive](ctx, c, basePath, q)
		if err != nil {
			return nil, ListSpacesOutput{}, err
		}
		return nil, ListSpacesOutput{
			Spaces:     drives,
			TotalCount: len(drives),
			HasMore:    len(drives) == limit,
			NextOffset: input.Offset + len(drives),
		}, nil
	}
}

func handleGetSpace(c *client.Client) mcp.ToolHandlerFor[GetSpaceInput, GetSpaceOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input GetSpaceInput) (*mcp.CallToolResult, GetSpaceOutput, error) {
		if err := client.ValidateID("space_id", input.SpaceID); err != nil {
			return nil, GetSpaceOutput{}, err
		}
		drive, err := client.GetJSON[Drive](ctx, c, "/graph/v1.0/drives/"+url.PathEscape(input.SpaceID), nil)
		if err != nil {
			return nil, GetSpaceOutput{}, err
		}
		return nil, GetSpaceOutput{Drive: drive}, nil
	}
}

func handleCreateSpace(c *client.Client) mcp.ToolHandlerFor[CreateSpaceInput, GetSpaceOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input CreateSpaceInput) (*mcp.CallToolResult, GetSpaceOutput, error) {
		body := map[string]any{
			"name":      input.Name,
			"driveType": "project",
		}
		if input.Description != "" {
			body["description"] = input.Description
		}
		if input.Quota > 0 {
			body["quota"] = map[string]int64{"total": input.Quota}
		}
		drive, err := client.PostJSON[Drive](ctx, c, "/graph/v1.0/drives", body)
		if err != nil {
			return nil, GetSpaceOutput{}, err
		}
		return nil, GetSpaceOutput{Drive: drive}, nil
	}
}

func handleUpdateSpace(c *client.Client) mcp.ToolHandlerFor[UpdateSpaceInput, GetSpaceOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input UpdateSpaceInput) (*mcp.CallToolResult, GetSpaceOutput, error) {
		if err := client.ValidateID("space_id", input.SpaceID); err != nil {
			return nil, GetSpaceOutput{}, err
		}
		body := make(map[string]any)
		if input.Name != nil {
			body["name"] = *input.Name
		}
		if input.Description != nil {
			body["description"] = *input.Description
		}
		if input.Quota != nil {
			body["quota"] = map[string]int64{"total": *input.Quota}
		}
		drive, err := client.PatchJSON[Drive](ctx, c, "/graph/v1.0/drives/"+url.PathEscape(input.SpaceID), body)
		if err != nil {
			return nil, GetSpaceOutput{}, err
		}
		return nil, GetSpaceOutput{Drive: drive}, nil
	}
}

func handleDisableSpace(c *client.Client) mcp.ToolHandlerFor[DisableSpaceInput, DeleteOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input DisableSpaceInput) (*mcp.CallToolResult, DeleteOutput, error) {
		if !input.Confirm {
			return nil, DeleteOutput{}, fmt.Errorf("this is a destructive operation. Set confirm=true to proceed")
		}
		if err := client.ValidateID("space_id", input.SpaceID); err != nil {
			return nil, DeleteOutput{}, err
		}
		if err := client.Delete(ctx, c, "/graph/v1.0/drives/"+url.PathEscape(input.SpaceID)); err != nil {
			return nil, DeleteOutput{}, err
		}
		return nil, DeleteOutput{Success: true, Message: "space disabled"}, nil
	}
}

func handleDeleteSpace(c *client.Client) mcp.ToolHandlerFor[DeleteSpaceInput, DeleteOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input DeleteSpaceInput) (*mcp.CallToolResult, DeleteOutput, error) {
		if !input.Confirm {
			return nil, DeleteOutput{}, fmt.Errorf("this is a destructive operation. Set confirm=true to proceed")
		}
		if err := client.ValidateID("space_id", input.SpaceID); err != nil {
			return nil, DeleteOutput{}, err
		}
		err := client.DeleteWithHeaders(ctx, c, "/graph/v1.0/drives/"+url.PathEscape(input.SpaceID), map[string]string{
			"Purge": "T",
		})
		if err != nil {
			return nil, DeleteOutput{}, err
		}
		return nil, DeleteOutput{Success: true, Message: "space permanently deleted"}, nil
	}
}

func handleRestoreSpace(c *client.Client) mcp.ToolHandlerFor[RestoreSpaceInput, GetSpaceOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input RestoreSpaceInput) (*mcp.CallToolResult, GetSpaceOutput, error) {
		if err := client.ValidateID("space_id", input.SpaceID); err != nil {
			return nil, GetSpaceOutput{}, err
		}
		body := map[string]any{
			"@UI.Hidden": false,
		}
		drive, err := client.PatchJSON[Drive](ctx, c, "/graph/v1.0/drives/"+url.PathEscape(input.SpaceID), body)
		if err != nil {
			return nil, GetSpaceOutput{}, err
		}
		return nil, GetSpaceOutput{Drive: drive}, nil
	}
}

func handleInviteToSpace(c *client.Client) mcp.ToolHandlerFor[InviteToSpaceInput, InviteOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input InviteToSpaceInput) (*mcp.CallToolResult, InviteOutput, error) {
		if err := client.ValidateID("space_id", input.SpaceID); err != nil {
			return nil, InviteOutput{}, err
		}
		recipients := make([]map[string]any, len(input.Recipients))
		for i, r := range input.Recipients {
			recipients[i] = map[string]any{
				"objectId": r,
			}
		}
		body := map[string]any{
			"recipients": recipients,
			"roles":      input.Roles,
		}
		path := fmt.Sprintf("/graph/v1beta1/drives/%s/root/invite", url.PathEscape(input.SpaceID))
		result, err := client.PostJSON[InviteOutput](ctx, c, path, body)
		if err != nil {
			return nil, InviteOutput{}, err
		}
		return nil, result, nil
	}
}

func handleCreateSpaceLink(c *client.Client) mcp.ToolHandlerFor[CreateSpaceLinkInput, Permission] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input CreateSpaceLinkInput) (*mcp.CallToolResult, Permission, error) {
		if err := client.ValidateID("space_id", input.SpaceID); err != nil {
			return nil, Permission{}, err
		}
		linkType := input.Type
		if linkType == "" {
			linkType = "view"
		}
		body := map[string]any{
			"type": linkType,
		}
		if input.Password != "" {
			body["password"] = input.Password
		}
		path := fmt.Sprintf("/graph/v1beta1/drives/%s/root/createLink", url.PathEscape(input.SpaceID))
		perm, err := client.PostJSON[Permission](ctx, c, path, body)
		if err != nil {
			return nil, Permission{}, err
		}
		return nil, perm, nil
	}
}

func handleListSpacePermissions(c *client.Client) mcp.ToolHandlerFor[ListSpacePermissionsInput, ListPermissionsOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input ListSpacePermissionsInput) (*mcp.CallToolResult, ListPermissionsOutput, error) {
		if err := client.ValidateID("space_id", input.SpaceID); err != nil {
			return nil, ListPermissionsOutput{}, err
		}
		path := fmt.Sprintf("/graph/v1beta1/drives/%s/root/permissions", url.PathEscape(input.SpaceID))
		perms, err := client.ListJSON[Permission](ctx, c, path, nil)
		if err != nil {
			return nil, ListPermissionsOutput{}, err
		}
		return nil, ListPermissionsOutput{Permissions: perms}, nil
	}
}

func handleEmptyTrashbin(c *client.Client) mcp.ToolHandlerFor[EmptyTrashbinInput, DeleteOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input EmptyTrashbinInput) (*mcp.CallToolResult, DeleteOutput, error) {
		if !input.Confirm {
			return nil, DeleteOutput{}, fmt.Errorf("this is a destructive operation. Set confirm=true to proceed")
		}
		if err := client.ValidateID("space_id", input.SpaceID); err != nil {
			return nil, DeleteOutput{}, err
		}
		path := fmt.Sprintf("/dav/spaces/%s/.trash/", url.PathEscape(input.SpaceID))
		if err := client.WebDAVDelete(ctx, c, path); err != nil {
			return nil, DeleteOutput{}, err
		}
		return nil, DeleteOutput{Success: true, Message: "trashbin emptied"}, nil
	}
}

func handleSetSpaceImage(c *client.Client) mcp.ToolHandlerFor[SetSpaceSpecialInput, DeleteOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input SetSpaceSpecialInput) (*mcp.CallToolResult, DeleteOutput, error) {
		if err := client.ValidateID("space_id", input.SpaceID); err != nil {
			return nil, DeleteOutput{}, err
		}
		path := fmt.Sprintf("/dav/spaces/%s/.space/image", url.PathEscape(input.SpaceID))
		if err := client.Upload(ctx, c, path, nil, ""); err != nil {
			return nil, DeleteOutput{}, fmt.Errorf("uploading space image: %w", err)
		}
		return nil, DeleteOutput{Success: true, Message: "space image set"}, nil
	}
}

func handleSetSpaceReadme(c *client.Client) mcp.ToolHandlerFor[SetSpaceSpecialInput, DeleteOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input SetSpaceSpecialInput) (*mcp.CallToolResult, DeleteOutput, error) {
		if err := client.ValidateID("space_id", input.SpaceID); err != nil {
			return nil, DeleteOutput{}, err
		}
		path := fmt.Sprintf("/dav/spaces/%s/.space/readme.md", url.PathEscape(input.SpaceID))
		if err := client.Upload(ctx, c, path, nil, "text/markdown"); err != nil {
			return nil, DeleteOutput{}, fmt.Errorf("uploading space readme: %w", err)
		}
		return nil, DeleteOutput{Success: true, Message: "space readme set"}, nil
	}
}
