#!/usr/bin/env bash
#
# install.sh -- Setup helper for ocis-mcp-server
#
# Detects your OS, checks for required tools (Go, Docker, Claude Desktop,
# Ollama, mcphost), and offers to build the server and write config files.
#
# Safe: always asks before writing anything. Works on macOS, Linux, and WSL.

set -euo pipefail

# ─── Colors ───────────────────────────────────────────────────────────────────

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
BOLD='\033[1m'
NC='\033[0m' # No Color

ok()   { printf "${GREEN}[OK]${NC}    %s\n" "$1"; }
warn() { printf "${YELLOW}[MISS]${NC}  %s\n" "$1"; }
info() { printf "${BLUE}[INFO]${NC}  %s\n" "$1"; }
err()  { printf "${RED}[ERR]${NC}   %s\n" "$1"; }

ask_yes_no() {
    local prompt="$1"
    local answer
    printf "${BOLD}%s [y/N]: ${NC}" "$prompt"
    read -r answer
    case "$answer" in
        [yY]|[yY][eE][sS]) return 0 ;;
        *) return 1 ;;
    esac
}

# ─── Detect OS ────────────────────────────────────────────────────────────────

detect_os() {
    case "$(uname -s)" in
        Darwin)
            OS="macos"
            ;;
        Linux)
            if grep -qiE '(microsoft|wsl)' /proc/version 2>/dev/null; then
                OS="wsl"
            else
                OS="linux"
            fi
            ;;
        MINGW*|MSYS*|CYGWIN*)
            OS="windows"
            ;;
        *)
            OS="unknown"
            ;;
    esac
}

# ─── Claude Desktop config path per OS ────────────────────────────────────────

claude_config_path() {
    case "$OS" in
        macos)
            echo "$HOME/Library/Application Support/Claude/claude_desktop_config.json"
            ;;
        linux|wsl)
            echo "$HOME/.config/Claude/claude_desktop_config.json"
            ;;
        windows)
            echo "$APPDATA/Claude/claude_desktop_config.json"
            ;;
        *)
            echo ""
            ;;
    esac
}

# ─── mcphost config path ─────────────────────────────────────────────────────

mcphost_config_path() {
    echo "$HOME/.mcphost/config.json"
}

# ─── Check tools ──────────────────────────────────────────────────────────────

HAS_GO=false
HAS_DOCKER=false
HAS_CLAUDE=false
HAS_OLLAMA=false
HAS_MCPHOST=false
HAS_BINARY=false

check_tools() {
    echo ""
    printf "${BOLD}Checking your system...${NC}\n"
    echo ""

    # OS
    detect_os
    case "$OS" in
        macos)   info "Operating system: macOS" ;;
        linux)   info "Operating system: Linux" ;;
        wsl)     info "Operating system: Windows (WSL)" ;;
        windows) info "Operating system: Windows (Git Bash / MSYS)" ;;
        *)       warn "Operating system: unknown ($(uname -s))" ;;
    esac

    echo ""

    # Go
    if command -v go &>/dev/null; then
        local go_version
        go_version=$(go version | awk '{print $3}' | sed 's/go//')
        ok "Go is installed (version $go_version)"
        HAS_GO=true
    else
        warn "Go is not installed -- get it from https://go.dev/dl/"
    fi

    # Docker
    if command -v docker &>/dev/null; then
        ok "Docker is installed"
        HAS_DOCKER=true
    else
        warn "Docker is not installed -- get it from https://docs.docker.com/get-docker/"
    fi

    # Claude Desktop
    case "$OS" in
        macos)
            if [ -d "/Applications/Claude.app" ]; then
                ok "Claude Desktop is installed"
                HAS_CLAUDE=true
            else
                warn "Claude Desktop is not installed -- get it from https://claude.ai/download"
            fi
            ;;
        linux|wsl)
            if command -v claude-desktop &>/dev/null || \
               [ -f "/usr/bin/claude-desktop" ] || \
               [ -f "$HOME/.local/bin/claude-desktop" ] || \
               snap list claude-desktop &>/dev/null 2>&1 || \
               flatpak list 2>/dev/null | grep -qi claude; then
                ok "Claude Desktop is installed"
                HAS_CLAUDE=true
            else
                warn "Claude Desktop is not found -- check https://claude.ai/download"
            fi
            ;;
        windows)
            if [ -f "$LOCALAPPDATA/Programs/claude-desktop/Claude.exe" ] 2>/dev/null || \
               [ -f "$PROGRAMFILES/Claude/Claude.exe" ] 2>/dev/null; then
                ok "Claude Desktop is installed"
                HAS_CLAUDE=true
            else
                warn "Claude Desktop not detected -- get it from https://claude.ai/download"
            fi
            ;;
        *)
            warn "Cannot detect Claude Desktop on this OS"
            ;;
    esac

    # Ollama
    if command -v ollama &>/dev/null; then
        ok "Ollama is installed"
        HAS_OLLAMA=true
    else
        warn "Ollama is not installed -- get it from https://ollama.com/"
    fi

    # mcphost
    if command -v mcphost &>/dev/null; then
        ok "mcphost is installed"
        HAS_MCPHOST=true
    else
        warn "mcphost is not installed -- run: go install github.com/mark3labs/mcphost@latest"
    fi

    # ocis-mcp-server binary
    if [ -f "./ocis-mcp-server" ] || [ -f "./bin/ocis-mcp-server" ]; then
        ok "ocis-mcp-server binary found"
        HAS_BINARY=true
    elif [ -f "./ocis-mcp-server.exe" ] || [ -f "./bin/ocis-mcp-server.exe" ]; then
        ok "ocis-mcp-server.exe binary found"
        HAS_BINARY=true
    else
        warn "ocis-mcp-server binary not built yet"
    fi

    echo ""
}

