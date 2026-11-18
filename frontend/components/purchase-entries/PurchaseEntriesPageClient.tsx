"use client";

import { useState, useEffect } from "react";
import { purchaseEntriesApi } from "@/lib/api/purchaseEntries";
import { suppliersApi } from "@/lib/api/suppliers";
import type { PurchaseEntry, CreatePurchaseEntryRequest, PurchaseEntryFilters } from "@/types/purchaseEntry";
import type { Supplier } from "@/types/supplier";
import PurchaseEntryList from "./PurchaseEntryList";
import PurchaseEntryForm from "./PurchaseEntryForm";
import PurchaseEntryDetail from "./PurchaseEntryDetail";
import { PermissionGate } from "@/components/permissions";
import { PERMISSIONS } from "@/lib/permissions";

const getMonthBounds = () => {
  const now = new Date();
  const first = new Date(now.getFullYear(), now.getMonth(), 1);
  const last = new Date(now.getFullYear(), now.getMonth() + 1, 0);
  const fmt = (d: Date) => d.toISOString().split("T")[0];
  return { start: fmt(first), end: fmt(last) };
};

interface PurchaseEntriesPageClientProps {
  initialEntries: PurchaseEntry[];
  initialSuppliers: Supplier[];
}

export default function PurchaseEntriesPageClient({
  initialEntries,
  initialSuppliers,
}: PurchaseEntriesPageClientProps) {
  const monthBounds = getMonthBounds();

  const [entries, setEntries] = useState<PurchaseEntry[]>(initialEntries);
  const [suppliers, setSuppliers] = useState<Supplier[]>(initialSuppliers);
  const [loading, setLoading] = useState<boolean>(false);
  const [error, setError] = useState<string>("");
  const [success, setSuccess] = useState<string>("");
  const [showForm, setShowForm] = useState<boolean>(false);
  const [formLoading, setFormLoading] = useState<boolean>(false);
  const [detailEntryId, setDetailEntryId] = useState<string | null>(null);
  const [exportLoading, setExportLoading] = useState<boolean>(false);

  // Filter state — defaults to current month
  const [filterSupplier, setFilterSupplier] = useState<string>("");
  const [startDate, setStartDate] = useState<string>(monthBounds.start);
  const [endDate, setEndDate] = useState<string>(monthBounds.end);

  const buildFilters = (): PurchaseEntryFilters => ({
    supplier_id: filterSupplier || undefined,
    start_date: startDate || undefined,
    end_date: endDate || undefined,
  });

  const loadEntries = async (filters?: PurchaseEntryFilters) => {
    try {
      setLoading(true);
      setError("");
      const fetchedEntries = await purchaseEntriesApi.getAll(filters ?? buildFilters());
      setEntries(fetchedEntries);
    } catch (err) {
      const errorMessage =
        err instanceof Error ? err.message : "Error al cargar entradas";
      setError(errorMessage);
      console.error("Error loading entries:", err);
    } finally {
      setLoading(false);
    }
  };

  // On mount, fetch with current-month defaults (overrides SSR data which has all records)
  useEffect(() => {
    loadEntries({ start_date: monthBounds.start, end_date: monthBounds.end });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const loadSuppliers = async () => {
    try {
      const suppliersData = await suppliersApi.getAll();
      setSuppliers(suppliersData);
    } catch (err) {
      console.error("Error loading suppliers:", err);
    }
  };

  const handleApplyFilters = () => {
    loadEntries(buildFilters());
  };

  const handleClearFilters = () => {
    const { start, end } = getMonthBounds();
    setFilterSupplier("");
    setStartDate(start);
    setEndDate(end);
    loadEntries({ start_date: start, end_date: end });
  };

  const handleExportCSV = async () => {
    try {
      setExportLoading(true);
      setError("");
      await purchaseEntriesApi.exportCSV(buildFilters());
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
    await loadSuppliers();
    setShowForm(true);
  };

  const handleViewDetail = (entry: PurchaseEntry) => {
    setDetailEntryId(entry.id);
  };

  const handleFormSubmit = async (
    data: CreatePurchaseEntryRequest,
    files: { pdf?: File | null; xml?: File | null; zip?: File | null }
  ) => {
    try {
      setFormLoading(true);
      setError("");

      // Create the purchase entry first
      const savedEntry = await purchaseEntriesApi.create(data);

      // Upload documents if provided
      const uploadErrors: string[] = [];

      // If ZIP file is provided, upload it (contains both PDF and XML)
      if (files.zip) {
        try {
          await purchaseEntriesApi.uploadZipDocument(
            savedEntry.id,
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
            await purchaseEntriesApi.uploadDocument(
              savedEntry.id,
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
            await purchaseEntriesApi.uploadDocument(
              savedEntry.id,
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

      await loadEntries();
      setShowForm(false);

      // Show upload errors if any (entry was still saved)
      if (uploadErrors.length > 0) {
        setError(
          `La entrada se guardó pero hubo errores al subir documentos: ${uploadErrors.join(". ")}`
        );
      }
    } catch (err) {
      const errorMessage =
        err instanceof Error ? err.message : "Error al guardar entrada";
      setError(errorMessage);
      console.error("Error saving entry:", err);
    } finally {
      setFormLoading(false);
    }
  };

  const handleCancel = () => {
    setShowForm(false);
    setError("");
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
              Entradas de Compra
            </h1>
            <p
              style={{
                margin: "0.5rem 0 0 0",
                color: "var(--color-text-secondary)",
                fontSize: "1rem",
              }}
            >
              Registro de productos recibidos de proveedores
            </p>
          </div>
          <div style={{ display: "flex", gap: "0.75rem", flexWrap: "wrap" }}>
            <PermissionGate permission={PERMISSIONS.PURCHASE_ENTRIES_EXPORT}>
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
            <PermissionGate permission={PERMISSIONS.PURCHASE_ENTRIES_CREATE}>
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
                + Nueva Entrada
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

        {/* Entries List */}
        <PurchaseEntryList
          entries={entries}
          onViewDetail={handleViewDetail}
          isLoading={loading}
        />

        {/* Entry Form Modal */}
        {showForm && (
          <PurchaseEntryForm
            suppliers={suppliers}
            onSubmit={handleFormSubmit}
            onCancel={handleCancel}
            isLoading={formLoading}
          />
        )}

        {/* Entry Detail Modal */}
        {detailEntryId && (
          <PurchaseEntryDetail
            entryId={detailEntryId}
            onClose={() => setDetailEntryId(null)}
          />
        )}
      </div>
    </div>
  );
}
