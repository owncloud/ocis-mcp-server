package client

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// MultiStatus represents a WebDAV 207 Multi-Status response.
type MultiStatus struct {
	XMLName   xml.Name   `xml:"DAV: multistatus"`
	Responses []Response `xml:"response"`
}

// Response represents a single resource in a Multi-Status response.
type Response struct {
	Href      string     `xml:"href"`
	PropStats []PropStat `xml:"propstat"`
}

// PropStat contains the properties and status for a response.
type PropStat struct {
	Prop   Prop   `xml:"prop"`
	Status string `xml:"status"`
}

// Prop holds the WebDAV properties of a resource.
type Prop struct {
	ResourceType     ResourceType `xml:"resourcetype"`
	DisplayName      string       `xml:"displayname"`
	GetContentLength int64        `xml:"getcontentlength"`
	GetContentType   string       `xml:"getcontenttype"`
	GetLastModified  string       `xml:"getlastmodified"`
	GetETag          string       `xml:"getetag"`
	// ownCloud-specific properties
	FileID           string `xml:"http://owncloud.org/ns fileid"`
	Permissions      string `xml:"http://owncloud.org/ns permissions"`
	Size             int64  `xml:"http://owncloud.org/ns size"`
	OwnerID          string `xml:"http://owncloud.org/ns owner-id"`
	OwnerDisplayName string `xml:"http://owncloud.org/ns owner-display-name"`
	Favorite         string `xml:"http://owncloud.org/ns favorite"`
	ShareTypes       string `xml:"http://owncloud.org/ns share-types"`
	Tags             string `xml:"http://owncloud.org/ns tags"`
	// Search-specific
	Highlights string `xml:"http://owncloud.org/ns highlights"`
	Score      string `xml:"http://owncloud.org/ns score"`
	SpaceID    string `xml:"http://owncloud.org/ns spaceid"`
	Name       string `xml:"http://owncloud.org/ns name"`
}

// ResourceType contains the resource type element.
type ResourceType struct {
	Collection *struct{} `xml:"collection"`
}

// IsCollection returns true if the resource is a directory.
func (p *Prop) IsCollection() bool {
	return p.ResourceType.Collection != nil
}

// FileInfo is a structured representation of a WebDAV resource for JSON output.
type FileInfo struct {
	Name             string `json:"name"`
	Path             string `json:"path"`
	Type             string `json:"type"` // "file" or "folder"
	Size             int64  `json:"size"`
	ContentType      string `json:"content_type,omitempty"`
	LastModified     string `json:"last_modified"`
	ETag             string `json:"etag,omitempty"`
	FileID           string `json:"file_id,omitempty"`
	Permissions      string `json:"permissions,omitempty"`
	OwnerID          string `json:"owner_id,omitempty"`
	OwnerDisplayName string `json:"owner_display_name,omitempty"`
	Tags             string `json:"tags,omitempty"`
}

// ToFileInfo converts a WebDAV Response to a FileInfo.
func (r *Response) ToFileInfo() FileInfo {
	fi := FileInfo{
		Path: r.Href,
		Name: nameFromHref(r.Href),
	}
	for _, ps := range r.PropStats {
		if !strings.Contains(ps.Status, "200") {
			continue
		}
		p := ps.Prop
		if p.IsCollection() {
			fi.Type = "folder"
		} else {
			fi.Type = "file"
		}
		fi.Size = p.Size
		if fi.Size == 0 {
			fi.Size = p.GetContentLength
		}
		fi.ContentType = p.GetContentType
		fi.LastModified = p.GetLastModified
		fi.ETag = p.GetETag
		fi.FileID = p.FileID
		fi.Permissions = p.Permissions
		fi.OwnerID = p.OwnerID
		fi.OwnerDisplayName = p.OwnerDisplayName
		fi.Tags = p.Tags
		if p.Name != "" {
			fi.Name = p.Name
		}
	}
	return fi
}

func nameFromHref(href string) string {
	href = strings.TrimRight(href, "/")
	if idx := strings.LastIndex(href, "/"); idx >= 0 {
		return href[idx+1:]
	}
	return href
}

// Propfind sends a PROPFIND request and parses the Multi-Status response.
func Propfind(ctx context.Context, c *Client, path string, depth string) (*MultiStatus, error) {
	body := `<?xml version="1.0" encoding="UTF-8"?>
<d:propfind xmlns:d="DAV:" xmlns:oc="http://owncloud.org/ns">
  <d:prop>
    <d:resourcetype/>
    <d:displayname/>
    <d:getcontentlength/>
    <d:getcontenttype/>
    <d:getlastmodified/>
    <d:getetag/>
    <oc:fileid/>
    <oc:permissions/>
    <oc:size/>
    <oc:owner-id/>
    <oc:owner-display-name/>
    <oc:favorite/>
    <oc:share-types/>
    <oc:tags/>
    <oc:name/>
  </d:prop>
</d:propfind>`

	req, err := c.NewRequest(ctx, "PROPFIND", path, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/xml")
	req.Header.Set("Depth", depth)

	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, &APIError{StatusCode: 404, Message: "resource not found: " + path}
	}
	if resp.StatusCode != http.StatusMultiStatus {
		return nil, errorFromResponse(resp)
	}

	var ms MultiStatus
	if err := xml.NewDecoder(resp.Body).Decode(&ms); err != nil {
		return nil, fmt.Errorf("parsing PROPFIND response: %w", err)
	}
	return &ms, nil
}

