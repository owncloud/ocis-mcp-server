package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/owncloud/ocis-mcp-server/internal/client"
)

const propfindResponse = `<?xml version="1.0" encoding="UTF-8"?>
<d:multistatus xmlns:d="DAV:" xmlns:oc="http://owncloud.org/ns">
  <d:response>
    <d:href>/dav/spaces/s1/</d:href>
    <d:propstat>
      <d:prop>
        <d:displayname>root</d:displayname>
        <d:resourcetype><d:collection/></d:resourcetype>
        <d:getcontentlength>0</d:getcontentlength>
      </d:prop>
      <d:status>HTTP/1.1 200 OK</d:status>
    </d:propstat>
  </d:response>
  <d:response>
    <d:href>/dav/spaces/s1/file.txt</d:href>
    <d:propstat>
      <d:prop>
        <d:displayname>file.txt</d:displayname>
        <d:resourcetype/>
        <d:getcontentlength>42</d:getcontentlength>
        <d:getcontenttype>text/plain</d:getcontenttype>
      </d:prop>
      <d:status>HTTP/1.1 200 OK</d:status>
    </d:propstat>
  </d:response>
  <d:response>
    <d:href>/dav/spaces/s1/folder/</d:href>
    <d:propstat>
      <d:prop>
        <d:displayname>folder</d:displayname>
        <d:resourcetype><d:collection/></d:resourcetype>
        <d:getcontentlength>0</d:getcontentlength>
      </d:prop>
      <d:status>HTTP/1.1 200 OK</d:status>
    </d:propstat>
  </d:response>
</d:multistatus>`

const propfindSingleFile = `<?xml version="1.0" encoding="UTF-8"?>
<d:multistatus xmlns:d="DAV:" xmlns:oc="http://owncloud.org/ns">
  <d:response>
    <d:href>/dav/spaces/s1/file.txt</d:href>
    <d:propstat>
      <d:prop>
        <d:displayname>file.txt</d:displayname>
        <d:resourcetype/>
        <d:getcontentlength>100</d:getcontentlength>
        <d:getcontenttype>text/plain</d:getcontenttype>
        <oc:fileid>abc123</oc:fileid>
      </d:prop>
      <d:status>HTTP/1.1 200 OK</d:status>
    </d:propstat>
  </d:response>
</d:multistatus>`

func TestHandleListFiles(t *testing.T) {
	tests := []struct {
		name       string
		spaceID    string
		path       string
		statusCode int
		body       string
		wantCount  int
		wantErr    bool
	}{
		{
			name:       "success lists 2 entries",
			spaceID:    "s1",
			path:       "/",
			statusCode: 207,
			body:       propfindResponse,
			wantCount:  2,
		},
		{name: "empty space_id", spaceID: "", path: "/", wantErr: true},
		{name: "path traversal", spaceID: "s1", path: "/../etc", wantErr: true},
		{
			name:       "server error",
			spaceID:    "s1",
			path:       "/",
			statusCode: 500,
			body:       "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/xml")
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			c := client.New(newTestConfig(srv.URL))
			handler := handleListFiles(c)
			_, output, err := handler(context.Background(), &mcp.CallToolRequest{}, ListFilesInput{
				SpaceID: tt.spaceID, Path: tt.path,
			})

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if output.Count != tt.wantCount {
				t.Errorf("Count = %d, want %d", output.Count, tt.wantCount)
			}
		})
	}
}

func TestHandleGetFileInfo(t *testing.T) {
	tests := []struct {
		name       string
		spaceID    string
		path       string
		statusCode int
		body       string
		wantName   string
		wantErr    bool
	}{
		{
			name:       "success",
			spaceID:    "s1",
			path:       "/file.txt",
			statusCode: 207,
			body:       propfindSingleFile,
			wantName:   "file.txt",
		},
		{name: "empty space_id", spaceID: "", path: "/file.txt", wantErr: true},
		{name: "path traversal", spaceID: "s1", path: "/../etc/passwd", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/xml")
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			c := client.New(newTestConfig(srv.URL))
			handler := handleGetFileInfo(c)
			_, output, err := handler(context.Background(), &mcp.CallToolRequest{}, GetFileInfoInput{
				SpaceID: tt.spaceID, Path: tt.path,
			})

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if output.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", output.Name, tt.wantName)
			}
		})
	}
}

