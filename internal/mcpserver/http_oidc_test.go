package mcpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	sdkauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/owncloud/ocis-mcp-server/internal/auth"
)

const (
	testIssuer = "https://idp.example.test/realms/ocis"
	testScope  = "ocis:mcp"
	validToken = "valid-token"
	lowScope   = "low-scope-token"
	secretVal  = "dummy-secret"
)

// stubVerifier accepts two fixed tokens and rejects everything else.
func stubVerifier(_ context.Context, token string, _ *http.Request) (*sdkauth.TokenInfo, error) {
	switch token {
	case validToken:
		return &sdkauth.TokenInfo{Scopes: []string{testScope}, Expiration: time.Now().Add(time.Hour)}, nil
	case lowScope:
		return &sdkauth.TokenInfo{Scopes: []string{"other"}, Expiration: time.Now().Add(time.Hour)}, nil
	}
	return nil, sdkauth.ErrInvalidToken
}

func oidcAuth(publicURL string) HTTPAuth {
	return HTTPAuth{
		Mode:        AuthModeOIDC,
		Secret:      secretVal, // must be ignored in oidc mode
		PublicURL:   publicURL,
		Resource:    publicURL + "/mcp",
		Issuer:      testIssuer,
		MetadataURL: publicURL + metadataPath,
		Scopes:      []string{testScope},
		Verifier:    stubVerifier,
	}
}

// startOIDC serves an oidc-mode handler whose public URL is the test server's own origin.
func startOIDC(t *testing.T) (base string, fake *fakeOCIS) {
	t.Helper()
	s, fake := newTestServer(t, "")
	srv := httptest.NewUnstartedServer(nil)
	base = "http://" + srv.Listener.Addr().String()
	h, err := NewHTTPHandler(s, oidcAuth(base))
	if err != nil {
		t.Fatalf("NewHTTPHandler: %v", err)
	}
	srv.Config.Handler = h
	srv.Start()
	t.Cleanup(srv.Close)
	return base, fake
}

const callBody = `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"ocis_get_version","arguments":{},"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{}}}}`

func TestHTTPOIDC(t *testing.T) {
	tests := []struct {
		name     string
		auth     string
		query    string
		want     int
		wantHits int
		wantChal bool
	}{
		{"no header", "", "", http.StatusUnauthorized, 0, true},
		{"valid token", "Bearer " + validToken, "", http.StatusOK, 1, false},
		{"wrong scope", "Bearer " + lowScope, "", http.StatusForbidden, 0, true},
		{"access_token query only", "", "?access_token=" + validToken, http.StatusUnauthorized, 0, true},
		{"lowercase bearer", "bearer " + validToken, "", http.StatusOK, 1, false},
		{"configured secret as bearer", "Bearer " + secretVal, "", http.StatusUnauthorized, 0, true},
		{"unknown token", "Bearer nope", "", http.StatusUnauthorized, 0, true},
		{"basic scheme", "Basic " + validToken, "", http.StatusUnauthorized, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base, fake := startOIDC(t)
			h := headers2026("tools/call", "ocis_get_version")
			if tt.auth != "" {
				h["Authorization"] = tt.auth
			}
			status, hdr, rpc := rawPost(t, base+"/mcp"+tt.query, h, callBody)
			if status != tt.want {
				t.Fatalf("status = %d, want %d", status, tt.want)
			}
			if tt.want == http.StatusOK {
				rpcResult(t, rpc)
			}
			if n := fake.Hits(); n != tt.wantHits {
				t.Errorf("fake oCIS saw %d requests, want %d", n, tt.wantHits)
			}
			chal := hdr.Get("WWW-Authenticate")
			if tt.wantChal {
				for _, part := range []string{`resource_metadata="` + base + metadataPath + `"`, `scope="` + testScope + `"`} {
					if !strings.Contains(chal, part) {
						t.Errorf("WWW-Authenticate = %q, want to contain %q", chal, part)
					}
				}
			} else if chal != "" {
				t.Errorf("unexpected WWW-Authenticate %q", chal)
			}
		})
	}
}

func TestHTTPOIDCNoTokenNeverReachesBackend(t *testing.T) {
	base, fake := startOIDC(t)
	for _, m := range []string{http.MethodPost, http.MethodGet, http.MethodDelete} {
		req, err := http.NewRequestWithContext(context.Background(), m, base+"/mcp", strings.NewReader(callBody))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := httpClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s without token: status = %d, want 401", m, resp.StatusCode)
		}
	}
	if n := fake.Hits(); n != 0 {
		t.Errorf("fake oCIS saw %d requests, want 0", n)
	}
}

