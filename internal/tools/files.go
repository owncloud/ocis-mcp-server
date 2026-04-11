package tools

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/owncloud/ocis-mcp-server/internal/client"
)

// --- Input/Output types ---

type ListFilesInput struct {
	SpaceID string `json:"space_id" jsonschema:"Space/drive ID,required"`
	Path    string `json:"path,omitempty" jsonschema:"Directory path within the space (default: root /)"`
}

type ListFilesOutput struct {
	Files []client.FileInfo `json:"files"`
	Path  string            `json:"path"`
	Count int               `json:"count"`
}

type GetFileInfoInput struct {
	SpaceID string `json:"space_id" jsonschema:"Space/drive ID,required"`
	Path    string `json:"path" jsonschema:"File or folder path,required"`
}

type GetFileInfoOutput struct {
	client.FileInfo
}

type CreateFolderInput struct {
	SpaceID string `json:"space_id" jsonschema:"Space/drive ID,required"`
	Path    string `json:"path" jsonschema:"Folder path to create,required"`
}

type UploadFileInput struct {
	SpaceID     string `json:"space_id" jsonschema:"Space/drive ID,required"`
	Path        string `json:"path" jsonschema:"Destination file path,required"`
	Content     string `json:"content" jsonschema:"File content (text or base64-encoded),required"`
	ContentType string `json:"content_type,omitempty" jsonschema:"MIME type (default: application/octet-stream)"`
	Base64      bool   `json:"base64,omitempty" jsonschema:"Set true if content is base64-encoded"`
}

type DownloadFileInput struct {
	SpaceID string `json:"space_id" jsonschema:"Space/drive ID,required"`
	Path    string `json:"path" jsonschema:"File path to download,required"`
}

type DownloadFileOutput struct {
	Content     string `json:"content"`
	ContentType string `json:"content_type"`
	Size        int    `json:"size"`
	Truncated   bool   `json:"truncated"`
}

type MoveFileInput struct {
	SpaceID     string `json:"space_id" jsonschema:"Space/drive ID,required"`
	SourcePath  string `json:"source_path" jsonschema:"Source file/folder path,required"`
	DestPath    string `json:"dest_path" jsonschema:"Destination path,required"`
	Overwrite   bool   `json:"overwrite,omitempty" jsonschema:"Overwrite if destination exists"`
}

type CopyFileInput struct {
	SpaceID   string `json:"space_id" jsonschema:"Space/drive ID,required"`
	SourcePath string `json:"source_path" jsonschema:"Source file/folder path,required"`
	DestPath   string `json:"dest_path" jsonschema:"Destination path,required"`
	Overwrite  bool   `json:"overwrite,omitempty" jsonschema:"Overwrite if destination exists"`
}

type DeleteFileInput struct {
	SpaceID string `json:"space_id" jsonschema:"Space/drive ID,required"`
	Path    string `json:"path" jsonschema:"File/folder path to delete,required"`
	Confirm bool   `json:"confirm" jsonschema:"Must be true to confirm deletion"`
}

type GetFileVersionsInput struct {
	SpaceID string `json:"space_id" jsonschema:"Space/drive ID,required"`
	FileID  string `json:"file_id" jsonschema:"File ID,required"`
}

type FileVersion struct {
	Name         string `json:"name"`
	Size         int64  `json:"size"`
	LastModified string `json:"last_modified"`
	ETag         string `json:"etag,omitempty"`
}

type FileVersionsOutput struct {
	Versions []FileVersion `json:"versions"`
}

type RestoreFileVersionInput struct {
	SpaceID   string `json:"space_id" jsonschema:"Space/drive ID,required"`
	FileID    string `json:"file_id" jsonschema:"File ID,required"`
	VersionID string `json:"version_id" jsonschema:"Version name to restore,required"`
}

type GetResourceByIDInput struct {
	SpaceID    string `json:"space_id" jsonschema:"Space/drive ID,required"`
	ResourceID string `json:"resource_id" jsonschema:"Resource ID,required"`
}

type TagResourceInput struct {
	SpaceID string `json:"space_id" jsonschema:"Space/drive ID,required"`
	Path    string `json:"path" jsonschema:"Resource path,required"`
	Tags    string `json:"tags" jsonschema:"Comma-separated tags to add,required"`
}

