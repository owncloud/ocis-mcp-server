package tools

import (
	"context"
	"fmt"
	"strings"

	libregraph "github.com/owncloud/libre-graph-api-go"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/owncloud/ocis-mcp-server/internal/client"
)

// --- Input/Output types ---

type UploadAndShareInput struct {
	SpaceID     string   `json:"space_id" jsonschema:"Space/drive ID,required"`
	Path        string   `json:"path" jsonschema:"Destination file path,required"`
	Content     string   `json:"content" jsonschema:"File content (text),required"`
	ContentType string   `json:"content_type,omitempty" jsonschema:"MIME type (default: text/plain)"`
	Recipients  []string `json:"recipients" jsonschema:"User or group IDs to share with,required"`
	Roles       []string `json:"roles" jsonschema:"Sharing role IDs,required"`
}

type UploadAndShareOutput struct {
	FileUploaded bool         `json:"file_uploaded"`
	Permissions  []Permission `json:"permissions"`
	Message      string       `json:"message"`
}

type CreateProjectSpaceInput struct {
	Name        string   `json:"name" jsonschema:"Space name,required"`
	Description string   `json:"description,omitempty" jsonschema:"Space description"`
	Quota       int64    `json:"quota,omitempty" jsonschema:"Quota in bytes"`
	Members     []string `json:"members,omitempty" jsonschema:"User IDs to invite as members"`
	MemberRoles []string `json:"member_roles,omitempty" jsonschema:"Role IDs for invited members"`
}

type CreateProjectSpaceOutput struct {
	Space       Drive        `json:"space"`
	Permissions []Permission `json:"permissions,omitempty"`
	Message     string       `json:"message"`
}

type FindAndDownloadInput struct {
	Pattern string `json:"pattern" jsonschema:"Search pattern,required"`
	SpaceID string `json:"space_id,omitempty" jsonschema:"Limit search to this space"`
}

type FindAndDownloadOutput struct {
	FileName    string `json:"file_name"`
	Content     string `json:"content"`
	ContentType string `json:"content_type"`
	Size        int    `json:"size"`
	Truncated   bool   `json:"truncated"`
}

type ShareWithLinkInput struct {
	SpaceID string `json:"space_id" jsonschema:"Space/drive ID,required"`
	ItemID  string `json:"item_id" jsonschema:"Item ID,required"`
	Type    string `json:"type,omitempty" jsonschema:"Link type: view or edit (default: view)"`
}

type ShareWithLinkOutput struct {
	URL        string     `json:"url"`
	Permission Permission `json:"permission"`
}

type GetSpaceOverviewInput struct {
	SpaceID string `json:"space_id" jsonschema:"Space/drive ID,required"`
}

type GetSpaceOverviewOutput struct {
	Space       Drive              `json:"space"`
	RootFiles   []client.FileInfo  `json:"root_files"`
	Permissions []Permission       `json:"permissions"`
	FileCount   int                `json:"file_count"`
}

// --- Registration ---

func registerWorkflows(s *mcp.Server, c *client.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_upload_and_share",
		Description: "Upload a file and immediately share it with specified users or groups. Combines upload + create_share.",
		Annotations: mutatingAnnotations(),
	}, handleUploadAndShare(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_create_project_space",
		Description: "Create a project space with quota and initial members in one call. Combines create_space + invite_to_space.",
		Annotations: mutatingAnnotations(),
	}, handleCreateProjectSpace(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_find_and_download",
		Description: "Search for a file by name/pattern and download the first match. Combines search + download.",
		Annotations: readOnlyAnnotations(),
	}, handleFindAndDownload(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_share_with_link",
		Description: "Create a public link for a file and return the sharing URL directly.",
		Annotations: mutatingAnnotations(),
	}, handleShareWithLink(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_get_space_overview",
		Description: "Get a complete overview of a space: metadata, root contents, and permissions in one call.",
		Annotations: readOnlyAnnotations(),
	}, handleGetSpaceOverview(c))
}

// --- Handlers ---

