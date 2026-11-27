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

// postToolsList sends tools/list with the public host, because the handler checks the
// Host header against the public URL while the connection goes to the httptest listener.
func postToolsList(t *testing.T, mcpURL, token string) int {
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
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode
}

func TestKeycloakInbound(t *testing.T) {
	startKeycloak(t)
	mcpURL := startMCP(t)

	tests := []struct {
		name  string
		token string
		want  int
	}{
		{"valid audience and scope", accessToken(t, audienceClient, "ocis:mcp"), http.StatusOK},
		{"wrong audience", accessToken(t, noAudienceClient, "ocis:mcp"), http.StatusUnauthorized},
		{"missing scope", accessToken(t, audienceClient, ""), http.StatusForbidden},
		{"no token", "", http.StatusUnauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := postToolsList(t, mcpURL, tt.token); got != tt.want {
				t.Errorf("status = %d, want %d", got, tt.want)
			}
		})
	}
}
