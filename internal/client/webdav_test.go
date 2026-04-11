package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPropfind(t *testing.T) {
	tests := []struct {
		name          string
		fixture       string
		statusCode    int
		wantResponses int
		wantErr       bool
		wantErrMsg    string
	}{
		{
			name:          "parses 207 Multi-Status from fixture",
			fixture:       "propfind_folder.xml",
			statusCode:    207,
			wantResponses: 3,
		},
		{
			name:       "404 returns not found error",
			fixture:    "",
			statusCode: 404,
			wantErr:    true,
			wantErrMsg: "not found",
		},
		{
			name:       "403 returns permissions error",
			fixture:    "",
			statusCode: 403,
			wantErr:    true,
			wantErrMsg: "insufficient permissions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body []byte
			if tt.fixture != "" {
				var err error
				body, err = os.ReadFile(filepath.Join(testdataDir(), tt.fixture))
				if err != nil {
					t.Fatalf("reading fixture: %v", err)
				}
			}

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "PROPFIND" {
					t.Errorf("expected PROPFIND, got %s", r.Method)
				}
				if tt.statusCode == 207 {
					w.Header().Set("Content-Type", "application/xml")
					w.WriteHeader(http.StatusMultiStatus)
					w.Write(body)
				} else {
					w.WriteHeader(tt.statusCode)
					w.Write([]byte(`{}`))
				}
			}))
			defer srv.Close()

			cfg := newTestConfig(srv.URL)
			c := New(cfg)

			ms, err := Propfind(context.Background(), c, "/dav/spaces/space1/Documents/", "1")
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.wantErrMsg != "" && !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Errorf("error = %q, want it to contain %q", err.Error(), tt.wantErrMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(ms.Responses) != tt.wantResponses {
				t.Errorf("got %d responses, want %d", len(ms.Responses), tt.wantResponses)
			}
		})
	}
}

func TestToFileInfo(t *testing.T) {
	tests := []struct {
		name     string
		response Response
		wantName string
		wantType string
		wantSize int64
	}{
		{
			name: "file response",
			response: Response{
				Href: "/dav/spaces/space1/Documents/report.pdf",
				PropStats: []PropStat{
					{
						Prop: Prop{
							DisplayName:      "report.pdf",
							GetContentLength: 102400,
							GetContentType:   "application/pdf",
							GetLastModified:  "Tue, 11 Mar 2025 09:15:00 GMT",
							GetETag:          `"def456"`,
							FileID:           "fileid-file1",
							Size:             102400,
						},
						Status: "HTTP/1.1 200 OK",
					},
				},
			},
			wantName: "report.pdf",
			wantType: "file",
			wantSize: 102400,
		},
		{
			name: "folder response",
			response: Response{
				Href: "/dav/spaces/space1/Documents/Drafts/",
				PropStats: []PropStat{
					{
						Prop: Prop{
							DisplayName: "Drafts",
							ResourceType: ResourceType{
								Collection: &struct{}{},
							},
							GetLastModified: "Wed, 12 Mar 2025 16:45:00 GMT",
							FileID:          "fileid-subfolder",
							Size:            51200,
						},
						Status: "HTTP/1.1 200 OK",
					},
				},
			},
			wantName: "Drafts",
			wantType: "folder",
			wantSize: 51200,
		},
		{
			name: "name from oc:name overrides href",
			response: Response{
				Href: "/dav/spaces/space1/some-opaque-id",
				PropStats: []PropStat{
					{
						Prop: Prop{
							Name:            "my-document.txt",
							GetContentLength: 256,
							GetContentType:  "text/plain",
							Size:            256,
						},
						Status: "HTTP/1.1 200 OK",
					},
				},
			},
			wantName: "my-document.txt",
			wantType: "file",
			wantSize: 256,
		},
		{
			name: "size falls back to getcontentlength",
			response: Response{
				Href: "/dav/spaces/space1/fallback.txt",
				PropStats: []PropStat{
					{
						Prop: Prop{
							GetContentLength: 1024,
							GetContentType:   "text/plain",
						},
						Status: "HTTP/1.1 200 OK",
					},
				},
			},
			wantName: "fallback.txt",
			wantType: "file",
			wantSize: 1024,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fi := tt.response.ToFileInfo()
			if fi.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", fi.Name, tt.wantName)
			}
			if fi.Type != tt.wantType {
				t.Errorf("Type = %q, want %q", fi.Type, tt.wantType)
			}
			if fi.Size != tt.wantSize {
				t.Errorf("Size = %d, want %d", fi.Size, tt.wantSize)
			}
		})
	}
}

