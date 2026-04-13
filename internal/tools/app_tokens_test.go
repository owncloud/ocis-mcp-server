package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/owncloud/ocis-mcp-server/internal/client"
)

func TestHandleListAppTokens(t *testing.T) {
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
			body:       `[{"label":"MCP","expiry":"2025-12-31"},{"label":"CI","expiry":"2026-01-15"}]`,
			wantCount:  2,
		},
		{
			name:       "empty",
			statusCode: 200,
			body:       `null`,
			wantCount:  0,
		},
		{
			name:       "error",
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

			c := client.New(newTestConfig(srv.URL))
			handler := handleListAppTokens(c)
			_, output, err := handler(context.Background(), &mcp.CallToolRequest{}, ListAppTokensInput{})

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(output.Tokens) != tt.wantCount {
				t.Errorf("len(Tokens) = %d, want %d", len(output.Tokens), tt.wantCount)
			}
		})
	}
}

func TestHandleCreateAppToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"token":"new-token-value","label":"Test","expiry":"2025-12-31"}`))
	}))
	defer srv.Close()

	c := client.New(newTestConfig(srv.URL))
	handler := handleCreateAppToken(c)
	_, output, err := handler(context.Background(), &mcp.CallToolRequest{}, CreateAppTokenInput{
		Label: "Test", Expiry: "72h",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.Token != "new-token-value" {
		t.Errorf("Token = %q, want new-token-value", output.Token)
	}
}

func TestHandleDeleteAppToken(t *testing.T) {
	tests := []struct {
		name    string
		token   string
		confirm bool
		wantErr bool
	}{
		{name: "success", token: "tok123", confirm: true},
		{name: "no confirm", token: "tok123", confirm: false, wantErr: true},
		{name: "empty token", token: "", confirm: true, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(204)
			}))
			defer srv.Close()

			c := client.New(newTestConfig(srv.URL))
			handler := handleDeleteAppToken(c)
			_, _, err := handler(context.Background(), &mcp.CallToolRequest{}, DeleteAppTokenInput{
				Token: tt.token, Confirm: tt.confirm,
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
