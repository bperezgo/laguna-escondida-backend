"use client";

import OpenBillCard from "./OpenBillCard";
import type { OpenBill } from "@/types/order";
import { getGroupAccent, type WaitressGroup } from "@/lib/orders/grouping";

interface WaitressOrderGroupProps {
  group: WaitressGroup;
  onBillClick: (bill: OpenBill) => void;
  onPayClick: (bill: OpenBill) => void;
  onRemoveClick: (bill: OpenBill) => void;
}

export default function WaitressOrderGroup({
  group,
  onBillClick,
  onPayClick,
  onRemoveClick,
}: WaitressOrderGroupProps) {
  const accent = getGroupAccent(group);

  return (
    <section
      style={{
        marginBottom: "2rem",
        padding: "1rem 1.25rem 1.25rem",
        backgroundColor: accent.tint,
        border: "1px solid var(--color-border)",
        borderLeftWidth: "4px",
        borderLeftStyle: "solid",
        borderLeftColor: accent.stripe,
        borderRadius: "var(--radius-lg)",
      }}
    >
      <div
        style={{
          display: "flex",
          alignItems: "center",
          gap: "0.75rem",
          marginBottom: "1rem",
        }}
      >
        <span
          style={{
            display: "inline-flex",
            alignItems: "center",
            gap: "0.5rem",
            padding: "0.3rem 0.75rem",
            borderRadius: "999px",
            backgroundColor: accent.chipBg,
            border: `1px solid ${accent.chipBorder}`,
          }}
        >
          <span
            aria-hidden
            style={{
              width: "10px",
              height: "10px",
              borderRadius: "50%",
              backgroundColor: accent.stripe,
              flexShrink: 0,
            }}
          />
          <span
            style={{
              fontSize: "1.05rem",
              fontWeight: 700,
              color: accent.heading,
              lineHeight: 1.2,
            }}
          >
            {group.label}
          </span>
        </span>
        <span
          style={{
            fontSize: "0.85rem",
            fontWeight: 600,
            color: "var(--color-text-secondary)",
            backgroundColor: "var(--color-surface)",
            border: "1px solid var(--color-border)",
            borderRadius: "999px",
            padding: "0.1rem 0.6rem",
            fontVariantNumeric: "tabular-nums",
          }}
        >
          {group.count}
        </span>
      </div>

      <div
        style={{
          display: "grid",
          gridTemplateColumns: "repeat(auto-fill, minmax(min(320px, 100%), 1fr))",
          gap: "1.25rem",
        }}
      >
        {group.bills.map((bill) => (
          <OpenBillCard
            key={bill.id}
            openBill={bill}
            isMine={group.isMine}
            onClick={() => onBillClick(bill)}
            onPayClick={() => onPayClick(bill)}
            onRemoveClick={() => onRemoveClick(bill)}
          />
        ))}
      </div>
    </section>
  );
}
