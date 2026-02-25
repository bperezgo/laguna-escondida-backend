"use client";

import { useState } from "react";

interface SummaryKpiCardProps {
  label: string;
  amount: string;
  count?: number;
  countLabel?: string;
  accentColor: string;
  fullWidth?: boolean;
}

const formatCurrency = (value: string) => {
  const num = parseFloat(value);
  if (isNaN(num)) return "$0";
  return new Intl.NumberFormat("es-CO", {
    style: "currency",
    currency: "COP",
    minimumFractionDigits: 0,
    maximumFractionDigits: 0,
  }).format(num);
};

export default function SummaryKpiCard({
  label,
  amount,
  count,
  countLabel = "registros",
  accentColor,
  fullWidth = false,
}: SummaryKpiCardProps) {
  const [hovered, setHovered] = useState(false);

  return (
    <div
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
      style={{
        backgroundColor: "var(--color-surface)",
        border: `1px solid ${hovered ? accentColor : "var(--color-border)"}`,
        borderRadius: "var(--radius-md)",
        padding: "1.25rem 1.5rem",
        boxShadow: hovered ? "var(--shadow-lg)" : "var(--shadow-md)",
        transition: "all var(--transition-normal)",
        transform: hovered ? "translateY(-2px)" : "translateY(0)",
        ...(fullWidth ? { gridColumn: "1 / -1" } : {}),
      }}
    >
      <p
        style={{
          fontSize: "0.75rem",
          fontWeight: 600,
          textTransform: "uppercase",
          letterSpacing: "0.05em",
          color: "var(--color-text-secondary)",
          marginBottom: "0.5rem",
        }}
      >
        {label}
      </p>
      <p
        style={{
          fontSize: "1.75rem",
          fontWeight: 700,
          color: accentColor,
          margin: "0 0 0.25rem",
          lineHeight: 1.1,
        }}
      >
        {formatCurrency(amount)}
      </p>
      {count !== undefined && (
        <p
          style={{
            fontSize: "0.8rem",
            color: "var(--color-text-muted)",
            margin: 0,
          }}
        >
          {count} {countLabel}
        </p>
      )}
    </div>
  );
}
