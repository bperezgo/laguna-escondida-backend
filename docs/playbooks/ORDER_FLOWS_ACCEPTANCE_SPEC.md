# Order Flows Acceptance Spec — Takeaway, Delivery and the Agent

**Read this when you change how orders are taken, edited, paid or prepared, or what the
WhatsApp agent may do.** It is the plain-language record of what must hold; the tests prove it.

| Document | Role |
| --- | --- |
| **This file** | The flows and rules, step by step, and why each matters |
| `test/acceptance/sync/order_flows_test.go` | The flows, run over HTTP against real Postgres |
| `cmd/agent_routes_test.go` | The agent's route allowlist, checked against every registered route |
| `internal/domain/service/order_service_agent_test.go` | The agent scope rule, unit-level |

Each flow has an ID (`FLOW-…`) and numbered steps. A test is named after its flow and each
step is a subtest named after the step, so `go test -v -run TestOrderFlow` prints this
document back. Run them with `make test-acceptance` (needs Docker; Postgres starts itself).

**Who does what.** Each step calls the API as the person who does it in the restaurant: a
**server** (or the **agent**) takes and changes the order, a **manager** voids it or records
the payment, a **cook** prepares it. Payment is checked by a human outside the system, so
the tests record it the way staff do, without a real payment provider.

---

## FLOW-DEL / FLOW-TKW — the takeaway and delivery lifecycle

Both order types wait for payment before the kitchen sees them, so they share one flow. A
delivery additionally carries an address and a pin inside an active delivery zone; a
takeaway carries only the customer's phone.

| Step | What must hold | Why |
| --- | --- | --- |
| 01 | Creating the order answers `201`, records the type, the phone in E.164 and (delivery) the zone; it is **unpaid** and **not on the kitchen board**. | Food must not be cooked for an order nobody has paid. |
| 02 | Editing it — changing a quantity, removing a line, adding a product — answers `200`; the total follows; the type, contact and location never change. | A customer changes their mind before paying; a moved customer re-orders instead. |
| 03 | Cancelling a single line answers `200` and the line is `cancelled`. | Partial changes without voiding the whole order. |
| 04 | `in-progress` and `complete` on an unpaid line answer `409 ORDER_AWAITING_PAYMENT` for **a cook and a manager alike**; the line does not move. | Nobody, whatever their role, can start an unpaid order. |
| 05 | Once staff record the payment the order **closes**: it leaves the active list and appears, with `paid_at`, in today's closed orders. | Payment is the hand-off from front of house to the kitchen. |
| 06 | Paying puts the live lines on the board (a cancelled line never gets there); the cook moves each to `in_progress` then `completed`, and done lines leave the board. | The kitchen works only on paid food, and finishes it. |
| 07 | A paid order is locked: an edit answers `404`, cancelling a line `409`, and a **delete answers `404` with no side effects** — no order-deleted event (which would hand stock back and pull lines off the board) and no delete replicated to other nodes. | Money has been taken; refunds are a staff decision outside the system. |

**The end of the flow today is "every line completed".** Nothing records that food left the
restaurant or reached the customer yet; that is a planned feature (dispatched / delivered
states, marked by staff).

## FLOW-VOID — cancelling a whole unpaid order

A manager, or the agent on its own order, deletes an unpaid order: `200`, it leaves the
active list, and every line action on it answers `404`. Servers cannot void (no
`orders:delete`).

Voiding it a second time answers `404` and publishes nothing, so its stock is never returned
twice. Likewise an id that is not an order answers `404` on read (open and closed) and on
delete, with nothing published or replicated — never an empty order with `200`.

## FLOW-AGT — what the WhatsApp agent may do

The agent signs in as a service user with only the `agent` role. All customers share that
one user, so the backend can tell **agent orders from staff orders**, but not one customer
from another — keeping customer A away from customer B's order is the MCP customer
toolset's job (it checks the `X-Customer-Phone` header).

| ID | What must hold | Why |
| --- | --- | --- |
| AGT-01 | The agent runs FLOW-DEL/TKW steps 01–03 and 07 on its own orders; staff pay and cook it like any other order. | The agent takes orders; the restaurant's normal pipeline does the rest. |
| AGT-02 | Pay, `in-progress`, `complete` and `uncomplete` answer `403`; nothing is paid and no line moves. | The agent never takes money nor drives the kitchen. |
| AGT-03 | Creating a `dine_in` order — explicitly or by omitting `order_type` — answers `403 AGENT_ORDER_TYPE_NOT_ALLOWED` and creates nothing. | Tables are taken by staff in the restaurant. |
| AGT-04 | Editing, cancelling a line of, or deleting an order **it did not create** — a dine-in table, or a takeaway/delivery taken by staff — answers `403 ORDER_NOT_OWNED`; the order is untouched. | A leaked agent credential must not be able to void the restaurant's tables. |
| AGT-05 | Against every registered route, in cloud and edge mode, the agent gets past the guards **only** on the allowlist in `cmd/agent_routes_test.go`; everything else answers `401`/`403`. A new route reachable by the agent fails the test until it is added to the list on purpose. | "The agent can do nothing else" is checked, not assumed. |

**Known by design, reviewed here:** `orders:read` lets the agent read *every* order and both
kitchen boards, not just its own (the allowlist says so explicitly). Narrowing reads is a
separate decision.

## How to extend

Add a step to the table first, in words, and agree on it; then add the subtest with the
same number and name. Never change what a step asserts to make a red run green — change the
spec, on purpose, first.
