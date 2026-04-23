package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/owncloud/ocis-mcp-server/internal/client"
)

func TestCapabilitiesHandler(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantErr    bool
	}{
		{
			name:       "success",
			statusCode: 200,
			body:       `{"ocs":{"data":{"capabilities":{"core":{}}}}}`,
		},
		{
			name:       "server error",
			statusCode: 500,
			body:       "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			c := client.New(newTestConfig(srv.URL))
			handler := capabilitiesHandler(c)
			result, err := handler(context.Background(), &mcp.ReadResourceRequest{
				Params: &mcp.ReadResourceParams{URI: "ocis://capabilities"},
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
			if len(result.Contents) != 1 {
				t.Fatalf("expected 1 content, got %d", len(result.Contents))
			}
			if result.Contents[0].URI != "ocis://capabilities" {
				t.Errorf("URI = %q", result.Contents[0].URI)
			}
		})
	}
}

func TestVersionHandler(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"ocs":{"data":{"version":{"major":8,"string":"8.0.1","edition":"Community"}}}}`))
	}))
	defer srv.Close()

	c := client.New(newTestConfig(srv.URL))
	handler := versionHandler(c)
	result, err := handler(context.Background(), &mcp.ReadResourceRequest{
		Params: &mcp.ReadResourceParams{URI: "ocis://version"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Contents[0].Text, "8.0.1") {
		t.Errorf("version text doesn't contain 8.0.1: %s", result.Contents[0].Text)
	}
}

func TestDriveTypesHandler(t *testing.T) {
	handler := driveTypesHandler()
	result, err := handler(context.Background(), &mcp.ReadResourceRequest{
		Params: &mcp.ReadResourceParams{URI: "ocis://drive-types"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := result.Contents[0].Text
	if !strings.Contains(text, "personal") || !strings.Contains(text, "project") {
		t.Errorf("drive types missing expected types: %s", text)
	}
}

func TestAuthModeHandler(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	cfg := newTestConfig(srv.URL)
	c := client.New(cfg)
	handler := authModeHandler(c)
	result, err := handler(context.Background(), &mcp.ReadResourceRequest{
		Params: &mcp.ReadResourceParams{URI: "ocis://auth-mode"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := result.Contents[0].Text
	if !strings.Contains(text, "app-token") {
		t.Errorf("auth mode text doesn't contain 'app-token': %s", text)
	}
}

func TestSharingRolesHandler(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		// roleDefinitions returns a plain JSON array, not {value:[...]}
		_, _ = w.Write([]byte(`[{"id":"r1","displayName":"Viewer"},{"id":"r2","displayName":"Editor"}]`))
	}))
	defer srv.Close()

	c := client.New(newTestConfig(srv.URL))
	handler := sharingRolesHandler(c)
	result, err := handler(context.Background(), &mcp.ReadResourceRequest{
		Params: &mcp.ReadResourceParams{URI: "ocis://sharing-roles"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Contents[0].Text, "Viewer") {
		t.Errorf("sharing roles text doesn't contain 'Viewer': %s", result.Contents[0].Text)
	}
}

func TestTextResource(t *testing.T) {
	result := textResource("ocis://test", `{"key":"value"}`)
	if len(result.Contents) != 1 {
		t.Fatalf("expected 1 content, got %d", len(result.Contents))
	}
	if result.Contents[0].URI != "ocis://test" {
		t.Errorf("URI = %q", result.Contents[0].URI)
	}
	if result.Contents[0].MIMEType != "application/json" {
		t.Errorf("MIMEType = %q", result.Contents[0].MIMEType)
	}
}
