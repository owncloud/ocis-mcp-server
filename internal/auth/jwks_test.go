package auth

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

// newWarmKeySet builds an IdP with one RSA key "a", a key set on a fake clock,
// and performs the first fetch.
func newWarmKeySet(t *testing.T) (*testIdP, *KeySet, *fakeClock) {
	t.Helper()
	idp := newTestIdP(t, idpOptions{})
	idp.AddRSAKey(t, "a")
	clock := newFakeClock()
	ks := NewKeySet(idp.Client(t), idp.JWKSURL(), clock.Now)
	if _, err := ks.Key(context.Background(), "a"); err != nil {
		t.Fatalf("warm up: %v", err)
	}
	if got := idp.jwksHits.Load(); got != 1 {
		t.Fatalf("warm up fetched %d times, want 1", got)
	}
	return idp, ks, clock
}

func TestKeySetKnownKid(t *testing.T) {
	idp := newTestIdP(t, idpOptions{})
	rk := idp.AddRSAKey(t, "rsa1")
	ek := idp.AddECKey(t, "ec1")
	ks := NewKeySet(idp.Client(t), idp.JWKSURL(), newFakeClock().Now)

	got, err := ks.Key(context.Background(), "rsa1")
	if err != nil {
		t.Fatalf("rsa key: %v", err)
	}
	if pub, ok := got.(*rsa.PublicKey); !ok || !pub.Equal(&rk.PublicKey) {
		t.Fatalf("rsa key mismatch, got %T", got)
	}
	got, err = ks.Key(context.Background(), "ec1")
	if err != nil {
		t.Fatalf("ec key: %v", err)
	}
	if pub, ok := got.(*ecdsa.PublicKey); !ok || !pub.Equal(&ek.PublicKey) {
		t.Fatalf("ec key mismatch, got %T", got)
	}
	if hits := idp.jwksHits.Load(); hits != 1 {
		t.Fatalf("two known kids cost %d fetches, want 1", hits)
	}
}

func TestKeySetRotationAndUnknownKidThrottle(t *testing.T) {
	idp, ks, clock := newWarmKeySet(t)
	ctx := context.Background()

	idp.AddRSAKey(t, "b")
	if _, err := ks.Key(ctx, "b"); err != nil {
		t.Fatalf("rotated kid: %v", err)
	}
	if got := idp.jwksHits.Load(); got != 2 {
		t.Fatalf("rotation fetched %d times in total, want 2", got)
	}

	// The rotation used up the refetch budget of this minute.
	clock.Advance(61 * time.Second)
	if _, err := ks.Key(ctx, "nope1"); !errors.Is(err, ErrKeyNotFound) {
		t.Fatalf("unknown kid error = %v, want ErrKeyNotFound", err)
	}
	if got := idp.jwksHits.Load(); got != 3 {
		t.Fatalf("first unknown kid fetched %d times in total, want 3", got)
	}

	clock.Advance(30 * time.Second)
	if _, err := ks.Key(ctx, "nope2"); !errors.Is(err, ErrKeyNotFound) {
		t.Fatalf("second unknown kid error = %v, want ErrKeyNotFound", err)
	}
	if got := idp.jwksHits.Load(); got != 3 {
		t.Fatalf("second unknown kid inside 60s refetched (%d), want no refetch", got)
	}

	// A key rotated in right now stays unreachable until the throttle expires.
	idp.AddRSAKey(t, "c")
	if _, err := ks.Key(ctx, "c"); !errors.Is(err, ErrKeyNotFound) {
		t.Fatalf("throttled rotated kid error = %v, want ErrKeyNotFound", err)
	}
	clock.Advance(31 * time.Second)
	if _, err := ks.Key(ctx, "c"); err != nil {
		t.Fatalf("rotated kid after the throttle: %v", err)
	}
	if got := idp.jwksHits.Load(); got != 4 {
		t.Fatalf("fetched %d times in total, want 4", got)
	}
}

func TestKeySetSingleflight(t *testing.T) {
	const callers = 100
	tests := []struct {
		name    string
		kid     string
		rotate  bool
		wantErr error
	}{
		{name: "100 callers for a rotated kid", kid: "new", rotate: true},
		{name: "100 callers for an unknown kid", kid: "ghost", wantErr: ErrKeyNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idp, ks, _ := newWarmKeySet(t)
			if tt.rotate {
				idp.AddRSAKey(t, tt.kid)
			}
			idp.Set(func(c *idpConfig) { c.jwksDelay = 150 * time.Millisecond })

			start := make(chan struct{})
			errs := make([]error, callers)
			var wg sync.WaitGroup
			for i := 0; i < callers; i++ {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					<-start
					_, errs[i] = ks.Key(context.Background(), tt.kid)
				}(i)
			}
			close(start)
			wg.Wait()

			for i, err := range errs {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("caller %d error = %v, want %v", i, err, tt.wantErr)
				}
			}
			if got := idp.jwksHits.Load(); got != 2 {
				t.Fatalf("%d callers caused %d fetches in total, want 2 (warm up plus one)", callers, got)
			}
		})
	}
}

