<!--
SPDX-FileCopyrightText: 2026 ownCloud GmbH
SPDX-License-Identifier: Apache-2.0
-->

# OIDC inbound for the HTTP transport (Phase 2) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The HTTP transport authenticates callers with OIDC access tokens (OAuth 2.1 resource server) and logs the verified caller per tool call.

**Architecture:** A new `internal/auth` package discovers the issuer, caches its JWKS and implements the SDK's `auth.TokenVerifier` with local JWT validation. `internal/mcpserver` wraps `/mcp` in the SDK's `RequireBearerToken`, serves RFC 9728 metadata on a public route, and keeps the `secret` and `none` modes. `internal/config` gains the inbound `OCIS_MCP_HTTP_*` settings and deprecated aliases for the old names.

**Tech Stack:** Go 1.26, go-sdk v1.8.0 (`auth`, `oauthex`), `github.com/golang-jwt/jwt/v5` v5.3.1 (already in the module graph, issue pending), `golang.org/x/sync/singleflight` (already in the module graph).

**Spec:** `docs/superpowers/specs/2026-11-27-mcp-2026-07-28-and-oidc-design.md`. Rules and names: `DECISIONS.md` G3, G4.3, G6, G7.

## Global Constraints

- Algorithm allow-list RS256, PS256, ES256. Never `none`, never HS*.
- Required claims `iss` (exact match to configured issuer), `aud`, `exp`, `sub`. `nbf` checked when present. Leeway 60 s, a constant.
- `aud` must contain the canonical resource URI `<public base URL>/mcp` (lowercase scheme and host, no trailing slash, no fragment), compared with `oauthex.MatchesResource`. `azp` is never a substitute. No audience override setting.
- Discovery: OIDC Discovery, then RFC 8414. Discovered `issuer` must equal the configured one. Issuer and `jwks_uri` must be https, plain http only for a loopback host. Redirects that downgrade to http are refused.
- JWKS: refresh hourly. Refetch on unknown `kid` at most once per 60 s, fetch timeout 10 s, singleflight, 1 MiB body cap. On fetch failure serve last-known-good keys up to 24 h and fail closed for an unknown `kid`. A token without `kid` is rejected.
- Token only from the `Authorization: Bearer` header. Never log a token, JWT or claim body. Only `iss`, `sub` and a redacted `kid`.
- One mode per instance: `oidc`, `secret` (deprecated, startup warning) or `none` (loopback only). The secret never works in `oidc` mode. No token is ever forwarded to oCIS.
- One required scope, default `ocis:mcp`, enforced by SDK middleware. Env names per `DECISIONS.md` G7. Old names work, log one deprecation warning each, new name wins. No setting value or secret appears in a warning.
- stdio ignores every `OCIS_MCP_HTTP_*` setting.
- Table-driven tests, no skipped tests, no TODO comments. Commits: `git commit -s -S`, conventional, `Signed-off-by` David Walter, `Co-Authored-By` and `🤖 Generated with Claude Code` trailers, no force push. Docs and commit text terse, no em dashes. Actions only from owncloud or `actions/*`.
- Test output never contains a token. Test helpers sign tokens in memory and never print them.

## Review Focus

- Token only in `?access_token=`: 401, tool never runs, no oCIS hit. Pinned in Task 7.
- `aud` forms: string versus array, trailing slash, `azp`-only token (rejected). Pinned in Task 6.
- JWKS endpoint slow, huge or 5xx: stale keys served, unknown `kid` returns 401 (not 500, not a hang past 10 s). Pinned in Task 5.
- Issuer or `jwks_uri` redirecting https to http: refused. Pinned in Task 5.
- Scope claim as `scope` string versus `scp` array, and a valid token missing the scope gives 403 not 401. Pinned in Tasks 6 and 7.

## File Structure

- `internal/auth/discovery.go`, `jwks.go`, `verifier.go`, `identity.go` (new): discovery, key cache, verifier, caller identity helper.
- `internal/auth/*_test.go`, `internal/auth/testidp_test.go`: fake IdP and in-memory token signer.
- `internal/config/config.go`: inbound settings, aliases, warnings.
- `internal/mcpserver/http.go`, `audit.go` (new): auth wiring, audit middleware.
- `cmd/ocis-mcp-server/main.go`: build verifier, log warnings.
- `integration/keycloak/` (new): realm import, `//go:build integration` test, `make test-integration`.

