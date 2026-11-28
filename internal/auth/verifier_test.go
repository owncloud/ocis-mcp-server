package auth

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	sdkauth "github.com/modelcontextprotocol/go-sdk/auth"
)

const (
	verifierResource = "https://mcp.example.test/mcp"
	verifierSubject  = "subject-value-0f3a"
	rsaKID           = "rsa-kid-7c1d"
	ecKID            = "ec-kid-9b2e"
)

type verifierEnv struct {
	idp   *testIdP
	rsa   *rsa.PrivateKey
	ec    crypto.Signer
	clock *fakeClock
	v     *Verifier
}

func newVerifierEnv(t *testing.T) *verifierEnv {
	t.Helper()
	idp := newTestIdP(t, idpOptions{path: "/realms/test"})
	e := &verifierEnv{idp: idp, clock: newFakeClock()}
	e.rsa = idp.AddRSAKey(t, rsaKID)
	e.ec = idp.AddECKey(t, ecKID)
	v, err := NewVerifier(context.Background(), VerifierOptions{
		Issuer:     idp.Issuer(),
		Resource:   verifierResource,
		HTTPClient: idp.Client(t),
		Now:        e.clock.Now,
	})
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	e.v = v
	return e
}

func (e *verifierEnv) claims() map[string]any {
	now := e.clock.Now()
	return map[string]any{
		"iss": e.idp.Issuer(),
		"sub": verifierSubject,
		"aud": verifierResource,
		"exp": now.Add(5 * time.Minute).Unix(),
		"iat": now.Unix(),
	}
}

// paddedToken returns a validly signed RS256 token whose length is within three
// characters of the 8 KiB cap, plus extra padding characters. extra 0 yields a
// token of 8189 to 8192 characters, extra 3 one of at least 8193.
func paddedToken(t *testing.T, e *verifierEnv, extra int) string {
	t.Helper()
	build := func(pad int) string {
		c := e.claims()
		c["pad"] = strings.Repeat("a", pad)
		return signToken(t, c, signOpts{Kid: rsaKID, Key: e.rsa})
	}
	pad := (maxTokenBytes - len(build(0))) * 3 / 4
	for i := 0; i < 20; i++ {
		n := len(build(pad))
		switch {
		case n > maxTokenBytes:
			pad--
		case n < maxTokenBytes-3:
			pad++
		default:
			return build(pad + extra)
		}
	}
	t.Fatal("could not size the padded token")
	return ""
}

