package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/owncloud/ocis-mcp-server/internal/client"
)

func TestHandleListEducationSchools(t *testing.T) {
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
			body:       `{"value":[{"id":"sch1","displayName":"Test School"}]}`,
			wantCount:  1,
		},
		{
			name:       "empty",
			statusCode: 200,
			body:       `{"value":null}`,
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
			handler := handleListEducationSchools(c)
			_, output, err := handler(context.Background(), &mcp.CallToolRequest{}, ListEducationSchoolsInput{})

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(output.Schools) != tt.wantCount {
				t.Errorf("len(Schools) = %d, want %d", len(output.Schools), tt.wantCount)
			}
		})
	}
}

func TestHandleGetEducationSchool(t *testing.T) {
	tests := []struct {
		name     string
		schoolID string
		wantErr  bool
	}{
		{name: "success", schoolID: "sch1"},
		{name: "empty id", schoolID: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(200)
				_, _ = w.Write([]byte(`{"id":"sch1","displayName":"Test School"}`))
			}))
			defer srv.Close()

			c := client.New(newTestConfig(srv.URL))
			handler := handleGetEducationSchool(c)
			_, _, err := handler(context.Background(), &mcp.CallToolRequest{}, GetEducationSchoolInput{SchoolID: tt.schoolID})
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

func TestHandleListEducationUsers(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"value":[{"id":"eu1","displayName":"Student A"}]}`))
	}))
	defer srv.Close()

	c := client.New(newTestConfig(srv.URL))
	handler := handleListEducationUsers(c)
	_, output, err := handler(context.Background(), &mcp.CallToolRequest{}, ListEducationUsersInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(output.Users) != 1 {
		t.Errorf("len(Users) = %d, want 1", len(output.Users))
	}
}

func TestHandleGetEducationUser(t *testing.T) {
	tests := []struct {
		name    string
		userID  string
		wantErr bool
	}{
		{name: "success", userID: "eu1"},
		{name: "empty id", userID: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(200)
				_, _ = w.Write([]byte(`{"id":"eu1","displayName":"Student A"}`))
			}))
			defer srv.Close()

			c := client.New(newTestConfig(srv.URL))
			handler := handleGetEducationUser(c)
			_, _, err := handler(context.Background(), &mcp.CallToolRequest{}, GetEducationUserInput{UserID: tt.userID})
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

func TestHandleCreateEducationUser(t *testing.T) {
	tests := []struct {
		name        string
		displayName string
		username    string
		mail        string
		wantErr     bool
	}{
		{name: "success", displayName: "Student", username: "student1", mail: "s@example.com"},
		{name: "empty display_name", displayName: "", username: "s1", mail: "s@e.com", wantErr: true},
		{name: "empty username", displayName: "S", username: "", mail: "s@e.com", wantErr: true},
		{name: "empty mail", displayName: "S", username: "s1", mail: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(201)
				_, _ = w.Write([]byte(`{"id":"eu-new","displayName":"Student"}`))
			}))
			defer srv.Close()

			c := client.New(newTestConfig(srv.URL))
			handler := handleCreateEducationUser(c)
			_, _, err := handler(context.Background(), &mcp.CallToolRequest{}, CreateEducationUserInput{
				DisplayName: tt.displayName, Username: tt.username, Mail: tt.mail, Password: "pass",
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
