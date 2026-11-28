package auth

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeClock is an injectable clock for age based cache tests.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{t: time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// idpConfig holds the failure switches of the fake IdP.
type idpConfig struct {
	oidcStatus      int    // non-zero: OIDC well-known answers with this status
	rfcStatus       int    // non-zero: RFC 8414 well-known answers with this status
	issuerOverride  string // issuer in the discovery document
	jwksURIOverride string // jwks_uri in the discovery document
	discoveryMode   string // "", "huge", "redirect-http"
	jwksMode        string // "", "5xx", "hang", "huge", "redirect-http"
	jwksDelay       time.Duration
	redirectTo      string // target of the "redirect-http" modes; defaults to a non-loopback http URL
}

// idpOptions selects how the fake IdP is served.
type idpOptions struct {
	plainHTTP bool   // serve over http instead of https
	path      string // issuer path, for example "/realms/test"
}

// testIdP is an in-memory identity provider: discovery documents, a JWKS
// endpoint, key rotation, failure switches and request counters.
type testIdP struct {
	srv  *httptest.Server
	path string

	mu       sync.Mutex
	rsaKeys  map[string]*rsa.PrivateKey
	ecKeys   map[string]*ecdsa.PrivateKey
	extraJWK []map[string]any
	cfg      idpConfig

	oidcHits atomic.Int64
	rfcHits  atomic.Int64
	jwksHits atomic.Int64

	release chan struct{}
}

func newTestIdP(t *testing.T, o idpOptions) *testIdP {
	t.Helper()
	idp := &testIdP{
		path:    o.path,
		rsaKeys: map[string]*rsa.PrivateKey{},
		ecKeys:  map[string]*ecdsa.PrivateKey{},
		release: make(chan struct{}),
	}
	mux := http.NewServeMux()
	mux.HandleFunc(o.path+"/.well-known/openid-configuration", idp.serveDiscovery(&idp.oidcHits, func(c idpConfig) int { return c.oidcStatus }))
	mux.HandleFunc("/.well-known/oauth-authorization-server"+o.path, idp.serveDiscovery(&idp.rfcHits, func(c idpConfig) int { return c.rfcStatus }))
	mux.HandleFunc(o.path+"/jwks", idp.serveJWKS)
	idp.srv = httptest.NewUnstartedServer(mux)
	idp.srv.Config.ErrorLog = log.New(io.Discard, "", 0)
	if o.plainHTTP {
		idp.srv.Start()
	} else {
		idp.srv.StartTLS()
	}
	t.Cleanup(func() {
		close(idp.release)
		idp.srv.Close()
	})
	return idp
}

func (i *testIdP) config() idpConfig {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.cfg
}

// Set mutates the failure switches.
func (i *testIdP) Set(f func(*idpConfig)) {
	i.mu.Lock()
	defer i.mu.Unlock()
	f(&i.cfg)
}

// Issuer is the issuer URL the fake IdP announces by default.
func (i *testIdP) Issuer() string { return i.srv.URL + i.path }

// JWKSURL is the JWKS endpoint of the fake IdP.
func (i *testIdP) JWKSURL() string { return i.srv.URL + i.path + "/jwks" }

// Client returns a client from NewIssuerHTTPClient that trusts the fake IdP.
func (i *testIdP) Client(t *testing.T) *http.Client {
	t.Helper()
	caFile := ""
	if i.srv.TLS != nil {
		caFile = i.CAFile(t)
	}
	c, err := NewIssuerHTTPClient(caFile)
	if err != nil {
		t.Fatalf("NewIssuerHTTPClient: %v", err)
	}
	return c
}

// CAFile writes the server certificate as a PEM CA bundle and returns its path.
func (i *testIdP) CAFile(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "ca.pem")
	block := &pem.Block{Type: "CERTIFICATE", Bytes: i.srv.Certificate().Raw}
	if err := os.WriteFile(p, pem.EncodeToMemory(block), 0o600); err != nil {
		t.Fatalf("write CA file: %v", err)
	}
	return p
}

