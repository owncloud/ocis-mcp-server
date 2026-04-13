package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/owncloud/ocis-mcp-server/internal/client"
)

func TestHandleListGroups(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantCount  int
		wantErr    bool
	}{
		{
			name:       "success returns groups",
			statusCode: 200,
			body:       `{"value":[{"id":"g1","displayName":"Group1"},{"id":"g2","displayName":"Group2"}]}`,
			wantCount:  2,
		},
		{
			name:       "empty result",
			statusCode: 200,
			body:       `{"value":[]}`,
			wantCount:  0,
		},
		{
			name:       "server error",
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
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			c := client.New(newTestConfig(srv.URL))
			handler := handleListGroups(c)
			_, output, err := handler(context.Background(), &mcp.CallToolRequest{}, ListGroupsInput{})

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

func TestHandleGetGroup(t *testing.T) {
	tests := []struct {
		name       string
		groupID    string
		statusCode int
		body       string
		wantName   string
		wantErr    bool
	}{
		{
			name:       "success",
			groupID:    "g1",
			statusCode: 200,
			body:       `{"id":"g1","displayName":"Engineers","members":[{"id":"u1","displayName":"Alice"}]}`,
			wantName:   "Engineers",
		},
		{
			name:    "empty group_id",
			groupID: "",
			wantErr: true,
		},
		{
			name:       "not found",
			groupID:    "missing",
			statusCode: 404,
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
			handler := handleGetGroup(c)
			_, output, err := handler(context.Background(), &mcp.CallToolRequest{}, GetGroupInput{GroupID: tt.groupID})

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if output.DisplayName != tt.wantName {
				t.Errorf("DisplayName = %q, want %q", output.DisplayName, tt.wantName)
			}
		})
	}
}

func TestHandleCreateGroup(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"id":"new-g","displayName":"New Group"}`))
	}))
	defer srv.Close()

	c := client.New(newTestConfig(srv.URL))
	handler := handleCreateGroup(c)
	_, output, err := handler(context.Background(), &mcp.CallToolRequest{}, CreateGroupInput{
		DisplayName: "New Group",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.DisplayName != "New Group" {
		t.Errorf("DisplayName = %q, want New Group", output.DisplayName)
	}
}

func TestHandleDeleteGroup(t *testing.T) {
	tests := []struct {
		name    string
		groupID string
		confirm bool
		wantErr bool
	}{
		{name: "success", groupID: "g1", confirm: true},
		{name: "no confirm", groupID: "g1", confirm: false, wantErr: true},
		{name: "empty id", groupID: "", confirm: true, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(204)
			}))
			defer srv.Close()

			c := client.New(newTestConfig(srv.URL))
			handler := handleDeleteGroup(c)
			_, output, err := handler(context.Background(), &mcp.CallToolRequest{}, DeleteGroupInput{
				GroupID: tt.groupID, Confirm: tt.confirm,
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
			if !output.Success {
				t.Error("expected Success=true")
			}
		})
	}
}

func TestHandleAddGroupMember(t *testing.T) {
	tests := []struct {
		name    string
		groupID string
		userID  string
		wantErr bool
	}{
		{name: "success", groupID: "g1", userID: "u1"},
		{name: "empty group_id", groupID: "", userID: "u1", wantErr: true},
		{name: "empty user_id", groupID: "g1", userID: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(204)
			}))
			defer srv.Close()

			c := client.New(newTestConfig(srv.URL))
			handler := handleAddGroupMember(c)
			_, output, err := handler(context.Background(), &mcp.CallToolRequest{}, AddGroupMemberInput{
				GroupID: tt.groupID, UserID: tt.userID,
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
			if !output.Success {
				t.Error("expected Success=true")
			}
		})
	}
}

func TestHandleRemoveGroupMember(t *testing.T) {
	tests := []struct {
		name    string
		groupID string
		userID  string
		confirm bool
		wantErr bool
	}{
		{name: "success", groupID: "g1", userID: "u1", confirm: true},
		{name: "no confirm", groupID: "g1", userID: "u1", confirm: false, wantErr: true},
		{name: "empty group_id", groupID: "", userID: "u1", confirm: true, wantErr: true},
		{name: "empty user_id", groupID: "g1", userID: "", confirm: true, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(204)
			}))
			defer srv.Close()

			c := client.New(newTestConfig(srv.URL))
			handler := handleRemoveGroupMember(c)
			_, _, err := handler(context.Background(), &mcp.CallToolRequest{}, RemoveGroupMemberInput{
				GroupID: tt.groupID, UserID: tt.userID, Confirm: tt.confirm,
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

func TestHandleUpdateGroup(t *testing.T) {
	name := "Updated"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"id":"g1","displayName":"Updated"}`))
	}))
	defer srv.Close()

	c := client.New(newTestConfig(srv.URL))
	handler := handleUpdateGroup(c)
	_, output, err := handler(context.Background(), &mcp.CallToolRequest{}, UpdateGroupInput{
		GroupID: "g1", DisplayName: &name,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.DisplayName != "Updated" {
		t.Errorf("DisplayName = %q, want Updated", output.DisplayName)
	}
}