# ─── Build ────────────────────────────────────────────────────────────────────

build_server() {
    if [ "$HAS_BINARY" = true ]; then
        info "Binary already exists. Skipping build."
        return
    fi

    if [ "$HAS_GO" = false ]; then
        info "Go is not installed, cannot build from source."
        if [ "$HAS_DOCKER" = true ]; then
            info "You can use Docker instead. See the README for Docker instructions."
        fi
        return
    fi

    if ask_yes_no "Build ocis-mcp-server from source?"; then
        echo ""
        info "Building..."
        if go build -o ocis-mcp-server ./cmd/ocis-mcp-server; then
            ok "Built successfully: $(pwd)/ocis-mcp-server"
            HAS_BINARY=true
        else
            err "Build failed. Check the error messages above."
        fi
    fi
}

# ─── Prompt for credentials ──────────────────────────────────────────────────

OCIS_URL=""
TOKEN_USER=""
TOKEN_VALUE=""
SERVER_PATH=""

ask_credentials() {
    echo ""
    printf "${BOLD}Let's set up your oCIS connection.${NC}\n"
    echo ""

    printf "Enter your oCIS server URL (e.g. https://ocis.example.com): "
    read -r OCIS_URL
    if [ -z "$OCIS_URL" ]; then
        err "oCIS URL is required. Skipping config setup."
        return 1
    fi

    printf "Enter your app token username (e.g. admin): "
    read -r TOKEN_USER
    if [ -z "$TOKEN_USER" ]; then
        err "Username is required. Skipping config setup."
        return 1
    fi

    printf "Enter your app token value: "
    read -rs TOKEN_VALUE
    echo ""
    if [ -z "$TOKEN_VALUE" ]; then
        err "Token is required. Skipping config setup."
        return 1
    fi

    # Determine server path
    if [ -f "$(pwd)/ocis-mcp-server" ]; then
        SERVER_PATH="$(pwd)/ocis-mcp-server"
    elif [ -f "$(pwd)/bin/ocis-mcp-server" ]; then
        SERVER_PATH="$(pwd)/bin/ocis-mcp-server"
    elif [ -f "$(pwd)/ocis-mcp-server.exe" ]; then
        SERVER_PATH="$(pwd)/ocis-mcp-server.exe"
    else
        printf "Enter the full path to ocis-mcp-server binary: "
        read -r SERVER_PATH
    fi

    return 0
}

# ─── Write Claude Desktop config ─────────────────────────────────────────────

