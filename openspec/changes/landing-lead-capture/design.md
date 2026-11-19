## Context

See `proposal.md` for the motivation. What shapes the approach:

- **`landing/` does not exist.** No app in the repo loads an analytics tag.
- **CORS is already open.** `handler.CORSMiddleware`
  (`internal/platform/handler/middleware.go:~29`) answers every origin and
  allows `Content-Type`, so a browser on the landing origin can post JSON
  with no backend change.
- **There is no rate limiting** anywhere in the backend, and nothing sets
  gin's trusted proxies, so `c.ClientIP()` believes any
  `X-Forwarded-For`.
- **Platform data already has a pattern.** `organizations` has no
  `organization_id` and no row-level security; `OrganizationRepository`
  takes the plain `*gorm.DB`, and its routes sit behind
  `handler.AdminAPIKeyMiddleware` in `routes_cloud.go:~37`.
- **The durable event bus and staff notifications are per organization**
  (`staff-notifications` D3, D8): the relay and the rules run inside one
  organization's database session. A lead has no organization.
- **A Discord sender exists.**
  `httpclient.NewDiscordWebhookClient` implements
  `ports.StaffNotificationSender` and already classifies 2xx, 401/403/404,
  429 and the rest.
- **Phones:** `shared.NewPhone(raw, homeRegion)` reads typed input against
  a home region and stores E.164.
- **The operations UI's look:** teal `#14b8a6`, gold `#c9a227`, slate
  neutrals, system fonts (`frontend/app/globals.css`).
- **Hosting infrastructure is outside this repo** (CDK).

## Goals / Non-Goals

**Goals:**
- A static site whose only runtime dependency is one `POST`.
- One place for brand and tracking configuration, and one function through
  which every analytics event passes.
- A lead is never lost because an alert or a tracking script failed.
- Leads follow the existing platform-data pattern; nothing about tenant
  isolation changes.

**Non-Goals:**
- Deploying the site: this change delivers a static `dist/` and documents
  its build settings. Wiring a host and DNS is done where the
  infrastructure lives.
- Putting `landing/` in the `v*` release. It ships on its own.
- A/B testing, a blog, more languages, a CMS.
- An operator screen for leads in `frontend/`. The platform endpoints and
  the alert are enough for the first leads.
- Moving lead alerts onto the durable event bus.

## Decisions

### Landing site

**D1. Astro, static output, Tailwind, no UI framework.**
- `output: 'static'`, TypeScript strict, pnpm with its own lockfile, the
  same Node and pnpm versions as `frontend/`.
- Tailwind v4 through its Vite plugin, with tokens in
  `src/styles/global.css` (`@theme`).
- The form, attribution and tracking are plain TypeScript modules loaded
  by an Astro `<script>`. No React island: three inputs don't justify a
  runtime.
- Alternative: a Next.js static export, to share a stack with `frontend/`.
  Rejected: the owner already uses Astro, and it ships no JavaScript for
  the content sections.

**D2. Design system: the `landing-page-design` skill, scoped and with its
palette replaced.**
- Copy `skills/landing-page-design/SKILL.md` from
  `elayadesign/ai-design-skills` (MIT) by hand to
  `landing/.claude/skills/landing-page-design/SKILL.md`. Record the source
  commit and licence in a `NOTICE` beside it. Do not run its installer.
  Read the file in full before committing it.
- Living under `landing/` keeps it away from `frontend/`, whose system
  fonts the skill bans.
- `landing/DESIGN.md` records the overrides, and wins over the skill:
  - **Light and warm**, not the skill's dark greys. Background off-white,
    text slate `#0f172a`.
  - **Primary teal.** `#0f766e` for buttons and links (white text on
    `#14b8a6` fails contrast); `#14b8a6` for accents and illustration.
  - **Gold `#c9a227`** only as a highlight, never for text.
  - **One dark section**, for the "internet is down" story.
  - **Manrope**, self-hosted, one family, no italics (from the skill's
    list).
  - **Phosphor icons.**
  - Kept from the skill as written: the type scale, spacing tokens, radius
    rule, motion rules, the copy bans, and no invented proof.