func TestKeySetHourlyRefresh(t *testing.T) {
	idp, ks, clock := newWarmKeySet(t)
	ctx := context.Background()

	clock.Advance(59 * time.Minute)
	if _, err := ks.Key(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	if got := idp.jwksHits.Load(); got != 1 {
		t.Fatalf("fetched %d times before the hour is up, want 1", got)
	}

	idp.RemoveKey("a")
	idp.AddECKey(t, "e")
	clock.Advance(2 * time.Minute)
	if _, err := ks.Key(ctx, "e"); err != nil {
		t.Fatalf("new key after the hourly refresh: %v", err)
	}
	if got := idp.jwksHits.Load(); got != 2 {
		t.Fatalf("fetched %d times after the hour, want 2", got)
	}
	// The refresh replaces the set: a withdrawn key is gone.
	if _, err := ks.Key(ctx, "a"); !errors.Is(err, ErrKeyNotFound) {
		t.Fatalf("withdrawn key error = %v, want ErrKeyNotFound", err)
	}
}

func TestKeySetServesStaleThenFails(t *testing.T) {
	for _, mode := range []string{"5xx", "huge", "redirect-http"} {
		t.Run(mode, func(t *testing.T) {
			idp, ks, clock := newWarmKeySet(t)
			ctx := context.Background()
			idp.Set(func(c *idpConfig) {
				c.jwksMode = mode
				if mode == "redirect-http" {
					// An answering loopback server: a followed redirect would refresh the set.
					c.redirectTo = newHTTPLoopbackTarget(t, "", "", "a").JWKSURL()
				}
			})

			clock.Advance(2 * time.Hour)
			if _, err := ks.Key(ctx, "a"); err != nil {
				t.Fatalf("known kid with stale keys after a failed refresh: %v", err)
			}
			if got := idp.jwksHits.Load(); got != 2 {
				t.Fatalf("fetched %d times, want 2 (warm up plus the failed refresh)", got)
			}
			// No hammering of a failing endpoint inside 60 s.
			if _, err := ks.Key(ctx, "a"); err != nil {
				t.Fatal(err)
			}
			if got := idp.jwksHits.Load(); got != 2 {
				t.Fatalf("refetched inside 60s after a failure (%d)", got)
			}

			// Unknown kid with the endpoint down fails closed.
			if _, err := ks.Key(ctx, "other"); !errors.Is(err, ErrKeyNotFound) {
				t.Fatalf("unknown kid with failing endpoint = %v, want ErrKeyNotFound", err)
			}

			clock.Advance(21 * time.Hour) // 23h since the last success
			if _, err := ks.Key(ctx, "a"); err != nil {
				t.Fatalf("known kid at 23h: %v", err)
			}
			clock.Advance(59 * time.Minute) // 23h59m
			if _, err := ks.Key(ctx, "a"); err != nil {
				t.Fatalf("known kid just under 24h: %v", err)
			}
			clock.Advance(2 * time.Minute) // 24h01m
			_, err := ks.Key(ctx, "a")
			if !errors.Is(err, ErrKeysUnavailable) {
				t.Fatalf("known kid after 24h = %v, want ErrKeysUnavailable", err)
			}
			if strings.Contains(err.Error(), "secret-body-must-not-leak") || strings.Contains(err.Error(), "aaaa") {
				t.Fatalf("error leaks a response body: %q", err)
			}
		})
	}
}

func TestKeySetRecoversAfterFailure(t *testing.T) {
	idp, ks, clock := newWarmKeySet(t)
	ctx := context.Background()
	idp.Set(func(c *idpConfig) { c.jwksMode = "5xx" })
	clock.Advance(2 * time.Hour)
	if _, err := ks.Key(ctx, "a"); err != nil {
		t.Fatal(err)
	}

	idp.Set(func(c *idpConfig) { c.jwksMode = "" })
	idp.AddRSAKey(t, "fresh")
	clock.Advance(61 * time.Second)
	if _, err := ks.Key(ctx, "fresh"); err != nil {
		t.Fatalf("kid after the endpoint recovered: %v", err)
	}
	// Success resets the 24 h window.
	clock.Advance(23 * time.Hour)
	if _, err := ks.Key(ctx, "a"); err != nil {
		t.Fatalf("known kid after recovery: %v", err)
	}
}

func TestKeySetColdStartFailure(t *testing.T) {
	for _, mode := range []string{"5xx", "huge", "redirect-http"} {
		t.Run(mode, func(t *testing.T) {
			idp := newTestIdP(t, idpOptions{})
			idp.AddRSAKey(t, "a")
			idp.Set(func(c *idpConfig) {
				c.jwksMode = mode
				if mode == "redirect-http" {
					c.redirectTo = newHTTPLoopbackTarget(t, "", "", "a").JWKSURL()
				}
			})
			ks := NewKeySet(idp.Client(t), idp.JWKSURL(), newFakeClock().Now)
			_, err := ks.Key(context.Background(), "a")
			if !errors.Is(err, ErrKeysUnavailable) {
				t.Fatalf("error = %v, want ErrKeysUnavailable", err)
			}
			if strings.Contains(err.Error(), "secret-body-must-not-leak") || strings.Contains(err.Error(), "aaaa") {
				t.Fatalf("error leaks a response body: %q", err)
			}
		})
	}
}

func TestKeySetHangingEndpoint(t *testing.T) {
	short := func(idp *testIdP, t *testing.T) *http.Client {
		c := idp.Client(t)
		c.Timeout = 300 * time.Millisecond
		return c
	}
	t.Run("cold start returns an error in bounded time", func(t *testing.T) {
		idp := newTestIdP(t, idpOptions{})
		idp.AddRSAKey(t, "a")
		idp.Set(func(c *idpConfig) { c.jwksMode = "hang" })
		ks := NewKeySet(short(idp, t), idp.JWKSURL(), newFakeClock().Now)
		began := time.Now()
		_, err := ks.Key(context.Background(), "a")
		if !errors.Is(err, ErrKeysUnavailable) {
			t.Fatalf("error = %v, want ErrKeysUnavailable", err)
		}
		if d := time.Since(began); d > 5*time.Second {
			t.Fatalf("returned after %v, want well under 10s", d)
		}
	})
	t.Run("stale keys served and unknown kid refused in bounded time", func(t *testing.T) {
		idp := newTestIdP(t, idpOptions{})
		idp.AddRSAKey(t, "a")
		clock := newFakeClock()
		ks := NewKeySet(short(idp, t), idp.JWKSURL(), clock.Now)
		if _, err := ks.Key(context.Background(), "a"); err != nil {
			t.Fatal(err)
		}
		idp.Set(func(c *idpConfig) { c.jwksMode = "hang" })
		clock.Advance(2 * time.Hour)
		began := time.Now()
		if _, err := ks.Key(context.Background(), "a"); err != nil {
			t.Fatalf("known kid while the endpoint hangs: %v", err)
		}
		clock.Advance(2 * time.Minute)
		if _, err := ks.Key(context.Background(), "ghost"); !errors.Is(err, ErrKeyNotFound) {
			t.Fatalf("unknown kid while the endpoint hangs = %v, want ErrKeyNotFound", err)
		}
		if d := time.Since(began); d > 5*time.Second {
			t.Fatalf("returned after %v, want well under 10s", d)
		}
	})
	t.Run("caller context cancellation returns promptly", func(t *testing.T) {
		idp := newTestIdP(t, idpOptions{})
		idp.AddRSAKey(t, "a")
		idp.Set(func(c *idpConfig) { c.jwksMode = "hang" })
		ks := NewKeySet(idp.Client(t), idp.JWKSURL(), newFakeClock().Now)
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()
		began := time.Now()
		_, err := ks.Key(ctx, "a")
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("error = %v, want context.DeadlineExceeded", err)
		}
		if d := time.Since(began); d > 5*time.Second {
			t.Fatalf("returned after %v", d)
		}
	})
}

