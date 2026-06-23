# Backend Plan — POS Ticket Printing (edge node → local thermal printer)

> Status: planning. Companion doc: `EDGE_OFFLINE_SYNC_PLAN.md` (§7 Hardware adapters).
> Scope: add the ability to print the bill/receipt (the "cuenta" / POS ticket) from the
> **Go backend** to a thermal printer physically attached to the restaurant's Windows box.
> Today this is done in the browser with `window.print()`; we move it to the edge node so it
> works offline and drives real hardware.

## 1. Problem & decisions taken

The frontend already renders the invoice as HTML and prints it with the browser dialog
(`PaymentModal.handlePrint()` → `generateInvoicePrintHTML()` → `window.print()`). That path
depends on the operator picking the right printer/paper every time, can't drive the device
reliably, and is the browser's problem, not ours. The edge node (a Windows box on the
restaurant LAN, `APP_MODE=edge`) is the right place to own printing: it has the authoritative
bill data and a stable connection to the hardware.

**Decisions already made (do not re-litigate):**

| # | Decision |
|---|----------|
| 1 | **Backend renders the ticket from a bill ID.** Frontend sends `{ open_bill_id }`; the edge node loads the authoritative bill and generates the ticket. The client never sends layout or computed totals — a ticket is a (near-)fiscal document and the backend is the source of truth. |
| 2 | **Text-mode ESC/POS first** (structured render), not HTML raster. The current ticket is plain text + dashed rules; a small pure-Go renderer covers it and is fully testable on macOS. HTML-raster (for a logo/rich layout) is a later, optional renderer behind the same port. |
| 3 | **Printer is locally attached to the Windows box**, not a network printer. Production transport = **Windows print spooler, RAW datatype**. Network/TCP is dev/emulator only. |
| 4 | **Wired in edge mode only** and **must work fully offline** — printing never depends on the cloud or the electronic-invoice provider. |
| 5 | **Hexagonal split:** the domain produces a structured `dto.Ticket`; ESC/POS encoding *and* transport are an **adapter** concern (`internal/platform/device/`). Domain never sees a byte. |
| 6 | **Standalone, re-printable endpoint** (`POST /api/device/print`), decoupled from pay-order, so a ticket can be reprinted on demand. |

**Foundations we already have:**

- Authoritative bill data in the domain: `internal/domain/dto/open_bill.go` (`OpenBill` with
  products, totals, `temporal_identifier`, `created_by`; `Bill` after payment with tax
  breakdown, CUFE, etc.).
- Edge mode + config plumbing on this branch (`internal/platform/config/config.go`,
  edge-only wiring in `cmd/main.go` ~lines 451–487).
- Gin router, `gocron`, Unit-of-Work, mockery + `make generate-mocks`, golden-test-friendly
  layout.
- Frontend print UI already exists and is marked "route through the edge node, confirm payload
  first" (`laguna-escondida-frontend/TODO.md` §2b; `components/orders/PaymentModal.tsx`
  ~line 91; `components/orders/EditOrderForm.tsx`).

## 2. The macOS-vs-Windows split (why most of this lands on the Mac now)

The feature has two layers; only the second is platform-specific.

```
  domain (pure Go)                 platform/device (adapter)
  ┌───────────────────┐            ┌───────────────────────────────────────────┐
  │ PrintService      │   Ticket   │ ReceiptPrinter                            │
  │  load bill        │──────────▶ │  1. render: Ticket → ESC/POS bytes (pure) │  ← golden-testable on Mac
  │  build dto.Ticket │            │  2. transport: bytes → device (Writer)    │  ← the only platform piece
  └───────────────────┘            └───────────────────────────────────────────┘
                                              │
                         ┌────────────────────┼─────────────────────┐
                         ▼                    ▼                      ▼
                   fileTransport         tcpTransport          windowsSpooler
                   (Mac dev: write       (emulator / net       (RAW to local
                    .escpos / preview)    printer on :9100)     printer queue) ← finish on Windows
```

- **Render (Ticket → ESC/POS bytes)** is pure Go: 100% built and unit/golden-tested on the Mac.
- **Transport** is just an `io.Writer`. Swap it by config. On the Mac you use `file` (and
  optionally `tcp` against an emulator); on the Windows box you use `windows`.
- Net effect: build the **entire vertical slice** (frontend button → handler → service →
  render → port → file output) on the Mac. The Windows machine only has to implement and
  validate one thin transport (~50–100 lines, `//go:build windows`).

## 3. Endpoint & contract

```
POST /api/device/print            (edge mode only; same auth as other order endpoints)
Request:  { "open_bill_id": "uuid", "copies": 1 }     // copies optional, default 1
Response: 200 { "printed": true }
          409 { "error": "printer_unavailable", "message": "..." }   // out of paper / offline / no device
          404 { "error": "bill_not_found" }
```

- `open_bill_id` is primary (matches what the frontend prints today — the "cuenta").
- Keep the body extensible: a later `"type": "cuenta" | "fiscal"` switch lets us add the
  post-payment fiscal ticket (with CUFE/QR) without changing the route.
