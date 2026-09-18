"use client";

import { calculateRemainingMs, getCountdownColor } from "@/lib/kitchen/countdown";

// "In progress" blue — mirrors STATUS_CONFIG.in_progress (orderTakingParts.tsx) and
// CommandCard so the whole in-progress language stays consistent.
const IN_PROGRESS_ACCENT = "#1d4ed8";
const IN_PROGRESS_PANEL = "#eff6ff";
const IN_PROGRESS_BORDER = "#bfdbfe";

/** One table's contribution to a batch of the same product. */
export interface InProgressBreakdownEntry {
  open_bill_product_id: string;
  temporal_identifier: string;
  quantity: number;
  notes?: string | null;
  // Countdown inputs — used to color the batch by its most-urgent line.
  priority: number;
  created_at: string;
  // Resolved side dishes (names resolved by the backend). Only feeds the
  // aggregate "Guarniciones a preparar" tally — not shown per table.
  side_dishes?: { name: string; quantity: number }[];
}

/** All in-progress lines of one product, aggregated across tables. */
export interface InProgressGroup {
  product_name: string;
  totalQuantity: number;
  entries: InProgressBreakdownEntry[];
}

interface InProgressStripProps {
  groups: InProgressGroup[];
  /** Shared clock from the parent view — drives the per-batch urgency accent. */
  now: number;
}

/**
 * Read-only top strip for the kitchen board: the products cooks are cooking right
 * now, batched across tables ("3× Mojarra frita → mesa 4 · mesa 7 · barra 2") so
 * identical dishes get fired together. Completing a line stays on its command card
 * (the table context there is unambiguous); this strip only summarizes.
 */
