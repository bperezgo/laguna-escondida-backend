# Landing — Agent Rules

Applies to everything under `landing/`; paths below are relative to it, and every `pnpm` command runs from here. Repo-wide rules: `../AGENTS.md`.

Static Astro site in Spanish (es-CO) that sells the product to restaurant owners and collects demo requests. Astro (`output: "static"`), TypeScript strict, Tailwind v4, pnpm. Working product name: **Kuenta**.

## Rules

- **The backend contract is `../backend/docs/api/leads.md`.** The only runtime dependency is one `POST /api/public/leads`. Read the doc before changing the form's request or error handling.
- **One configuration module.** Product name, site URL, backend address, tracking ids and the WhatsApp number live in `src/config/` (`site.ts`, `build-site-config.ts`, `defaults.ts`), read from `PUBLIC_*` variables listed in `.env.example`. Never hard-code them in a page or component. A missing `PUBLIC_API_BASE_URL` fails the build; a missing tracking id turns that tool off.
- **All analytics events go through `src/lib/track.ts`.** It is the only caller of `gtag` and `fbq`, every call is guarded, and no event ever carries a name, restaurant or phone. Fire the lead event only after the backend answers `201`.
- **`DESIGN.md` wins over `.claude/skills/landing-page-design/SKILL.md`.** The skill is third party and pinned (see its `NOTICE`); do not edit it, and do not run an installer for it. Use the named text styles and tokens in `src/styles/global.css`.
- **No invented proof.** No testimonials, customer logos, prices or figures unless the owner supplies them. Screenshots show demo data only.
- **One primary action**, labelled `Pide una demo`, pointing at `#demo`.
- **Apps never import each other's code.** Talk to the backend over HTTP only.
- This is a self-contained pnpm project with its own lockfile, **not part of the `v*` release**: the site ships on its own as a static `dist/`.
- Comments: the default is none. Add one only when the *why* isn't obvious.

## Commands

`pnpm dev` · `pnpm build` (needs `PUBLIC_API_BASE_URL`) · `pnpm check` (Astro and TypeScript) · `pnpm test` (node test runner, `src/**/*.test.ts`). After a change, run check, test and build.
