## 1. One-time setup (owner, by hand)

- [ ] 1.1 Create the Pages project with Direct Upload and production branch `main` (`wrangler pages project create kuenta-landing --production-branch=main`); verify it is listed in the Cloudflare dashboard and note its `*.pages.dev` address
- [ ] 1.2 Create a Cloudflare API token with only Account > Cloudflare Pages > Edit on this account; verify with `curl -H "Authorization: Bearer <token>" https://api.cloudflare.com/client/v4/user/tokens/verify` returning `"status": "active"`
- [ ] 1.3 Add repo secrets `CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ACCOUNT_ID`; verify `gh secret list` shows both
- [ ] 1.4 Add repo variables `PUBLIC_SITE_URL` (the address from 1.1) and the optional `PUBLIC_GA_MEASUREMENT_ID`, `PUBLIC_META_PIXEL_ID`, `PUBLIC_WHATSAPP_NUMBER`, `PUBLIC_LEGAL_NAME`, `PUBLIC_PRIVACY_EMAIL` as available; leave `PUBLIC_API_BASE_URL` unset until 3.1; verify with `gh variable list`

## 2. Workflow

- [x] 2.1 In `release-landing.yml`, replace the placeholder build env with `vars.PUBLIC_*`; verify a run with `PUBLIC_API_BASE_URL` unset fails at Build naming the variable
- [x] 2.2 Add a pinned wrangler deploy step (`pages deploy dist --project-name=<project> --branch=main`) after packaging and before the release, using the two secrets; verify with `actionlint` and that the release step runs only after the deploy
- [x] 2.3 Add `workflow_dispatch` with a required `tag` input that checks out that tag, builds and deploys, and skips the release step; verify with `actionlint` and that the release step's condition is `github.event_name == 'push'`
- [x] 2.4 Update the workflow's header comment and `landing/AGENTS.md` to say the site deploys to Cloudflare Pages and reads `PUBLIC_*` from repo variables; verify `pnpm check` still passes from `landing/`

## 3. First release (after the domain is bought)

- [ ] 3.1 Once `https://api.laguna-escondida.com` answers over HTTPS, set `PUBLIC_API_BASE_URL=https://api.laguna-escondida.com` (origin only, no `/api`); verify `curl -i -X OPTIONS https://api.laguna-escondida.com/api/public/leads` returns `Access-Control-Allow-Origin`
- [ ] 3.2 Push `landing/v1.0.0`; verify the run passes, the `pages.dev` address serves the new build, and the release carries the zip and sha256
- [ ] 3.3 Submit a test lead from the live site; verify it is returned by `GET /api/platform/leads`, then delete it
- [ ] 3.4 Run the workflow by hand with `tag=landing/v1.0.0`; verify it deploys and creates no new release
