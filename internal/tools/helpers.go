package tools

import "github.com/modelcontextprotocol/go-sdk/mcp"

// Common annotation helpers.
func boolPtr(b bool) *bool { return &b }

func readOnlyAnnotations() *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{
		ReadOnlyHint:    true,
		DestructiveHint: boolPtr(false),
		OpenWorldHint:   boolPtr(true),
	}
}

func mutatingAnnotations() *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{
		DestructiveHint: boolPtr(false),
		OpenWorldHint:   boolPtr(true),
	}
}

func idempotentAnnotations() *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{
		DestructiveHint: boolPtr(false),
		IdempotentHint:  true,
		OpenWorldHint:   boolPtr(true),
	}
}

func destructiveAnnotations() *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{
		DestructiveHint: boolPtr(true),
		OpenWorldHint:   boolPtr(true),
	}
}
