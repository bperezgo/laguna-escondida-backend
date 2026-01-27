"use client";

import { useState, useEffect } from "react";
import type { Expense } from "@/types/expense";
import { expensesApi } from "@/lib/api/expenses";

interface ExpenseDetailProps {
  expenseId: string;
  onClose: () => void;
}

export default function ExpenseDetail({
  expenseId,
  onClose,
}: ExpenseDetailProps) {
  const [expense, setExpense] = useState<Expense | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    loadExpense();
  }, [expenseId]);

  const loadExpense = async () => {
    try {
      setLoading(true);
      setError("");
      const data = await expensesApi.getById(expenseId);
      setExpense(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Error al cargar el gasto");
    } finally {
      setLoading(false);
    }
  };

  const formatCurrency = (amount: string) => {
    return new Intl.NumberFormat("es-CO", {
      style: "currency",
      currency: "COP",
      minimumFractionDigits: 0,
      maximumFractionDigits: 0,
    }).format(parseFloat(amount));
  };

  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleDateString("es-CO", {
      year: "numeric",
      month: "long",
      day: "numeric",
    });
  };

  const formatDateTime = (dateString: string) => {
    return new Date(dateString).toLocaleString("es-CO", {
      year: "numeric",
      month: "long",
      day: "numeric",
      hour: "2-digit",
      minute: "2-digit",
    });
  };

  const getCategoryColor = (code?: string) => {
    const colors: Record<string, string> = {
      indirect_cost: "#6366f1",
      expense: "#8b5cf6",
      investment: "#10b981",
      rent: "#f59e0b",
      service: "#3b82f6",
    };
    return colors[code || ""] || "#64748b";
  };

  return (
    <div
      style={{
        position: "fixed",
        top: 0,
        left: 0,
        right: 0,
        bottom: 0,
        backgroundColor: "var(--color-overlay)",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        zIndex: 1000,
        padding: "1rem",
      }}
    >
      <div
        style={{
          backgroundColor: "var(--color-surface)",
          borderRadius: "var(--radius-md)",
          padding: "2rem",
          maxWidth: "600px",
          width: "100%",
          maxHeight: "90vh",
          overflowY: "auto",
          boxShadow: "var(--shadow-xl)",
          border: "1px solid var(--color-border)",
        }}
      >
        {/* Header */}
        <div
          style={{
            display: "flex",
            justifyContent: "space-between",
            alignItems: "center",
            marginBottom: "1.5rem",
          }}
        >
          <h2
            style={{
              margin: 0,
              fontSize: "1.5rem",
              fontWeight: "bold",
              color: "var(--color-text-primary)",
            }}
          >
            Detalle del Gasto
          </h2>
          <button
            onClick={onClose}
            style={{
              padding: "0.5rem",
              backgroundColor: "transparent",
              border: "none",
              cursor: "pointer",
              color: "var(--color-text-secondary)",
              fontSize: "1.5rem",
              lineHeight: 1,
            }}
          >
            ×
          </button>
        </div>

        {/* Error */}
        {error && (
          <div
            style={{
              padding: "1rem",
              backgroundColor: "var(--color-danger-light)",
              color: "var(--color-danger)",
              border: "1px solid var(--color-danger)",
              borderRadius: "var(--radius-sm)",
              marginBottom: "1rem",
            }}
          >
            {error}
          </div>
        )}

        {/* Loading */}
        {loading && (
          <div style={{ textAlign: "center", padding: "2rem" }}>
            <p style={{ color: "var(--color-text-secondary)" }}>
              Cargando detalles...
            </p>
          </div>
        )}

        {/* Expense Details */}
        {expense && !loading && (
          <>
            {/* Category Badge and Amount */}
            <div
              style={{
                display: "flex",
                justifyContent: "space-between",
                alignItems: "flex-start",
                marginBottom: "1.5rem",
                padding: "1.5rem",
                backgroundColor: "var(--color-bg)",
                borderRadius: "var(--radius-sm)",
                border: "1px solid var(--color-border)",
              }}
            >
              <div>
                <span
                  style={{
                    display: "inline-block",
                    padding: "0.5rem 1rem",
                    backgroundColor: getCategoryColor(expense.category_code),
                    color: "white",
                    borderRadius: "var(--radius-sm)",
                    fontSize: "0.875rem",
                    fontWeight: "600",
                    textTransform: "uppercase",
                    marginBottom: "0.5rem",
                  }}
                >
                  {expense.category_name || expense.category_code || "Sin categoría"}
                </span>
                {expense.reference && (
                  <p
                    style={{
                      margin: "0.5rem 0 0 0",
                      color: "var(--color-text-secondary)",
                      fontSize: "0.875rem",
                      fontFamily: "monospace",
                    }}
                  >
                    Ref: {expense.reference}
                  </p>
                )}
              </div>
              <div style={{ textAlign: "right" }}>
                <p
                  style={{
                    margin: "0 0 0.25rem 0",
                    color: "var(--color-text-muted)",
                    fontSize: "0.75rem",
                  }}
                >
                  Monto
                </p>
                <p
                  style={{
                    margin: 0,
                    fontSize: "2rem",
                    fontWeight: "bold",
                    color: "var(--color-danger)",
                  }}
                >
                  {formatCurrency(expense.amount)}
                </p>
              </div>
            </div>

            {/* Description */}
            <div style={{ marginBottom: "1.5rem" }}>
              <p
                style={{
                  margin: "0 0 0.5rem 0",
                  color: "var(--color-text-muted)",
                  fontSize: "0.875rem",
                  fontWeight: "600",
                }}
              >
                Descripción
              </p>
              <p
                style={{
                  margin: 0,
                  fontSize: "1.1rem",
                  color: "var(--color-text-primary)",
                }}
              >
                {expense.description}
              </p>
            </div>

            {/* Info Grid */}
            <div
              style={{
                display: "grid",
                gridTemplateColumns: "repeat(auto-fit, minmax(200px, 1fr))",
                gap: "1.5rem",
                marginBottom: "1.5rem",
                padding: "1.5rem",
                backgroundColor: "var(--color-bg)",
                borderRadius: "var(--radius-sm)",
                border: "1px solid var(--color-border)",
              }}
            >
              <div>
                <p
                  style={{
                    margin: "0 0 0.25rem 0",
                    color: "var(--color-text-muted)",
                    fontSize: "0.875rem",
                  }}
                >
                  Fecha del Gasto
                </p>
                <p
                  style={{
                    margin: 0,
                    fontSize: "1rem",
                    fontWeight: "600",
                    color: "var(--color-text-primary)",
                  }}
                >
                  {formatDate(expense.expense_date)}
                </p>
              </div>

              {expense.supplier_name && (
                <div>
                  <p
                    style={{
                      margin: "0 0 0.25rem 0",
                      color: "var(--color-text-muted)",
                      fontSize: "0.875rem",
                    }}
                  >
                    Proveedor
                  </p>
                  <p
                    style={{
                      margin: 0,
                      fontSize: "1rem",
                      fontWeight: "600",
                      color: "var(--color-text-primary)",
                    }}
                  >
                    {expense.supplier_name}
                  </p>
                </div>
              )}

              <div>
                <p
                  style={{
                    margin: "0 0 0.25rem 0",
                    color: "var(--color-text-muted)",
                    fontSize: "0.875rem",
                  }}
                >
                  Fecha de Registro
                </p>
                <p
                  style={{
                    margin: 0,
                    fontSize: "1rem",
                    fontWeight: "600",
                    color: "var(--color-text-primary)",
                  }}
                >
                  {formatDateTime(expense.created_at)}
                </p>
              </div>
            </div>

            {/* Notes */}
            {expense.notes && (
              <div
                style={{
                  marginBottom: "1.5rem",
                  padding: "1rem",
                  backgroundColor: "var(--color-bg)",
                  borderRadius: "var(--radius-sm)",
                  border: "1px solid var(--color-border)",
                }}
              >
                <p
                  style={{
                    margin: "0 0 0.5rem 0",
                    color: "var(--color-text-muted)",
                    fontSize: "0.875rem",
                    fontWeight: "600",
                  }}
                >
                  Notas
                </p>
                <p
                  style={{
                    margin: 0,
                    color: "var(--color-text-secondary)",
                    whiteSpace: "pre-wrap",
                  }}
                >
                  {expense.notes}
                </p>
              </div>
            )}

            {/* Documents */}
            {(expense.pdf_storage_path || expense.xml_storage_path) && (
              <div
                style={{
                  marginBottom: "1.5rem",
                  padding: "1rem",
                  backgroundColor: "var(--color-bg)",
                  borderRadius: "var(--radius-sm)",
                  border: "1px solid var(--color-border)",
                }}
              >
                <p
                  style={{
                    margin: "0 0 0.5rem 0",
                    color: "var(--color-text-muted)",
                    fontSize: "0.875rem",
                    fontWeight: "600",
                  }}
                >
                  Documentos Adjuntos
                </p>
                <div style={{ display: "flex", gap: "0.5rem", flexWrap: "wrap" }}>
                  {expense.pdf_storage_path && (
                    <span
                      style={{
                        padding: "0.5rem 1rem",
                        backgroundColor: "var(--color-primary-light)",
                        color: "var(--color-primary)",
                        borderRadius: "var(--radius-sm)",
                        fontSize: "0.875rem",
                      }}
                    >
                      PDF
                    </span>
                  )}
                  {expense.xml_storage_path && (
                    <span
                      style={{
                        padding: "0.5rem 1rem",
                        backgroundColor: "var(--color-success-light)",
                        color: "var(--color-success)",
                        borderRadius: "var(--radius-sm)",
                        fontSize: "0.875rem",
                      }}
                    >
                      XML
                    </span>
                  )}
                </div>
              </div>
            )}
          </>
        )}

        {/* Close button */}
        <div
          style={{
            marginTop: "2rem",
            display: "flex",
            justifyContent: "flex-end",
          }}
        >
          <button
            onClick={onClose}
            style={{
              padding: "0.75rem 1.5rem",
              backgroundColor: "var(--color-surface-hover)",
              color: "var(--color-text-primary)",
              border: "1px solid var(--color-border)",
              borderRadius: "var(--radius-sm)",
              cursor: "pointer",
              fontSize: "1rem",
              fontWeight: "500",
            }}
          >
            Cerrar
          </button>
        </div>
      </div>
    </div>
  );
}
