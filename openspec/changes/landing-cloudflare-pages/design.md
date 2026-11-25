## Context

See proposal.md for why. Today `release-landing.yml` runs on `landing/v*`: check, test, build with `PUBLIC_API_BASE_URL=http://127.0.0.1:8080`, then zip `dist/` and `gh release create`. The landing build already fails when `PUBLIC_API_BASE_URL` is empty (`landing/src/config/build-site-config.ts`), and the form appends `/api/public/leads` to that value (`landing/src/lib/lead-form.ts`). The backend sends `Access-Control-Allow-Origin: *`, so any Pages origin can post a lead.

## Goals / Non-Goals

**Goals:**
- A tag is the only thing that changes the live site, and what is live matches the tagged commit.
- Moving from `pages.dev` to a bought domain needs no workflow change.

**Non-Goals:**
- Preview deployments for pull requests or branches.
- Building without a backend address (the form would post nowhere; the build keeps failing).
- `noindex` while on `pages.dev`, and the redirect to the future domain. Decide when the domain is bought.
- Managing Cloudflare resources from code (Terraform). The project, token and domain are set up once by hand.

## Decisions

### 1. GitHub Actions uploads the build (Direct Upload), not Cloudflare's Git integration
The workflow runs `wrangler pages deploy dist --project-name=<project> --branch=main` after the build.
*Why:* the Git integration builds on branch pushes and cannot trigger on a tag; it would also deploy every merge to `main`, skipping the release gate. Direct Upload keeps one build for both the Release zip and the live site, and keeps check/test in front of it.
*Alternative:* Workers with static assets, which Cloudflare now favours for new sites. Rejected for now: Pages is enough for a static `dist/` and serves `pages.dev` directly; switching later changes only the deploy command.

### 2. `--branch=main` makes it the production deployment
A Direct Upload project's production branch is the one named at creation (`main`). Deploying with that branch updates the production address; any other branch name would make a preview.

### 3. `PUBLIC_*` values are repository variables, Cloudflare credentials are secrets
Everything `PUBLIC_*` is compiled into public JavaScript, so it is a variable (`vars.*`), visible and easy to change. `CLOUDFLARE_API_TOKEN` (permission: Account > Cloudflare Pages > Edit, this account only) and `CLOUDFLARE_ACCOUNT_ID` are secrets. Unset optional variables expand to empty strings, which the build already treats as "off".

### 4. Deploy before the release
Order: check, test, build, package, deploy, release. If the deploy fails no release is created, so a `landing/*` release always means "this was live". Rerunning the job is safe: the deploy is idempotent and `gh release create` runs only after it.

### 5. Rollback through `workflow_dispatch`
The workflow also accepts a manual run with a `tag` input. It checks out that tag, builds and deploys, and skips the release step (the release already exists). The Pages dashboard's own rollback also works, but a manual run keeps GitHub as the record.

### 6. Wrangler is pinned
Run `pnpm dlx wrangler@<exact version>` (or `cloudflare/wrangler-action` pinned by commit SHA, as the other actions are). No new dependency in `landing/package.json`.

## Implementation map

- `.github/workflows/release-landing.yml`: the only workflow changed.
- `.github/workflows/release-cloud.yml`: reference for action pinning style only.
- `landing/AGENTS.md`: one line on where the site deploys and which variables it reads.
- `landing/src/config/build-site-config.ts`: read-only, the list of `PUBLIC_*` variables.

## Risks / Trade-offs

- [The Pages project name is the subdomain and cannot be renamed; Cloudflare adds a suffix if it is taken] → Create the project first, read the address it gets, and set `PUBLIC_SITE_URL` from that.
- [`PUBLIC_API_BASE_URL` set with a path (`.../api`) gives `/api/api/public/leads`] → Set the origin only: `https://api.laguna-escondida.com`. Smoke-test the form after the first deploy.
- [`api.laguna-escondida.com` does not answer yet] → Do not push a landing tag until it does; the form would show a network error to every prospect.
- [A leaked token could replace the site] → Scope it to Pages: Edit on this account only, keep it out of `pull_request` workflows (the workflow runs only on tags and manual runs).
- [Search engines index `pages.dev` before the real domain] → Accepted for now; a redirect when the domain is bought moves the ranking. See Non-Goals.

## Migration Plan

1. Owner: create the Pages project (`wrangler pages project create kuenta-landing --production-branch=main`), the scoped API token, and the repo secrets and variables.
2. Merge the workflow change.
3. When `https://api.laguna-escondida.com` answers, push `landing/v1.0.0`, then submit a test lead from the live site and check it arrives (`GET /api/platform/leads`).
4. Later, with the bought domain: attach it to the Pages project, update `PUBLIC_SITE_URL`, push a new tag.

Rollback: run the workflow by hand with the previous tag, or roll back in the Pages dashboard.

## Open Questions

- Final Pages project name (proposed `kuenta-landing`); only affects the `pages.dev` address.