- Errors are **actionable for the frontend**: a `printer_unavailable` lets the UI fall back to
  the browser print path and/or show a toast.

## 4. Code to add (by layer)

### domain/dto
- `internal/domain/dto/ticket.go` — `Ticket` (plain struct, no logic):
  - Header: business name, NIT/address (from config/org), `temporal_identifier`, date/time,
    server name (`created_by.name`), area/table if available.
  - `Items []TicketItem` — name, quantity, unit price, line total, notes.
  - Totals: subtotal, IVA, ICO, tip (propina) if any, total.
  - Footer text (e.g. "¡Gracias por su visita! Laguna Escondida"), plus any required legal
    legend (see §8 open items).
- `internal/domain/dto/print.go` — `PrintTicketRequest { OpenBillID string; Copies int }`.

### domain/ports
- `internal/domain/ports/receipt_printer.go`:
  ```go
  type ReceiptPrinter interface {
      Print(ctx context.Context, ticket *dto.Ticket) error
  }
  ```
  Port takes the structured `Ticket`, **not bytes** — ESC/POS is an adapter detail.
- Add `ReceiptPrinter` to the mockery config and run `make generate-mocks`.

### domain/service
- `internal/domain/service/print_service.go`:
  - `PrintTicket(ctx, req)` → load the open bill (existing `OpenBillRepository`), map to
    `dto.Ticket`, call `printer.Print(ctx, ticket)` `Copies` times.
  - No byte/ESC/POS knowledge. Pure orchestration + mapping.
- `internal/domain/service/print_service_test.go` (MANDATORY): mock `ReceiptPrinter` +
  bill repo. Success, bill-not-found, printer-error, copies>1, mapping/total correctness.

### internal/platform/device  (new package — the adapter)
- `escpos/renderer.go` — `Render(*dto.Ticket) ([]byte, error)`, renders into a `bytes.Buffer`
  and returns `buf.Bytes()`, so the same output feeds *any* transport (serial / spooler / tcp /
  file). **ESC/POS library decision: deferred — review later** (`hennedo/escpos` was a candidate
  but is unmaintained, ~4 yrs no updates; rejected for now). For Phase A, hand-roll a small
  pure-Go renderer (the current ticket is just text + dashed rules):
  - `ESC @` init; alignment (`ESC a`), bold (`ESC E`), size (`GS ! `), feed + cut (`GS V`).
  - Column formatter: left-align name / right-align money to the paper width
    (80 mm ≈ 48 chars Font A; 58 mm ≈ 32). Width from config.
  - **Codepage matters for accents/ñ:** select a codepage (`ESC t n`, e.g. CP850/CP858/
    WPC1252) and transcode UTF-8 → that codepage with `golang.org/x/text/encoding/charmap`.
    Validate the exact codepage on the real device.
  - Keep all ESC/POS bytes **inside this one file** so we can later swap in a maintained library
    (or keep the hand-rolled version) without touching domain/service.
  - Optional later: QR (`GS ( k`, model 2) for the CUFE, cash-drawer kick (`ESC p`), logo raster.
    QR is the main thing that might justify pulling in a (maintained) library — revisit at
    Phase C when we actually need the CUFE on the fiscal ticket.
- `escpos/renderer_test.go` — **golden-file tests**: sample `Ticket` → exact byte sequence.
  Runs on macOS, no hardware.
- `transport.go` — `Transport interface { Write(ctx, []byte) error }` + a config-driven
  factory `NewTransport(cfg)`.
- `transport_file.go` — writes `ticket-<id>-<n>.escpos` (Mac dev / golden inspection).
- `transport_tcp.go` — `net.Dial("tcp", host:9100)` (emulator or network printer; optional).
- `transport_windows.go` (`//go:build windows`) — **finish on the Windows box.** winspool RAW:
  `OpenPrinter → StartDocPrinter(DOCINFO{ datatype:"RAW" }) → StartPagePrinter → WritePrinter →
  EndPagePrinter → EndDocPrinter → ClosePrinter` via `golang.org/x/sys/windows`.
- `transport_windows_stub.go` (`//go:build !windows`) — returns "windows transport unavailable
  on this platform" so the project still **compiles, lints, and tests on macOS**.
- `transport_serial.go` (optional, `go.bug.st/serial`) — only if the printer is a COM device.
- `receipt_printer.go` — struct implementing `ports.ReceiptPrinter` = renderer + transport.

### platform/handler
- `internal/platform/handler/device_handler.go` — `PrintTicketHandler(service)`: bind JSON,
  call service, map domain/printer errors to the status codes in §3.

### cmd/main.go (edge branch only, ~lines 451–487)
- Build the transport from config, construct `device.NewReceiptPrinter(renderer, transport)`,
  inject into `NewPrintService`, register `POST /api/device/print`.
- In `cloud` mode: do **not** register the route (or return 404/"edge only").

