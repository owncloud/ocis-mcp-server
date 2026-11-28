package auth

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscover(t *testing.T) {
	tests := []struct {
		name     string
		opts     idpOptions
		setup    func(*testIdP)
		issuer   func(*testIdP) string // defaults to idp.Issuer()
		wantErr  string                // empty means success
		wantJWKS string                // expected jwks_uri; defaults to idp.JWKSURL()
		check    func(*testing.T, *testIdP)
	}{
		{
			name: "OIDC document ok",
			check: func(t *testing.T, idp *testIdP) {
				if idp.oidcHits.Load() != 1 || idp.rfcHits.Load() != 0 {
					t.Fatalf("hits oidc=%d rfc=%d, want 1 and 0", idp.oidcHits.Load(), idp.rfcHits.Load())
				}
			},
		},
		{
			name: "OIDC document ok for issuer with path",
			opts: idpOptions{path: "/realms/test"},
		},
		{
			name:  "OIDC 404 then RFC 8414 ok",
			setup: func(i *testIdP) { i.Set(func(c *idpConfig) { c.oidcStatus = http.StatusNotFound }) },
			check: func(t *testing.T, idp *testIdP) {
				if idp.oidcHits.Load() != 1 || idp.rfcHits.Load() != 1 {
					t.Fatalf("hits oidc=%d rfc=%d, want 1 and 1", idp.oidcHits.Load(), idp.rfcHits.Load())
				}
			},
		},
		{
			name:  "OIDC 500 then RFC 8414 ok for issuer with path",
			opts:  idpOptions{path: "/realms/test"},
			setup: func(i *testIdP) { i.Set(func(c *idpConfig) { c.oidcStatus = http.StatusInternalServerError }) },
		},
		{
			name: "both well-known documents fail",
			setup: func(i *testIdP) {
				i.Set(func(c *idpConfig) { c.oidcStatus = http.StatusNotFound; c.rfcStatus = http.StatusNotFound })
			},
			wantErr: "status 404",
		},
		{
			name:    "issuer mismatch",
			setup:   func(i *testIdP) { i.Set(func(c *idpConfig) { c.issuerOverride = "https://evil.example.com" }) },
			wantErr: "issuer",
		},
		{
			name:    "issuer differs only in case",
			setup:   func(i *testIdP) { i.Set(func(c *idpConfig) { c.issuerOverride = strings.ToUpper(i.Issuer()) }) },
			wantErr: "issuer",
		},
		{
			name:    "configured issuer has trailing slash, discovered has none",
			issuer:  func(i *testIdP) string { return i.Issuer() + "/" },
			wantErr: "issuer",
		},
		{
			name:    "discovered issuer has trailing slash, configured has none",
			setup:   func(i *testIdP) { i.Set(func(c *idpConfig) { c.issuerOverride = i.Issuer() + "/" }) },
			wantErr: "issuer",
		},
		{
			name:   "both issuers have the same trailing slash",
			issuer: func(i *testIdP) string { return i.Issuer() + "/" },
			setup:  func(i *testIdP) { i.Set(func(c *idpConfig) { c.issuerOverride = i.Issuer() + "/" }) },
		},
		{
			name:    "two trailing slashes are not tolerated",
			setup:   func(i *testIdP) { i.Set(func(c *idpConfig) { c.issuerOverride = i.Issuer() + "//" }) },
			wantErr: "issuer",
		},
		{
			name:    "http issuer on a non-loopback host rejected",
			issuer:  func(*testIdP) string { return "http://idp.example.com" },
			wantErr: "https",
		},
		{
			name:    "issuer with query rejected",
			issuer:  func(i *testIdP) string { return i.Issuer() + "?x=1" },
			wantErr: "query",
		},
		{
			name:    "empty issuer rejected",
			issuer:  func(*testIdP) string { return "" },
			wantErr: "issuer",
		},
		{
			name: "http loopback issuer allowed",
			opts: idpOptions{plainHTTP: true},
		},
		{
			name:    "jwks_uri http on a non-loopback host rejected",
			setup:   func(i *testIdP) { i.Set(func(c *idpConfig) { c.jwksURIOverride = "http://idp.example.com/jwks" }) },
			wantErr: "jwks_uri",
		},
		{
			name:    "https issuer with an http loopback jwks_uri rejected",
			setup:   func(i *testIdP) { i.Set(func(c *idpConfig) { c.jwksURIOverride = "http://127.0.0.1:9/jwks" }) },
			wantErr: "jwks_uri must use https",
		},
		{
			name:    "https issuer with an http localhost jwks_uri rejected",
			setup:   func(i *testIdP) { i.Set(func(c *idpConfig) { c.jwksURIOverride = "http://localhost:9/jwks" }) },
			wantErr: "jwks_uri must use https",
		},
		{
			name: "http loopback issuer with an http loopback jwks_uri allowed",
			opts: idpOptions{plainHTTP: true},
			setup: func(i *testIdP) {
				i.Set(func(c *idpConfig) { c.jwksURIOverride = "http://127.0.0.1:9/jwks" })
			},
			wantJWKS: "http://127.0.0.1:9/jwks",
		},
		{
			name:    "jwks_uri missing rejected",
			setup:   func(i *testIdP) { i.Set(func(c *idpConfig) { c.jwksURIOverride = " " }) },
			wantErr: "jwks_uri",
		},
		{
			name:    "body over 1 MiB rejected",
			setup:   func(i *testIdP) { i.Set(func(c *idpConfig) { c.discoveryMode = "huge" }) },
			wantErr: "1 MiB",
		},
		{
			name: "redirect from https to an answering http loopback server rejected",
			setup: func(i *testIdP) {
				target := newHTTPLoopbackTarget(t, i.Issuer(), i.JWKSURL())
				i.Set(func(c *idpConfig) {
					c.discoveryMode = "redirect-http"
					c.redirectTo = target.Issuer() + "/.well-known/openid-configuration"
				})
			},
			wantErr: "redirect target must use https",
		},
		{
			name:    "redirect from https to http on a non-loopback host rejected",
			setup:   func(i *testIdP) { i.Set(func(c *idpConfig) { c.discoveryMode = "redirect-http" }) },
			wantErr: "redirect target must use https",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idp := newTestIdP(t, tt.opts)
			if tt.setup != nil {
				tt.setup(idp)
			}
			issuer := idp.Issuer()
			if tt.issuer != nil {
				issuer = tt.issuer(idp)
			}
			got, err := Discover(context.Background(), idp.Client(t), issuer)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("Discover succeeded with %q, want error containing %q", got, tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error %q does not contain %q", err, tt.wantErr)
				}
				if strings.Contains(err.Error(), "secret-body-must-not-leak") || strings.Contains(err.Error(), "aaaa") {
					t.Fatalf("error leaks a response body: %q", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Discover: %v", err)
			}
			want := idp.JWKSURL()
			if tt.wantJWKS != "" {
				want = tt.wantJWKS
			}
			if got != want {
				t.Fatalf("jwks uri = %q, want %q", got, want)
			}
			if tt.check != nil {
				tt.check(t, idp)
			}
		})
	}
}