- Alternative: reuse the operations UI's CSS variables directly. Rejected:
  apps never import each other's code, and an operations UI's density is
  wrong for a marketing page. Only the colours carry over.

**D3. Page brief** (the skill's intake, answered from the exploration):
- **Visitor:** owner or manager of a restaurant in Colombia, cold, from a
  Meta or Google ad, on a phone.
- **Action:** request a demo. Label everywhere: `Pide una demo`.
- **Objections:** the internet drops; is it valid for DIAN; is it hard to
  switch; will my staff learn it.
- **Proof:** it runs a real restaurant every day and was built by its
  owner. Real screenshots. Nothing invented.
- **Order:** hero (offline) → proof line → the outage story (dark) →
  WhatsApp agent → delivery → DIAN → kitchen and back office → how it
  starts (3 steps) → questions → form.
- **Form placement:** one form, at `#demo` at the end. Every primary
  action is an anchor to it. On phones a bar fixed to the bottom carries
  the action and hides while the form is on screen.

**D4. One configuration module.**
- `src/config/site.ts` exports the product name (`Kuenta`), the site URL,
  and values read from `PUBLIC_GA_MEASUREMENT_ID`, `PUBLIC_META_PIXEL_ID`,
  `PUBLIC_WHATSAPP_NUMBER` and `PUBLIC_API_BASE_URL`.
- `.env.example` lists them. A missing id turns that tool off; a missing
  `PUBLIC_API_BASE_URL` fails the build.
- Renaming the product is an edit to this file plus the copy.

**D5. Tracking: two tags loaded directly, one wrapper.**
- `src/components/Analytics.astro` injects each tag only when its id is
  set, with `async`, after the content.
- `src/lib/track.ts` is the only caller of `gtag` and `fbq`:

  | Call | Google Analytics | Meta |
  |---|---|---|
  | page load | `page_view` (automatic) | `PageView` |
  | `trackLead(eventId)` | `generate_lead` | `Lead`, `eventID` |
  | `trackWhatsApp()` | `contact_whatsapp` | `Contact` |

- Every call is guarded: if the global is missing (blocked or not
  configured) it does nothing and never throws.
- `event_id` is `crypto.randomUUID()`, made at submit, sent to the backend
  and to Meta. Storing it is what lets a later Conversions API call
  deduplicate against the browser event.
- No parameter ever carries form values.
- Alternative: Google Tag Manager. Rejected for two tags: a third script,
  and events that live in a web console, not in review.

**D6. Attribution: first touch in `sessionStorage`.**
- `src/lib/attribution.ts`: `capture()` runs on every page and writes
  only if nothing is stored; `read()` returns the stored value, or what
  the current URL gives when storage throws.
- Pure parsing is separate from storage, so it is unit tested with
  `node --test`.
- Click ids are stored here, unlike `ad-attribution`, which refuses
  `ctwa_clid`. The difference is deliberate: this is a prospect who gave
  consent on the form, and the click id is what server-side conversion
  reporting needs.

**D7. The form.**
- `src/components/LeadForm.astro` plus `src/lib/lead-form.ts`.
- Posts JSON to `${PUBLIC_API_BASE_URL}/api/public/leads`.
- The honeypot is a `website` input, off screen, `tabindex="-1"`,
  `autocomplete="off"`, hidden from assistive technology.
- `400` with a `field` maps to that input; `429` and everything else show
  the general error with the WhatsApp action.
- `trackLead` is called only after a `201`.

### Backend

**D8. A `lead` slice in the cloud binary, as platform data.**
- `leads` table: no `organization_id`, no row-level security, like
  `organizations`. Columns per the spec, plus the alert columns of D10.
  Index on `created_at`, and on `(phone, created_at)` for the duplicate
  check.
- The repository takes the plain `*gorm.DB`, as `OrganizationRepository`
  does.
- The aggregate `lead` owns validation: required fields fail, attribution
  fields drop (returned as a list so the service logs them).
- Phone: `shared.NewPhone(raw, "CO")`.
- Routes, cloud only:
  - `POST /api/public/leads`: no auth, behind the limiter of D9.
  - `/api/platform/leads` (list, get, patch, delete): behind
    `AdminAPIKeyMiddleware`.
- The edge binary registers none of them, so they answer `404` there.

**D9. Abuse protection in four layers, cheapest first.**
1. **Body limit:** `http.MaxBytesReader` at 16 KB.
2. **Per address:** a small in-memory fixed-window limiter middleware,
   5 per hour, keyed by `c.ClientIP()`, with a sweep of expired keys. It
   is per process, which is acceptable: more instances only loosen it.
3. **Honeypot and duplicate phone:** in the service; both answer `201`.
4. **Global cap:** a count of leads stored in the last hour, 300, checked
   in the service. It holds across instances and does not depend on the
   address.
- No captcha: it costs conversions, and these four cover a new, unknown
  form. Revisit if spam gets through.
- **Trusted proxies:** a new optional cloud setting `TRUSTED_PROXIES`
  (comma-separated CIDRs) is passed to `router.SetTrustedProxies`. Unset
  keeps today's behaviour. Production must set it to the load balancer's
  range, or the per-address limit can be dodged with a forged header.
- Alternative: `golang.org/x/time/rate` per key. Rejected: a token bucket
  per address needs the same map and sweep, for a limit this coarse.

**D10. Alerts: a column-backed outbox and one platform job.**
- Columns on `leads`: `alert_status` (`pending`, `sent`, `failed`,
  `skipped`), `alert_attempts`, `alert_next_attempt_at`.
- A lead is created `pending` when a destination is configured,
  `skipped` otherwise, so configuring later sends nothing old.
- `LeadAlertService.DeliverDue` claims due `pending` rows with
  `FOR UPDATE SKIP LOCKED`, sends, and marks `sent`, or retries with
  backoff from 15 seconds doubling to 30 minutes; after 24 hours it marks
  `failed` and logs an error.
- It runs every 15 seconds from a new `LeadAlertScheduler`, which is not
  per organization.
- Sending reuses `ports.StaffNotificationSender` with the Discord client:
  the service builds a `dto.StaffMessage` and a `dto.ChannelSecret` from
  the configured URL.
- Config: `LEAD_ALERT_DISCORD_WEBHOOK_URL` in
  `internal/platform/config/leads.go`, validated at startup with
  `notification_channel.ValidateDiscordWebhookURL`.
- Alternative: publish a `lead.created` domain event. Rejected: the bus
  relays inside one organization's session, and making it handle
  organization-less events is a larger change than this one.
- Alternative: send in the request. Rejected: a slow Discord would slow
  the form, and a failed send would be lost.

**D11. Docs and repo rules.**
- `backend/docs/api/leads.md` for both route groups.
- `landing/AGENTS.md` (commands, the config module, "all events go
  through `track.ts`", "`DESIGN.md` wins over the skill") and a
  `landing/CLAUDE.md` holding `@AGENTS.md`.
- Root `AGENTS.md`: fill the `landing/` row, and say it is not part of
  the `v*` release.
- `openspec/config.yaml` is not touched.

**D12. CI.** `.github/workflows/landing-ci.yml`, copied from
`frontend-ci.yml` and filtered to `landing/**`: install, `astro check`,
test, build. Build runs with a placeholder `PUBLIC_API_BASE_URL` and no
tracking ids.

## Risks / Trade-offs

- **[No cookie banner]** → Tags load for every visitor. Accepted for a
  Colombian audience; the policy page discloses both tools. Running ads
  to the EU would need consent mode first.
- **[Browser-only conversion events are undercounted]** → Ad blockers and
  iOS drop some. `event_id` is stored now so the Conversions API can be
  added without a data change.
- **[Forged `X-Forwarded-For` dodges the per-address limit]** →
  `TRUSTED_PROXIES` in production; the global cap holds regardless.
- **[The global cap blocks real leads during an attack]** → A `429` shows
  the WhatsApp action, so a real prospect still has a way in.
- **[Alert destination misconfigured]** → Leads are still stored and
  listable; failures are logged at error level.
- **[Screenshots leak real guests]** → Captured only from a database of
  demo data; a spec scenario requires the review.
- **[A third-party skill file steers the agent]** → It is one Markdown
  file, read in full before commit, pinned, and overridden by
  `DESIGN.md`.
- **[The name]** → "Kuenta" sounds like Q·enta. One config value and the
  copy change if the trademark search goes badly.

## Migration Plan

1. Backend: one cloud migration, `create_leads`, at the next free number,
   with a down file. It adds a table and touches no other.
2. Deploy the backend. Set `LEAD_ALERT_DISCORD_WEBHOOK_URL` and
   `TRUSTED_PROXIES`. The public route is live but nothing links to it.
3. Create the GA4 property and the Meta Pixel; point `kuenta.com` at the
   host; set the four `PUBLIC_*` values there.
4. Publish the site. Submit one test lead and check: the row, the alert,
   `generate_lead` in GA4's realtime view, and `Lead` in Meta's Events
   Manager.
5. Rollback: unpublish the site. The backend route can stay; migrating
   down drops only `leads`.

## Open Questions

- Whether the page names Laguna Escondida as the restaurant that runs on
  it. The copy works either way ("un restaurante real").
- Whether prices appear on the page. The first version shows none.
- Which WhatsApp number the button uses.
- The result of the SIC trademark search for the name.

## Implementation Map

Backend (paths under `backend/`):

| New | Copy from |
|---|---|
| `internal/domain/dto/lead.go` | `internal/domain/dto/organization.go` |
| `internal/domain/aggregate/lead/` (+ test) | `internal/domain/aggregate/reservation/` (field validation), `internal/domain/aggregate/shared/phone.go` |
| `internal/domain/ports/leads.go` | `internal/domain/ports/staff_notifications.go` |
| `internal/domain/service/lead_service.go` (+ test) | `internal/domain/service/organization_service.go` (+ test) |
| `internal/domain/service/lead_alert_service.go` (+ test) | `internal/domain/service/staff_notification_delivery_service.go` (+ test) |
| `internal/platform/postgres/migrations/cloud/0000NN_create_leads.{up,down}.sql` | `000082_create_notification_channels.up.sql` (shape only; no `organization_id`, no policy) |
| `internal/platform/postgres/repository/lead_repository.go` (+ integration test) | `organization_repository.go`, `organization_repository_integration_test.go` |
| `internal/platform/handler/lead_handler.go` (+ test) | `organization_handler.go` (+ test) |
| `internal/platform/handler/rate_limit_middleware.go` (+ test) | `internal/platform/handler/middleware.go` |
| `internal/platform/config/leads.go` (+ test) | `internal/platform/config/staff_notifications.go` (+ test) |
| `internal/platform/cron/lead_alert_scheduler.go` | `internal/platform/cron/event_relay_scheduler.go` |
| Wiring | `internal/bootstrap/cloud.go` (~185, ~225–243), `routes_cloud.go` (~37), `core.go` (~209, trusted proxies), `testdata/routes_cloud.golden` |
| Docs | `docs/api/organizations.md` → `docs/api/leads.md` |

Landing (paths under `landing/`, all new):

| File | Holds |
|---|---|
| `astro.config.mjs`, `package.json`, `tsconfig.json`, `.env.example` | D1, D4 |
| `.claude/skills/landing-page-design/SKILL.md`, `NOTICE`, `DESIGN.md` | D2 |
| `AGENTS.md`, `CLAUDE.md` | D11 |
| `src/config/site.ts` | D4 |
| `src/styles/global.css` | D2 tokens |
| `src/layouts/Base.astro` | head, metadata, footer |
| `src/components/Analytics.astro`, `src/lib/track.ts` | D5 |
| `src/lib/attribution.ts` (+ test) | D6 |
| `src/components/LeadForm.astro`, `src/lib/lead-form.ts` (+ test) | D7 |
| `src/components/sections/*.astro` | D3, one per section |
| `src/pages/index.astro`, `src/pages/privacidad.astro` | pages |
| `public/` | screenshots, social image, `robots.txt` |

Repo: `.github/workflows/landing-ci.yml` (from `frontend-ci.yml`), root
`AGENTS.md`.
