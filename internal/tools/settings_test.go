package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/owncloud/ocis-mcp-server/internal/client"
)

func TestHandleListRoles(t *testing.T) {
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
			body:       `{"bundles":[{"id":"r1","name":"Admin"},{"id":"r2","name":"User"}]}`,
			wantCount:  2,
		},
		{
			name:       "empty",
			statusCode: 200,
			body:       `{"bundles":null}`,
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
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			c := client.New(newTestConfig(srv.URL))
			handler := handleListRoles(c)
			_, output, err := handler(context.Background(), &mcp.CallToolRequest{}, ListRolesInput{})

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(output.Roles) != tt.wantCount {
				t.Errorf("len(Roles) = %d, want %d", len(output.Roles), tt.wantCount)
			}
		})
	}
}

func TestHandleAssignRole(t *testing.T) {
	tests := []struct {
		name    string
		userID  string
		roleID  string
		wantErr bool
	}{
		{name: "success", userID: "u1", roleID: "r1"},
		{name: "empty user_id", userID: "", roleID: "r1", wantErr: true},
		{name: "empty role_id", userID: "u1", roleID: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(200)
				_, _ = w.Write([]byte(`{"id":"a1","accountUuid":"u1","roleId":"r1"}`))
			}))
			defer srv.Close()

			c := client.New(newTestConfig(srv.URL))
			handler := handleAssignRole(c)
			_, _, err := handler(context.Background(), &mcp.CallToolRequest{}, AssignRoleInput{
				UserID: tt.userID, RoleID: tt.roleID,
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

func TestHandleListAssignments(t *testing.T) {
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
			body:       `{"assignments":[{"accountUuid":"u1","roleId":"r1"}]}`,
			wantCount:  1,
		},
		{
			name:       "empty",
			statusCode: 200,
			body:       `{"assignments":null}`,
			wantCount:  0,
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
			handler := handleListAssignments(c)
			_, output, err := handler(context.Background(), &mcp.CallToolRequest{}, ListAssignmentsInput{})

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(output.Assignments) != tt.wantCount {
				t.Errorf("len(Assignments) = %d, want %d", len(output.Assignments), tt.wantCount)
			}
		})
	}
}