### Task 1: Tighten the parked bearer-secret assertions

**Files:** Modify `internal/mcpserver/http_test.go` (`TestHTTPBearerSecret`, rows for the right token and the empty secret).

- [ ] **Step 1:** Change those two rows from "not 401" to an exact `200` assertion.
- [ ] **Step 2:** Run `go test -race -count=1 ./internal/mcpserver -run TestHTTPBearerSecret`. Expected: PASS. If a row is not 200, report it instead of loosening it.
- [ ] **Step 3:** Commit `test(http): assert exact 200 for accepted bearer rows`.

### Task 2: Caller identity in stateless mode and audit log

**Files:** Create `internal/auth/identity.go`, `internal/mcpserver/audit.go`. Test `internal/mcpserver/audit_test.go`.

**Interfaces:**
- Produces: `type Identity struct{ Issuer, Subject string }`, `func IdentityFrom(ctx context.Context, extra *mcp.RequestExtra) (Identity, bool)`, `func AuditMiddleware(logger *slog.Logger) mcp.Middleware`.

- [ ] **Step 1:** Test `TestIdentityReachesHandlerStateless`: stub `auth.TokenVerifier` returning `TokenInfo{UserID:"u1", Extra:{"iss":"https://idp.test"}}`, real `RequireBearerToken`, the stateless handler from `NewHTTPHandler` internals, a tool that returns the identity. Assert `IdentityFrom` yields `u1` and the issuer. Run it first to see whether `req.Extra.TokenInfo` or only `auth.TokenInfoFromContext` carries it, and implement `IdentityFrom` to use whichever source works, preferring `extra.TokenInfo`.
- [ ] **Step 2:** Test `TestAuditLogFields` (table: success, tool error, no identity): captured slog records contain `iss`, `sub`, `method`, `tool`, `outcome`, and never the token string.
- [ ] **Step 3:** Implement. `AuditMiddleware` logs for `tools/call` only, at info level.
- [ ] **Step 4:** Run `go test -race ./internal/mcpserver ./internal/auth`. Commit `feat(auth): expose verified caller to handlers and audit log`.

### Task 3: Inbound config

**Files:** Modify `internal/config/config.go`, test `config_test.go`.

**Interfaces:**
- Produces on `Config`: `HTTPAuth string` (`oidc|secret|none`), `HTTPPublicURL string`, `HTTPOIDCIssuer string`, `HTTPOIDCScopes []string`, `HTTPOIDCCAFile string`, `Warnings []string`, `func (c *Config) ResourceURI() string`, `func (c *Config) MetadataURL() string` (public URL + `/.well-known/oauth-protected-resource/mcp`).

- [ ] **Step 1:** Table test `TestLoadHTTPAuthModes` rows: unset inferred (`issuer` gives oidc, `secret` gives secret, neither gives none); issuer and secret both set without mode is an error; explicit mode wins and warns for the unused credential; `oidc` without issuer or public URL errors; public URL with a path, query or non-https non-loopback errors; issuer or public URL uses http on non-loopback errors; `none` on non-loopback bind errors; unknown mode errors; default scope `ocis:mcp`, comma or space separated scopes parse. Rows assert no warning text contains the secret value.
- [ ] **Step 2:** Test `TestResourceURI` rows: trailing slash, upper-case host, default port. Expected: lowercase, no trailing slash, ends `/mcp`.
- [ ] **Step 3:** Implement. Keep `HTTPAuthEnabled()` true for `oidc` and `secret`. Replace the existing secret-only non-loopback check with the mode rule.
- [ ] **Step 4:** Run `go test -race ./internal/config`. Commit `feat(config): add inbound OIDC settings`.

### Task 4: Outbound names, aliases and dead settings

**Files:** Modify `internal/config/config.go`, `internal/client/client.go:126`, `internal/tools/{admin,resources}.go` only if their tests pin the value. Tests in `config_test.go`, `internal/client/client_test.go`.

**Interfaces:**
- Produces: env `OCIS_MCP_OCIS_AUTH_MODE` (`app-token|static-bearer|none`), `OCIS_MCP_OCIS_APP_TOKEN_USER`, `_VALUE`, `OCIS_MCP_OCIS_BEARER_TOKEN`. `Config.AuthMode` reports the new value.

