# oCIS MCP Server

<!-- OSPO-managed README | Generated: 2026-04-16 | v2 -->

[![License](https://img.shields.io/badge/License-Apache--2.0-blue.svg)](LICENSE) [![ownCloud OSPO](https://img.shields.io/badge/OSPO-ownCloud-blue)](https://kiteworks.com/opensource) [![Docker Hub](https://img.shields.io/docker/pulls/owncloud)](https://hub.docker.com/r/owncloud/ocis-mcp-server)

A standalone Model Context Protocol (MCP) server written in Go that exposes ownCloud Infinite Scale as a set of 80+ AI-accessible tools. It enables AI assistants such as Claude, Ollama, and other MCP-compatible clients to manage oCIS users, groups, spaces, files, shares, and more through natural language, communicating exclusively over the public oCIS APIs (LibreGraph, WebDAV, OCS).

## Getting Started

See the **[Getting Started Guide](GETTING_STARTED.md)** for a full walkthrough.

### Quick Setup

```bash
# Build
make build

# Configure
export OCIS_URL=https://your-ocis-instance.example.com
export OCIS_ACCESS_TOKEN=your-token

# Run
./ocis-mcp-server
```

### Docker

```bash
make docker-build
docker run -e OCIS_URL=... -e OCIS_ACCESS_TOKEN=... owncloud/ocis-mcp-server
```

### Run Tests

```bash
make test
make lint
```

## Documentation

- [Getting Started Guide](GETTING_STARTED.md)
- [MCP Protocol](https://modelcontextprotocol.io/)
- [oCIS Developer Documentation](https://owncloud.dev/)

## Part of ownCloud Infinite Scale

This MCP server is an external integration for [oCIS](https://github.com/owncloud/ocis). It communicates over the public API surface and has no dependency on oCIS internals, allowing independent versioning and release cycles.

[![Docker Hub](https://img.shields.io/docker/v/owncloud/ocis-mcp-server?label=Docker%20Hub&logo=docker&sort=semver)](https://hub.docker.com/r/owncloud/ocis-mcp-server)

## Tool Inventory

The server exposes 80 tools in 13 categories:

| Category | Tools | Examples |
|---|---|---|
| **Users** | 6 | `ocis_list_users`, `ocis_create_user`, `ocis_get_me` |
| **Groups** | 7 | `ocis_list_groups`, `ocis_create_group`, `ocis_add_group_member` |
| **Spaces** | 14 | `ocis_list_spaces`, `ocis_create_space`, `ocis_invite_to_space` |
| **Files** | 14 | `ocis_list_files`, `ocis_upload_file`, `ocis_download_file`, `ocis_move_file` |
| **Shares** | 11 | `ocis_create_share`, `ocis_create_link`, `ocis_list_shared_by_me` |
| **Search** | 2 | `ocis_search`, `ocis_search_by_tag` |
| **Notifications** | 2 | `ocis_list_notifications`, `ocis_delete_notification` |
| **Settings** | 3 | `ocis_list_roles`, `ocis_assign_role` |
| **App Tokens** | 3 | `ocis_list_app_tokens`, `ocis_create_app_token` |
| **Admin** | 4 | `ocis_health_check`, `ocis_get_version`, `ocis_get_capabilities` |
| **Education** | 5 | `ocis_list_education_schools`, `ocis_create_education_user` |
| **OCM** | 4 | `ocis_ocm_create_share`, `ocis_ocm_list_received` |
| **Workflows** | 5 | `ocis_upload_and_share`, `ocis_create_project_space` |

### Authentication to oCIS (outbound)

**App tokens (recommended):** create one in the oCIS web UI under Settings > Security > App tokens.

```bash
export OCIS_MCP_OCIS_URL="https://ocis.example.com"
export OCIS_MCP_OCIS_APP_TOKEN_USER="admin"
export OCIS_MCP_OCIS_APP_TOKEN_VALUE="<token>"
```

**Static bearer token (alternative):**

```bash
export OCIS_MCP_OCIS_AUTH_MODE="static-bearer"
export OCIS_MCP_OCIS_BEARER_TOKEN="<access-token>"
```

The token is read once at startup. This is an outbound credential only. It is not OIDC login.

### Key Environment Variables

| Variable | Required | Description |
|---|---|---|
| `OCIS_MCP_OCIS_URL` | Yes | Base URL of the oCIS instance |
| `OCIS_MCP_OCIS_AUTH_MODE` | No | `app-token`, `static-bearer` or `none`. Inferred from the credentials set. An unknown value is a startup error. |
| `OCIS_MCP_TRANSPORT` | No | `stdio` (default) or `http` |
| `OCIS_MCP_HTTP_ADDR` | No | Listen address for HTTP transport (default `127.0.0.1:8090`) |
| `OCIS_MCP_HTTP_AUTH` | No | Inbound auth for HTTP: `oidc`, `secret` (deprecated) or `none`. Inferred when unset. |
| `OCIS_MCP_HTTP_PUBLIC_URL` | oidc | Public origin of this server, for example `https://mcp.example.com`. No path. |
| `OCIS_MCP_HTTP_OIDC_ISSUER` | oidc | Issuer URL, exactly as the IdP advertises it |
| `OCIS_MCP_HTTP_OIDC_SCOPES` | No | Required scopes, space or comma separated (default `ocis:mcp`) |
| `OCIS_MCP_HTTP_OIDC_CA_FILE` | No | PEM file with an extra CA for the issuer, for self-signed IdPs |
| `OCIS_MCP_HTTP_SECRET` | secret | Shared secret for the deprecated `secret` mode |
| `OCIS_MCP_LOG_LEVEL` | No | `debug`, `info`, `warn`, `error` |

Renamed settings keep working and log one deprecation warning each:

| Old name | New name |
|---|---|
| `OCIS_MCP_APP_TOKEN_USER` | `OCIS_MCP_OCIS_APP_TOKEN_USER` |
| `OCIS_MCP_APP_TOKEN_VALUE` | `OCIS_MCP_OCIS_APP_TOKEN_VALUE` |
| `OCIS_MCP_AUTH_MODE` | `OCIS_MCP_OCIS_AUTH_MODE` (the value `oidc` maps to `static-bearer`) |
| `OCIS_MCP_OIDC_ACCESS_TOKEN` | `OCIS_MCP_OCIS_BEARER_TOKEN` |

`OCIS_MCP_OIDC_ISSUER`, `OCIS_MCP_OIDC_CLIENT_ID` and `OCIS_MCP_OIDC_CLIENT_SECRET` never had an effect.
They are ignored with a warning. For inbound OIDC use `OCIS_MCP_HTTP_OIDC_ISSUER`.

### Securing the HTTP transport

The HTTP transport implements the MCP 2026-07-28 stateless transport. In `oidc` mode it is an
OAuth 2.1 resource server: it validates access tokens from your IdP locally and publishes
protected resource metadata (RFC 9728) at `/.well-known/oauth-protected-resource/mcp`.

Minimal setup:

```bash
export OCIS_MCP_TRANSPORT=http
export OCIS_MCP_HTTP_ADDR=0.0.0.0:8090
export OCIS_MCP_HTTP_AUTH=oidc
export OCIS_MCP_HTTP_PUBLIC_URL=https://mcp.example.com
export OCIS_MCP_HTTP_OIDC_ISSUER=https://idp.example.com/realms/main
```

Requirements for the IdP and tokens:

- The issuer must match the IdP's advertised `issuer` exactly, and the IdP must be reachable at startup.
  The issuer and its `jwks_uri` must use https. Plain http is accepted for loopback only. Use
  `OCIS_MCP_HTTP_OIDC_CA_FILE` for a private CA.
- The access token `aud` must contain `<public-url>/mcp` (RFC 8707). In Keycloak add an audience
  mapper with that custom audience. The `azp` claim is not accepted instead of `aud`.
- The token needs the scope `ocis:mcp` (or your `OCIS_MCP_HTTP_OIDC_SCOPES`). Both the `scope`
  string and the `scp` array are read.
- Tokens must be signed with RS256, PS256 or ES256, carry a `kid`, `sub` and `exp`, and be at most
  8 KiB. Large group claims in Keycloak or Azure can exceed that, so trim them.
- Signing keys are cached for up to an hour. An unknown `kid` triggers one refetch, at most once per
  60 seconds, so a rotated key can take up to a minute to be accepted. Revocation takes effect when
  the token expires, so keep access token lifetimes short.
- A missing or wrong token gets `401` with a `WWW-Authenticate` challenge that carries the metadata
  URL and scope. A token without the required scope gets `403`. The SDK does not add an `error=`
  parameter to these challenges.

Behind a reverse proxy:

- In `oidc` mode the server accepts requests whose `Host` is the public hostname (any port) or a
  loopback name, and refuses others with `403`. Browsers on another origin get `403`. Only the
  public origin is trusted.
- Terminate TLS at the proxy and restrict the network. Do not expose `/mcp` to the internet without it.

Other modes:

- `secret` is deprecated. `OCIS_MCP_HTTP_SECRET` is a shared bearer secret. Prefer `oidc`.
- `none` disables auth and is allowed on loopback only. Never put a proxy in front of a `none`
  server, because the server cannot tell proxied callers from local ones.
- Without `OCIS_MCP_HTTP_AUTH`, the mode is `oidc` when an issuer is set, `secret` when only a secret
  is set, and `none` otherwise (loopback only, with a warning). Setting both is a startup error.
- **Prefer `stdio`** (the default) for single-client, local use such as Claude Desktop. It has no
  network listener.

Limitations:

- **Every authenticated caller acts as the same oCIS user.** Inbound auth decides who may call the
  server. It does not change which oCIS identity runs the tools: that is the one outbound credential
  configured on the server. If it is an admin app token, every authorized caller has admin access.
  The audit log (`iss`, `sub`, tool, outcome per `tools/call`, at Info level) is the only per-caller
  record. It is hidden when `OCIS_MCP_LOG_LEVEL` is `warn` or higher.
- **Credential rotation requires a restart.** The outbound token is read once at startup. If it
  expires or is revoked, every tool call fails until the process restarts with a fresh one.
- The transport is stateless. There is no session and no server-initiated GET stream.
- No MCP conformance claim is made.

### Running the integration test

`make test-integration` starts a real Keycloak 26.2.5 container with Docker and checks a valid token,
a wrong audience, a missing scope and a missing token. It needs Docker and network access for the
first image pull. The realm file contains a dummy client secret that is not for use anywhere else.

### MCP Resources and Prompts

5 read-only resources (`ocis://capabilities`, `ocis://version`, `ocis://sharing-roles`, `ocis://drive-types`, `ocis://auth-mode`) and 4 guided workflow prompts (`ocis_onboard_user`, `ocis_migrate_files`, `ocis_audit_space`, `ocis_share_report`).

## Community & Support

**[Star](https://github.com/owncloud/ocis-mcp-server)** this repo and **Watch** for release notifications!

- [ownCloud Website](https://owncloud.com)
- [Community Discussions](https://github.com/orgs/owncloud/discussions)
- [Matrix Chat](https://app.element.io/#/room/#owncloud:matrix.org)
- [Documentation](https://doc.owncloud.com)
- [Enterprise Support](https://owncloud.com/contact-us/)
- [OSPO Home](https://kiteworks.com/opensource)

## Contributing

We welcome contributions! Please read the [Contributing Guidelines](CONTRIBUTING.md)
and our [Code of Conduct](CODE_OF_CONDUCT.md) before getting started.

### Workflow

- **Rebase Early, Rebase Often!** We use a rebase workflow. Always rebase on the target branch before submitting a PR.
- **Dependabot**: Automated dependency updates are managed via Dependabot. Review and merge dependency PRs promptly.
- **Signed Commits**: All commits **must** be PGP/GPG signed. See [GitHub's signing guide](https://docs.github.com/en/authentication/managing-commit-signature-verification).
- **DCO Sign-off**: Every commit must carry a `Signed-off-by` line:
  ```
  git commit -s -S -m "your commit message"
  ```
- **GitHub Actions Policy**: Workflows may only use actions that are (a) owned by `owncloud`, (b) created by GitHub (`actions/*`), or (c) verified in the GitHub Marketplace.

## Security

**Do not open a public GitHub issue for security vulnerabilities.**

Report vulnerabilities at **<https://security.owncloud.com>** -- see [SECURITY.md](SECURITY.md).

Bug bounty: [YesWeHack ownCloud Program](https://yeswehack.com/programs/owncloud-bug-bounty-program)

## License

This project is licensed under the [Apache-2.0](LICENSE).

## About the ownCloud OSPO

The [Kiteworks Open Source Program Office](https://kiteworks.com/opensource), operating under
the [ownCloud](https://owncloud.com) brand, launched on May 5, 2026, to steward the open source
ecosystem around ownCloud's products. The OSPO ensures transparent governance, license compliance,
community health, and sustainable collaboration between the open source community and
[Kiteworks](https://www.kiteworks.com), which acquired ownCloud in 2023.

- **OSPO Home**: <https://kiteworks.com/opensource>
- **GitHub**: <https://github.com/owncloud>
- **ownCloud**: <https://owncloud.com>

For questions about the OSPO or licensing, contact ospo@kiteworks.com.

> **License status:** This repository is already licensed under Apache-2.0 -- the OSPO target license.
> No migration is required.
