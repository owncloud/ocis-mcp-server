package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/owncloud/ocis-mcp-server/internal/client"
)

// Issue #14: the LibreGraph /invite endpoint requires each recipient to be an
// object with both objectId and @libre.graph.recipient.type; sending a bare
// objectId returns HTTP 400 (validation on the LibreGraphRecipientType field).
func TestHandleCreateShareSetsRecipientType(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"permissions":[{"id":"p1"}]}`))
	}))
	defer srv.Close()

	c := client.New(newTestConfig(srv.URL))
	handler := handleCreateShare(c)
	if _, _, err := handler(context.Background(), &mcp.CallToolRequest{}, CreateShareInput{
		SpaceID: "s1", ItemID: "item1",
		Recipients: []string{"u1", "u2"}, Roles: []string{"viewer"},
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	recips, ok := gotBody["recipients"].([]any)
	if !ok || len(recips) != 2 {
		t.Fatalf("recipients = %v, want 2 entries", gotBody["recipients"])
	}
	for i, raw := range recips {
		r, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("recipient[%d] is not an object: %v", i, raw)
		}
		if id, _ := r["objectId"].(string); id == "" {
			t.Errorf("recipient[%d] missing objectId: %v", i, r)
		}
		if got := r["@libre.graph.recipient.type"]; got != "user" {
			t.Errorf("recipient[%d] @libre.graph.recipient.type = %v, want \"user\"", i, got)
		}
	}
}
