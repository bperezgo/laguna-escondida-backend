## 1. Backend: lead domain

- [x] 1.1 Add `internal/domain/dto/lead.go` (lead, attribution, status and alert-status constants, create/update/list requests) and `internal/domain/ports/leads.go` (`LeadRepository`: create, get, list, update, delete, find recent by phone, count since, claim due alerts, mark alert). Run `make generate-mocks` and verify with `go build ./...`
- [x] 1.2 Add the `lead` aggregate: required-field validation naming the field, phone through `shared.NewPhone(raw, "CO")`, consent required, and attribution fields validated one by one with the invalid ones dropped and returned. Verify with unit tests for every scenario of "Required fields are validated" and "Invalid attribution never blocks a lead" in the `sales-leads` spec
- [x] 1.3 Add `LeadService.Submit`: honeypot answers success and stores nothing; a phone seen in the last 24 hours stores nothing; more than 300 leads in the last hour returns a rate-limit error; otherwise store with alert status `pending` or `skipped` depending on configuration, and log dropped attribution fields. Verify with service tests for honeypot, duplicate, global cap, and both alert statuses
- [x] 1.4 Add `LeadService` list, get, update (status and notes only) and delete. Verify with service tests that an unknown status is rejected and that update cannot change name, phone or attribution

## 2. Backend: persistence and routes

- [x] 2.1 Add the cloud migration `create_leads` at the next free number, with a down file: no `organization_id`, no row-level security, indexes on `created_at` and `(phone, created_at)`, and the alert columns. Verify that `make migrate-up` then `make migrate-down` run cleanly
- [x] 2.2 Implement `lead_repository.go` on the plain `*gorm.DB`, including claiming due alerts with `FOR UPDATE SKIP LOCKED`. Verify with integration tests (`DB_HOST=`): create and read back with attribution, recent-by-phone, count since, list newest first with a status filter, delete, and a claimed alert not claimed twice
- [x] 2.3 Add `lead_handler.go`: the public submit (16 KB body limit, `201 {"status":"received"}`, `400` with `field`, `413`, `429` with `Retry-After`) and the four platform handlers. Verify with handler tests for each status code and that the public response never contains an id
- [x] 2.4 Add `rate_limit_middleware.go`: in-memory fixed window per client address with a sweep of expired keys. Verify with unit tests: the sixth request in a window is `429` with `Retry-After`, a new window resets, and two addresses don't share a count
- [x] 2.5 Register `POST /api/public/leads` (limiter, no auth) and `/api/platform/leads` (admin API key) in `routes_cloud.go` only, wire them in `cloud.go`, and update `testdata/routes_cloud.golden`. Verify that the route golden tests pass, the edge golden file is unchanged, and a route test shows `401` for a user token on `/api/platform/leads`
- [x] 2.6 Add the optional `TRUSTED_PROXIES` cloud setting and pass it to `router.SetTrustedProxies`; unset keeps today's behaviour. Verify with config tests (unset, valid list, malformed entry fails startup) and a test that a forged `X-Forwarded-For` is ignored when the peer is not trusted

## 3. Backend: lead alerts

- [x] 3.1 Add `internal/platform/config/leads.go` for `LEAD_ALERT_DISCORD_WEBHOOK_URL` (missing means off, malformed fails startup) and load it in the cloud config. Verify with config tests for the three cases
- [x] 3.2 Add `LeadAlertService.DeliverDue`: claim due `pending` leads, build the message (name, restaurant, phone, `utm_source`, `utm_campaign`), send through `ports.StaffNotificationSender`, mark `sent`, or back off from 15 seconds doubling to 30 minutes, and mark `failed` with an error log after 24 hours. Verify with service tests: sent, retried after a `503`, failed after 24 hours, and `skipped` leads never claimed
- [x] 3.3 Add `lead_alert_scheduler.go` (a 15-second singleton job, not per organization), started from `cloud.go` only when the webhook is configured. Verify with a scheduler test and `go build ./...`

## 4. Backend: docs and checks

- [x] 4.1 Write `backend/docs/api/leads.md` (both route groups, fields, error table, limits, the two environment settings) and add examples to `docs/examples/curl-examples.md`. Verify each example against a local cloud binary
- [x] 4.2 Run `make test` and `make lint` from `backend/`. Verify both pass

## 5. Landing: scaffold and design system

