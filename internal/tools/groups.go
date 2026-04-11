package tools

import (
	"context"
	"fmt"
	"net/url"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/owncloud/ocis-mcp-server/internal/client"
)

// Group represents a LibreGraph group.
type Group struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	Description string `json:"description,omitempty"`
	GroupTypes  []string `json:"groupTypes,omitempty"`
	Members     []User   `json:"members,omitempty"`
}

// --- Input/Output types ---

type ListGroupsInput struct {
	Search  string `json:"search,omitempty" jsonschema:"Search term to filter groups"`
	OrderBy string `json:"orderby,omitempty" jsonschema:"OData orderby expression"`
	Filter  string `json:"filter,omitempty" jsonschema:"OData filter expression"`
	Limit   int    `json:"limit,omitempty" jsonschema:"Maximum number of results (default 50, max 200)"`
	Offset  int    `json:"offset,omitempty" jsonschema:"Number of results to skip"`
}

type ListGroupsOutput struct {
	Groups     []Group `json:"groups"`
	TotalCount int     `json:"total_count"`
	HasMore    bool    `json:"has_more"`
	NextOffset int     `json:"next_offset"`
}

type GetGroupInput struct {
	GroupID string `json:"group_id" jsonschema:"Group ID,required"`
}

type GetGroupOutput struct {
	Group
}

type CreateGroupInput struct {
	DisplayName string `json:"display_name" jsonschema:"Group display name,required"`
	Description string `json:"description,omitempty" jsonschema:"Group description"`
}

type UpdateGroupInput struct {
	GroupID     string  `json:"group_id" jsonschema:"Group ID to update,required"`
	DisplayName *string `json:"display_name,omitempty" jsonschema:"New display name"`
	Description *string `json:"description,omitempty" jsonschema:"New description"`
}

type DeleteGroupInput struct {
	GroupID string `json:"group_id" jsonschema:"Group ID to delete,required"`
	Confirm bool   `json:"confirm" jsonschema:"Must be true to confirm deletion"`
}

type AddGroupMemberInput struct {
	GroupID string `json:"group_id" jsonschema:"Group ID,required"`
	UserID  string `json:"user_id" jsonschema:"User ID to add,required"`
}

type RemoveGroupMemberInput struct {
	GroupID string `json:"group_id" jsonschema:"Group ID,required"`
	UserID  string `json:"user_id" jsonschema:"User ID to remove,required"`
	Confirm bool   `json:"confirm" jsonschema:"Must be true to confirm removal"`
}

// --- Registration ---

func registerGroups(s *mcp.Server, c *client.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_list_groups",
		Description: "List all groups with optional search and pagination.",
		Annotations: readOnlyAnnotations(),
	}, handleListGroups(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_get_group",
		Description: "Get group details by ID including member list.",
		Annotations: readOnlyAnnotations(),
	}, handleGetGroup(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_create_group",
		Description: "Create a new group with a display name and optional description.",
		Annotations: mutatingAnnotations(),
	}, handleCreateGroup(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_update_group",
		Description: "Update group attributes such as display name or description.",
		Annotations: idempotentAnnotations(),
	}, handleUpdateGroup(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_delete_group",
		Description: "Permanently delete a group. Requires confirm=true.",
		Annotations: destructiveAnnotations(),
	}, handleDeleteGroup(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_add_group_member",
		Description: "Add a user to a group by user ID.",
		Annotations: mutatingAnnotations(),
	}, handleAddGroupMember(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_remove_group_member",
		Description: "Remove a user from a group. Requires confirm=true.",
		Annotations: destructiveAnnotations(),
	}, handleRemoveGroupMember(c))
}

// --- Handlers ---

func handleListGroups(c *client.Client) mcp.ToolHandlerFor[ListGroupsInput, ListGroupsOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input ListGroupsInput) (*mcp.CallToolResult, ListGroupsOutput, error) {
		limit := client.ValidateLimit(input.Limit)
		q := url.Values{}
		if input.Search != "" {
			q.Set("$search", input.Search)
		}
		if input.OrderBy != "" {
			q.Set("$orderby", input.OrderBy)
		}
		if input.Filter != "" {
			q.Set("$filter", input.Filter)
		}
		q.Set("$top", fmt.Sprintf("%d", limit))
		if input.Offset > 0 {
			q.Set("$skip", fmt.Sprintf("%d", input.Offset))
		}

		groups, err := client.ListJSON[Group](ctx, c, "/graph/v1.0/groups", q)
		if err != nil {
			return nil, ListGroupsOutput{}, err
		}
		return nil, ListGroupsOutput{
			Groups:     groups,
			TotalCount: len(groups),
			HasMore:    len(groups) == limit,
			NextOffset: input.Offset + len(groups),
		}, nil
	}
}

