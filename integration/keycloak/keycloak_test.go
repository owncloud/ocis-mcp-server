//go:build integration

// Package keycloak proves the OIDC inbound authentication against a real Keycloak.
//
// The test starts and removes its own container, so Docker must be running. It is
// equivalent to:
//
//	docker run --rm -p 127.0.0.1:18080:8080 \
//	  -e KC_HOSTNAME=http://localhost:18080 -e KC_HTTP_ENABLED=true \
//	  -v ./integration/keycloak/realm.json:/opt/keycloak/data/import/realm.json:ro \
//	  quay.io/keycloak/keycloak:26.2.5 start-dev --import-realm
//
// Run it with `make test-integration`.
//
// Ports are fixed because realm.json bakes the audience in:
//   - keycloakPort 18080: Keycloak, issuer http://localhost:18080/realms/mcp-test
//   - publicPort 18090: the public URL the MCP server claims, so the canonical resource
//     is http://localhost:18090/mcp (the audience mapper in realm.json).
//
// The test server itself listens on a random port. Only the configured public URL
// matters for token validation, not where httptest binds.
package keycloak

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/owncloud/ocis-mcp-server/internal/auth"
	"github.com/owncloud/ocis-mcp-server/internal/client"
	"github.com/owncloud/ocis-mcp-server/internal/config"
	"github.com/owncloud/ocis-mcp-server/internal/mcpserver"
)

const (
	keycloakImage = "quay.io/keycloak/keycloak:26.2.5"
	keycloakPort  = "18080"
	publicPort    = "18090"
	realm         = "mcp-test"

	issuer    = "http://localhost:" + keycloakPort + "/realms/" + realm
	publicURL = "http://localhost:" + publicPort

	// Dummy value that only exists in realm.json and here.
	clientSecret = "dummy-integration-secret-not-for-use"

	audienceClient   = "mcp-audience-client"
	noAudienceClient = "mcp-no-audience-client"
)

var testHTTP = &http.Client{Timeout: 15 * time.Second}

func startKeycloak(t *testing.T) {
	t.Helper()
	realmFile, err := filepath.Abs("realm.json")
	if err != nil {
		t.Fatalf("realm path: %v", err)
	}
	suffix := make([]byte, 4)
	if _, err := rand.Read(suffix); err != nil {
		t.Fatalf("random name: %v", err)
	}
	name := "ocis-mcp-it-keycloak-" + hex.EncodeToString(suffix)

	out, err := exec.Command("docker", "run", "-d", "--rm", "--name", name,
		"-p", "127.0.0.1:"+keycloakPort+":8080",
		"-e", "KC_HOSTNAME=http://localhost:"+keycloakPort,
		"-e", "KC_HTTP_ENABLED=true",
		"-e", "KC_BOOTSTRAP_ADMIN_USERNAME=admin",
		"-e", "KC_BOOTSTRAP_ADMIN_PASSWORD=dummy-integration-admin-not-for-use",
		"-v", realmFile+":/opt/keycloak/data/import/realm.json:ro",
		keycloakImage, "start-dev", "--import-realm",
	).CombinedOutput()
	if err != nil {
		t.Fatalf("docker run failed: %v\n%s", err, out)
	}
	// Remove only the container created above, by its unique name.
	t.Cleanup(func() { _ = exec.Command("docker", "rm", "-f", name).Run() })

	deadline := time.Now().Add(4 * time.Minute)
	for {
		resp, err := testHTTP.Get(issuer + "/.well-known/openid-configuration")
		if err == nil {
			var doc struct {
				Issuer string `json:"issuer"`
			}
			decErr := json.NewDecoder(resp.Body).Decode(&doc)
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK && decErr == nil {
				if doc.Issuer != issuer {
					t.Fatalf("discovery issuer = %q, want %q", doc.Issuer, issuer)
				}
				return
			}
		}
		if time.Now().After(deadline) {
			logs, _ := exec.Command("docker", "logs", "--tail", "40", name).CombinedOutput()
			t.Fatalf("keycloak not ready in time, last log lines:\n%s", logs)
		}
		time.Sleep(2 * time.Second)
	}
}

