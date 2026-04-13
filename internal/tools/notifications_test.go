package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/owncloud/ocis-mcp-server/internal/client"
)

func TestHandleListNotifications(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantCount  int
		wantErr    bool
	}{
		{
			name:       "success with notifications",
			statusCode: 200,
			body:       `{"ocs":{"data":[{"notification_id":1,"subject":"File shared"},{"notification_id":2,"subject":"Welcome"}]}}`,
			wantCount:  2,
		},
		{
			name:       "empty notifications",
			statusCode: 200,
			body:       `{"ocs":{"data":null}}`,
			wantCount:  0,
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
			handler := handleListNotifications(c)
			_, output, err := handler(context.Background(), &mcp.CallToolRequest{}, ListNotificationsInput{})

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

func TestHandleDeleteNotification(t *testing.T) {
	tests := []struct {
		name           string
		notificationID string
		confirm        bool
		wantErr        bool
	}{
		{name: "success", notificationID: "123", confirm: true},
		{name: "no confirm", notificationID: "123", confirm: false, wantErr: true},
		{name: "empty id", notificationID: "", confirm: true, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(204)
			}))
			defer srv.Close()

			c := client.New(newTestConfig(srv.URL))
			handler := handleDeleteNotification(c)
			_, _, err := handler(context.Background(), &mcp.CallToolRequest{}, DeleteNotificationInput{
				NotificationID: tt.notificationID, Confirm: tt.confirm,
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
