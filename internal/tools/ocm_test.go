package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/owncloud/ocis-mcp-server/internal/client"
)

func TestHandleListOCMProviders(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantCount  int
		wantErr    bool
	}{
		{
			name:       "success",
			statusCode: 200,
			body:       `[{"name":"Remote","domain":"remote.example.com"}]`,
			wantCount:  1,
		},
		{
			name:       "empty",
			statusCode: 200,
			body:       `null`,
			wantCount:  0,
		},
		{
			name:       "error",
			statusCode: 500,
			body:       `{}`,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			c := client.New(newTestConfig(srv.URL))
			handler := handleListOCMProviders(c)
			_, output, err := handler(context.Background(), &mcp.CallToolRequest{}, ListOCMProvidersInput{})

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(output.Providers) != tt.wantCount {
				t.Errorf("len(Providers) = %d, want %d", len(output.Providers), tt.wantCount)
			}
		})
	}
}

func TestHandleCreateOCMShare(t *testing.T) {
	tests := []struct {
		name       string
		resourceID string
		granteeID  string
		granteeIdp string
		wantErr    bool
	}{
		{name: "success", resourceID: "r1", granteeID: "u1", granteeIdp: "remote.example.com"},
		{name: "empty resource_id", resourceID: "", granteeID: "u1", granteeIdp: "remote.example.com", wantErr: true},
		{name: "empty grantee_id", resourceID: "r1", granteeID: "", granteeIdp: "remote.example.com", wantErr: true},
		{name: "empty grantee_idp", resourceID: "r1", granteeID: "u1", granteeIdp: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(201)
				w.Write([]byte(`{"id":"ocm1","name":"shared-file"}`))
			}))
			defer srv.Close()

			c := client.New(newTestConfig(srv.URL))
			handler := handleCreateOCMShare(c)
			_, _, err := handler(context.Background(), &mcp.CallToolRequest{}, CreateOCMShareInput{
				ResourceID: tt.resourceID, GranteeID: tt.granteeID, GranteeIdp: tt.granteeIdp,
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

func TestHandleListOCMShares(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`[{"id":"ocm1"},{"id":"ocm2"}]`))
	}))
	defer srv.Close()

	c := client.New(newTestConfig(srv.URL))
	handler := handleListOCMShares(c)
	_, output, err := handler(context.Background(), &mcp.CallToolRequest{}, ListOCMSharesInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(output.Shares) != 2 {
		t.Errorf("len(Shares) = %d, want 2", len(output.Shares))
	}
}

func TestHandleListOCMReceived(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`null`))
	}))
	defer srv.Close()

	c := client.New(newTestConfig(srv.URL))
	handler := handleListOCMReceived(c)
	_, output, err := handler(context.Background(), &mcp.CallToolRequest{}, ListOCMReceivedInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(output.Shares) != 0 {
		t.Errorf("len(Shares) = %d, want 0", len(output.Shares))
	}
}
