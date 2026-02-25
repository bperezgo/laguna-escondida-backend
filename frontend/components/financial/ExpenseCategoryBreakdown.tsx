"use client";

import type { ExpenseCategoryData } from "@/types/financialSummary";

interface ExpenseCategoryBreakdownProps {
  categories: ExpenseCategoryData[];
  totalAmount: string;
  totalCount: number;
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

export default function ExpenseCategoryBreakdown({
  categories,
  totalAmount,
  totalCount,
}: ExpenseCategoryBreakdownProps) {
  const total = parseFloat(totalAmount) || 0;

  return (
    <div
      style={{
        backgroundColor: "var(--color-surface)",
        border: "1px solid var(--color-border)",
        borderRadius: "var(--radius-md)",
        padding: "1.25rem 1.5rem",
        boxShadow: "var(--shadow-md)",
      }}
    >
      <p
        style={{
          fontSize: "0.75rem",
          fontWeight: 600,
          textTransform: "uppercase",
          letterSpacing: "0.05em",
          color: "var(--color-text-secondary)",
          marginBottom: "1rem",
        }}
      >
        Gastos por Categoría
      </p>

      {categories.length === 0 ? (
        <p style={{ color: "var(--color-text-muted)", fontSize: "0.9rem" }}>
          Sin datos de categorías.
        </p>
      ) : (
        <div style={{ display: "flex", flexDirection: "column", gap: "0.75rem" }}>
          {categories.map((cat) => {
            const catAmount = parseFloat(cat.total_amount) || 0;
            const pct = total > 0 ? (catAmount / total) * 100 : 0;

            return (
              <div key={cat.category_id}>
                <div
                  style={{
                    display: "flex",
                    justifyContent: "space-between",
                    alignItems: "baseline",
                    marginBottom: "0.3rem",
                    gap: "0.5rem",
                  }}
                >
                  <span
                    style={{
                      fontSize: "0.875rem",
                      color: "var(--color-text-primary)",
                      fontWeight: 500,
                      display: "flex",
                      alignItems: "center",
                      gap: "0.4rem",
                    }}
                  >
                    {cat.category_name}
                    <span
                      style={{
                        fontSize: "0.7rem",
                        fontWeight: 600,
                        color: "var(--color-text-muted)",
                        backgroundColor: "var(--color-surface-hover)",
                        padding: "0.1rem 0.4rem",
                        borderRadius: "var(--radius-sm)",
                      }}
                    >
                      {cat.category_code}
                    </span>
                  </span>
                  <span
                    style={{
                      fontSize: "0.875rem",
                      color: "var(--color-text-primary)",
                      fontWeight: 600,
                      whiteSpace: "nowrap",
                    }}
                  >
                    {formatCurrency(cat.total_amount)}
                    <span
                      style={{
                        fontSize: "0.75rem",
                        fontWeight: 400,
                        color: "var(--color-text-muted)",
                        marginLeft: "0.4rem",
                      }}
                    >
                      ({cat.count} docs)
                    </span>
                  </span>
                </div>
                {/* Progress bar */}
                <div
                  style={{
                    height: "6px",
                    backgroundColor: "var(--color-surface-hover)",
                    borderRadius: "var(--radius-sm)",
                    overflow: "hidden",
                  }}
                >
                  <div
                    style={{
                      height: "100%",
                      width: `${pct}%`,
                      backgroundColor: "var(--color-warning)",
                      borderRadius: "var(--radius-sm)",
                      transition: "width var(--transition-slow)",
                    }}
                  />
                </div>
              </div>
            );
          })}

          {/* Total row */}
          <div
            style={{
              borderTop: "1px solid var(--color-border)",
              paddingTop: "0.75rem",
              marginTop: "0.25rem",
              display: "flex",
              justifyContent: "space-between",
            }}
          >
            <span
              style={{
                fontSize: "0.875rem",
                fontWeight: 600,
                color: "var(--color-text-secondary)",
              }}
            >
              Total gastos
            </span>
            <span
              style={{
                fontSize: "0.875rem",
                fontWeight: 700,
                color: "var(--color-text-primary)",
              }}
            >
              {formatCurrency(totalAmount)}
              <span
                style={{
                  fontSize: "0.75rem",
                  fontWeight: 400,
                  color: "var(--color-text-muted)",
                  marginLeft: "0.4rem",
                }}
              >
                ({totalCount} docs)
              </span>
            </span>
          </div>
        </div>
      )}
    </div>
  );
}
