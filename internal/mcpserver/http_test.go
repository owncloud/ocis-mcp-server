package mcpserver

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	v2026 = "2026-07-28"
	v2025 = "2025-11-25"
)

// meta2026 is the _meta block every 2026-07-28 request carries.
func meta2026(version string) string {
	return `"_meta":{"io.modelcontextprotocol/protocolVersion":"` + version +
		`","io.modelcontextprotocol/clientCapabilities":{}}`
}

func headers2026(method, name string) map[string]string {
	h := map[string]string{"MCP-Protocol-Version": v2026, "Mcp-Method": method}
	if name != "" {
		h["Mcp-Name"] = name
	}
	return h
}

func rpcResult(t *testing.T, rpc map[string]any) map[string]any {
	t.Helper()
	res, ok := rpc["result"].(map[string]any)
	if !ok {
		t.Fatalf("no result in %v", rpc)
	}
	return res
}

func rpcErrorCode(rpc map[string]any) float64 {
	e, _ := rpc["error"].(map[string]any)
	c, _ := e["code"].(float64)
	return c
}

func TestHTTP2026Request(t *testing.T) {
	url, _ := startHTTP(t, "")
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{` + meta2026(v2026) + `}}`
	status, _, rpc := rawPost(t, url, headers2026("tools/list", ""), body)
	if status != http.StatusOK {
		t.Fatalf("status = %d, rpc = %v", status, rpc)
	}
	res := rpcResult(t, rpc)
	if res["resultType"] != "complete" {
		t.Errorf("resultType = %v, want complete", res["resultType"])
	}
	if tools, _ := res["tools"].([]any); len(tools) == 0 {
		t.Error("tools is empty")
	}
}

func TestCacheHints(t *testing.T) {
	tests := []struct {
		method    string
		name      string
		params    string
		wantTTL   float64
		wantScope string
	}{
		{"tools/list", "", "", 300000, "public"},
		{"prompts/list", "", "", 300000, "public"},
		{"resources/list", "", "", 300000, "public"},
		{"resources/templates/list", "", "", 300000, "public"},
		{"server/discover", "", "", 300000, "public"},
		{"resources/read", "ocis://capabilities", `"uri":"ocis://capabilities",`, 30000, "private"},
	}
	url, _ := startHTTP(t, "")
	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			body := `{"jsonrpc":"2.0","id":1,"method":"` + tt.method + `","params":{` + tt.params + meta2026(v2026) + `}}`
			status, _, rpc := rawPost(t, url, headers2026(tt.method, tt.name), body)
			if status != http.StatusOK {
				t.Fatalf("status = %d, rpc = %v", status, rpc)
			}
			res := rpcResult(t, rpc)
			if got, _ := res["ttlMs"].(float64); got != tt.wantTTL {
				t.Errorf("ttlMs = %v, want %v", res["ttlMs"], tt.wantTTL)
			}
			if got := res["cacheScope"]; got != tt.wantScope {
				t.Errorf("cacheScope = %v, want %s", got, tt.wantScope)
			}
		})
	}
}

func TestHTTPServerDiscover(t *testing.T) {
	url, _ := startHTTP(t, "")
	body := `{"jsonrpc":"2.0","id":1,"method":"server/discover","params":{` + meta2026(v2026) + `}}`
	status, _, rpc := rawPost(t, url, headers2026("server/discover", ""), body)
	if status != http.StatusOK {
		t.Fatalf("status = %d, rpc = %v", status, rpc)
	}
	versions, _ := rpcResult(t, rpc)["supportedVersions"].([]any)
	for _, want := range []string{v2026, v2025} {
		found := false
		for _, v := range versions {
			if v == want {
				found = true
			}
		}
		if !found {
			t.Errorf("supportedVersions %v missing %s", versions, want)
		}
	}
}

func TestHTTPSDKClients(t *testing.T) {
	tests := []struct {
		name    string
		version string
	}{
		{"legacy 2025-11-25", v2025},
		{"latest default", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, _ := startHTTP(t, "")
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			c := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0"}, nil)
			cs, err := c.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: url, DisableStandaloneSSE: true},
				&mcp.ClientSessionOptions{ProtocolVersion: tt.version})
			if err != nil {
				t.Fatalf("connect: %v", err)
			}
			defer cs.Close()
			tools, err := cs.ListTools(ctx, nil)
			if err != nil {
				t.Fatalf("ListTools: %v", err)
			}
			if len(tools.Tools) == 0 {
				t.Error("no tools listed")
			}
			res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "ocis_get_version"})
			if err != nil {
				t.Fatalf("CallTool: %v", err)
			}
			if res.IsError {
				t.Errorf("tool returned error: %+v", res.Content)
			}
		})
	}
}

func TestHTTPUnsupportedMethods(t *testing.T) {
	url, _ := startHTTP(t, "")
	for _, method := range []string{http.MethodGet, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			req, err := http.NewRequest(method, url, nil)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Accept", "application/json, text/event-stream")
			resp, err := httpClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusMethodNotAllowed {
				t.Errorf("status = %d, want 405", resp.StatusCode)
			}
			if got := resp.Header.Get("Allow"); got != "POST" {
				t.Errorf("Allow = %q, want POST", got)
			}
		})
	}
}

func TestHTTPStaleSessionID(t *testing.T) {
	url, _ := startHTTP(t, "")
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`
	status, _, rpc := rawPost(t, url, map[string]string{
		"Mcp-Session-Id":       "stale-session-id",
		"MCP-Protocol-Version": v2025,
	}, body)
	if status >= 500 {
		t.Errorf("status = %d, rpc = %v, want non-5xx", status, rpc)
	}
}

