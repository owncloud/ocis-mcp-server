# Getting Started with the oCIS MCP Server

Hey there! This guide will help you connect an AI assistant (like Claude or an Ollama model) to your
ownCloud files. Don't worry if some of this is new to you -- we'll go step by step.

## What does this do?

Imagine you have files stored in the cloud using **ownCloud** (called **oCIS**). Now imagine you
could talk to an AI and say things like:

- "Show me all my files"
- "Create a new folder called Homework"
- "Share my project with my friend"

That's exactly what this MCP server does. It's like a **translator** between an AI assistant and
your cloud files. The AI talks to the MCP server, and the MCP server talks to oCIS.

```
You  -->  AI Assistant  -->  MCP Server  -->  Your oCIS Cloud Files
```

## What you need

Before we start, make sure you have:

- [ ] A computer (Mac, Windows, or Linux)
- [ ] An **oCIS server** running somewhere (your school or organization might have one,
      or you can [run one with Docker](https://doc.owncloud.com/ocis/next/quickstart/docker.html))
- [ ] Either **Claude Desktop** or **Ollama** installed (we'll cover both below)
- [ ] **Go** installed (version 1.25 or newer) -- only needed if building from source.
      Get it from [go.dev/dl](https://go.dev/dl/). Not needed if you download a pre-built binary.

## Step 1: Get the MCP Server Ready

You can either **download a pre-built binary** (easiest) or build from source.

### Option A: Download from Releases (recommended)

Go to the [Releases page](https://github.com/owncloud/ocis-mcp-server/releases) and download the
right file for your system:

| System | File to download |
|---|---|
| Mac (Apple Silicon / M1-M4) | `ocis-mcp-server_*_darwin_arm64.tar.gz` |
| Mac (Intel) | `ocis-mcp-server_*_darwin_amd64.tar.gz` |
| Windows | `ocis-mcp-server_*_windows_amd64.zip` |
| Linux | `ocis-mcp-server_*_linux_amd64.tar.gz` |

Extract it somewhere you'll remember (like your home folder).

**On Mac/Linux**, open a terminal and run:

```bash
cd ~/Downloads
tar xzf ocis-mcp-server_*.tar.gz
chmod +x ocis-mcp-server
mv ocis-mcp-server ~/ocis-mcp-server
```

**On Windows**, right-click the `.zip` file and select "Extract All".

> **Mac users -- important!** macOS blocks downloaded programs by default. You need to run this
> command once to allow it:
>
> ```bash
> xattr -d com.apple.quarantine ~/ocis-mcp-server
> ```
>
> If you skip this step you'll see *"Apple could not verify this software"* or
> *"Permission denied"* when Claude Desktop tries to start the server.

### Option B: Build from source

If you have Go installed (version 1.25+), you can build it yourself:

```bash
git clone https://github.com/owncloud/ocis-mcp-server.git
cd ocis-mcp-server
go build -o ocis-mcp-server ./cmd/ocis-mcp-server
```

On **Windows**, the built file will be called `ocis-mcp-server.exe`.

> **Tip:** Run `./install.sh` (Mac/Linux) to automatically check what's installed on your system
> and help you set everything up. See [Using the Install Script](#using-the-install-script) below.

## Step 2: Get Your oCIS App Token

To let the MCP server talk to oCIS, you need a special password called an **app token**. There are
two ways to get one:

### Option A: Using the command line (most common)

If your oCIS runs in Docker, open a terminal and run:

```bash
docker compose exec ocis ocis auth-app create \
  --user-name="admin" \
  --expiration="8760h"
```

This creates a token that lasts for 1 year (8760 hours). It will print something like:

```
App token created:
  User: admin
  Token: WExn...long-string-here...kQ==
```

**Write down both the user name and the token** -- you'll need them in the next step!

### Option B: Using the web interface

If your oCIS has [ocis-app-tokens](https://github.com/mschlachter/ocis-app-tokens) installed, you
can create tokens through the web browser. Open the app-tokens page, click "Create", give it a name
like "MCP Server", and copy the token that appears.

---

## Connect with Claude Desktop

Claude Desktop is an app from Anthropic that lets you chat with Claude AI. It has built-in support
for MCP servers, so connecting is easy.

**First**, find out the full path to your `ocis-mcp-server` file. For example:
- Mac/Linux: `/home/yourname/ocis-mcp-server/ocis-mcp-server`
- Windows: `C:\Users\yourname\ocis-mcp-server\ocis-mcp-server.exe`

You can find it by running `pwd` in the terminal while inside the project folder.

### On Mac

1. Open Finder.
2. Press **Cmd + Shift + G** and paste this path:
   ```
   ~/Library/Application Support/Claude
   ```
3. Open (or create) the file `claude_desktop_config.json`.
4. Paste this inside (replace the placeholder values with your own):

```json
{
  "mcpServers": {
    "ocis": {
      "command": "/full/path/to/ocis-mcp-server",
      "env": {
        "OCIS_MCP_OCIS_URL": "https://your-ocis-server.example.com",
        "OCIS_MCP_APP_TOKEN_USER": "admin",
        "OCIS_MCP_APP_TOKEN_VALUE": "your-token-here"
      }
    }
  }
}
```

5. Save the file and **restart Claude Desktop** (quit it completely and open it again).

### On Windows

1. Press **Win + R**, type this, and press Enter:
   ```
   %APPDATA%\Claude
   ```
2. Open (or create) the file `claude_desktop_config.json`.
3. Paste this inside:

```json
{
  "mcpServers": {
    "ocis": {
      "command": "C:\\Users\\yourname\\ocis-mcp-server\\ocis-mcp-server.exe",
      "env": {
        "OCIS_MCP_OCIS_URL": "https://your-ocis-server.example.com",
        "OCIS_MCP_APP_TOKEN_USER": "admin",
        "OCIS_MCP_APP_TOKEN_VALUE": "your-token-here"
      }
    }
  }
}
```

4. Save the file and **restart Claude Desktop**.

### On Linux

1. Open a terminal and run:
   ```bash
   mkdir -p ~/.config/Claude
   nano ~/.config/Claude/claude_desktop_config.json
   ```
2. Paste this inside:

```json
{
  "mcpServers": {
    "ocis": {
      "command": "/full/path/to/ocis-mcp-server",
      "env": {
        "OCIS_MCP_OCIS_URL": "https://your-ocis-server.example.com",
        "OCIS_MCP_APP_TOKEN_USER": "admin",
        "OCIS_MCP_APP_TOKEN_VALUE": "your-token-here"
      }
    }
  }
}
```

3. Press **Ctrl + O** to save, then **Ctrl + X** to exit nano.
4. **Restart Claude Desktop**.

### Try it out!

Open Claude Desktop and try typing:

- "List all my spaces"
- "What files are in my personal space?"
- "Create a folder called 'My Project' in my personal space"

You should see Claude use the oCIS tools to do what you asked!

---

## Connect with Ollama

[Ollama](https://ollama.com/) lets you run AI models on your own computer for free. It doesn't
directly speak MCP, so we need a small helper tool called **mcphost** that acts as a bridge.

```
You  -->  mcphost  -->  Ollama (AI brain)
               |
               +--->  MCP Server  -->  oCIS
```

### Step 1: Install Ollama

#### On Mac

```bash
# Using Homebrew
brew install ollama

# Or download from https://ollama.com/download/mac
```

#### On Windows

Download the installer from [ollama.com/download/windows](https://ollama.com/download/windows)
and run it.

#### On Linux

```bash
curl -fsSL https://ollama.com/install.sh | sh
```

### Step 2: Download an AI model

Open a terminal and run:

```bash
ollama pull llama3.2
```

This downloads a free AI model to your computer. It might take a few minutes depending on your
internet speed.

### Step 3: Install mcphost

mcphost is the bridge between Ollama and MCP servers. Install it with:

```bash
go install github.com/mark3labs/mcphost@latest
```

Make sure `$(go env GOPATH)/bin` is in your PATH. If you're not sure, run:

```bash
# Mac / Linux
export PATH="$PATH:$(go env GOPATH)/bin"

# On Windows (PowerShell)
$env:PATH += ";$(go env GOPATH)\bin"
```

### Step 4: Create the mcphost config

#### On Mac / Linux

```bash
mkdir -p ~/.mcphost
```

Create the file `~/.mcphost/config.json`:

```json
{
  "mcpServers": {
    "ocis": {
      "command": "/full/path/to/ocis-mcp-server",
      "env": {
        "OCIS_MCP_OCIS_URL": "https://your-ocis-server.example.com",
        "OCIS_MCP_APP_TOKEN_USER": "admin",
        "OCIS_MCP_APP_TOKEN_VALUE": "your-token-here"
      }
    }
  }
}
```

#### On Windows

Create the folder and file at `%USERPROFILE%\.mcphost\config.json`:

```powershell
mkdir "$env:USERPROFILE\.mcphost"
notepad "$env:USERPROFILE\.mcphost\config.json"
```

Paste the same JSON content as above (use the Windows path to your `ocis-mcp-server.exe`).

### Step 5: Run it!

```bash
mcphost --model ollama:llama3.2
```

You'll get a chat prompt. Try typing:

- "Check if the oCIS server is healthy"
- "List all users"
- "What spaces do I have?"

> **Note:** Ollama models run locally on your computer. They need a decent amount of RAM (at least
> 8 GB free). If things are slow, try a smaller model: `ollama pull llama3.2:1b` and use
> `mcphost --model ollama:llama3.2:1b`.

---

## Using the Install Script

We've included a helper script that checks your system and helps set things up. Run it from the
project folder:

```bash
# Mac / Linux
chmod +x install.sh
./install.sh
```

The script will:
1. Detect your operating system
2. Check if Go, Docker, Claude Desktop, Ollama, and mcphost are installed
3. Show you a status report of what's ready and what's missing
4. Offer to build the MCP server for you
5. Offer to write the config files for Claude Desktop and/or mcphost

It will always **ask before changing anything** on your computer.

> **Windows users:** Run the script inside WSL (Windows Subsystem for Linux) or Git Bash.

---

## Troubleshooting

### "Permission denied" or "Apple could not verify" (Mac only)

macOS blocks programs downloaded from the internet. Run this in Terminal:

```bash
xattr -d com.apple.quarantine /path/to/ocis-mcp-server
```

Replace `/path/to/ocis-mcp-server` with the actual location of the file.

### "Connection refused" or "cannot connect"

- Double-check your oCIS URL. Can you open it in a web browser?
- Make sure the oCIS server is running.
- If using `https://localhost`, you may need to add `OCIS_MCP_INSECURE=true` to your config.

### "Authentication failed" or "401 error"

- Check that your app token user and value are correct.
- Make sure there are no extra spaces when you copy-paste the token.
- The token might have expired -- create a new one.

### "Command not found"

- Make sure the path to `ocis-mcp-server` in your config file is correct.
- Try using the **full absolute path** (starting with `/` on Mac/Linux or `C:\` on Windows).
- If you used `go build`, the binary is in the project folder.

### Claude Desktop doesn't show oCIS tools

- Make sure you saved the config file in the **right location** for your OS (see above).
- Make sure the JSON is valid (no missing commas, brackets, etc.).
- Restart Claude Desktop completely (quit and reopen).

### Ollama is slow

- Try a smaller model: `ollama pull llama3.2:1b`
- Close other apps to free up memory.
- Ollama works best with at least 8 GB of free RAM.

---

## Cool Things to Try

Here are some fun prompts to get you started:

1. **"Show me what's in my personal space"** -- See all your files and folders.

2. **"Create a folder called 'School Projects' and then create three subfolders inside it:
   Math, Science, and History"** -- Watch the AI create a whole folder structure for you!

3. **"Search for all PDF files"** -- Find every PDF across all your spaces.

4. **"Share my 'School Projects' folder with marie@example.com as a viewer"** -- Collaborate
   with friends.

5. **"Give me an overview of all my spaces -- how much storage am I using?"** -- Get a
   summary of everything.

6. **"Find all files tagged 'important' and tell me when they were last modified"** -- Use
   tags to organize your stuff.

---

## Next Steps

- Check out the full [README](README.md) for all 80 tools and advanced configuration.
- Look at the [MCP Prompts](README.md#mcp-prompts) for guided workflows like onboarding users
  and generating sharing reports.
- Want to help improve this project? See the [Contributing](README.md#contributing) section.

Happy coding!