func handleGetGroup(c *client.Client) mcp.ToolHandlerFor[GetGroupInput, GetGroupOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input GetGroupInput) (*mcp.CallToolResult, GetGroupOutput, error) {
		if err := client.ValidateID("group_id", input.GroupID); err != nil {
			return nil, GetGroupOutput{}, err
		}
		q := url.Values{}
		q.Set("$expand", "members")
		group, err := client.GetJSON[Group](ctx, c, "/graph/v1.0/groups/"+url.PathEscape(input.GroupID), q)
		if err != nil {
			return nil, GetGroupOutput{}, err
		}
		return nil, GetGroupOutput{Group: group}, nil
	}
}

func handleCreateGroup(c *client.Client) mcp.ToolHandlerFor[CreateGroupInput, GetGroupOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input CreateGroupInput) (*mcp.CallToolResult, GetGroupOutput, error) {
		body := map[string]any{
			"displayName": input.DisplayName,
		}
		if input.Description != "" {
			body["description"] = input.Description
		}
		group, err := client.PostJSON[Group](ctx, c, "/graph/v1.0/groups", body)
		if err != nil {
			return nil, GetGroupOutput{}, err
		}
		return nil, GetGroupOutput{Group: group}, nil
	}
}

func handleUpdateGroup(c *client.Client) mcp.ToolHandlerFor[UpdateGroupInput, GetGroupOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input UpdateGroupInput) (*mcp.CallToolResult, GetGroupOutput, error) {
		if err := client.ValidateID("group_id", input.GroupID); err != nil {
			return nil, GetGroupOutput{}, err
		}
		body := make(map[string]any)
		if input.DisplayName != nil {
			body["displayName"] = *input.DisplayName
		}
		if input.Description != nil {
			body["description"] = *input.Description
		}
		group, err := client.PatchJSON[Group](ctx, c, "/graph/v1.0/groups/"+url.PathEscape(input.GroupID), body)
		if err != nil {
			return nil, GetGroupOutput{}, err
		}
		return nil, GetGroupOutput{Group: group}, nil
	}
}

func handleDeleteGroup(c *client.Client) mcp.ToolHandlerFor[DeleteGroupInput, DeleteOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input DeleteGroupInput) (*mcp.CallToolResult, DeleteOutput, error) {
		if !input.Confirm {
			return nil, DeleteOutput{}, fmt.Errorf("this is a destructive operation. Set confirm=true to proceed")
		}
		if err := client.ValidateID("group_id", input.GroupID); err != nil {
			return nil, DeleteOutput{}, err
		}
		if err := client.Delete(ctx, c, "/graph/v1.0/groups/"+url.PathEscape(input.GroupID)); err != nil {
			return nil, DeleteOutput{}, err
		}
		return nil, DeleteOutput{Success: true, Message: "group deleted"}, nil
	}
}

func handleAddGroupMember(c *client.Client) mcp.ToolHandlerFor[AddGroupMemberInput, DeleteOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input AddGroupMemberInput) (*mcp.CallToolResult, DeleteOutput, error) {
		if err := client.ValidateID("group_id", input.GroupID); err != nil {
			return nil, DeleteOutput{}, err
		}
		if err := client.ValidateID("user_id", input.UserID); err != nil {
			return nil, DeleteOutput{}, err
		}
		body := map[string]string{
			"@odata.id": input.UserID,
		}
		_, err := client.PostJSONRaw(ctx, c, "/graph/v1.0/groups/"+url.PathEscape(input.GroupID)+"/members/$ref", body)
		if err != nil {
			return nil, DeleteOutput{}, err
		}
		return nil, DeleteOutput{Success: true, Message: "member added"}, nil
	}
}

func handleRemoveGroupMember(c *client.Client) mcp.ToolHandlerFor[RemoveGroupMemberInput, DeleteOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input RemoveGroupMemberInput) (*mcp.CallToolResult, DeleteOutput, error) {
		if !input.Confirm {
			return nil, DeleteOutput{}, fmt.Errorf("this is a destructive operation. Set confirm=true to proceed")
		}
		if err := client.ValidateID("group_id", input.GroupID); err != nil {
			return nil, DeleteOutput{}, err
		}
		if err := client.ValidateID("user_id", input.UserID); err != nil {
			return nil, DeleteOutput{}, err
		}
		path := fmt.Sprintf("/graph/v1.0/groups/%s/members/%s/$ref",
			url.PathEscape(input.GroupID), url.PathEscape(input.UserID))
		if err := client.Delete(ctx, c, path); err != nil {
			return nil, DeleteOutput{}, err
		}
		return nil, DeleteOutput{Success: true, Message: "member removed"}, nil
	}
}
