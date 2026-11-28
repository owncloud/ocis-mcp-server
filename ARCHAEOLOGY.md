<!--
SPDX-FileCopyrightText: 2026 ownCloud GmbH
SPDX-License-Identifier: Apache-2.0
-->

# Archaeology: MCP 2026-07-28 and OIDC inbound

Read-only findings. Base: `origin/main` at 7cc057c. Facts come from the live code, the go-sdk
at v1.8.0, the MCP spec sources for 2026-07-28 and owncloud/ocis at 64404ab (8.2.3).
Items marked **unconfirmed** were not verified. Paths are relative to the repo root unless noted.

## 0. Where the sources contradict the starting position

| # | Starting position | Finding |
|---|---|---|
| C1 | go-sdk v1.7.0 or later, pin newest stable | `go.mod:5` already pins **v1.8.0**, the newest stable tag (2026-09-14). v1.9.0-pre.1 is a pre-release. No SDK bump is needed. |
| C2 | A version bump may not change behavior | Confirmed, and worse. `main.go:73-76` passes nil options, so `Stateless=false`. The v1.8.0 stateful handler rejects 2026-07-28 requests with HTTP 400 and `-32022`. **2026-07-28 clients cannot connect today.** |
| C3 | 80 tools in 13 categories | 80 `AddTool` calls in 13 files, but the 5 education tools register only when `OCIS_MCP_EDUCATION_ACCESS_TOKEN` is set (`internal/tools/education.go:70-73`, `register.go:26`). Default is 75 tools in 12 categories. |
| C4 | `OCIS_MCP_AUTH_MODE=oidc` reads one static token | Confirmed (`internal/client/client.go:119-126`). Also `OCIS_MCP_OIDC_ISSUER`, `_CLIENT_ID`, `_CLIENT_SECRET` are read (`internal/config/config.go:61-63`) but used by no code. |
| C5 | oCIS checks bearer tokens | The oCIS proxy **ignores `aud` by default**. A foreign-audience token signed by the configured issuer is accepted. An allow-list exists since 8.2.0 (`PROXY_OIDC_ACCESS_TOKEN_VERIFY_AUD`). |
| C6 | Built-in IdP may issue a usable audience | lico implements RFC 8707, but stock oCIS cannot register a resource, so every `resource` request is refused. No RFC 8693. Details in section 5. |
| C7 | SDK auth middleware emits spec challenges | At v1.8.0 the 401 and 403 challenges carry no `error=` parameter, and the 403 lists all required scopes, not the missing ones. Partly fixed in v1.9.0-pre.1 (401 only). |
| C8 | Deprecated features may be in use | Roots, Sampling and Logging are not used. The SDK still advertises the `logging` capability by default when `ServerOptions.Capabilities` is nil. |
| C9 | Repo docs on OCS | `AGENTS.md:50` says OCS is used for sharing. No sharing call uses OCS (section 6). |
| C10 | New code is free to add dependencies | `AGENTS.md` requires an issue before any new dependency. No JWT or JWKS library is in `go.mod`. This feeds gate G4. |

## 1. go.mod and dependencies

- Module `github.com/owncloud/ocis-mcp-server` (`go.mod:1`), `go 1.26.2` (`go.mod:3`).
  Local toolchain go1.27.3, Dockerfile builder `golang:1.27-alpine` (`Dockerfile:2`).
- One direct dependency: `github.com/modelcontextprotocol/go-sdk v1.8.0` (`go.mod:5`).
- Indirect (`go.mod:7-15`): jsonschema-go v0.4.3, segmentio/asm v1.2.1, segmentio/encoding v0.5.4,
  yosida95/uritemplate/v3 v3.0.2, golang.org/x/oauth2 v0.36.0, x/sync, x/sys, x/time, x/tools.
- No JWT, JWKS or OIDC library. `golang-jwt/jwt/v5` is required only by the SDK for its own tests and
  examples. No `vendor/`, no `tools.go`, no golangci config.

## 2. Code layout

### Entry point `cmd/ocis-mcp-server/main.go`

- `config.Load()` (`:24`), `slog` text logger on stderr (`:125`).
- `mcp.NewServer(&mcp.Implementation{...}, nil)` (`:41-47`). Options are nil: no `ServerOptions`,
  no `SetCacheable`, no capabilities override.
