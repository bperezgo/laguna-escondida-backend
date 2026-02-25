"use client";

import { useState, useEffect } from "react";
import { expensesApi, expenseCategoriesApi } from "@/lib/api/expenses";
import { suppliersApi } from "@/lib/api/suppliers";
import type { Expense, ExpenseCategory, CreateExpenseRequest, ExpenseFilters } from "@/types/expense";
import type { Supplier } from "@/types/supplier";
import ExpenseList from "./ExpenseList";
import ExpenseForm from "./ExpenseForm";
import ExpenseDetail from "./ExpenseDetail";
import { PermissionGate } from "@/components/permissions";
import { PERMISSIONS } from "@/lib/permissions";

const getMonthBounds = () => {
  const now = new Date();
  const first = new Date(now.getFullYear(), now.getMonth(), 1);
  const last = new Date(now.getFullYear(), now.getMonth() + 1, 0);
  const fmt = (d: Date) => d.toISOString().split("T")[0];
  return { start: fmt(first), end: fmt(last) };
};

interface ExpensesPageClientProps {
  initialExpenses: Expense[];
  initialCategories: ExpenseCategory[];
  initialSuppliers: Supplier[];
}

export default function ExpensesPageClient({
  initialExpenses,
  initialCategories,
  initialSuppliers,
}: ExpensesPageClientProps) {
  const monthBounds = getMonthBounds();

  const [expenses, setExpenses] = useState<Expense[]>(initialExpenses);
  const [categories, setCategories] = useState<ExpenseCategory[]>(initialCategories);
  const [suppliers, setSuppliers] = useState<Supplier[]>(initialSuppliers);
  const [loading, setLoading] = useState<boolean>(false);
  const [error, setError] = useState<string>("");
  const [success, setSuccess] = useState<string>("");
  const [showForm, setShowForm] = useState<boolean>(false);
  const [formLoading, setFormLoading] = useState<boolean>(false);
  const [detailExpenseId, setDetailExpenseId] = useState<string | null>(null);
  const [editingExpense, setEditingExpense] = useState<Expense | null>(null);
  const [deleteConfirm, setDeleteConfirm] = useState<Expense | null>(null);
  const [exportLoading, setExportLoading] = useState<boolean>(false);

  // Filter state — defaults to current month
  const [filterCategory, setFilterCategory] = useState<string>("");
  const [filterSupplier, setFilterSupplier] = useState<string>("");
  const [startDate, setStartDate] = useState<string>(monthBounds.start);
  const [endDate, setEndDate] = useState<string>(monthBounds.end);

  const buildFilters = (): ExpenseFilters => ({
    category_id: filterCategory || undefined,
    supplier_id: filterSupplier || undefined,
    start_date: startDate || undefined,
    end_date: endDate || undefined,
  });

  const loadExpenses = async (filters?: ExpenseFilters) => {
    try {
      setLoading(true);
      setError("");
      const fetchedExpenses = await expensesApi.getAll(filters ?? buildFilters());
      setExpenses(fetchedExpenses);
    } catch (err) {
      const errorMessage =
        err instanceof Error ? err.message : "Error al cargar gastos";
      setError(errorMessage);
      console.error("Error loading expenses:", err);
    } finally {
      setLoading(false);
    }
  };

  // On mount, fetch with current-month defaults (overrides SSR data which has all records)
  useEffect(() => {
    loadExpenses({ start_date: monthBounds.start, end_date: monthBounds.end });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const loadCategories = async () => {
    try {
      const categoriesData = await expenseCategoriesApi.getAll();
      setCategories(categoriesData);
    } catch (err) {
      console.error("Error loading categories:", err);
    }
  };

  const loadSuppliers = async () => {
    try {
      const suppliersData = await suppliersApi.getAll();
      setSuppliers(suppliersData);
    } catch (err) {
      console.error("Error loading suppliers:", err);
    }
  };

  const handleApplyFilters = () => {
    loadExpenses(buildFilters());
  };

  const handleClearFilters = () => {
    const { start, end } = getMonthBounds();
    setFilterCategory("");
    setFilterSupplier("");
    setStartDate(start);
    setEndDate(end);
    loadExpenses({ start_date: start, end_date: end });
  };

  const handleExportCSV = async () => {
    try {
      setExportLoading(true);
      setError("");
      await expensesApi.exportCSV(buildFilters());
      setSuccess("CSV exportado correctamente");
      setTimeout(() => setSuccess(""), 3000);
    } catch (err) {
      const errorMessage =
        err instanceof Error ? err.message : "Error al exportar CSV";
      setError(errorMessage);
    } finally {
      setExportLoading(false);
    }
  };

  const handleCreate = async () => {
    await Promise.all([loadCategories(), loadSuppliers()]);
    setEditingExpense(null);
    setShowForm(true);
  };

  const handleEdit = async (expense: Expense) => {
    await Promise.all([loadCategories(), loadSuppliers()]);
    setEditingExpense(expense);
    setShowForm(true);
  };

  const handleViewDetail = (expense: Expense) => {
    setDetailExpenseId(expense.id);
  };

  const handleFormSubmit = async (
    data: CreateExpenseRequest,
    files: { pdf?: File | null; xml?: File | null; zip?: File | null }
  ) => {
    try {
      setFormLoading(true);
      setError("");

      let savedExpense: Expense;

      if (editingExpense) {
        savedExpense = await expensesApi.update(editingExpense.id, data);
      } else {
        savedExpense = await expensesApi.create(data);
      }

      // Get the category code for the document upload
      const category = categories.find((c) => c.id === data.category_id);
      const categoryCode = category?.code || "expense";

      // Upload documents if provided
      const uploadErrors: string[] = [];

      // If ZIP file is provided, upload it (contains both PDF and XML)
      if (files.zip) {
        try {
          await expensesApi.uploadZipDocument(
            savedExpense.id,
            categoryCode,
            files.zip
          );
        } catch (err) {
          uploadErrors.push(
            `Error al subir ZIP: ${err instanceof Error ? err.message : "Error desconocido"}`
          );
        }
      } else {
        // Upload individual files
        if (files.pdf) {
          try {
            await expensesApi.uploadDocument(
              savedExpense.id,
              categoryCode,
              "pdf",
              files.pdf
            );
          } catch (err) {
            uploadErrors.push(
              `Error al subir PDF: ${err instanceof Error ? err.message : "Error desconocido"}`
            );
          }
        }

        if (files.xml) {
          try {
            await expensesApi.uploadDocument(
              savedExpense.id,
              categoryCode,
              "xml",
              files.xml
            );
          } catch (err) {
            uploadErrors.push(
              `Error al subir XML: ${err instanceof Error ? err.message : "Error desconocido"}`
            );
          }
        }
      }

      await loadExpenses();
      setShowForm(false);
      setEditingExpense(null);

      // Show upload errors if any (expense was still saved)
      if (uploadErrors.length > 0) {
        setError(
          `El gasto se guardó pero hubo errores al subir documentos: ${uploadErrors.join(". ")}`
        );
      }
    } catch (err) {
      const errorMessage =
        err instanceof Error ? err.message : "Error al guardar gasto";
      setError(errorMessage);
      console.error("Error saving expense:", err);
    } finally {
      setFormLoading(false);
    }
  };

  const handleCancel = () => {
    setShowForm(false);
    setEditingExpense(null);
    setError("");
  };

  const handleDeleteClick = (expense: Expense) => {
    setDeleteConfirm(expense);
  };

  const handleDeleteConfirm = async () => {
    if (!deleteConfirm) return;

    try {
      setError("");
      await expensesApi.delete(deleteConfirm.id);
      await loadExpenses();
      setDeleteConfirm(null);
    } catch (err) {
      const errorMessage =
        err instanceof Error ? err.message : "Error al eliminar gasto";
      setError(errorMessage);
      console.error("Error deleting expense:", err);
    }
  };

  const handleDeleteCancel = () => {
    setDeleteConfirm(null);
  };

  const formatCurrency = (amount: string) => {
    return new Intl.NumberFormat("es-CO", {
      style: "currency",
      currency: "COP",
      minimumFractionDigits: 0,
      maximumFractionDigits: 0,
    }).format(parseFloat(amount));
  };

  return (
    <div
      style={{
        minHeight: "100vh",
        backgroundColor: "var(--color-bg)",
        padding: "2rem",
      }}
    >
      <div
        style={{
          maxWidth: "1200px",
          margin: "0 auto",
        }}
      >
        {/* Header */}
        <div
          style={{
            display: "flex",
            justifyContent: "space-between",
            alignItems: "center",
            marginBottom: "2rem",
            flexWrap: "wrap",
            gap: "1rem",
          }}
        >
          <div>
            <h1
              style={{
                margin: 0,
                fontSize: "2rem",
                fontWeight: "bold",
                color: "var(--color-text-primary)",
              }}
            >
              Gastos
            </h1>
            <p
              style={{
                margin: "0.5rem 0 0 0",
                color: "var(--color-text-secondary)",
                fontSize: "1rem",
              }}
            >
              Registro de gastos operativos, inversiones y servicios
            </p>
          </div>
          <div style={{ display: "flex", gap: "0.75rem", flexWrap: "wrap" }}>
            <PermissionGate permission={PERMISSIONS.EXPENSES_EXPORT}>
              <button
                onClick={handleExportCSV}
                disabled={exportLoading}
                style={{
                  padding: "0.75rem 1.5rem",
                  backgroundColor: "transparent",
                  color: "var(--color-primary)",
                  border: "1px solid var(--color-primary)",
                  borderRadius: "var(--radius-sm)",
                  cursor: exportLoading ? "not-allowed" : "pointer",
                  fontSize: "1rem",
                  fontWeight: "500",
                  opacity: exportLoading ? 0.7 : 1,
                  transition: "background-color var(--transition-normal)",
                }}
              >
                {exportLoading ? "Exportando..." : "Exportar CSV"}
              </button>
            </PermissionGate>
            <PermissionGate permission={PERMISSIONS.EXPENSES_CREATE}>
              <button
                onClick={handleCreate}
                style={{
                  padding: "0.75rem 1.5rem",
                  backgroundColor: "var(--color-success)",
                  color: "white",
                  border: "none",
                  borderRadius: "var(--radius-sm)",
                  cursor: "pointer",
                  fontSize: "1rem",
                  fontWeight: "500",
                  boxShadow: "var(--shadow-sm)",
                  transition: "background-color var(--transition-normal)",
                }}
                onMouseEnter={(e) => {
                  e.currentTarget.style.backgroundColor = "var(--color-success-hover)";
                }}
                onMouseLeave={(e) => {
                  e.currentTarget.style.backgroundColor = "var(--color-success)";
                }}
              >
                + Nuevo Gasto
              </button>
            </PermissionGate>
          </div>
        </div>

        {/* Success Message */}
        {success && (
          <div
            style={{
              padding: "1rem",
              backgroundColor: "var(--color-success-light)",
              color: "var(--color-success)",
              border: "1px solid var(--color-success)",
              borderRadius: "var(--radius-sm)",
              marginBottom: "1.5rem",
            }}
          >
            {success}
          </div>
        )}

        {/* Error Message */}
        {error && (
          <div
            style={{
              padding: "1rem",
              backgroundColor: "var(--color-danger-light)",
              color: "var(--color-danger)",
              border: "1px solid var(--color-danger)",
              borderRadius: "var(--radius-sm)",
              marginBottom: "1.5rem",
            }}
          >
            <strong>Error:</strong> {error}
          </div>
        )}

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
          <div style={{ display: "flex", gap: "0.75rem" }}>
            <button
              onClick={handleApplyFilters}
              style={{
                padding: "0.5rem 1.25rem",
                backgroundColor: "var(--color-primary)",
                color: "white",
                border: "none",
                borderRadius: "var(--radius-sm)",
                cursor: "pointer",
                fontSize: "0.875rem",
                fontWeight: "500",
              }}
            >
              Aplicar Filtros
            </button>
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
          </div>
        </div>

        {/* Expense List */}
        <ExpenseList
          expenses={expenses}
          onViewDetail={handleViewDetail}
          onEdit={handleEdit}
          onDelete={handleDeleteClick}
          isLoading={loading}
        />

        {/* Expense Form Modal */}
        {showForm && (
          <ExpenseForm
            categories={categories}
            suppliers={suppliers}
            expense={editingExpense}
            onSubmit={handleFormSubmit}
            onCancel={handleCancel}
            isLoading={formLoading}
          />
        )}

        {/* Expense Detail Modal */}
        {detailExpenseId && (
          <ExpenseDetail
            expenseId={detailExpenseId}
            onClose={() => setDetailExpenseId(null)}
          />
        )}

        {/* Delete Confirmation Modal */}
        {deleteConfirm && (
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
                maxWidth: "450px",
                width: "100%",
                boxShadow: "var(--shadow-xl)",
                border: "1px solid var(--color-border)",
              }}
            >
              <h3
                style={{
                  margin: "0 0 1rem 0",
                  fontSize: "1.25rem",
                  fontWeight: "bold",
                  color: "var(--color-text-primary)",
                }}
              >
                Confirmar Eliminación
              </h3>
              <p
                style={{
                  margin: "0 0 1rem 0",
                  color: "var(--color-text-secondary)",
                }}
              >
                ¿Estás seguro de que deseas eliminar este gasto?
              </p>
              <div
                style={{
                  padding: "1rem",
                  backgroundColor: "var(--color-bg)",
                  borderRadius: "var(--radius-sm)",
                  marginBottom: "1.5rem",
                }}
              >
                <p
                  style={{
                    margin: "0 0 0.5rem 0",
                    fontWeight: "600",
                    color: "var(--color-text-primary)",
                  }}
                >
                  {deleteConfirm.description}
                </p>
                <p
                  style={{
                    margin: 0,
                    fontSize: "1.25rem",
                    fontWeight: "bold",
                    color: "var(--color-danger)",
                  }}
                >
                  {formatCurrency(deleteConfirm.amount)}
                </p>
              </div>
              <div
                style={{
                  display: "flex",
                  gap: "1rem",
                  justifyContent: "flex-end",
                }}
              >
                <button
                  onClick={handleDeleteCancel}
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
                  Cancelar
                </button>
                <button
                  onClick={handleDeleteConfirm}
                  style={{
                    padding: "0.75rem 1.5rem",
                    backgroundColor: "var(--color-danger)",
                    color: "white",
                    border: "none",
                    borderRadius: "var(--radius-sm)",
                    cursor: "pointer",
                    fontSize: "1rem",
                    fontWeight: "500",
                  }}
                >
                  Eliminar
                </button>
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
