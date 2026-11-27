package mcpserver

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestRegistrationCounts(t *testing.T) {
	tests := []struct {
		name           string
		educationToken string
		wantTools      int
	}{
		{"education token empty", "", 75},
		{"education token set", "dummy-education-token", 80},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, _ := newTestServer(t, tt.educationToken)
			cs := connectInMemory(t, s, "")
			ctx := context.Background()

			tools, err := cs.ListTools(ctx, nil)
			if err != nil {
				t.Fatalf("ListTools: %v", err)
			}
			if got := len(tools.Tools); got != tt.wantTools {
				t.Errorf("tools = %d, want %d", got, tt.wantTools)
			}
			res, err := cs.ListResources(ctx, nil)
			if err != nil {
				t.Fatalf("ListResources: %v", err)
			}
			if got := len(res.Resources); got != 5 {
				t.Errorf("resources = %d, want 5", got)
			}
			prompts, err := cs.ListPrompts(ctx, nil)
			if err != nil {
				t.Fatalf("ListPrompts: %v", err)
			}
			if got := len(prompts.Prompts); got != 4 {
				t.Errorf("prompts = %d, want 4", got)
			}
		})
	}
}

func TestLegacyClientOverMemory(t *testing.T) {
	tests := []struct {
		name    string
		version string
	}{
		{"client pinned to 2025-11-25", "2025-11-25"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, fake := newTestServer(t, "")
			cs := connectInMemory(t, s, tt.version)
			ctx := context.Background()

			tools, err := cs.ListTools(ctx, nil)
			if err != nil {
				t.Fatalf("ListTools: %v", err)
			}
			if len(tools.Tools) == 0 {
				t.Fatal("no tools listed")
			}

			res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "ocis_get_version"})
			if err != nil {
				t.Fatalf("CallTool: %v", err)
			}
			if res.IsError {
				t.Fatalf("tool returned error: %+v", res.Content)
			}
			raw, err := json.Marshal(res.StructuredContent)
			if err != nil {
				t.Fatalf("marshal structured content: %v", err)
			}
			var out struct {
				Version struct {
					String string `json:"string"`
				} `json:"version"`
			}
			if err := json.Unmarshal(raw, &out); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if out.Version.String != "7.1.3" {
				t.Errorf("version = %q, want 7.1.3", out.Version.String)
			}
			if fake.Hits() != 1 {
				t.Errorf("fake oCIS hits = %d, want 1", fake.Hits())
			}
		})
	}
}
