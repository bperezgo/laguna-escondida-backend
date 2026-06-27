# Planning: Fish Weight Discount Entity

> Status: **Investigation / options** — no implementation yet.
> Goal: avoid double-charging the weight of fish that a guest both **catches** (charged by weight) and **eats as a prepared plate** (fixed plate price that already covers a standard fish weight).

## 1. The business problem

The fishing place sells fish two ways, at (roughly) the same effective price:

1. **By weight (raw catch)** — the guest fishes, we weigh the catch and charge per gram. Example product: `Tilapia (peso)`, `unit_of_measure = g`, priced per gram.
2. **As a prepared plate (fixed price)** — e.g. `Plato Tilapia = $40.000`. A plate is assumed to contain a **standard weight of fish** (e.g. 500 g), regardless of the actual fish used.

When a guest catches fish **and** eats some of it as a plate, the plate price already covers that fish. If we also charged the full caught weight, we'd bill the same fish twice.

### Worked example (the canonical case)

- Guest catches **2 tilapias = 800 g** total.
- Guest eats **1 plate of tilapia** (`$40.000`), which represents **500 g** of fish.
- We want to bill:
  - `Plato Tilapia` × 1 = **$40.000**
  - `Tilapia (peso)` = **800 g caught − 500 g (covered by the plate) = 300 g charged**

### Hard requirements (from the request)

- **R1 — Record the gross caught weight (800 g).** We do *not* want to silently store only 300 g. The 800 g must remain visible/recorded.
- **R2 — Apply a deduction of the plate's standard weight (500 g).** Deduction is a **fixed weight per plate** (500 g for a tilapia plate), not the plate's real fish content.
- **R3 — Deduction is per species.** A tilapia plate deducts from tilapia weight, not from another species' weight.
- **R4 — Net charge = gross weight − Σ(plate standard weight) for that species**, floored at 0 (can't deduct more than was caught — see open questions).

### Constraints from the current model

- Line `Quantity` is an **`int`** everywhere (`OrderProductItem`, `OpenBillProductDetail`, `BillProduct`). Grams therefore fit naturally as integer quantities.
- A discount/allowance pipeline **already exists** and flows into the electronic invoice (DIAN):
  - `Bill.DiscountAmount`, `BillProduct.Allowance []InvoiceAllowance` (`BaseAmount`, `Amount`, `ReasonCode`, `Description`), DB column `bills.discount_amount`.
  - `PayAmount = TotalAmount + TaxAmount − DiscountAmount`.
- Composite plates already exist: `ProductTypeComposite` + `product_ingredients` (composite → ingredient + decimal quantity).
- `FinancialSummary.Revenue.TotalDiscount` already aggregates discounts for reporting.

> **Key tension for R1 vs R4:** "record 800, charge 300" can be modeled either as
> (a) one line of 800 g **plus a discount** of 500 g-equivalent (gross-and-allowance), or
> (b) one line of 300 g net **plus a separate record** of the 800 g caught.
> Most options below favor (a) because it reuses the existing allowance pipeline and keeps the invoice math honest.

---

## 2. Where the deduction can live (two orthogonal decisions)

Designing this entity is really **two** decisions:

- **Decision A — What declares "a plate covers N g of species X"?** (the *configuration*)
- **Decision B — How is the deduction represented on the bill?** (the *effect*)

The options below are combinations of these two.

---

## 3. Options

### Option A — Plate carries a "fish weight equivalent"; deduction as an allowance

**Config (Decision A):** Add a small mapping that says *"plate product P offsets N grams of weight-product W."* This can be:
- a new column/field on the plate `Product` (e.g. `fish_offset_product_id`, `fish_offset_grams`), or
- a dedicated tiny entity `PlateFishOffset { plate_product_id, weight_product_id, grams }`.

**Effect (Decision B):** At `PayOrder`, for every plate on the bill, accumulate offset grams per weight-product, then attach an `InvoiceAllowance` to that weight-product's bill line (`Amount = offset_grams × unit_price`, `BaseAmount`, a `ReasonCode` like "fish prepared as plate"). The raw line keeps its **gross 800 g** (satisfies R1); the allowance reduces the charge (satisfies R2/R4).

**Pros**
- Reuses the existing allowance → discount → electronic-invoice pipeline. Tax base reduction is handled by the mechanism already shipped.
- Keeps gross weight on the line (R1) and is fiscally explicit (the deduction shows as a documented allowance).
- Small, focused new config entity; per-species mapping is explicit (R3).

**Cons**
- Need a clear rule when there's no matching raw-weight line (guest ordered a plate but didn't fish / fished a different species) — see open questions.
- Allowance math must be exact to the cent (decimal) to keep DIAN happy.

