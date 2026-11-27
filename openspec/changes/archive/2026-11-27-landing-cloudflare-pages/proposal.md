## Why

A `landing/v*` tag builds the marketing site and attaches `dist/` to a GitHub Release, but the site is not published anywhere, so no prospect can reach it or request a demo. `split-release-tags` left the deploy target open; it is now Cloudflare Pages.

## What Changes

- `release-landing.yml` deploys the built `dist/` to a Cloudflare Pages project with `wrangler pages deploy` (Direct Upload), after check, test and build pass. The GitHub Release with the zip and sha256 stays; both come from the same build.
- The build reads its `PUBLIC_*` settings from GitHub repository variables instead of the `http://127.0.0.1:8080` placeholder. They end up in public JavaScript, so they are variables, not secrets.
- Cloudflare credentials (`CLOUDFLARE_API_TOKEN` scoped to Pages: Edit, `CLOUDFLARE_ACCOUNT_ID`) are repository secrets.
- The site is served at `https://<project>.pages.dev` until a custom domain is bought; moving to it is a variable change and a new tag, not a workflow change.
- A redeploy of an existing tag is possible by hand (`workflow_dispatch`), for rollback.
- No landing source change. A missing `PUBLIC_API_BASE_URL` still fails the build, so nothing deploys until the variables are set.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `release-channels` (added by `split-release-tags`, not yet archived): the landing tag now publishes the site to Cloudflare Pages instead of only attaching it to a release.

## Impact

- **Code**: `.github/workflows/release-landing.yml` only. `landing/AGENTS.md` notes where the site deploys.
- **Systems**: a new Cloudflare Pages project (proposed name `kuenta-landing`), repo variables `PUBLIC_API_BASE_URL` (`https://api.laguna-escondida.com`, origin only: the form adds `/api/public/leads`), `PUBLIC_SITE_URL` (the `pages.dev` address), and the optional tracking/contact `PUBLIC_*` values.
- **Backend**: none. `POST /api/public/leads` already allows any origin.
- **Ordering**: archive `split-release-tags` before this change, since this modifies the requirement it adds.
- **Blocked on the owner**: the API host must answer at `https://api.laguna-escondida.com` before the first tag; until the domain is bought the variables stay empty and a landing tag fails at Build by design.
