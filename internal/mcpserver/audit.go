package mcpserver

import (
	"context"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/owncloud/ocis-mcp-server/internal/auth"
)

const methodToolsCall = "tools/call"

// AuditMiddleware logs one info record per tools/call with the verified caller,
// the tool name and the outcome. It never logs tokens, claims or arguments.
func AuditMiddleware(logger *slog.Logger) mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			res, err := next(ctx, method, req)
			if method != methodToolsCall {
				return res, err
			}
			var tool string
			if cr, ok := req.(*mcp.CallToolRequest); ok && cr.Params != nil {
				tool = cr.Params.Name
			}
			id, _ := auth.IdentityFrom(ctx, req.GetExtra())
			outcome := "ok"
			if ctr, ok := res.(*mcp.CallToolResult); err != nil || (ok && ctr != nil && ctr.IsError) {
				outcome = "error"
			}
			logger.LogAttrs(ctx, slog.LevelInfo, "tool call",
				slog.String("iss", id.Issuer),
				slog.String("sub", id.Subject),
				slog.String("method", method),
				slog.String("tool", tool),
				slog.String("outcome", outcome),
			)
			return res, err
		}
	}
}
