# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- 80 MCP tools across 13 categories: users, groups, spaces, files, shares,
  search, notifications, settings, app tokens, admin, education, OCM, and
  multi-step workflows
- 5 MCP resources: capabilities, version, sharing-roles, drive-types, auth-mode
- 4 MCP prompts: onboard-user, migrate-files, audit-space, share-report
- Dual transport support: stdio (Claude Desktop / Claude Code) and HTTP
  (network deployments on port 8090)
- Authentication via app tokens (Basic auth) and OIDC (Bearer tokens) with
  auto-detection
- Rate-limit handling with automatic retry (HTTP 429)
- Input validation: path traversal prevention, ID checks, limit clamping
- Destructive operations require explicit `confirm=true` parameter
- Docker multi-stage build (golang:1.26-alpine / alpine:3.23)
- GoReleaser configuration for cross-platform releases (linux/darwin/windows,
  amd64/arm64)
- GitHub Actions CI: build, test, lint, coverage threshold (70%)
- Dependabot for automated dependency updates (actions, Go modules, Docker)
- Getting Started guide for connecting Claude Desktop and Ollama on
  Mac, Windows, and Linux
- Interactive `install.sh` setup script with OS detection

[Unreleased]: https://github.com/owncloud/ocis-mcp-server/commits/main/
