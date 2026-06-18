"use client";

import { useState, useEffect } from "react";
import { invoicesApi } from "@/lib/api/invoices";
import type {
  CreateElectronicInvoiceRequest,
  InvoiceListItem,
  InvoiceFilters,
} from "@/types/invoice";
import InvoiceForm from "@/components/invoices/InvoiceForm";
import { PermissionGate } from "@/components/permissions";
import { PERMISSIONS } from "@/lib/permissions";
import { Button, Input, Table } from "@/components/ui";

export default function InvoicesPageClient() {
  const [showForm, setShowForm] = useState<boolean>(false);
  const [formLoading, setFormLoading] = useState<boolean>(false);
  const [exportLoading, setExportLoading] = useState<boolean>(false);
  const [error, setError] = useState<string>("");
  const [success, setSuccess] = useState<string>("");

  // Invoice list state
  const [invoices, setInvoices] = useState<InvoiceListItem[]>([]);
  const [loading, setLoading] = useState<boolean>(false);
  const [total, setTotal] = useState<number>(0);
  const [totalPages, setTotalPages] = useState<number>(0);
  const [currentPage, setCurrentPage] = useState<number>(1);
  const [pageSize] = useState<number>(20);

  // Filter state
  const [filters, setFilters] = useState<InvoiceFilters>({
    page: 1,
    page_size: 20,
  });
  const [createdAtStart, setCreatedAtStart] = useState<string>("");
  const [createdAtEnd, setCreatedAtEnd] = useState<string>("");
  const [nationalIdentification, setNationalIdentification] =
    useState<string>("");

  // Fetch invoices
  const fetchInvoices = async () => {
    try {
      setLoading(true);
      setError("");
      const response = await invoicesApi.getAll(filters);
      setInvoices(response.invoices || []);
      setTotal(response.total || 0);
      setTotalPages(response.total_pages || 0);
      setCurrentPage(response.page || 1);
    } catch (err) {
      const errorMessage =
        err instanceof Error ? err.message : "Failed to fetch invoices";
      setError(errorMessage);
      console.error("Error fetching invoices:", err);
    } finally {
      setLoading(false);
    }
  };

  // Fetch invoices on mount and when filters change
  useEffect(() => {
    fetchInvoices();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [
    filters.page,
    filters.page_size,
    filters.created_at_start,
    filters.created_at_end,
    filters.national_identification,
  ]);

  // Apply filters
  const handleApplyFilters = () => {
    const newFilters: InvoiceFilters = {
      page: 1,
      page_size: pageSize,
    };

    if (createdAtStart) {
      newFilters.created_at_start = new Date(createdAtStart).toISOString();
    }
    if (createdAtEnd) {
      newFilters.created_at_end = new Date(createdAtEnd).toISOString();
    }
    if (nationalIdentification) {
      newFilters.national_identification = nationalIdentification;
    }

    setFilters(newFilters);
  };

  // Clear filters
  const handleClearFilters = () => {
    setCreatedAtStart("");
    setCreatedAtEnd("");
    setNationalIdentification("");
    setFilters({
      page: 1,
      page_size: pageSize,
    });
  };

  // Pagination
  const handlePageChange = (newPage: number) => {
    setFilters({ ...filters, page: newPage });
  };

  const handleCreate = () => {
    setShowForm(true);
    setError("");
    setSuccess("");
  };

  const handleFormSubmit = async (data: CreateElectronicInvoiceRequest) => {
    try {
      setFormLoading(true);
      setError("");
      setSuccess("");

      await invoicesApi.create(data);

      setSuccess("Invoice created successfully!");
      setShowForm(false);

      // Refresh invoice list
      fetchInvoices();

      // Clear success message after 3 seconds
      setTimeout(() => setSuccess(""), 3000);
    } catch (err) {
      const errorMessage =
        err instanceof Error ? err.message : "Failed to create invoice";
      setError(errorMessage);
      console.error("Error creating invoice:", err);
    } finally {
      setFormLoading(false);
    }
  };

  const handleCancel = () => {
    setShowForm(false);
    setError("");
    setSuccess("");
  };

  // Handle CSV export
  const handleExportCSV = async () => {
    try {
      setExportLoading(true);
      setError("");

      await invoicesApi.exportCSV(filters);

      setSuccess("CSV exportado correctamente");
      setTimeout(() => setSuccess(""), 3000);
    } catch (err) {
      const errorMessage =
        err instanceof Error ? err.message : "Error al exportar CSV";
      setError(errorMessage);
      console.error("Error exporting CSV:", err);
    } finally {
      setExportLoading(false);
    }
  };

  // Handle document click - open PDF in new tab
  const handleDocumentClick = (url: string) => {
    window.open(url, "_blank", "noopener,noreferrer");
  };

  // Format date and time
  const formatDateTime = (dateString: string) => {
    const date = new Date(dateString);
    return date.toLocaleString("es-CO", {
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
      hour12: false,
    });
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
              Facturas Electrónicas
            </h1>
            <p
              style={{
                margin: "0.5rem 0 0 0",
                color: "var(--color-text-secondary)",
                fontSize: "1rem",
              }}
            >
              Crea y administra facturas electrónicas
            </p>
          </div>
          <div style={{ display: "flex", gap: "0.75rem" }}>
            <PermissionGate permission={PERMISSIONS.INVOICES_EXPORT}>
              <Button
                variant="secondary"
                onClick={handleExportCSV}
                disabled={exportLoading}
                leftIcon={
                  <svg
                    width="18"
                    height="18"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="2"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  >
                    <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
                    <polyline points="7 10 12 15 17 10" />
                    <line x1="12" y1="15" x2="12" y2="3" />
                  </svg>
                }
              >
                {exportLoading ? "Exportando..." : "Descargar CSV"}
              </Button>
            </PermissionGate>
            <PermissionGate permission={PERMISSIONS.INVOICES_CREATE}>
              <Button onClick={handleCreate}>+ Crear Factura</Button>
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
            <strong>Éxito:</strong> {success}
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

        {/* Filters Section */}
        <div
          style={{
            backgroundColor: "var(--color-surface)",
            padding: "1.5rem",
            borderRadius: "var(--radius-md)",
            border: "1px solid var(--color-border)",
            marginBottom: "1.5rem",
          }}
        >
          <h2
            style={{
              margin: "0 0 1rem 0",
              fontSize: "1.25rem",
              fontWeight: "bold",
              color: "var(--color-text-primary)",
            }}
          >
            Filtros de Búsqueda
          </h2>
          <div
            style={{
              display: "grid",
              gridTemplateColumns: "repeat(auto-fit, minmax(200px, 1fr))",
              gap: "1rem",
              marginBottom: "1rem",
            }}
          >
            <Input
              label="Fecha de Inicio"
              type="date"
              value={createdAtStart}
              onChange={(e) => setCreatedAtStart(e.target.value)}
            />
            <Input
              label="Fecha de Fin"
              type="date"
              value={createdAtEnd}
              onChange={(e) => setCreatedAtEnd(e.target.value)}
            />
            <Input
              label="Identificación Nacional"
              type="text"
              value={nationalIdentification}
              onChange={(e) => setNationalIdentification(e.target.value)}
              placeholder="Ingresa el número de identificación"
            />
          </div>
          <div
            style={{
              display: "flex",
              gap: "1rem",
            }}
          >
            <Button onClick={handleApplyFilters}>Aplicar Filtros</Button>
            <Button variant="secondary" onClick={handleClearFilters}>
              Limpiar Filtros
            </Button>
          </div>
        </div>

        {/* Invoice List */}
        {loading ? (
          <div
            style={{
              padding: "3rem",
              textAlign: "center",
              backgroundColor: "var(--color-surface)",
              borderRadius: "var(--radius-md)",
              border: "1px solid var(--color-border)",
            }}
          >
            <p
              style={{
                color: "var(--color-text-secondary)",
                fontSize: "1rem",
                margin: 0,
              }}
            >
              Cargando facturas...
            </p>
          </div>
        ) : invoices.length === 0 ? (
          <div
            style={{
              padding: "3rem",
              textAlign: "center",
              backgroundColor: "var(--color-surface)",
              borderRadius: "var(--radius-md)",
              border: "1px solid var(--color-border)",
            }}
          >
            <p
              style={{
                color: "var(--color-text-secondary)",
                fontSize: "1rem",
                margin: 0,
              }}
            >
              No se encontraron facturas. Haz clic en &quot;Crear Factura&quot;
              para comenzar a crear una nueva factura electrónica.
            </p>
          </div>
        ) : (
          <>
            <div
              style={{
                marginBottom: "0.75rem",
                fontSize: "0.875rem",
                color: "var(--color-text-secondary)",
              }}
            >
              Mostrando {invoices.length} de {total} facturas
            </div>
            <Table>
              <thead>
                <tr>
                  <th>Fecha de Creación</th>
                  <th>CUFE</th>
                  <th>Tascode</th>
                  <th data-numeric>Total</th>
                  <th data-numeric>Descuento</th>
                  <th data-numeric>VAT</th>
                  <th data-numeric>ICO</th>
                  <th data-numeric>Propina</th>
                  <th style={{ textAlign: "center" }}>Documento</th>
                </tr>
              </thead>
              <tbody>
                {invoices.map((invoice) => (
                  <tr key={invoice.id}>
                    <td>{formatDateTime(invoice.created_at)}</td>
                    <td
                      style={{
                        fontSize: "0.75rem",
                        color: "var(--color-text-secondary)",
                        maxWidth: "150px",
                        overflow: "hidden",
                        textOverflow: "ellipsis",
                        whiteSpace: "nowrap",
                      }}
                      title={invoice.cufe}
                    >
                      {invoice.cufe}
                    </td>
                    <td>{invoice.tascode}</td>
                    <td data-numeric style={{ fontWeight: 600 }}>
                      ${invoice.total_amount}
                    </td>
                    <td data-numeric>${invoice.discount_amount}</td>
                    <td data-numeric>${invoice.vat}</td>
                    <td data-numeric>${invoice.ico}</td>
                    <td data-numeric>${invoice.tip}</td>
                    <td style={{ textAlign: "center" }}>
                      {invoice.document_url ? (
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() =>
                            handleDocumentClick(invoice.document_url!)
                          }
                          title="Ver documento (PDF)"
                        >
                          <svg
                            width="20"
                            height="20"
                            viewBox="0 0 24 24"
                            fill="none"
                            stroke="var(--color-success)"
                            strokeWidth="2"
                            strokeLinecap="round"
                            strokeLinejoin="round"
                          >
                            <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
                            <polyline points="14 2 14 8 20 8" />
                            <line x1="16" y1="13" x2="8" y2="13" />
                            <line x1="16" y1="17" x2="8" y2="17" />
                            <polyline points="10 9 9 9 8 9" />
                          </svg>
                        </Button>
                      ) : (
                        <span
                          style={{
                            display: "inline-flex",
                            alignItems: "center",
                            justifyContent: "center",
                            padding: "0.25rem",
                          }}
                          title="Documento no disponible"
                        >
                          <svg
                            width="20"
                            height="20"
                            viewBox="0 0 24 24"
                            fill="none"
                            stroke="var(--color-danger)"
                            strokeWidth="2"
                            strokeLinecap="round"
                            strokeLinejoin="round"
                          >
                            <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
                            <polyline points="14 2 14 8 20 8" />
                            <line x1="9" y1="15" x2="15" y2="15" />
                            <line x1="12" y1="12" x2="12" y2="18" />
                            <line x1="9" y1="12" x2="15" y2="18" />
                            <line x1="15" y1="12" x2="9" y2="18" />
                          </svg>
                        </span>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </Table>

            {/* Pagination */}
            {totalPages > 1 && (
              <div
                style={{
                  display: "flex",
                  justifyContent: "center",
                  alignItems: "center",
                  gap: "0.5rem",
                  marginTop: "1.5rem",
                }}
              >
                <Button
                  variant="secondary"
                  onClick={() => handlePageChange(currentPage - 1)}
                  disabled={currentPage === 1}
                >
                  Anterior
                </Button>
                <span
                  style={{
                    padding: "0.5rem 1rem",
                    fontSize: "0.875rem",
                    color: "var(--color-text-primary)",
                  }}
                >
                  Página {currentPage} de {totalPages}
                </span>
                <Button
                  variant="secondary"
                  onClick={() => handlePageChange(currentPage + 1)}
                  disabled={currentPage === totalPages}
                >
                  Siguiente
                </Button>
              </div>
            )}
          </>
        )}

        {/* Invoice Form Modal */}
        {showForm && (
          <InvoiceForm
            onSubmit={handleFormSubmit}
            onCancel={handleCancel}
            isLoading={formLoading}
          />
        )}
      </div>
    </div>
  );
}
