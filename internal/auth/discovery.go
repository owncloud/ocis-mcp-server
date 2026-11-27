package auth

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	// maxResponseBytes caps every body read from the issuer.
	maxResponseBytes = 1 << 20
	// issuerClientTimeout bounds a whole request to the issuer.
	issuerClientTimeout = 10 * time.Second
	// maxRedirects is the number of redirects the issuer client follows.
	maxRedirects = 3
)

var errResponseTooLarge = errors.New("response body exceeds the 1 MiB limit")

// statusError reports a non-200 answer. It carries the status code only,
// never the body.
type statusError struct{ code int }

func (e *statusError) Error() string { return fmt.Sprintf("unexpected HTTP status %d", e.code) }

// isLoopbackHost reports whether host (as returned by URL.Hostname) is a
// loopback name or address.
func isLoopbackHost(host string) bool {
	h := strings.ToLower(host)
	if h == "localhost" {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}

// schemeAllowed reports whether u may be fetched: https always, http only for
// a loopback host.
func schemeAllowed(u *url.URL) bool {
	switch strings.ToLower(u.Scheme) {
	case "https":
		return true
	case "http":
		return isLoopbackHost(u.Hostname())
	}
	return false
}

// parseSecureURL parses raw and enforces the https-or-loopback rule.
func parseSecureURL(raw, what string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("%s is not a valid URL", what)
	}
	if u.Hostname() == "" || u.User != nil || u.Fragment != "" {
		return nil, fmt.Errorf("%s must be an absolute URL without credentials or fragment", what)
	}
	if !schemeAllowed(u) {
		return nil, fmt.Errorf("%s must use https (plain http is allowed for a loopback host only)", what)
	}
	return u, nil
}

// NewIssuerHTTPClient returns the HTTP client used to talk to the issuer. It
// has a 10 s timeout, verifies TLS against the system roots plus the PEM
// bundle in caFile when given, and refuses redirects that leave https (except
// to a loopback host) or exceed three hops.
func NewIssuerHTTPClient(caFile string) (*http.Client, error) {
	tr, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, errors.New("default HTTP transport is not an *http.Transport")
	}
	tr = tr.Clone()
	tr.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	if caFile != "" {
		pool, err := x509.SystemCertPool()
		if err != nil {
			return nil, fmt.Errorf("load system CA pool: %w", err)
		}
		pem, err := os.ReadFile(caFile)
		if err != nil {
			return nil, fmt.Errorf("read issuer CA file: %w", err)
		}
		if !pool.AppendCertsFromPEM(pem) {
			return nil, errors.New("issuer CA file contains no PEM certificates")
		}
		tr.TLSClientConfig.RootCAs = pool
	}
	return &http.Client{
		Timeout:       issuerClientTimeout,
		Transport:     tr,
		CheckRedirect: checkIssuerRedirect,
	}, nil
}

func checkIssuerRedirect(req *http.Request, via []*http.Request) error {
	if len(via) > maxRedirects {
		return fmt.Errorf("stopped after %d redirects", maxRedirects)
	}
	if !schemeAllowed(req.URL) {
		return errors.New("redirect target must use https (plain http is allowed for a loopback host only)")
	}
	return nil
}

// getJSON GETs rawURL and decodes a JSON object into v. The body is capped at
// 1 MiB; reaching the cap is an error, never a silent truncation. Errors carry
// no response content.
func getJSON(ctx context.Context, c *http.Client, rawURL string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return errors.New("build request")
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.Do(req)
	if err != nil {
		var uerr *url.Error
		if errors.As(err, &uerr) {
			// Drop the URL: only the cause is useful and it is never a body.
			err = uerr.Err
		}
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	// Belt and braces: a client without our redirect policy must not end up on http.
	if resp.Request != nil && resp.Request.URL != nil && !schemeAllowed(resp.Request.URL) {
		return errors.New("response came from a redirect to a non-https URL")
	}
	if resp.StatusCode != http.StatusOK {
		return &statusError{code: resp.StatusCode}
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if len(body) > maxResponseBytes {
		return errResponseTooLarge
	}
	if err := json.Unmarshal(body, v); err != nil {
		return errors.New("response is not a valid JSON document")
	}
	return nil
}

// trimOneSlash removes a single trailing slash.
func trimOneSlash(s string) string { return strings.TrimSuffix(s, "/") }

// Discover resolves the JWKS URI of issuer. It tries OIDC Discovery first and
// RFC 8414 second (only when the first answers with a non-200 status). The
// discovered issuer must equal the configured one, tolerating a single
// trailing slash difference, and both the issuer and the jwks_uri must be
// https, except plain http for a loopback host.
func Discover(ctx context.Context, c *http.Client, issuer string) (jwksURI string, err error) {
	if c == nil {
		return "", errors.New("discovery needs an HTTP client")
	}
	iss, err := parseSecureURL(issuer, "issuer")
	if err != nil {
		return "", err
	}
	if iss.RawQuery != "" || iss.ForceQuery {
		return "", errors.New("issuer must not contain a query")
	}
	base := trimOneSlash(issuer)
	path := strings.TrimSuffix(iss.EscapedPath(), "/")
	rfc := *iss
	rfc.RawQuery, rfc.Fragment = "", ""
	rfc.Path, rfc.RawPath = "", ""
	rfc8414 := rfc.String() + "/.well-known/oauth-authorization-server" + path
	endpoints := []string{base + "/.well-known/openid-configuration", rfc8414}

	for i, endpoint := range endpoints {
		var doc struct {
			Issuer  string `json:"issuer"`
			JWKSURI string `json:"jwks_uri"`
		}
		err := getJSON(ctx, c, endpoint, &doc)
		var se *statusError
		if errors.As(err, &se) && i < len(endpoints)-1 {
			continue
		}
		if err != nil {
			if i == 0 {
				return "", fmt.Errorf("discovery (openid-configuration): %w", err)
			}
			return "", fmt.Errorf("discovery (oauth-authorization-server): %w", err)
		}
		if trimOneSlash(doc.Issuer) != trimOneSlash(issuer) {
			return "", errors.New("discovered issuer does not match the configured issuer")
		}
		if strings.TrimSpace(doc.JWKSURI) == "" {
			return "", errors.New("discovery document has no jwks_uri")
		}
		if _, err := parseSecureURL(doc.JWKSURI, "jwks_uri"); err != nil {
			return "", err
		}
		return doc.JWKSURI, nil
	}
	return "", errors.New("discovery failed")
}