// AddRSAKey generates and publishes an RSA key under kid.
func (i *testIdP) AddRSAKey(t *testing.T, kid string) *rsa.PrivateKey {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	i.rsaKeys[kid] = k
	return k
}

// AddECKey generates and publishes a P-256 key under kid.
func (i *testIdP) AddECKey(t *testing.T, kid string) *ecdsa.PrivateKey {
	t.Helper()
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate EC key: %v", err)
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	i.ecKeys[kid] = k
	return k
}

// RemoveKey withdraws the key published under kid.
func (i *testIdP) RemoveKey(kid string) {
	i.mu.Lock()
	defer i.mu.Unlock()
	delete(i.rsaKeys, kid)
	delete(i.ecKeys, kid)
}

// AddRawJWK publishes an arbitrary JWK entry.
func (i *testIdP) AddRawJWK(jwk map[string]any) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.extraJWK = append(i.extraJWK, jwk)
}

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func rsaJWK(kid string, pub *rsa.PublicKey) map[string]any {
	e := []byte{byte(pub.E >> 24), byte(pub.E >> 16), byte(pub.E >> 8), byte(pub.E)}
	for len(e) > 1 && e[0] == 0 {
		e = e[1:]
	}
	return map[string]any{"kty": "RSA", "kid": kid, "use": "sig", "alg": "RS256", "n": b64(pub.N.Bytes()), "e": b64(e)}
}

func ecJWK(kid string, pub *ecdsa.PublicKey) map[string]any {
	point, err := pub.Bytes() // 0x04 || X || Y, 32 bytes each
	if err != nil {
		panic(err)
	}
	return map[string]any{"kty": "EC", "crv": "P-256", "kid": kid, "use": "sig", "alg": "ES256", "x": b64(point[1:33]), "y": b64(point[33:])}
}

func (i *testIdP) jwksDocument() []byte {
	i.mu.Lock()
	defer i.mu.Unlock()
	keys := []map[string]any{}
	for kid, k := range i.rsaKeys {
		keys = append(keys, rsaJWK(kid, &k.PublicKey))
	}
	for kid, k := range i.ecKeys {
		keys = append(keys, ecJWK(kid, &k.PublicKey))
	}
	keys = append(keys, i.extraJWK...)
	doc, _ := json.Marshal(map[string]any{"keys": keys})
	return doc
}

func (i *testIdP) serveDiscovery(hits *atomic.Int64, status func(idpConfig) int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		c := i.config()
		if s := status(c); s != 0 {
			http.Error(w, "secret-body-must-not-leak", s)
			return
		}
		switch c.discoveryMode {
		case "huge":
			writeHuge(w, `{"issuer":"`+i.Issuer()+`","jwks_uri":"`+i.JWKSURL()+`","pad":"`)
			return
		case "redirect-http":
			http.Redirect(w, r, orDefault(c.redirectTo, "http://idp.example.test/.well-known/openid-configuration"), http.StatusFound)
			return
		}
		issuer, jwksURI := i.Issuer(), i.JWKSURL()
		if c.issuerOverride != "" {
			issuer = c.issuerOverride
		}
		if c.jwksURIOverride != "" {
			jwksURI = c.jwksURIOverride
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"issuer": issuer, "jwks_uri": jwksURI})
	}
}

