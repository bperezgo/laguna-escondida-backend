# Laguna Escondida — Monorepo Agent Rules

One repo, several apps. Each app has its own rules file; read the one for the folder you're working in.

| Folder | What | Stack | Rules | Commands run from |
|---|---|---|---|---|
| `backend/` | API (cloud + edge binaries, MCP server) | Go, Gin, GORM, Postgres | `backend/AGENTS.md` | `backend/` (`make lint`, `make test`, …) |
| `frontend/` | Operations UI (cloud + Windows edge box) | Next.js, React, pnpm | `frontend/AGENTS.md` | `frontend/` (`pnpm lint`, `pnpm test`, …) |
| `landing/` | Product marketing site (static, es-CO) | Astro, Tailwind, pnpm | `landing/AGENTS.md` | `landing/` (`pnpm check`, `pnpm test`, `pnpm build`) |
| `edge/` | Windows edge-box appliance (services, proxy, updater, install scripts) | PowerShell, WinSW, Caddy | `edge/AGENTS.md` | `edge/` (scripts run on the Windows box; CI is `edge-ci.yml`) |

Repo-wide: `openspec/` (specs and changes — one change may span apps), `docs/WORKFLOW.md` (how a change goes from idea to merge), `.github/workflows/` (one CI workflow per app, filtered by path; `security.yml` covers everything).

## Cross-app rules

- **The API contract lives in `backend/docs/api/{entity}.md`.** A change to an endpoint updates that doc in the same PR as the backend change and the frontend code that uses it.
- **A feature that touches the backend and the frontend ships as one change and one PR.** Don't split it across branches.
- **Run each app's checks from its own folder**, and only for apps the change touches.
- **Apps never import each other's code.** They talk over HTTP only.
- **`edge/` consumes the apps only as release artifacts** (edge-node binary, frontend bundle), by tag and verified by sha256. It never reads `backend/` or `frontend/` source.
- **Three release channels, three tag prefixes** (a bare `v*` tag releases nothing). `cloud/vX.Y.Z` pushes the backend and web (`laguna-web`, run by aws-infra) images to ECR, with the unprefixed version as the image tag (`release-cloud.yml`). `edge/vX.Y.Z` is the appliance release: one GitHub Release with the appliance zip, edge-node and Next.js Windows zips and a `manifest.json` carrying `min_cloud` from `edge/compat.json` (`release-edge.yml`); boxes follow only these, and only when the live cloud satisfies `min_cloud`. `landing/vX.Y.Z` builds the static site (`release-landing.yml`). `scripts/edge-fingerprint.sh` says whether the edge inputs changed; an unchanged edge is not re-released. The cloud rejects edges below `MIN_EDGE_VERSION`.
