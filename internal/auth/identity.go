// Package auth carries the verified caller identity from the HTTP layer to MCP handlers.
package auth

import (
	"context"

	sdkauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Identity is the verified caller of a request.
type Identity struct {
	Issuer  string
	Subject string
}

// IdentityFrom returns the verified caller, or false when the request is unauthenticated.
//
// The SDK populates RequestExtra.TokenInfo in stateless mode, which is preferred;
// the request context (set by sdkauth.RequireBearerToken) is the fallback.
func IdentityFrom(ctx context.Context, extra *mcp.RequestExtra) (Identity, bool) {
	var info *sdkauth.TokenInfo
	if extra != nil {
		info = extra.TokenInfo
	}
	if info == nil {
		info = sdkauth.TokenInfoFromContext(ctx)
	}
	if info == nil || info.UserID == "" {
		return Identity{}, false
	}
	iss, _ := info.Extra["iss"].(string)
	return Identity{Issuer: iss, Subject: info.UserID}, true
}
