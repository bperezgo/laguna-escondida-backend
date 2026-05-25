# Plan 2: Desktop App for Windows

## Context

Printing an invoice today is a Ctrl+P + browser print dialog + paper-size hack. This
is slowing adoption inside the restaurant. We also want to integrate the credit-card
payment terminal soon (Datafono). Browsers cannot reach those drivers — payment
terminals expose USB/Serial/COM or a vendor SDK, and thermal printers want ESC/POS
over USB or raw TCP.

The right answer is a **Windows desktop app** that bundles:

- The local Go backend from plan 1 (sqlite, sync worker)
- The existing Next.js frontend, served from inside the binary
- Native bridges for the printer and the payment terminal

The same app on a register PC becomes the local server for the KDS, the kitchen
tablets, and any phone or laptop on the staff VLAN.

## Goals

1. One install on the register PC: backend + frontend + drivers.
2. **One-click invoice print** to the connected thermal printer. No paper-size dance.
3. Payment-terminal integration (Datafono) — read amount, charge, confirm — from the
   same UI flow.
4. The same UI works inside the desktop app **and** in a regular browser on any LAN
   device (KDS tablet, manager's phone), because the backend runs on the host's LAN
   interface, not just loopback.
5. Updates are pushed (auto-update channel) so the restaurant is not pinned to a
   stale build.

## Framework choice

Four candidates, ranked for this stack:

| Framework | Language | Pros | Cons |
|-----------|----------|------|------|
| **Wails v2** | Go + WebView2 | Reuses Go backend skill set, single binary, small (~10 MB) | Smaller ecosystem; Windows-printer integration via Go ESC/POS libs or `syscall` to winspool |
| **Electron** | Node + Chromium | Huge driver ecosystem (`node-thermal-printer`, escpos libs, easy N-API for vendor DLLs) | Big binary (~120 MB), separate Node process to manage alongside Go backend |
| **Tauri 2** | Rust + WebView2 | Tiniest binary, modern | Rust learning curve; you'd ship Go backend as a sidecar process anyway |
| **.NET (WPF/MAUI)** | C# | Best Windows driver support, COM/DLL native | Throws away the Next.js frontend; biggest rewrite |

**Recommendation: Wails v2.** Reasons:

- The local backend is already Go (plan 1). Wails lets you embed it directly with
  zero IPC overhead.
- WebView2 (the Edge engine) is preinstalled on Windows 11 and trivially installed on
  10. The current Next.js UI runs unchanged inside it.
- Single binary, single auto-updater (e.g., `go-update`), single process tree to
  supervise.
- For vendor DLLs, Go can call them via `syscall.NewLazyDLL`. For pure ESC/POS over
  USB/Serial, several Go libraries already exist (`mugli/escpos`, `hennedo/escpos`).

If a key payment-terminal vendor ships **only** a .NET SDK, fall back to Electron or
add a tiny .NET sidecar exposing HTTP on localhost. Don't redo the whole stack for
one driver.

## Process model

```
┌─────────────────────────────  Wails app  ─────────────────────────────┐
│                                                                       │
│  Main Go process                                                      │
│  ┌──────────────────────────────────────────────────────────────────┐ │
│  │  Local backend (HTTP listener on 0.0.0.0:8080)                  │ │
│  │  - All current REST endpoints                                   │ │
│  │  - Sqlite (WAL)                                                 │ │
│  │  - Outbox + sync worker (see plan 1)                            │ │
│  └──────────────────────────────────────────────────────────────────┘ │
│  ┌──────────────────────────────────────────────────────────────────┐ │
│  │  Driver bridge (Wails JS bindings — only from the desktop UI)   │ │
│  │  - PrinterService.PrintInvoice(payload)                         │ │
│  │  - PaymentTerminalService.Charge(amount, method)                │ │
│  │  - DeviceService.ListDevices(), .GetStatus()                    │ │
│  └──────────────────────────────────────────────────────────────────┘ │
│  ┌──────────────────────────────────────────────────────────────────┐ │
│  │  WebView2 hosting the Next.js build                             │ │
│  │  - Built with `next export` or wrapped via `next start`         │ │
│  │  - Fetches API at http://localhost:8080                         │ │
│  └──────────────────────────────────────────────────────────────────┘ │
└───────────────────────────────────────────────────────────────────────┘
```

Other LAN devices (KDS tablet, manager phone) talk to `http://<register-ip>:8080`
directly over the staff VLAN. They get the SSE stream and the same REST API; they
just don't have the native driver bridge (they don't need one — only the register
prints and charges).

## Frontend integration

The current frontend is Next.js 16 (App Router) with `proxy.ts`. Two embedding
strategies:

### Strategy A — static export, embedded

- Add `output: 'export'` to `next.config.js`.
- `next build` produces a static `out/` folder.
- Wails embeds `out/` via `//go:embed`.
- All current API routes under `app/api/...` (auth proxies, SSE pipe) must be moved
  to the Go backend, because static export drops API routes. This is a non-trivial
  change but a cleaner architecture (no Next.js server-side fetch layer).

### Strategy B — run `next start` as a sidecar

- Bundle Node + the built Next.js app inside the Wails resources.
- Wails spawns `node server.js` on a random port at startup.
- WebView2 navigates to `http://localhost:<port>`.
- Heavier (Node ships with the app), but keeps `app/api/...` working unchanged.

**Recommend Strategy A.** The current `app/api/...` routes are thin pass-throughs to
the Go backend (e.g., `app/api/sse/commands/[area]/route.ts` pipes a stream — see
that file for the pattern). Folding them into the Go backend removes the Next.js
server tier entirely, which simplifies the desktop bundle and deployment.

