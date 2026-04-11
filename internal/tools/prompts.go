package tools

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerPrompts(s *mcp.Server) {
	s.AddPrompt(
		&mcp.Prompt{
			Name:        "ocis_onboard_user",
			Description: "Guide: create a user, assign a role, and add to project spaces",
			Arguments: []*mcp.PromptArgument{
				{Name: "username", Description: "Username for the new user", Required: true},
				{Name: "email", Description: "Email address for the new user", Required: true},
				{Name: "space_names", Description: "Comma-separated project space names to add the user to"},
			},
		},
		func(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			username := req.Params.Arguments["username"]
			email := req.Params.Arguments["email"]
			spaceNames := req.Params.Arguments["space_names"]

			prompt := fmt.Sprintf(
				"Onboard a new user with username %q and email %q. Follow these steps:\n"+
					"1) Use ocis_create_user to create the account (generate a secure password)\n"+
					"2) Use ocis_list_roles to find the appropriate role, then ocis_assign_role\n"+
					"3) Use ocis_get_me to verify the user was created correctly",
				username, email)

			if spaceNames != "" {
				prompt += fmt.Sprintf(
					"\n4) For each space in [%s]: use ocis_list_spaces to find it by name, "+
						"then use ocis_invite_to_space to add the user with an appropriate role\n"+
						"5) Summarize what was done: user created, role assigned, spaces joined",
					spaceNames)
			}

			return &mcp.GetPromptResult{
				Messages: []*mcp.PromptMessage{
					{Role: "user", Content: &mcp.TextContent{Text: prompt}},
				},
			}, nil
		},
	)

	s.AddPrompt(
		&mcp.Prompt{
			Name:        "ocis_migrate_files",
			Description: "Guide: list files in source space, copy to destination, verify",
			Arguments: []*mcp.PromptArgument{
				{Name: "source_space_id", Description: "Source drive/space ID", Required: true},
				{Name: "dest_space_id", Description: "Destination drive/space ID", Required: true},
				{Name: "path", Description: "Path within source to migrate (default: root)"},
			},
		},
		func(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			src := req.Params.Arguments["source_space_id"]
			dst := req.Params.Arguments["dest_space_id"]
			path := req.Params.Arguments["path"]
			if path == "" {
				path = "/"
			}

			prompt := fmt.Sprintf(
				"Migrate files from space %q (path: %s) to space %q. Follow these steps:\n"+
					"1) Use ocis_list_files on the source space at path %q to see all files and folders\n"+
					"2) For each folder, use ocis_create_folder in the destination space\n"+
					"3) For each file, use ocis_download_file from source, then ocis_upload_file to destination\n"+
					"4) After migration, use ocis_list_files on the destination to verify all files were copied\n"+
					"5) Compare file counts and report any discrepancies",
				src, path, dst, path)

			return &mcp.GetPromptResult{
				Messages: []*mcp.PromptMessage{
					{Role: "user", Content: &mcp.TextContent{Text: prompt}},
				},
			}, nil
		},
	)

	s.AddPrompt(
		&mcp.Prompt{
			Name:        "ocis_audit_space",
			Description: "Audit a space: metadata, permissions, recent activity",
			Arguments: []*mcp.PromptArgument{
				{Name: "space_id", Description: "The drive/space ID to audit", Required: true},
			},
		},
		func(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			spaceID := req.Params.Arguments["space_id"]

			prompt := fmt.Sprintf(
				"Audit space %q comprehensively. Follow these steps:\n"+
					"1) Use ocis_get_space to get space metadata (name, owner, quota usage)\n"+
					"2) Use ocis_list_space_permissions to list all permissions — identify who has access and what roles\n"+
					"3) Use ocis_list_files at the root to see what's stored\n"+
					"4) Summarize findings: space name, owner, quota used/total, number of files, "+
					"who has access (with role names), and flag any concerns (over-sharing, near-quota, etc.)",
				spaceID)

			return &mcp.GetPromptResult{
				Messages: []*mcp.PromptMessage{
					{Role: "user", Content: &mcp.TextContent{Text: prompt}},
				},
			}, nil
		},
	)

	s.AddPrompt(
		&mcp.Prompt{
			Name:        "ocis_share_report",
			Description: "Generate a sharing report for a user: shares created and received",
			Arguments: []*mcp.PromptArgument{
				{Name: "user_id", Description: "User ID to generate share report for", Required: true},
			},
		},
		func(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			userID := req.Params.Arguments["user_id"]

			prompt := fmt.Sprintf(
				"Generate a comprehensive sharing report for user %q. Follow these steps:\n"+
					"1) Use ocis_get_user to get the user's profile and display name\n"+
					"2) Use ocis_list_shared_by_me to see all shares this user has created "+
					"(note: this shows shares by the authenticated user — if auditing another user, "+
					"describe what admin tools would be needed)\n"+
					"3) Use ocis_list_received_shares to see all shares received\n"+
					"4) For any public links found, note the link type and whether they have passwords/expiration\n"+
					"5) Summarize: total shares created, total shares received, public links count, "+
					"any shares without expiration dates (potential security concern), "+
					"and recommendations for share hygiene",
				userID)

			return &mcp.GetPromptResult{
				Messages: []*mcp.PromptMessage{
					{Role: "user", Content: &mcp.TextContent{Text: prompt}},
				},
			}, nil
		},
	)
}
