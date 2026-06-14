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

### Authentication

**App Tokens (recommended):** Create in the oCIS web UI under Settings > Security > App tokens.

```bash
export OCIS_MCP_OCIS_URL="https://ocis.example.com"
export OCIS_MCP_APP_TOKEN_USER="admin"
export OCIS_MCP_APP_TOKEN_VALUE="<token>"
```

**OIDC (alternative):**

```bash
export OCIS_MCP_AUTH_MODE="oidc"
export OCIS_MCP_OIDC_ACCESS_TOKEN="<access-token>"
```

### Key Environment Variables

| Variable | Required | Description |
|---|---|---|
| `OCIS_MCP_OCIS_URL` | Yes | Base URL of the oCIS instance |
| `OCIS_MCP_TRANSPORT` | No | `stdio` (default) or `http` |
| `OCIS_MCP_HTTP_ADDR` | No | Listen address for HTTP transport (default `127.0.0.1:8090`) |
| `OCIS_MCP_LOG_LEVEL` | No | `debug`, `info`, `warn`, `error` |

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
