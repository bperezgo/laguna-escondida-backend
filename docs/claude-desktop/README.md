# Claude Desktop setup

How to drive the Laguna Escondida backend from the Claude Desktop app.

## 1. Connect the MCP server (stdio)

Claude Desktop launches MCP servers as local subprocesses over stdio and has **no
local shell or file access of its own** — every local capability comes from an MCP
server. Build the server and register it plus the official filesystem server in
`~/Library/Application Support/Claude/claude_desktop_config.json`:

```jsonc
{
  "mcpServers": {
    "laguna-escondida": {
      "command": "/absolute/path/to/laguna-escondida-backend/bin/mcp-server",
      "args": ["--stdio"],
      "env": {
        "LAGUNA_API_URL": "https://<backend-that-can-reach-the-bucket>",
        "LAGUNA_USERNAME": "mcp-service",
        "LAGUNA_PASSWORD": "…"
      }
    },
    "filesystem": {
      "command": "/usr/local/bin/npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "/Users/you/Downloads"]
    }
  }
}
```

Build with `make build-mcp`. Point `LAGUNA_API_URL` at a backend that can reach
the **internal bucket** (invoice attachments are stored via the backend, so a
local backend isolated from the bucket network can't persist them). Fully quit and
reopen Desktop after editing.

## 2. Install the invoice skill

Desktop skills are uploaded as a zip through **Settings → Capabilities/Skills**
(they run in a sandbox and cannot touch local files — they only orchestrate MCP
tools, which is why extraction is the `extract_archive` MCP tool, not a bash step).

```bash
cd docs/claude-desktop
zip -r ingest-invoice.zip ingest-invoice   # archive contains ingest-invoice/SKILL.md
```

Upload `ingest-invoice.zip` in Desktop. Then, in a chat: attach a PDF invoice (or
give the path to a DIAN `.zip` in `~/Downloads`) and ask Claude to ingest it.

> The same recipe exists as a **Claude Code** command at
> `.claude/commands/ingest-invoice.md` for use in the terminal, where local file
> reads and `Read` are available.

## 3. Reuse across sessions (Project / Cowork)

Claude Desktop **cannot scope skills or MCP servers per project** — uploaded skills
and `claude_desktop_config.json` servers are **account-global** (available in every
chat). Per-project scoping only exists in Claude Code (`.claude/skills/`, project
`.mcp.json`). Desktop's reuse mechanism is a **Project / Cowork space**, which
scopes *context* (custom instructions, knowledge files, memory, a mounted folder) —
not tools.

Recommended setup:

1. MCP server configured (step 1) and skill uploaded (step 2) — both global; fine,
   since they're invoice-specific anyway.
2. **Cowork → Projects → Create project** → name it e.g. "Invoice Processing",
   point it at a folder (e.g. `~/Documents/invoices/`), and add standing
   instructions (your FACTURA/DIAN rules, default category, supplier conventions).
3. Reopen that project any time: instructions + folder + memory persist, and the
   global skill + MCP are already available — no re-setup.

Cowork is beta; menu labels may shift. Docs: <https://claude.com/docs/cowork/guide/projects>,
<https://support.claude.com/en/articles/12512180-use-skills-in-claude>.
