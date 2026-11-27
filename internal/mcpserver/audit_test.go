package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	sdkauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/owncloud/ocis-mcp-server/internal/auth"
)

const auditTestToken = "secret-token-xyz"

type bearerRoundTripper struct{ token string }

func (b bearerRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	if b.token != "" {
		r.Header.Set("Authorization", "Bearer "+b.token)
	}
	return http.DefaultTransport.RoundTrip(r)
}

type seenIdentity struct {
	mu          sync.Mutex
	id          auth.Identity
	ok          bool
	extraHasTok bool
	ctxHasTok   bool
}

// newStatelessAuthServer builds its own stateless handler (not NewHTTPHandler, whose signature changes later).
func newStatelessAuthServer(t *testing.T, withBearer bool, logger *slog.Logger, seen *seenIdentity) string {
	t.Helper()
	srv := mcp.NewServer(&mcp.Implementation{Name: "t", Version: "0"}, nil)
	if logger != nil {
		srv.AddReceivingMiddleware(AuditMiddleware(logger))
	}
	mcp.AddTool(srv, &mcp.Tool{Name: "whoami"}, func(ctx context.Context, req *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		id, ok := auth.IdentityFrom(ctx, req.Extra)
		if seen != nil {
			seen.mu.Lock()
			seen.id, seen.ok = id, ok
			seen.extraHasTok = req.Extra != nil && req.Extra.TokenInfo != nil
			seen.ctxHasTok = sdkauth.TokenInfoFromContext(ctx) != nil
			seen.mu.Unlock()
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "ok"}}}, nil, nil
	})
	mcp.AddTool(srv, &mcp.Tool{Name: "boom"}, func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, any, error) {
		return nil, nil, errors.New("boom")
	})
	var h http.Handler = mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return srv },
		&mcp.StreamableHTTPOptions{Stateless: true, PropagateRequestCancellation: true},
	)
	if withBearer {
		// The stub puts the presented token into every TokenInfo field an audit
		// record could be tempted to copy, so the never-logs-token check is
		// meaningful: a record built from Extra or Scopes would leak it.
		stub := func(_ context.Context, token string, _ *http.Request) (*sdkauth.TokenInfo, error) {
			return &sdkauth.TokenInfo{
				UserID:     "u1",
				Scopes:     []string{token},
				Expiration: time.Now().Add(time.Hour),
				Extra:      map[string]any{"iss": "https://idp.test", "raw": token, "jti": token},
			}, nil
		}
		h = sdkauth.RequireBearerToken(stub, nil)(h)
	}
	mux := http.NewServeMux()
	mux.Handle("/mcp", h)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts.URL + "/mcp"
}

func callTool(t *testing.T, url, token, tool string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c := mcp.NewClient(&mcp.Implementation{Name: "c", Version: "0"}, nil)
	cs, err := c.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint:             url,
		DisableStandaloneSSE: true,
		HTTPClient:           &http.Client{Transport: bearerRoundTripper{token: token}},
	}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = cs.Close() }()
	if _, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: tool}); err != nil {
		t.Logf("CallTool(%s) protocol error: %v", tool, err)
	}
}

func TestIdentityReachesHandlerStateless(t *testing.T) {
	seen := &seenIdentity{}
	url := newStatelessAuthServer(t, true, nil, seen)
	callTool(t, url, auditTestToken, "whoami")

	seen.mu.Lock()
	defer seen.mu.Unlock()
	t.Logf("sources: extra.TokenInfo=%v ctx TokenInfo=%v", seen.extraHasTok, seen.ctxHasTok)
	if !seen.ok {
		t.Fatal("IdentityFrom found no identity in stateless mode")
	}
	if seen.id.Subject != "u1" || seen.id.Issuer != "https://idp.test" {
		t.Errorf("identity = %+v, want sub u1 iss https://idp.test", seen.id)
	}
}

func TestAuditLogFields(t *testing.T) {
	tests := []struct {
		name       string
		withBearer bool
		token      string
		tool       string
		wantIss    string
		wantSub    string
		wantOut    string
	}{
		{"success", true, auditTestToken, "whoami", "https://idp.test", "u1", "ok"},
		{"tool error", true, auditTestToken, "boom", "https://idp.test", "u1", "error"},
		{"no identity", false, "", "whoami", "", "", "ok"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
			url := newStatelessAuthServer(t, tt.withBearer, logger, nil)
			callTool(t, url, tt.token, tt.tool)

			if strings.Contains(buf.String(), auditTestToken) {
				// Do not print the buffer: it holds the leaked token.
				t.Fatal("audit log output contains the bearer token")
			}
			var rec map[string]any
			n := 0
			for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
				if line == "" {
					continue
				}
				var r map[string]any
				if err := json.Unmarshal([]byte(line), &r); err != nil {
					t.Fatalf("bad log line %q: %v", line, err)
				}
				if r["method"] == "tools/call" {
					rec = r
					n++
				}
			}
			if n != 1 {
				t.Fatalf("got %d tools/call records, want 1 (%d bytes of log)", n, buf.Len())
			}
			if rec["level"] != "INFO" {
				t.Errorf("level = %v, want INFO", rec["level"])
			}
			for k, want := range map[string]string{"iss": tt.wantIss, "sub": tt.wantSub, "tool": tt.tool, "outcome": tt.wantOut} {
				if got, ok := rec[k]; !ok || got != want {
					t.Errorf("%s = %v (present=%v), want %q", k, got, ok, want)
				}
			}
			for k := range rec {
				switch k {
				case "time", "level", "msg", "iss", "sub", "method", "tool", "outcome":
				default:
					t.Errorf("unexpected log field %q", k)
				}
			}
		})
	}
}

func TestAuditLogSkipsOtherMethods(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	url := newStatelessAuthServer(t, true, logger, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c := mcp.NewClient(&mcp.Implementation{Name: "c", Version: "0"}, nil)
	cs, err := c.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint: url, DisableStandaloneSSE: true,
		HTTPClient: &http.Client{Transport: bearerRoundTripper{token: auditTestToken}},
	}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = cs.Close() }()
	if _, err := cs.ListTools(ctx, nil); err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no audit output for tools/list, got %d bytes", buf.Len())
	}
}
