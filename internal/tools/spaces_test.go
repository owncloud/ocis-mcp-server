package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/owncloud/ocis-mcp-server/internal/client"
)

func TestHandleListSpaces(t *testing.T) {
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
			body:       `{"value":[{"id":"s1","name":"Space1","driveType":"project"},{"id":"s2","name":"Space2","driveType":"personal"}]}`,
			wantCount:  2,
		},
		{
			name:       "empty",
			statusCode: 200,
			body:       `{"value":[]}`,
			wantCount:  0,
		},
		{
			name:       "error",
			statusCode: 403,
			body:       `{}`,
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
			handler := handleListSpaces(c, "/graph/v1.0/drives")
			_, output, err := handler(context.Background(), &mcp.CallToolRequest{}, ListSpacesInput{})

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

func TestHandleGetSpace(t *testing.T) {
	tests := []struct {
		name       string
		spaceID    string
		statusCode int
		body       string
		wantName   string
		wantErr    bool
	}{
		{
			name:       "success",
			spaceID:    "s1",
			statusCode: 200,
			body:       `{"id":"s1","name":"My Project","driveType":"project"}`,
			wantName:   "My Project",
		},
		{name: "empty space_id", spaceID: "", wantErr: true},
		{name: "not found", spaceID: "bad", statusCode: 404, body: `{}`, wantErr: true},
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
			handler := handleGetSpace(c)
			_, output, err := handler(context.Background(), &mcp.CallToolRequest{}, GetSpaceInput{SpaceID: tt.spaceID})

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

func TestHandleCreateSpace(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"id":"new-s","name":"Test Space","driveType":"project"}`))
	}))
	defer srv.Close()

	c := client.New(newTestConfig(srv.URL))
	handler := handleCreateSpace(c)
	_, output, err := handler(context.Background(), &mcp.CallToolRequest{}, CreateSpaceInput{
		Name: "Test Space", Description: "A test", Quota: 1073741824,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.Name != "Test Space" {
		t.Errorf("Name = %q, want Test Space", output.Name)
	}
}

func TestHandleUpdateSpace(t *testing.T) {
	name := "Renamed"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"id":"s1","name":"Renamed","driveType":"project"}`))
	}))
	defer srv.Close()

	c := client.New(newTestConfig(srv.URL))
	handler := handleUpdateSpace(c)
	_, output, err := handler(context.Background(), &mcp.CallToolRequest{}, UpdateSpaceInput{
		SpaceID: "s1", Name: &name,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.Name != "Renamed" {
		t.Errorf("Name = %q, want Renamed", output.Name)
	}
}

func TestHandleDisableSpace(t *testing.T) {
	tests := []struct {
		name    string
		spaceID string
		confirm bool
		wantErr bool
	}{
		{name: "success", spaceID: "s1", confirm: true},
		{name: "no confirm", spaceID: "s1", confirm: false, wantErr: true},
		{name: "empty id", spaceID: "", confirm: true, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(204)
			}))
			defer srv.Close()

			c := client.New(newTestConfig(srv.URL))
			handler := handleDisableSpace(c)
			_, _, err := handler(context.Background(), &mcp.CallToolRequest{}, DisableSpaceInput{
				SpaceID: tt.spaceID, Confirm: tt.confirm,
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

func TestHandleDeleteSpace(t *testing.T) {
	tests := []struct {
		name    string
		spaceID string
		confirm bool
		wantErr bool
	}{
		{name: "success", spaceID: "s1", confirm: true},
		{name: "no confirm", spaceID: "s1", confirm: false, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Purge") != "T" && tt.confirm {
					t.Error("expected Purge: T header")
				}
				w.WriteHeader(204)
			}))
			defer srv.Close()

			c := client.New(newTestConfig(srv.URL))
			handler := handleDeleteSpace(c)
			_, _, err := handler(context.Background(), &mcp.CallToolRequest{}, DeleteSpaceInput{
				SpaceID: tt.spaceID, Confirm: tt.confirm,
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

func TestHandleRestoreSpace(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"id":"s1","name":"Restored","driveType":"project"}`))
	}))
	defer srv.Close()

	c := client.New(newTestConfig(srv.URL))
	handler := handleRestoreSpace(c)
	_, output, err := handler(context.Background(), &mcp.CallToolRequest{}, RestoreSpaceInput{SpaceID: "s1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.Name != "Restored" {
		t.Errorf("Name = %q, want Restored", output.Name)
	}
}

func TestHandleListSpacePermissions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"value":[{"id":"p1","roles":["viewer"]}]}`))
	}))
	defer srv.Close()

	c := client.New(newTestConfig(srv.URL))
	handler := handleListSpacePermissions(c)
	_, output, err := handler(context.Background(), &mcp.CallToolRequest{}, ListSpacePermissionsInput{SpaceID: "s1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(output.Permissions) != 1 {
		t.Errorf("len(Permissions) = %d, want 1", len(output.Permissions))
	}
}

func TestHandleEmptyTrashbin(t *testing.T) {
	tests := []struct {
		name    string
		spaceID string
		confirm bool
		wantErr bool
	}{
		{name: "success", spaceID: "s1", confirm: true},
		{name: "no confirm", spaceID: "s1", confirm: false, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(204)
			}))
			defer srv.Close()

			c := client.New(newTestConfig(srv.URL))
			handler := handleEmptyTrashbin(c)
			_, _, err := handler(context.Background(), &mcp.CallToolRequest{}, EmptyTrashbinInput{
				SpaceID: tt.spaceID, Confirm: tt.confirm,
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

func TestHandleInviteToSpace(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"permissions":[{"id":"p1"}]}`))
	}))
	defer srv.Close()

	c := client.New(newTestConfig(srv.URL))
	handler := handleInviteToSpace(c)
	_, output, err := handler(context.Background(), &mcp.CallToolRequest{}, InviteToSpaceInput{
		SpaceID: "s1", Recipients: []string{"u1"}, Roles: []string{"viewer"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(output.Permissions) != 1 {
		t.Errorf("len(Permissions) = %d, want 1", len(output.Permissions))
	}
}

func TestHandleCreateSpaceLink(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"id":"link1","link":{"type":"view","webUrl":"https://example.com/s/abc"}}`))
	}))
	defer srv.Close()

	c := client.New(newTestConfig(srv.URL))
	handler := handleCreateSpaceLink(c)
	_, output, err := handler(context.Background(), &mcp.CallToolRequest{}, CreateSpaceLinkInput{SpaceID: "s1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.ID != "link1" {
		t.Errorf("ID = %q, want link1", output.ID)
	}
}