- `tools.RegisterAll(server, ocisClient, cfg)` (`:49`). One shared server and one shared client for all callers.
- Transport switch (`:61-69`): stdio runs `server.Run(ctx, &mcp.StdioTransport{})`, http calls `runHTTP`.
- `runHTTP` (`:72-111`): `mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {return server}, nil)`.
  Only `/mcp` is registered, wrapped as `SecurityHeaders(RequireBearer(cfg.HTTPSecret)(handler))`.
  No `/healthz`, no `/.well-known/*`. `http.Server` has no timeouts (`:100`).

### Config `internal/config/config.go`

All env vars, parsed in `Load()` (`:56-85`):

| Variable | Default | Note |
|---|---|---|
| `OCIS_MCP_OCIS_URL` | required | |
| `OCIS_MCP_AUTH_MODE` | auto: app-token, then oidc, else none (`:146-157`) | Not validated against an allowed set. Unknown value means no outbound auth. |
| `OCIS_MCP_APP_TOKEN_USER`, `_VALUE` | none | |
| `OCIS_MCP_OIDC_ISSUER`, `_CLIENT_ID`, `_CLIENT_SECRET` | none | Read, never used. |
| `OCIS_MCP_OIDC_ACCESS_TOKEN` | none | Static outbound bearer. |
| `OCIS_MCP_EDUCATION_ACCESS_TOKEN` | none | Only toggles tool registration. |
| `OCIS_MCP_TRANSPORT` | `stdio` | |
| `OCIS_MCP_HTTP_ADDR` | `127.0.0.1:8090` | Non-loopback bind without secret is refused (`:129`). |
| `OCIS_MCP_HTTP_SECRET` | none | |
| `OCIS_MCP_LOG_LEVEL` | `info` | |
| `OCIS_MCP_INSECURE`, `OCIS_MCP_TLS_SKIP_VERIFY` | false | |
| `OCIS_MCP_HTTP_TIMEOUT` | `30s` | |

### Inbound middleware `internal/middleware/http.go`

- `RequireBearer(secret)` (`:31-46`): empty secret is a pass-through. Otherwise SHA-256 of the secret
  once, SHA-256 of the presented token per request, `subtle.ConstantTimeCompare`. Scheme match is
  case-insensitive. Failure gives 401 with `WWW-Authenticate: Bearer realm="ocis-mcp-server"`, no `resource_metadata`.
- No identity is placed in the request context. `internal/middleware/logging.go` is an empty stub.

### Outbound client `internal/client`

- `NewRequest(ctx, ...)` (`client.go:38-46`) is the single place that calls `applyAuth`.
- `applyAuth` (`client.go:119-126`): `app-token` uses basic auth from config, `oidc` sets a static bearer
  from config, anything else adds nothing. Credentials are process-wide and immutable.
- `Do` retries 429 up to 3 times and its `time.Sleep` ignores context cancellation (`:68-101`).
- No `context.WithValue`, `ctx.Value`, `TokenInfo` or `RequestExtra` anywhere.
  A caller identity can travel in `ctx` from the handler to `NewRequest`. The SDK exposes verified
  identity as `req.Extra.TokenInfo` (tool handlers) and `auth.TokenInfoFromContext` (middleware).

### State

No sessions, caches or credential stores in our code, no `sync` use, one package var (`version`).
Session state exists only inside the SDK stateful handler. `ocis://auth-mode` exposes mode, URL and
transport, no secrets (`internal/tools/resources.go:133-144`).

### Counts

80 tools (admin 4, app_tokens 3, education 5, files 14, groups 7, notifications 2, ocm 4, search 2,
settings 3, shares 11, spaces 14, users 6, workflows 5), 5 resources, 4 prompts. See C3.

## 3. go-sdk at v1.8.0

Clone used: scratchpad `sdk-research/sdk`. Paths below are inside the SDK.

### Version and protocol

- Newest stable v1.8.0 (2026-09-14). Pre-releases: v1.9.0-pre.1 (2026-11-17). Previous stable v1.7.0 (2026-07-28).
- `mcp/shared.go:50-63`: latest 2026-07-28, also 2025-11-25, 2025-06-18, 2025-03-26, 2024-11-05.
  Legacy `initialize` negotiation is capped at 2025-11-25 (`:80-98`).
  `ServerOptions.SupportedProtocolVersions` can only narrow the set (`mcp/server.go:184-199`).

