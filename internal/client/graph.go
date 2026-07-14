package client

import (
	"net/http"
	"time"

	libregraph "github.com/owncloud/libre-graph-api-go"
)

// GraphClient returns a libre-graph-api-go SDK client configured for this oCIS
// instance. Auth (app-token Basic / OIDC Bearer) and HTTP 429 retry are applied
// by graphTransport, so callers use the returned client with a plain context.
//
// The SDK's operation paths are relative to a server URL that includes the
// "/graph" prefix (e.g. server "<base>/graph" + path "/v1beta1/drives/..."),
// which is why the version prefixes no longer need to be hand-written.
func (c *Client) GraphClient() *libregraph.APIClient {
	base := http.DefaultTransport
	if c.http != nil && c.http.Transport != nil {
		base = c.http.Transport
	}

	cfg := libregraph.NewConfiguration()
	cfg.Servers = libregraph.ServerConfigurations{{URL: c.baseURL + "/graph"}}
	cfg.HTTPClient = &http.Client{
		Transport: &graphTransport{base: base, client: c},
		Timeout:   c.http.Timeout,
	}
	return libregraph.NewAPIClient(cfg)
}

// graphTransport gives SDK requests the same auth + rate-limit handling as the
// hand-rolled helpers: it injects the configured auth header and retries on 429.
type graphTransport struct {
	base   http.RoundTripper
	client *Client
}

func (t *graphTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.client.applyAuth(req)

	const maxRetries = 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		r := req
		// Replay the body on retries when possible.
		if req.GetBody != nil {
			body, err := req.GetBody()
			if err != nil {
				return nil, err
			}
			r = req.Clone(req.Context())
			r.Body = body
		}

		resp, err := t.base.RoundTrip(r)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode == http.StatusTooManyRequests && attempt < maxRetries-1 {
			_ = resp.Body.Close()
			time.Sleep(parseRetryAfter(resp.Header.Get("Retry-After")))
			continue
		}
		return resp, nil
	}

	return nil, &APIError{StatusCode: http.StatusTooManyRequests, Message: "rate limited by oCIS after 3 retries"}
}
