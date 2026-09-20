# Dev container — one box per git worktree, safe to leave unattended

This folder defines an isolated Docker environment for running **Claude Code with
`--dangerously-skip-permissions`** (zero permission prompts) against this repo —
one container per **git worktree**, so several tickets can be built in parallel
without touching each other's files, database or ports.

The idea in one sentence: **bypass mode removes the *inner* guard (prompts), so we
add an *outer* guard (a container + an egress firewall)** — a bad or hijacked command
can only touch this box and the worktree it was opened on, and can't phone home.

> ⚠️ For **trusted code only** (your own repo). Bypass mode has no prompt-injection
> defence, and this box mounts the host Docker socket — a deliberate hole that
> `make test-acceptance` needs. See *Safety model*.

---

## Files in this folder

| File | What it is |
|------|------------|
| `devcontainer.json` | The manifest. Parsed as **JSONC** (comments allowed). |
| `docker-compose.devcontainer.yml` | The dev box **plus this worktree's own Postgres and MinIO**, as one compose project. |
| `docker-compose.generated.yml` | **Generated, git-ignored.** Written by `initialize.sh`: the repo mount, the git-dir mount, and this box's host ports. |
| `initialize.sh` | Runs **on your Mac** before the container is created. Resolves the main repo's git dir and picks a free port block. |
| `Dockerfile` | Ubuntu base + the firewall's system packages + the ownership/sudo fixes. |
| `init-firewall.sh` | Runs as root on **every container start**. Builds an allow-list, then default-**DROP**s outbound. Self-verifies. |
| `post-create.sh` | Runs once inside the box: installs golangci-lint, mockery, gotestsum, migrate, lefthook, govulncheck. |
| `devcontainer-lock.json` | Pins the four features to exact digests. |
| `.ports` | **Generated, git-ignored.** This worktree's port assignment, so reopening the box doesn't shift it. |

---

## The worktree model

`/ticket-start` creates `../laguna-escondida-backend.worktrees/<TICKET>`. Open **that
folder** in a container and you get a box dedicated to that ticket.

**What each worktree box gets to itself:** its own Postgres (`postgres:5432` in here),
its own MinIO, its own data volumes, its own block of host ports. Migrations and seed
data from one ticket can never leak into another.

**What every box shares** (named volumes, created by `initialize.sh`): your Claude
login (`laguna-claude-config`), the Go module cache, the Go build cache, and
`$GOPATH/bin`. So you sign in once, and the second worktree is ready in seconds
instead of re-downloading the module graph and rebuilding the toolchain.

### Why `initialize.sh` exists

Two things can't be written into a static `devcontainer.json`, because they depend on
which worktree you opened:

1. **Git.** In a linked worktree `.git` is a *file* pointing at
   `<main-repo>/.git/worktrees/<TICKET>` — a path **outside** the folder you opened.
   Mount only the worktree and every git command in the box says
   `fatal: not a git repository`: no commits, no lefthook, no `/ticket-start`
   follow-through. So the main repo's git dir is bind-mounted at the **same absolute
   path** it has on the host (the paths inside that `.git` file are absolute).
2. **Ports.** Each box owns a block of 10 consecutive host ports (`8100+`), probed for
   availability and then remembered in `.ports`. The main checkout keeps the familiar
   `8080 / 8090 / 9090 / 5432 / 9000 / 9001` when they're free.

The script prints what it chose on every start:

```
[init] workspace   /Users/you/.../laguna-escondida-backend.worktrees/BSP-42 (linked worktree: yes)
[init] git dir     /Users/you/.../laguna-escondida-backend/.git (mounted at the same path)
[init] host ports  api 8130 · mcp 8131 · metrics 8132 · postgres 8133 · minio 8134/8135
```

---

## Launch runbook

**Prereqs:** Docker Desktop running + VS Code with the **Dev Containers** extension
(CLI alternative: `npm i -g @devcontainers/cli`).

