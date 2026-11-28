package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestReadMissingResource(t *testing.T) {
	tests := []struct {
		name string
		uri  string
	}{
		{"unregistered ocis uri", "ocis://does-not-exist"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, _ := newTestServer(t, "")
			cs := connectInMemory(t, s, "")
			_, err := cs.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: tt.uri})
			if err == nil {
				t.Fatal("ReadResource returned no error")
			}
			var wire *jsonrpc.Error
			if !errors.As(err, &wire) {
				t.Fatalf("error %T (%v) is not a *jsonrpc.Error", err, err)
			}
			if wire.Code != jsonrpc.CodeInvalidParams {
				t.Errorf("code = %d, want %d", wire.Code, jsonrpc.CodeInvalidParams)
			}
		})
	}
}

func TestCallUnknownTool(t *testing.T) {
	tests := []struct {
		name string
		tool string
	}{
		{"unregistered tool", "no_such_tool"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, fake := newTestServer(t, "")
			cs := connectInMemory(t, s, "")
			res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: tt.tool})
			if err == nil {
				t.Fatalf("unknown tool returned no error, result: %+v", res)
			}
			var wire *jsonrpc.Error
			if !errors.As(err, &wire) {
				t.Fatalf("error %T (%v) is not a *jsonrpc.Error", err, err)
			}
			if wire.Code != jsonrpc.CodeInvalidParams {
				t.Errorf("code = %d, want %d (message %q)", wire.Code, jsonrpc.CodeInvalidParams, wire.Message)
			}
			if fake.Hits() != 0 {
				t.Errorf("fake oCIS hits = %d, want 0", fake.Hits())
			}
		})
	}
}

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
