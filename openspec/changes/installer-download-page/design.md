## Context

See proposal.md for motivation. What exists:

- The cloud's only public route is `POST /api/public/leads`, behind `RateLimitMiddleware` (per address), registered in `registerCloudRoutes`.
- The cloud image is stamped with its release version (`buildinfo.Version`; `dev` when unstamped).
- Boxes pick their release in `edge/scripts/releases.ps1`: list the repository's GitHub releases, keep `edge/vX.Y.Z` that are neither draft nor prerelease, newest first, and take the first whose `manifest.json` has `min_cloud` at or below the cloud's version.
- `manifest.json` is an asset of each edge release. After `edge-installer` it has `components.installer` with `version`, `url` and `sha256`.
- The landing is static Astro. Its backend address comes from `PUBLIC_API_BASE_URL` through `src/config/`; `src/lib/track.ts` is the only caller of the analytics tools; the footer lives in `layouts/Base.astro`, whose header action takes a `ctaHref` prop.

## Goals / Non-Goals

**Goals:**

- The download and the boxes' updater agree on which release is current, by applying the same rule.
- The landing page stays static: a link, no fetch.
- A GitHub outage does not break downloads that worked a minute ago.

**Non-Goals:**

- No proxying of the file: the cloud redirects, GitHub serves the bytes.
- No persistence. The cloud keeps what it learned in memory, per instance.
- No GitHub token. The repository is public and the cache keeps the cloud far below the unauthenticated limit.

## Decisions

### 1. Port, adapter, service

- `ports.EdgeReleaseCatalog` with two methods: `ListEdgeReleases` (version, draft and prerelease flags of each `edge/vX.Y.Z`) and `GetEdgeManifest(version)` (`min_cloud`, installer address). Two methods because eligibility depends on the cloud version, which only the service holds: the service walks the list newest first and asks for manifests one at a time, so it can stop at the first eligible release.
- `platform/httpclient` adapter over GitHub: `GET /repos/<owner>/<name>/releases?per_page=50`, then each candidate's `manifest.json` from its release download address. Manifests are fetched lazily, newest first, and stop at the first eligible release, so the usual refresh is two requests. A manifest that fails to download or parse makes that release ineligible.
- `service.InstallerDownloadService` takes the catalog and the cloud's version as a constructor argument (the domain cannot import `buildinfo`) and returns the current installer or `ErrInstallerUnavailable`. Version comparison is a small pure function in the service, with table tests mirroring `edge/scripts/test-releases.ps1`.
- `handler.InstallerDownloadHandler` maps success to `302` + `Cache-Control: no-store`, the error to `503` + `Retry-After: 300`, and logs the version served.

Alternative: put the selection in the adapter. Rejected: the rule is the part worth unit-testing, and it must not depend on HTTP.

### 2. The cache wraps the catalog, in the platform layer

A caching decorator implements the same port. It caches the release list, and each manifest separately by version, for ten minutes. An expired entry refreshes under a mutex, so concurrent requests cause one upstream read. A failed refresh returns the stale value and logs a warning. Before the first success there is nothing to return, and the service reports unavailable. A failed read is retried after one minute, so an outage does not make every visitor wait on the timeout, unless the failure came from the visitor giving up. Each upstream call has a ten-second timeout, and the handler bounds the whole lookup to ten seconds.

Because the decorator caches the list (with the manifests it resolved), the selection runs on every request but costs nothing.

Alternative: a scheduled job that refreshes in the background. Rejected: more moving parts for a route that will see little traffic; lazy refresh gives the same freshness.

### 3. Only project release addresses are redirect targets

The adapter accepts an installer address only when it starts with `https://github.com/<owner>/<name>/releases/download/`. The manifest is built by the project's own workflow, so this is defence in depth: it keeps the endpoint from being an open redirect if a release asset were ever replaced.

### 4. Configuration

One optional cloud setting, `EDGE_RELEASES_REPO` (`owner/name`), defaulting to the repository the appliance scripts already name in `releases.ps1`. Validated in `LoadCloud` as two path-safe segments. The cache lifetime and the rate limit are constants.

Rate limit: `RateLimitMiddleware` with 20 requests per address per ten minutes, twice the floor the spec sets.