// accessToken fetches a client credentials token. The token is never logged.
func accessToken(t *testing.T, clientID, scope string) string {
	t.Helper()
	form := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
	}
	if scope != "" {
		form.Set("scope", scope)
	}
	resp, err := testHTTP.PostForm(issuer+"/protocol/openid-connect/token", form)
	if err != nil {
		t.Fatalf("token request for %s: %v", clientID, err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("token response for %s: %v", clientID, err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("token request for %s: status %d", clientID, resp.StatusCode)
	}
	var tok struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(raw, &tok); err != nil || tok.AccessToken == "" {
		t.Fatalf("token response for %s has no access_token", clientID)
	}
	return tok.AccessToken
}

// startMCP builds the real handler the way cmd/ocis-mcp-server does in oidc mode.
func startMCP(t *testing.T) string {
	t.Helper()
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ocs/v1.php/cloud/capabilities" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ocs":{"data":{"version":{"major":7,"minor":1,"micro":3,"string":"7.1.3","edition":"test","product":"Infinite Scale"},"capabilities":{}}}}`))
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(backend.Close)

	cfg := &config.Config{
		OcisURL:        backend.URL,
		AuthMode:       "app-token",
		AppTokenUser:   "admin",
		AppTokenValue:  "test-token",
		Transport:      "http",
		HTTPAddr:       "127.0.0.1:0",
		LogLevel:       "info",
		HTTPTimeout:    30 * time.Second,
		HTTPAuth:       mcpserver.AuthModeOIDC,
		HTTPPublicURL:  publicURL,
		HTTPOIDCIssuer: issuer,
		HTTPOIDCScopes: []string{"ocis:mcp"},
	}
	server := mcpserver.New("integration", client.New(cfg), cfg)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	hc, err := auth.NewIssuerHTTPClient("")
	if err != nil {
		t.Fatalf("issuer http client: %v", err)
	}
	v, err := auth.NewVerifier(ctx, auth.VerifierOptions{
		Issuer:     cfg.HTTPOIDCIssuer,
		Resource:   cfg.ResourceURI(),
		HTTPClient: hc,
	})
	if err != nil {
		t.Fatalf("verifier: %v", err)
	}
	h, err := mcpserver.NewHTTPHandler(server, mcpserver.HTTPAuth{
		Mode:        mcpserver.AuthModeOIDC,
		PublicURL:   cfg.HTTPPublicURL,
		Resource:    cfg.ResourceURI(),
		Issuer:      cfg.HTTPOIDCIssuer,
		MetadataURL: cfg.MetadataURL(),
		Scopes:      cfg.HTTPOIDCScopes,
		Verifier:    v.Verify,
	})
	if err != nil {
		t.Fatalf("NewHTTPHandler: %v", err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv.URL + "/mcp"
}

// reply is what the MCP endpoint answered.
type reply struct {
	status    int
	challenge string
	body      string
	rpc       map[string]any
}

// postToolsList sends tools/list with the public host, because the handler checks the
// Host header against the public URL while the connection goes to the httptest listener.
// The reply body is JSON or an event stream whose first data line carries the message.
func postToolsList(t *testing.T, mcpURL, token string) reply {
	t.Helper()
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{}}}}`
	req, err := http.NewRequest(http.MethodPost, mcpURL, strings.NewReader(body))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	req.Host = "localhost:" + publicPort
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", "2026-07-28")
	req.Header.Set("Mcp-Method", "tools/list")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := testHTTP.Do(req)
	if err != nil {
		t.Fatalf("POST /mcp: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read /mcp reply: %v", err)
	}
	r := reply{status: resp.StatusCode, challenge: resp.Header.Get("WWW-Authenticate"), body: string(raw)}
	payload := raw
	if strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		payload = nil
		for _, line := range strings.Split(string(raw), "\n") {
			if rest, ok := strings.CutPrefix(line, "data:"); ok {
				payload = []byte(strings.TrimSpace(rest))
				break
			}
		}
	}
	_ = json.Unmarshal(payload, &r.rpc)
	return r
}

// claims decodes the JWT payload in memory to check the test's preconditions. Neither
// the token nor the payload is ever printed.
func claims(t *testing.T, token string) (audiences []string, scopes []string) {
	t.Helper()
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatal("access token is not a JWT")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode token payload: %v", err)
	}
	var c struct {
		Aud   json.RawMessage `json:"aud"`
		Scope string          `json:"scope"`
	}
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatalf("parse token payload: %v", err)
	}
	var one string
	var many []string
	switch {
	case json.Unmarshal(c.Aud, &many) == nil:
		audiences = many
	case json.Unmarshal(c.Aud, &one) == nil:
		audiences = []string{one}
	}
	return audiences, strings.Fields(c.Scope)
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

func TestKeycloakInbound(t *testing.T) {
	startKeycloak(t)
	mcpURL := startMCP(t)
	resource := publicURL + "/mcp"
	metadataURL := publicURL + "/.well-known/oauth-protected-resource/mcp"

	tests := []struct {
		name      string
		token     string
		wantAud   bool // the token names the resource in aud
		wantScope bool // the token carries ocis:mcp
		want      int
		wantBody  string // fixed rejection reason
	}{
		{"valid audience and scope", accessToken(t, audienceClient, "ocis:mcp"), true, true, http.StatusOK, ""},
		{"wrong audience", accessToken(t, noAudienceClient, "ocis:mcp"), false, true, http.StatusUnauthorized, "invalid token: audience"},
		{"missing scope", accessToken(t, audienceClient, ""), true, false, http.StatusForbidden, "insufficient scope"},
		{"no token", "", false, false, http.StatusUnauthorized, "no bearer token"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.token != "" {
				aud, scopes := claims(t, tt.token)
				if got := contains(aud, resource); got != tt.wantAud {
					t.Fatalf("precondition: aud contains resource = %v, want %v", got, tt.wantAud)
				}
				if got := contains(scopes, "ocis:mcp"); got != tt.wantScope {
					t.Fatalf("precondition: scope contains ocis:mcp = %v, want %v", got, tt.wantScope)
				}
			}
			r := postToolsList(t, mcpURL, tt.token)
			if r.status != tt.want {
				t.Fatalf("status = %d, want %d", r.status, tt.want)
			}
			if tt.want == http.StatusOK {
				result, _ := r.rpc["result"].(map[string]any)
				if tools, _ := result["tools"].([]any); len(tools) == 0 {
					t.Error("200 reply has no tools in result")
				}
				return
			}
			if !strings.Contains(r.challenge, "resource_metadata="+`"`+metadataURL+`"`) {
				t.Errorf("challenge lacks resource_metadata for %s", metadataURL)
			}
			if !strings.Contains(r.challenge, "ocis:mcp") {
				t.Error("challenge lacks the required scope")
			}
			if got := strings.TrimSpace(r.body); got != tt.wantBody {
				t.Errorf("rejection reason = %q, want %q", got, tt.wantBody)
			}
		})
	}
}
