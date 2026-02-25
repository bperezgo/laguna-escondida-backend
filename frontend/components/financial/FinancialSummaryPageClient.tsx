"use client";

import { useState, useEffect, useCallback } from "react";
import { financialSummaryApi } from "@/lib/api/financialSummary";
import type { FinancialSummary } from "@/types/financialSummary";
import SummaryKpiCard from "./SummaryKpiCard";
import ExpenseCategoryBreakdown from "./ExpenseCategoryBreakdown";

const getMonthBounds = () => {
  const now = new Date();
  const first = new Date(now.getFullYear(), now.getMonth(), 1);
  const last = new Date(now.getFullYear(), now.getMonth() + 1, 0);
  const fmt = (d: Date) => d.toISOString().split("T")[0];
  return { start: fmt(first), end: fmt(last) };
};

export default function FinancialSummaryPageClient() {
  const monthBounds = getMonthBounds();

  const [startDate, setStartDate] = useState<string>(monthBounds.start);
  const [endDate, setEndDate] = useState<string>(monthBounds.end);
  const [summary, setSummary] = useState<FinancialSummary | null>(null);
  const [loading, setLoading] = useState<boolean>(false);
  const [error, setError] = useState<string>("");

  const fetchSummary = useCallback(async () => {
    if (!startDate || !endDate) return;
    setLoading(true);
    setError("");
    try {
      const data = await financialSummaryApi.getSummary({
        start_date: startDate,
        end_date: endDate,
      });
      setSummary(data);
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Error al cargar el resumen financiero"
      );
    } finally {
      setLoading(false);
    }
  }, [startDate, endDate]);

  useEffect(() => {
    fetchSummary();
  }, [fetchSummary]);

  const netIncome = summary ? parseFloat(summary.net_income) : 0;
  const netIncomeColor =
    netIncome >= 0 ? "var(--color-success)" : "var(--color-danger)";

  return (
    <div
      style={{
        minHeight: "100vh",
        backgroundColor: "var(--color-bg)",
        padding: "2rem 1rem",
      }}
    >
      <div style={{ maxWidth: "72rem", margin: "0 auto" }}>
        {/* Header */}
        <div
          style={{
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
            flexWrap: "wrap",
            gap: "1rem",
            marginBottom: "2rem",
          }}
        >
          <div>
            <h1
              style={{
                fontSize: "1.75rem",
                fontWeight: 700,
                color: "var(--color-text-primary)",
                margin: 0,
              }}
            >
              Resumen Financiero
            </h1>
            <p
              style={{
                color: "var(--color-text-secondary)",
                fontSize: "0.9rem",
                marginTop: "0.25rem",
              }}
            >
              Vista consolidada de ingresos, gastos y compras
            </p>
          </div>

          {/* Date filter + action */}
          <div
            style={{
              display: "flex",
              alignItems: "center",
              gap: "0.75rem",
              flexWrap: "wrap",
            }}
          >
            <div style={{ display: "flex", alignItems: "center", gap: "0.4rem" }}>
              <label
                style={{
                  fontSize: "0.8rem",
                  color: "var(--color-text-secondary)",
                }}
              >
                Desde
              </label>
              <input
                type="date"
                value={startDate}
                onChange={(e) => setStartDate(e.target.value)}
                style={{
                  padding: "0.4rem 0.6rem",
                  backgroundColor: "var(--color-surface)",
                  border: "1px solid var(--color-border)",
                  borderRadius: "var(--radius-md)",
                  color: "var(--color-text-primary)",
                  fontSize: "0.875rem",
                  cursor: "pointer",
                }}
              />
            </div>
            <div style={{ display: "flex", alignItems: "center", gap: "0.4rem" }}>
              <label
                style={{
                  fontSize: "0.8rem",
                  color: "var(--color-text-secondary)",
                }}
              >
                Hasta
              </label>
              <input
                type="date"
                value={endDate}
                onChange={(e) => setEndDate(e.target.value)}
                style={{
                  padding: "0.4rem 0.6rem",
                  backgroundColor: "var(--color-surface)",
                  border: "1px solid var(--color-border)",
                  borderRadius: "var(--radius-md)",
                  color: "var(--color-text-primary)",
                  fontSize: "0.875rem",
                  cursor: "pointer",
                }}
              />
            </div>
            <button
              onClick={fetchSummary}
              disabled={loading}
              style={{
                padding: "0.4rem 1rem",
                backgroundColor: loading
                  ? "var(--color-surface-hover)"
                  : "var(--color-primary)",
                color: loading ? "var(--color-text-muted)" : "white",
                border: "none",
                borderRadius: "var(--radius-md)",
                cursor: loading ? "not-allowed" : "pointer",
                fontSize: "0.875rem",
                fontWeight: 500,
                transition: "background-color var(--transition-normal)",
              }}
            >
              {loading ? "Cargando..." : "Actualizar"}
            </button>
          </div>
        </div>

        {/* Error state */}
        {error && (
          <div
            style={{
              padding: "0.875rem 1rem",
              backgroundColor: "var(--color-danger-light)",
              border: "1px solid var(--color-danger)",
              borderRadius: "var(--radius-md)",
              color: "var(--color-danger)",
              marginBottom: "1.5rem",
              fontSize: "0.9rem",
            }}
          >
            {error}
          </div>
        )}

        {/* Loading skeleton */}
        {loading && !summary && (
          <div
            style={{
              display: "grid",
              gridTemplateColumns: "repeat(auto-fit, minmax(200px, 1fr))",
              gap: "1rem",
            }}
          >
            {[1, 2, 3, 4].map((i) => (
              <div
                key={i}
                style={{
                  height: "110px",
                  backgroundColor: "var(--color-surface)",
                  border: "1px solid var(--color-border)",
                  borderRadius: "var(--radius-md)",
                  animation: "pulse 1.5s ease-in-out infinite",
                }}
              />
            ))}
          </div>
        )}

        {/* Main content */}
        {summary && (
          <>
            {/* KPI cards — top row */}
            <div
              style={{
                display: "grid",
                gridTemplateColumns: "repeat(auto-fit, minmax(200px, 1fr))",
                gap: "1rem",
                marginBottom: "1rem",
              }}
            >
              <SummaryKpiCard
                label="Ingresos"
                amount={summary.revenue.total_amount}
                count={summary.revenue.count}
                accentColor="var(--color-primary)"
              />
              <SummaryKpiCard
                label="Gastos"
                amount={summary.expenses.total_amount}
                count={summary.expenses.count}
                accentColor="var(--color-warning)"
              />
              <SummaryKpiCard
                label="Compras"
                amount={summary.purchases.total_amount}
                count={summary.purchases.count}
                accentColor="var(--color-text-secondary)"
              />
            </div>

            {/* Net income — full-width accent card */}
            <div
              style={{
                backgroundColor: "var(--color-surface)",
                border: `1px solid ${netIncomeColor}`,
                borderRadius: "var(--radius-md)",
                padding: "1.25rem 1.5rem",
                boxShadow: "var(--shadow-md)",
                marginBottom: "1rem",
                display: "flex",
                alignItems: "center",
                justifyContent: "space-between",
                gap: "1rem",
                flexWrap: "wrap",
              }}
            >
              <div>
                <p
                  style={{
                    fontSize: "0.75rem",
                    fontWeight: 600,
                    textTransform: "uppercase",
                    letterSpacing: "0.05em",
                    color: "var(--color-text-secondary)",
                    marginBottom: "0.4rem",
                  }}
                >
                  Utilidad Neta
                </p>
                <p
                  style={{
                    fontSize: "2rem",
                    fontWeight: 700,
                    color: netIncomeColor,
                    margin: 0,
                    lineHeight: 1.1,
                  }}
                >
                  {new Intl.NumberFormat("es-CO", {
                    style: "currency",
                    currency: "COP",
                    minimumFractionDigits: 0,
                    maximumFractionDigits: 0,
                  }).format(netIncome)}
                </p>
              </div>
              <p
                style={{
                  fontSize: "0.8rem",
                  color: "var(--color-text-muted)",
                  margin: 0,
                }}
              >
                Ingresos − Gastos − Compras
              </p>
            </div>

            {/* Expense category breakdown */}
            <ExpenseCategoryBreakdown
              categories={summary.expenses.by_category}
              totalAmount={summary.expenses.total_amount}
              totalCount={summary.expenses.count}
            />
          </>
        )}

        {/* Empty state — no data */}
        {!loading && !error && summary && summary.revenue.count === 0 &&
          summary.expenses.count === 0 && summary.purchases.count === 0 && (
          <div
            style={{
              textAlign: "center",
              padding: "3rem 1rem",
              color: "var(--color-text-muted)",
              fontSize: "0.95rem",
            }}
          >
            No hay datos financieros para el período seleccionado.
          </div>
        )}
      </div>
    </div>
  );
}
