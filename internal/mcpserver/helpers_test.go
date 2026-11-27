package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/owncloud/ocis-mcp-server/internal/client"
	"github.com/owncloud/ocis-mcp-server/internal/config"
)

const capabilitiesBody = `{"ocs":{"data":{"version":{"major":7,"minor":1,"micro":3,"string":"7.1.3","edition":"test","product":"Infinite Scale"},"capabilities":{}}}}`

// fakeOCIS is a stand-in oCIS backend with a request counter and a swappable handler.
type fakeOCIS struct {
	srv  *httptest.Server
	hits atomic.Int64
	mu   sync.RWMutex
	h    http.HandlerFunc
}

func newFakeOCIS(t *testing.T) *fakeOCIS {
	t.Helper()
	f := &fakeOCIS{}
	f.h = func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ocs/v1.php/cloud/capabilities" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(capabilitiesBody))
			return
		}
		http.NotFound(w, r)
	}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.hits.Add(1)
		f.mu.RLock()
		h := f.h
		f.mu.RUnlock()
		h(w, r)
	}))
	t.Cleanup(f.srv.Close)
	return f
}

// Hits returns the number of requests received so far.
func (f *fakeOCIS) Hits() int { return int(f.hits.Load()) }

// SetHandler replaces the handler serving requests.
func (f *fakeOCIS) SetHandler(h http.HandlerFunc) {
	f.mu.Lock()
	f.h = h
	f.mu.Unlock()
}

func newTestConfig(srvURL, educationToken string) *config.Config {
	return &config.Config{
		OcisURL:              srvURL,
		AuthMode:             "app-token",
		AppTokenUser:         "admin",
		AppTokenValue:        "test-token",
		EducationAccessToken: educationToken,
		Transport:            "stdio",
		HTTPAddr:             "127.0.0.1:8090",
		LogLevel:             "info",
		HTTPTimeout:          30 * time.Second,
		Insecure:             true,
	}
}

// newTestServer builds the MCP server against a fake oCIS.
func newTestServer(t *testing.T, educationToken string) (*mcp.Server, *fakeOCIS) {
	t.Helper()
	fake := newFakeOCIS(t)
	cfg := newTestConfig(fake.srv.URL, educationToken)
	return New("test", client.New(cfg), cfg), fake
}

// httpClient bounds every test request so a hung handler cannot stall the suite.
var httpClient = &http.Client{Timeout: 10 * time.Second}

// startHTTP serves NewHTTPHandler on an httptest server and returns the /mcp URL
// together with the fake oCIS backing it.
func startHTTP(t *testing.T, secret string) (string, *fakeOCIS) {
	t.Helper()
	s, fake := newTestServer(t, "")
	srv := httptest.NewServer(NewHTTPHandler(s, secret))
	t.Cleanup(srv.Close)
	return srv.URL + "/mcp", fake
}

// rawPost POSTs body and decodes the reply, which is either application/json or
// an event stream whose first data line carries the JSON-RPC message.
func rawPost(t *testing.T, url string, headers map[string]string, body string) (int, http.Header, map[string]any) {
	t.Helper()
	status, hdr, rpc, err := doRawPost(context.Background(), url, headers, body)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	return status, hdr, rpc
}

func doRawPost(ctx context.Context, url string, headers map[string]string, body string) (int, http.Header, map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		return 0, nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, nil, nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, nil, err
	}
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
	var rpc map[string]any
	if len(bytes.TrimSpace(payload)) > 0 {
		if err := json.Unmarshal(payload, &rpc); err != nil {
			rpc = map[string]any{"_raw": string(raw)}
		}
	}
	return resp.StatusCode, resp.Header, rpc, nil
}

// connectInMemory connects a client to s over an in-memory transport.
// An empty protocolVersion means the SDK's latest.
func connectInMemory(t *testing.T, s *mcp.Server, protocolVersion string) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	st, ct := mcp.NewInMemoryTransports()
	ss, err := s.Connect(ctx, st, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	t.Cleanup(func() { _ = ss.Close() })
	c := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0"}, nil)
	cs, err := c.Connect(ctx, ct, &mcp.ClientSessionOptions{ProtocolVersion: protocolVersion})
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}