- [ ] **Step 1:** Table test `TestLoadOutboundAliases`: each old name alone still works and adds exactly one deprecation warning naming the old and new variable; both set means the new wins and still warns; legacy `OCIS_MCP_AUTH_MODE=oidc` maps to `static-bearer`; the three dead `OCIS_MCP_OIDC_*` variables each warn "no effect", the issuer one points at `OCIS_MCP_HTTP_OIDC_ISSUER`. No warning contains a value.
- [ ] **Step 2:** Test that the `auth_mode` field in the admin and resources tool output is `static-bearer` for a bearer config (pins the visible change).
- [ ] **Step 3:** Implement a small `getenvAlias(newName, oldName string, w *[]string) string`. Update `applyAuth` to the new value.
- [ ] **Step 4:** Run `go test -race ./...`. Commit `feat(config): name outbound settings by direction, keep old names as aliases`.

### Task 5: Discovery and JWKS cache

**Files:** Create `internal/auth/discovery.go`, `jwks.go`. Tests `discovery_test.go`, `jwks_test.go`, `testidp_test.go`.

**Interfaces:**
- Produces: `func NewIssuerHTTPClient(caFile string) (*http.Client, error)` (10 s timeout, optional extra CA, refuses redirect to http), `func Discover(ctx context.Context, c *http.Client, issuer string) (jwksURI string, err error)`, `type KeySet` with `func NewKeySet(c *http.Client, jwksURI string, now func() time.Time) *KeySet` and `func (k *KeySet) Key(ctx context.Context, kid string) (crypto.PublicKey, error)`.
- `testidp_test.go` provides a fake IdP (httptest, in-memory RSA and EC keys, rotation, failure switches) and `signToken(t, claims, opts) string` that never prints.

- [ ] **Step 1:** `TestDiscover` rows: OIDC document ok; OIDC 404 then RFC 8414 ok; issuer mismatch; trailing-slash issuer; http issuer non-loopback rejected, loopback allowed; `jwks_uri` http non-loopback rejected; body over 1 MiB rejected; redirect https to http rejected.
- [ ] **Step 2:** `TestKeySet` rows: known `kid`; rotation (unknown `kid` refetches once); second unknown `kid` inside 60 s does not refetch; 100 concurrent unknown `kid` calls cause one fetch (singleflight, count the hits); hourly refresh; fetch fails with cached keys serves stale until 24 h then fails; unknown `kid` with fetch failure errors; fetch hanging returns within 10 s using the fake clock for ages and a real short-timeout client for the hang.
- [ ] **Step 3:** Implement. `Key` returns an error that unwraps to `auth.ErrInvalidToken` only from the verifier, not here.
- [ ] **Step 4:** Run `go test -race ./internal/auth`. Commit `feat(auth): add issuer discovery and JWKS cache`.

### Task 6: Verifier

**Files:** Create `internal/auth/verifier.go`. Test `verifier_test.go`. Modify `go.mod` and `go.sum` (jwt becomes a direct requirement).

**Interfaces:**
- Consumes: Task 5.
- Produces: `type VerifierOptions struct{ Issuer, Resource string; HTTPClient *http.Client; Now func() time.Time }`, `func NewVerifier(ctx context.Context, o VerifierOptions) (*Verifier, error)` (runs discovery, a failure is a startup error), `func (v *Verifier) Verify(ctx context.Context, token string, r *http.Request) (*auth.TokenInfo, error)`. Every rejection wraps `auth.ErrInvalidToken` so the SDK answers 401 and never 500.
- `TokenInfo`: `UserID` is `sub`, `Scopes` from `scope` (space string) or `scp` (array), `Expiration` from `exp`, `Extra["iss"]`.

- [ ] **Step 1:** Table `TestVerify` rows, each asserting error class and that the error text holds no token: valid; expired; expired inside 60 s leeway accepted; `nbf` in future; wrong `iss`; wrong `aud`; `aud` string; `aud` array with ours; `aud` with trailing slash; `azp` equals resource but `aud` is `web` (rejected); missing `aud`, `exp`, `sub`; bad signature; unknown `kid`; no `kid`; `alg: none`; HS256 signed with the public key bytes as secret; ES256 and PS256 accepted; empty token; token with a space.
- [ ] **Step 2:** `TestVerifyScopes`: `scope` string and `scp` array both yield the scopes.
- [ ] **Step 3:** Implement with `jwt.NewParser` options (`WithValidMethods`, `WithIssuer`, `WithExpirationRequired`, `WithLeeway`), then the `aud` check via `oauthex.MatchesResource` on `GetAudience()`.
- [ ] **Step 4:** Run `go test -race ./internal/auth`. Commit `feat(auth): validate access tokens locally`.

