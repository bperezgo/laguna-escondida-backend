"use client";

import { useState, useMemo } from "react";
import type { Expense, ExpenseCategory } from "@/types/expense";
import type { Supplier } from "@/types/supplier";
import ExpenseCard from "./ExpenseCard";

interface ExpenseListProps {
  expenses: Expense[];
  categories: ExpenseCategory[];
  suppliers: Supplier[];
  onViewDetail: (expense: Expense) => void;
  onEdit: (expense: Expense) => void;
  onDelete: (expense: Expense) => void;
  isLoading?: boolean;
}

export default function ExpenseList({
  expenses,
  categories,
  suppliers,
  onViewDetail,
  onEdit,
  onDelete,
  isLoading = false,
}: ExpenseListProps) {
  const [filterCategory, setFilterCategory] = useState("");
  const [filterSupplier, setFilterSupplier] = useState("");
  const [searchTerm, setSearchTerm] = useState("");
  const [startDate, setStartDate] = useState("");
  const [endDate, setEndDate] = useState("");

  // Filter expenses
  const filteredExpenses = useMemo(() => {
    return expenses.filter((expense) => {
      const matchesCategory =
        !filterCategory || expense.category_id === filterCategory;
      const matchesSupplier =
        !filterSupplier || expense.supplier_id === filterSupplier;
      const matchesSearch =
        !searchTerm ||
        expense.description?.toLowerCase().includes(searchTerm.toLowerCase()) ||
        expense.reference?.toLowerCase().includes(searchTerm.toLowerCase()) ||
        expense.notes?.toLowerCase().includes(searchTerm.toLowerCase()) ||
        expense.supplier_name?.toLowerCase().includes(searchTerm.toLowerCase());

      // Date filters
      let matchesStartDate = true;
      let matchesEndDate = true;

      if (startDate) {
        const expenseDate = new Date(expense.expense_date);
        const filterStart = new Date(startDate);
        matchesStartDate = expenseDate >= filterStart;
      }

      if (endDate) {
        const expenseDate = new Date(expense.expense_date);
        const filterEnd = new Date(endDate);
        filterEnd.setHours(23, 59, 59, 999);
        matchesEndDate = expenseDate <= filterEnd;
      }

      return (
        matchesCategory &&
        matchesSupplier &&
        matchesSearch &&
        matchesStartDate &&
        matchesEndDate
      );
    });
  }, [expenses, filterCategory, filterSupplier, searchTerm, startDate, endDate]);

  // Calculate total
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

  const handleClearFilters = () => {
    setFilterCategory("");
    setFilterSupplier("");
    setSearchTerm("");
    setStartDate("");
    setEndDate("");
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
          No se encontraron gastos. ¡Registra tu primer gasto!
        </p>
      </div>
    );
  }

  return (
    <div>
      {/* Filters */}
      <div
        style={{
          marginBottom: "2rem",
          padding: "1.5rem",
          backgroundColor: "var(--color-surface)",
          borderRadius: "var(--radius-md)",
          border: "1px solid var(--color-border)",
        }}
      >
        <div
          style={{
            display: "grid",
            gridTemplateColumns: "repeat(auto-fit, minmax(180px, 1fr))",
            gap: "1rem",
            marginBottom: "1rem",
          }}
        >
          <div>
            <label
              style={{
                display: "block",
                marginBottom: "0.5rem",
                fontWeight: "500",
                fontSize: "0.875rem",
                color: "var(--color-text-primary)",
              }}
            >
              Buscar
            </label>
            <input
              type="text"
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
              placeholder="Descripción, referencia..."
              style={{
                width: "100%",
                padding: "0.75rem",
                border: "1px solid var(--color-border)",
                borderRadius: "var(--radius-sm)",
                fontSize: "1rem",
                boxSizing: "border-box",
                backgroundColor: "var(--color-bg)",
                color: "var(--color-text-primary)",
              }}
            />
          </div>
          <div>
            <label
              style={{
                display: "block",
                marginBottom: "0.5rem",
                fontWeight: "500",
                fontSize: "0.875rem",
                color: "var(--color-text-primary)",
              }}
            >
              Categoría
            </label>
            <select
              value={filterCategory}
              onChange={(e) => setFilterCategory(e.target.value)}
              style={{
                width: "100%",
                padding: "0.75rem",
                border: "1px solid var(--color-border)",
                borderRadius: "var(--radius-sm)",
                fontSize: "1rem",
                boxSizing: "border-box",
                backgroundColor: "var(--color-bg)",
                color: "var(--color-text-primary)",
              }}
            >
              <option value="">Todas las categorías</option>
              {categories.map((category) => (
                <option key={category.id} value={category.id}>
                  {category.name}
                </option>
              ))}
            </select>
          </div>
          <div>
            <label
              style={{
                display: "block",
                marginBottom: "0.5rem",
                fontWeight: "500",
                fontSize: "0.875rem",
                color: "var(--color-text-primary)",
              }}
            >
              Proveedor
            </label>
            <select
              value={filterSupplier}
              onChange={(e) => setFilterSupplier(e.target.value)}
              style={{
                width: "100%",
                padding: "0.75rem",
                border: "1px solid var(--color-border)",
                borderRadius: "var(--radius-sm)",
                fontSize: "1rem",
                boxSizing: "border-box",
                backgroundColor: "var(--color-bg)",
                color: "var(--color-text-primary)",
              }}
            >
              <option value="">Todos los proveedores</option>
              {suppliers.map((supplier) => (
                <option key={supplier.id} value={supplier.id}>
                  {supplier.name}
                </option>
              ))}
            </select>
          </div>
          <div>
            <label
              style={{
                display: "block",
                marginBottom: "0.5rem",
                fontWeight: "500",
                fontSize: "0.875rem",
                color: "var(--color-text-primary)",
              }}
            >
              Desde
            </label>
            <input
              type="date"
              value={startDate}
              onChange={(e) => setStartDate(e.target.value)}
              style={{
                width: "100%",
                padding: "0.75rem",
                border: "1px solid var(--color-border)",
                borderRadius: "var(--radius-sm)",
                fontSize: "1rem",
                boxSizing: "border-box",
                backgroundColor: "var(--color-bg)",
                color: "var(--color-text-primary)",
              }}
            />
          </div>
          <div>
            <label
              style={{
                display: "block",
                marginBottom: "0.5rem",
                fontWeight: "500",
                fontSize: "0.875rem",
                color: "var(--color-text-primary)",
              }}
            >
              Hasta
            </label>
            <input
              type="date"
              value={endDate}
              onChange={(e) => setEndDate(e.target.value)}
              style={{
                width: "100%",
                padding: "0.75rem",
                border: "1px solid var(--color-border)",
                borderRadius: "var(--radius-sm)",
                fontSize: "1rem",
                boxSizing: "border-box",
                backgroundColor: "var(--color-bg)",
                color: "var(--color-text-primary)",
              }}
            />
          </div>
        </div>
        {(filterCategory || filterSupplier || searchTerm || startDate || endDate) && (
          <button
            onClick={handleClearFilters}
            style={{
              padding: "0.5rem 1rem",
              backgroundColor: "transparent",
              color: "var(--color-text-secondary)",
              border: "1px solid var(--color-border)",
              borderRadius: "var(--radius-sm)",
              cursor: "pointer",
              fontSize: "0.875rem",
            }}
          >
            Limpiar filtros
          </button>
        )}
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

      {/* Expense List */}
      {filteredExpenses.length === 0 ? (
        <div style={{ textAlign: "center", padding: "3rem" }}>
          <p style={{ fontSize: "1.1rem", color: "var(--color-text-secondary)" }}>
            No hay gastos que coincidan con tu búsqueda.
          </p>
        </div>
      ) : (
        <div>
          {filteredExpenses.map((expense) => (
            <ExpenseCard
              key={expense.id}
              expense={expense}
              onViewDetail={onViewDetail}
              onEdit={onEdit}
              onDelete={onDelete}
            />
          ))}
        </div>
      )}
    </div>
  );
}