func TestSearchReport(t *testing.T) {
	tests := []struct {
		name       string
		pattern    string
		limit      int
		offset     int
		statusCode int
		body       string
		wantErr    bool
		wantCount  int
	}{
		{
			name:       "successful search with results",
			pattern:    "report",
			limit:      10,
			offset:     0,
			statusCode: 207,
			body: `<?xml version="1.0" encoding="UTF-8"?>
<d:multistatus xmlns:d="DAV:" xmlns:oc="http://owncloud.org/ns">
  <d:response>
    <d:href>/dav/spaces/space1/Documents/report.pdf</d:href>
    <d:propstat>
      <d:prop>
        <d:resourcetype/>
        <d:displayname>report.pdf</d:displayname>
        <oc:fileid>fileid-1</oc:fileid>
        <oc:highlights>&lt;em&gt;report&lt;/em&gt;.pdf</oc:highlights>
      </d:prop>
      <d:status>HTTP/1.1 200 OK</d:status>
    </d:propstat>
  </d:response>
</d:multistatus>`,
			wantCount: 1,
		},
		{
			name:       "empty search results",
			pattern:    "nonexistent",
			limit:      10,
			offset:     0,
			statusCode: 207,
			body: `<?xml version="1.0" encoding="UTF-8"?>
<d:multistatus xmlns:d="DAV:" xmlns:oc="http://owncloud.org/ns">
</d:multistatus>`,
			wantCount: 0,
		},
		{
			name:       "server error",
			pattern:    "test",
			limit:      10,
			offset:     0,
			statusCode: 500,
			body:       `{}`,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotMethod string
			var gotContentType string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod = r.Method
				gotContentType = r.Header.Get("Content-Type")
				if tt.statusCode == 207 {
					w.Header().Set("Content-Type", "application/xml")
					w.WriteHeader(http.StatusMultiStatus)
				} else {
					w.WriteHeader(tt.statusCode)
				}
				w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			cfg := newTestConfig(srv.URL)
			c := New(cfg)

			ms, err := SearchReport(context.Background(), c, "/dav/spaces/space1/", tt.pattern, tt.limit, tt.offset)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotMethod != "REPORT" {
				t.Errorf("method = %q, want REPORT", gotMethod)
			}
			if gotContentType != "application/xml" {
				t.Errorf("Content-Type = %q, want application/xml", gotContentType)
			}
			if len(ms.Responses) != tt.wantCount {
				t.Errorf("got %d responses, want %d", len(ms.Responses), tt.wantCount)
			}
		})
	}
}

func TestMkcol(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantErr    bool
	}{
		{name: "201 success", statusCode: 201, wantErr: false},
		{name: "405 already exists", statusCode: 405, wantErr: true},
		{name: "409 conflict", statusCode: 409, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotMethod string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod = r.Method
				w.WriteHeader(tt.statusCode)
			}))
			defer srv.Close()

			c := New(newTestConfig(srv.URL))
			err := Mkcol(context.Background(), c, "/dav/spaces/s1/NewFolder")
			if gotMethod != "MKCOL" {
				t.Errorf("method = %q, want MKCOL", gotMethod)
			}
			if (err != nil) != tt.wantErr {
				t.Errorf("Mkcol error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUpload(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		statusCode  int
		wantErr     bool
	}{
		{name: "201 success", contentType: "text/plain", statusCode: 201, wantErr: false},
		{name: "204 overwrite", contentType: "application/pdf", statusCode: 204, wantErr: false},
		{name: "no content type", contentType: "", statusCode: 201, wantErr: false},
		{name: "507 insufficient storage", statusCode: 507, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotMethod, gotCT string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod = r.Method
				gotCT = r.Header.Get("Content-Type")
				w.WriteHeader(tt.statusCode)
			}))
			defer srv.Close()

			c := New(newTestConfig(srv.URL))
			err := Upload(context.Background(), c, "/dav/spaces/s1/file.txt", strings.NewReader("hello"), tt.contentType)
			if gotMethod != "PUT" {
				t.Errorf("method = %q, want PUT", gotMethod)
			}
			if tt.contentType != "" && gotCT != tt.contentType {
				t.Errorf("Content-Type = %q, want %q", gotCT, tt.contentType)
			}
			if (err != nil) != tt.wantErr {
				t.Errorf("Upload error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDownload(t *testing.T) {
	tests := []struct {
		name        string
		statusCode  int
		body        string
		contentType string
		wantErr     bool
	}{
		{name: "200 success", statusCode: 200, body: "file contents", contentType: "text/plain", wantErr: false},
		{name: "404 not found", statusCode: 404, wantErr: true},
		{name: "403 forbidden", statusCode: 403, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" {
					t.Errorf("method = %q, want GET", r.Method)
				}
				if tt.statusCode >= 400 {
					w.WriteHeader(tt.statusCode)
					return
				}
				w.Header().Set("Content-Type", tt.contentType)
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			c := New(newTestConfig(srv.URL))
			data, ct, err := Download(context.Background(), c, "/dav/spaces/s1/file.txt")
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(data) != tt.body {
				t.Errorf("body = %q, want %q", string(data), tt.body)
			}
			if ct != tt.contentType {
				t.Errorf("Content-Type = %q, want %q", ct, tt.contentType)
			}
		})
	}
}

