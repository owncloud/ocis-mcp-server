package client

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/owncloud/ocis-mcp-server/internal/config"
)

// Client is the shared HTTP client for all oCIS API calls.
type Client struct {
	http    *http.Client
	baseURL string
	cfg     *config.Config
}

// New creates a Client from the given configuration.
func New(cfg *config.Config) *Client {
	return &Client{
		http:    cfg.NewHTTPClient(),
		baseURL: cfg.OcisBaseURL(),
		cfg:     cfg,
	}
}

// Config returns the underlying configuration.
func (c *Client) Config() *config.Config {
	return c.cfg
}

// NewRequest creates an HTTP request with the base URL prepended and auth injected.
func (c *Client) NewRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	url := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	c.applyAuth(req)
	return req, nil
}

// Do executes an HTTP request with rate-limit retry logic and error mapping.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	const maxRetries = 3

	for attempt := range maxRetries {
		resp, err := c.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("HTTP request failed: %w", err)
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			_ = resp.Body.Close()
			if attempt == maxRetries-1 {
				return nil, &APIError{
					StatusCode: 429,
					Message:    "rate limited by oCIS after 3 retries",
				}
			}
			retryAfter := resp.Header.Get("Retry-After")
			delay := parseRetryAfter(retryAfter)
			slog.Warn("rate limited, retrying", "attempt", attempt+1, "delay", delay)
			time.Sleep(delay)
			continue
		}

		return resp, nil
	}

	return nil, fmt.Errorf("exhausted retries")
}

// DoJSON performs a request and checks for a successful status code. Returns the response for body reading.
func (c *Client) DoJSON(req *http.Request) (*http.Response, error) {
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "application/json")
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		defer func() { _ = resp.Body.Close() }()
		return nil, errorFromResponse(resp)
	}
	return resp, nil
}

func (c *Client) applyAuth(req *http.Request) {
	switch c.cfg.AuthMode {
	case "app-token":
		req.SetBasicAuth(c.cfg.AppTokenUser, c.cfg.AppTokenValue)
	case "oidc":
		req.Header.Set("Authorization", "Bearer "+c.cfg.OidcAccessToken)
	}
}

// BaseURL returns the oCIS base URL.
func (c *Client) BaseURL() string {
	return c.baseURL
}

func parseRetryAfter(val string) time.Duration {
	if val == "" {
		return 2 * time.Second
	}
	if secs, err := strconv.Atoi(val); err == nil {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(val); err == nil {
		d := time.Until(t)
		if d > 0 {
			return d
		}
	}
	return 2 * time.Second
}

// ValidatePath rejects path traversal attempts.
func ValidatePath(path string) error {
	if strings.Contains(path, "..") {
		return fmt.Errorf("path traversal detected: %q", path)
	}
	if strings.ContainsAny(path, "\x00") {
		return fmt.Errorf("null byte in path: %q", path)
	}
	return nil
}

// ValidateID checks that an ID parameter is non-empty.
func ValidateID(name, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is required", name)
	}
	return nil
}

// ValidateLimit clamps limit to [1, 200] and returns a validated value.
func ValidateLimit(limit int) int {
	if limit <= 0 {
		return 50
	}
	if limit > 200 {
		return 200
	}
	return limit
}