### Task 7: HTTP wiring

**Files:** Modify `internal/mcpserver/http.go`, `helpers_test.go`, `http_test.go`.

**Interfaces:**
- Produces: `type HTTPAuth struct{ Mode, Secret, PublicURL string; Scopes []string; Verifier sdkauth.TokenVerifier }` and `func NewHTTPHandler(server *mcp.Server, a HTTPAuth) http.Handler` (replaces the `secret string` parameter).

- [ ] **Step 1:** Update `startHTTP` helpers to the new signature. Existing tests must still pass for `secret` and `none`.
- [ ] **Step 2:** `TestHTTPOIDC` table with a stub verifier and real middleware: no header 401 with `WWW-Authenticate` holding `resource_metadata=` and `scope=`; valid token 200 and tool runs; token with wrong scope 403 and no oCIS hit; `?access_token=` only 401 and no hit; lowercase `bearer` accepted; the configured secret presented as a bearer in `oidc` mode 401.
- [ ] **Step 3:** `TestProtectedResourceMetadata`: GET `/.well-known/oauth-protected-resource/mcp` unauthenticated 200 JSON with `resource` equal to `<public>/mcp`, `authorization_servers` equal to the issuer, `scopes_supported`, `bearer_methods_supported:["header"]`. The URL in the challenge equals the served URL. Security headers present.
- [ ] **Step 4:** `TestHTTPTrustedOrigin`: a request with `Origin` equal to the public URL is accepted, another origin still 403.
- [ ] **Step 5:** Implement. `AddTrustedOrigin(PublicURL)` on the `CrossOriginProtection`, `ClockSkew: 60s` on the SDK options, `AuditMiddleware` registered on the server in `New`.
- [ ] **Step 6:** Run `go test -race ./...`. Commit `feat(http): serve OIDC protected resource and require verified tokens`.

### Task 8: main wiring and startup behavior

**Files:** Modify `cmd/ocis-mcp-server/main.go`. Test `internal/mcpserver/http_test.go` for the pieces that are testable.

- [ ] **Step 1:** In `runHTTP`, build the issuer client, call `NewVerifier` in `oidc` mode (exit 1 with a clear error on failure), pass `HTTPAuth`. Log `cfg.Warnings` once each. Log mode `secret` as deprecated. Replace the old "UNAUTHENTICATED" warning wording with the mode.
- [ ] **Step 2:** Run the binary against a fake IdP in a Go test or script: `oidc` with a bad issuer exits non-zero without printing a secret, `oidc` with a good issuer serves metadata. stdio with `OCIS_MCP_HTTP_*` set ignores them.
- [ ] **Step 3:** Run `make test lint docker-build`. Commit `feat: start the HTTP transport in the configured auth mode`.

### Task 9: Keycloak integration test

**Files:** Create `integration/keycloak/realm.json`, `integration/keycloak/keycloak_test.go` (`//go:build integration`), modify `Makefile`.

- [ ] **Step 1:** Realm import with a client, an audience mapper adding `<public>/mcp`, and the `ocis:mcp` scope. Document the compose command in the test header.
- [ ] **Step 2:** Test: obtain a token from Keycloak (client credentials), call `/mcp` through the real handler, expect 200. Token with the wrong audience expects 401. Token without the scope expects 403.
- [ ] **Step 3:** Add `make test-integration` (`go test -tags integration ./integration/...`). Default `make test` stays unchanged.
- [ ] **Step 4:** Run it against Keycloak 26.2.5 from the `ocis_full` example. Report real output. If Keycloak is not runnable here, say so and leave the test uncommitted as a result, not as a skip.

## Self-Review

- Spec coverage: PRM (T7), verifier and G4.3 rules (T5, T6), identity and audit (T2), config and aliases per G7 (T3, T4), modes per G3 (T3, T7, T8), trusted origins (T7), Keycloak test (T9), stdio untouched (T8). Docs and CHANGELOG belong to Phase 3.
- Calls made while planning: discovery runs at startup and a failure stops the server (fail fast over a running server that answers 401 to everyone); a token without `kid` is rejected; the 403 `error="insufficient_scope"` parameter is not added, the SDK omits it and it is a SHOULD, to be stated in the Phase 3 docs; the visible `auth_mode` tool field changes from `oidc` to `static-bearer`.
