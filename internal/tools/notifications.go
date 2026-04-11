package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/owncloud/ocis-mcp-server/internal/client"
)

// Notification represents an oCIS notification from the OCS API.
type Notification struct {
	ID         int    `json:"notification_id"`
	DateTime   string `json:"datetime"`
	Subject    string `json:"subject"`
	Message    string `json:"message,omitempty"`
	ObjectType string `json:"object_type,omitempty"`
	ObjectID   string `json:"object_id,omitempty"`
}

// ocsNotificationsResponse wraps the OCS envelope for notifications.
type ocsNotificationsResponse struct {
	OCS struct {
		Data []Notification `json:"data"`
	} `json:"ocs"`
}

// --- Input/Output types ---

type ListNotificationsInput struct{}

type ListNotificationsOutput struct {
	Notifications []Notification `json:"notifications"`
	TotalCount    int            `json:"total_count"`
}

type DeleteNotificationInput struct {
	NotificationID string `json:"notification_id" jsonschema:"Notification ID to delete,required"`
	Confirm        bool   `json:"confirm" jsonschema:"Must be true to confirm deletion"`
}

// --- Registration ---

func registerNotifications(s *mcp.Server, c *client.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_list_notifications",
		Description: "List all notifications for the current user. Returns notification subjects, messages, and related object references.",
		Annotations: readOnlyAnnotations(),
	}, handleListNotifications(c))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ocis_delete_notification",
		Description: "Delete a specific notification by ID. Requires confirm=true.",
		Annotations: destructiveAnnotations(),
	}, handleDeleteNotification(c))
}

// --- Handlers ---

func handleListNotifications(c *client.Client) mcp.ToolHandlerFor[ListNotificationsInput, ListNotificationsOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, _ ListNotificationsInput) (*mcp.CallToolResult, ListNotificationsOutput, error) {
		raw, status, err := client.GetRaw(ctx, c, "/ocs/v2.php/apps/notifications/api/v1/notifications")
		if err != nil {
			return nil, ListNotificationsOutput{}, err
		}
		if status >= 400 {
			return nil, ListNotificationsOutput{}, fmt.Errorf("notifications API returned status %d", status)
		}

		var resp ocsNotificationsResponse
		if err := json.Unmarshal(raw, &resp); err != nil {
			return nil, ListNotificationsOutput{}, fmt.Errorf("decoding notifications response: %w", err)
		}

		notifications := resp.OCS.Data
		if notifications == nil {
			notifications = []Notification{}
		}
		return nil, ListNotificationsOutput{
			Notifications: notifications,
			TotalCount:    len(notifications),
		}, nil
	}
}

func handleDeleteNotification(c *client.Client) mcp.ToolHandlerFor[DeleteNotificationInput, DeleteOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input DeleteNotificationInput) (*mcp.CallToolResult, DeleteOutput, error) {
		if !input.Confirm {
			return nil, DeleteOutput{}, fmt.Errorf("this is a destructive operation. Set confirm=true to proceed")
		}
		if err := client.ValidateID("notification_id", input.NotificationID); err != nil {
			return nil, DeleteOutput{}, err
		}
		path := fmt.Sprintf("/ocs/v2.php/apps/notifications/api/v1/notifications/%s", url.PathEscape(input.NotificationID))
		if err := client.Delete(ctx, c, path); err != nil {
			return nil, DeleteOutput{}, err
		}
		return nil, DeleteOutput{Success: true, Message: "notification deleted"}, nil
	}
}