- [x] 5.1 Create `landing/` as an Astro static project with TypeScript strict, Tailwind v4 and pnpm (same Node and pnpm versions as `frontend/`), plus `.env.example`. Verify that `pnpm install`, `pnpm astro check` and `pnpm build` succeed and produce `dist/index.html`
- [x] 5.2 Read `skills/landing-page-design/SKILL.md` from `elayadesign/ai-design-skills` in full, then copy it by hand to `landing/.claude/skills/landing-page-design/SKILL.md` with a `NOTICE` recording the source commit and MIT licence. Verify that the file contains no instruction to run commands or fetch URLs, and that the skill is listed with a `landing` prefix in a new session
- [x] 5.3 Write `landing/DESIGN.md` with the overrides of design D2, and the tokens in `src/styles/global.css` (colours, Manrope self-hosted, spacing, radius, motion). Verify that the primary button's text contrast is at least 4.5:1
- [x] 5.4 Add `src/config/site.ts` reading the `PUBLIC_*` values, failing the build when `PUBLIC_API_BASE_URL` is missing. Verify that a build without it fails with a clear message and a build with it succeeds
- [x] 5.5 Add `landing/AGENTS.md`, `landing/CLAUDE.md` (`@AGENTS.md`) and fill the `landing/` row of the root `AGENTS.md`, noting it is outside the `v*` release. Verify the three files exist and the commands they name run

## 6. Landing: tracking and attribution

- [x] 6.1 Add `src/lib/attribution.ts`: pure parsing of the query, landing URL and referrer, plus first-touch storage in `sessionStorage` with a fallback when storage throws. Verify with `node --test`: all seven parameters, first touch not overwritten, and the fallback path
- [x] 6.2 Add `src/components/Analytics.astro` and `src/lib/track.ts` (`trackLead(eventId)`, `trackWhatsApp()`), each tag injected only when its id is set and every call a no-op when the global is missing. Verify with `node --test` that calls don't throw without globals and send the event names of design D5, and that a build without ids contains neither tag URL in `dist/`
- [x] 6.3 Add `src/layouts/Base.astro` with title, description, canonical, social metadata, the footer link to the policy, and the attribution capture on every page. Verify in the built HTML of both pages

## 7. Landing: page and form

- [ ] 7.1 Capture product screenshots from the operations UI running on demo data only, and add them optimised to `public/`. Verify by reviewing each image for real names, phones and addresses
- [x] 7.2 Build the sections of design D3 in `src/components/sections/` and `src/pages/index.astro`, in Spanish, following the skill and `DESIGN.md`, with `Pide una demo` as the only primary label and the fixed bottom bar on phones. Verify at 360 by 640 that the headline and action are visible without scrolling, and that the page has no invented testimonial, logo, price or figure
- [x] 7.3 Add `LeadForm.astro` and `src/lib/lead-form.ts`: three fields, consent unchecked with the policy link, the off-screen `website` honeypot, a disabled control while sending, success replacing the form, a field error on `400`, and the general error with the WhatsApp action otherwise. Verify with `node --test` for the request body and the response mapping, and by hand against a local backend for each scenario of "The lead form" in the `landing-site` spec
- [x] 7.4 Add the WhatsApp action (hidden when no number is configured) and call `trackWhatsApp()` on it; call `trackLead` only after a `201`. Verify in the browser's network panel that a failed submission sends no lead event and that no event carries a name or phone
- [x] 7.5 Write `src/pages/privacidad.astro` covering the four points of the "privacy policy" requirement, and add the sitemap and `robots.txt`. Verify that the page is linked from the form and the footer, and that `dist/` contains the sitemap

## 8. CI

- [ ] 8.1 Add `.github/workflows/landing-ci.yml` from `frontend-ci.yml`, filtered to `landing/**`: install, `astro check`, test, build with a placeholder `PUBLIC_API_BASE_URL`. Verify that the workflow passes on the pull request and does not run for a backend-only change
- [ ] 8.2 Check that `security.yml` covers `landing/` (dependency and secret scans). Verify from the workflow's run on the pull request

## 9. End to end

- [ ] 9.1 With the cloud binary and the built site running locally, submit a lead that arrived with `?utm_source=facebook&utm_campaign=test&fbclid=abc` after visiting the policy page. Verify the stored row's attribution and `event_id`, the alert in a test Discord channel, and `GET /api/platform/leads` with the admin API key
- [ ] 9.2 Run a mobile Lighthouse audit on the built home page. Verify performance and accessibility scores of at least 90, and fix what falls short
- [x] 9.3 Run `openspec validate landing-lead-capture --strict`. Verify it passes