func handleUploadAndShare(c *client.Client) mcp.ToolHandlerFor[UploadAndShareInput, UploadAndShareOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input UploadAndShareInput) (*mcp.CallToolResult, UploadAndShareOutput, error) {
		if err := client.ValidateID("space_id", input.SpaceID); err != nil {
			return nil, UploadAndShareOutput{}, err
		}
		if err := client.ValidatePath(input.Path); err != nil {
			return nil, UploadAndShareOutput{}, err
		}

		// Step 1: Upload
		ct := input.ContentType
		if ct == "" {
			ct = "text/plain"
		}
		if err := client.Upload(ctx, c, davPath(input.SpaceID, input.Path), strings.NewReader(input.Content), ct); err != nil {
			return nil, UploadAndShareOutput{}, fmt.Errorf("upload step failed: %w", err)
		}

		// Step 2: Get the uploaded file's item ID via PROPFIND
		ms, err := client.Propfind(ctx, c, davPath(input.SpaceID, input.Path), "0")
		if err != nil {
			return nil, UploadAndShareOutput{FileUploaded: true}, fmt.Errorf("file uploaded but could not get file info for sharing: %w", err)
		}
		if len(ms.Responses) == 0 {
			return nil, UploadAndShareOutput{FileUploaded: true}, fmt.Errorf("file uploaded but file info not returned")
		}
		fi := ms.Responses[0].ToFileInfo()

		// Step 3: Share
		recipients := make([]libregraph.DriveRecipient, len(input.Recipients))
		for i, r := range input.Recipients {
			rec := libregraph.NewDriveRecipient()
			rec.SetObjectId(r)
			recipients[i] = *rec
		}
		invite := libregraph.NewDriveItemInvite()
		invite.SetRecipients(recipients)
		invite.SetRoles(input.Roles)
		resp, _, err := c.GraphClient().DrivesPermissionsApi.
			Invite(ctx, input.SpaceID, fi.FileID).DriveItemInvite(*invite).Execute()
		if err != nil {
			return nil, UploadAndShareOutput{FileUploaded: true}, fmt.Errorf("file uploaded but sharing failed: %w", sdkError(err))
		}
		perms, err := sdkConvert[[]Permission](resp.GetValue())
		if err != nil {
			return nil, UploadAndShareOutput{FileUploaded: true}, fmt.Errorf("file uploaded but mapping permissions failed: %w", err)
		}

		return nil, UploadAndShareOutput{
			FileUploaded: true,
			Permissions:  perms,
			Message:      "file uploaded and shared",
		}, nil
	}
}

func handleCreateProjectSpace(c *client.Client) mcp.ToolHandlerFor[CreateProjectSpaceInput, CreateProjectSpaceOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input CreateProjectSpaceInput) (*mcp.CallToolResult, CreateProjectSpaceOutput, error) {
		// Step 1: Create space
		drv := libregraph.NewDrive(input.Name)
		drv.SetDriveType("project")
		if input.Description != "" {
			drv.SetDescription(input.Description)
		}
		if input.Quota > 0 {
			q := libregraph.NewQuota()
			q.SetTotal(input.Quota)
			drv.SetQuota(*q)
		}
		created, _, err := c.GraphClient().DrivesApi.CreateDrive(ctx).Drive(*drv).Execute()
		if err != nil {
			return nil, CreateProjectSpaceOutput{}, fmt.Errorf("create space step failed: %w", sdkError(err))
		}
		drive, err := sdkConvert[Drive](created)
		if err != nil {
			return nil, CreateProjectSpaceOutput{}, fmt.Errorf("create space step failed: %w", err)
		}

		out := CreateProjectSpaceOutput{
			Space:   drive,
			Message: "space created",
		}

		// Step 2: Invite members (if provided)
		if len(input.Members) > 0 && len(input.MemberRoles) > 0 {
			recipients := make([]libregraph.DriveRecipient, len(input.Members))
			for i, m := range input.Members {
				rec := libregraph.NewDriveRecipient()
				rec.SetObjectId(m)
				recipients[i] = *rec
			}
			invite := libregraph.NewDriveItemInvite()
			invite.SetRecipients(recipients)
			invite.SetRoles(input.MemberRoles)
			resp, _, err := c.GraphClient().DrivesRootApi.
				InviteSpaceRoot(ctx, drive.ID).DriveItemInvite(*invite).Execute()
			switch {
			case err != nil:
				out.Message = fmt.Sprintf("space created but inviting members failed: %v", sdkError(err))
			default:
				if perms, cerr := sdkConvert[[]Permission](resp.GetValue()); cerr == nil {
					out.Permissions = perms
				}
				out.Message = "space created with members"
			}
		}

		return nil, out, nil
	}
}

