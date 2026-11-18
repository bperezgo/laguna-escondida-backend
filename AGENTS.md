# Laguna Escondida — Monorepo Agent Rules

One repo, several apps. Each app has its own rules file; read the one for the folder you're working in.

| Folder | What | Stack | Rules | Commands run from |
|---|---|---|---|---|
| `backend/` | API (cloud + edge binaries, MCP server) | Go, Gin, GORM, Postgres | `backend/AGENTS.md` | `backend/` (`make lint`, `make test`, …) |
| `frontend/` | Operations UI (cloud + Windows edge box) | Next.js, React, pnpm | `frontend/AGENTS.md` | `frontend/` (`pnpm lint`, `pnpm test`, …) |
| `landing/` | Product marketing site (planned) | — | — | — |

Repo-wide: `openspec/` (specs and changes — one change may span apps), `docs/WORKFLOW.md` (how a change goes from idea to merge), `.github/workflows/` (one CI workflow per app, filtered by path; `security.yml` covers everything).

## Cross-app rules

- **The API contract lives in `backend/docs/api/{entity}.md`.** A change to an endpoint updates that doc in the same PR as the backend change and the frontend code that uses it.
- **A feature that touches the backend and the frontend ships as one change and one PR.** Don't split it across branches.
- **Run each app's checks from its own folder**, and only for apps the change touches.
- **Apps never import each other's code.** They talk over HTTP only.
- **One `v*` tag releases everything**: the backend image, the edge-node binary and the frontend Windows bundle, all from the same commit (`.github/workflows/release.yml`).