func TestProtectedResourceMetadata(t *testing.T) {
	base, _ := startOIDC(t)
	resp, err := httpClient.Get(base + metadataPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q", ct)
	}
	if resp.Header.Get("X-Content-Type-Options") != "nosniff" || resp.Header.Get("X-Frame-Options") != "DENY" {
		t.Errorf("security headers missing: %v", resp.Header)
	}
	var md struct {
		Resource               string   `json:"resource"`
		AuthorizationServers   []string `json:"authorization_servers"`
		ScopesSupported        []string `json:"scopes_supported"`
		BearerMethodsSupported []string `json:"bearer_methods_supported"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&md); err != nil {
		t.Fatal(err)
	}
	if md.Resource != base+"/mcp" {
		t.Errorf("resource = %q, want %q", md.Resource, base+"/mcp")
	}
	if len(md.AuthorizationServers) != 1 || md.AuthorizationServers[0] != testIssuer {
		t.Errorf("authorization_servers = %v", md.AuthorizationServers)
	}
	if len(md.ScopesSupported) != 1 || md.ScopesSupported[0] != testScope {
		t.Errorf("scopes_supported = %v", md.ScopesSupported)
	}
	if len(md.BearerMethodsSupported) != 1 || md.BearerMethodsSupported[0] != "header" {
		t.Errorf("bearer_methods_supported = %v", md.BearerMethodsSupported)
	}
	// The challenge URL is the URL just served.
	_, hdr, _ := rawPost(t, base+"/mcp", nil, callBody)
	if !strings.Contains(hdr.Get("WWW-Authenticate"), `resource_metadata="`+base+metadataPath+`"`) {
		t.Errorf("challenge %q does not point at the served metadata URL", hdr.Get("WWW-Authenticate"))
	}
}

func TestMetadataAbsentOutsideOIDC(t *testing.T) {
	for _, a := range []HTTPAuth{{Mode: AuthModeNone}, {Mode: AuthModeSecret, Secret: secretVal}} {
		s, _ := newTestServer(t, "")
		h, err := NewHTTPHandler(s, a)
		if err != nil {
			t.Fatal(err)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, metadataPath, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("mode %s: status = %d, want 404", a.Mode, rec.Code)
		}
	}
}

// TestHTTPTrustedOrigin sends requests whose Host (loopback, so the SDK accepts it)
// differs from the public URL's host, so that only the explicit trusted-origin entry,
// not same-origin matching, can admit the Origin.
func TestHTTPTrustedOrigin(t *testing.T) {
	const public = "http://localhost:9443"
	tests := []struct {
		name   string
		origin string
		want   int
	}{
		{"public URL origin", public, http.StatusOK},
		{"other origin", "https://evil.example", http.StatusForbidden},
		{"same host other port", "http://localhost:9444", http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, fake := newTestServer(t, "")
			h, err := NewHTTPHandler(s, oidcAuth(public))
			if err != nil {
				t.Fatal(err)
			}
			srv := httptest.NewServer(h)
			t.Cleanup(srv.Close)
			hdr := headers2026("tools/call", "ocis_get_version")
			hdr["Authorization"] = "Bearer " + validToken
			hdr["Origin"] = tt.origin
			status, _, _ := rawPost(t, srv.URL+"/mcp", hdr, callBody)
			if status != tt.want {
				t.Errorf("status = %d, want %d", status, tt.want)
			}
			if tt.want == http.StatusForbidden && fake.Hits() != 0 {
				t.Errorf("fake oCIS saw %d requests, want 0", fake.Hits())
			}
		})
	}
}

func TestNewHTTPHandlerFailsClosed(t *testing.T) {
	good := oidcAuth("https://mcp.example.test")
	mut := func(f func(a *HTTPAuth)) HTTPAuth {
		a := good
		f(&a)
		return a
	}
	tests := []struct {
		name string
		a    HTTPAuth
	}{
		{"empty mode", HTTPAuth{}},
		{"empty mode with secret", HTTPAuth{Secret: secretVal}},
		{"unknown mode", HTTPAuth{Mode: "bogus", Secret: secretVal}},
		{"uppercase mode", HTTPAuth{Mode: "NONE"}},
		{"secret mode empty secret", HTTPAuth{Mode: AuthModeSecret}},
		{"oidc nil verifier", mut(func(a *HTTPAuth) { a.Verifier = nil })},
		{"oidc empty public url", mut(func(a *HTTPAuth) { a.PublicURL = "" })},
		{"oidc empty scopes", mut(func(a *HTTPAuth) { a.Scopes = nil })},
		{"oidc empty resource", mut(func(a *HTTPAuth) { a.Resource = "" })},
		{"oidc empty issuer", mut(func(a *HTTPAuth) { a.Issuer = "" })},
		{"oidc empty metadata url", mut(func(a *HTTPAuth) { a.MetadataURL = "" })},
		{"oidc metadata url wrong path", mut(func(a *HTTPAuth) { a.MetadataURL = "https://mcp.example.test/meta" })},
		{"oidc relative resource", mut(func(a *HTTPAuth) { a.Resource = "/mcp" })},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, _ := newTestServer(t, "")
			h, err := NewHTTPHandler(s, tt.a)
			if err == nil {
				t.Fatal("error = nil, want error")
			}
			if h != nil {
				t.Error("handler is non-nil alongside an error")
			}
		})
	}
	t.Run("valid oidc builds", func(t *testing.T) {
		s, _ := newTestServer(t, "")
		if h, err := NewHTTPHandler(s, good); err != nil || h == nil {
			t.Fatalf("got (%v, %v), want a handler", h, err)
		}
	})
}

func TestClockSkewMatchesVerifier(t *testing.T) {
	if auth.ClockSkew != 60*time.Second {
		t.Errorf("auth.ClockSkew = %v, want 60s", auth.ClockSkew)
	}
}