### Breaking changes from our version to the target

None to apply, since we are on the target. For reference, v1.7.0 to v1.8.0 removed four `MCPGODEBUG`
flags, changed the stateful rejection of new-protocol requests to a `-32022` JSON-RPC error (PR #1143)
and made cancel notifications asynchronous (PR #1151). v1.9.0-pre.1 removes seven more debug flags,
maps uncoded handler errors to `-32603`, decodes structured numbers as `json.Number`, stops following
cross-origin redirects in HTTP clients, adds `error="invalid_token"` to 401s (PR #1507) and checks
expiry before scopes (PR #1350). Diffing v1.8.0 exported API against v1.9.0-pre.1: **unconfirmed**.

### StreamableHTTPOptions (`mcp/streamable.go:128-220`)

| Field | Default |
|---|---|
| `Stateless` | false |
| `JSONResponse` | false |
| `Logger` | nil |
| `EventStore` | nil |
| `SessionTimeout` | 0 (idle sessions never closed) |
| `DisableLocalhostProtection` | false |
| `CrossOriginProtection` | nil, deprecated: wrap with `http.CrossOriginProtection` |
| `MaxRequestBodyBytes` | 4 MiB |
| `PropagateRequestCancellation` | false |

- **Stateless must be set explicitly.** `mcp/streamable.go:1511` comment: the new protocol is supported
  on HTTP only when `Stateless` is true. Stateful mode answers 2026-07-28 requests with 400 and `-32022`
  (`:1515-1558`). The conformance server defaults `-stateless` to true (`conformance/everything-server/main.go:33`).
- Stateless handler (`:353-430`) speaks both protocols, POST only (GET and DELETE give 405), ephemeral
  session per request, ignores `Mcp-Session-Id`. For legacy-protocol requests it synthesizes the
  initialize handshake (`:477-526`).
- 2026-07-28 behavior in the SDK: per-request detection through `_meta.protocolVersion`
  (`mcp/server.go:1940-2000`), `setCompleteResultType` stamps `resultType` (`:2037-2038`),
  `server/discover` is auto-registered, `initialize`, `ping`, `logging/setLevel` and subscribe methods
  return `-32601` under the new protocol, `Mcp-Method` and `Mcp-Name` are validated with `-32020` on
  mismatch (`mcp/streamable_headers.go:362-390`), `CodeResourceNotFound` equals `-32602` (`mcp/shared.go:421-429`).
  Handlers should return `mcp.ResourceNotFoundError(uri)` (`mcp/resource.go:54-66`).
- Caching: `ServerOptions.SetCacheable func(ctx, req, *Cacheable)` with `Cacheable{TTLMs int; CacheScope string}`
  (`mcp/server.go:171-176`, `mcp/protocol.go:1148-1182`). Default `CacheScope` is `public`, `TTLMs` is 0.
  The callback may run under the server lock and must not call back into the server.

### Roots, Sampling, Logging

Deprecated per SEP-2577, fully functional in the window. A nil `Capabilities` still advertises `logging`.
Pass `&ServerCapabilities{}` to drop it (`docs/server.md:1147-1190`).

### auth and oauthex (`auth/auth.go`)

- Provided: `TokenInfo{Scopes, Expiration, UserID, Extra}`, `TokenVerifier func(ctx, token, *http.Request) (*TokenInfo, error)`,
  `RequireBearerToken(verifier, opts)` with `ResourceMetadataURL`, `Scopes`, `AllowMissingExpiration`, `ClockSkew`,
  `TokenInfoFromContext`, `ProtectedResourceMetadataHandler(*oauthex.ProtectedResourceMetadata)`,
  `oauthex.MatchesResource(claims, resource)`, sentinel errors `ErrInvalidToken` and `ErrOAuth`.
- Middleware semantics: missing or malformed header gives 401. Verifier error wrapping `ErrInvalidToken` gives 401,
  wrapping `ErrOAuth` gives 400, anything else gives 500. Missing scope gives 403. Missing expiration gives 401 unless allowed.
  The challenge is `Bearer resource_metadata="...", scope="..."` when configured.
  No `error=` parameter at v1.8.0. Body is plain text.
- `TokenInfo.UserID` also drives the SDK's session-hijack check (403 on mismatch, `mcp/streamable.go:565-571`).
- **Not provided:** JWKS fetch or cache, JWT parsing or signature checks, OIDC or RFC 8414 discovery for a resource
  server, issuer or audience checks, introspection, RFC 6750 error-code mapping, signed metadata.
- Examples: `examples/auth/server/main.go` (PRM handler and `RequireBearerToken`),
  `examples/server/auth-middleware/` (HS256 with a hard-coded secret, not suitable for OIDC).

### Conformance

`.github/workflows/conformance.yml` in the SDK runs `npx @modelcontextprotocol/conformance@0.2.0-alpha.11`
against the everything-server in two legs, stateful with `--requirements 2025-11-25` and stateless with
`--requirements 2026-07-28`. Local script `scripts/server-conformance.sh`. Needs Node 22 and npx network access.
Whether it runs against a non-SDK server, and whether it has server-side auth scenarios: **unconfirmed**.

## 4. Spec 2026-07-28, server-side authorization

Base: `https://modelcontextprotocol.io/specification/2026-07-28`. Page keys: AUTH `/basic/authorization`,
DISC `/basic/authorization/authorization-server-discovery`, SEC `/basic/authorization/security-considerations`,
TR `/basic/transports/streamable-http`.

### Non-auth server changes

Sessions removed, `Mcp-Session-Id` ignored. Handshake removed, every request carries `_meta` protocolVersion
and clientCapabilities (missing gives `-32602`). `server/discover` required. `resultType` required on
results. `Mcp-Method` required, `Mcp-Name` required for `tools/call`, `resources/read`, `prompts/get`, mismatch
gives 400 `-32020`. Unsupported version gives `-32022` with `data.supported`. `ttlMs` (>= 0) and `cacheScope` required
on discover, the four list methods and `resources/read`. Resource-not-found is `-32602`, never `-32002`.
`Origin` validation MUST, 403 on invalid. GET and DELETE on the endpoint SHOULD give 405.
Roots, Sampling, Logging deprecated, earliest removal on or after 2027-07-28. Details and URLs are in the spec changelog.

### MUST and SHOULD table with our status

Status for the current code. "met" means already true today, "partial" means some of it, "missing" means absent.
"SDK" notes what the SDK covers once wired.

| # | Requirement | Level | Our status | Notes |
|---|---|---|---|---|
| 1 | Authorization for HTTP transports | SHOULD (optional overall) | partial | Shared secret only, no OAuth. |
| 2 | Serve RFC 9728 protected resource metadata, with `authorization_servers` | MUST | missing | SDK has `ProtectedResourceMetadataHandler`. |
| 3 | At least one discovery mechanism: `resource_metadata` in 401, or well-known URI | MUST (one) | missing | Plan: both. Path-suffixed well-known `/.well-known/oauth-protected-resource/mcp`. |
| 4 | Validate access tokens per OAuth 2.1 5.2 | MUST | missing | Needs our `TokenVerifier`. |
| 5 | Validate audience (RFC 8707) | MUST | missing | `oauthex.MatchesResource` helps. |
| 6 | Invalid or expired token gives 401 | MUST | partial | 401 exists for the shared secret only. |
| 7 | Accept only tokens valid for our own resources, no other tokens | MUST | missing | Falls out of 4 and 5. |
| 8 | Do not pass through the inbound token to upstream APIs | MUST NOT | met | Outbound uses a separate static credential (`client.go:119-126`). Must stay true. |
| 9 | 401 `WWW-Authenticate: Bearer resource_metadata=...` | MUST (mechanism 1) | missing | Current header has realm only. SDK emits it. |
| 10 | `scope` in the challenge | SHOULD | missing | SDK emits when scopes are configured. |
| 11 | 403 `insufficient_scope` with `scope` and `resource_metadata` | SHOULD | missing | SDK 403 lacks `error=` and lists all required scopes. Gap for G6. |
| 12 | Honor scope hierarchies when judging sufficiency | MUST | missing | Only matters if G6 has more than one scope. |
| 13 | 401 required or invalid, 403 insufficient, 400 malformed | MUST | partial | |
| 14 | Token only in `Authorization` header, never in the query string | MUST NOT (client rule) | met | We read only the header. Explicit rejection of a query token is our inference, **unconfirmed** as a server rule. |
| 15 | Canonical resource URI: scheme, no fragment, lowercase scheme and host, no trailing slash | SHOULD, MUST (no fragment) | missing | Needs a public base URL setting. |
| 16 | AS offers RFC 8414 or OIDC discovery | MUST (AS) | n/a | We consume the issuer's discovery. See section 5. |
| 17 | Do not offer `offline_access` in scopes | SHOULD NOT | met | No scopes today. |
| 18 | Do not log or cache tokens insecurely | MUST | partial | No token logging in our code today. Needs a test once tokens exist. |
| 19 | Validate `Origin`, 403 on invalid | MUST | partial | SDK localhost protection covers loopback binds. Non-loopback coverage and `http.CrossOriginProtection` use: **unconfirmed**. |
| 20 | Authenticate all connections | SHOULD | partial | Only with a secret set. Loopback without secret is allowed. |
| 21 | stdio SHOULD NOT use this flow, read credentials from the environment | SHOULD NOT | met | stdio uses env credentials. |
| 22 | `server/discover` implemented | MUST | missing | SDK stateless path registers it. |
| 23 | `resultType` on every result | MUST | missing | SDK stamps it in stateless mode. |
| 24 | `Mcp-Method`, `Mcp-Name` validation, 400 `-32020` | MUST | missing | SDK validates in stateless mode. |
| 25 | `ttlMs` and `cacheScope` on list and read results | MUST | missing | Needs `SetCacheable`. |
| 26 | Resource-not-found is `-32602` | MUST | partial | SDK maps it. Our handlers must return `mcp.ResourceNotFoundError`. Handler inspection: **unconfirmed**. |
| 27 | Ignore `Mcp-Session-Id` and `Last-Event-ID` | SHOULD | missing | SDK ignores them in stateless mode. |

Security best practice items beyond the table: caching security (public scope is shareable across tokens,
so per-primitive access control stays our job), MCP proxy confused-deputy rules (we are a single-credential
proxy to oCIS, so per-caller authorization is the open question for G5), scope minimization (no wildcard
scopes, log elevation events). Bodies of the State Handle Hijacking and SSRF sections: **unconfirmed**.

### Downstream credentials (input to G5)

The spec forbids passthrough and requires that the upstream token is a separate token issued by the
upstream authorization server. It names no mechanism for obtaining it. RFC 8693 token exchange is neither
required nor forbidden by the core spec, and only appears inside the optional Enterprise-Managed Authorization
extension, where the client does the exchange. Any claim that token exchange is "spec-blessed" is
**unconfirmed** and should not be made.

Extensions Tasks, MCP Apps and Enterprise Managed Authorization are optional. MRTR and `subscriptions/listen`
are core patterns, but the SDK implements them and we use neither.

## 5. oCIS side (owncloud/ocis 64404ab)

### Proxy token verification

- Discovery: OIDC only, `<issuer>/.well-known/openid-configuration`, fetched lazily on first request, `issuer`
  must equal `PROXY_OIDC_ISSUER` (`ocis-pkg/oidc/client.go:107-155`). No RFC 8414.
- Settings (`services/proxy/pkg/config/config.go:116-132`, defaults `defaultconfig.go:42-58`):
  `PROXY_OIDC_ISSUER`, `PROXY_OIDC_INSECURE`, `PROXY_OIDC_ACCESS_TOKEN_VERIFY_METHOD` (`jwt` default, or `none`),
  `PROXY_OIDC_SKIP_USER_INFO`, `PROXY_OIDC_REWRITE_WELLKNOWN`, `PROXY_OIDC_ACCESS_TOKEN_VERIFY_AUD`,
  `PROXY_OIDC_JWKS_REFRESH_INTERVAL` (60 min), `_TIMEOUT` (10 s), `_RATE_LIMIT` (60 s), `_UNKNOWN_KID` (true),
  `PROXY_OIDC_USERINFO_CACHE_*`. User mapping through `PROXY_USER_OIDC_CLAIM` (default `preferred_username`)
  and `PROXY_USER_CS3_CLAIM`.
- JWT verification (`client.go:301-341`): signature via JWKS, `iss`, `exp` only if present. No algorithm
  allow-list on this path (`_supportedAlgorithms` at `client.go:76` is used for logout tokens). Exploitability
  of the missing allow-list: **unconfirmed**.
- `aud`: **ignored by default.** Checked only when `PROXY_OIDC_ACCESS_TOKEN_VERIFY_AUD` is set (since 8.2.0,
  `changelog/8.2.0_2026-08-10/proxy-oidc-access-token-verify-audience.md`). A matching `azp` also passes
  (`client.go:335`, `:345-359`). Test case "check disabled accepts foreign audience" at `ocis-pkg/oidc/client_test.go:208`.
- Userinfo mode: unless `SKIP_USER_INFO`, the proxy calls the IdP userinfo endpoint with the token
  (`middleware/oidc_auth.go:91-106`). With `VERIFY_METHOD=none` no local checks run at all (`client.go:291-293`).
  Verified claims are cached by token hash until `exp`. IdP outage gives 503, not 401.
- Authenticators in order (`services/proxy/pkg/command/server.go:274-321`): basic (off by default, on in `ocis_full`
  with `DEMO_USERS`), app auth (`PROXY_ENABLE_APP_AUTH`, `Authorization: Basic base64(user:apptoken)`,
  owner-scope only, rejected in multi-instance mode), OIDC bearer, public share, signed URL.
- Scopes: **not enforced** anywhere in `services/proxy/pkg`. A valid token acts as the full user on Graph,
  WebDAV and OCS.

### Built-in IdP (lico v0.69.0)

- Access tokens are PS256 JWTs. `aud` is the client ID, or the requested resource. Lifetimes: access 5 min,
  ID 5 min, refresh 30 days (`services/idp/pkg/config/defaults/defaultconfig.go:65-71`).
- Clients come from config only: `web` (public) plus desktop, Android and iOS with demo defaults baked into source.
  Dynamic registration is off by default (`IDP_ALLOW_DYNAMIC_CLIENT_REGISTRATION`).
- Grants: `authorization_code` and `refresh_token`. PKCE S256 only.
- RFC 8707: implemented in lico (`oidc/payload/authentication.go:349-359`), one resource per request, unregistered
  resource gives `invalid_target`. Resources come from a `resources:` list in the registration file, but oCIS writes
  only `clients:` (`services/idp/pkg/service/v0/service.go:157-160`) and has no config for resources.
  Net effect: **every `resource` request is refused in stock oCIS.** The temp file is regenerated at start, so
  hand edits do not persist (not run to confirm).
- RFC 8693 token exchange: not supported.
- Discovery: OIDC at `/.well-known/openid-configuration`. RFC 8414 exists on the IdP service, but the oCIS
  proxy routes only the OIDC path (`defaultconfig.go:132`). Whether the RFC 8414 path answers through the public URL: **unconfirmed**.
- JWKS at `/konnect/v1/jwks.json`. The built-in IdP cannot issue a token whose audience is an MCP server URL.
  A resource server could still verify its signature through JWKS.
- CIMD support is advertised in lico and gated by a bootstrap option. Whether oCIS exposes it: **unconfirmed**.

### Keycloak in `deployments/examples/ocis_full`

- Wiring `ocis_full/keycloak.yml:9-23`: issuer `https://keycloak.owncloud.test/realms/oCIS`,
  `PROXY_OIDC_REWRITE_WELLKNOWN=true`, built-in `idp` excluded, `preferred_username` to `username`,
  autoprovision on, role assignment from OIDC. `PROXY_OIDC_ACCESS_TOKEN_VERIFY_AUD` is not set.
- Image `quay.io/keycloak/keycloak:26.2.5` (`ocis_full/.env:199`). Realm `oCIS` from
  `config/keycloak/ocis-realm.dist.json`: access token 300 s, RS256, default scopes include `roles`.
  Clients `web` (public), desktop, Android, iOS (same IDs as the built-in IdP), plus standard Keycloak clients.
  No service-account client exists.
- `roles` scope carries an audience-resolve mapper, so `aud` is typically `account`, not `web`. This is why oCIS
  also checks `azp`.
- Keycloak capabilities:
  - `aud` control: audience mappers (included client or custom audience). Available in 26.2.5.
  - RFC 8707 `resource`: experimental from 26.7 behind `--features=resource-indicators`. Not in 26.2.5.
    Open bugs on introspection (#54296) and offline refresh (#53261).
  - RFC 8693: standard token exchange v2 on by default, confidential client with the "Standard token exchange"
    switch, internal-to-internal only, `audience` can only narrow, no `resource` support. Release that introduced v2:
    **unconfirmed** (possibly 26.2). Legacy v1 needs a preview feature and is deprecated.
  - Discovery: OIDC at `/realms/{realm}/.well-known/openid-configuration`. RFC 8414 path and version: **unconfirmed**
    (upstream issue #43253 still open).
  - DCR at `/realms/{realm}/clients-registrations/openid-connect`. CIMD behind an experimental flag.
- The existing `ocis_full/mcp.yml:16-18` pairs a shared HTTP secret with one static app token, which is the
  single-identity setup this work addresses.

### Implications for a resource server that must not pass through tokens

By default oCIS ignores `aud`, so any token that reaches the MCP server would also work directly against oCIS.
The reverse also holds: an oCIS-client token (for example `web`) is a valid bearer at the MCP server unless we
check audience. With the built-in IdP we cannot obtain a distinct MCP audience. With Keycloak 26.2.5 the audience
mapper approach works, token exchange can only narrow. Operators on a shared IdP should set
`PROXY_OIDC_ACCESS_TOKEN_VERIFY_AUD`, noting that a matching `azp` still passes.

## 6. OCS API usage (report only)

All through `client.GetRaw` or a delete helper with the shared credential.

- `GET /ocs/v1.php/cloud/capabilities?format=json`: `internal/tools/resources.go:74` and `:87` (version resource),
  `internal/tools/admin.go:120` (`ocis_get_version`) and `:138` (`ocis_get_capabilities`).
- `/ocs/v2.php/apps/notifications/api/v1/notifications`: `internal/tools/notifications.go:64` (list) and `:96` (delete by id,
  HTTP method not captured: **unconfirmed**).
- OCS error-envelope parsing: `internal/client/errors.go:60-70`.
- No sharing call uses OCS (see C9). Not extending OCS, per scope.

## 7. Roots, Sampling, Logging

No use anywhere in `cmd/` or `internal/`. Grep for ListRoots, CreateMessage, LoggingMessage, SetLevel, LoggingHandler,
Sampling, Roots, ServerOptions, StreamableHTTPOptions, Stateless, EventStore, SessionTimeout found nothing. Server logging
is `slog` to stderr. Only the default `logging` capability advertisement remains (C8).

## 8. Tests, build and CI (relevant for Phase 1 gates)

- 27 test files, table-driven with `httptest.NewServer` as a fake oCIS. `newTestConfig` is duplicated in
  `internal/tools/users_test.go` and `internal/client/client_test.go`. `register_test.go` has three no-panic smoke tests,
  no tool-count assertions.
- **No test exercises the MCP HTTP transport.** No use of `StreamableHTTPHandler` or an in-memory transport.
  Middleware tests cover `RequireBearer` and `SecurityHeaders` with `httptest.NewRecorder`.
  `cmd/ocis-mcp-server` has no tests.
- `Makefile`: `build`, `test` (`go test -race ./...`), `lint` (`golangci-lint run ./...`, no project config), `cover`,
  `clean`, `docker-build`, `release-dry-run`. **No `reuse` target and no CI job runs `reuse lint`**, although `REUSE.toml` exists.
- CI (`.github/workflows`): `ci.yml` (build, `go test -race` with a 70% coverage gate, `go vet`, golangci-lint v2),
  `release.yml` (GoReleaser, macOS sign and notarize), `zizmor.yml`. Actions pinned by SHA.
- Docker: non-root user, port 8090. Default bind `127.0.0.1:8090` is unreachable from outside a container unless overridden. Not tested.
- Policy (`AGENTS.md`): no new dependencies without a prior issue, signed and DCO commits, pinned actions from approved owners.

## 9. Observations for later phases (not decisions)

- `http.Server` without timeouts is a hardening gap on any non-loopback bind. In scope for Phase 2 only if a gate says so.
- Stateless mode removes server-initiated requests and GET streams. We use none, so no feature loss is expected.
- `-32022` when a 2026-07-28 client hits a stateful handler is the exact failure users would see today.
- Verified identity can reach tool handlers through `req.Extra.TokenInfo`, which makes audit logging of `iss` and `sub` cheap.
  It does not change outbound identity. That is G5.
