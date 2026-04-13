package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/owncloud/ocis-mcp-server/internal/client"
	"github.com/owncloud/ocis-mcp-server/internal/config"
)

func testdataDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "client", "testdata")
}

func newTestConfig(srvURL string) *config.Config {
	return &config.Config{
		OcisURL:       srvURL,
		AuthMode:      "app-token",
		AppTokenUser:  "admin",
		AppTokenValue: "test-token",
		Transport:     "stdio",
		HTTPAddr:      "127.0.0.1:8090",
		LogLevel:      "info",
		HTTPTimeout:   30 * time.Second,
		Insecure:      true,
	}
}

func TestHandleListUsers(t *testing.T) {
	tests := []struct {
		name       string
		fixture    string
		statusCode int
		wantCount  int
		wantErr    bool
	}{
		{
			name:       "success returns 3 users",
			fixture:    "graph_users.json",
			statusCode: 200,
			wantCount:  3,
		},
		{
			name:       "401 unauthorized returns error",
			fixture:    "",
			statusCode: 401,
			wantErr:    true,
		},
		{
			name:       "empty result returns 0 users",
			fixture:    "",
			statusCode: 200,
			wantCount:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body []byte
			if tt.fixture != "" {
				var err error
				body, err = os.ReadFile(filepath.Join(testdataDir(), tt.fixture))
				if err != nil {
					t.Fatalf("reading fixture: %v", err)
				}
			} else if tt.statusCode == 200 {
				body = []byte(`{"value":[]}`)
			} else {
				body = []byte(`{}`)
			}

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write(body)
			}))
			defer srv.Close()

			cfg := newTestConfig(srv.URL)
			c := client.New(cfg)

			handler := handleListUsers(c)
			_, output, err := handler(context.Background(), &mcp.CallToolRequest{}, ListUsersInput{})

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if output.TotalCount != tt.wantCount {
				t.Errorf("TotalCount = %d, want %d", output.TotalCount, tt.wantCount)
			}
			if len(output.Users) != tt.wantCount {
				t.Errorf("len(Users) = %d, want %d", len(output.Users), tt.wantCount)
			}
		})
	}
}

func TestHandleGetUser(t *testing.T) {
	tests := []struct {
		name       string
		userID     string
		statusCode int
		body       string
		wantName   string
		wantErr    bool
	}{
		{
			name:       "success returns user",
			userID:     "user1",
			statusCode: 200,
			body:       `{"id":"user1","displayName":"Albert Einstein","mail":"einstein@example.org"}`,
			wantName:   "Albert Einstein",
		},
		{
			name:       "404 returns error",
			userID:     "nonexistent",
			statusCode: 404,
			body:       `{}`,
			wantErr:    true,
		},
		{
			name:    "empty user_id returns validation error",
			userID:  "",
			wantErr: true,
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

			cfg := newTestConfig(srv.URL)
			c := client.New(cfg)

			handler := handleGetUser(c)
			_, output, err := handler(context.Background(), &mcp.CallToolRequest{}, GetUserInput{UserID: tt.userID})

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
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

func TestHandleGetMe(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantName   string
		wantErr    bool
	}{
		{
			name:       "success returns current user",
			statusCode: 200,
			body:       `{"id":"me1","displayName":"Current User","mail":"me@example.org"}`,
			wantName:   "Current User",
		},
		{
			name:       "401 returns error",
			statusCode: 401,
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

			cfg := newTestConfig(srv.URL)
			c := client.New(cfg)

			handler := handleGetMe(c)
			_, output, err := handler(context.Background(), &mcp.CallToolRequest{}, GetMeInput{})

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
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

func TestHandleDeleteUser(t *testing.T) {
	tests := []struct {
		name       string
		userID     string
		confirm    bool
		statusCode int
		wantErr    bool
	}{
		{
			name:       "success with confirm",
			userID:     "user1",
			confirm:    true,
			statusCode: 204,
		},
		{
			name:    "fails without confirm",
			userID:  "user1",
			confirm: false,
			wantErr: true,
		},
		{
			name:    "fails with empty user_id",
			userID:  "",
			confirm: true,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))
			defer srv.Close()

			cfg := newTestConfig(srv.URL)
			c := client.New(cfg)

			handler := handleDeleteUser(c)
			_, output, err := handler(context.Background(), &mcp.CallToolRequest{}, DeleteUserInput{
				UserID:  tt.userID,
				Confirm: tt.confirm,
			})

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !output.Success {
				t.Error("expected Success = true")
			}
		})
	}
}