func TestKeySetRejectsInsecureURL(t *testing.T) {
	idp := newTestIdP(t, idpOptions{})
	// The fetch must be refused by the scheme rule before any connection is
	// attempted, so the failing host is unreachable on purpose.
	ks := NewKeySet(idp.Client(t), "http://idp.example.invalid/jwks", newFakeClock().Now)
	_, err := ks.Key(context.Background(), "a")
	if !errors.Is(err, ErrKeysUnavailable) {
		t.Fatalf("error = %v, want ErrKeysUnavailable", err)
	}
	if !strings.Contains(err.Error(), "jwks_uri must use https") {
		t.Fatalf("error %q does not name the https rule", err)
	}
	if strings.Contains(err.Error(), "lookup") || strings.Contains(err.Error(), "dial") {
		t.Fatalf("error %q shows a connection was attempted", err)
	}
}

func TestKeySetEmptyKid(t *testing.T) {
	idp, ks, _ := newWarmKeySet(t)
	if _, err := ks.Key(context.Background(), ""); !errors.Is(err, ErrKeyNotFound) {
		t.Fatalf("empty kid error = %v, want ErrKeyNotFound", err)
	}
	if got := idp.jwksHits.Load(); got != 1 {
		t.Fatalf("empty kid triggered a fetch (%d)", got)
	}
}

