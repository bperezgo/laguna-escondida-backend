"use client";

import { useState, useEffect, useCallback, useMemo } from "react";
import { dailyCloseApi } from "@/lib/api/dailyClose";
import type { DailyCloseReport } from "@/types/dailyClose";
import { Card, CardBody, Input, Button, StatCard } from "@/components/ui";

// One row per payment kind — cards are NOT bucketed (user decision). Canonical order +
// Spanish labels. Any kind the backend returns that isn't here is appended and labeled
// by its raw code (empty string → "Sin clasificar", only pre-migration rows).
const PAYMENT_LABELS: Record<string, string> = {
  cash: "Efectivo",
  credit_card: "Tarjeta de Crédito",
  debit_card: "Tarjeta de Débito",
  transfer_debit_bank: "Transferencia Débito Bancaria",
  transfer_credit_bank: "Transferencia Crédito Bancaria",
  transfer_debit_interbank: "Transferencia Débito Interbancaria",
};
const CANONICAL_KINDS = Object.keys(PAYMENT_LABELS);

const labelFor = (code: string) =>
  PAYMENT_LABELS[code] ?? (code === "" ? "Sin clasificar" : code);

// America/Bogota is UTC-5 year-round (no DST). en-CA formats as YYYY-MM-DD.
const todayBogota = () =>
  new Intl.DateTimeFormat("en-CA", { timeZone: "America/Bogota" }).format(new Date());

const shiftDate = (dateStr: string, days: number) => {
  const [y, m, d] = dateStr.split("-").map(Number);
  const dt = new Date(Date.UTC(y, m - 1, d));
  dt.setUTCDate(dt.getUTCDate() + days);
  return dt.toISOString().split("T")[0];
};

const formatCurrency = (value: string | number) => {
  const num = typeof value === "number" ? value : parseFloat(value);
  if (isNaN(num)) return "$0";
  return new Intl.NumberFormat("es-CO", {
    style: "currency",
    currency: "COP",
    minimumFractionDigits: 0,
    maximumFractionDigits: 0,
  }).format(num);
};

interface Row {
  code: string;
  label: string;
  expected: number; // gross collected per system
  count: number;
}

