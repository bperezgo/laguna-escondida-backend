# Device (Ticket Printing)

Drives the receipt printer physically attached to the restaurant's edge node. The
backend renders the bill/receipt (the "cuenta") from the authoritative open bill
and sends ESC/POS bytes to the printer, so printing works fully offline and does
not depend on the browser or the electronic-invoice provider.

> **Edge mode only.** This route is registered only when the backend runs with
> `APP_MODE=edge`. In `cloud` mode it does not exist (404). If the printer
> transport fails to initialize at startup, the route is skipped and the frontend
> should fall back to browser printing.

## Endpoints

| Method | Endpoint             | Description                              |
| ------ | -------------------- | ---------------------------------------- |
| POST   | `/api/device/print`  | Render and print the ticket for a bill   |

---

## Print Ticket

`POST /api/device/print`

Renders the ticket for the given open bill and prints it. The client sends only
the bill id and an optional copy count — never layout or computed totals, which
the backend owns.

**Auth:** JWT (same as order endpoints); requires the `orders:read` permission.

### Request Body

| Field          | Type   | Required | Description                                        |
| -------------- | ------ | -------- | -------------------------------------------------- |
| `open_bill_id` | string | Yes      | UUID of the open bill to print                     |
| `copies`       | int    | No       | Number of copies to print (default `1`, min `1`, max `10`). Values above the max are clamped to `10`. |

### Example Request

```json
{
  "open_bill_id": "550e8400-e29b-41d4-a716-446655440099",
  "copies": 1
}
```

### Example Response (200 OK)

```json
{
  "printed": true
}
```

### Error Responses

**400 Bad Request** — invalid body

```json
{
  "error": "validation_error",
  "message": "invalid request body"
}
```

**404 Not Found** — no open bill with that id

```json
{
  "error": "bill_not_found"
}
```

**409 Conflict** — the printer could not be reached (offline, out of paper, no
device). The frontend can fall back to browser printing and/or show a toast.

```json
{
  "error": "printer_unavailable",
  "message": "the printer is unavailable (offline, out of paper, or no device)"
}
```

---

## Configuration (edge node)

The printer is selected at startup via environment variables:

| Env var            | Meaning                                            | Example                       |
| ------------------ | -------------------------------------------------- | ----------------------------- |
| `PRINTER_TRANSPORT`| `file` \| `tcp` \| `windows` \| `serial`           | `windows` (prod), `file` (dev)|
| `PRINTER_TARGET`   | file dir / `host:port` / Windows printer name / COM| `POS-80`                      |
| `PRINTER_WIDTH_MM` | `80` or `58` (→ 48 or 32 chars per line)           | `80`                          |
| `PRINTER_CODEPAGE` | codepage for accents/ñ (`CP850`/`CP858`/`CP1252`)  | `CP850`                       |
| `PRINTER_CUT`      | `full` \| `partial` \| `none`                      | `partial`                     |
| `BUSINESS_NAME`    | header business name                               | `Laguna Escondida`            |
| `BUSINESS_NIT`     | header NIT                                          | `900.123.456-7`               |
| `BUSINESS_ADDRESS` | header address                                     | `Vereda La Laguna`            |
| `TICKET_FOOTER`    | footer line                                        | `¡Gracias por su visita!`     |
| `TICKET_LEGAL_NOTICE` | legal legend                                    | `Documento equivalente`       |

**Transports:**

- `file` (default) — writes `ticket-NNNN.escpos` under `PRINTER_TARGET` for local
  inspection (`hexdump -C ticket-0001.escpos`). No hardware. macOS dev.
- `tcp` — sends raw ESC/POS to `host:port` (e.g. an emulator or network printer on
  `:9100`). Dev/test.
- `windows` — RAW print spooler to a locally-attached printer. Production on the
  Windows edge node (finished in Phase B).
- `serial` — reserved for a COM device (Phase B, not yet implemented).