func TestMove(t *testing.T) {
	tests := []struct {
		name          string
		overwrite     bool
		statusCode    int
		wantOverwrite string
		wantErr       bool
	}{
		{name: "201 with overwrite", overwrite: true, statusCode: 201, wantOverwrite: "T"},
		{name: "201 no overwrite", overwrite: false, statusCode: 201, wantOverwrite: "F"},
		{name: "409 conflict", overwrite: false, statusCode: 409, wantOverwrite: "F", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotMethod, gotDest, gotOverwrite string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod = r.Method
				gotDest = r.Header.Get("Destination")
				gotOverwrite = r.Header.Get("Overwrite")
				w.WriteHeader(tt.statusCode)
			}))
			defer srv.Close()

			c := New(newTestConfig(srv.URL))
			err := Move(context.Background(), c, "/dav/spaces/s1/old.txt", "/dav/spaces/s1/new.txt", tt.overwrite)
			if gotMethod != "MOVE" {
				t.Errorf("method = %q, want MOVE", gotMethod)
			}
			if !strings.HasSuffix(gotDest, "/dav/spaces/s1/new.txt") {
				t.Errorf("Destination = %q, want suffix /dav/spaces/s1/new.txt", gotDest)
			}
			if gotOverwrite != tt.wantOverwrite {
				t.Errorf("Overwrite = %q, want %q", gotOverwrite, tt.wantOverwrite)
			}
			if (err != nil) != tt.wantErr {
				t.Errorf("Move error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCopy(t *testing.T) {
	tests := []struct {
		name          string
		overwrite     bool
		statusCode    int
		wantOverwrite string
		wantErr       bool
	}{
		{name: "201 with overwrite", overwrite: true, statusCode: 201, wantOverwrite: "T"},
		{name: "201 no overwrite", overwrite: false, statusCode: 201, wantOverwrite: "F"},
		{name: "412 precondition failed", overwrite: false, statusCode: 412, wantOverwrite: "F", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotMethod, gotDest, gotOverwrite string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod = r.Method
				gotDest = r.Header.Get("Destination")
				gotOverwrite = r.Header.Get("Overwrite")
				w.WriteHeader(tt.statusCode)
			}))
			defer srv.Close()

			c := New(newTestConfig(srv.URL))
			err := Copy(context.Background(), c, "/dav/spaces/s1/src.txt", "/dav/spaces/s1/dst.txt", tt.overwrite)
			if gotMethod != "COPY" {
				t.Errorf("method = %q, want COPY", gotMethod)
			}
			if !strings.HasSuffix(gotDest, "/dav/spaces/s1/dst.txt") {
				t.Errorf("Destination = %q, want suffix /dav/spaces/s1/dst.txt", gotDest)
			}
			if gotOverwrite != tt.wantOverwrite {
				t.Errorf("Overwrite = %q, want %q", gotOverwrite, tt.wantOverwrite)
			}
			if (err != nil) != tt.wantErr {
				t.Errorf("Copy error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestWebDAVDelete(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantErr    bool
	}{
		{name: "204 success", statusCode: 204, wantErr: false},
		{name: "404 not found", statusCode: 404, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotMethod string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod = r.Method
				w.WriteHeader(tt.statusCode)
			}))
			defer srv.Close()

			c := New(newTestConfig(srv.URL))
			err := WebDAVDelete(context.Background(), c, "/dav/spaces/s1/file.txt")
			if gotMethod != "DELETE" {
				t.Errorf("method = %q, want DELETE", gotMethod)
			}
			if (err != nil) != tt.wantErr {
				t.Errorf("WebDAVDelete error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestProppatch(t *testing.T) {
	tests := []struct {
		name       string
		props      map[string]string
		statusCode int
		wantErr    bool
	}{
		{
			name:       "success",
			props:      map[string]string{"oc:tags": "important,review"},
			statusCode: 207,
			wantErr:    false,
		},
		{
			name:       "server error",
			props:      map[string]string{"oc:tags": "test"},
			statusCode: 500,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotMethod, gotContentType string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod = r.Method
				gotContentType = r.Header.Get("Content-Type")
				w.WriteHeader(tt.statusCode)
			}))
			defer srv.Close()

			c := New(newTestConfig(srv.URL))
			err := Proppatch(context.Background(), c, "/dav/spaces/s1/file.txt", tt.props)
			if gotMethod != "PROPPATCH" {
				t.Errorf("method = %q, want PROPPATCH", gotMethod)
			}
			if gotContentType != "application/xml" {
				t.Errorf("Content-Type = %q, want application/xml", gotContentType)
			}
			if (err != nil) != tt.wantErr {
				t.Errorf("Proppatch error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestXmlEscape(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "hello"},
		{"<script>", "&lt;script&gt;"},
		{"a & b", "a &amp; b"},
		{`"quotes"`, "&#34;quotes&#34;"},
		{"normal text", "normal text"},
	}

	for _, tt := range tests {
		got := xmlEscape(tt.input)
		if got != tt.want {
			t.Errorf("xmlEscape(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNameFromHref(t *testing.T) {
	tests := []struct {
		href string
		want string
	}{
		{"/dav/spaces/s1/Documents/report.pdf", "report.pdf"},
		{"/dav/spaces/s1/Documents/", "Documents"},
		{"/dav/spaces/s1/", "s1"},
		{"file.txt", "file.txt"},
	}

	for _, tt := range tests {
		got := nameFromHref(tt.href)
		if got != tt.want {
			t.Errorf("nameFromHref(%q) = %q, want %q", tt.href, got, tt.want)
		}
	}
}

func TestPropfindFixture(t *testing.T) {
	// Integration test using the full propfind_folder.xml fixture.
	body, err := os.ReadFile(filepath.Join(testdataDir(), "propfind_folder.xml"))
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusMultiStatus)
		w.Write(body)
	}))
	defer srv.Close()

	cfg := newTestConfig(srv.URL)
	c := New(cfg)

	ms, err := Propfind(context.Background(), c, "/dav/spaces/space1/Documents/", "1")
	if err != nil {
		t.Fatalf("Propfind failed: %v", err)
	}

	if len(ms.Responses) != 3 {
		t.Fatalf("expected 3 responses, got %d", len(ms.Responses))
	}

	// Verify parent folder
	parent := ms.Responses[0].ToFileInfo()
	if parent.Type != "folder" {
		t.Errorf("parent type = %q, want folder", parent.Type)
	}
	if parent.Name != "Documents" {
		t.Errorf("parent name = %q, want Documents", parent.Name)
	}

	// Verify child file
	child := ms.Responses[1].ToFileInfo()
	if child.Type != "file" {
		t.Errorf("child file type = %q, want file", child.Type)
	}
	if child.Name != "report.pdf" {
		t.Errorf("child name = %q, want report.pdf", child.Name)
	}
	if child.Size != 102400 {
		t.Errorf("child size = %d, want 102400", child.Size)
	}
	if child.ContentType != "application/pdf" {
		t.Errorf("child content type = %q, want application/pdf", child.ContentType)
	}

	// Verify subfolder
	subfolder := ms.Responses[2].ToFileInfo()
	if subfolder.Type != "folder" {
		t.Errorf("subfolder type = %q, want folder", subfolder.Type)
	}
	if subfolder.Name != "Drafts" {
		t.Errorf("subfolder name = %q, want Drafts", subfolder.Name)
	}
}
