package tools

import (
	"context"
	"fmt"
	"net/url"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/owncloud/ocis-mcp-server/internal/client"
)

// User represents a LibreGraph user.
type User struct {
	ID                          string           `json:"id"`
	DisplayName                 string           `json:"displayName"`
	Mail                        string           `json:"mail,omitempty"`
	OnPremisesSamAccountName    string           `json:"onPremisesSamAccountName,omitempty"`
	AccountEnabled              *bool            `json:"accountEnabled,omitempty"`
	UserType                    string           `json:"userType,omitempty"`
	Surname                     string           `json:"surname,omitempty"`
	GivenName                   string           `json:"givenName,omitempty"`
	PreferredLanguage           string           `json:"preferredLanguage,omitempty"`
	MemberOf                    []GroupRef       `json:"memberOf,omitempty"`
	Drive                       *DriveRef        `json:"drive,omitempty"`
	Drives                      []DriveRef       `json:"drives,omitempty"`
	PasswordProfile             *PasswordProfile `json:"passwordProfile,omitempty"`
	AppRoleAssignments          []AppRoleAssignment `json:"appRoleAssignments,omitempty"`
}

// GroupRef is a minimal group reference.
type GroupRef struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName,omitempty"`
}

// DriveRef is a minimal drive reference.
type DriveRef struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

// PasswordProfile for user creation/update.
type PasswordProfile struct {
	Password string `json:"password,omitempty"`
}

// AppRoleAssignment represents a role assignment.
type AppRoleAssignment struct {
	ID          string `json:"id"`
	AppRoleID   string `json:"appRoleId"`
	PrincipalID string `json:"principalId,omitempty"`
}

// --- Input/Output types ---

type ListUsersInput struct {
	Search  string `json:"search,omitempty" jsonschema:"Search term to filter users by name or email"`
	OrderBy string `json:"orderby,omitempty" jsonschema:"OData orderby expression (e.g. displayName)"`
	Filter  string `json:"filter,omitempty" jsonschema:"OData filter expression"`
	Limit   int    `json:"limit,omitempty" jsonschema:"Maximum number of results (default 50, max 200)"`
	Offset  int    `json:"offset,omitempty" jsonschema:"Number of results to skip"`
}

type ListUsersOutput struct {
	Users      []User `json:"users"`
	TotalCount int    `json:"total_count"`
	HasMore    bool   `json:"has_more"`
	NextOffset int    `json:"next_offset"`
}

type GetUserInput struct {
	UserID string `json:"user_id" jsonschema:"User ID or username,required"`
}

type GetUserOutput struct {
	User
}

type CreateUserInput struct {
	DisplayName              string `json:"display_name" jsonschema:"Display name for the user,required"`
	OnPremisesSamAccountName string `json:"username" jsonschema:"Username (onPremisesSamAccountName),required"`
	Mail                     string `json:"mail" jsonschema:"Email address,required"`
	Password                 string `json:"password" jsonschema:"Initial password,required"`
}

type UpdateUserInput struct {
	UserID      string  `json:"user_id" jsonschema:"User ID to update,required"`
	DisplayName *string `json:"display_name,omitempty" jsonschema:"New display name"`
	Mail        *string `json:"mail,omitempty" jsonschema:"New email address"`
	Surname     *string `json:"surname,omitempty" jsonschema:"New surname"`
	GivenName   *string `json:"given_name,omitempty" jsonschema:"New given name"`
	Password    *string `json:"password,omitempty" jsonschema:"New password"`
	Enabled     *bool   `json:"enabled,omitempty" jsonschema:"Enable or disable the account"`
}

type DeleteUserInput struct {
	UserID  string `json:"user_id" jsonschema:"User ID to delete,required"`
	Confirm bool   `json:"confirm" jsonschema:"Must be true to confirm deletion"`
}

type DeleteOutput struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type GetMeInput struct{}

// --- Registration ---

func registerUsers(s *mcp.Server, c *client.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_list_users",
		Description: "List all users with optional filtering, search, and pagination. Returns user IDs, display names, emails, and account status.",
		Annotations: readOnlyAnnotations(),
	}, handleListUsers(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_get_user",
		Description: "Get a single user by ID or username. Returns full user profile including group memberships and drive info.",
		Annotations: readOnlyAnnotations(),
	}, handleGetUser(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_create_user",
		Description: "Create a new user account with display name, username, email, and password.",
		Annotations: mutatingAnnotations(),
	}, handleCreateUser(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_update_user",
		Description: "Update user attributes such as display name, email, password, or account enabled status.",
		Annotations: idempotentAnnotations(),
	}, handleUpdateUser(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_delete_user",
		Description: "Permanently delete a user account. Requires confirm=true.",
		Annotations: destructiveAnnotations(),
	}, handleDeleteUser(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_get_me",
		Description: "Get the currently authenticated user's profile.",
		Annotations: readOnlyAnnotations(),
	}, handleGetMe(c))
}

