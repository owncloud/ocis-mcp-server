package tools

import (
	"context"
	"fmt"
	"net/url"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/owncloud/ocis-mcp-server/internal/client"
)

// --- Input/Output types ---

type SearchInput struct {
	Pattern string `json:"pattern" jsonschema:"Search pattern (filename or content query),required"`
	SpaceID string `json:"space_id,omitempty" jsonschema:"Limit search to a specific space/drive ID"`
	Limit   int    `json:"limit,omitempty" jsonschema:"Maximum number of results (default 50, max 200)"`
	Offset  int    `json:"offset,omitempty" jsonschema:"Number of results to skip"`
}

type SearchResult struct {
	Name             string `json:"name"`
	Path             string `json:"path"`
	Type             string `json:"type"`
	Size             int64  `json:"size"`
	ContentType      string `json:"content_type,omitempty"`
	LastModified     string `json:"last_modified,omitempty"`
	FileID           string `json:"file_id,omitempty"`
	SpaceID          string `json:"space_id,omitempty"`
	OwnerID          string `json:"owner_id,omitempty"`
	OwnerDisplayName string `json:"owner_display_name,omitempty"`
	Highlights       string `json:"highlights,omitempty"`
	Score            string `json:"score,omitempty"`
	Tags             string `json:"tags,omitempty"`
}

type SearchOutput struct {
	Results    []SearchResult `json:"results"`
	TotalCount int            `json:"total_count"`
	HasMore    bool           `json:"has_more"`
	NextOffset int            `json:"next_offset"`
}

type SearchByTagInput struct {
	Tag     string `json:"tag" jsonschema:"Tag name to search for,required"`
	SpaceID string `json:"space_id,omitempty" jsonschema:"Limit search to a specific space/drive ID"`
	Limit   int    `json:"limit,omitempty" jsonschema:"Maximum number of results (default 50, max 200)"`
	Offset  int    `json:"offset,omitempty" jsonschema:"Number of results to skip"`
}

// --- Registration ---

func registerSearch(s *mcp.Server, c *client.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_search",
		Description: "Full-text search across files and folders. Returns matching resources with highlights and relevance scores.",
		Annotations: readOnlyAnnotations(),
	}, handleSearch(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_search_by_tag",
		Description: "Search for files and folders by tag. Returns matching resources tagged with the specified value.",
		Annotations: readOnlyAnnotations(),
	}, handleSearchByTag(c))
}

// --- Handlers ---

func searchPath(spaceID string) string {
	if spaceID != "" {
		return fmt.Sprintf("/dav/spaces/%s/", url.PathEscape(spaceID))
	}
	return "/dav/spaces/"
}

func toSearchResults(ms *client.MultiStatus) []SearchResult {
	results := make([]SearchResult, 0, len(ms.Responses))
	for _, r := range ms.Responses {
		fi := r.ToFileInfo()
		sr := SearchResult{
			Name:             fi.Name,
			Path:             fi.Path,
			Type:             fi.Type,
			Size:             fi.Size,
			ContentType:      fi.ContentType,
			LastModified:     fi.LastModified,
			FileID:           fi.FileID,
			OwnerID:          fi.OwnerID,
			OwnerDisplayName: fi.OwnerDisplayName,
			Tags:             fi.Tags,
		}
		// Extract search-specific properties from the raw response.
		for _, ps := range r.PropStats {
			sr.Highlights = ps.Prop.Highlights
			sr.Score = ps.Prop.Score
			sr.SpaceID = ps.Prop.SpaceID
		}
		results = append(results, sr)
	}
	return results
}

func handleSearch(c *client.Client) mcp.ToolHandlerFor[SearchInput, SearchOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input SearchInput) (*mcp.CallToolResult, SearchOutput, error) {
		if err := client.ValidateID("pattern", input.Pattern); err != nil {
			return nil, SearchOutput{}, fmt.Errorf("pattern is required")
		}
		limit := client.ValidateLimit(input.Limit)

		ms, err := client.SearchReport(ctx, c, searchPath(input.SpaceID), input.Pattern, limit, input.Offset)
		if err != nil {
			return nil, SearchOutput{}, err
		}

		results := toSearchResults(ms)
		return nil, SearchOutput{
			Results:    results,
			TotalCount: len(results),
			HasMore:    len(results) == limit,
			NextOffset: input.Offset + len(results),
		}, nil
	}
}

func handleSearchByTag(c *client.Client) mcp.ToolHandlerFor[SearchByTagInput, SearchOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input SearchByTagInput) (*mcp.CallToolResult, SearchOutput, error) {
		if err := client.ValidateID("tag", input.Tag); err != nil {
			return nil, SearchOutput{}, fmt.Errorf("tag is required")
		}
		limit := client.ValidateLimit(input.Limit)

		// Use tag: prefix filter for tag-based search.
		pattern := fmt.Sprintf("Tags:%s", input.Tag)

		ms, err := client.SearchReport(ctx, c, searchPath(input.SpaceID), pattern, limit, input.Offset)
		if err != nil {
			return nil, SearchOutput{}, err
		}

		results := toSearchResults(ms)
		return nil, SearchOutput{
			Results:    results,
			TotalCount: len(results),
			HasMore:    len(results) == limit,
			NextOffset: input.Offset + len(results),
		}, nil
	}
}
