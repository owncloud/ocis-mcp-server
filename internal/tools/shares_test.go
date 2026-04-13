package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/owncloud/ocis-mcp-server/internal/client"
)

func TestHandleCreateShare(t *testing.T) {
	tests := []struct {
		name    string
		spaceID string
		itemID  string
		wantErr bool
	}{
		{name: "success", spaceID: "s1", itemID: "item1"},
		{name: "empty space_id", spaceID: "", itemID: "item1", wantErr: true},
		{name: "empty item_id", spaceID: "s1", itemID: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(200)
				_, _ = w.Write([]byte(`{"permissions":[{"id":"p1"}]}`))
			}))
			defer srv.Close()

			c := client.New(newTestConfig(srv.URL))
			handler := handleCreateShare(c)
			_, _, err := handler(context.Background(), &mcp.CallToolRequest{}, CreateShareInput{
				SpaceID: tt.spaceID, ItemID: tt.itemID,
				Recipients: []string{"u1"}, Roles: []string{"viewer"},
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

func TestHandleCreateLink(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"id":"link1","link":{"type":"view","webUrl":"https://example.com/s/abc"}}`))
	}))
	defer srv.Close()

	c := client.New(newTestConfig(srv.URL))
	handler := handleCreateLink(c)
	_, output, err := handler(context.Background(), &mcp.CallToolRequest{}, CreateLinkInput{
		SpaceID: "s1", ItemID: "item1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.ID != "link1" {
		t.Errorf("ID = %q, want link1", output.ID)
	}
}

func TestHandleListShares(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"value":[{"id":"p1"},{"id":"p2"}]}`))
	}))
	defer srv.Close()

	c := client.New(newTestConfig(srv.URL))
	handler := handleListShares(c)
	_, output, err := handler(context.Background(), &mcp.CallToolRequest{}, ListSharesInput{
		SpaceID: "s1", ItemID: "item1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(output.Permissions) != 2 {
		t.Errorf("len(Permissions) = %d, want 2", len(output.Permissions))
	}
}

func TestHandleUpdateShare(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"id":"p1","roles":["editor"]}`))
	}))
	defer srv.Close()

	c := client.New(newTestConfig(srv.URL))
	handler := handleUpdateShare(c)
	_, output, err := handler(context.Background(), &mcp.CallToolRequest{}, UpdateShareInput{
		SpaceID: "s1", ItemID: "item1", PermissionID: "p1", Roles: []string{"editor"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.ID != "p1" {
		t.Errorf("ID = %q, want p1", output.ID)
	}
}

func TestHandleUpdateShareExpiration(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"id":"p1","expirationDateTime":"2025-12-31T00:00:00Z"}`))
	}))
	defer srv.Close()

	c := client.New(newTestConfig(srv.URL))
	handler := handleUpdateShareExpiration(c)
	_, output, err := handler(context.Background(), &mcp.CallToolRequest{}, UpdateShareExpirationInput{
		SpaceID: "s1", ItemID: "item1", PermissionID: "p1", Expiration: "2025-12-31T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.ExpirationDateTime != "2025-12-31T00:00:00Z" {
		t.Errorf("Expiration = %q", output.ExpirationDateTime)
	}
}

func TestHandleDeleteShare(t *testing.T) {
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
			handler := handleDeleteShare(c)
			_, _, err := handler(context.Background(), &mcp.CallToolRequest{}, DeleteShareInput{
				SpaceID: "s1", ItemID: "item1", PermissionID: "p1", Confirm: tt.confirm,
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

func TestHandleListSharedByMe(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"value":[{"id":"i1","name":"doc.pdf"}]}`))
	}))
	defer srv.Close()

	c := client.New(newTestConfig(srv.URL))
	handler := handleListSharedByMe(c)
	_, output, err := handler(context.Background(), &mcp.CallToolRequest{}, ListSharedByMeInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.TotalCount != 1 {
		t.Errorf("TotalCount = %d, want 1", output.TotalCount)
	}
}

func TestHandleListReceivedShares(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"value":[]}`))
	}))
	defer srv.Close()

	c := client.New(newTestConfig(srv.URL))
	handler := handleListReceivedShares(c)
	_, output, err := handler(context.Background(), &mcp.CallToolRequest{}, ListSharedByMeInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.TotalCount != 0 {
		t.Errorf("TotalCount = %d, want 0", output.TotalCount)
	}
}

func TestHandleAcceptShare(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := client.New(newTestConfig(srv.URL))
	handler := handleAcceptShare(c)
	_, output, err := handler(context.Background(), &mcp.CallToolRequest{}, AcceptShareInput{
		SpaceID: "s1", ItemID: "item1", PermissionID: "p1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !output.Success {
		t.Error("expected Success=true")
	}
}

func TestHandleRejectShare(t *testing.T) {
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
			handler := handleRejectShare(c)
			_, _, err := handler(context.Background(), &mcp.CallToolRequest{}, DeleteShareInput{
				SpaceID: "s1", ItemID: "item1", PermissionID: "p1", Confirm: tt.confirm,
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

func TestHandleGetSharingRoles(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"value":[{"id":"r1","displayName":"Viewer"},{"id":"r2","displayName":"Editor"}]}`))
	}))
	defer srv.Close()

	c := client.New(newTestConfig(srv.URL))
	handler := handleGetSharingRoles(c)
	_, output, err := handler(context.Background(), &mcp.CallToolRequest{}, GetSharingRolesInput{SpaceID: "s1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(output.Roles) != 2 {
		t.Errorf("len(Roles) = %d, want 2", len(output.Roles))
	}
}