export default function InProgressStrip({ groups, now }: InProgressStripProps) {
  if (groups.length === 0) return null;

  const totalPlates = groups.reduce((sum, g) => sum + g.totalQuantity, 0);

  // Total portions of each side dish across everything in progress, so the line
  // knows how much rice / fries / patacones / salad to have ready. Portions scale
  // with the number of plates on the line (3 plates × 2 arroz = 6 arroz). A
  // removed side dish (qty 0) contributes nothing. Biggest tallies first.
  const sideDishTotals = (() => {
    const totals = new Map<string, number>();
    for (const group of groups) {
      for (const entry of group.entries) {
        for (const sd of entry.side_dishes ?? []) {
          if (sd.quantity <= 0) continue;
          totals.set(
            sd.name,
            (totals.get(sd.name) ?? 0) + sd.quantity * entry.quantity,
          );
        }
      }
    }
    return Array.from(totals.entries()).sort((a, b) => b[1] - a[1]);
  })();

  return (
    <div
      style={{
        backgroundColor: IN_PROGRESS_PANEL,
        border: `1px solid ${IN_PROGRESS_BORDER}`,
        borderRadius: "var(--radius-lg)",
        padding: "1rem 1.25rem",
        marginBottom: "1.5rem",
      }}
    >
      {/* Header */}
      <div
        style={{
          display: "flex",
          alignItems: "center",
          gap: "0.75rem",
          marginBottom: "1rem",
        }}
      >
        <h2
          style={{
            margin: 0,
            fontSize: "1.25rem",
            fontWeight: "bold",
            color: IN_PROGRESS_ACCENT,
            display: "flex",
            alignItems: "center",
            gap: "0.5rem",
          }}
        >
          🔥 En Progreso
        </h2>
        <span
          style={{
            fontSize: "0.95rem",
            fontWeight: "600",
            color: IN_PROGRESS_ACCENT,
            backgroundColor: "#dbeafe",
            padding: "0.25rem 0.75rem",
            borderRadius: "var(--radius-sm)",
          }}
        >
          {totalPlates} cocinándose ahora
        </span>
      </div>

      {/* Side-dish prep list: total portions of each guarnición to have ready. */}
      {sideDishTotals.length > 0 && (
        <div
          style={{
            display: "flex",
            flexWrap: "wrap",
            alignItems: "center",
            gap: "0.5rem",
            marginBottom: "1rem",
            paddingBottom: "0.875rem",
            borderBottom: `1px solid ${IN_PROGRESS_BORDER}`,
          }}
        >
          <span
            style={{
              fontSize: "0.9rem",
              fontWeight: "700",
              color: IN_PROGRESS_ACCENT,
            }}
          >
            🍽️ Guarniciones a preparar:
          </span>
          {sideDishTotals.map(([name, total]) => (
            <span
              key={name}
              style={{
                fontSize: "1.05rem",
                fontWeight: "800",
                color: "var(--color-text-primary)",
                backgroundColor: "var(--color-surface)",
                border: `1px solid ${IN_PROGRESS_BORDER}`,
                borderLeft: `4px solid ${IN_PROGRESS_ACCENT}`,
                padding: "0.25rem 0.7rem",
                borderRadius: "var(--radius-sm)",
                whiteSpace: "nowrap",
                fontVariantNumeric: "tabular-nums",
              }}
            >
              {total}× {name}
            </span>
          ))}
        </div>
      )}

      {/* Batch tiles */}
      <div
        style={{
          display: "grid",
          gridTemplateColumns: "repeat(auto-fill, minmax(min(280px, 100%), 1fr))",
          gap: "0.75rem",
        }}
      >
        {groups.map((group) => {
          // Color the batch by its most-urgent line so a near-due batch pops.
          const mostUrgentRemaining = Math.min(
            ...group.entries.map((e) =>
              calculateRemainingMs(e.priority, e.created_at, now)
            )
          );
          const urgency = getCountdownColor(mostUrgentRemaining);

          return (
            <div
              key={group.product_name}
              style={{
                backgroundColor: "var(--color-surface)",
                border: `1px solid ${IN_PROGRESS_BORDER}`,
                borderLeft: `4px solid ${IN_PROGRESS_ACCENT}`,
                borderRadius: "var(--radius-md)",
                padding: "0.75rem",
                display: "flex",
                flexDirection: "column",
                gap: "0.5rem",
              }}
            >
              {/* Product + total quantity */}
              <div
                style={{ display: "flex", alignItems: "center", gap: "0.625rem" }}
              >
                <span
                  style={{
                    fontSize: "1.4rem",
                    fontWeight: "bold",
                    lineHeight: 1.1,
                    color: urgency.text,
                    backgroundColor: urgency.bg,
                    padding: "0.25rem 0.625rem",
                    borderRadius: "var(--radius-sm)",
                    minWidth: "3rem",
                    textAlign: "center",
                    flexShrink: 0,
                  }}
                  title="Total en progreso"
                >
                  {group.totalQuantity}×
                </span>
                <span
                  style={{
                    fontSize: "1.1rem",
                    fontWeight: "600",
                    color: "var(--color-text-primary)",
                    minWidth: 0,
                  }}
                >
                  {group.product_name}
                </span>
              </div>

              {/* Per-table breakdown */}
              <div
                style={{
                  display: "flex",
                  flexWrap: "wrap",
                  gap: "0.375rem",
                }}
              >
                {group.entries.map((entry) => (
                  <span
                    key={entry.open_bill_product_id}
                    title={entry.notes ? `📝 ${entry.notes}` : undefined}
                    style={{
                      fontSize: "0.85rem",
                      fontWeight: "600",
                      whiteSpace: "nowrap",
                      color: IN_PROGRESS_ACCENT,
                      backgroundColor: "#dbeafe",
                      padding: "0.2rem 0.5rem",
                      borderRadius: "var(--radius-sm)",
                    }}
                  >
                    {entry.temporal_identifier}
                    {entry.quantity > 1 ? ` ×${entry.quantity}` : ""}
                    {entry.notes ? " 📝" : ""}
                  </span>
                ))}
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
