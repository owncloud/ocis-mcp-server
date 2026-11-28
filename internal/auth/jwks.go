package auth

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

const (
	// jwksRefreshInterval is the age after which the key set is refetched.
	jwksRefreshInterval = time.Hour
	// jwksMinRefetch spaces unknown-kid refetches, and retries after a failure.
	jwksMinRefetch = 60 * time.Second
	// jwksStaleLimit is how long the last good key set may be served after
	// its last successful fetch.
	jwksStaleLimit = 24 * time.Hour
	// jwksFetchTimeout bounds one JWKS fetch.
	jwksFetchTimeout = 10 * time.Second
	// minRSAModulusBits is the smallest RSA key accepted.
	minRSAModulusBits = 2048
)

var (
	// ErrKeyNotFound means no usable signing key has the requested key ID.
	ErrKeyNotFound = errors.New("jwks: no signing key for the key id")
	// ErrKeysUnavailable means no key set is available: it was never fetched,
	// or the last successful fetch is older than 24 hours.
	ErrKeysUnavailable = errors.New("jwks: no usable key set available")
)

// KeySet caches the signing keys published at a JWKS URI.
type KeySet struct {
	client *http.Client
	uri    string
	now    func() time.Time
	sf     singleflight.Group

	mu          sync.Mutex
	keys        map[string]crypto.PublicKey
	lastSuccess time.Time // zero until the first successful fetch
	lastFailure time.Time // zero while the last fetch succeeded
	lastMiss    time.Time // last fetch triggered by an unknown kid
	lastErr     error
}

// NewKeySet returns a key set that fetches jwksURI with c. now is the clock
// used for every age decision; nil means time.Now.
func NewKeySet(c *http.Client, jwksURI string, now func() time.Time) *KeySet {
	if now == nil {
		now = time.Now
	}
	return &KeySet{client: c, uri: jwksURI, now: now}
}

// Key returns the public key for kid. The key set is refetched when older than
// an hour and, at most once per 60 s, when kid is unknown. When a fetch fails
// the last good set keeps serving known key IDs for up to 24 hours after its
// last success; an unknown kid then fails closed with ErrKeyNotFound, and any
// kid fails with ErrKeysUnavailable once the 24 hours are over.
func (k *KeySet) Key(ctx context.Context, kid string) (crypto.PublicKey, error) {
	if kid == "" {
		return nil, ErrKeyNotFound
	}
	k.mu.Lock()
	now := k.now()
	if key, ok := k.keys[kid]; ok && k.freshLocked(now) {
		k.mu.Unlock()
		return key, nil
	}
	fetch := k.shouldFetchLocked(kid, now)
	k.mu.Unlock()

	if fetch {
		done := k.sf.DoChan("jwks", func() (any, error) {
			k.refresh(kid)
			return nil, nil
		})
		select {
		case <-done:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return k.lookup(kid)
}

func (k *KeySet) freshLocked(now time.Time) bool {
	return !k.lastSuccess.IsZero() && now.Sub(k.lastSuccess) < jwksRefreshInterval
}

// shouldFetchLocked decides whether a fetch is due for kid. It is evaluated
// again inside the singleflight function, so a caller that arrives just after
// a fetch finished does not start another.
func (k *KeySet) shouldFetchLocked(kid string, now time.Time) bool {
	if _, ok := k.keys[kid]; ok && k.freshLocked(now) {
		return false
	}
	if !k.freshLocked(now) {
		// Cold start or hourly refresh due; do not hammer an endpoint that just failed.
		return k.lastFailure.IsZero() || now.Sub(k.lastFailure) >= jwksMinRefetch
	}
	// Fresh set, unknown kid.
	return k.lastMiss.IsZero() || now.Sub(k.lastMiss) >= jwksMinRefetch
}

// refresh fetches the key set once. It runs inside singleflight and uses its
// own 10 s deadline, independent of any single caller's context.
func (k *KeySet) refresh(kid string) {
	k.mu.Lock()
	now := k.now()
	if !k.shouldFetchLocked(kid, now) {
		k.mu.Unlock()
		return
	}
	// The throttle stamp is taken when the fetch ends, not when it starts, so
	// callers arriving during the fetch still join it instead of failing.
	miss := k.freshLocked(now)
	k.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), jwksFetchTimeout)
	defer cancel()
	keys, err := k.fetch(ctx)

	k.mu.Lock()
	defer k.mu.Unlock()
	done := k.now()
	if miss {
		k.lastMiss = done
	}
	if err != nil {
		k.lastFailure = done
		k.lastErr = err
		return
	}
	k.keys = keys
	k.lastSuccess = done
	k.lastFailure = time.Time{}
	k.lastErr = nil
}