func (i *testIdP) serveJWKS(w http.ResponseWriter, r *http.Request) {
	i.jwksHits.Add(1)
	c := i.config()
	if c.jwksDelay > 0 {
		time.Sleep(c.jwksDelay)
	}
	switch c.jwksMode {
	case "5xx":
		http.Error(w, "secret-body-must-not-leak", http.StatusBadGateway)
		return
	case "hang":
		select {
		case <-r.Context().Done():
		case <-i.release:
		}
		return
	case "huge":
		writeHuge(w, `{"keys":[],"pad":"`)
		return
	case "redirect-http":
		http.Redirect(w, r, orDefault(c.redirectTo, "http://idp.example.test/jwks"), http.StatusFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(i.jwksDocument())
}

func orDefault(v, def string) string {
	if v != "" {
		return v
	}
	return def
}

// newHTTPLoopbackTarget starts a plain http loopback IdP that really answers
// discovery and JWKS requests, so a redirect that is wrongly followed to it
// would succeed. Its discovery document advertises issuer and jwksURI, and it
// publishes an RSA key for each kid.
func newHTTPLoopbackTarget(t *testing.T, issuer, jwksURI string, kids ...string) *testIdP {
	t.Helper()
	target := newTestIdP(t, idpOptions{plainHTTP: true})
	for _, kid := range kids {
		target.AddRSAKey(t, kid)
	}
	target.Set(func(c *idpConfig) {
		c.issuerOverride = issuer
		c.jwksURIOverride = jwksURI
	})
	return target
}

// writeHuge writes a syntactically valid JSON document of about 2 MiB.
func writeHuge(w http.ResponseWriter, prefix string) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(prefix))
	_, _ = w.Write([]byte(strings.Repeat("a", 2<<20)))
	_, _ = w.Write([]byte(`"}`))
}

// signOpts controls how signToken builds a JWT.
type signOpts struct {
	Alg        string        // default RS256; also PS256, ES256, none, HS256
	Kid        string        // header kid
	OmitKid    bool          // leave kid out of the header
	Key        crypto.Signer // *rsa.PrivateKey or *ecdsa.PrivateKey
	HMACSecret []byte        // for HS256
	Header     map[string]any
}

// signToken builds a compact JWT with the standard library only. It never
// prints the token and its failures never include token material.
func signToken(t *testing.T, claims map[string]any, o signOpts) string {
	t.Helper()
	alg := o.Alg
	if alg == "" {
		alg = "RS256"
	}
	header := map[string]any{"alg": alg, "typ": "JWT"}
	if !o.OmitKid {
		header["kid"] = o.Kid
	}
	for k, v := range o.Header {
		header[k] = v
	}
	hb, err := json.Marshal(header)
	if err != nil {
		t.Fatalf("marshal JWT header: %v", err)
	}
	cb, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal JWT claims: %v", err)
	}
	signingInput := b64(hb) + "." + b64(cb)
	digest := sha256.Sum256([]byte(signingInput))

	var sig []byte
	switch alg {
	case "none":
	case "HS256":
		m := hmac.New(sha256.New, o.HMACSecret)
		m.Write([]byte(signingInput))
		sig = m.Sum(nil)
	case "RS256":
		k, ok := o.Key.(*rsa.PrivateKey)
		if !ok {
			t.Fatalf("signToken: RS256 needs an RSA key")
		}
		sig, err = rsa.SignPKCS1v15(rand.Reader, k, crypto.SHA256, digest[:])
	case "PS256":
		k, ok := o.Key.(*rsa.PrivateKey)
		if !ok {
			t.Fatalf("signToken: PS256 needs an RSA key")
		}
		sig, err = rsa.SignPSS(rand.Reader, k, crypto.SHA256, digest[:], &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash})
	case "ES256":
		k, ok := o.Key.(*ecdsa.PrivateKey)
		if !ok {
			t.Fatalf("signToken: ES256 needs an EC key")
		}
		r, s, e := ecdsa.Sign(rand.Reader, k, digest[:])
		err = e
		if err == nil {
			sig = append(r.FillBytes(make([]byte, 32)), s.FillBytes(make([]byte, 32))...)
		}
	default:
		t.Fatalf("signToken: unsupported alg %q", alg)
	}
	if err != nil {
		t.Fatalf("signToken: signing failed")
	}
	return signingInput + "." + b64(sig)
}