func TestHandleCreateFolder(t *testing.T) {
	tests := []struct {
		name    string
		spaceID string
		path    string
		wantErr bool
	}{
		{name: "success", spaceID: "s1", path: "/newfolder"},
		{name: "empty space_id", spaceID: "", path: "/new", wantErr: true},
		{name: "path traversal", spaceID: "s1", path: "/../evil", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(201)
			}))
			defer srv.Close()

			c := client.New(newTestConfig(srv.URL))
			handler := handleCreateFolder(c)
			_, _, err := handler(context.Background(), &mcp.CallToolRequest{}, CreateFolderInput{
				SpaceID: tt.spaceID, Path: tt.path,
			})
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestHandleUploadFile(t *testing.T) {
	tests := []struct {
		name    string
		spaceID string
		path    string
		content string
		base64  bool
		wantErr bool
	}{
		{name: "text upload", spaceID: "s1", path: "/test.txt", content: "hello"},
		{name: "base64 upload", spaceID: "s1", path: "/bin.dat", content: "aGVsbG8=", base64: true},
		{name: "invalid base64", spaceID: "s1", path: "/bad.dat", content: "not-base64!!!", base64: true, wantErr: true},
		{name: "empty space_id", spaceID: "", path: "/test.txt", content: "hi", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(201)
			}))
			defer srv.Close()

			c := client.New(newTestConfig(srv.URL))
			handler := handleUploadFile(c)
			_, _, err := handler(context.Background(), &mcp.CallToolRequest{}, UploadFileInput{
				SpaceID: tt.spaceID, Path: tt.path, Content: tt.content, Base64: tt.base64,
			})
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestHandleDownloadFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(200)
		_, _ = w.Write([]byte("file content here"))
	}))
	defer srv.Close()

	c := client.New(newTestConfig(srv.URL))
	handler := handleDownloadFile(c)
	_, output, err := handler(context.Background(), &mcp.CallToolRequest{}, DownloadFileInput{
		SpaceID: "s1", Path: "/test.txt",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.Content != "file content here" {
		t.Errorf("Content = %q", output.Content)
	}
	if output.ContentType != "text/plain" {
		t.Errorf("ContentType = %q, want text/plain", output.ContentType)
	}
}

func TestHandleDeleteFile(t *testing.T) {
	tests := []struct {
		name    string
		confirm bool
		wantErr bool
	}{
		{name: "with confirm", confirm: true},
		{name: "without confirm", confirm: false, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(204)
			}))
			defer srv.Close()

			c := client.New(newTestConfig(srv.URL))
			handler := handleDeleteFile(c)
			_, _, err := handler(context.Background(), &mcp.CallToolRequest{}, DeleteFileInput{
				SpaceID: "s1", Path: "/file.txt", Confirm: tt.confirm,
			})
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestHandleMoveFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(201)
	}))
	defer srv.Close()

	c := client.New(newTestConfig(srv.URL))
	handler := handleMoveFile(c)
	_, output, err := handler(context.Background(), &mcp.CallToolRequest{}, MoveFileInput{
		SpaceID: "s1", SourcePath: "/old.txt", DestPath: "/new.txt",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !output.Success {
		t.Error("expected Success=true")
	}
}

func TestHandleCopyFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(201)
	}))
	defer srv.Close()

	c := client.New(newTestConfig(srv.URL))
	handler := handleCopyFile(c)
	_, output, err := handler(context.Background(), &mcp.CallToolRequest{}, CopyFileInput{
		SpaceID: "s1", SourcePath: "/a.txt", DestPath: "/b.txt",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !output.Success {
		t.Error("expected Success=true")
	}
}

func TestHandleTagResource(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(207)
		_, _ = w.Write([]byte(`<?xml version="1.0"?><d:multistatus xmlns:d="DAV:"><d:response><d:href>/dav/spaces/s1/f.txt</d:href><d:propstat><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response></d:multistatus>`))
	}))
	defer srv.Close()

	c := client.New(newTestConfig(srv.URL))
	handler := handleTagResource(c)
	_, output, err := handler(context.Background(), &mcp.CallToolRequest{}, TagResourceInput{
		SpaceID: "s1", Path: "/f.txt", Tags: "important,urgent",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !output.Success {
		t.Error("expected Success=true")
	}
}

func TestHandleGetResourceMetadata(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"id":"item1","name":"file.txt","size":42}`))
	}))
	defer srv.Close()

	c := client.New(newTestConfig(srv.URL))
	handler := handleGetResourceMetadata(c)
	_, output, err := handler(context.Background(), &mcp.CallToolRequest{}, GetResourceMetadataInput{
		DriveID: "d1", ItemID: "item1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.Name != "file.txt" {
		t.Errorf("Name = %q, want file.txt", output.Name)
	}
}

func TestIsTextContentType(t *testing.T) {
	tests := []struct {
		ct   string
		want bool
	}{
		{"text/plain", true},
		{"text/html", true},
		{"application/json", true},
		{"application/xml", true},
		{"application/javascript", true},
		{"application/yaml", true},
		{"application/octet-stream", false},
		{"image/png", false},
	}
	for _, tt := range tests {
		if got := isTextContentType(tt.ct); got != tt.want {
			t.Errorf("isTextContentType(%q) = %v, want %v", tt.ct, got, tt.want)
		}
	}
}

func TestDavPath(t *testing.T) {
	tests := []struct {
		spaceID string
		path    string
		want    string
	}{
		{"s1", "/", "/dav/spaces/s1/"},
		{"s1", "", "/dav/spaces/s1/"},
		{"s1", "/docs/file.txt", "/dav/spaces/s1/docs/file.txt"},
		{"s1", "file.txt", "/dav/spaces/s1/file.txt"},
	}
	for _, tt := range tests {
		got := davPath(tt.spaceID, tt.path)
		if got != tt.want {
			t.Errorf("davPath(%q, %q) = %q, want %q", tt.spaceID, tt.path, got, tt.want)
		}
	}
}