func TestVerify(t *testing.T) {
	type row struct {
		name   string
		claims func(e *verifierEnv, c map[string]any)
		opts   func(e *verifierEnv) signOpts
		token  func(e *verifierEnv) string // overrides signing when set
		accept bool
		// reason is the fixed rejection reason the verifier must give. Pinning it
		// shows which layer rejected: the parser alone would give "signature" for
		// a mismatched key type, only the key binding gives "signing key".
		reason string
		// noKeyFetch requires that no JWKS request was made: the allow-list, the
		// kid check and the size guard must reject before any key lookup.
		noKeyFetch bool
	}
	rsaOpts := func(e *verifierEnv) signOpts { return signOpts{Kid: rsaKID, Key: e.rsa} }
	rows := []row{
		{name: "valid", accept: true},
		{name: "expired", claims: func(e *verifierEnv, c map[string]any) { c["exp"] = e.clock.Now().Add(-5 * time.Minute).Unix() }},
		{name: "expired inside leeway", accept: true, claims: func(e *verifierEnv, c map[string]any) { c["exp"] = e.clock.Now().Add(-30 * time.Second).Unix() }},
		{name: "expired beyond leeway", claims: func(e *verifierEnv, c map[string]any) { c["exp"] = e.clock.Now().Add(-90 * time.Second).Unix() }},
		{name: "nbf in future", claims: func(e *verifierEnv, c map[string]any) { c["nbf"] = e.clock.Now().Add(10 * time.Minute).Unix() }},
		{name: "nbf in past", accept: true, claims: func(e *verifierEnv, c map[string]any) { c["nbf"] = e.clock.Now().Add(-time.Minute).Unix() }},
		{name: "wrong iss", claims: func(e *verifierEnv, c map[string]any) { c["iss"] = "https://other.example.test/realms/test" }},
		{name: "iss with trailing slash", claims: func(e *verifierEnv, c map[string]any) { c["iss"] = e.idp.Issuer() + "/" }},
		{name: "missing iss", claims: func(e *verifierEnv, c map[string]any) { delete(c, "iss") }},
		{name: "wrong aud", claims: func(e *verifierEnv, c map[string]any) { c["aud"] = "https://mcp.example.test/other" }},
		{name: "aud string", accept: true, claims: func(e *verifierEnv, c map[string]any) { c["aud"] = verifierResource }},
		{name: "aud array with ours", accept: true, claims: func(e *verifierEnv, c map[string]any) { c["aud"] = []string{"web", verifierResource} }},
		{name: "aud array without ours", claims: func(e *verifierEnv, c map[string]any) { c["aud"] = []string{"web", "account"} }},
		// oauthex.MatchesResource deliberately tolerates one trailing slash on
		// the token audience; path, scheme and host stay strict.
		{name: "aud with trailing slash", accept: true, claims: func(e *verifierEnv, c map[string]any) { c["aud"] = verifierResource + "/" }},
		{name: "aud with different path prefix", claims: func(e *verifierEnv, c map[string]any) { c["aud"] = verifierResource + "/x" }},
		{name: "azp equals resource but aud web", claims: func(e *verifierEnv, c map[string]any) {
			c["aud"] = "web"
			c["azp"] = verifierResource
		}},
		{name: "missing aud", claims: func(e *verifierEnv, c map[string]any) { delete(c, "aud") }},
		{name: "empty aud array", claims: func(e *verifierEnv, c map[string]any) { c["aud"] = []string{} }},
		{name: "missing exp", claims: func(e *verifierEnv, c map[string]any) { delete(c, "exp") }},
		{name: "missing sub", claims: func(e *verifierEnv, c map[string]any) { delete(c, "sub") }},
		{name: "empty sub", claims: func(e *verifierEnv, c map[string]any) { c["sub"] = "" }},
		{name: "bad signature", reason: "signature", token: func(e *verifierEnv) string {
			tok := signToken(t, e.claims(), rsaOpts(e))
			i := strings.LastIndex(tok, ".")
			sig := []byte(tok[i+1:])
			if sig[10] == 'A' {
				sig[10] = 'B'
			} else {
				sig[10] = 'A'
			}
			return tok[:i+1] + string(sig)
		}},
		{name: "signed by foreign key", reason: "signature", opts: func(e *verifierEnv) signOpts {
			other := newTestIdP(t, idpOptions{path: "/x"})
			return signOpts{Kid: rsaKID, Key: other.AddRSAKey(t, "foreign")}
		}},
		{name: "unknown kid", reason: "signing key", opts: func(e *verifierEnv) signOpts { return signOpts{Kid: "unknown-kid-55aa", Key: e.rsa} }},
		{name: "no kid", reason: "signing key", noKeyFetch: true, opts: func(e *verifierEnv) signOpts { return signOpts{OmitKid: true, Key: e.rsa} }},
		{name: "empty kid", reason: "signing key", noKeyFetch: true, opts: func(e *verifierEnv) signOpts { return signOpts{Kid: "", Key: e.rsa} }},
		{name: "alg none", reason: "signature", noKeyFetch: true, opts: func(e *verifierEnv) signOpts { return signOpts{Alg: "none", Kid: rsaKID} }},
		{name: "HS256 with DER public key as secret", reason: "signature", noKeyFetch: true, opts: func(e *verifierEnv) signOpts {
			return signOpts{Alg: "HS256", Kid: rsaKID, HMACSecret: x509.MarshalPKCS1PublicKey(&e.rsa.PublicKey)}
		}},
		{name: "HS256 with PEM public key as secret", reason: "signature", noKeyFetch: true, opts: func(e *verifierEnv) signOpts {
			der, _ := x509.MarshalPKIXPublicKey(&e.rsa.PublicKey)
			return signOpts{Alg: "HS256", Kid: rsaKID, HMACSecret: pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})}
		}},
		{name: "ES256 accepted", accept: true, opts: func(e *verifierEnv) signOpts { return signOpts{Alg: "ES256", Kid: ecKID, Key: e.ec} }},
		{name: "PS256 accepted", accept: true, opts: func(e *verifierEnv) signOpts { return signOpts{Alg: "PS256", Kid: rsaKID, Key: e.rsa} }},
		{name: "RS256 header with EC key under kid", reason: "signing key", opts: func(e *verifierEnv) signOpts { return signOpts{Alg: "RS256", Kid: ecKID, Key: e.rsa} }},
		{name: "PS256 header with EC key under kid", reason: "signing key", opts: func(e *verifierEnv) signOpts { return signOpts{Alg: "PS256", Kid: ecKID, Key: e.rsa} }},
		{name: "ES256 header with RSA key under kid", reason: "signing key", opts: func(e *verifierEnv) signOpts { return signOpts{Alg: "ES256", Kid: rsaKID, Key: e.ec} }},
		{name: "empty token", reason: "empty", noKeyFetch: true, token: func(*verifierEnv) string { return "" }},
		{name: "token with a space", reason: "malformed", noKeyFetch: true, token: func(e *verifierEnv) string { return signToken(t, e.claims(), rsaOpts(e)) + " x" }},
		{name: "token with a newline", reason: "malformed", noKeyFetch: true, token: func(e *verifierEnv) string { return signToken(t, e.claims(), rsaOpts(e)) + "\n" }},
		{name: "validly signed token just under the size cap", accept: true, token: func(e *verifierEnv) string { return paddedToken(t, e, 0) }},
		{name: "validly signed token just over the size cap", reason: "too large", noKeyFetch: true, token: func(e *verifierEnv) string { return paddedToken(t, e, 3) }},
		{name: "oversized garbage", reason: "too large", noKeyFetch: true, token: func(e *verifierEnv) string { return strings.Repeat("a", 8*1024+1) }},
		{name: "not a jwt", reason: "malformed", noKeyFetch: true, token: func(*verifierEnv) string { return "opaque-access-token-value" }},
	}
	for _, tc := range rows {
		t.Run(tc.name, func(t *testing.T) {
			e := newVerifierEnv(t)
			var token string
			if tc.token != nil {
				token = tc.token(e)
			} else {
				c := e.claims()
				if tc.claims != nil {
					tc.claims(e, c)
				}
				o := rsaOpts(e)
				if tc.opts != nil {
					o = tc.opts(e)
				}
				token = signToken(t, c, o)
			}
			r := httptest.NewRequest(http.MethodPost, "/mcp", nil)
			info, err := e.v.Verify(context.Background(), token, r)
			if tc.accept {
				if err != nil {
					t.Fatalf("expected accept, got error: %v", err)
				}
				if info == nil || info.UserID != verifierSubject {
					t.Fatalf("unexpected token info: %+v", info)
				}
				if info.Extra["iss"] != e.idp.Issuer() {
					t.Fatalf("iss not exposed in Extra")
				}
				if info.Expiration.IsZero() {
					t.Fatalf("expiration not set")
				}
				return
			}
			if err == nil {
				t.Fatalf("expected rejection, token was accepted")
			}
			if info != nil {
				t.Fatalf("rejection returned token info")
			}
			if !errors.Is(err, sdkauth.ErrInvalidToken) {
				t.Fatalf("rejection does not wrap ErrInvalidToken: %v", err)
			}
			text := err.Error()
			if tc.reason != "" && text != "invalid token: "+tc.reason {
				t.Fatalf("rejection reason = %q, want %q", text, "invalid token: "+tc.reason)
			}
			if tc.noKeyFetch {
				if n := e.idp.jwksHits.Load(); n != 0 {
					t.Fatalf("JWKS was fetched %d times, want a rejection before any key lookup", n)
				}
			}
			for _, secret := range []string{rsaKID, ecKID, "unknown-kid-55aa", verifierSubject, "opaque-access-token-value"} {
				if strings.Contains(text, secret) {
					t.Fatalf("error text leaks a sensitive value: %q", secret)
				}
			}
			if token != "" && strings.Contains(text, token) {
				t.Fatalf("error text contains the token")
			}
			for _, part := range strings.Split(token, ".") {
				if len(part) > 12 && strings.Contains(text, part) {
					t.Fatalf("error text contains a token segment")
				}
			}
		})
	}
}