func TestKeySetIgnoresUnusableKeys(t *testing.T) {
	idp := newTestIdP(t, idpOptions{})
	good := idp.AddRSAKey(t, "good")
	other := idp.AddRSAKey(t, "x")
	small, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatal(err)
	}
	p384, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	idp.RemoveKey("x")

	// "use" absent is accepted.
	sigless := rsaJWK("sigless", &good.PublicKey)
	delete(sigless, "use")
	enc := rsaJWK("enc", &good.PublicKey)
	enc["use"] = "enc"
	noKid := rsaJWK("", &good.PublicKey)
	delete(noKid, "kid")
	offCurve := map[string]any{"kty": "EC", "crv": "P-256", "kid": "offcurve", "use": "sig", "x": b64([]byte{1}), "y": b64([]byte{1})}
	p384JWK := map[string]any{"kty": "EC", "crv": "P-384", "kid": "p384", "use": "sig",
		"x": b64(p384.X.Bytes()), "y": b64(p384.Y.Bytes())}
	for _, j := range []map[string]any{
		{"kty": "oct", "kid": "oct", "k": "c2VjcmV0"},
		enc,
		noKid,
		p384JWK,
		offCurve,
		rsaJWK("small", &small.PublicKey),
		{"kty": "RSA", "kid": "garbage", "n": "!!!", "e": "AQAB"},
		// Two different keys under one kid are ambiguous and not addressable.
		rsaJWK("dup", &good.PublicKey),
		rsaJWK("dup", &other.PublicKey),
		sigless,
	} {
		idp.AddRawJWK(j)
	}

	clock := newFakeClock()
	ks := NewKeySet(idp.Client(t), idp.JWKSURL(), clock.Now)
	ctx := context.Background()
	if _, err := ks.Key(ctx, "good"); err != nil {
		t.Fatalf("good key: %v", err)
	}
	if _, err := ks.Key(ctx, "sigless"); err != nil {
		t.Fatalf("key without use: %v", err)
	}
	for _, kid := range []string{"oct", "enc", "p384", "offcurve", "small", "garbage", "dup"} {
		clock.Advance(61 * time.Second)
		if _, err := ks.Key(ctx, kid); !errors.Is(err, ErrKeyNotFound) {
			t.Errorf("kid %q error = %v, want ErrKeyNotFound", kid, err)
		}
	}
}

func TestKeySetEmptyKeySetIsAFailure(t *testing.T) {
	idp, ks, clock := newWarmKeySet(t)
	idp.RemoveKey("a")
	clock.Advance(2 * time.Hour)
	// The endpoint now publishes no usable key: keep the last known good one.
	if _, err := ks.Key(context.Background(), "a"); err != nil {
		t.Fatalf("known kid after an empty JWKS: %v", err)
	}
}

func TestSignTokenVerifiesAgainstKeySet(t *testing.T) {
	idp := newTestIdP(t, idpOptions{})
	rk := idp.AddRSAKey(t, "r")
	ek := idp.AddECKey(t, "e")
	ks := NewKeySet(idp.Client(t), idp.JWKSURL(), newFakeClock().Now)
	claims := map[string]any{"iss": idp.Issuer(), "sub": "u1"}

	tests := []struct {
		name string
		kid  string
		opts signOpts
	}{
		{"RS256", "r", signOpts{Alg: "RS256", Kid: "r", Key: rk}},
		{"PS256", "r", signOpts{Alg: "PS256", Kid: "r", Key: rk}},
		{"ES256", "e", signOpts{Alg: "ES256", Kid: "e", Key: ek}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tok := signToken(t, claims, tt.opts)
			parts := strings.Split(tok, ".")
			if len(parts) != 3 {
				t.Fatalf("token has %d parts", len(parts))
			}
			sig, err := base64.RawURLEncoding.DecodeString(parts[2])
			if err != nil {
				t.Fatal(err)
			}
			digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
			key, err := ks.Key(context.Background(), tt.kid)
			if err != nil {
				t.Fatal(err)
			}
			switch k := key.(type) {
			case *rsa.PublicKey:
				if tt.opts.Alg == "PS256" {
					err = rsa.VerifyPSS(k, crypto.SHA256, digest[:], sig, &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash})
				} else {
					err = rsa.VerifyPKCS1v15(k, crypto.SHA256, digest[:], sig)
				}
				if err != nil {
					t.Fatalf("signature does not verify: %v", err)
				}
			case *ecdsa.PublicKey:
				if len(sig) != 64 || !ecdsa.Verify(k, digest[:], new(big.Int).SetBytes(sig[:32]), new(big.Int).SetBytes(sig[32:])) {
					t.Fatal("EC signature does not verify")
				}
			default:
				t.Fatalf("unexpected key type %T", key)
			}
		})
	}
}