Detection of "running inside desktop app" — for showing the print button vs. hiding
it on a manager's phone:

- Wails exposes `window.go` and `window.runtime` globals. Feature-detect at runtime.
- Wrap in a `useDesktopBridge()` React hook that returns `{ available: boolean, print: (payload) => Promise<void>, charge: ... }`.

## Printer integration

### Hardware assumption

Thermal receipt printer, ESC/POS protocol, USB or LAN. Common models: Epson TM-T20,
Bematech MP-4200, Xprinter XP-58. All speak ESC/POS.

### Implementation

1. New `internal/platform/printer/` package in the Go backend.
2. Detection: enumerate USB printers via `syscall` (Windows `winspool`) or scan a
   configured static IP/port (LAN printers).
3. Driver: render the invoice to ESC/POS bytes — header logo (raster bitmap),
   left/right-aligned line items, totals, footer text, paper cut.
4. Expose `POST /api/invoices/:id/print` on the local backend. The desktop UI calls
   this. Anyone on the LAN can also call it — but the desktop is where the printer
   is physically attached.
5. Add a printer-status endpoint (`GET /api/devices/printer/status`) for the UI to
   show online/out-of-paper.

Drop the Ctrl+P workflow from the invoice page. Replace it with a single "Imprimir"
button that calls the new endpoint and shows success/failure inline.

## Payment-terminal integration

### Hardware assumption (Colombia)

Common options:

- **Bold** — provides a local REST endpoint on the device or paired phone (port
  8181). Easy: just HTTP from Go.
- **Wompi (Mini POS)** — REST API or QR.
- **Redeban / Credibanco / ACH** integrated terminals — usually a Windows DLL or
  serial COM interface.
- **Verifone / Ingenico / PAX** — C SDK or .NET SDK on Windows.

**Pick the provider first**, then choose integration path. Do not design the
integration in the abstract.

### Implementation (sketch)

1. Define a port: `internal/domain/ports/payment_terminal.go` with `Charge`, `Cancel`,
   `Refund`, `GetReceipt`.
2. Add adapters per provider under `internal/platform/payment/<provider>/`.
3. Each adapter encapsulates SDK quirks. For DLL-only SDKs, wrap with
   `syscall.NewLazyDLL` + typed wrappers.
4. Expose `POST /api/payments/charge` that takes amount, currency, method, returns a
   transaction id + receipt payload.
5. UI: after the "cobrar" action, present a card-or-cash modal; if card, call the
   endpoint and wait (60–120 s timeout) for terminal confirmation; on success,
   immediately trigger the invoice print.

This is a sequential flow. Lock the UI during the terminal interaction; show
"Esperando confirmación del datáfono..." with a cancel button.

## Auto-update

- Use `equinox` or a homemade flow with `go-update`.
- Updates hosted on a small S3 / R2 bucket as `latest.json` + signed binary.
- App checks at startup + every 6 h. Downloads in the background. Restarts after a
  user-confirmed prompt during a quiet hour.
- Critical: sign the binary (Authenticode) once you have a code-signing cert.
  Unsigned binaries trigger SmartScreen warnings and look unprofessional.

## SSE — answering the user's question

> If we move to a desktop app, will we have access to SSE events easily?

Yes. SSE is just HTTP with `Content-Type: text/event-stream`. Inside the desktop
app's WebView2, the standard browser `EventSource` API works exactly the same. The
existing `app/api/sse/commands/[area]/route.ts` and `app/api/sse/open-bill-products/[area]/route.ts`
patterns keep working — but with the API folded into the Go backend, the SSE endpoint
is served directly by Go on `http://localhost:8080/sse/...` with no Next.js pipe in
between.

For the KDS tablet on the LAN (not running the desktop app), SSE goes to
`http://<register-ip>:8080/sse/commands/...` over plain HTTP on the staff VLAN. No
intermediary.

## Migration sequence

1. **Land plan 1.** No point in desktop-app-ifying a cloud-dependent web app.
2. **Fold `app/api/...` routes into the Go backend.** Audit each route; most are
   straight proxies. Move auth, sse, and any pass-throughs. The frontend becomes
   pure UI.
3. **Add `output: 'export'`** to Next config; verify the SPA still works as a static
   site against the Go backend.
4. **Wails skeleton.** Embed `out/`. Hook WebView2 to load the static index.
5. **Printer driver, end-to-end.** First the success path with one printer model.
   Replace the Ctrl+P flow in the invoice UI.
6. **Pilot the desktop app on the register** for a week alongside the existing web
   workflow. Roll back is just opening the browser if anything blocks.
7. **Payment-terminal pilot** once the provider is selected and the SDK arrives.
8. **Auto-update + code signing** before declaring the project "done."

## Risks and mitigations

| Risk | Mitigation |
|------|------------|
| Vendor SDK is .NET-only | Tiny .NET sidecar exposing localhost HTTP; Go calls it |
| SmartScreen flags unsigned binary | Buy an OV or EV code-signing certificate ($80–300/yr) |
| WebView2 missing on a target machine | Wails installer can bundle the bootstrapper |
| Printer model variations | Abstract behind a `PrinterDriver` interface; one impl per model class |
| User opens the same UI in browser and desktop simultaneously, gets different feature sets | The `useDesktopBridge()` hook makes the difference explicit; hide print/charge actions in browser context |

## Open questions for the next session

- Confirm the printer make/model already in use at the restaurant.
- Decide the payment-terminal provider before designing the adapter.
- Code-signing cert: who buys it, in whose name?
- Is there appetite for macOS/Linux desktop builds (KDS on a Linux tablet)? Wails
  supports both; minor extra CI.
