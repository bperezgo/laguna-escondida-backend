# Landing — Agent Rules

Applies to everything under `landing/`; paths below are relative to it, and every `pnpm` command runs from here. Repo-wide rules: `../AGENTS.md`.

Static Astro site in Spanish (es-CO) that sells the product to restaurant owners, sends them to the 14-day trial and collects demo requests. Astro (`output: "static"`), TypeScript strict, Tailwind v4, pnpm. Working product name: **Kuenta**.

## Rules

- **The site uses two backend addresses, each with its contract.** The demo form's `POST /api/public/leads` (`../backend/docs/api/leads.md`), and the installer download link `GET /api/public/edge/installer` (`../backend/docs/api/edge-downloads.md`), a plain link on `/descargar` built by `installerDownloadUrl` in `src/config/`. Read the doc before changing either.
- **One configuration module.** Product name, site URL, backend address, tracking ids and the WhatsApp number live in `src/config/` (`site.ts`, `build-site-config.ts`, `defaults.ts`), read from `PUBLIC_*` variables listed in `.env.example`. Never hard-code them in a page or component. A missing `PUBLIC_API_BASE_URL` fails the build; a missing tracking id turns that tool off.
- **All analytics events go through `src/lib/track.ts`.** It is the only caller of `gtag` and `fbq`, every call is guarded, and no event ever carries a name, restaurant or phone. Fire the lead event only after the backend answers `201`.
- **`DESIGN.md` wins over `.claude/skills/landing-page-design/SKILL.md`.** The skill is third party and pinned (see its `NOTICE`); do not edit it, and do not run an installer for it. Use the named text styles and tokens in `src/styles/global.css`.
- **No invented proof.** No testimonials, customer logos or figures unless the owner supplies them. The only prices on the site are the plans the owner supplied, held in `src/config/plans.ts` with a test, and `pnpm check:prices` (run by `release-landing.yml`, not by CI) fails the release when they differ from the cloud's `GET /api/plans` (`../backend/docs/api/plans.md`); nothing else gets a price (the WhatsApp agent has none). Screenshots show demo data only.
- **The WhatsApp agent is another product.** It is presented as an add-on contracted apart, and no text says a plan includes it.
- **Two actions, computed once** by `siteActions` in `src/config/actions.ts` and rendered by `PrimaryAction.astro` (filled) and `SecondaryAction.astro` (outlined). With `PUBLIC_SIGNUP_URL` set, the primary is `Prueba 14 días` to the signup page and the secondary is `Pide una demo` to `/#demo`; without it, the demo request is the primary and there is no trial or sign-in link. In the pricing section each plan's trial action adds `plan=<id>` to the signup address (`planAction`); elsewhere it carries no plan. Never write either label in a component.
- **Apps never import each other's code.** Talk to the backend over HTTP only.
- This is a self-contained pnpm project with its own lockfile, **not part of the `cloud/` or `edge/` releases**: a `landing/v*` tag (`release-landing.yml`) builds the static `dist/`, deploys it to Cloudflare Pages and attaches it to a GitHub Release. `PUBLIC_*` values come from repo variables, Cloudflare credentials from repo secrets.
- Comments: the default is none. Add one only when the *why* isn't obvious.

## Commands

`pnpm dev` · `pnpm build` (needs `PUBLIC_API_BASE_URL`) · `pnpm check` (Astro and TypeScript) · `pnpm test` (node test runner, `src/**/*.test.ts` and `scripts/**/*.test.ts`) · `pnpm check:prices` (needs `PUBLIC_API_BASE_URL` and the cloud up). After a change, run check, test and build.