func handleFindAndDownload(c *client.Client) mcp.ToolHandlerFor[FindAndDownloadInput, FindAndDownloadOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input FindAndDownloadInput) (*mcp.CallToolResult, FindAndDownloadOutput, error) {
		// Step 1: Search
		searchPath := "/dav/spaces/"
		if input.SpaceID != "" {
			searchPath = fmt.Sprintf("/dav/spaces/%s/", input.SpaceID)
		}
		ms, err := client.SearchReport(ctx, c, searchPath, input.Pattern, 1, 0)
		if err != nil {
			return nil, FindAndDownloadOutput{}, fmt.Errorf("search step failed: %w", err)
		}
		if len(ms.Responses) == 0 {
			return nil, FindAndDownloadOutput{}, fmt.Errorf("no files found matching pattern %q", input.Pattern)
		}

		// Step 2: Download the first match
		href := ms.Responses[0].Href
		data, ct, err := client.Download(ctx, c, href)
		if err != nil {
			return nil, FindAndDownloadOutput{}, fmt.Errorf("download step failed: %w", err)
		}

		fi := ms.Responses[0].ToFileInfo()
		out := FindAndDownloadOutput{
			FileName:    fi.Name,
			ContentType: ct,
			Size:        len(data),
		}

		const maxText = 100 * 1024
		if isTextContentType(ct) && len(data) <= maxText {
			out.Content = string(data)
		} else if isTextContentType(ct) {
			out.Content = string(data[:maxText])
			out.Truncated = true
		} else {
			out.Content = fmt.Sprintf("[Binary file, %d bytes, type: %s]", len(data), ct)
		}

		return nil, out, nil
	}
}

func handleShareWithLink(c *client.Client) mcp.ToolHandlerFor[ShareWithLinkInput, ShareWithLinkOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input ShareWithLinkInput) (*mcp.CallToolResult, ShareWithLinkOutput, error) {
		if err := client.ValidateID("space_id", input.SpaceID); err != nil {
			return nil, ShareWithLinkOutput{}, err
		}
		if err := client.ValidateID("item_id", input.ItemID); err != nil {
			return nil, ShareWithLinkOutput{}, err
		}
		linkType := input.Type
		if linkType == "" {
			linkType = "view"
		}
		lt, err := libregraph.NewSharingLinkTypeFromValue(linkType)
		if err != nil {
			return nil, ShareWithLinkOutput{}, fmt.Errorf("invalid link type %q: %w", linkType, err)
		}
		link := libregraph.NewDriveItemCreateLink()
		link.SetType(*lt)
		sdkPerm, _, err := c.GraphClient().DrivesPermissionsApi.
			CreateLink(ctx, input.SpaceID, input.ItemID).DriveItemCreateLink(*link).Execute()
		if err != nil {
			return nil, ShareWithLinkOutput{}, sdkError(err)
		}
		perm, err := sdkConvert[Permission](sdkPerm)
		if err != nil {
			return nil, ShareWithLinkOutput{}, err
		}

		linkURL := ""
		if perm.Link != nil {
			linkURL = perm.Link.WebURL
		}
		return nil, ShareWithLinkOutput{
			URL:        linkURL,
			Permission: perm,
		}, nil
	}
}

func handleGetSpaceOverview(c *client.Client) mcp.ToolHandlerFor[GetSpaceOverviewInput, GetSpaceOverviewOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input GetSpaceOverviewInput) (*mcp.CallToolResult, GetSpaceOverviewOutput, error) {
		if err := client.ValidateID("space_id", input.SpaceID); err != nil {
			return nil, GetSpaceOverviewOutput{}, err
		}

		// Step 1: Get space details
		sdkDrive, _, err := c.GraphClient().DrivesApi.GetDrive(ctx, input.SpaceID).Execute()
		if err != nil {
			return nil, GetSpaceOverviewOutput{}, fmt.Errorf("get space step failed: %w", sdkError(err))
		}
		drive, err := sdkConvert[Drive](sdkDrive)
		if err != nil {
			return nil, GetSpaceOverviewOutput{}, fmt.Errorf("get space step failed: %w", err)
		}

		out := GetSpaceOverviewOutput{Space: drive}

		// Step 2: List root files
		ms, err := client.Propfind(ctx, c, davPath(input.SpaceID, "/"), "1")
		if err == nil {
			for i, r := range ms.Responses {
				if i == 0 {
					continue
				}
				out.RootFiles = append(out.RootFiles, r.ToFileInfo())
			}
			out.FileCount = len(out.RootFiles)
		}

		// Step 3: List permissions
		if resp, _, perr := c.GraphClient().DrivesRootApi.ListPermissionsSpaceRoot(ctx, input.SpaceID).Execute(); perr == nil {
			if perms, cerr := sdkConvert[[]Permission](resp.GetValue()); cerr == nil {
				out.Permissions = perms
			}
		}

		return nil, out, nil
	}
}
