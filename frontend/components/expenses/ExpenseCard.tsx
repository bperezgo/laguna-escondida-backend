"use client";

import type { Expense } from "@/types/expense";

interface ExpenseCardProps {
  expense: Expense;
  onViewDetail: (expense: Expense) => void;
  onEdit: (expense: Expense) => void;
  onDelete: (expense: Expense) => void;
}

export default function ExpenseCard({
  expense,
  onViewDetail,
  onEdit,
  onDelete,
}: ExpenseCardProps) {
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
      month: "short",
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
        border: "1px solid var(--color-border)",
        borderRadius: "var(--radius-md)",
        padding: "1.5rem",
        marginBottom: "1rem",
        backgroundColor: "var(--color-surface)",
        boxShadow: "var(--shadow-sm)",
        transition: "box-shadow var(--transition-normal)",
      }}
      onMouseEnter={(e) => {
        e.currentTarget.style.boxShadow = "var(--shadow-md)";
        e.currentTarget.style.borderColor = "var(--color-primary)";
      }}
      onMouseLeave={(e) => {
        e.currentTarget.style.boxShadow = "var(--shadow-sm)";
        e.currentTarget.style.borderColor = "var(--color-border)";
      }}
    >
      <div
        style={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "flex-start",
        }}
      >
        <div style={{ flex: 1 }}>
          <div
            style={{
              display: "flex",
              alignItems: "center",
              gap: "0.75rem",
              marginBottom: "0.5rem",
              flexWrap: "wrap",
            }}
          >
            <span
              style={{
                padding: "0.25rem 0.75rem",
                backgroundColor: getCategoryColor(expense.category_code),
                color: "white",
                borderRadius: "var(--radius-sm)",
                fontSize: "0.75rem",
                fontWeight: "600",
                textTransform: "uppercase",
              }}
            >
              {expense.category_name || expense.category_code || "Sin categoría"}
            </span>
            {expense.reference && (
              <span
                style={{
                  padding: "0.25rem 0.5rem",
                  backgroundColor: "var(--color-bg)",
                  borderRadius: "var(--radius-sm)",
                  fontSize: "0.75rem",
                  fontFamily: "monospace",
                  color: "var(--color-text-secondary)",
                }}
              >
                Ref: {expense.reference}
              </span>
            )}
          </div>

          <h3
            style={{
              margin: "0 0 0.5rem 0",
              fontSize: "1.1rem",
              fontWeight: "600",
              color: "var(--color-text-primary)",
            }}
          >
            {expense.description}
          </h3>

          <p
            style={{
              margin: "0 0 0.5rem 0",
              color: "var(--color-text-secondary)",
              fontSize: "0.9rem",
            }}
          >
            <strong>Fecha:</strong> {formatDate(expense.expense_date)}
          </p>

          {expense.supplier_name && (
            <p
              style={{
                margin: "0 0 0.5rem 0",
                color: "var(--color-text-secondary)",
                fontSize: "0.875rem",
              }}
            >
              <strong>Proveedor:</strong> {expense.supplier_name}
            </p>
          )}

          {expense.notes && (
            <p
              style={{
                margin: 0,
                color: "var(--color-text-muted)",
                fontSize: "0.875rem",
                fontStyle: "italic",
              }}
            >
              {expense.notes}
            </p>
          )}
        </div>

        <div style={{ textAlign: "right", marginLeft: "1rem" }}>
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
              fontSize: "1.5rem",
              fontWeight: "bold",
              color: "var(--color-danger)",
            }}
          >
            {formatCurrency(expense.amount)}
          </p>
        </div>
      </div>

      <div
        style={{
          marginTop: "1rem",
          paddingTop: "1rem",
          borderTop: "1px solid var(--color-border)",
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
        }}
      >
        <p
          style={{
            margin: 0,
            color: "var(--color-text-muted)",
            fontSize: "0.75rem",
          }}
        >
          Registrado: {formatDateTime(expense.created_at)}
        </p>
        <div style={{ display: "flex", gap: "0.5rem" }}>
          <button
            onClick={() => onViewDetail(expense)}
            style={{
              padding: "0.5rem 1rem",
              backgroundColor: "transparent",
              color: "var(--color-primary)",
              border: "1px solid var(--color-primary)",
              borderRadius: "var(--radius-sm)",
              cursor: "pointer",
              fontSize: "0.875rem",
              fontWeight: "500",
              transition: "all var(--transition-normal)",
            }}
            onMouseEnter={(e) => {
              e.currentTarget.style.backgroundColor = "var(--color-primary)";
              e.currentTarget.style.color = "white";
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.backgroundColor = "transparent";
              e.currentTarget.style.color = "var(--color-primary)";
            }}
          >
            Ver
          </button>
          <button
            onClick={() => onEdit(expense)}
            style={{
              padding: "0.5rem 1rem",
              backgroundColor: "transparent",
              color: "var(--color-warning)",
              border: "1px solid var(--color-warning)",
              borderRadius: "var(--radius-sm)",
              cursor: "pointer",
              fontSize: "0.875rem",
              fontWeight: "500",
              transition: "all var(--transition-normal)",
            }}
            onMouseEnter={(e) => {
              e.currentTarget.style.backgroundColor = "var(--color-warning)";
              e.currentTarget.style.color = "white";
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.backgroundColor = "transparent";
              e.currentTarget.style.color = "var(--color-warning)";
            }}
          >
            Editar
          </button>
          <button
            onClick={() => onDelete(expense)}
            style={{
              padding: "0.5rem 1rem",
              backgroundColor: "transparent",
              color: "var(--color-danger)",
              border: "1px solid var(--color-danger)",
              borderRadius: "var(--radius-sm)",
              cursor: "pointer",
              fontSize: "0.875rem",
              fontWeight: "500",
              transition: "all var(--transition-normal)",
            }}
            onMouseEnter={(e) => {
              e.currentTarget.style.backgroundColor = "var(--color-danger)";
              e.currentTarget.style.color = "white";
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.backgroundColor = "transparent";
              e.currentTarget.style.color = "var(--color-danger)";
            }}
          >
            Eliminar
          </button>
        </div>
      </div>
    </div>
  );
}