type GetResourceMetadataInput struct {
	DriveID string `json:"drive_id" jsonschema:"Drive ID,required"`
	ItemID  string `json:"item_id" jsonschema:"Item ID,required"`
}

type ResourceMetadataOutput struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Size         int64  `json:"size"`
	LastModified string `json:"lastModifiedDateTime,omitempty"`
	ETag         string `json:"eTag,omitempty"`
}

// --- Registration ---

func registerFiles(s *mcp.Server, c *client.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_list_files",
		Description: "List directory contents in a space. Returns file names, sizes, types, and modification dates.",
		Annotations: readOnlyAnnotations(),
	}, handleListFiles(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_get_file_info",
		Description: "Get metadata for a specific file or folder (size, type, permissions, owner).",
		Annotations: readOnlyAnnotations(),
	}, handleGetFileInfo(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_create_folder",
		Description: "Create a new folder at the specified path in a space.",
		Annotations: idempotentAnnotations(),
	}, handleCreateFolder(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_upload_file",
		Description: "Upload file content to a path in a space. Content can be text or base64-encoded binary.",
		Annotations: mutatingAnnotations(),
	}, handleUploadFile(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_download_file",
		Description: "Download file content from a space. Returns text content for text files, or metadata for binary files.",
		Annotations: readOnlyAnnotations(),
	}, handleDownloadFile(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_move_file",
		Description: "Move or rename a file or folder within a space.",
		Annotations: mutatingAnnotations(),
	}, handleMoveFile(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_copy_file",
		Description: "Copy a file or folder to a new location within a space.",
		Annotations: mutatingAnnotations(),
	}, handleCopyFile(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_delete_file",
		Description: "Delete a file or folder. Moves to trashbin. Requires confirm=true.",
		Annotations: destructiveAnnotations(),
	}, handleDeleteFile(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_get_file_versions",
		Description: "List available versions of a file.",
		Annotations: readOnlyAnnotations(),
	}, handleGetFileVersions(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_restore_file_version",
		Description: "Restore a previous version of a file.",
		Annotations: mutatingAnnotations(),
	}, handleRestoreFileVersion(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_get_resource_by_id",
		Description: "Get file or folder by unique resource ID.",
		Annotations: readOnlyAnnotations(),
	}, handleGetResourceByID(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_tag_resource",
		Description: "Add tags to a file or folder.",
		Annotations: idempotentAnnotations(),
	}, handleTagResource(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_untag_resource",
		Description: "Remove tags from a file or folder.",
		Annotations: idempotentAnnotations(),
	}, handleUntagResource(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_get_resource_metadata",
		Description: "Get LibreGraph metadata for a resource by drive and item ID.",
		Annotations: readOnlyAnnotations(),
	}, handleGetResourceMetadata(c))
}

// --- Handlers ---

func davPath(spaceID, path string) string {
	path = strings.TrimLeft(path, "/")
	if path == "" {
		return fmt.Sprintf("/dav/spaces/%s/", url.PathEscape(spaceID))
	}
	return fmt.Sprintf("/dav/spaces/%s/%s", url.PathEscape(spaceID), path)
}

func handleListFiles(c *client.Client) mcp.ToolHandlerFor[ListFilesInput, ListFilesOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input ListFilesInput) (*mcp.CallToolResult, ListFilesOutput, error) {
		if err := client.ValidateID("space_id", input.SpaceID); err != nil {
			return nil, ListFilesOutput{}, err
		}
		if err := client.ValidatePath(input.Path); err != nil {
			return nil, ListFilesOutput{}, err
		}
		ms, err := client.Propfind(ctx, c, davPath(input.SpaceID, input.Path), "1")
		if err != nil {
			return nil, ListFilesOutput{}, err
		}
		files := make([]client.FileInfo, 0, len(ms.Responses))
		for i, r := range ms.Responses {
			if i == 0 {
				continue // skip the directory itself
			}
			files = append(files, r.ToFileInfo())
		}
		return nil, ListFilesOutput{
			Files: files,
			Path:  input.Path,
			Count: len(files),
		}, nil
	}
}

