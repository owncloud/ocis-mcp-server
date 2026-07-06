package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/owncloud/ocis-mcp-server/internal/client"
)

// Regression tests for tool-request-shape bugs surfaced by full integration
// testing against oCIS 8.1.0-rc.1.

// invite_to_space must send @libre.graph.recipient.type on each recipient, or
// oCIS rejects the DriveItemInvite with HTTP 400 (LibreGraphRecipientType
// validation) — the same requirement create_share already satisfies (#14/#22).
func TestHandleInviteToSpaceSetsRecipientType(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"value":[{"id":"p1"}]}`))
	}))
	defer srv.Close()

	c := client.New(newTestConfig(srv.URL))
	handler := handleInviteToSpace(c)
	if _, _, err := handler(context.Background(), &mcp.CallToolRequest{}, InviteToSpaceInput{
		SpaceID: "s1", Recipients: []string{"u1", "u2"}, Roles: []string{"viewer"},
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertRecipientsTyped(t, gotBody)
}

// upload_and_share shares the file via the same /invite endpoint and must also
// set the recipient type on the share step.
func TestHandleUploadAndShareSetsRecipientType(t *testing.T) {
	var inviteBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "PUT":
			w.WriteHeader(201)
		case "PROPFIND":
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(207)
			_, _ = w.Write([]byte(propfindResponse))
		default: // POST invite
			_ = json.NewDecoder(r.Body).Decode(&inviteBody)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"value":[{"id":"p1"}]}`))
		}
	}))
	defer srv.Close()

	c := client.New(newTestConfig(srv.URL))
	handler := handleUploadAndShare(c)
	if _, _, err := handler(context.Background(), &mcp.CallToolRequest{}, UploadAndShareInput{
		SpaceID: "s1", Path: "/test.txt", Content: "hi",
		Recipients: []string{"u1"}, Roles: []string{"viewer"},
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertRecipientsTyped(t, inviteBody)
}

func assertRecipientsTyped(t *testing.T, body map[string]any) {
	t.Helper()
	recips, ok := body["recipients"].([]any)
	if !ok || len(recips) == 0 {
		t.Fatalf("recipients = %v, want non-empty", body["recipients"])
	}
	for i, raw := range recips {
		r, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("recipient[%d] not an object: %v", i, raw)
		}
		if id, _ := r["objectId"].(string); id == "" {
			t.Errorf("recipient[%d] missing objectId: %v", i, r)
		}
		if got := r["@libre.graph.recipient.type"]; got != "user" {
			t.Errorf("recipient[%d] @libre.graph.recipient.type = %v, want \"user\"", i, got)
		}
	}
}

// add_group_member must send @odata.id as a full URL pointing at the user
// resource; a bare ID makes oCIS return HTTP 400 "Error parsing @odata.id url".
func TestHandleAddGroupMemberUsesFullODataURL(t *testing.T) {
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(204)
	}))
	defer srv.Close()

	c := client.New(newTestConfig(srv.URL))
	handler := handleAddGroupMember(c)
	if _, _, err := handler(context.Background(), &mcp.CallToolRequest{}, AddGroupMemberInput{
		GroupID: "g1", UserID: "u1",
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ref := gotBody["@odata.id"]
	if !strings.Contains(ref, "://") {
		t.Errorf("@odata.id = %q, want an absolute URL (with scheme)", ref)
	}
	if !strings.HasSuffix(ref, "/graph/v1.0/users/u1") {
		t.Errorf("@odata.id = %q, want it to reference /graph/v1.0/users/u1", ref)
	}
}

// get_resource_by_id must address the dav-by-id endpoint as
// /dav/spaces/<full-resource-id> when given a full ID (storageid$spaceid!item,
// which is what get_file_info returns), not /dav/spaces/<space>/<id>.
func TestHandleGetResourceByIDPath(t *testing.T) {
	tests := []struct {
		name       string
		resourceID string
		wantPath   string
	}{
		{"full resource id", "st$sp!item", "/dav/spaces/st$sp!item"},
		{"bare item id", "item1", "/dav/spaces/s1!item1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotPath string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				w.Header().Set("Content-Type", "application/xml")
				w.WriteHeader(207)
				_, _ = w.Write([]byte(propfindResponse))
			}))
			defer srv.Close()

			c := client.New(newTestConfig(srv.URL))
			handler := handleGetResourceByID(c)
			if _, _, err := handler(context.Background(), &mcp.CallToolRequest{}, GetResourceByIDInput{
				SpaceID: "s1", ResourceID: tt.resourceID,
			}); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotPath != tt.wantPath {
				t.Errorf("PROPFIND path = %q, want %q", gotPath, tt.wantPath)
			}
		})
	}
}

// list_assignments must resolve the current user and pass account_uuid; the
// settings assignments-list endpoint returns HTTP 400 without it.
func TestHandleListAssignmentsSendsAccountUUID(t *testing.T) {
	var assignmentsBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/graph/v1.0/me") {
			_, _ = w.Write([]byte(`{"id":"me-123","displayName":"Admin"}`))
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&assignmentsBody)
		_, _ = w.Write([]byte(`{"assignments":[]}`))
	}))
	defer srv.Close()

	c := client.New(newTestConfig(srv.URL))
	handler := handleListAssignments(c)
	if _, _, err := handler(context.Background(), &mcp.CallToolRequest{}, ListAssignmentsInput{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := assignmentsBody["account_uuid"]; got != "me-123" {
		t.Errorf("account_uuid = %v, want \"me-123\"", got)
	}
}
