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

func TestHandleShareWithLink(t *testing.T) {
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
				_, _ = w.Write([]byte(`{"id":"link1","link":{"type":"view","webUrl":"https://example.com/s/abc"}}`))
			}))
			defer srv.Close()

			c := client.New(newTestConfig(srv.URL))
			handler := handleShareWithLink(c)
			_, output, err := handler(context.Background(), &mcp.CallToolRequest{}, ShareWithLinkInput{
				SpaceID: tt.spaceID, ItemID: tt.itemID,
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
			if output.URL != "https://example.com/s/abc" {
				t.Errorf("URL = %q", output.URL)
			}
		})
	}
}

func TestHandleGetSpaceOverview(t *testing.T) {
	requestCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if strings.Contains(r.URL.Path, "/graph/v1.0/drives/") && !strings.Contains(r.URL.Path, "permissions") {
			// GetJSON for space
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"id":"s1","name":"Project","driveType":"project"}`))
			return
		}
		if strings.Contains(r.URL.Path, "permissions") {
			// ListJSON for permissions
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"value":[{"id":"p1"}]}`))
			return
		}
		// PROPFIND for files
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(207)
		_, _ = w.Write([]byte(propfindResponse))
	}))
	defer srv.Close()

	c := client.New(newTestConfig(srv.URL))
	handler := handleGetSpaceOverview(c)
	_, output, err := handler(context.Background(), &mcp.CallToolRequest{}, GetSpaceOverviewInput{SpaceID: "s1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.Space.Name != "Project" {
		t.Errorf("Space.Name = %q, want Project", output.Space.Name)
	}
}

func TestHandleGetSpaceOverviewValidation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := client.New(newTestConfig(srv.URL))
	handler := handleGetSpaceOverview(c)
	_, _, err := handler(context.Background(), &mcp.CallToolRequest{}, GetSpaceOverviewInput{SpaceID: ""})
	if err == nil {
		t.Fatal("expected validation error for empty space_id")
	}
}

func TestHandleCreateProjectSpace(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "invite") {
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"permissions":[{"id":"p1"}]}`))
			return
		}
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"id":"new-s","name":"Created","driveType":"project"}`))
	}))
	defer srv.Close()

	c := client.New(newTestConfig(srv.URL))
	handler := handleCreateProjectSpace(c)
	_, output, err := handler(context.Background(), &mcp.CallToolRequest{}, CreateProjectSpaceInput{
		Name: "Created", Members: []string{"u1"}, MemberRoles: []string{"viewer"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.Space.Name != "Created" {
		t.Errorf("Space.Name = %q, want Created", output.Space.Name)
	}
	if output.Message != "space created with members" {
		t.Errorf("Message = %q", output.Message)
	}
}

func TestHandleCreateProjectSpaceNoMembers(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"id":"new-s","name":"Solo","driveType":"project"}`))
	}))
	defer srv.Close()

	c := client.New(newTestConfig(srv.URL))
	handler := handleCreateProjectSpace(c)
	_, output, err := handler(context.Background(), &mcp.CallToolRequest{}, CreateProjectSpaceInput{Name: "Solo"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.Message != "space created" {
		t.Errorf("Message = %q, want 'space created'", output.Message)
	}
}

func TestHandleUploadAndShare(t *testing.T) {
	requestCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if r.Method == "PUT" {
			w.WriteHeader(201)
			return
		}
		if r.Method == "PROPFIND" {
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(207)
			_, _ = w.Write([]byte(propfindSingleFile))
			return
		}
		// POST invite
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"permissions":[{"id":"p1"}]}`))
	}))
	defer srv.Close()

	c := client.New(newTestConfig(srv.URL))
	handler := handleUploadAndShare(c)
	_, output, err := handler(context.Background(), &mcp.CallToolRequest{}, UploadAndShareInput{
		SpaceID: "s1", Path: "/test.txt", Content: "hello",
		Recipients: []string{"u1"}, Roles: []string{"viewer"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !output.FileUploaded {
		t.Error("expected FileUploaded=true")
	}
}

func TestHandleUploadAndShareValidation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := client.New(newTestConfig(srv.URL))
	handler := handleUploadAndShare(c)
	_, _, err := handler(context.Background(), &mcp.CallToolRequest{}, UploadAndShareInput{
		SpaceID: "", Path: "/test.txt",
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
}