// --- Handlers ---

func handleListUsers(c *client.Client) mcp.ToolHandlerFor[ListUsersInput, ListUsersOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input ListUsersInput) (*mcp.CallToolResult, ListUsersOutput, error) {
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

		users, err := client.ListJSON[User](ctx, c, "/graph/v1.0/users", q)
		if err != nil {
			return nil, ListUsersOutput{}, err
		}

		return nil, ListUsersOutput{
			Users:      users,
			TotalCount: len(users),
			HasMore:    len(users) == limit,
			NextOffset: input.Offset + len(users),
		}, nil
	}
}

func handleGetUser(c *client.Client) mcp.ToolHandlerFor[GetUserInput, GetUserOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input GetUserInput) (*mcp.CallToolResult, GetUserOutput, error) {
		if err := client.ValidateID("user_id", input.UserID); err != nil {
			return nil, GetUserOutput{}, err
		}
		user, err := client.GetJSON[User](ctx, c, "/graph/v1.0/users/"+url.PathEscape(input.UserID), nil)
		if err != nil {
			return nil, GetUserOutput{}, err
		}
		return nil, GetUserOutput{User: user}, nil
	}
}

func handleCreateUser(c *client.Client) mcp.ToolHandlerFor[CreateUserInput, GetUserOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input CreateUserInput) (*mcp.CallToolResult, GetUserOutput, error) {
		body := map[string]any{
			"displayName":              input.DisplayName,
			"onPremisesSamAccountName": input.OnPremisesSamAccountName,
			"mail":                     input.Mail,
			"passwordProfile": map[string]string{
				"password": input.Password,
			},
		}
		user, err := client.PostJSON[User](ctx, c, "/graph/v1.0/users", body)
		if err != nil {
			return nil, GetUserOutput{}, err
		}
		return nil, GetUserOutput{User: user}, nil
	}
}

func handleUpdateUser(c *client.Client) mcp.ToolHandlerFor[UpdateUserInput, GetUserOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input UpdateUserInput) (*mcp.CallToolResult, GetUserOutput, error) {
		if err := client.ValidateID("user_id", input.UserID); err != nil {
			return nil, GetUserOutput{}, err
		}
		body := make(map[string]any)
		if input.DisplayName != nil {
			body["displayName"] = *input.DisplayName
		}
		if input.Mail != nil {
			body["mail"] = *input.Mail
		}
		if input.Surname != nil {
			body["surname"] = *input.Surname
		}
		if input.GivenName != nil {
			body["givenName"] = *input.GivenName
		}
		if input.Password != nil {
			body["passwordProfile"] = map[string]string{"password": *input.Password}
		}
		if input.Enabled != nil {
			body["accountEnabled"] = *input.Enabled
		}
		user, err := client.PatchJSON[User](ctx, c, "/graph/v1.0/users/"+url.PathEscape(input.UserID), body)
		if err != nil {
			return nil, GetUserOutput{}, err
		}
		return nil, GetUserOutput{User: user}, nil
	}
}

func handleDeleteUser(c *client.Client) mcp.ToolHandlerFor[DeleteUserInput, DeleteOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input DeleteUserInput) (*mcp.CallToolResult, DeleteOutput, error) {
		if !input.Confirm {
			return nil, DeleteOutput{}, fmt.Errorf("this is a destructive operation. Set confirm=true to proceed")
		}
		if err := client.ValidateID("user_id", input.UserID); err != nil {
			return nil, DeleteOutput{}, err
		}
		if err := client.Delete(ctx, c, "/graph/v1.0/users/"+url.PathEscape(input.UserID)); err != nil {
			return nil, DeleteOutput{}, err
		}
		return nil, DeleteOutput{Success: true, Message: "user deleted"}, nil
	}
}

func handleGetMe(c *client.Client) mcp.ToolHandlerFor[GetMeInput, GetUserOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, _ GetMeInput) (*mcp.CallToolResult, GetUserOutput, error) {
		user, err := client.GetJSON[User](ctx, c, "/graph/v1.0/me", nil)
		if err != nil {
			return nil, GetUserOutput{}, err
		}
		return nil, GetUserOutput{User: user}, nil
	}
}