// Mkcol creates a collection (folder) at the given path.
func Mkcol(ctx context.Context, c *Client, path string) error {
	req, err := c.NewRequest(ctx, "MKCOL", path, nil)
	if err != nil {
		return err
	}
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		return errorFromResponse(resp)
	}
	return nil
}

// Upload puts file content at the given path.
func Upload(ctx context.Context, c *Client, path string, content io.Reader, contentType string) error {
	req, err := c.NewRequest(ctx, http.MethodPut, path, content)
	if err != nil {
		return err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		return errorFromResponse(resp)
	}
	return nil
}

// Download retrieves file content from the given path.
func Download(ctx context.Context, c *Client, path string) ([]byte, string, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		return nil, "", errorFromResponse(resp)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit
	return data, resp.Header.Get("Content-Type"), err
}

// Move performs a WebDAV MOVE operation.
func Move(ctx context.Context, c *Client, srcPath, destPath string, overwrite bool) error {
	req, err := c.NewRequest(ctx, "MOVE", srcPath, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Destination", c.baseURL+destPath)
	if overwrite {
		req.Header.Set("Overwrite", "T")
	} else {
		req.Header.Set("Overwrite", "F")
	}
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		return errorFromResponse(resp)
	}
	return nil
}

// Copy performs a WebDAV COPY operation.
func Copy(ctx context.Context, c *Client, srcPath, destPath string, overwrite bool) error {
	req, err := c.NewRequest(ctx, "COPY", srcPath, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Destination", c.baseURL+destPath)
	if overwrite {
		req.Header.Set("Overwrite", "T")
	} else {
		req.Header.Set("Overwrite", "F")
	}
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		return errorFromResponse(resp)
	}
	return nil
}

// WebDAVDelete performs a WebDAV DELETE operation.
func WebDAVDelete(ctx context.Context, c *Client, path string) error {
	req, err := c.NewRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		return errorFromResponse(resp)
	}
	return nil
}

// SearchReport sends a REPORT request for full-text search.
func SearchReport(ctx context.Context, c *Client, path, pattern string, limit, offset int) (*MultiStatus, error) {
	body := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<oc:search-files xmlns:d="DAV:" xmlns:oc="http://owncloud.org/ns">
  <d:prop>
    <d:resourcetype/>
    <d:displayname/>
    <d:getcontentlength/>
    <d:getcontenttype/>
    <d:getlastmodified/>
    <d:getetag/>
    <oc:fileid/>
    <oc:permissions/>
    <oc:size/>
    <oc:owner-id/>
    <oc:owner-display-name/>
    <oc:tags/>
    <oc:name/>
    <oc:highlights/>
    <oc:score/>
    <oc:spaceid/>
  </d:prop>
  <oc:search>
    <oc:pattern>%s</oc:pattern>
    <oc:limit>%d</oc:limit>
    <oc:offset>%d</oc:offset>
  </oc:search>
</oc:search-files>`, xmlEscape(pattern), limit, offset)

	req, err := c.NewRequest(ctx, "REPORT", path, bytes.NewReader([]byte(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/xml")

	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusMultiStatus && resp.StatusCode != http.StatusOK {
		return nil, errorFromResponse(resp)
	}

	var ms MultiStatus
	if err := xml.NewDecoder(resp.Body).Decode(&ms); err != nil {
		return nil, fmt.Errorf("parsing search response: %w", err)
	}
	return &ms, nil
}

// Proppatch sends a PROPPATCH request to set or remove properties.
func Proppatch(ctx context.Context, c *Client, path string, setProps map[string]string) error {
	var propEntries strings.Builder
	for k, v := range setProps {
		fmt.Fprintf(&propEntries, "<%s>%s</%s>", k, xmlEscape(v), k)
	}

	body := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<d:propertyupdate xmlns:d="DAV:" xmlns:oc="http://owncloud.org/ns">
  <d:set>
    <d:prop>
      %s
    </d:prop>
  </d:set>
</d:propertyupdate>`, propEntries.String())

	req, err := c.NewRequest(ctx, "PROPPATCH", path, strings.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/xml")

	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		return errorFromResponse(resp)
	}
	return nil
}

func xmlEscape(s string) string {
	var buf bytes.Buffer
	_ = xml.EscapeText(&buf, []byte(s))
	return buf.String()
}
