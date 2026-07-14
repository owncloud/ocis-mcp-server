# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.1.0] - 2026-07-14

### Added

- **HTTP transport authentication.** The HTTP transport now enforces a Bearer
  secret (`OCIS_MCP_HTTP_SECRET`) on `/mcp`, closing a gap where any client
  able to reach the listener could invoke every tool under the server's oCIS
  identity. Binding to a non-loopback address without a secret now fails fast
  at startup instead of serving unauthenticated; a loopback bind without a
  secret remains allowed for local/dev use (with a warning logged). `stdio`
  is unaffected. (#29)

### Fixed

- **Four tools rejected by oCIS 8.1.0-rc.1** — `ocis_invite_to_space` /
  `ocis_upload_and_share` (and `ocis_create_project_space`, which invites
  members) omitted the required `@libre.graph.recipient.type` on invite
  recipients; `ocis_add_group_member` sent a bare user ID instead of a full
  URL for `@odata.id`; `ocis_get_resource_by_id` double-prefixed the dav path
  for full resource IDs; `ocis_list_assignments` omitted the required
  `account_uuid`. All four request shapes now match oCIS 8.1.0-rc.1's
  validation, with regression tests added for each. (#26)
- **Root Apache-2.0 `LICENSE` text** — corrected fabricated and drifted
  wording in the Appendix boilerplate so the file matches the canonical
  SPDX Apache-2.0 text exactly. (#37)

### Compatibility

- Go 1.26+
- oCIS 8.x (tested against 8.0.1 and 8.1.0-rc.1)
- MCP via `github.com/modelcontextprotocol/go-sdk` v1.6.1

## [1.0.0] - 2026-06-14

First stable release of the **ownCloud Infinite Scale (oCIS) MCP Server** — a
[Model Context Protocol](https://modelcontextprotocol.io) server that exposes
oCIS administration and collaboration as **tools**, **resources**, and
**prompts** for AI assistants such as Claude Desktop, Claude Code, Ollama, and
any MCP-compatible client.

Because this is the initial release, the entries below describe the full feature
set rather than a delta from a previous version.

### Added

#### MCP tools (80, across 13 categories)

- **Users (6)** — `ocis_list_users`, `ocis_get_user`, `ocis_create_user`,
  `ocis_update_user`, `ocis_delete_user`, `ocis_get_me`
- **Groups (7)** — `ocis_list_groups`, `ocis_get_group`, `ocis_create_group`,
  `ocis_update_group`, `ocis_delete_group`, `ocis_add_group_member`,
  `ocis_remove_group_member`
- **Spaces / drives (14)** — `ocis_list_spaces`, `ocis_list_my_spaces`,
  `ocis_get_space`, `ocis_create_space`, `ocis_update_space`,
  `ocis_disable_space`, `ocis_delete_space`, `ocis_restore_space`,
  `ocis_invite_to_space`, `ocis_create_space_link`,
  `ocis_list_space_permissions`, `ocis_empty_trashbin`, `ocis_set_space_image`,
  `ocis_set_space_readme`
- **Files & folders (14)** — `ocis_list_files`, `ocis_get_file_info`,
  `ocis_create_folder`, `ocis_upload_file`, `ocis_download_file`,
  `ocis_move_file`, `ocis_copy_file`, `ocis_delete_file`,
  `ocis_get_file_versions`, `ocis_restore_file_version`,
  `ocis_get_resource_by_id`, `ocis_tag_resource`, `ocis_untag_resource`,
  `ocis_get_resource_metadata`
- **Shares (11)** — `ocis_create_share`, `ocis_create_link`, `ocis_list_shares`,
  `ocis_update_share`, `ocis_update_share_expiration`, `ocis_delete_share`,
  `ocis_list_shared_by_me`, `ocis_list_received_shares`, `ocis_accept_share`,
  `ocis_reject_share`, `ocis_get_sharing_roles`
- **Search (2)** — `ocis_search`, `ocis_search_by_tag`
- **Notifications (2)** — `ocis_list_notifications`, `ocis_delete_notification`
- **Settings / roles (3)** — `ocis_list_roles`, `ocis_assign_role`,
  `ocis_list_assignments`
- **App tokens (3)** — `ocis_list_app_tokens`, `ocis_create_app_token`,
  `ocis_delete_app_token`
- **Admin (4)** — `ocis_health_check`, `ocis_get_version`,
  `ocis_get_capabilities`, `ocis_get_config`
- **Education (5)** — `ocis_list_education_schools`,
  `ocis_get_education_school`, `ocis_list_education_users`,
  `ocis_get_education_user`, `ocis_create_education_user`
- **OCM / federated sharing (4)** — `ocis_ocm_list_providers`,
  `ocis_ocm_create_share`, `ocis_ocm_list_shares`, `ocis_ocm_list_received`
- **Multi-step workflows (5)** — `ocis_upload_and_share`,
  `ocis_create_project_space`, `ocis_find_and_download`,
  `ocis_share_with_link`, `ocis_get_space_overview` (each orchestrates several
  underlying operations into a single call)

#### MCP resources (5)

Read-only context an assistant can pull on demand:

- `ocis://capabilities` — the instance's advertised capabilities
- `ocis://version` — the oCIS server version
- `ocis://sharing-roles` — available sharing role definitions and their IDs
- `ocis://drive-types` — supported drive/space types
- `ocis://auth-mode` — the authentication mode the server is configured for

#### MCP prompts (4)

Guided, multi-step prompt templates: `onboard-user`, `migrate-files`,
`audit-space`, `share-report`.

#### Transports & connectivity

- **stdio transport** for Claude Desktop / Claude Code (subprocess integration).
- **HTTP transport** (default port `8090`) for networked deployments.

#### Authentication

- **App tokens** (HTTP Basic auth) and **OIDC** (Bearer tokens), with automatic
  mode detection based on the provided configuration.

#### Safety & robustness

- Destructive operations (delete user/group/space/file/share, empty trashbin,
  reject share, …) require an explicit `confirm=true` parameter.
- Tool annotations classify each tool as read-only, mutating, idempotent, or
  destructive so clients can reason about side effects.
- Input validation: path-traversal prevention, ID validation, and result-limit
  clamping (default 50, max 200).
- Automatic retry with `Retry-After` handling on rate limiting (HTTP 429).

#### Packaging, operations & documentation

- Docker multi-stage build (`golang:1.26-alpine` → `alpine:3.23`).
- GoReleaser cross-platform release builds: linux / darwin / windows ×
  amd64 / arm64, with checksums.
- Optional macOS code signing and notarization in the release pipeline
  (engaged only when the Apple signing secrets are configured).
- GitHub Actions CI: build, test (race detector), lint (golangci-lint), and a
  70% coverage threshold.
- Dependabot for GitHub Actions, Go modules, and Docker base images.
- Getting Started guide for Claude Desktop and Ollama on macOS, Windows, and
  Linux, plus an interactive `install.sh` setup script with OS detection.

### Fixed

Fixes made since the `v1.0.0-beta` pre-release:

- **Search** — `ocis_search` now wraps wildcard-free patterns in `*…*` so a
  plain term (e.g. `elmo`) matches as a substring (e.g. `burning_elmo.gif`),
  consistent with the web UI. Patterns that already use `*`/`?` are passed
  through unchanged. (#21)
- **Sharing** — `ocis_create_share` now sends the required
  `@libre.graph.recipient.type` on each recipient, fixing the `HTTP 400`
  ("Field validation for 'LibreGraphRecipientType' failed") that previously
  made every invite fail. (#22)
- **Release pipeline** — fixed a `release.yml` startup failure caused by using
  the `secrets` context in a job-level `if:` (not permitted by GitHub Actions),
  which had broken the Release workflow. (#23)

### Known issues & limitations

- **`ocis_create_share` and `ocis_invite_to_space` may return an empty
  `permissions` list even when the share/invite succeeds.** The share *is*
  created (the `HTTP 200` from oCIS is honored); only the returned object is
  parsed from the wrong response envelope (`{"permissions": […]}` instead of
  the `{"value": […]}` that oCIS actually returns). Verified against oCIS
  8.0.1. The fix is in progress as part of the SDK migration (#13, #24).
- **`ocis_get_sharing_roles` is not yet SDK-backed.** It uses a direct HTTP
  call because the generated `libre-graph-api-go` SDK types the
  `roleDefinitions` list response as a single object rather than an array
  (upstream fix: owncloud/libre-graph-api#212). The tool works correctly today.
- **Public links require a password when the oCIS instance enforces a password
  policy.** `ocis_create_link` and `ocis_create_space_link` will return
  `HTTP 400` ("password protection is enforced") if a link is created without a
  `password` on such instances — supply a `password`.
- **Space invites need space-scoped role IDs.** `ocis_invite_to_space` expects
  role IDs whose condition is `exists @Resource.Root`, which differ from the
  file/folder sharing role IDs. Use `ocis_get_sharing_roles` and pick the role
  that matches the resource type (space vs. file).
- **`ocis_accept_share`** is performed via a direct API call; there is no
  dedicated SDK/Graph action method for accepting a received share.
- **Pagination is offset/limit based** for list tools (e.g.
  `ocis_list_spaces`, `ocis_list_shared_by_me`); a single call returns at most
  `limit` results (default 50, max 200) — page with `offset`.
- **Full-text / content search requires the oCIS Search service** (and Tika for
  content extraction) to be enabled on the server. Without it, `ocis_search`
  matches on file names only.
- **OIDC tokens are not auto-refreshed.** When using OIDC, a valid bearer token
  must be supplied; the server does not perform a refresh on expiry.
- **Transport layer is hand-rolled.** The server currently constructs LibreGraph
  HTTP requests directly; migrating to the generated `libre-graph-api-go` SDK
  (for maintained paths/types) is tracked in #13.
- **Tested against oCIS 8.0.1.** Other oCIS versions may expose API differences.

### Compatibility

- Go 1.26+
- oCIS 8.x (tested against 8.0.1)
- MCP via `github.com/modelcontextprotocol/go-sdk` v1.6.1

[1.1.0]: https://github.com/owncloud/ocis-mcp-server/releases/tag/v1.1.0
[1.0.0]: https://github.com/owncloud/ocis-mcp-server/releases/tag/v1.0.0
[Unreleased]: https://github.com/owncloud/ocis-mcp-server/compare/v1.1.0...HEAD