// lookup answers from the cache, enforcing the 24 h staleness limit.
func (k *KeySet) lookup(kid string) (crypto.PublicKey, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.lastSuccess.IsZero() || k.now().Sub(k.lastSuccess) > jwksStaleLimit {
		if k.lastErr != nil {
			return nil, fmt.Errorf("%w: last fetch failed: %v", ErrKeysUnavailable, k.lastErr)
		}
		return nil, ErrKeysUnavailable
	}
	if key, ok := k.keys[kid]; ok {
		return key, nil
	}
	return nil, ErrKeyNotFound
}

type jwk struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Crv string `json:"crv"`
	N   string `json:"n"`
	E   string `json:"e"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

func (k *KeySet) fetch(ctx context.Context) (map[string]crypto.PublicKey, error) {
	if _, err := parseSecureURL(k.uri, "jwks_uri"); err != nil {
		return nil, err
	}
	var doc struct {
		Keys []json.RawMessage `json:"keys"`
	}
	if err := getJSON(ctx, k.client, k.uri, &doc); err != nil {
		return nil, err
	}
	keys := make(map[string]crypto.PublicKey, len(doc.Keys))
	ambiguous := map[string]bool{}
	for _, raw := range doc.Keys {
		var j jwk
		if err := json.Unmarshal(raw, &j); err != nil || j.Kid == "" {
			continue
		}
		if j.Use != "" && j.Use != "sig" {
			continue
		}
		pub, err := parseJWK(j)
		if err != nil {
			continue
		}
		if _, dup := keys[j.Kid]; dup {
			ambiguous[j.Kid] = true
			continue
		}
		keys[j.Kid] = pub
	}
	for kid := range ambiguous {
		delete(keys, kid)
	}
	if len(keys) == 0 {
		return nil, errors.New("key set contains no usable signing key")
	}
	return keys, nil
}

func decodeB64(s string) ([]byte, error) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil || len(b) == 0 {
		return nil, errors.New("invalid base64url value")
	}
	return b, nil
}

func parseJWK(j jwk) (crypto.PublicKey, error) {
	switch j.Kty {
	case "RSA":
		nb, err := decodeB64(j.N)
		if err != nil {
			return nil, err
		}
		eb, err := decodeB64(j.E)
		if err != nil {
			return nil, err
		}
		n := new(big.Int).SetBytes(nb)
		e := new(big.Int).SetBytes(eb)
		if n.BitLen() < minRSAModulusBits || !e.IsInt64() || e.Int64() < 3 || e.Int64() > 1<<31-1 || e.Bit(0) == 0 {
			return nil, errors.New("unacceptable RSA key")
		}
		return &rsa.PublicKey{N: n, E: int(e.Int64())}, nil
	case "EC":
		if j.Crv != "P-256" {
			return nil, errors.New("unsupported curve")
		}
		xb, err := decodeB64(j.X)
		if err != nil {
			return nil, err
		}
		yb, err := decodeB64(j.Y)
		if err != nil {
			return nil, err
		}
		if len(xb) > 32 || len(yb) > 32 {
			return nil, errors.New("coordinate too long")
		}
		point := make([]byte, 65)
		point[0] = 4
		copy(point[1+32-len(xb):33], xb)
		copy(point[33+32-len(yb):], yb)
		// ParseUncompressedPublicKey rejects points that are not on the curve.
		pub, err := ecdsa.ParseUncompressedPublicKey(elliptic.P256(), point)
		if err != nil {
			return nil, errors.New("point is not on the curve")
		}
		return pub, nil
	}
	return nil, errors.New("unsupported key type")
}
