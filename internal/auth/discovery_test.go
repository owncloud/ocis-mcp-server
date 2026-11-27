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
		name    string
		opts    idpOptions
		setup   func(*testIdP)
		issuer  func(*testIdP) string // defaults to idp.Issuer()
		wantErr string                // empty means success
		check   func(*testing.T, *testIdP)
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
			name:   "configured issuer has trailing slash",
			issuer: func(i *testIdP) string { return i.Issuer() + "/" },
		},
		{
			name:  "discovered issuer has trailing slash",
			setup: func(i *testIdP) { i.Set(func(c *idpConfig) { c.issuerOverride = i.Issuer() + "/" }) },
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
			name:    "redirect from https to http rejected",
			setup:   func(i *testIdP) { i.Set(func(c *idpConfig) { c.discoveryMode = "redirect-http" }) },
			wantErr: "redirect",
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
			if got != idp.JWKSURL() {
				t.Fatalf("jwks uri = %q, want %q", got, idp.JWKSURL())
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
	via := func(n int) []*http.Request {
		out := make([]*http.Request, n)
		for i := range out {
			out[i] = req("https://idp.example.com/start")
		}
		return out
	}
	tests := []struct {
		name    string
		target  string
		hops    int
		wantErr bool
	}{
		{"https target", "https://idp.example.com/next", 1, false},
		{"http non-loopback target", "http://idp.example.com/next", 1, true},
		{"http loopback ip target", "http://127.0.0.1:8080/next", 1, false},
		{"http localhost target any case", "http://LocalHost:8080/next", 1, false},
		{"http ipv6 loopback target", "http://[::1]:8080/next", 1, false},
		{"third redirect still allowed", "https://idp.example.com/next", 3, false},
		{"fourth redirect refused", "https://idp.example.com/next", 4, true},
		{"other scheme refused", "ftp://idp.example.com/next", 1, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := c.CheckRedirect(req(tt.target), via(tt.hops))
			if (err != nil) != tt.wantErr {
				t.Fatalf("CheckRedirect error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
