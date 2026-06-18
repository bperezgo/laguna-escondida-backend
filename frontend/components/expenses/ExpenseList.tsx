"use client";

import { useState, useMemo } from "react";
import type { Expense } from "@/types/expense";
import { Badge, Button, Input, Table } from "@/components/ui";
import { PermissionGate } from "@/components/permissions";
import { PERMISSIONS } from "@/lib/permissions";

interface ExpenseListProps {
  expenses: Expense[];
  onViewDetail: (expense: Expense) => void;
  onEdit: (expense: Expense) => void;
  onDelete: (expense: Expense) => void;
  isLoading?: boolean;
}

export default function ExpenseList({
  expenses,
  onViewDetail,
  onEdit,
  onDelete,
  isLoading = false,
}: ExpenseListProps) {
  const [searchTerm, setSearchTerm] = useState("");

  const filteredExpenses = useMemo(() => {
    if (!searchTerm) return expenses;
    return expenses.filter(
      (expense) =>
        expense.description?.toLowerCase().includes(searchTerm.toLowerCase()) ||
        expense.reference?.toLowerCase().includes(searchTerm.toLowerCase()) ||
        expense.notes?.toLowerCase().includes(searchTerm.toLowerCase()) ||
        expense.supplier_name?.toLowerCase().includes(searchTerm.toLowerCase())
    );
  }, [expenses, searchTerm]);

  const totalAmount = useMemo(() => {
    return filteredExpenses.reduce(
      (sum, expense) => sum + parseFloat(expense.amount),
      0
    );
  }, [filteredExpenses]);

  const formatCurrency = (amount: number) => {
    return new Intl.NumberFormat("es-CO", {
      style: "currency",
      currency: "COP",
      minimumFractionDigits: 0,
      maximumFractionDigits: 0,
    }).format(amount);
  };

  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleDateString("es-CO", {
      year: "numeric",
      month: "short",
      day: "numeric",
    });
  };

  if (isLoading) {
    return (
      <div style={{ textAlign: "center", padding: "3rem" }}>
        <p style={{ fontSize: "1.1rem", color: "var(--color-text-secondary)" }}>
          Cargando gastos...
        </p>
      </div>
    );
  }

  if (expenses.length === 0) {
    return (
      <div style={{ textAlign: "center", padding: "3rem" }}>
        <p style={{ fontSize: "1.1rem", color: "var(--color-text-secondary)" }}>
          No se encontraron gastos para el período seleccionado.
        </p>
      </div>
    );
  }

  return (
    <div>
      {/* Search */}
      <div style={{ marginBottom: "1.5rem" }}>
        <Input
          type="text"
          value={searchTerm}
          onChange={(e) => setSearchTerm(e.target.value)}
          placeholder="Buscar por descripción, referencia, proveedor, notas..."
        />
      </div>

      {/* Results count and total */}
      <div
        style={{
          marginBottom: "1rem",
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
          flexWrap: "wrap",
          gap: "1rem",
        }}
      >
        <span
          style={{
            color: "var(--color-text-secondary)",
            fontSize: "0.9rem",
          }}
        >
          Mostrando {filteredExpenses.length} de {expenses.length} gasto
          {expenses.length !== 1 ? "s" : ""}
        </span>
        <span
          style={{
            fontSize: "1.1rem",
            fontWeight: "bold",
            color: "var(--color-danger)",
          }}
        >
          Total: {formatCurrency(totalAmount)}
        </span>
      </div>

      {/* Expense Table */}
      {filteredExpenses.length === 0 ? (
        <div style={{ textAlign: "center", padding: "3rem" }}>
          <p style={{ fontSize: "1.1rem", color: "var(--color-text-secondary)" }}>
            No hay gastos que coincidan con tu búsqueda.
          </p>
        </div>
      ) : (
        <Table>
          <thead>
            <tr>
              <th>Categoría</th>
              <th>Descripción</th>
              <th>Fecha</th>
              <th>Proveedor</th>
              <th data-numeric>Monto</th>
              <th style={{ textAlign: "right" }}>Acciones</th>
            </tr>
          </thead>
          <tbody>
            {filteredExpenses.map((expense) => (
              <tr key={expense.id}>
                <td>
                  <Badge tone="info" dot={false}>
                    {expense.category_name ||
                      expense.category_code ||
                      "Sin categoría"}
                  </Badge>
                </td>
                <td>
                  <span style={{ color: "var(--color-text-primary)" }}>
                    {expense.description}
                  </span>
                  {expense.reference && (
                    <div
                      style={{
                        fontFamily: "monospace",
                        fontSize: "0.75rem",
                        color: "var(--color-text-muted)",
                      }}
                    >
                      Ref: {expense.reference}
                    </div>
                  )}
                </td>
                <td>{formatDate(expense.expense_date)}</td>
                <td>{expense.supplier_name || "—"}</td>
                <td
                  data-numeric
                  style={{ color: "var(--color-danger)", fontWeight: 600 }}
                >
                  {formatCurrency(parseFloat(expense.amount))}
                </td>
                <td style={{ textAlign: "right" }}>
                  <div
                    style={{
                      display: "flex",
                      gap: "0.5rem",
                      justifyContent: "flex-end",
                    }}
                  >
                    <Button
                      variant="secondary"
                      size="sm"
                      onClick={() => onViewDetail(expense)}
                    >
                      Ver
                    </Button>
                    <PermissionGate permission={PERMISSIONS.EXPENSES_UPDATE}>
                      <Button
                        variant="primary"
                        size="sm"
                        onClick={() => onEdit(expense)}
                      >
                        Editar
                      </Button>
                    </PermissionGate>
                    <PermissionGate permission={PERMISSIONS.EXPENSES_DELETE}>
                      <Button
                        variant="danger"
                        size="sm"
                        onClick={() => onDelete(expense)}
                      >
                        Eliminar
                      </Button>
                    </PermissionGate>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </Table>
      )}
    </div>
  );
}
