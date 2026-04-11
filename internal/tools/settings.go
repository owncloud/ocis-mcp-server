package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/owncloud/ocis-mcp-server/internal/client"
)

// SettingsRole represents a role from the settings service.
type SettingsRole struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"displayName,omitempty"`
	Description string `json:"description,omitempty"`
}

// SettingsRolesResponse wraps the roles list response.
type SettingsRolesResponse struct {
	Bundles []SettingsRole `json:"bundles"`
}

// SettingsAssignment represents a role assignment.
type SettingsAssignment struct {
	ID          string `json:"id,omitempty"`
	AccountUUID string `json:"accountUuid"`
	RoleID      string `json:"roleId"`
}

// SettingsAssignmentsResponse wraps the assignments list response.
type SettingsAssignmentsResponse struct {
	Assignments []SettingsAssignment `json:"assignments"`
}

// --- Input/Output types ---

type ListRolesInput struct{}

type ListRolesOutput struct {
	Roles []SettingsRole `json:"roles"`
}

type AssignRoleInput struct {
	UserID string `json:"user_id" jsonschema:"Account UUID to assign the role to,required"`
	RoleID string `json:"role_id" jsonschema:"Role ID to assign,required"`
}

type AssignRoleOutput struct {
	Assignment SettingsAssignment `json:"assignment"`
}

type ListAssignmentsInput struct{}

type ListAssignmentsOutput struct {
	Assignments []SettingsAssignment `json:"assignments"`
}

// --- Registration ---

func registerSettings(s *mcp.Server, c *client.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_list_roles",
		Description: "List all available roles in the oCIS settings service. Returns role IDs, names, and descriptions.",
		Annotations: readOnlyAnnotations(),
	}, handleListRoles(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_assign_role",
		Description: "Assign a role to a user account. Requires account UUID and role ID.",
		Annotations: mutatingAnnotations(),
	}, handleAssignRole(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_list_assignments",
		Description: "List all current role assignments. Returns account-to-role mappings.",
		Annotations: readOnlyAnnotations(),
	}, handleListAssignments(c))
}

// --- Handlers ---

func handleListRoles(c *client.Client) mcp.ToolHandlerFor[ListRolesInput, ListRolesOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, _ ListRolesInput) (*mcp.CallToolResult, ListRolesOutput, error) {
		resp, err := client.PostJSON[SettingsRolesResponse](ctx, c, "/api/v0/settings/roles-list", map[string]any{})
		if err != nil {
			return nil, ListRolesOutput{}, err
		}
		roles := resp.Bundles
		if roles == nil {
			roles = []SettingsRole{}
		}
		return nil, ListRolesOutput{Roles: roles}, nil
	}
}

func handleAssignRole(c *client.Client) mcp.ToolHandlerFor[AssignRoleInput, AssignRoleOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input AssignRoleInput) (*mcp.CallToolResult, AssignRoleOutput, error) {
		if err := client.ValidateID("user_id", input.UserID); err != nil {
			return nil, AssignRoleOutput{}, err
		}
		if err := client.ValidateID("role_id", input.RoleID); err != nil {
			return nil, AssignRoleOutput{}, err
		}
		body := map[string]string{
			"account_uuid": input.UserID,
			"role_id":      input.RoleID,
		}
		resp, err := client.PostJSON[SettingsAssignment](ctx, c, "/api/v0/settings/assignments-add", body)
		if err != nil {
			return nil, AssignRoleOutput{}, err
		}
		return nil, AssignRoleOutput{Assignment: resp}, nil
	}
}

func handleListAssignments(c *client.Client) mcp.ToolHandlerFor[ListAssignmentsInput, ListAssignmentsOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, _ ListAssignmentsInput) (*mcp.CallToolResult, ListAssignmentsOutput, error) {
		resp, err := client.PostJSON[SettingsAssignmentsResponse](ctx, c, "/api/v0/settings/assignments-list", map[string]any{})
		if err != nil {
			return nil, ListAssignmentsOutput{}, err
		}
		assignments := resp.Assignments
		if assignments == nil {
			assignments = []SettingsAssignment{}
		}
		return nil, ListAssignmentsOutput{Assignments: assignments}, nil
	}
}
