# ocis-mcp-server Docker Compose + Docker Hub Publishing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make this repo (`owncloud/ocis-mcp-server`) runnable via `docker-compose up`, and add a GitHub Actions workflow that builds and publishes the `owncloud/ocis-mcp-server` image to Docker Hub — closing the gap flagged in `owncloud/ocis` PR [#12603](https://github.com/owncloud/ocis/pull/12603), where the `ocis_full` deployment example references this image but it doesn't exist on any registry yet.

**Architecture:** Add a `docker-compose.yml` + `.env.example` at the repo root for local dev/testing (HTTP transport, port 8090 published, config from `.env`). Add `.github/workflows/docker.yml` with two jobs: `build` (runs on every push and PR — multi-arch build validation plus a local single-arch build+smoke-test, no registry credentials needed) and `publish` (runs only on push to `main`, on `v*` tags, or manual `workflow_dispatch` — logs into Docker Hub and pushes multi-arch images). Fix a pre-existing bug in `README.md` where the Quick Setup/Docker sections use the wrong environment variable names, while we're already touching that section to document Docker Compose.

**Tech Stack:** Docker Compose (Compose Spec, `docker-compose` v2 CLI), Docker Buildx + QEMU (multi-arch: `linux/amd64`, `linux/arm64`), GitHub Actions (`docker/setup-qemu-action`, `docker/setup-buildx-action`, `docker/login-action`, `docker/metadata-action`, `docker/build-push-action` — all Docker-owned, marketplace-verified actions).

## Global Constraints