func TestHTTPHeaderValidation(t *testing.T) {
	callBody := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"ocis_get_version","arguments":{},` + meta2026(v2026) + `}}`
	tests := []struct {
		name     string
		headers  map[string]string
		body     string
		wantCode float64 // error code expected when the request is rejected
		wantOK   bool    // expect HTTP 200 instead of a 400 rejection
		wantHits int     // requests the fake oCIS must have seen
	}{
		{
			name:     "Mcp-Method mismatches body",
			headers:  headers2026("tools/call", "ocis_get_version"),
			body:     `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{` + meta2026(v2026) + `}}`,
			wantCode: -32020,
		},
		{
			name: "protocol header disagrees with _meta",
			headers: map[string]string{
				"MCP-Protocol-Version": v2025,
				"Mcp-Method":           "tools/list",
			},
			body:     `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{` + meta2026(v2026) + `}}`,
			wantCode: -32020,
		},
		{
			name:     "Mcp-Method mismatches tools/call body",
			headers:  headers2026("tools/list", ""),
			body:     callBody,
			wantCode: -32020,
		},
		{
			name:     "Mcp-Name mismatches params.name",
			headers:  headers2026("tools/call", "ocis_get_other"),
			body:     callBody,
			wantCode: -32020,
		},
		{
			name:     "control: matching headers reach the tool",
			headers:  headers2026("tools/call", "ocis_get_version"),
			body:     callBody,
			wantOK:   true,
			wantHits: 1,
		},
		{
			name: "unsupported version",
			headers: map[string]string{
				"MCP-Protocol-Version": "2099-01-01",
				"Mcp-Method":           "tools/list",
			},
			body:     `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{` + meta2026("2099-01-01") + `}}`,
			wantCode: -32022,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, fake := startHTTP(t, "")
			status, _, rpc := rawPost(t, url, tt.headers, tt.body)
			wantStatus := http.StatusBadRequest
			if tt.wantOK {
				wantStatus = http.StatusOK
			}
			if status != wantStatus {
				t.Errorf("status = %d, want %d (rpc %v)", status, wantStatus, rpc)
			}
			if got := rpcErrorCode(rpc); got != tt.wantCode {
				t.Errorf("error code = %v, want %v (rpc %v)", got, tt.wantCode, rpc)
			}
			if n := fake.Hits(); n != tt.wantHits {
				t.Errorf("fake oCIS saw %d requests, want %d", n, tt.wantHits)
			}
		})
	}
}

func TestHTTPClientDisconnectCancels(t *testing.T) {
	url, fake := startHTTP(t, "")
	received := make(chan struct{})
	cancelled := make(chan struct{})
	fake.SetHandler(func(_ http.ResponseWriter, r *http.Request) {
		close(received)
		<-r.Context().Done()
		close(cancelled)
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"ocis_get_version","arguments":{},` + meta2026(v2026) + `}}`
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _, _, _ = doRawPost(ctx, url, headers2026("tools/call", "ocis_get_version"), body)
	}()

	select {
	case <-received:
	case <-time.After(5 * time.Second):
		t.Fatal("fake oCIS never received the request")
	}
	cancel()
	select {
	case <-cancelled:
	case <-time.After(2 * time.Second):
		t.Fatal("handler was not cancelled within 2s of client disconnect")
	}
	<-done
}

func TestHTTPBearerSecret(t *testing.T) {
	const secret = "dummy-secret"
	listBody := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`
	tests := []struct {
		name    string
		secret  string
		auth    string
		want401 bool
	}{
		{"no header", secret, "", true},
		{"wrong token", secret, "Bearer wrong-token", true},
		{"right token", secret, "Bearer " + secret, false},
		{"empty secret needs no header", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, _ := startHTTP(t, tt.secret)
			h := map[string]string{}
			if tt.auth != "" {
				h["Authorization"] = tt.auth
			}
			status, hdr, _ := rawPost(t, url, h, listBody)
			if (status == http.StatusUnauthorized) != tt.want401 {
				t.Fatalf("status = %d, want401 = %v", status, tt.want401)
			}
			if tt.want401 && !strings.HasPrefix(hdr.Get("WWW-Authenticate"), "Bearer") {
				t.Errorf("WWW-Authenticate = %q, want Bearer prefix", hdr.Get("WWW-Authenticate"))
			}
		})
	}
}
