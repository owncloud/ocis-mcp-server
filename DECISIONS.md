<!--
SPDX-FileCopyrightText: 2026 ownCloud GmbH
SPDX-License-Identifier: Apache-2.0
-->

# Decision gates: MCP 2026-07-28 and OIDC inbound

Every gate blocks until answered. Each has options, a recommendation and consequences.
Evidence is in `ARCHAEOLOGY.md` (cited as A§n). Reply with one line, for example
`G1 A, G2 A, G3 B, G4 A+i, G5 A now and B later, G6 A, G7 A, G8 A`, plus any overrides.

## G1. SDK version and legacy protocol support

Context. We are on go-sdk v1.8.0, the newest stable tag. It supports 2026-07-28 and also 2025-11-25,
2025-06-18, 2025-03-26, 2024-11-05 (A§3). v1.9.0-pre.1 is a pre-release. It adds `error="invalid_token"`
to 401 challenges, checks expiry before scopes and removes seven debug flags.

| Option | What it means |
|---|---|
| A | Stay on v1.8.0. Serve 2026-07-28 and older clients through SDK negotiation. |
| B | Move to v1.9.0-pre.1 now. |
| C | Stay on v1.8.0. Restrict to 2026-07-28 with `ServerOptions.SupportedProtocolVersions`. |

Recommendation: **A**. Re-pin when v1.9.0 ships stable.

Consequences.
- A: legacy clients keep working. The stateless handler synthesizes the initialize handshake for them.
  They lose server-initiated streams: GET returns 405, which the 2025-11-25 transport spec permits. We use no
  server-initiated features. Phase 1 proves it with a 2025-11-25 client test.
- A: the challenge gap stays until v1.9.0. 401s lack `error="invalid_token"`. Whether v1.9.0 fixes 403
  `insufficient_scope` is unconfirmed. If not, Phase 2 adds a small header wrapper. Builder's call.
- B: pre-release dependency in a security-sensitive path. Not recommended.
- C: breaks every client older than the 2026-07-28 spec for no gain.

## G2. Stateless mode and server state

Context. `Stateless` must be set explicitly. Today it is false, so 2026-07-28 clients get 400 and `-32022` (A§0 C2).
Our code holds no sessions or caches. Credentials are immutable config (A§2).

| Option | What it means |
|---|---|
| A | `Stateless: true` for the HTTP handler. |
| B | Keep stateful. Not viable: rejects 2026-07-28. |
| C | Two handlers, stateful for legacy and stateless for new, routed by header. |

Recommendation: **A**.

Consequences.
- A: no sessions anywhere. Any instance serves any request. `Mcp-Session-Id` is ignored. Server-initiated
  requests are impossible. We use none.
- A: Phase 2 adds an in-memory discovery and JWKS cache per instance. It is a rebuildable cache, not shared
  state, so it does not break the stateless core.
- C: doubles the transport surface to keep features we do not use. Not recommended.

My calls inside A, veto if you disagree:
- `SetCacheable`: `server/discover` and the four list methods get `ttlMs=300000`, `cacheScope=public`. The tool set is fixed at
  startup and identical for all callers. `resources/read` gets `ttlMs=30000`, `cacheScope=private`, because its
  content comes from oCIS through a credential that G5 may later make per-user.
- `JSONResponse` stays at the SDK default until tests show a reason to change it.
- Resource handlers return `mcp.ResourceNotFoundError` (`-32602`).

## G3. Inbound auth modes for HTTP

Context. Today one shared secret gates `/mcp` and gives no caller identity (A§2).

| Option | What it means |
|---|---|
| A | OIDC only. The secret is removed. |
| B | Explicit, mutually exclusive modes: `oidc`, `secret` (deprecated, warns at startup), `none` (loopback only). |
| C | OIDC and secret accepted at the same time. |

Recommendation: **B**. Remove `secret` in the next major version.