1. **Create the worktree first**, then open it:
   ```
   /ticket-start BSP-42          # or: git worktree add ../laguna-escondida-backend.worktrees/BSP-42 -b feat/BSP-42
   code ../laguna-escondida-backend.worktrees/BSP-42
   ```
   Then **Dev Containers: Reopen in Container**. First build takes a few minutes.
   *(CLI: `devcontainer up --workspace-folder .`)*

   `.devcontainer/` must be committed on the branch the worktree is based on —
   a fresh worktree only has what's in git.

2. **Watch for the proof the firewall applied**, at the end of the startup log:
   ```
   [firewall] active — egress restricted to the allow-set.
   ```

3. **Sign in** (only the first time — the volume is shared by every box):
   ```
   claude
   ```

4. **Sanity-check** (each line confirms one layer):
   ```
   uname -s                   # Linux — confirms you're inside the box
   go version                 # → go1.25.x, matching go.mod
   git status -sb             # the worktree's branch — proves the git-dir mount works
   docker ps                  # talks to the HOST daemon (DooD) — needed by acceptance tests
   golangci-lint --version    # toolchain installed by post-create.sh
   ls -ld ~/.claude           # owner must be vscode — root here means login won't persist
   sudo -l                    # must list ONLY the two scoped scripts, no blanket root
   psql -h postgres -U postgres -l   # this worktree's own database
   curl -sS --max-time 5 -o /dev/null https://example.com && echo LEAK || echo "blocked ✅"
   ```
   The last line **must** print `blocked ✅`. If it prints `LEAK`, do **not** run
   unattended — the firewall didn't apply.

   > Run these **inside the container**. On your Mac the leak check always prints
   > `LEAK` — there's no firewall out there.

5. **Launch the unattended run:**
   ```
   claude --dangerously-skip-permissions
   ```
   then `/openspec-apply-change`, or headless:
   ```
   claude -p "Work through openspec/changes/<change>/tasks.md in order — implement each task, \
   run make lint and make test, check it off, and commit per task." --dangerously-skip-permissions
   ```

**Suggestion:** watch the first task or two to confirm it's on the rails, then let it run.

---

## Running the app and the tests inside the box

Services are compose siblings on the same network, so reach them **by service name** —
`localhost` in here is the box itself. `devcontainer.json` already sets the env, and
`godotenv.Load()` never overrides real env vars, so these win over `.env`:

```
DB_HOST=postgres        STORAGE_ENDPOINT=http://minio:9000
```

```
make migrate-up         # the scripts honour DB_* now; no host stack needed
make run                # API on :8080 in here → the api port from [init] on your Mac
make run-mcp            # MCP on :8090 in here → the mcp port on your Mac
make test               # unit tests
make test-acceptance    # testcontainers → SIBLING containers on the host daemon
make lint               # golangci-lint v2, matching .golangci.yml
```

`make test-acceptance` works because of two things that are easy to lose: the host
Docker socket (DooD) and `TESTCONTAINERS_HOST_OVERRIDE=host.docker.internal` — the
throwaway Postgres publishes on a **host** port, so without the override every test
waits forever on a port that never opens in here.

The root `docker-compose.yml` / `make sync-up` stack is for running on your **Mac**,
not in here — it hardcodes `container_name:` and fixed ports, which is precisely what
stops two worktrees coexisting.

---

## Safety model & caveats

- **Bypass mode has no prompt-injection defence.** The protection is *containment*,
  not judgment — only run it against code and specs you trust.
- **The firewall is egress-only and IP-based, resolved at start.** It does not
  deep-inspect traffic, so data could in principle be pushed to an *allowlisted* host
  (a GitHub gist). Port 53 is open to any destination — DNS must work before the
  allow-list can be built — so a determined DNS exfiltration path remains. If a CDN
  rotates IPs mid-session, re-run the script to re-resolve.
