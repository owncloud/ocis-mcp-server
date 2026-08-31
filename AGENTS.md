# AGENTS.md -- oCIS MCP Server

## Repository Overview

Standalone MCP (Model Context Protocol) server for oCIS, written in Go. Exposes 80+ AI tools via LibreGraph, WebDAV, and OCS APIs. Licensed under Apache-2.0.

## Architecture & Key Paths

- `internal/` -- Server implementation and tool definitions
- `Makefile` -- Build, test, and lint automation
- `Dockerfile` -- Docker image build
- `go.mod` / `go.sum` -- Go module definition
- `GETTING_STARTED.md` -- Setup guide
- `evaluation.xml` -- Evaluation configuration
- `install.sh` -- Setup script

## Development Conventions

- Go codebase with minimal dependency tree
- MCP protocol (stdio/HTTP transport)
- No oCIS Go imports -- external client only

## Build & Test Commands

```bash
make build                    # Build the binary
make test                     # Run tests
make lint                     # Run linter
make cover                    # Generate coverage report
make docker-build             # Build Docker image
make clean                    # Clean build artifacts
```

## Important Constraints

- Licensed under Apache-2.0 (already at the OSPO target license). The broader ownCloud organization is migrating other repositories from copyleft licenses to Apache 2.0.
- No dependency on oCIS internals -- communicates only via public APIs.
- All contributions require a DCO sign-off.


## OSPO Policy Constraints

### GitHub Actions
- **Only** use actions owned by `owncloud`, created by GitHub (`actions/*`), verified on the GitHub Marketplace, or verified by the ownCloud Maintainers.
- Pin all actions to their full commit SHA (not tags): `uses: actions/checkout@<SHA> # vX.Y.Z`
- Never introduce actions from unverified third parties.

### Dependency Management
- Dependabot is configured for automated dependency updates.
- Review and merge Dependabot PRs as part of regular maintenance.
- Do not introduce new dependencies without discussion in an issue first.

### Git Workflow
- **Rebase policy**: Always rebase; never create merge commits. Use `git pull --rebase` and `git rebase` before pushing.
- **Signed commits**: All commits **must** be PGP/GPG signed (`git commit -S -s`).
- **DCO sign-off**: Every commit needs a `Signed-off-by` line (`git commit -s`).
- **Conventional Commits & Squash Merge**: Use the [Conventional Commits](https://www.conventionalcommits.org/) format where the repository enforces it. Many repos use squash merge, where the PR title becomes the commit message on the default branch — apply Conventional Commits format to PR titles as well. A reusable GitHub Actions workflow enforces this.

## Context for AI Agents

This server implements the MCP protocol to expose oCIS operations as AI-accessible tools. It uses LibreGraph API for user/group/drive management, WebDAV for file operations, and OCS for sharing. The `internal/` directory contains all tool implementations.