Consequences.
- A: breaking change for every existing HTTP deployment, including `ocis_full/mcp.yml` upstream.
- B: operators migrate on their schedule. One mode per instance keeps the security claim simple.
- C: a leaked shared secret would bypass identity entirely. That undoes the point of OIDC. Not recommended.
- B: if mode is unset it is inferred. Issuer set gives `oidc`, secret set gives `secret`, neither gives `none`.
  Both set is a startup error, matching the existing ambiguity check for outbound auth.
- Non-loopback bind with mode `none` stays refused.

## G4. Token validation

Context. The SDK gives no JWKS, signature, issuer or audience handling (A§3). oCIS ignores `aud` by default
and has no algorithm allow-list on its JWT path (A§5). Keycloak and the built-in IdP both issue JWTs.
Keycloak introspection has an open bug for resource servers (#54296). The built-in IdP cannot mint an
MCP audience (A§5).

### G4.1 Method

| Option | What it means |
|---|---|
| A | Local JWT validation. Keys from the JWKS in issuer discovery. Opaque tokens rejected. |
| B | RFC 7662 introspection. |
| C | Local JWT, with introspection as fallback for opaque tokens. |

Recommendation: **A**. No per-request call to the IdP, and it works under the stateless core.
Cost: revocation is only as fast as token lifetime (5 minutes in both IdPs).

### G4.2 Library (needs your decision, `AGENTS.md` requires an issue before any new dependency)

| Option | What it means |
|---|---|
| i | `golang-jwt/jwt/v5` plus our own discovery and JWKS cache, about 150 lines. Parser options map one to one onto the checks: signature, `iss`, `aud`, `exp`, `nbf`, algorithm allow-list, leeway. Already in the SDK's module graph. |
| ii | `coreos/go-oidc/v3`. Discovery, JWKS caching and rotation are built in. OIDC discovery only, so RFC 8414 fallback is still ours. `nbf` handling needs checking. |
| iii | No library. Hand-rolled JOSE. Not recommended. |

Recommendation: **i**. The security-relevant checks are explicit options and the JWKS cache is small enough to
review line by line. Table-driven tests cover rotation. I will not open the dependency issue in
owncloud/ocis-mcp-server without your OK, since that is outward-facing.

### G4.3 Validation rules (my calls, veto if you disagree)

- Algorithm allow-list: RS256, PS256, ES256. Never `none`, never HS*. Keycloak defaults to RS256 and the
  built-in IdP to PS256.
- Required claims: `iss` (exact match to the configured issuer), `aud`, `exp`, `sub`. `nbf` is checked when present.
  Unlike oCIS, `exp` is mandatory.
- Audience: the token `aud` must contain our canonical resource URI. `azp` is never accepted as a substitute
  (oCIS does accept it). A client-ID audience like `web` would let oCIS-bound tokens in, which is the confused-deputy
  case the spec forbids.
- Canonical resource URI: `<public base URL>` plus `/mcp`, for example `https://mcp.example.com/mcp`. Lowercase scheme and
  host, no trailing slash, no fragment. Needs a public base URL setting because we cannot trust the Host header
  behind a proxy. Comparison uses `oauthex.MatchesResource`.
- Optional audience override setting for IdPs that mint a different but dedicated audience string.
- Clock skew: 60 seconds leeway, constant, no setting.
- Discovery: try OIDC Discovery, then RFC 8414. The discovered `issuer` must equal the configured issuer.
  The `jwks_uri` must be https unless the operator marks the issuer insecure.
- JWKS cache: refresh hourly. Refetch on unknown `kid`, at most once per 60 seconds, fetch timeout 10 seconds,
  singleflight. On fetch failure keep serving last-known-good keys for up to 24 hours, and fail closed for unknown `kid`.
- Token only from the `Authorization: Bearer` header. An `access_token` query parameter is ignored and the
  request gets a 401. Tokens, JWTs and claim bodies are never logged. Only `iss`, `sub` and a redacted `kid` are.
- Issuer TLS trust: a CA file setting rather than skip-verify, so `ocis_full` self-signed certificates can be trusted
  without disabling verification.

### G4.4 Can the built-in IdP be used?

No, not as shipped. It signs verifiable JWTs, but the only audience it can emit is a registered client ID, and
stock oCIS cannot register a resource (A§5). Using `aud=web` would accept tokens meant for oCIS clients.
Recommendation: v1 supports Keycloak and any IdP that can mint a custom audience. The built-in IdP is documented as
unsupported for inbound OIDC. The integration test in Phase 2 therefore runs against Keycloak only.

## G5. Outbound identity

This is the hard one. The spec says we MUST NOT pass the inbound token on, and the upstream token must be a
separate token (A§4). It names no mechanism and does not require or forbid token exchange.

| | (a) One service credential, audit log of verified caller | (b) RFC 8693 token exchange | (c) Audience arrangement oCIS accepts |
|---|---|---|---|
| What oCIS sees | The service user for every caller | The real user | The real user |
| Shared-identity audit gap | Stays. Our own log is the only attribution (`iss`, `sub`, tool, outcome). | Closed. oCIS audit shows the user. | Closed. |
| Per-caller authorization | Only what G6 enforces in our server | oCIS enforces the real user's permissions | oCIS enforces the real user's permissions |
| Spec position | Compliant. Separate credential, no passthrough. | Compliant in intent. Not spec-endorsed, so no "blessed" claim. | I cannot write an honest compliance argument. A multi-audience token is still "the token it received from the MCP client". |
| Built-in IdP | Works | No. It has no RFC 8693. | No usable audience (G4.4). |
| Keycloak 26.2.5 | Works | Standard exchange v2 is confidential-client, internal-to-internal, and `audience` can only narrow. The inbound token may therefore have to carry the oCIS audience already. Needs a spike. | Possible with audience mappers, but still passthrough. |

Recommendation: **(a) now, (b) in Phase 4 as an opt-in for Keycloak deployments after a spike, and never (c).**

Consequences.
- (a): with a service credential that is an admin, every authenticated caller gets admin reach. Docs tell operators to
  use a least-privilege app token and to restrict who gets the MCP scope at the IdP. We say plainly in README that
  oCIS audit logs show one user.
- (b): fixes the audit gap and per-user authorization, but depends on IdP features. Spike first, then decide.
  oCIS must also accept the exchanged token. It does by default because it ignores `aud`.
- (c): rejected. It is the pattern the spec prohibits, and oCIS ignoring `aud` makes it look safe when it is not.
- Not researched in Phase 0, so not offered: service-account impersonation, per-user app tokens. App-token creation
  needs the user's own bearer token, which is itself passthrough.

## G6. Scopes

Context. oCIS does not enforce scopes (A§5). So any scope enforcement here is the only per-caller authorization under G5(a).
The SDK `RequireBearerToken` demands all listed scopes on every request, so per-tool checks need extra code.
Step-up (403 `insufficient_scope`) is a SHOULD. SDK 403s lack `error=`, and a tool handler cannot set an HTTP status.
`Mcp-Name` carries the tool name for 2026-07-28 requests, but legacy 2025-11-25 clients send no such header.

| Option | What it means |
|---|---|
| A | v1 uses one scope. Required to connect, enforced by SDK middleware. No per-category control. |
| B | Three scopes (read, write, admin), enforced per tool, with real step-up at the HTTP layer in v1. |
| C | One connect scope plus read, write, admin enforced per tool, returning a tool error. Step-up comes later. |

Recommendation: **A** for v1.

Consequences.
- A: smallest surface and fully spec-compliant. Operators control reach at the IdP by who may get the scope.
  Authorization per caller equals what the shared secret gives today, plus identity and revocation.
- B: needs our own body or header parsing for both protocol generations, classification of all 80 tools and a 403 wrapper.
  Few MCP clients implement step-up yet. Highest risk for the least certain payoff.
- C: real least privilege without HTTP complexity, but clients see a tool error with no re-auth path.
  Pick this if you want least privilege in the first release.
- Under A or C the scope name is a setting with a default, see G7. Under C, a test fails when any registered tool lacks a category.

## G7. Config naming

Today `OCIS_MCP_AUTH_MODE=oidc` means a static outbound token. `OCIS_MCP_OIDC_ISSUER`, `_CLIENT_ID` and `_CLIENT_SECRET` are read and
ignored (A§0 C4). An operator who set the issuer may believe inbound is protected.

| Option | What it means |
|---|---|
| A | Name by direction. Inbound under `OCIS_MCP_HTTP_*`. Outbound under `OCIS_MCP_OCIS_*`. Old names stay as deprecated aliases. |
| B | Add inbound names only. Leave outbound names as they are. |
| C | Clean break. No aliases. |

Recommendation: **A**.

Proposed names under A:

| Direction | New name | Replaces or adds |
|---|---|---|
| Inbound | `OCIS_MCP_HTTP_AUTH` (`oidc`, `secret`, `none`) | new |
| Inbound | `OCIS_MCP_HTTP_PUBLIC_URL` | new, canonical base URL |
| Inbound | `OCIS_MCP_HTTP_OIDC_ISSUER` | new |
| Inbound | `OCIS_MCP_HTTP_OIDC_AUDIENCE` | new, optional override |
| Inbound | `OCIS_MCP_HTTP_OIDC_SCOPES` | new, default one scope (name to confirm, proposal `ocis:mcp`) |
| Inbound | `OCIS_MCP_HTTP_OIDC_CA_FILE` | new |
| Inbound | `OCIS_MCP_HTTP_SECRET` | stays, deprecated |
| Outbound | `OCIS_MCP_OCIS_AUTH_MODE` (`app-token`, `static-bearer`, `none`) | replaces `OCIS_MCP_AUTH_MODE` (`oidc` value maps to `static-bearer`) |
| Outbound | `OCIS_MCP_OCIS_APP_TOKEN_USER`, `_VALUE` | replace `OCIS_MCP_APP_TOKEN_*` |
| Outbound | `OCIS_MCP_OCIS_BEARER_TOKEN` | replaces `OCIS_MCP_OIDC_ACCESS_TOKEN` |
| Dead | `OCIS_MCP_OIDC_ISSUER`, `_CLIENT_ID`, `_CLIENT_SECRET` | never had effect |

Rules.
- Old names keep working and log one deprecation warning each at startup. The new name wins when both are set.
- The three dead variables log a warning saying they have no effect and, for the issuer, pointing at `OCIS_MCP_HTTP_OIDC_ISSUER`.
- No setting value or secret appears in any warning.
- stdio ignores all `OCIS_MCP_HTTP_*` settings. It keeps environment credentials, per the spec.

## G8. Version and CHANGELOG framing

Context. 2026-07-28 clients cannot use the HTTP transport today. Moving to stateless changes HTTP behavior: no session IDs,
GET and DELETE return 405. OIDC inbound is additive. G3 deprecates and does not remove.

| Option | What it means |
|---|---|
| A | One release, v1.2.0. Sections: Fixed (2026-07-28 support), Changed (HTTP transport), Added (OIDC inbound), Deprecated (names and shared secret). |
| B | Split. Phase 1 as v1.2.0, Phase 2 as v1.3.0. |
| C | v2.0.0 because of the HTTP behavior change. |

Recommendation: **A**, with a prominent "HTTP transport behavior" note. v2.0.0 is reserved for removing the shared secret
and the old variable names. Tagging is the maintainers' call. We only write the CHANGELOG entry.

## Also flagged, my calls (veto if you disagree)

- Add `ReadHeaderTimeout` and related timeouts to `http.Server` (A§9). Cheap, and needed once the endpoint is internet-facing.
- Keep the SDK localhost and cross-origin protections. Check non-loopback coverage in Phase 1 and add `http.CrossOriginProtection` if the SDK does not cover it.
- Add a `reuse` Makefile target and a CI step, so the REUSE gate you listed is runnable.
- Roots, Sampling and Logging stay untouched (out of scope). The SDK still advertises `logging` by default.
- The conformance harness is tried in Phase 1. If it cannot target our server, I report that and rely on in-process tests.

## What I need

One line per gate. Open items only you can answer: G4.2 library choice, whether I may open the dependency issue,
G6 (A or C), and whether Phase 4 should be planned as part of this effort.
