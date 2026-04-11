package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/owncloud/ocis-mcp-server/internal/client"
)

const searchReportResponse = `<?xml version="1.0" encoding="UTF-8"?>
<d:multistatus xmlns:d="DAV:" xmlns:oc="http://owncloud.org/ns">
  <d:response>
    <d:href>/dav/spaces/s1/report.pdf</d:href>
    <d:propstat>
      <d:prop>
        <d:displayname>report.pdf</d:displayname>
        <d:resourcetype/>
        <d:getcontentlength>2048</d:getcontentlength>
        <d:getcontenttype>application/pdf</d:getcontenttype>
      </d:prop>
      <d:status>HTTP/1.1 200 OK</d:status>
    </d:propstat>
  </d:response>
</d:multistatus>`

func TestHandleSearch(t *testing.T) {
	tests := []struct {
		name       string
		pattern    string
		statusCode int
		body       string
		wantCount  int
		wantErr    bool
	}{
		{
			name:       "success finds files",
			pattern:    "report",
			statusCode: 207,
			body:       searchReportResponse,
			wantCount:  1,
		},
		{
			name:    "empty pattern",
			pattern: "",
			wantErr: true,
		},
		{
			name:       "empty results",
			pattern:    "nothing",
			statusCode: 207,
			body:       `<?xml version="1.0"?><d:multistatus xmlns:d="DAV:"></d:multistatus>`,
			wantCount:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/xml")
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			c := client.New(newTestConfig(srv.URL))
			handler := handleSearch(c)
			_, output, err := handler(context.Background(), &mcp.CallToolRequest{}, SearchInput{
				Pattern: tt.pattern,
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
			if output.TotalCount != tt.wantCount {
				t.Errorf("TotalCount = %d, want %d", output.TotalCount, tt.wantCount)
			}
		})
	}
}

func TestHandleSearchByTag(t *testing.T) {
	tests := []struct {
		name    string
		tag     string
		wantErr bool
	}{
		{
			name: "success",
			tag:  "important",
		},
		{
			name:    "empty tag",
			tag:     "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/xml")
				w.WriteHeader(207)
				w.Write([]byte(`<?xml version="1.0"?><d:multistatus xmlns:d="DAV:"></d:multistatus>`))
			}))
			defer srv.Close()

			c := client.New(newTestConfig(srv.URL))
			handler := handleSearchByTag(c)
			_, _, err := handler(context.Background(), &mcp.CallToolRequest{}, SearchByTagInput{
				Tag: tt.tag,
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

func TestSearchPath(t *testing.T) {
	tests := []struct {
		spaceID string
		want    string
	}{
		{"", "/dav/spaces/"},
		{"s1", "/dav/spaces/s1/"},
	}
	for _, tt := range tests {
		got := searchPath(tt.spaceID)
		if got != tt.want {
			t.Errorf("searchPath(%q) = %q, want %q", tt.spaceID, got, tt.want)
		}
	}
}
