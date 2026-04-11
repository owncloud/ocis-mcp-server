package tools

import (
	"context"
	"fmt"
	"net/url"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/owncloud/ocis-mcp-server/internal/client"
)

// EducationSchool represents a school in the education API.
type EducationSchool struct {
	ID              string `json:"id"`
	DisplayName     string `json:"displayName"`
	SchoolNumber    string `json:"schoolNumber,omitempty"`
	TerminationDate string `json:"terminationDate,omitempty"`
}

// EducationUser represents a user in the education API.
type EducationUser struct {
	ID          string           `json:"id"`
	DisplayName string           `json:"displayName"`
	Mail        string           `json:"mail,omitempty"`
	Username    string           `json:"onPremisesSamAccountName,omitempty"`
	PrimaryRole string           `json:"primaryRole,omitempty"`
	MemberOf    []EducationSchool `json:"memberOf,omitempty"`
}

// --- Input/Output types ---

type ListEducationSchoolsInput struct{}

type ListEducationSchoolsOutput struct {
	Schools []EducationSchool `json:"schools"`
}

type GetEducationSchoolInput struct {
	SchoolID string `json:"school_id" jsonschema:"Education school ID,required"`
}

type GetEducationSchoolOutput struct {
	EducationSchool
}

type ListEducationUsersInput struct{}

type ListEducationUsersOutput struct {
	Users []EducationUser `json:"users"`
}

type GetEducationUserInput struct {
	UserID string `json:"user_id" jsonschema:"Education user ID,required"`
}

type GetEducationUserOutput struct {
	EducationUser
}

type CreateEducationUserInput struct {
	DisplayName string `json:"display_name" jsonschema:"Display name for the education user,required"`
	Username    string `json:"username" jsonschema:"Username (onPremisesSamAccountName),required"`
	Mail        string `json:"mail" jsonschema:"Email address,required"`
	Password    string `json:"password" jsonschema:"Initial password,required"`
	PrimaryRole string `json:"primary_role,omitempty" jsonschema:"Primary role (e.g. student or teacher)"`
}

// --- Registration ---

func registerEducation(s *mcp.Server, c *client.Client, educationEnabled bool) {
	if !educationEnabled {
		return
	}

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_list_education_schools",
		Description: "List all education schools. Returns school IDs, display names, and school numbers.",
		Annotations: readOnlyAnnotations(),
	}, handleListEducationSchools(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_get_education_school",
		Description: "Get details of a specific education school by ID.",
		Annotations: readOnlyAnnotations(),
	}, handleGetEducationSchool(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_list_education_users",
		Description: "List all education users. Returns user IDs, display names, roles, and school memberships.",
		Annotations: readOnlyAnnotations(),
	}, handleListEducationUsers(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_get_education_user",
		Description: "Get details of a specific education user by ID.",
		Annotations: readOnlyAnnotations(),
	}, handleGetEducationUser(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_create_education_user",
		Description: "Create a new education user with display name, username, email, password, and optional primary role.",
		Annotations: mutatingAnnotations(),
	}, handleCreateEducationUser(c))
}

// --- Handlers ---

func handleListEducationSchools(c *client.Client) mcp.ToolHandlerFor[ListEducationSchoolsInput, ListEducationSchoolsOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, _ ListEducationSchoolsInput) (*mcp.CallToolResult, ListEducationSchoolsOutput, error) {
		schools, err := client.ListJSON[EducationSchool](ctx, c, "/graph/v1.0/education/schools", nil)
		if err != nil {
			return nil, ListEducationSchoolsOutput{}, err
		}
		if schools == nil {
			schools = []EducationSchool{}
		}
		return nil, ListEducationSchoolsOutput{Schools: schools}, nil
	}
}

func handleGetEducationSchool(c *client.Client) mcp.ToolHandlerFor[GetEducationSchoolInput, GetEducationSchoolOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input GetEducationSchoolInput) (*mcp.CallToolResult, GetEducationSchoolOutput, error) {
		if err := client.ValidateID("school_id", input.SchoolID); err != nil {
			return nil, GetEducationSchoolOutput{}, err
		}
		school, err := client.GetJSON[EducationSchool](ctx, c, "/graph/v1.0/education/schools/"+url.PathEscape(input.SchoolID), nil)
		if err != nil {
			return nil, GetEducationSchoolOutput{}, err
		}
		return nil, GetEducationSchoolOutput{EducationSchool: school}, nil
	}
}

func handleListEducationUsers(c *client.Client) mcp.ToolHandlerFor[ListEducationUsersInput, ListEducationUsersOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, _ ListEducationUsersInput) (*mcp.CallToolResult, ListEducationUsersOutput, error) {
		users, err := client.ListJSON[EducationUser](ctx, c, "/graph/v1.0/education/users", nil)
		if err != nil {
			return nil, ListEducationUsersOutput{}, err
		}
		if users == nil {
			users = []EducationUser{}
		}
		return nil, ListEducationUsersOutput{Users: users}, nil
	}
}

func handleGetEducationUser(c *client.Client) mcp.ToolHandlerFor[GetEducationUserInput, GetEducationUserOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input GetEducationUserInput) (*mcp.CallToolResult, GetEducationUserOutput, error) {
		if err := client.ValidateID("user_id", input.UserID); err != nil {
			return nil, GetEducationUserOutput{}, err
		}
		user, err := client.GetJSON[EducationUser](ctx, c, "/graph/v1.0/education/users/"+url.PathEscape(input.UserID), nil)
		if err != nil {
			return nil, GetEducationUserOutput{}, err
		}
		return nil, GetEducationUserOutput{EducationUser: user}, nil
	}
}

func handleCreateEducationUser(c *client.Client) mcp.ToolHandlerFor[CreateEducationUserInput, GetEducationUserOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input CreateEducationUserInput) (*mcp.CallToolResult, GetEducationUserOutput, error) {
		if err := client.ValidateID("display_name", input.DisplayName); err != nil {
			return nil, GetEducationUserOutput{}, fmt.Errorf("display_name is required")
		}
		if err := client.ValidateID("username", input.Username); err != nil {
			return nil, GetEducationUserOutput{}, fmt.Errorf("username is required")
		}
		if err := client.ValidateID("mail", input.Mail); err != nil {
			return nil, GetEducationUserOutput{}, fmt.Errorf("mail is required")
		}

		body := map[string]any{
			"displayName":              input.DisplayName,
			"onPremisesSamAccountName": input.Username,
			"mail":                     input.Mail,
			"passwordProfile": map[string]string{
				"password": input.Password,
			},
		}
		if input.PrimaryRole != "" {
			body["primaryRole"] = input.PrimaryRole
		}

		user, err := client.PostJSON[EducationUser](ctx, c, "/graph/v1.0/education/users", body)
		if err != nil {
			return nil, GetEducationUserOutput{}, err
		}
		return nil, GetEducationUserOutput{EducationUser: user}, nil
	}
}
