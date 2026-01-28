"use client";

import { useState } from "react";
import { purchaseEntriesApi } from "@/lib/api/purchaseEntries";
import { suppliersApi } from "@/lib/api/suppliers";
import type { PurchaseEntry, CreatePurchaseEntryRequest } from "@/types/purchaseEntry";
import type { Supplier } from "@/types/supplier";
import PurchaseEntryList from "./PurchaseEntryList";
import PurchaseEntryForm from "./PurchaseEntryForm";
import PurchaseEntryDetail from "./PurchaseEntryDetail";
import { PermissionGate } from "@/components/permissions";
import { PERMISSIONS } from "@/lib/permissions";

interface PurchaseEntriesPageClientProps {
  initialEntries: PurchaseEntry[];
  initialSuppliers: Supplier[];
}

export default function PurchaseEntriesPageClient({
  initialEntries,
  initialSuppliers,
}: PurchaseEntriesPageClientProps) {
  const [entries, setEntries] = useState<PurchaseEntry[]>(initialEntries);
  const [suppliers, setSuppliers] = useState<Supplier[]>(initialSuppliers);
  const [loading, setLoading] = useState<boolean>(false);
  const [error, setError] = useState<string>("");
  const [showForm, setShowForm] = useState<boolean>(false);
  const [formLoading, setFormLoading] = useState<boolean>(false);
  const [detailEntryId, setDetailEntryId] = useState<string | null>(null);

  const loadEntries = async () => {
    try {
      setLoading(true);
      setError("");
      const fetchedEntries = await purchaseEntriesApi.getAll();
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

  const loadSuppliers = async () => {
    try {
      const suppliersData = await suppliersApi.getAll();
      setSuppliers(suppliersData);
    } catch (err) {
      console.error("Error loading suppliers:", err);
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

        {/* Entries List */}
        <PurchaseEntryList
          entries={entries}
          suppliers={suppliers}
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