func handleGetFileInfo(c *client.Client) mcp.ToolHandlerFor[GetFileInfoInput, GetFileInfoOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input GetFileInfoInput) (*mcp.CallToolResult, GetFileInfoOutput, error) {
		if err := client.ValidateID("space_id", input.SpaceID); err != nil {
			return nil, GetFileInfoOutput{}, err
		}
		if err := client.ValidatePath(input.Path); err != nil {
			return nil, GetFileInfoOutput{}, err
		}
		ms, err := client.Propfind(ctx, c, davPath(input.SpaceID, input.Path), "0")
		if err != nil {
			return nil, GetFileInfoOutput{}, err
		}
		if len(ms.Responses) == 0 {
			return nil, GetFileInfoOutput{}, fmt.Errorf("no response for path %q", input.Path)
		}
		return nil, GetFileInfoOutput{FileInfo: ms.Responses[0].ToFileInfo()}, nil
	}
}

func handleCreateFolder(c *client.Client) mcp.ToolHandlerFor[CreateFolderInput, DeleteOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input CreateFolderInput) (*mcp.CallToolResult, DeleteOutput, error) {
		if err := client.ValidateID("space_id", input.SpaceID); err != nil {
			return nil, DeleteOutput{}, err
		}
		if err := client.ValidatePath(input.Path); err != nil {
			return nil, DeleteOutput{}, err
		}
		if err := client.Mkcol(ctx, c, davPath(input.SpaceID, input.Path)); err != nil {
			return nil, DeleteOutput{}, err
		}
		return nil, DeleteOutput{Success: true, Message: "folder created"}, nil
	}
}

func handleUploadFile(c *client.Client) mcp.ToolHandlerFor[UploadFileInput, DeleteOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input UploadFileInput) (*mcp.CallToolResult, DeleteOutput, error) {
		if err := client.ValidateID("space_id", input.SpaceID); err != nil {
			return nil, DeleteOutput{}, err
		}
		if err := client.ValidatePath(input.Path); err != nil {
			return nil, DeleteOutput{}, err
		}
		var reader *strings.Reader
		if input.Base64 {
			data, err := base64.StdEncoding.DecodeString(input.Content)
			if err != nil {
				return nil, DeleteOutput{}, fmt.Errorf("invalid base64 content: %w", err)
			}
			reader = strings.NewReader(string(data))
		} else {
			reader = strings.NewReader(input.Content)
		}
		ct := input.ContentType
		if ct == "" {
			ct = "application/octet-stream"
		}
		if err := client.Upload(ctx, c, davPath(input.SpaceID, input.Path), reader, ct); err != nil {
			return nil, DeleteOutput{}, err
		}
		return nil, DeleteOutput{Success: true, Message: "file uploaded"}, nil
	}
}

func handleDownloadFile(c *client.Client) mcp.ToolHandlerFor[DownloadFileInput, DownloadFileOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input DownloadFileInput) (*mcp.CallToolResult, DownloadFileOutput, error) {
		if err := client.ValidateID("space_id", input.SpaceID); err != nil {
			return nil, DownloadFileOutput{}, err
		}
		if err := client.ValidatePath(input.Path); err != nil {
			return nil, DownloadFileOutput{}, err
		}
		data, ct, err := client.Download(ctx, c, davPath(input.SpaceID, input.Path))
		if err != nil {
			return nil, DownloadFileOutput{}, err
		}

		const maxText = 100 * 1024 // 100KB
		out := DownloadFileOutput{
			ContentType: ct,
			Size:        len(data),
		}

		if utf8.Valid(data) && isTextContentType(ct) {
			if len(data) > maxText {
				out.Content = string(data[:maxText])
				out.Truncated = true
			} else {
				out.Content = string(data)
			}
		} else {
			out.Content = fmt.Sprintf("[Binary file, %d bytes, type: %s]", len(data), ct)
		}
		return nil, out, nil
	}
}