- **Blanket sudo is revoked.** The base image ships `/etc/sudoers.d/vscode` granting
  `vscode ALL=(ALL) NOPASSWD: ALL`, which would let the agent `iptables -F` its own
  fence off. The Dockerfile deletes it and **asserts** the end state at build time;
  `post-create.sh` re-checks, because features install afterwards. What's left:
  `laguna-firewall.sh` and the DooD init script. Cost: no `sudo apt-get` in the box —
  add packages to the `Dockerfile` and rebuild, which is reproducible anyway.
- **The host Docker socket is the real hole.** `make test-acceptance` needs it, and
  anything holding it can start a privileged sibling container that bind-mounts the
  host `/`. So revoking sudo raises the bar against the agent casually undoing its own
  guardrail; it is **not** a defence against a determined escape. Trusted repos only.
  To close it later, front the socket with a Docker socket proxy that whitelists API
  calls.
- **Image pulls bypass the firewall** — `postgres:16-alpine`, `minio`, and
  testcontainers' images are pulled by the *host* daemon, so Docker Hub doesn't need
  to be allowlisted. The firewall governs traffic originating *inside* the box.

---

## Maintaining the firewall allow-list

If a legit download fails with a connection timeout, the allow-list is probably
missing a host. Fix without rebuilding:

1. Add the domain to `ALLOWED_DOMAINS` in `init-firewall.sh`.
2. Re-apply: `sudo /usr/local/bin/laguna-firewall.sh`.

Currently allowed: Anthropic APIs + `claude.ai` (the MCP servers this project uses);
Go proxy/checksum/storage and `vuln.go.dev` (govulncheck); `registry.npmjs.org`; all
GitHub server ranges from `api.github.com/meta`; this box's attached container subnets
(its own Postgres/MinIO); and the host (`host.docker.internal` + default gateway, where
testcontainers publishes).

---

## Troubleshooting

| Symptom | Likely cause / fix |
|---|---|
| `LEAK` from the example.com check | Firewall didn't apply. Re-run `sudo /usr/local/bin/laguna-firewall.sh` and read its log; confirm `cap_add` NET_ADMIN/NET_RAW and that the container was **rebuilt**, not just restarted. |
| `fatal: not a git repository` | The git-dir mount is missing — check the `[init] git dir` line, and that `docker-compose.generated.yml` lists two volumes for a worktree. |
| `docker ps` → permission denied | The DooD socat proxy didn't start: `sudo /usr/local/share/docker-init.sh`. Until it works, `make test-acceptance` will fail. |
| Acceptance tests hang on container startup | `TESTCONTAINERS_HOST_OVERRIDE` lost — it must be `host.docker.internal`. |
| `make lint` / `mockery`: command not found | `post-create.sh` didn't finish. Re-run it: `.devcontainer/post-create.sh`. |
| Claude asks to log in again | `~/.claude` is root-owned (the volume was seeded before the Dockerfile created the dir) or `CLAUDE_CONFIG_DIR` isn't set. |
| Ports moved unexpectedly | Something else held the block. `.devcontainer/.ports` records the assignment; delete it (or set `LE_REPICK_PORTS=1`) to re-pick. |
| `--dangerously-skip-permissions` refuses to start | Running as root — confirm `remoteUser: vscode`. |
| Two boxes fighting over a port | A `.devcontainer/.ports` copied between worktrees. It records its owner path and re-picks when it doesn't match, so delete it and reopen. |

---

## Review checklist (before committing changes to this folder)

- [ ] Go version in `devcontainer.json` still matches `go.mod`.
- [ ] `initialize.sh` prints a git-dir line in a worktree, and none in the main checkout.
- [ ] Two boxes open at once get different port blocks.
- [ ] `git status`, `make lint`, `make test`, `make test-acceptance` all work inside the box.
- [ ] Startup log reaches `[firewall] active …` and the leak check prints `blocked ✅`.
- [ ] `sudo -l` inside the running box lists only the two scoped scripts.
