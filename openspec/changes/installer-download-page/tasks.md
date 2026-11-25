## 1. Choosing the installer (backend, domain)

- [x] 1.1 Add the edge release DTO, the `EdgeReleaseCatalog` port, `ErrInstallerUnavailable` and `InstallerDownloadService` with the selection rule of the spec (cloud version passed at construction); run `make generate-mocks`; verify with table tests covering every scenario of "The current installer is the newest one the cloud can sync with", a `dev` cloud, and an empty catalog

## 2. Reading releases (backend, platform)

- [x] 2.1 Add the GitHub catalog adapter: list releases, keep `edge/vX.Y.Z`, read each candidate's `manifest.json` lazily, accept an installer address only under the configured repository's release download path, ten-second timeout; verify against an `httptest` server for: a normal list, a manifest that fails to load (release skipped), a manifest without an installer, and a foreign installer address (release skipped)
- [x] 2.2 Add the caching decorator of design decision 2 with an injectable clock; verify with tests that fifty calls inside ten minutes read upstream once, concurrent calls on an expired cache read upstream once, a failed refresh returns the previous list, and a failure before any success returns the error
- [ ] 2.3 Add the optional `EDGE_RELEASES_REPO` setting to `LoadCloud` with its default and validation, and to `backend/.env.example`; verify config tests for unset (default), a valid value, and a malformed one

## 3. The endpoint (backend)

- [x] 3.1 Add the handler (`302` with `Location` and `Cache-Control: no-store`; `503` with `error: installer_unavailable` and `Retry-After`; one log line naming the version served) and register `GET /api/public/edge/installer` in the cloud routes behind `RateLimitMiddleware`; update `testdata/routes_cloud.golden`; verify handler tests for both answers, a route test for `429` above the limit and five quick requests all redirected, and that the edge router answers `404`
- [x] 3.2 Write `backend/docs/api/edge-downloads.md` in the format of `products.md` and add the `curl` example to `backend/docs/examples/curl-examples.md`; verify the documented status codes and headers match the handler tests
- [x] 3.3 Run `make lint` and `make test` from `backend/`; verify both pass

## 4. The page (landing)

- [x] 4.1 Add `installerDownloadUrl` to the configuration module and export it through `site.ts`; verify `build-site-config.test.ts` covers the address for a backend with and without a trailing slash
- [x] 4.2 Add `trackInstallerDownload` to `src/lib/track.ts` and the `data-installer-download` selector to the click listener in `Base.astro`; verify `track.test.ts` covers both tools called once, a missing tool, and a throwing tool
- [ ] 4.3 Build `src/pages/descargar.astro` following `DESIGN.md`: requirements, the three steps, the download button, the Windows warning note ("Más información" then "Ejecutar de todas formas"), the account notice with a link to `/#demo`, and a WhatsApp line for a download that does not start; header action pointing at `/#demo`; verify `pnpm build` emits the page, the built HTML's button `href` is the configured backend's endpoint, the page contains no version, size, duration, price or testimonial, and the button works with JavaScript disabled in a browser
- [x] 4.4 Add the "Descargar Kuenta" footer link in `Base.astro` and reword step 2 of `HowItStarts.astro` (draft in design decision 6); verify the home and privacy pages show the footer link, the header still has one action, and no page says the operator installs the equipment
- [x] 4.5 Update `landing/AGENTS.md`: the site uses two backend addresses, the leads endpoint and the installer download link, with `backend/docs/api/edge-downloads.md` as the second contract; verify the rule text names both
- [x] 4.6 Run `pnpm check`, `pnpm test` and `pnpm build` from `landing/`; verify all pass

## 5. Checks

- [x] 5.1 Run `openspec validate installer-download-page --strict`; verify it reports the change as valid

## 6. Rollout (owner, after merge)

- [ ] 6.1 Release the cloud; verify `GET /api/public/edge/installer` answers `503` before an installer exists, or `302` to the newest compatible installer once one does
- [ ] 6.2 In the Cloudflare Pages build, set `PUBLIC_API_BASE_URL` to the real cloud address; verify on the hosted site that the download button's link starts with that address, not `127.0.0.1`
- [ ] 6.3 After the first `edge/v*` with an installer, press the button on the hosted page from a Windows PC; verify the file downloaded is that release's installer and its sha256 matches the release's
- [ ] 6.4 Decide when to give the link to customers (before or after the installer is signed) and review the final Spanish copy; verify the Windows warning note is removed or kept to match