**Best when:** we want correctness on the invoice and reuse of existing infra. **(Recommended — see §5.)**

---

### Option B — Reuse `product_ingredients` (composite recipe) as the offset source

**Config (Decision A):** Declare each plate as `COMPOSITE` and add a `product_ingredient` row linking the plate to the weight-product `Tilapia (peso)` with `quantity = 500`. The recipe *is* the offset table.

**Effect (Decision B):** same as Option A — translate matched ingredient quantities into an `InvoiceAllowance` on the raw-weight line at pay time.

**Pros**
- No new entity at all; uses a table that already exists.
- Conceptually elegant: "the plate is made of 500 g of this fish."

**Cons**
- Overloads `product_ingredients`, whose current purpose is composition/inventory, not billing offsets. A recipe quantity used for stock could diverge from the **billing** offset (the request explicitly says "we remove 500 for simplicity… even if the real fish weighs less"). Mixing the two meanings is a latent bug.
- Requires every plate to be `COMPOSITE`, which may not be true today.
- Harder to special-case billing rules (floor at 0, no-catch case) without polluting recipe semantics.

**Best when:** recipes and billing offsets are guaranteed to be the same number forever (they probably aren't).

---

### Option C — First-class generic `Discount` entity with rules

**Config (Decision A):** Introduce a general `Discount` domain entity: `{ id, type, trigger, target, amount/percent, ... }`. The fish-weight case is one rule type (e.g. `type = WEIGHT_OFFSET`, `trigger_product = plate`, `target_product = weight fish`, `grams = 500`).

**Effect (Decision B):** A discount-evaluation step in `PayOrder` applies all matching rules, emitting `InvoiceAllowance`s (and/or bill-level discount).

**Pros**
- Future-proof: percentage promos, happy-hour, member discounts, combos all fit later.
- Centralizes discount logic in one place.

**Cons**
- Significantly more design + surface area than the problem needs **today**. Risk of over-engineering.
- A generic rules engine invites edge-case complexity (stacking, ordering, precedence) we don't have requirements for yet.

**Best when:** we already know several other discount types are coming soon. Otherwise premature.

---

### Option D — Net-weight line + separate "caught weight" record (no allowance)

**Config (Decision A):** same mapping as Option A (plate → grams of species).

**Effect (Decision B):** Store the **net 300 g** as the billed quantity, and record the **gross 800 g** somewhere else (a `caught_weight` field/record on the order line or a separate catch log).

**Pros**
- The invoice line is simply the net amount — no allowance math, no DIAN allowance entries.

**Cons**
- Splits "what was caught" from "what was charged" into two places — easy to desync and harder to audit.
- Reporting of discounts/foregone revenue is lost unless separately computed (Option A gets it free via `TotalDiscount`).
- Arguably violates the spirit of R1 ("record the 800g") by hiding it off the bill line.

**Best when:** we explicitly do **not** want the deduction to appear as a fiscal allowance and prefer net pricing. Needs confirmation from accounting.

---

### Option E — Negative-price "discount product" line

**Config (Decision A):** Create a regular `Product` whose price is **negative** (e.g. `Descuento pescado capturado (peso)` at a negative per-gram price, or a generic negative-amount discount product). Staff add it as a line on the bill.

**Effect (Decision B):** The negative line reduces `TotalAmount` directly when the bill totals are summed.

> "Probably the easy one" — true for the *first edit*, but the codebase turns it into several non-obvious consequences, including a silent tax bug. Anchored to the current code:

**What blocks it today**
- `product/product.go → calculateTaxesAndUnitPrice` rejects it twice:
  - lines 48–50: `total_price_with_taxes must be greater than 0` (negative **and** zero are refused).
  - line 69: `vat` and `ico` cannot both be `0` (division-by-zero guard) — so even a *tax-free* discount product is rejected. You'd have to loosen both checks and rethink the proportional tax math for a negative base.

**The silent tax bug (most important)**
- `bill/bill_product.go` only emits tax lines when the amount is positive:
  - line 43: `if vatAmount.GreaterThan(decimal.Zero)` … line 52: `if icoAmount.GreaterThan(decimal.Zero)`.
- A negative-price product has **negative** `vatAmount`/`icoAmount`, so **no tax line is emitted at all**.
- Then in `bill/bill.go → NewBillFromCreateElectronicInvoiceRequest`:
  - line 49: `totalAmount` **decreases** by the negative line, but
  - lines 59–72: `taxAmount` is summed only from emitted tax lines, so it **does not decrease**.
- Net effect: the discount lowers the subtotal but the customer is **still taxed on the pre-discount base**. The product-level invariant `unitPrice + vatAmount + icoAmount = totalPriceWithTaxes` no longer holds at the bill level. This is wrong and easy to miss.

**Fiscal / DIAN**
- The electronic-invoice pipeline (`pending_invoices`, CUFE, `InvoiceAllowance`) targets Colombian e-invoicing, which models discounts as **allowances (descuentos)**, not negative-priced lines / negative quantities. A negative line is typically rejected or non-conformant. The `InvoiceAllowance` type exists precisely to do this correctly — a negative product bypasses it.

**Reporting**
- `FinancialSummary.Revenue.TotalDiscount` is fed by `discountAmount` (allowances). A negative product contributes **$0** to `TotalDiscount` and instead just lowers revenue — so comped fish weight becomes invisible *as a discount* and looks like lower sales. Lost audit trail.

**Catalog / UX**
- The discount product shows up in product lists, menus, SKU lookups, bulk ops and search; every listing needs to filter it out. It can also be added to arbitrary bills by mistake, and (if mis-configured with a preparation responsibility) could surface on a kitchen display.

**Matching the species / weight**
- A negative product doesn't inherently know "500 g of tilapia". You'd either create one negative product **per species** (then add `quantity = 500`, which parallels the weight model and is the least-bad variant) or a single generic discount product where staff **type the amount manually** (error-prone, no link to the plate, no enforcement of the 500 g rule).

**Where it does (narrowly) work**
- Arithmetic is fine: `quantity` is `int`, and `unitPrice.Mul(quantity)` with a negative price computes correctly (`bill.go:49`).
- It does satisfy **R1** at the line level: the 800 g fish line stays intact and the deduction appears as its own −500 g-equivalent line.

**Verdict:** cheap to start, but it bypasses the system's purpose-built discount primitive and silently breaks the tax base, fiscal conformance, and discount reporting. If the goal is "simplest thing that's still correct," that's **Option A** (emit an `InvoiceAllowance` on the fish line), not a negative product. Only consider Option E if (a) these items never go on an electronic invoice, and (b) the tax-emission guards are fixed to allow negative tax — at which point it's no longer the "easy" change.

---

## 4. Comparison

| Criterion | A: Plate offset + allowance | B: Reuse recipe | C: Generic discount | D: Net + catch log | E: Negative product |
|---|---|---|---|---|---|
| New entities | 1 tiny mapping | 0 | 1 large + rules | 1 field/log | 0 (1+ products) |
| Reuses allowance/invoice pipeline | ✅ | ✅ | ✅ | ❌ | ❌ (bypasses it) |
| Keeps gross 800 g on the line (R1) | ✅ | ✅ | ✅ | ⚠️ (off-line) | ✅ (2 lines) |
| Per-species (R3) | ✅ explicit | ✅ via recipe | ✅ via rule | ✅ | ⚠️ manual / per-species product |
| Billing offset ≠ recipe weight kept clean | ✅ | ❌ overloads recipe | ✅ | ✅ | ✅ |
| Implementation cost | Low | Lowest | High | Low–Med | Low to start, hidden cost after |
| Future discount types | ❌ | ❌ | ✅ | ❌ | ❌ |
| Tax base reduced correctly | ✅ | ✅ | ✅ | N/A (net) | ❌ **silent bug** |
| Fiscal/DIAN correctness handled | ✅ | ✅ | ✅ | N/A (net) | ❌ negative line non-conformant |
| Shows as discount in reports | ✅ | ✅ | ✅ | ❌ | ❌ (looks like lower sales) |

---

## 5. Recommendation (to validate, not yet build)

**Option A** — a small dedicated `PlateFishOffset` mapping + emit an `InvoiceAllowance` on the raw-weight line at `PayOrder`.

Rationale:
- Matches both requirements cleanly: gross 800 g stays on the line (R1), allowance encodes the 500 g deduction (R2/R4), mapping is per-species (R3).
- Reuses the **already-shipped** allowance → `discount_amount` → electronic-invoice path, so tax base and DIAN reporting are correct with little new code.
- Keeps **billing offset** separate from **recipe/inventory weight**, which the request says will deliberately differ ("we remove 500 for simplicity").
- Leaves the door open to refactor into Option C later if more discount types appear, without a rewrite.

Sketch of the touched layers (per CLAUDE.md flow):
1. `domain/dto` — `PlateFishOffset` DTO + request DTOs.
2. `domain/ports` — `PlateFishOffsetRepository` interface.
3. `domain/service` — offset-resolution logic inside `OrderService.PayOrder` (or a small `DiscountService`); unit tests (mandatory).
4. `platform/postgres/migrations` — `plate_fish_offsets` table.
5. `platform/postgres/repository` — implementation.
6. `platform/handler` — CRUD for offsets (admin config).
7. `docs/api/` — document the new endpoints.

---

## 6. Open questions (need answers before building)

1. **No-catch / over-catch:** If a guest orders a tilapia plate but caught **0 g** (or 300 g < 500 g) of tilapia, what happens? Options: floor the deduction at the caught weight (deduct max 300, never go negative) vs. allow the plate to "consume" only what exists. The example floors at 0 net; please confirm the over-deduction rule.
2. **Multiple species / multiple plates:** A guest catches tilapia + trout and orders 2 tilapia plates + 1 trout plate. Confirm: deductions are summed **per species** and matched independently. (Assumed yes — R3.)
3. **Plate without fishing (kitchen fish):** Plates ordered with no fishing at all should produce **no deduction** and no negative line. Confirm the offset only applies when a matching raw-weight line exists on the same bill.
4. **Identifying "the species":** How do we link a plate to its weight-product? By an explicit `weight_product_id` (precise, recommended) or by category/name convention (fragile)?
5. **Gross weight capture:** Where is the 800 g entered today — is the weight-fish product added to the order with `quantity = 800`, or is weight captured elsewhere? This determines whether the raw line already exists at pay time.
6. **Tax behavior:** The fish weight presumably carries VAT/ICO. Confirm the allowance should reduce the **taxable base** (standard) so taxes are computed on the net 300 g.
7. **Plate "standard weight" source of truth:** Is 500 g fixed per plate product, or could it vary by plate size (e.g. small/large plate)? This sizes the mapping (one grams value vs. several).
8. **Rounding/units:** Per-gram price × grams can produce fractional cents. Confirm rounding rule (the codebase uses `shopspring/decimal`, so we can be exact, but DIAN expects a defined rounding).

---

## 7. Glossary / current-model anchors

- **Weight-fish product:** `Product` with `unit_of_measure = g|kg`; line quantity = grams.
- **Plate:** fixed-price `Product` (possibly `COMPOSITE`).
- **Allowance:** `InvoiceAllowance` per `BillProduct`; sums into `Bill.DiscountAmount`; feeds the electronic invoice.
- **PayAmount:** `TotalAmount + TaxAmount − DiscountAmount`.