### internal/platform/config/config.go
| Env var | Meaning | Example |
|---|---|---|
| `PRINTER_TRANSPORT` | `file` \| `tcp` \| `windows` \| `serial` | `windows` (prod), `file` (Mac dev) |
| `PRINTER_TARGET` | file dir / `host:port` / Windows printer name / COM port | `POS-80` |
| `PRINTER_WIDTH_MM` | `80` or `58` → chars per line | `80` |
| `PRINTER_CODEPAGE` | codepage for accents | `CP850` |
| `PRINTER_CUT` | `full` \| `partial` \| `none` | `partial` |

## 5. Frontend changes (companion)

In `laguna-escondida-frontend` (already scoped in its `TODO.md` §2b):
- `lib/api/device.ts` — `deviceApi.print({ open_bill_id, copies })`.
- `app/api/device/print/route.ts` — proxy → `serverApiRequest("/device/print")` (preserves the
  httpOnly-cookie auth, same as other proxied routes).
- `components/orders/PaymentModal.tsx` — replace the `window.print()` block (~lines 91–112)
  with `await deviceApi.print({ open_bill_id: openBill.id })`; **on failure fall back to the
  existing browser print** and show a toast. Same change in `EditOrderForm.tsx`.
- Keep `lib/templates/invoicePrint.ts` as the on-screen preview / browser fallback only; it is
  no longer the source of truth for the printed ticket.

## 6. Dependencies to add

- **ESC/POS encoding library: deferred — review later.** Phase A hand-rolls a small pure-Go
  renderer (text + dashed rules), keeping all bytes in `escpos/renderer.go`. Whatever we choose
  is **independent of the transport** (render into a `bytes.Buffer`, ship the bytes), so this
  decision blocks nothing. Revisit when we need QR/logo (Phase C) and pick a *maintained* lib
  then — `hennedo/escpos` is rejected for now (no updates in ~4 years).
- `golang.org/x/text` (codepage transcoding for accents/ñ).
- `golang.org/x/sys/windows` (spooler; Windows build only) — likely already indirect.
- `go.bug.st/serial` — **only if** the device turns out to be serial/COM.

## 7. Work breakdown (phased)

**Phase A — vertical slice on macOS (no hardware)**
- [ ] `dto.Ticket` + `dto.PrintTicketRequest`; `ports.ReceiptPrinter` (+ `make generate-mocks`).
- [ ] `PrintService` + unit tests (mock printer).
- [ ] `escpos.Render` + golden tests (accents, width, totals, cut).
- [ ] `fileTransport` + `device.ReceiptPrinter`; config selector with `windows` stub for non-win.
- [ ] `device_handler` + edge-only wiring; `PRINTER_TRANSPORT=file`.
- [ ] Manual check: print a real open bill → inspect `ticket-*.escpos` (`hexdump -C`), or pipe
      to a `:9100` ESC/POS emulator via `tcp` transport.
- [ ] Frontend: device API client + proxy + repoint PaymentModal (with browser fallback).
- [ ] Docs: `docs/api/device.md`; curl examples.

**Phase B — finish on the Windows box (with the printer)**
- [ ] Install the printer driver; note the Windows printer name → `PRINTER_TARGET`.
- [ ] Implement/validate `transport_windows.go` (winspool RAW).
- [ ] `GOOS=windows GOARCH=amd64 CGO_ENABLED=0` build (pgx is pure Go, fine).
- [ ] Test print on device; tune `PRINTER_WIDTH_MM`, `PRINTER_CODEPAGE`, cut, accents/ñ.
- [ ] `PRINTER_TRANSPORT=windows`; smoke-test reprint + copies + offline.

**Phase C — later / optional**
- [ ] Fiscal ticket variant (`type: "fiscal"`): payment method, CUFE + QR (`GS ( k`).
- [ ] Logo (HTML-raster renderer behind the same port, or ESC/POS bit-image).
- [ ] Cash-drawer kick; printer-status endpoint (`GET /api/device/printer/status`:
      online / out-of-paper) for the connectivity badge.

## 8. Open items / risks

- **Codepage / accents** — exact codepage (CP850 vs 858 vs 1252) is per-model; must be
  validated on the real device or "ñ/á" will be garbled.
- **Paper width** — confirm 80 mm vs 58 mm → chars-per-line (48 vs 32).
- **Legal/fiscal text** — does the "cuenta" need a "documento equivalente / no es factura"
  legend, or the QR/CUFE on the post-payment ticket? Confirm DIAN/accountant requirements
  before shipping the fiscal variant.
- **Which Windows transport** — RAW spooler is the default for a locally-attached USB printer;
  if it's actually a COM/serial device, use `serial` instead. Decide once the model is known.
- **Auth** — the device route must sit behind the same auth as order endpoints (cookie/JWT via
  the proxy). Confirm during wiring.
- **Concurrency** — serialize prints to one device (a mutex in the printer adapter) so two
  tablets don't interleave bytes.
