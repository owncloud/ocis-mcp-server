package tools

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/owncloud/ocis-mcp-server/internal/client"
)

// Issue #15: a plain-text pattern like "elmo" must match "burning_elmo.gif"
// (substring), so wildcard-free patterns are wrapped in *...*. Patterns that
// already use glob wildcards are passed through unchanged.
func TestHandleSearchWrapsPlainPattern(t *testing.T) {
	tests := []struct {
		name        string
		pattern     string
		wantPattern string
	}{
		{"plain text wrapped for substring match", "elmo", "*elmo*"},
		{"star glob passed through", "*.gif", "*.gif"},
		{"question-mark glob passed through", "file?.txt", "file?.txt"},
		{"already wrapped passed through", "*elmo*", "*elmo*"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotBody string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				b, _ := io.ReadAll(r.Body)
				gotBody = string(b)
				w.Header().Set("Content-Type", "application/xml")
				w.WriteHeader(207)
				_, _ = w.Write([]byte(`<?xml version="1.0"?><d:multistatus xmlns:d="DAV:"></d:multistatus>`))
			}))
			defer srv.Close()

			c := client.New(newTestConfig(srv.URL))
			handler := handleSearch(c)
			if _, _, err := handler(context.Background(), &mcp.CallToolRequest{}, SearchInput{Pattern: tt.pattern}); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			want := "<oc:pattern>" + tt.wantPattern + "</oc:pattern>"
			if !strings.Contains(gotBody, want) {
				t.Errorf("REPORT body missing %q\ngot body:\n%s", want, gotBody)
			}
		})
	}
}