func handleMoveFile(c *client.Client) mcp.ToolHandlerFor[MoveFileInput, DeleteOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input MoveFileInput) (*mcp.CallToolResult, DeleteOutput, error) {
		if err := client.ValidateID("space_id", input.SpaceID); err != nil {
			return nil, DeleteOutput{}, err
		}
		if err := client.ValidatePath(input.SourcePath); err != nil {
			return nil, DeleteOutput{}, err
		}
		if err := client.ValidatePath(input.DestPath); err != nil {
			return nil, DeleteOutput{}, err
		}
		src := davPath(input.SpaceID, input.SourcePath)
		dst := davPath(input.SpaceID, input.DestPath)
		if err := client.Move(ctx, c, src, dst, input.Overwrite); err != nil {
			return nil, DeleteOutput{}, err
		}
		return nil, DeleteOutput{Success: true, Message: "file moved"}, nil
	}
}

func handleCopyFile(c *client.Client) mcp.ToolHandlerFor[CopyFileInput, DeleteOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input CopyFileInput) (*mcp.CallToolResult, DeleteOutput, error) {
		if err := client.ValidateID("space_id", input.SpaceID); err != nil {
			return nil, DeleteOutput{}, err
		}
		if err := client.ValidatePath(input.SourcePath); err != nil {
			return nil, DeleteOutput{}, err
		}
		if err := client.ValidatePath(input.DestPath); err != nil {
			return nil, DeleteOutput{}, err
		}
		src := davPath(input.SpaceID, input.SourcePath)
		dst := davPath(input.SpaceID, input.DestPath)
		if err := client.Copy(ctx, c, src, dst, input.Overwrite); err != nil {
			return nil, DeleteOutput{}, err
		}
		return nil, DeleteOutput{Success: true, Message: "file copied"}, nil
	}
}

func handleDeleteFile(c *client.Client) mcp.ToolHandlerFor[DeleteFileInput, DeleteOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input DeleteFileInput) (*mcp.CallToolResult, DeleteOutput, error) {
		if !input.Confirm {
			return nil, DeleteOutput{}, fmt.Errorf("this is a destructive operation. Set confirm=true to proceed")
		}
		if err := client.ValidateID("space_id", input.SpaceID); err != nil {
			return nil, DeleteOutput{}, err
		}
		if err := client.ValidatePath(input.Path); err != nil {
			return nil, DeleteOutput{}, err
		}
		if err := client.WebDAVDelete(ctx, c, davPath(input.SpaceID, input.Path)); err != nil {
			return nil, DeleteOutput{}, err
		}
		return nil, DeleteOutput{Success: true, Message: "file deleted"}, nil
	}
}

func handleGetFileVersions(c *client.Client) mcp.ToolHandlerFor[GetFileVersionsInput, FileVersionsOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input GetFileVersionsInput) (*mcp.CallToolResult, FileVersionsOutput, error) {
		if err := client.ValidateID("space_id", input.SpaceID); err != nil {
			return nil, FileVersionsOutput{}, err
		}
		if err := client.ValidateID("file_id", input.FileID); err != nil {
			return nil, FileVersionsOutput{}, err
		}
		path := fmt.Sprintf("/dav/meta/%s/v", url.PathEscape(input.FileID))
		ms, err := client.Propfind(ctx, c, path, "1")
		if err != nil {
			return nil, FileVersionsOutput{}, err
		}
		versions := make([]FileVersion, 0, len(ms.Responses))
		for i, r := range ms.Responses {
			if i == 0 {
				continue
			}
			fi := r.ToFileInfo()
			versions = append(versions, FileVersion{
				Name:         fi.Name,
				Size:         fi.Size,
				LastModified: fi.LastModified,
				ETag:         fi.ETag,
			})
		}
		return nil, FileVersionsOutput{Versions: versions}, nil
	}
}

func handleRestoreFileVersion(c *client.Client) mcp.ToolHandlerFor[RestoreFileVersionInput, DeleteOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input RestoreFileVersionInput) (*mcp.CallToolResult, DeleteOutput, error) {
		if err := client.ValidateID("space_id", input.SpaceID); err != nil {
			return nil, DeleteOutput{}, err
		}
		if err := client.ValidateID("file_id", input.FileID); err != nil {
			return nil, DeleteOutput{}, err
		}
		src := fmt.Sprintf("/dav/meta/%s/v/%s", url.PathEscape(input.FileID), url.PathEscape(input.VersionID))
		dst := fmt.Sprintf("/dav/meta/%s/v/restore", url.PathEscape(input.FileID))
		if err := client.Copy(ctx, c, src, dst, true); err != nil {
			return nil, DeleteOutput{}, err
		}
		return nil, DeleteOutput{Success: true, Message: "version restored"}, nil
	}
}