func TestDiscoverNilClient(t *testing.T) {
	if _, err := Discover(context.Background(), nil, "https://idp.example.com"); err == nil {
		t.Fatal("Discover with a nil client succeeded")
	}
}

func TestNewIssuerHTTPClient(t *testing.T) {
	idp := newTestIdP(t, idpOptions{})

	t.Run("trusts the extra CA", func(t *testing.T) {
		c, err := NewIssuerHTTPClient(idp.CAFile(t))
		if err != nil {
			t.Fatal(err)
		}
		resp, err := c.Get(idp.JWKSURL())
		if err != nil {
			t.Fatalf("GET with extra CA: %v", err)
		}
		_ = resp.Body.Close()
	})

	t.Run("does not trust the fake IdP without the CA", func(t *testing.T) {
		c, err := NewIssuerHTTPClient("")
		if err != nil {
			t.Fatal(err)
		}
		resp, err := c.Get(idp.JWKSURL())
		if err == nil {
			_ = resp.Body.Close()
			t.Fatal("GET without the CA succeeded, verification is not enforced")
		}
	})

	t.Run("never skips verification", func(t *testing.T) {
		c, err := NewIssuerHTTPClient(idp.CAFile(t))
		if err != nil {
			t.Fatal(err)
		}
		tr, ok := c.Transport.(*http.Transport)
		if !ok {
			t.Fatalf("transport is %T, want *http.Transport", c.Transport)
		}
		if tr.TLSClientConfig == nil || tr.TLSClientConfig.InsecureSkipVerify {
			t.Fatal("TLS config missing or InsecureSkipVerify set")
		}
	})

	t.Run("timeout is 10s", func(t *testing.T) {
		c, err := NewIssuerHTTPClient("")
		if err != nil {
			t.Fatal(err)
		}
		if c.Timeout.Seconds() != 10 {
			t.Fatalf("timeout = %v, want 10s", c.Timeout)
		}
	})

	t.Run("missing CA file", func(t *testing.T) {
		if _, err := NewIssuerHTTPClient(filepath.Join(t.TempDir(), "nope.pem")); err == nil {
			t.Fatal("missing CA file accepted")
		}
	})

	t.Run("CA file without certificates", func(t *testing.T) {
		p := filepath.Join(t.TempDir(), "empty.pem")
		if err := os.WriteFile(p, []byte("not a certificate"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := NewIssuerHTTPClient(p); err == nil {
			t.Fatal("CA file without certificates accepted")
		}
	})
}

func TestIssuerClientRedirectPolicy(t *testing.T) {
	c, err := NewIssuerHTTPClient("")
	if err != nil {
		t.Fatal(err)
	}
	req := func(raw string) *http.Request {
		u, perr := url.Parse(raw)
		if perr != nil {
			t.Fatal(perr)
		}
		return &http.Request{URL: u}
	}
	via := func(source string, n int) []*http.Request {
		out := make([]*http.Request, n)
		for i := range out {
			out[i] = req(source)
		}
		return out
	}
	const (
		httpsSource    = "https://idp.example.com/start"
		loopbackSource = "http://127.0.0.1:8080/start"
	)
	tests := []struct {
		name    string
		source  string
		target  string
		hops    int
		wantErr string // empty means the redirect is followed
	}{
		{"https to https", httpsSource, "https://idp.example.com/next", 1, ""},
		{"https to http non-loopback", httpsSource, "http://idp.example.com/next", 1, "https"},
		{"https to http loopback ip", httpsSource, "http://127.0.0.1:8080/next", 1, "https"},
		{"https to http localhost any case", httpsSource, "http://LocalHost:8080/next", 1, "https"},
		{"https to http ipv6 loopback", httpsSource, "http://[::1]:8080/next", 1, "https"},
		{"https to other scheme", httpsSource, "ftp://idp.example.com/next", 1, "https"},
		{"https to target with userinfo", httpsSource, "https://user:pw@idp.example.com/next", 1, "credentials"},
		{"https to target with fragment", httpsSource, "https://idp.example.com/next#frag", 1, "fragment"},
		{"https to target without host", httpsSource, "https:///next", 1, "absolute"},
		{"loopback http to loopback http", loopbackSource, "http://127.0.0.1:8080/next", 1, ""},
		{"loopback http to https", loopbackSource, "https://idp.example.com/next", 1, ""},
		{"loopback http to http non-loopback", loopbackSource, "http://idp.example.com/next", 1, "https"},
		{"third redirect still allowed", httpsSource, "https://idp.example.com/next", 3, ""},
		{"fourth redirect refused", httpsSource, "https://idp.example.com/next", 4, "redirects"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := c.CheckRedirect(req(tt.target), via(tt.source, tt.hops))
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("CheckRedirect = %v, want the redirect followed", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("CheckRedirect error = %v, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}