- Repository: `/Users/david.walter/Documents/Code/oCIS-Bughunter/ocis-mcp-server` (remote `origin` = `owncloud/ocis-mcp-server`, remote `fork` = `dj4oC/ocis-mcp-server`). Default branch is `main`.
- Commits in this repo are GPG-signed (`commit.gpgsign=true`, a signing key is already configured) — do not disable signing.
- This repo's own `.github/workflows/*` policy (stated in `README.md` under Contributing > Workflow) requires every GitHub Action used to be (a) owned by `owncloud`, (b) created by GitHub (`actions/*`), or (c) verified in the GitHub Marketplace. `docker/setup-qemu-action`, `docker/setup-buildx-action`, `docker/login-action`, `docker/metadata-action`, and `docker/build-push-action` are all official, marketplace-verified Docker Inc. actions — allowed under (c).
- Existing workflows in this repo pin every action to a full commit SHA with a `# vX.Y.Z` comment (see `.github/workflows/ci.yml`, `.github/workflows/release.yml`). Match that convention exactly — every SHA used in this plan was resolved from the real upstream tag via `git ls-remote --tags`, not guessed.
- This machine's docker daemon is Colima; the working compose CLI is the hyphenated standalone binary `docker-compose` (not `docker compose`). This machine has no local `docker buildx` — multi-arch build verification in this plan relies on real GitHub Actions CI runs (triggered by opening the PR), not local dry runs.
- Do not touch `Dockerfile` — it already cross-compiles correctly for any target platform buildx selects (no `GOARCH` is hardcoded, so it picks up the builder container's native arch under `--platform`), no change is needed there.
- Do not fabricate Docker Hub credentials or attempt to configure the `DOCKERHUB_USERNAME`/`DOCKERHUB_TOKEN` repository secrets yourself — that is a manual, human-only step requiring real owncloud-org Docker Hub credentials, called out explicitly in Task 4.
- Keep the change minimal: no changes to `Dockerfile`, `.goreleaser.yml`, `cmd/`, or `internal/` — this plan is 100% Docker Compose + CI + docs.

---

### Task 1: Add `docker-compose.yml` and `.env.example` for local dev

**Files:**
- Create: `docker-compose.yml`
- Create: `.env.example`
- Modify: `.gitignore`

**Interfaces:**
- Produces: a `docker-compose.yml` service named `ocis-mcp-server` that later tasks (README docs in Task 2) reference by name and by its published port (`8090`, path `/mcp`).
- Consumes: the `owncloud/ocis-mcp-server` container's existing env var contract from `internal/config/config.go` (`OCIS_MCP_OCIS_URL`, `OCIS_MCP_HTTP_SECRET`, `OCIS_MCP_APP_TOKEN_USER`, `OCIS_MCP_APP_TOKEN_VALUE`, `OCIS_MCP_TRANSPORT`, `OCIS_MCP_HTTP_ADDR`, `OCIS_MCP_LOG_LEVEL`, `OCIS_MCP_TLS_SKIP_VERIFY`, `OCIS_MCP_INSECURE`) — do not rename any of these, they are fixed by existing code.

- [ ] **Step 1: Create the feature branch off a clean, up-to-date `main`**

```bash
cd /Users/david.walter/Documents/Code/oCIS-Bughunter/ocis-mcp-server
git fetch origin
git status --porcelain
```

Expected: no output (or only pre-existing untracked noise like `.DS_Store`). Do not proceed
if it shows anything else.

```bash
git checkout -b feat/docker-compose-and-hub-publish origin/main
```

Expected: `Switched to a new branch 'feat/docker-compose-and-hub-publish'`. All commits in
this plan (Tasks 1-3) land on this branch, never directly on `main`.

- [ ] **Step 2: Confirm `.env` is not already git-ignored (write the failing check)**

```bash
git check-ignore -q .env && echo "already ignored" || echo "NOT ignored"
```

Expected: `NOT ignored` (the current `.gitignore` has no `.env` entry).

- [ ] **Step 3: Add `.env` to `.gitignore`**

Use the Edit tool on `.gitignore`. Exact `old_string`:

```
# OS
.DS_Store
Thumbs.db
```

Exact `new_string`:

```
# OS
.DS_Store
Thumbs.db

# Local environment config (see .env.example) — never commit real secrets
.env
```

- [ ] **Step 4: Verify the ignore rule now takes effect**

```bash
touch .env
git check-ignore -q .env && echo "now ignored" || echo "still NOT ignored"
rm .env
```

Expected: `now ignored`.

- [ ] **Step 5: Create `.env.example`**

```
# Copy this file to .env and fill in your own values, then run:
#   docker-compose up --build
# See GETTING_STARTED.md for a full walkthrough.

# Required: base URL of your oCIS instance.
OCIS_MCP_OCIS_URL=https://your-ocis-instance.example.com

# Required: shared secret every MCP client must send as "Authorization: Bearer <secret>".
# docker-compose.yml always binds 0.0.0.0:8090 (needed for port publishing to work), so the
# server refuses to start without this secret. Generate one with: openssl rand -hex 32
OCIS_MCP_HTTP_SECRET=

# Recommended: an oCIS app token. Create one in the oCIS web UI under
# Settings > Security > App tokens, or run (against your oCIS deployment, not this repo):
#   docker compose exec ocis ocis auth-app create --user-name="admin" --expiration="8760h"
OCIS_MCP_APP_TOKEN_USER=
OCIS_MCP_APP_TOKEN_VALUE=

# Alternative to an app token: OIDC (see README > Authentication).
#OCIS_MCP_AUTH_MODE=oidc
#OCIS_MCP_OIDC_ACCESS_TOKEN=

# Dev-only: skip TLS verification for a self-signed oCIS test instance.
#OCIS_MCP_TLS_SKIP_VERIFY=true
#OCIS_MCP_INSECURE=true

# Optional: log verbosity (debug, info, warn, error). Defaults to info.
#OCIS_MCP_LOG_LEVEL=debug

# Optional: host port to publish. Defaults to 8090.
#OCIS_MCP_SERVER_PORT=8090

# Optional: image tag to build/run. Defaults to "dev" (local build).
#OCIS_MCP_SERVER_TAG=dev
```

Write this to `.env.example`.

- [ ] **Step 6: Create `docker-compose.yml`**

```yaml
---
services:
  ocis-mcp-server:
    build:
      context: .
    image: owncloud/ocis-mcp-server:${OCIS_MCP_SERVER_TAG:-dev}
    env_file:
      - .env
    environment:
      OCIS_MCP_TRANSPORT: http
      OCIS_MCP_HTTP_ADDR: 0.0.0.0:8090
    ports:
      - "${OCIS_MCP_SERVER_PORT:-8090}:8090"
    restart: unless-stopped
```

Write this to `docker-compose.yml`.

- [ ] **Step 7: Write a real `.env` for testing and validate the compose config**

```bash
cp .env.example .env
sed -i '' 's|^OCIS_MCP_OCIS_URL=.*|OCIS_MCP_OCIS_URL=https://example.com|' .env
SECRET=$(openssl rand -hex 32)
sed -i '' "s/^OCIS_MCP_HTTP_SECRET=$/OCIS_MCP_HTTP_SECRET=${SECRET}/" .env
docker-compose config --quiet && echo "config OK"
```

Expected: `config OK`, no errors. (If `docker info` fails first, run `colima start`.)

- [ ] **Step 8: Bring the service up and confirm it starts correctly**

```bash
docker-compose up -d --build
sleep 3
docker-compose logs ocis-mcp-server
```

Expected: logs include `"starting ocis-mcp-server"` with `transport=http`, `"HTTP transport listening"` with `addr=0.0.0.0:8090 authenticated=true`, and a `WARN` line about the non-loopback bind (expected — this is the same warning seen in the sibling `owncloud/ocis` deployment example's live verification). No `refuses to start` error, since `OCIS_MCP_HTTP_SECRET` is set.

- [ ] **Step 9: Confirm the published port enforces the bearer secret**

```bash
curl -s -o /dev/null -w "%{http_code}\n" http://localhost:8090/mcp
```

Expected: `401` (unauthenticated request rejected).

- [ ] **Step 10: Confirm an authenticated request completes the MCP handshake**

```bash
SECRET=$(grep '^OCIS_MCP_HTTP_SECRET=' .env | cut -d= -f2)
curl -s -X POST "http://localhost:8090/mcp" \
  -H "Authorization: Bearer ${SECRET}" \
  -H "Content-Type: application/json" \
  -H "Accept: application/json, text/event-stream" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"verify-script","version":"0.0.1"}}}'
```

Expected: HTTP 200, a JSON-RPC response (plain JSON or an SSE `data:` event) containing `"result"` with `serverInfo.name: "ocis-mcp-server"`. This works even though `OCIS_MCP_OCIS_URL` points at a placeholder (`https://example.com`) — the `initialize` handshake is local server metadata, it doesn't call oCIS.

- [ ] **Step 11: Tear down and remove the local `.env` (never commit it — it's git-ignored, but keep the working tree clean for the commit)**

```bash
docker-compose down
rm .env
git status --porcelain
```

Expected: no output (or only pre-existing untracked noise like `.DS_Store`) — `.env` was removed and was never tracked.

- [ ] **Step 12: Commit**

```bash
git add docker-compose.yml .env.example .gitignore
git commit -S -s -m "feat: add docker-compose.yml for local development

Adds a docker-compose.yml and .env.example so a contributor can run the MCP
server with docker-compose up --build instead of building/running the
binary or a bare docker run by hand. The HTTP transport is always used
(stdio doesn't fit a detached compose service), bound to 0.0.0.0:8090 so
port publishing works, which means OCIS_MCP_HTTP_SECRET is required — same
security posture already used in the owncloud/ocis ocis_full deployment
example."
```

---

### Task 2: Fix the README's stale env-var names and document Docker Compose

**Files:**
- Modify: `README.md`

**Interfaces:**
- Consumes: the exact env var names from Task 1 / existing code (`OCIS_MCP_OCIS_URL`, `OCIS_MCP_HTTP_SECRET`, `OCIS_MCP_APP_TOKEN_USER`, `OCIS_MCP_APP_TOKEN_VALUE`).

- [ ] **Step 1: Confirm the existing bug (write the failing check)**

```bash
cd /Users/david.walter/Documents/Code/oCIS-Bughunter/ocis-mcp-server
grep -n "^export OCIS_URL\|^export OCIS_ACCESS_TOKEN\|OCIS_URL=\.\.\.\|OCIS_ACCESS_TOKEN=\.\.\." README.md
```

Expected: matches at README.md lines 20, 21, and 31 — confirming the "Quick Setup" and "Docker" sections currently use the wrong env var names (`OCIS_URL`, `OCIS_ACCESS_TOKEN`) instead of the real ones the code reads (`OCIS_MCP_OCIS_URL`, `OCIS_MCP_APP_TOKEN_USER`/`OCIS_MCP_APP_TOKEN_VALUE`). Anyone following these two sections literally would hit `configuration error: OCIS_MCP_OCIS_URL is required` and get stuck.

- [ ] **Step 2: Fix both sections and add a Docker Compose section**

Use the Edit tool on `README.md`. Exact `old_string`:

```
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
```

Exact `new_string`:

```
### Quick Setup

```bash
# Build
make build

# Configure
export OCIS_MCP_OCIS_URL=https://your-ocis-instance.example.com
export OCIS_MCP_APP_TOKEN_USER=admin
export OCIS_MCP_APP_TOKEN_VALUE=your-app-token

# Run
./ocis-mcp-server
```

### Docker

```bash
make docker-build
docker run \
  -e OCIS_MCP_OCIS_URL=https://your-ocis-instance.example.com \
  -e OCIS_MCP_APP_TOKEN_USER=admin \
  -e OCIS_MCP_APP_TOKEN_VALUE=your-app-token \
  owncloud/ocis-mcp-server
```

### Docker Compose

```bash
cp .env.example .env
# edit .env: set OCIS_MCP_OCIS_URL, OCIS_MCP_HTTP_SECRET, and an app token
docker-compose up --build
```

The server listens on `http://localhost:8090/mcp` (see `.env.example` for every available
setting, and [Securing the HTTP transport](#securing-the-http-transport) below for what
`OCIS_MCP_HTTP_SECRET` protects against).
```

- [ ] **Step 3: Verify the fix and the new section**

```bash
grep -n "OCIS_URL=\|OCIS_ACCESS_TOKEN" README.md
echo "---"
grep -n "OCIS_MCP_OCIS_URL\|OCIS_MCP_APP_TOKEN_USER\|OCIS_MCP_APP_TOKEN_VALUE\|Docker Compose" README.md
```

Expected: the first command prints nothing (the wrong var names are gone); the second command shows the corrected Quick Setup/Docker sections plus the new "Docker Compose" heading.

- [ ] **Step 4: Commit**

```bash
git add README.md
git commit -S -s -m "docs: fix stale env var names and document Docker Compose

The Quick Setup and Docker sections used OCIS_URL / OCIS_ACCESS_TOKEN, which
internal/config/config.go has never read -- the real names are
OCIS_MCP_OCIS_URL and OCIS_MCP_APP_TOKEN_USER / OCIS_MCP_APP_TOKEN_VALUE.
Following the old docs literally produced 'OCIS_MCP_OCIS_URL is required'
at startup. Fixed both sections while adding the new Docker Compose section
introduced in the previous commit."
```

---

### Task 3: Add the `docker.yml` GitHub Actions workflow

**Files:**
- Create: `.github/workflows/docker.yml`

**Interfaces:**
- Produces: two jobs, `build` (runs on `push` and `pull_request`, no secrets required) and `publish` (runs on `push` to `main`, on `v*` tags, or `workflow_dispatch`; requires repository secrets `DOCKERHUB_USERNAME` and `DOCKERHUB_TOKEN`, documented as a manual prerequisite in Task 4 — not created by this task).

- [ ] **Step 1: Create `.github/workflows/docker.yml`**

```yaml
name: Docker

on:
  push:
    branches:
      - main
    tags:
      - "v*"
  pull_request:
  workflow_dispatch:

permissions:
  contents: read

jobs:
  build:
    name: Build (all platforms, no push)
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0 # v7.0.0

      - name: Set up QEMU
        uses: docker/setup-qemu-action@96fe6ef7f33517b61c61be40b68a1882f3264fb8 # v4.2.0

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@bb05f3f5519dd87d3ba754cc423b652a5edd6d2c # v4.2.0

      - name: Build linux/amd64 and load for smoke test
        uses: docker/build-push-action@53b7df96c91f9c12dcc8a07bcb9ccacbed38856a # v7.3.0
        with:
          context: .
          platforms: linux/amd64
          load: true
          tags: ocis-mcp-server:smoke-test

      - name: Smoke test the built image
        run: |
          set +e
          output=$(docker run --rm ocis-mcp-server:smoke-test 2>&1)
          status=$?
          echo "$output"
          if [ "$status" -ne 1 ]; then
            echo "::error::expected exit code 1 (missing OCIS_MCP_OCIS_URL), got $status"
            exit 1
          fi
          if ! echo "$output" | grep -q "OCIS_MCP_OCIS_URL is required"; then
            echo "::error::expected the config-validation error in the container's output, did not find it"
            exit 1
          fi
          echo "Smoke test passed: image starts and fails config validation as expected."

      - name: Build linux/amd64 + linux/arm64 (validate only, no push)
        uses: docker/build-push-action@53b7df96c91f9c12dcc8a07bcb9ccacbed38856a # v7.3.0
        with:
          context: .
          platforms: linux/amd64,linux/arm64
          push: false

  publish:
    name: Publish to Docker Hub
    if: github.event_name == 'push' || github.event_name == 'workflow_dispatch'
    needs: build
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0 # v7.0.0

      - name: Set up QEMU
        uses: docker/setup-qemu-action@96fe6ef7f33517b61c61be40b68a1882f3264fb8 # v4.2.0

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@bb05f3f5519dd87d3ba754cc423b652a5edd6d2c # v4.2.0

      - name: Log in to Docker Hub
        uses: docker/login-action@af1e73f918a031802d376d3c8bbc3fe56130a9b0 # v4.4.0
        with:
          username: ${{ secrets.DOCKERHUB_USERNAME }}
          password: ${{ secrets.DOCKERHUB_TOKEN }}

      - name: Extract image metadata
        id: meta
        uses: docker/metadata-action@dc802804100637a589fabce1cb79ff13a1411302 # v6.2.0
        with:
          images: owncloud/ocis-mcp-server
          tags: |
            type=raw,value=latest,enable={{is_default_branch}}
            type=semver,pattern={{version}}
            type=semver,pattern={{major}}.{{minor}}
            type=semver,pattern={{major}}

      - name: Build and push
        uses: docker/build-push-action@53b7df96c91f9c12dcc8a07bcb9ccacbed38856a # v7.3.0
        with:
          context: .
          platforms: linux/amd64,linux/arm64
          push: true
          tags: ${{ steps.meta.outputs.tags }}
          labels: ${{ steps.meta.outputs.labels }}
```

Write this to `.github/workflows/docker.yml`.

- [ ] **Step 2: Validate the YAML parses correctly**

```bash
python3 -c "import yaml, sys; yaml.safe_load(open('.github/workflows/docker.yml')); print('YAML OK')"
```

Expected: `YAML OK`. (If `python3`/`pyyaml` isn't available, use `ruby -ryaml -e "YAML.load_file('.github/workflows/docker.yml'); puts 'YAML OK'"` instead — either confirms the file parses as valid YAML before it ever reaches GitHub.)

- [ ] **Step 3: Reproduce the `build` job's smoke test locally, to catch a broken assertion before CI does**

```bash
docker build -t ocis-mcp-server:smoke-test-local .
docker run --rm ocis-mcp-server:smoke-test-local; echo "exit code: $?"
```

Expected: the container logs a `configuration error` line containing `OCIS_MCP_OCIS_URL is required`, and `exit code: 1`. This is the exact assertion the workflow's "Smoke test the built image" step checks — confirming it will pass once CI runs it for real (single-arch `linux/amd64` only here, since this machine has no buildx; the workflow's separate arm64 validation step will be verified for real in Task 4 via the actual PR-triggered CI run).

```bash
docker rmi ocis-mcp-server:smoke-test-local
```

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/docker.yml
git commit -S -s -m "ci: add Docker build and Docker Hub publish workflow

Adds .github/workflows/docker.yml with two jobs:
- build: runs on every push and pull_request, no credentials needed. Builds
  linux/amd64 and smoke-tests the resulting binary (confirms it starts and
  fails config validation with the expected error), then separately
  validates linux/amd64+linux/arm64 both build (push: false).
- publish: runs on push to main, on v* tags, or manual workflow_dispatch.
  Logs into Docker Hub (secrets.DOCKERHUB_USERNAME / secrets.DOCKERHUB_TOKEN)
  and pushes a multi-arch owncloud/ocis-mcp-server image, tagged 'latest' on
  main and semver tags (vX.Y.Z, X.Y, X) on version tags.

This closes the gap flagged in owncloud/ocis#12603: that PR's ocis_full
deployment example references owncloud/ocis-mcp-server, which does not
exist on any registry yet. The publish job requires DOCKERHUB_USERNAME and
DOCKERHUB_TOKEN repository secrets to be configured by a maintainer with
real Docker Hub credentials before it will succeed -- see the PR description
for this exact prerequisite (this commit cannot configure it)."
```

---

### Task 4: Push, open the PR, and verify the real CI run

**Files:**
- No file changes — this task pushes the branch, opens the PR, and verifies the `build` job actually passes in real GitHub Actions CI.

**Interfaces:**
- Consumes: all files from Tasks 1-3.

- [ ] **Step 1: Confirm the branch and commits from Tasks 1-3**

```bash
cd /Users/david.walter/Documents/Code/oCIS-Bughunter/ocis-mcp-server
git status --porcelain
git branch --show-current
git log --oneline origin/main..HEAD
```

Expected: `git status --porcelain` prints nothing (or only pre-existing untracked noise);
`git branch --show-current` prints `feat/docker-compose-and-hub-publish` (created in Task
1, Step 1); the log lists exactly the 3 commits from Tasks 1-3 (docker-compose.yml, README
fix, docker.yml workflow).

- [ ] **Step 2: Push to the fork**

```bash
git push fork feat/docker-compose-and-hub-publish
```

Expected: push succeeds, prints the new branch ref on `dj4oC/ocis-mcp-server`.

- [ ] **Step 3: Write the PR body**

```markdown
## Problem

This repo has no `docker-compose.yml` (a contributor has to build the binary or run a bare
`docker run` by hand to test changes), and no GitHub Actions workflow publishes the
`owncloud/ocis-mcp-server` image anywhere. `owncloud/ocis` PR
[#12603](https://github.com/owncloud/ocis/pull/12603) added this image as an optional
service in the `ocis_full` deployment example, but flagged as a known limitation that the
image doesn't exist on any registry yet — verified via a 404 from Docker Hub.

## Solution

- Added `docker-compose.yml` + `.env.example` for local dev: `cp .env.example .env`, fill in
  your oCIS URL and a secret, `docker-compose up --build`. Always uses the HTTP transport
  bound to `0.0.0.0:8090` (required for port publishing), so `OCIS_MCP_HTTP_SECRET` is
  required — same fail-closed posture as the `owncloud/ocis` `ocis_full` wiring.
- Fixed a pre-existing bug in `README.md`'s "Quick Setup" and "Docker" sections: they used
  `OCIS_URL` / `OCIS_ACCESS_TOKEN`, which `internal/config/config.go` has never read (the
  real names are `OCIS_MCP_OCIS_URL` / `OCIS_MCP_APP_TOKEN_USER` /
  `OCIS_MCP_APP_TOKEN_VALUE`). Following the old docs literally failed at startup. Fixed
  both sections and added a new "Docker Compose" section.
- Added `.github/workflows/docker.yml`:
  - `build` job (every push + PR, no credentials needed): builds `linux/amd64`, loads it,
    and smoke-tests that the binary actually starts and fails config validation with the
    expected error; separately validates `linux/amd64` + `linux/arm64` both build
    (`push: false`).
  - `publish` job (push to `main`, `v*` tags, or manual `workflow_dispatch`): logs into
    Docker Hub and pushes a multi-arch image — `latest` on `main`, semver tags
    (`vX.Y.Z`/`X.Y`/`X`) on version tags.

## Testing

- `docker-compose config --quiet`, `docker-compose up --build`, and a live curl round-trip
  (401 unauthenticated, 200 + valid JSON-RPC `initialize` result when authenticated)
  confirmed locally.
- Reproduced the workflow's smoke-test assertion locally with a plain `docker build` +
  `docker run` (linux/amd64 only, this machine has no local buildx for the arm64 leg).
- The real PR-triggered `build` job (multi-arch, via GitHub's hosted buildx+QEMU) ran in CI
  for this PR — see the Checks tab. [Fill in the actual run conclusion once observed.]

## Known Limitation / Manual Prerequisite

The `publish` job needs repository secrets `DOCKERHUB_USERNAME` and `DOCKERHUB_TOKEN` (a
Docker Hub access token, not a password) added under **Settings > Secrets and variables >
Actions**, using an account with push access to the `owncloud` Docker Hub org/namespace.
This PR cannot configure that — a maintainer needs to add both secrets. Until then, the
`build` job will keep passing on every PR, but `publish` will fail at the login step on
`main`/tag pushes. Once the secrets are added, either push to `main` or a `v*` tag, or
trigger `workflow_dispatch` manually, to get the first real Docker Hub image published —
that also unblocks `owncloud/ocis#12603`.

## Risk

Low. Purely additive (new files, one doc bug fix); no existing runtime code changed.

🤖 Generated with Claude Code
```

Write this to `/tmp/ocis-mcp-server-pr-body.md`.

- [ ] **Step 4: Open the PR**

```bash
gh pr list --repo owncloud/ocis-mcp-server --head dj4oC:feat/docker-compose-and-hub-publish
```

Expected: empty (no pre-existing PR for this branch). Then:

```bash
gh pr create \
  --repo owncloud/ocis-mcp-server \
  --base main \
  --title "feat: docker-compose support + Docker Hub publish workflow" \
  --body "$(cat /tmp/ocis-mcp-server-pr-body.md)"
```

Expected: prints the created PR URL.

- [ ] **Step 5: Watch the real `build` job run and confirm it passes**

```bash
gh pr checks --repo owncloud/ocis-mcp-server <PR_NUMBER> --watch
```

(Replace `<PR_NUMBER>` with the number from the URL Step 4 printed.) Expected: the `build`
job (from `.github/workflows/docker.yml`) eventually shows a passing conclusion. If it
fails, read the run's log via `gh run view --repo owncloud/ocis-mcp-server --log-failed`,
fix the root cause (do not disable the failing check), push a fix commit, and re-watch.

- [ ] **Step 6: Update the PR body's Testing section with the real result**

Once Step 5 shows a real conclusion, edit `/tmp/ocis-mcp-server-pr-body.md`'s bracketed
placeholder (`[Fill in the actual run conclusion once observed.]`) to state it plainly, e.g.
"Confirmed passing: `build` job succeeded on both `linux/amd64` and `linux/arm64` (run
`<url>`)." Then:

```bash
gh pr edit --repo owncloud/ocis-mcp-server <PR_NUMBER> --body "$(cat /tmp/ocis-mcp-server-pr-body.md)"
```

Expected: PR description updates; no error.
