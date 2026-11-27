## Why

Once the edge release carries an installer, a customer still has no place to get it: the file sits on a GitHub release page under a versioned name, and nothing tells them which version works with the live cloud. The landing site needs a page with one button that always gives the right installer.

## What Changes

- New public cloud endpoint `GET /api/public/edge/installer`. It answers with a redirect to the installer of the newest edge release that the running cloud can sync with: not a draft or prerelease, carrying an installer in its manifest, and with `min_cloud` at or below the cloud's own version. This is the rule the boxes' updater already applies.
- The cloud reads the release list from GitHub and keeps it for ten minutes; when GitHub cannot be reached it keeps serving the last answer. With nothing eligible it answers `503`.
- The endpoint is limited per address, redirects only to the project's own release downloads, and logs the version it served.
- New landing page `/descargar`: what the PC needs, three steps (download, run, sign in), the download button, what to do at the Windows warning screen, and a way to a demo for visitors without an account.
- The home page keeps "Pide una demo" as its only primary action. It gains a footer link to the page, and the "Así empieza" step that says the operator installs the equipment is reworded.
- A download click is tracked through `track.ts`, with no personal data.
- `landing/AGENTS.md` names the download endpoint as the second backend address the site uses.

Not in this change: hosting the landing (the owner's Cloudflare Pages work), a download button inside the cloud UI, showing the version or checksum on the page, and serving the file from the project's own domain.

## Capabilities

### New Capabilities

- `installer-download`: the public endpoint that picks and redirects to the current installer, and the landing page that offers it.

### Modified Capabilities

None.

## Impact

- **Backend (cloud)**: a release-catalog port with a GitHub adapter and cache, a small selection service, a handler and a route next to `POST /api/public/leads`; one optional setting for the release repository. No database change. The edge binary is untouched.
- **API docs**: new `backend/docs/api/edge-downloads.md`.
- **Landing**: `src/pages/descargar.astro`, a helper in `src/config/`, an event in `src/lib/track.ts`, the footer in `layouts/Base.astro`, one step in `sections/HowItStarts.astro`, `AGENTS.md`.
- **Depends on**: `edge-installer`. Until a release carries an installer the endpoint answers `503`.
- **Operations**: the landing's build must set `PUBLIC_API_BASE_URL` to the real cloud address, or the button points at the build placeholder. The cloud gains an outbound call to GitHub.
