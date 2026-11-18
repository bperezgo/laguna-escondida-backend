# Laguna Escondida Frontend — Agent Rules

Applies to everything under `frontend/`; paths below are relative to it, and every `pnpm` command runs from here. Repo-wide rules: `../AGENTS.md`.

Next.js (App Router) + React, TypeScript, pnpm. The same app serves the cloud and the Windows edge box, as a standalone build (`output: "standalone"` in `next.config.js`).

## Layout

- `app/`: routes. `(main)/` holds the logged-in screens; `app/api/` holds server routes.
- `components/<feature>/`: UI for each feature. Shared primitives live in `components/ui/`.
- `lib/`: API clients and helpers. Backend calls go through `lib/api/`.
- `types/`: shared TypeScript types.

## Rules

- **The backend contract is `../backend/docs/api/{entity}.md`.** Read it before calling an endpoint; never guess request or response shapes.
- **`NEXT_PUBLIC_*` values are inlined at build time**, so the edge bundle is the same for every box. Don't put per-box values in them.
- **This is a self-contained pnpm project** with its own lockfile, not a root workspace: the edge release depends on the standalone layout.
- Comments: the default is none. Add one only when the *why* isn't obvious.

## Commands

`pnpm dev` · `pnpm build` · `pnpm test` (node test runner, `**/*.test.ts`) · `pnpm lint` · `pnpm exec tsc --noEmit`. After a change, run typecheck, test and lint.
