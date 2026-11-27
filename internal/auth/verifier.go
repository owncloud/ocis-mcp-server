package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/rsa"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	sdkauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
)

const (
	// maxTokenBytes bounds the access token size accepted before parsing.
	maxTokenBytes = 8 * 1024
	// clockLeeway is the tolerated clock skew for exp and nbf.
	clockLeeway = 60 * time.Second
)

// allowedAlgs is the signature algorithm allow-list: never none, never HS*.
var allowedAlgs = []string{"RS256", "PS256", "ES256"}

// VerifierOptions configures a Verifier.
type VerifierOptions struct {
	// Issuer is the exact issuer string that tokens must carry in iss.
	Issuer string
	// Resource is the canonical resource URI that tokens must name in aud.
	Resource string
	// HTTPClient fetches the discovery document and the JWKS. Use
	// NewIssuerHTTPClient.
	HTTPClient *http.Client
	// Now is the clock; nil means time.Now.
	Now func() time.Time
}

// Verifier validates access tokens locally against the issuer's signing keys.
type Verifier struct {
	issuer   string
	resource string
	keys     *KeySet
	now      func() time.Time
}

// NewVerifier discovers the issuer's JWKS and returns a Verifier. Any failure
// is returned: a verifier that cannot reach its issuer must not start.
func NewVerifier(ctx context.Context, o VerifierOptions) (*Verifier, error) {
	if o.Issuer == "" {
		return nil, errors.New("verifier needs an issuer")
	}
	if o.Resource == "" {
		return nil, errors.New("verifier needs a resource URI")
	}
	jwksURI, err := Discover(ctx, o.HTTPClient, o.Issuer)
	if err != nil {
		return nil, err
	}
	now := o.Now
	if now == nil {
		now = time.Now
	}
	return &Verifier{
		issuer:   o.Issuer,
		resource: o.Resource,
		keys:     NewKeySet(o.HTTPClient, jwksURI, now),
		now:      now,
	}, nil
}

// reject builds a rejection. Every rejection wraps sdkauth.ErrInvalidToken so
// the SDK answers 401; the reason is a fixed string and never carries the
// token, a claim value or a key id.
func reject(reason string) error {
	return &rejection{reason: reason}
}

type rejection struct{ reason string }

func (r *rejection) Error() string { return "invalid token: " + r.reason }
func (r *rejection) Unwrap() error { return sdkauth.ErrInvalidToken }

// verifyClaims are the claims the verifier reads. scope and scp are decoded
// loosely because IdPs disagree on their JSON type.
type verifyClaims struct {
	jwt.RegisteredClaims
	Scope any `json:"scope"`
	Scp   any `json:"scp"`
}

// Verify validates token and returns the verified caller. Its signature
// matches sdkauth.TokenVerifier. Every error wraps sdkauth.ErrInvalidToken.
func (v *Verifier) Verify(ctx context.Context, token string, _ *http.Request) (*sdkauth.TokenInfo, error) {
	if token == "" {
		return nil, reject("empty")
	}
	if len(token) > maxTokenBytes {
		return nil, reject("too large")
	}
	if strings.ContainsAny(token, " \t\r\n\v\f") {
		return nil, reject("malformed")
	}

	parser := jwt.NewParser(
		jwt.WithValidMethods(allowedAlgs),
		jwt.WithIssuer(v.issuer),
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(clockLeeway),
		jwt.WithTimeFunc(v.now),
	)
	claims := &verifyClaims{}
	parsed, err := parser.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
		return v.keyFor(ctx, t)
	})
	if err != nil || parsed == nil || !parsed.Valid {
		return nil, reject(parseFailureReason(err))
	}

	if claims.Subject == "" {
		return nil, reject("subject")
	}
	aud, err := claims.GetAudience()
	if err != nil || len(aud) == 0 {
		return nil, reject("audience")
	}
	// azp is never a substitute for aud.
	if !oauthex.MatchesResource(aud, v.resource) {
		return nil, reject("audience")
	}
	if claims.ExpiresAt == nil {
		return nil, reject("expiration")
	}

	return &sdkauth.TokenInfo{
		UserID:     claims.Subject,
		Scopes:     scopesFrom(claims.Scope, claims.Scp),
		Expiration: claims.ExpiresAt.Time,
		Extra:      map[string]any{"iss": v.issuer},
	}, nil
}

// keyFor resolves the verification key for a token. The header must carry a
// non-empty kid, and the algorithm is bound to the key type: RS256 and PS256
// need an RSA key, ES256 needs an ECDSA key.
func (v *Verifier) keyFor(ctx context.Context, t *jwt.Token) (any, error) {
	kid, _ := t.Header["kid"].(string)
	if kid == "" {
		return nil, errKeyRejected
	}
	key, err := v.keys.Key(ctx, kid)
	if err != nil {
		return nil, errKeyRejected
	}
	switch t.Method.Alg() {
	case "RS256", "PS256":
		if pub, ok := key.(*rsa.PublicKey); ok {
			return pub, nil
		}
	case "ES256":
		if pub, ok := key.(*ecdsa.PublicKey); ok {
			return pub, nil
		}
	}
	return nil, errKeyRejected
}

var errKeyRejected = errors.New("key rejected")

// parseFailureReason maps a parser error to a fixed reason. The parser error
// text is never forwarded.
func parseFailureReason(err error) string {
	switch {
	case errors.Is(err, jwt.ErrTokenExpired):
		return "expired"
	case errors.Is(err, jwt.ErrTokenNotValidYet):
		return "not yet valid"
	case errors.Is(err, jwt.ErrTokenInvalidIssuer):
		return "issuer"
	case errors.Is(err, jwt.ErrTokenRequiredClaimMissing):
		return "required claim missing"
	case errors.Is(err, errKeyRejected):
		return "signing key"
	case errors.Is(err, jwt.ErrTokenSignatureInvalid):
		return "signature"
	default:
		return "malformed"
	}
}

// scopesFrom returns the union, without duplicates and in first-seen order, of
// the space-delimited "scope" claim and the "scp" claim (array of strings, or
// a space-delimited string).
func scopesFrom(scope, scp any) []string {
	var out []string
	seen := map[string]bool{}
	add := func(s string) {
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	for _, claim := range []any{scope, scp} {
		switch c := claim.(type) {
		case string:
			for _, s := range strings.Fields(c) {
				add(s)
			}
		case []any:
			for _, e := range c {
				if s, ok := e.(string); ok {
					add(s)
				}
			}
		}
	}
	return out
}