func TestVerifyScopes(t *testing.T) {
	tests := []struct {
		name   string
		claims map[string]any
		want   []string
	}{
		{"scope string", map[string]any{"scope": "openid files:read  files:write"}, []string{"openid", "files:read", "files:write"}},
		{"scp array", map[string]any{"scp": []string{"a", "b"}}, []string{"a", "b"}},
		{"both unioned without duplicates", map[string]any{"scope": "a b", "scp": []string{"b", "c"}}, []string{"a", "b", "c"}},
		{"none", map[string]any{}, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := newVerifierEnv(t)
			c := e.claims()
			for k, v := range tc.claims {
				c[k] = v
			}
			token := signToken(t, c, signOpts{Kid: rsaKID, Key: e.rsa})
			info, err := e.v.Verify(context.Background(), token, httptest.NewRequest(http.MethodGet, "/mcp", nil))
			if err != nil {
				t.Fatalf("Verify: %v", err)
			}
			if !reflect.DeepEqual(info.Scopes, tc.want) {
				t.Fatalf("scopes = %v, want %v", info.Scopes, tc.want)
			}
		})
	}
}

func TestNewVerifierStartupFailure(t *testing.T) {
	t.Run("unreachable issuer", func(t *testing.T) {
		idp := newTestIdP(t, idpOptions{path: "/realms/test"})
		client := idp.Client(t)
		issuer := idp.Issuer()
		idp.srv.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		v, err := NewVerifier(ctx, VerifierOptions{Issuer: issuer, Resource: verifierResource, HTTPClient: client})
		if err == nil || v != nil {
			t.Fatalf("expected startup error, got verifier=%v err=%v", v, err)
		}
	})
	t.Run("issuer mismatch", func(t *testing.T) {
		idp := newTestIdP(t, idpOptions{path: "/realms/test"})
		idp.Set(func(c *idpConfig) { c.issuerOverride = "https://evil.example.test/realms/test" })
		v, err := NewVerifier(context.Background(), VerifierOptions{Issuer: idp.Issuer(), Resource: verifierResource, HTTPClient: idp.Client(t)})
		if err == nil || v != nil {
			t.Fatalf("expected startup error, got verifier=%v err=%v", v, err)
		}
	})
	t.Run("missing resource", func(t *testing.T) {
		idp := newTestIdP(t, idpOptions{path: "/realms/test"})
		v, err := NewVerifier(context.Background(), VerifierOptions{Issuer: idp.Issuer(), HTTPClient: idp.Client(t)})
		if err == nil || v != nil {
			t.Fatalf("expected startup error, got verifier=%v err=%v", v, err)
		}
	})
}

func TestRequireBearerTokenInvalidTokenIs401(t *testing.T) {
	e := newVerifierEnv(t)
	// The SDK middleware checks expiry against the wall clock, so move the
	// fake clock to the present.
	e.clock.Advance(time.Since(e.clock.Now()))
	reached := false
	h := sdkauth.RequireBearerToken(e.v.Verify, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
	}))
	c := e.claims()
	c["aud"] = "web"
	bad := signToken(t, c, signOpts{Kid: rsaKID, Key: e.rsa})
	good := signToken(t, e.claims(), signOpts{Kid: rsaKID, Key: e.rsa})

	for name, tc := range map[string]struct {
		token string
		code  int
	}{"wrong audience": {bad, http.StatusUnauthorized}, "garbage": {"not.a.jwt", http.StatusUnauthorized}, "valid": {good, http.StatusOK}} {
		t.Run(name, func(t *testing.T) {
			reached = false
			req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
			req.Header.Set("Authorization", "Bearer "+tc.token)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tc.code {
				t.Fatalf("status = %d, want %d", rec.Code, tc.code)
			}
			if reached != (tc.code == http.StatusOK) {
				t.Fatalf("handler reached = %v", reached)
			}
		})
	}
}