write_claude_config() {
    if [ "$HAS_CLAUDE" = false ]; then
        return
    fi

    echo ""
    local config_file
    config_file=$(claude_config_path)

    if [ -z "$config_file" ]; then
        warn "Could not determine Claude Desktop config path for this OS."
        return
    fi

    if [ -f "$config_file" ]; then
        info "Claude Desktop config already exists at: $config_file"
        if ! ask_yes_no "Overwrite it?"; then
            info "Skipping Claude Desktop config."
            return
        fi
    else
        if ! ask_yes_no "Write Claude Desktop config to $config_file?"; then
            return
        fi
    fi

    if [ -z "$OCIS_URL" ]; then
        if ! ask_credentials; then
            return
        fi
    fi

    # Escape backslashes for Windows paths in JSON
    local json_path
    json_path=$(echo "$SERVER_PATH" | sed 's/\\/\\\\/g')

    mkdir -p "$(dirname "$config_file")"

    cat > "$config_file" <<JSONEOF
{
  "mcpServers": {
    "ocis": {
      "command": "$json_path",
      "env": {
        "OCIS_MCP_OCIS_URL": "$OCIS_URL",
        "OCIS_MCP_APP_TOKEN_USER": "$TOKEN_USER",
        "OCIS_MCP_APP_TOKEN_VALUE": "$TOKEN_VALUE"
      }
    }
  }
}
JSONEOF

    ok "Wrote Claude Desktop config to: $config_file"
    info "Restart Claude Desktop to pick up the changes."
}

# ─── Write mcphost config ────────────────────────────────────────────────────

write_mcphost_config() {
    if [ "$HAS_OLLAMA" = false ]; then
        return
    fi

    echo ""
    local config_file
    config_file=$(mcphost_config_path)

    if [ -f "$config_file" ]; then
        info "mcphost config already exists at: $config_file"
        if ! ask_yes_no "Overwrite it?"; then
            info "Skipping mcphost config."
            return
        fi
    else
        if ! ask_yes_no "Write mcphost config to $config_file?"; then
            return
        fi
    fi

    if [ -z "$OCIS_URL" ]; then
        if ! ask_credentials; then
            return
        fi
    fi

    local json_path
    json_path=$(echo "$SERVER_PATH" | sed 's/\\/\\\\/g')

    mkdir -p "$(dirname "$config_file")"

    cat > "$config_file" <<JSONEOF
{
  "mcpServers": {
    "ocis": {
      "command": "$json_path",
      "env": {
        "OCIS_MCP_OCIS_URL": "$OCIS_URL",
        "OCIS_MCP_APP_TOKEN_USER": "$TOKEN_USER",
        "OCIS_MCP_APP_TOKEN_VALUE": "$TOKEN_VALUE"
      }
    }
  }
}
JSONEOF

    ok "Wrote mcphost config to: $config_file"

    if [ "$HAS_MCPHOST" = true ]; then
        echo ""
        info "You're all set! Run this to start chatting:"
        echo ""
        echo "  mcphost --model ollama:llama3.2"
        echo ""
    else
        echo ""
        info "Install mcphost to use Ollama with MCP:"
        echo ""
        echo "  go install github.com/mark3labs/mcphost@latest"
        echo "  mcphost --model ollama:llama3.2"
        echo ""
    fi
}

# ─── Summary ──────────────────────────────────────────────────────────────────

print_summary() {
    echo ""
    printf "${BOLD}────────────────────────────────────${NC}\n"
    printf "${BOLD}  Setup Complete!${NC}\n"
    printf "${BOLD}────────────────────────────────────${NC}\n"
    echo ""

    if [ "$HAS_CLAUDE" = true ]; then
        info "Claude Desktop: Restart the app, then ask Claude about your oCIS files."
    fi
    if [ "$HAS_OLLAMA" = true ] && [ "$HAS_MCPHOST" = true ]; then
        info "Ollama: Run  mcphost --model ollama:llama3.2  to start chatting."
    fi

    echo ""
    info "For the full guide, see: GETTING_STARTED.md"
    info "For advanced config, see: README.md"
    echo ""
}

# ─── Main ─────────────────────────────────────────────────────────────────────

main() {
    echo ""
    printf "${BOLD}ocis-mcp-server Setup${NC}\n"
    printf "${BOLD}=====================${NC}\n"

    check_tools
    build_server

    # Only offer config if at least one client is detected
    if [ "$HAS_CLAUDE" = true ] || [ "$HAS_OLLAMA" = true ]; then
        write_claude_config
        write_mcphost_config
        print_summary
    else
        echo ""
        info "No AI client detected (Claude Desktop or Ollama)."
        info "Install one of them first, then run this script again."
        echo ""
        info "  Claude Desktop: https://claude.ai/download"
        info "  Ollama:         https://ollama.com/"
        echo ""
    fi
}

main "$@"