func handleGetResourceByID(c *client.Client) mcp.ToolHandlerFor[GetResourceByIDInput, GetFileInfoOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input GetResourceByIDInput) (*mcp.CallToolResult, GetFileInfoOutput, error) {
		if err := client.ValidateID("space_id", input.SpaceID); err != nil {
			return nil, GetFileInfoOutput{}, err
		}
		if err := client.ValidateID("resource_id", input.ResourceID); err != nil {
			return nil, GetFileInfoOutput{}, err
		}
		path := fmt.Sprintf("/dav/spaces/%s/%s", url.PathEscape(input.SpaceID), url.PathEscape(input.ResourceID))
		ms, err := client.Propfind(ctx, c, path, "0")
		if err != nil {
			return nil, GetFileInfoOutput{}, err
		}
		if len(ms.Responses) == 0 {
			return nil, GetFileInfoOutput{}, fmt.Errorf("resource not found")
		}
		return nil, GetFileInfoOutput{FileInfo: ms.Responses[0].ToFileInfo()}, nil
	}
}

func handleTagResource(c *client.Client) mcp.ToolHandlerFor[TagResourceInput, DeleteOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input TagResourceInput) (*mcp.CallToolResult, DeleteOutput, error) {
		if err := client.ValidateID("space_id", input.SpaceID); err != nil {
			return nil, DeleteOutput{}, err
		}
		if err := client.ValidatePath(input.Path); err != nil {
			return nil, DeleteOutput{}, err
		}
		props := map[string]string{"oc:tags": input.Tags}
		if err := client.Proppatch(ctx, c, davPath(input.SpaceID, input.Path), props); err != nil {
			return nil, DeleteOutput{}, err
		}
		return nil, DeleteOutput{Success: true, Message: "tags added"}, nil
	}
}

func handleUntagResource(c *client.Client) mcp.ToolHandlerFor[TagResourceInput, DeleteOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input TagResourceInput) (*mcp.CallToolResult, DeleteOutput, error) {
		if err := client.ValidateID("space_id", input.SpaceID); err != nil {
			return nil, DeleteOutput{}, err
		}
		if err := client.ValidatePath(input.Path); err != nil {
			return nil, DeleteOutput{}, err
		}
		// For untag, we set tags to empty or remove specific ones via PROPPATCH
		props := map[string]string{"oc:tags": ""}
		if err := client.Proppatch(ctx, c, davPath(input.SpaceID, input.Path), props); err != nil {
			return nil, DeleteOutput{}, err
		}
		return nil, DeleteOutput{Success: true, Message: "tags removed"}, nil
	}
}

func handleGetResourceMetadata(c *client.Client) mcp.ToolHandlerFor[GetResourceMetadataInput, ResourceMetadataOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input GetResourceMetadataInput) (*mcp.CallToolResult, ResourceMetadataOutput, error) {
		if err := client.ValidateID("drive_id", input.DriveID); err != nil {
			return nil, ResourceMetadataOutput{}, err
		}
		if err := client.ValidateID("item_id", input.ItemID); err != nil {
			return nil, ResourceMetadataOutput{}, err
		}
		path := fmt.Sprintf("/graph/v1.0/drives/%s/items/%s",
			url.PathEscape(input.DriveID), url.PathEscape(input.ItemID))
		meta, err := client.GetJSON[ResourceMetadataOutput](ctx, c, path, nil)
		if err != nil {
			return nil, ResourceMetadataOutput{}, err
		}
		return nil, meta, nil
	}
}

func isTextContentType(ct string) bool {
	if strings.HasPrefix(ct, "text/") {
		return true
	}
	textTypes := []string{
		"application/json", "application/xml", "application/javascript",
		"application/yaml", "application/x-yaml", "application/toml",
	}
	for _, t := range textTypes {
		if strings.HasPrefix(ct, t) {
			return true
		}
	}
	return false
}