export default function DailyClosePageClient() {
  const [date, setDate] = useState<string>(todayBogota());
  const [report, setReport] = useState<DailyCloseReport | null>(null);
  const [loading, setLoading] = useState<boolean>(false);
  const [error, setError] = useState<string>("");
  // Physically-counted amounts, keyed by payment code. Ephemeral (never persisted) — the
  // report endpoint is read-only. Cleared whenever the day changes (a new day = new counts).
  const [counted, setCounted] = useState<Record<string, string>>({});

  const fetchReport = useCallback(async () => {
    if (!date) return;
    setLoading(true);
    setError("");
    try {
      const data = await dailyCloseApi.getDailyClose(date);
      setReport(data);
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Error al cargar el cierre de caja"
      );
    } finally {
      setLoading(false);
    }
  }, [date]);

  useEffect(() => {
    fetchReport();
  }, [fetchReport]);

  const goToDay = (days: number) => {
    setCounted({});
    setDate((d) => shiftDate(d, days));
  };

  const isToday = date === todayBogota();

  // Build the reconciliation rows: canonical kinds first (always shown, even with 0 sales),
  // then any extra kinds the backend returned that aren't canonical.
  const rows = useMemo<Row[]>(() => {
    const byKind = new Map(
      (report?.by_payment_method ?? []).map((r) => [r.payment_method, r])
    );
    const canonical: Row[] = CANONICAL_KINDS.map((code) => {
      const r = byKind.get(code);
      return {
        code,
        label: labelFor(code),
        expected: r ? parseFloat(r.collected || "0") : 0,
        count: r ? r.count : 0,
      };
    });
    const extras: Row[] = (report?.by_payment_method ?? [])
      .filter((r) => !CANONICAL_KINDS.includes(r.payment_method))
      .map((r) => ({
        code: r.payment_method,
        label: labelFor(r.payment_method),
        expected: parseFloat(r.collected || "0"),
        count: r.count,
      }));
    return [...canonical, ...extras];
  }, [report]);

  const totalExpected = report ? parseFloat(report.total_collected || "0") : 0;
  const totalCounted = rows.reduce((sum, row) => {
    const c = counted[row.code];
    return sum + (c && c.trim() !== "" ? parseFloat(c) || 0 : 0);
  }, 0);
  const totalDiff = totalCounted - totalExpected;

  return (
    <div
      style={{
        minHeight: "100vh",
        backgroundColor: "var(--color-bg)",
        padding: "2rem 1rem",
      }}
    >
      {/* Print rules: hide interactive chrome, flatten backgrounds for a clean count sheet. */}
      <style>{`
        @media print {
          .no-print { display: none !important; }
          input { border: none !important; padding: 0 !important; }
        }
      `}</style>

      <div style={{ maxWidth: "60rem", margin: "0 auto" }}>
        {/* Header + day selector */}
        <div
          style={{
            display: "flex",
            alignItems: "flex-end",
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
              Cierre de Caja
            </h1>
            <p
              style={{
                color: "var(--color-text-secondary)",
                fontSize: "0.9rem",
                marginTop: "0.25rem",
              }}
            >
              Cuadre del día — {new Intl.DateTimeFormat("es-CO", {
                weekday: "long",
                day: "numeric",
                month: "long",
                year: "numeric",
                timeZone: "UTC",
              }).format(new Date(`${date}T00:00:00Z`))}
            </p>
          </div>

          <div
            className="no-print"
            style={{
              display: "flex",
              alignItems: "flex-end",
              gap: "0.5rem",
              flexWrap: "wrap",
            }}
          >
            <Button variant="secondary" onClick={() => goToDay(-1)} disabled={loading}>
              ‹ Día anterior
            </Button>
            <div style={{ width: "170px" }}>
              <Input
                label="Día"
                type="date"
                value={date}
                onChange={(e) => {
                  setCounted({});
                  setDate(e.target.value);
                }}
              />
            </div>
            <Button
              variant="secondary"
              onClick={() => goToDay(1)}
              disabled={loading || isToday}
            >
              Día siguiente ›
            </Button>
            <Button variant="primary" onClick={() => window.print()} disabled={!report}>
              Imprimir
            </Button>
          </div>
        </div>

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

        {loading && !report && (
          <div
            style={{
              display: "grid",
              gridTemplateColumns: "repeat(auto-fit, minmax(180px, 1fr))",
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

        {report && (
          <>
            {/* ① Ventas — KPI cards */}
            <div
              style={{
                display: "grid",
                gridTemplateColumns: "repeat(auto-fit, minmax(180px, 1fr))",
                gap: "1rem",
                marginBottom: "1.5rem",
              }}
            >
              <StatCard
                label="Ventas"
                value={report.total_orders}
                delta="órdenes cobradas"
              />
              <StatCard
                label="Ingreso total"
                value={formatCurrency(report.total_collected)}
                delta="bruto cobrado"
              />
              <StatCard
                label="Propinas"
                value={formatCurrency(report.total_tip)}
              />
              <StatCard
                label="Descuentos"
                value={formatCurrency(report.total_discount)}
              />
            </div>

            {/* ② Cuadre de dinero — one row per payment kind */}
            <Card>
              <CardBody>
                <h2
                  style={{
                    fontSize: "1.1rem",
                    fontWeight: 700,
                    color: "var(--color-text-primary)",
                    margin: "0 0 0.25rem",
                  }}
                >
                  Cuadre de dinero
                </h2>
                <p
                  style={{
                    color: "var(--color-text-secondary)",
                    fontSize: "0.85rem",
                    margin: "0 0 1rem",
                  }}
                >
                  Escribe lo contado físicamente (efectivo) y lo reportado por datáfono /
                  banco. La diferencia debe quedar en $0.
                </p>

                <div style={{ overflowX: "auto" }}>
                  <table
                    style={{
                      width: "100%",
                      borderCollapse: "collapse",
                      fontSize: "0.9rem",
                    }}
                  >
                    <thead>
                      <tr
                        style={{
                          textAlign: "left",
                          color: "var(--color-text-secondary)",
                          fontSize: "0.75rem",
                          textTransform: "uppercase",
                          letterSpacing: "0.04em",
                        }}
                      >
                        <th style={{ padding: "0.5rem 0.75rem 0.5rem 0" }}>Método</th>
                        <th style={{ padding: "0.5rem 0.75rem", textAlign: "center" }}>
                          Ventas
                        </th>
                        <th style={{ padding: "0.5rem 0.75rem", textAlign: "right" }}>
                          Esperado
                        </th>
                        <th style={{ padding: "0.5rem 0.75rem", textAlign: "right" }}>
                          Contado
                        </th>
                        <th style={{ padding: "0.5rem 0 0.5rem 0.75rem", textAlign: "right" }}>
                          Diferencia
                        </th>
                      </tr>
                    </thead>
                    <tbody>
                      {rows.map((row) => {
                        const countedStr = counted[row.code] ?? "";
                        const countedNum =
                          countedStr.trim() === "" ? null : parseFloat(countedStr);
                        const diff =
                          countedNum === null ? null : countedNum - row.expected;
                        const diffColor =
                          diff === null
                            ? "var(--color-text-muted)"
                            : diff === 0
                            ? "var(--color-success)"
                            : "var(--color-danger)";
                        return (
                          <tr
                            key={row.code}
                            style={{ borderTop: "1px solid var(--color-border)" }}
                          >
                            <td
                              style={{
                                padding: "0.6rem 0.75rem 0.6rem 0",
                                color: "var(--color-text-primary)",
                                fontWeight: 500,
                              }}
                            >
                              {row.label}
                            </td>
                            <td
                              style={{
                                padding: "0.6rem 0.75rem",
                                textAlign: "center",
                                color: "var(--color-text-secondary)",
                              }}
                            >
                              {row.count}
                            </td>
                            <td
                              style={{
                                padding: "0.6rem 0.75rem",
                                textAlign: "right",
                                color: "var(--color-text-primary)",
                                fontVariantNumeric: "tabular-nums",
                              }}
                            >
                              {formatCurrency(row.expected)}
                            </td>
                            <td style={{ padding: "0.4rem 0.75rem", textAlign: "right" }}>
                              <input
                                type="number"
                                inputMode="numeric"
                                placeholder="0"
                                value={countedStr}
                                onChange={(e) =>
                                  setCounted((prev) => ({
                                    ...prev,
                                    [row.code]: e.target.value,
                                  }))
                                }
                                style={{
                                  width: "120px",
                                  textAlign: "right",
                                  padding: "0.4rem 0.5rem",
                                  border: "1px solid var(--color-border)",
                                  borderRadius: "var(--radius-sm)",
                                  backgroundColor: "var(--color-surface)",
                                  color: "var(--color-text-primary)",
                                  fontVariantNumeric: "tabular-nums",
                                }}
                              />
                            </td>
                            <td
                              style={{
                                padding: "0.6rem 0 0.6rem 0.75rem",
                                textAlign: "right",
                                color: diffColor,
                                fontWeight: 600,
                                fontVariantNumeric: "tabular-nums",
                                whiteSpace: "nowrap",
                              }}
                            >
                              {diff === null
                                ? "—"
                                : diff === 0
                                ? `${formatCurrency(0)} ✓`
                                : `${diff > 0 ? "+" : ""}${formatCurrency(diff)} ✗`}
                            </td>
                          </tr>
                        );
                      })}
                    </tbody>
                    <tfoot>
                      <tr
                        style={{
                          borderTop: "2px solid var(--color-border)",
                          fontWeight: 700,
                        }}
                      >
                        <td style={{ padding: "0.75rem 0.75rem 0.75rem 0" }}>TOTAL</td>
                        <td
                          style={{
                            padding: "0.75rem",
                            textAlign: "center",
                            color: "var(--color-text-secondary)",
                          }}
                        >
                          {report.total_orders}
                        </td>
                        <td
                          style={{
                            padding: "0.75rem",
                            textAlign: "right",
                            fontVariantNumeric: "tabular-nums",
                          }}
                        >
                          {formatCurrency(totalExpected)}
                        </td>
                        <td
                          style={{
                            padding: "0.75rem",
                            textAlign: "right",
                            fontVariantNumeric: "tabular-nums",
                          }}
                        >
                          {formatCurrency(totalCounted)}
                        </td>
                        <td
                          style={{
                            padding: "0.75rem 0 0.75rem 0.75rem",
                            textAlign: "right",
                            color:
                              totalDiff === 0
                                ? "var(--color-success)"
                                : "var(--color-danger)",
                            fontVariantNumeric: "tabular-nums",
                            whiteSpace: "nowrap",
                          }}
                        >
                          {`${totalDiff > 0 ? "+" : ""}${formatCurrency(totalDiff)}`}
                        </td>
                      </tr>
                    </tfoot>
                  </table>
                </div>
              </CardBody>
            </Card>

            {report.total_orders === 0 && (
              <div
                style={{
                  textAlign: "center",
                  padding: "2rem 1rem",
                  color: "var(--color-text-muted)",
                  fontSize: "0.95rem",
                }}
              >
                No hay ventas registradas para este día.
              </div>
            )}
          </>
        )}
      </div>
    </div>
  );
}