### 5. The landing page is a link plus one click listener

- `installerDownloadUrl(config)` in `src/config/build-site-config.ts` returns `${apiBaseUrl}/api/public/edge/installer`; exported through `site.ts`. Covered in `build-site-config.test.ts`.
- `src/pages/descargar.astro` uses `Base` with `path="/descargar"` and `ctaHref="/#demo"`, so the header action leads back to the home page's form. The button is an `<a href>` with `data-installer-download`.
- `trackInstallerDownload()` in `track.ts`: `gtag('event', 'installer_download')` and `fbq('trackCustom', 'InstallerDownload')`. The existing delegated click listener in `Base.astro` gains the second selector. The link is never intercepted, so the download does not wait on analytics.
- Footer: a "Descargar Kuenta" link in `Base.astro`'s footer navigation, whose `aria-label` changes from "Legal" to "Pie de página".
- Copy and layout follow `landing/DESIGN.md` and its named text styles; the page uses the same section and card patterns as "Así empieza".

Alternative: fetch the version from the backend and show it. Rejected for now: it adds a script and a second endpoint for a detail the file name already carries.

### 6. Copy that changes on the home page

"Así empieza", step 2, currently reads "Instalamos el equipo en el local y dejamos listos tu menú, tus prefijos de facturación y tu canal de WhatsApp." Working draft: "Dejamos listos tu menú, tus prefijos de facturación y tu canal de WhatsApp. Tú instalas Kuenta en el computador del local con nuestro instalador." The title "Lo configuramos contigo" stays.

## Risks / Trade-offs

- [The `503` body is raw JSON in the visitor's browser] → Rare (only before the first installer exists or on a cold start during a GitHub outage). The page tells visitors to write on WhatsApp if the download does not start.
- [A cloud instance that just started has an empty cache while GitHub is down] → It answers `503` until the first successful read. Accepted; persisting the list would add storage for a rare case.
- [The download address shows `github.com`] → Accepted for now. Because the page links to the cloud, moving the files to another host later changes only the adapter.
- [GitHub's unauthenticated limit is per cloud address] → About six list reads an hour per instance against a limit of sixty.
- [The page goes live while the installer is unsigned] → The page explains the Windows warning; the rollout keeps the link away from customers until the owner decides.
- [The landing is built with the placeholder backend address] → The page would link to `127.0.0.1`. The build setting is part of the owner's hosting work; a rollout task checks the deployed link.

## Migration Plan

1. Backend ships in a cloud release. With no installer published yet it answers `503`; nothing else changes.
2. Landing ships whenever the site is hosted; the page is reachable only by its address and the footer link.
3. After the first `edge/v*` carrying an installer, check the redirect, then the button on the hosted page.

Rollback: remove the footer link and the page (landing), or leave the endpoint in place: it has no side effects.

## Implementation map

- `backend/internal/domain/dto/`, `domain/ports/` (+ `make generate-mocks`), `domain/error/`, `domain/service/installer_download_service.go` (+ test)
- `backend/internal/platform/httpclient/` : GitHub release catalog and its caching decorator (+ tests against `httptest`)
- `backend/internal/platform/handler/` : installer download handler (+ test); `rate_limit_middleware.go` for the existing middleware
- `backend/internal/platform/config/cloud.go` (+ test), `backend/internal/bootstrap/cloud.go`, `routes_cloud.go`, `testdata/routes_cloud.golden`
- `backend/docs/api/edge-downloads.md` (new), `backend/docs/examples/curl-examples.md`, `backend/.env.example`
- `edge/scripts/releases.ps1`, `edge/scripts/test-releases.ps1` : read-only, the rule and its cases
- `landing/src/pages/descargar.astro` (new), `landing/src/config/build-site-config.ts`, `site.ts`, `build-site-config.test.ts`
- `landing/src/lib/track.ts`, `track.test.ts`, `landing/src/layouts/Base.astro`, `landing/src/components/sections/HowItStarts.astro`
- `landing/AGENTS.md`, `landing/DESIGN.md` (read-only)

## Open Questions

- Final Spanish copy of the page and of the reworded step; the drafts here and in tasks.md are working text.
