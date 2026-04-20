"use client";

import { useState, useEffect } from "react";
import { supportDocumentsApi } from "@/lib/api/support-documents";
import type {
  CreateSupportDocumentRequest,
  SupportDocumentListItem,
  SupportDocumentFilters,
} from "@/types/support-document";
import SupportDocumentForm from "@/components/support-documents/SupportDocumentForm";
import { PermissionGate } from "@/components/permissions";
import { PERMISSIONS } from "@/lib/permissions";

export default function SupportDocumentsPageClient() {
  const [showForm, setShowForm] = useState<boolean>(false);
  const [formLoading, setFormLoading] = useState<boolean>(false);
  const [exportLoading, setExportLoading] = useState<boolean>(false);
  const [error, setError] = useState<string>("");
  const [success, setSuccess] = useState<string>("");

  const [documents, setDocuments] = useState<SupportDocumentListItem[]>([]);
  const [loading, setLoading] = useState<boolean>(false);
  const [total, setTotal] = useState<number>(0);
  const [totalPages, setTotalPages] = useState<number>(0);
  const [currentPage, setCurrentPage] = useState<number>(1);
  const [pageSize] = useState<number>(20);

  const [filters, setFilters] = useState<SupportDocumentFilters>({
    page: 1,
    page_size: 20,
  });
  const [createdAtStart, setCreatedAtStart] = useState<string>("");
  const [createdAtEnd, setCreatedAtEnd] = useState<string>("");
  const [providerDocumentNumber, setProviderDocumentNumber] =
    useState<string>("");

  const fetchDocuments = async () => {
    try {
      setLoading(true);
      setError("");
      const response = await supportDocumentsApi.getAll(filters);
      setDocuments(response.support_documents || []);
      setTotal(response.total_count || 0);
      setTotalPages(response.total_pages || 0);
      setCurrentPage(response.page || 1);
    } catch (err) {
      const errorMessage =
        err instanceof Error
          ? err.message
          : "Error al obtener documentos soporte";
      setError(errorMessage);
      console.error("Error fetching support documents:", err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchDocuments();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [
    filters.page,
    filters.page_size,
    filters.created_at_start,
    filters.created_at_end,
    filters.provider_document_number,
  ]);

  const handleApplyFilters = () => {
    const newFilters: SupportDocumentFilters = {
      page: 1,
      page_size: pageSize,
    };

    if (createdAtStart) {
      newFilters.created_at_start = new Date(createdAtStart).toISOString();
    }
    if (createdAtEnd) {
      newFilters.created_at_end = new Date(createdAtEnd).toISOString();
    }
    if (providerDocumentNumber) {
      newFilters.provider_document_number = providerDocumentNumber;
    }

    setFilters(newFilters);
  };

  const handleClearFilters = () => {
    setCreatedAtStart("");
    setCreatedAtEnd("");
    setProviderDocumentNumber("");
    setFilters({
      page: 1,
      page_size: pageSize,
    });
  };

  const handlePageChange = (newPage: number) => {
    setFilters({ ...filters, page: newPage });
  };

  const handleCreate = () => {
    setShowForm(true);
    setError("");
    setSuccess("");
  };

  const handleFormSubmit = async (data: CreateSupportDocumentRequest) => {
    try {
      setFormLoading(true);
      setError("");
      setSuccess("");

      await supportDocumentsApi.create(data);

      setSuccess("Documento soporte creado exitosamente!");
      setShowForm(false);

      fetchDocuments();

      setTimeout(() => setSuccess(""), 3000);
    } catch (err) {
      const errorMessage =
        err instanceof Error
          ? err.message
          : "Error al crear documento soporte";
      setError(errorMessage);
      console.error("Error creating support document:", err);
    } finally {
      setFormLoading(false);
    }
  };

  const handleCancel = () => {
    setShowForm(false);
    setError("");
    setSuccess("");
  };

  const handleExportCSV = async () => {
    try {
      setExportLoading(true);
      setError("");

      await supportDocumentsApi.exportCSV(filters);

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

  const handleDocumentClick = (url: string) => {
    window.open(url, "_blank", "noopener,noreferrer");
  };

  const formatDateTime = (dateString: string) => {
    const date = new Date(dateString);
    return date.toLocaleString("en-US", {
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
              Documentos Soporte
            </h1>
            <p
              style={{
                margin: "0.5rem 0 0 0",
                color: "var(--color-text-secondary)",
                fontSize: "1rem",
              }}
            >
              Crea y administra documentos soporte para proveedores informales
            </p>
          </div>
          <div style={{ display: "flex", gap: "0.75rem" }}>
            <PermissionGate permission={PERMISSIONS.SUPPORT_DOCUMENTS_EXPORT}>
              <button
                onClick={handleExportCSV}
                disabled={exportLoading}
                style={{
                  padding: "0.75rem 1.5rem",
                  backgroundColor: exportLoading
                    ? "var(--color-border)"
                    : "var(--color-primary)",
                  color: exportLoading ? "var(--color-text-muted)" : "white",
                  border: "none",
                  borderRadius: "var(--radius-sm)",
                  cursor: exportLoading ? "not-allowed" : "pointer",
                  fontSize: "1rem",
                  fontWeight: "500",
                  boxShadow: "var(--shadow-sm)",
                  transition: "background-color var(--transition-normal)",
                  display: "flex",
                  alignItems: "center",
                  gap: "0.5rem",
                }}
                onMouseEnter={(e) => {
                  if (!exportLoading) {
                    e.currentTarget.style.backgroundColor =
                      "var(--color-primary-hover)";
                  }
                }}
                onMouseLeave={(e) => {
                  if (!exportLoading) {
                    e.currentTarget.style.backgroundColor =
                      "var(--color-primary)";
                  }
                }}
              >
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
                {exportLoading ? "Exportando..." : "Descargar CSV"}
              </button>
            </PermissionGate>
            <PermissionGate permission={PERMISSIONS.SUPPORT_DOCUMENTS_CREATE}>
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
                  e.currentTarget.style.backgroundColor =
                    "var(--color-success-hover)";
                }}
                onMouseLeave={(e) => {
                  e.currentTarget.style.backgroundColor =
                    "var(--color-success)";
                }}
              >
                + Crear Documento Soporte
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
            <div>
              <label
                style={{
                  display: "block",
                  marginBottom: "0.5rem",
                  fontSize: "0.875rem",
                  fontWeight: "500",
                  color: "var(--color-text-secondary)",
                }}
              >
                Fecha de Inicio
              </label>
              <input
                type="date"
                value={createdAtStart}
                onChange={(e) => setCreatedAtStart(e.target.value)}
                style={{
                  width: "100%",
                  padding: "0.5rem",
                  border: "1px solid var(--color-border)",
                  borderRadius: "var(--radius-sm)",
                  fontSize: "1rem",
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
                  fontSize: "0.875rem",
                  fontWeight: "500",
                  color: "var(--color-text-secondary)",
                }}
              >
                Fecha de Fin
              </label>
              <input
                type="date"
                value={createdAtEnd}
                onChange={(e) => setCreatedAtEnd(e.target.value)}
                style={{
                  width: "100%",
                  padding: "0.5rem",
                  border: "1px solid var(--color-border)",
                  borderRadius: "var(--radius-sm)",
                  fontSize: "1rem",
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
                  fontSize: "0.875rem",
                  fontWeight: "500",
                  color: "var(--color-text-secondary)",
                }}
              >
                NIT del Proveedor
              </label>
              <input
                type="text"
                value={providerDocumentNumber}
                onChange={(e) => setProviderDocumentNumber(e.target.value)}
                placeholder="Ingresa el NIT del proveedor"
                style={{
                  width: "100%",
                  padding: "0.5rem",
                  border: "1px solid var(--color-border)",
                  borderRadius: "var(--radius-sm)",
                  fontSize: "1rem",
                  backgroundColor: "var(--color-bg)",
                  color: "var(--color-text-primary)",
                }}
              />
            </div>
          </div>
          <div
            style={{
              display: "flex",
              gap: "1rem",
            }}
          >
            <button
              onClick={handleApplyFilters}
              style={{
                padding: "0.5rem 1.5rem",
                backgroundColor: "var(--color-primary)",
                color: "white",
                border: "none",
                borderRadius: "var(--radius-sm)",
                cursor: "pointer",
                fontSize: "1rem",
                fontWeight: "500",
                transition: "background-color var(--transition-normal)",
              }}
              onMouseEnter={(e) => {
                e.currentTarget.style.backgroundColor =
                  "var(--color-primary-hover)";
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.backgroundColor = "var(--color-primary)";
              }}
            >
              Aplicar Filtros
            </button>
            <button
              onClick={handleClearFilters}
              style={{
                padding: "0.5rem 1.5rem",
                backgroundColor: "var(--color-surface-hover)",
                color: "var(--color-text-primary)",
                border: "1px solid var(--color-border)",
                borderRadius: "var(--radius-sm)",
                cursor: "pointer",
                fontSize: "1rem",
                fontWeight: "500",
                transition: "background-color var(--transition-normal)",
              }}
              onMouseEnter={(e) => {
                e.currentTarget.style.backgroundColor =
                  "var(--color-surface-active)";
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.backgroundColor =
                  "var(--color-surface-hover)";
              }}
            >
              Limpiar Filtros
            </button>
          </div>
        </div>

        {/* Document List */}
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
              Cargando documentos soporte...
            </p>
          </div>
        ) : documents.length === 0 ? (
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
              No se encontraron documentos soporte. Haz clic en &quot;Crear
              Documento Soporte&quot; para comenzar.
            </p>
          </div>
        ) : (
          <>
            <div
              style={{
                backgroundColor: "var(--color-surface)",
                borderRadius: "var(--radius-md)",
                border: "1px solid var(--color-border)",
                overflow: "hidden",
              }}
            >
              <div
                style={{
                  padding: "1rem 1.5rem",
                  borderBottom: "1px solid var(--color-border)",
                  backgroundColor: "var(--color-bg)",
                }}
              >
                <p
                  style={{
                    margin: 0,
                    fontSize: "0.875rem",
                    color: "var(--color-text-secondary)",
                  }}
                >
                  Mostrando {documents.length} de {total} documentos soporte
                </p>
              </div>
              <div
                style={{
                  overflowX: "auto",
                }}
              >
                <table
                  style={{
                    width: "100%",
                    borderCollapse: "collapse",
                  }}
                >
                  <thead>
                    <tr
                      style={{
                        backgroundColor: "var(--color-bg)",
                        borderBottom: "1px solid var(--color-border)",
                      }}
                    >
                      <th
                        style={{
                          padding: "1rem",
                          textAlign: "left",
                          fontSize: "0.875rem",
                          fontWeight: "bold",
                          color: "var(--color-text-primary)",
                        }}
                      >
                        Fecha de Creación
                      </th>
                      <th
                        style={{
                          padding: "1rem",
                          textAlign: "left",
                          fontSize: "0.875rem",
                          fontWeight: "bold",
                          color: "var(--color-text-primary)",
                        }}
                      >
                        Proveedor
                      </th>
                      <th
                        style={{
                          padding: "1rem",
                          textAlign: "left",
                          fontSize: "0.875rem",
                          fontWeight: "bold",
                          color: "var(--color-text-primary)",
                        }}
                      >
                        CUDS
                      </th>
                      <th
                        style={{
                          padding: "1rem",
                          textAlign: "left",
                          fontSize: "0.875rem",
                          fontWeight: "bold",
                          color: "var(--color-text-primary)",
                        }}
                      >
                        Tascode
                      </th>
                      <th
                        style={{
                          padding: "1rem",
                          textAlign: "right",
                          fontSize: "0.875rem",
                          fontWeight: "bold",
                          color: "var(--color-text-primary)",
                        }}
                      >
                        Total
                      </th>
                      <th
                        style={{
                          padding: "1rem",
                          textAlign: "right",
                          fontSize: "0.875rem",
                          fontWeight: "bold",
                          color: "var(--color-text-primary)",
                        }}
                      >
                        Descuento
                      </th>
                      <th
                        style={{
                          padding: "1rem",
                          textAlign: "right",
                          fontSize: "0.875rem",
                          fontWeight: "bold",
                          color: "var(--color-text-primary)",
                        }}
                      >
                        VAT
                      </th>
                      <th
                        style={{
                          padding: "1rem",
                          textAlign: "right",
                          fontSize: "0.875rem",
                          fontWeight: "bold",
                          color: "var(--color-text-primary)",
                        }}
                      >
                        ICO
                      </th>
                      <th
                        style={{
                          padding: "1rem",
                          textAlign: "right",
                          fontSize: "0.875rem",
                          fontWeight: "bold",
                          color: "var(--color-text-primary)",
                        }}
                      >
                        Propina
                      </th>
                      <th
                        style={{
                          padding: "1rem",
                          textAlign: "center",
                          fontSize: "0.875rem",
                          fontWeight: "bold",
                          color: "var(--color-text-primary)",
                        }}
                      >
                        PDF
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    {documents.map((doc, index) => (
                      <tr
                        key={doc.id}
                        style={{
                          borderBottom: "1px solid var(--color-border)",
                          backgroundColor:
                            index % 2 === 0
                              ? "var(--color-surface)"
                              : "var(--color-bg)",
                        }}
                      >
                        <td
                          style={{
                            padding: "1rem",
                            fontSize: "0.875rem",
                            color: "var(--color-text-primary)",
                          }}
                        >
                          {formatDateTime(doc.created_at)}
                        </td>
                        <td
                          style={{
                            padding: "1rem",
                            fontSize: "0.875rem",
                            color: "var(--color-text-primary)",
                          }}
                        >
                          <div>{doc.provider_name}</div>
                          <div
                            style={{
                              fontSize: "0.75rem",
                              color: "var(--color-text-muted)",
                            }}
                          >
                            {doc.provider_document_number}
                          </div>
                        </td>
                        <td
                          style={{
                            padding: "1rem",
                            fontSize: "0.75rem",
                            color: "var(--color-text-secondary)",
                            maxWidth: "150px",
                            overflow: "hidden",
                            textOverflow: "ellipsis",
                            whiteSpace: "nowrap",
                          }}
                          title={doc.cuds}
                        >
                          {doc.cuds}
                        </td>
                        <td
                          style={{
                            padding: "1rem",
                            fontSize: "0.875rem",
                            color: "var(--color-text-primary)",
                          }}
                        >
                          {doc.tascode}
                        </td>
                        <td
                          style={{
                            padding: "1rem",
                            fontSize: "0.875rem",
                            fontWeight: "bold",
                            color: "var(--color-success)",
                            textAlign: "right",
                          }}
                        >
                          ${doc.total_amount}
                        </td>
                        <td
                          style={{
                            padding: "1rem",
                            fontSize: "0.875rem",
                            color: "var(--color-text-secondary)",
                            textAlign: "right",
                          }}
                        >
                          ${doc.discount_amount}
                        </td>
                        <td
                          style={{
                            padding: "1rem",
                            fontSize: "0.875rem",
                            color: "var(--color-text-secondary)",
                            textAlign: "right",
                          }}
                        >
                          ${doc.vat}
                        </td>
                        <td
                          style={{
                            padding: "1rem",
                            fontSize: "0.875rem",
                            color: "var(--color-text-secondary)",
                            textAlign: "right",
                          }}
                        >
                          ${doc.ico}
                        </td>
                        <td
                          style={{
                            padding: "1rem",
                            fontSize: "0.875rem",
                            color: "var(--color-text-secondary)",
                            textAlign: "right",
                          }}
                        >
                          ${doc.tip}
                        </td>
                        <td
                          style={{
                            padding: "1rem",
                            textAlign: "center",
                          }}
                        >
                          {doc.pdf_download_url ? (
                            <button
                              onClick={() =>
                                handleDocumentClick(doc.pdf_download_url!)
                              }
                              style={{
                                background: "none",
                                border: "none",
                                cursor: "pointer",
                                padding: "0.25rem",
                                display: "inline-flex",
                                alignItems: "center",
                                justifyContent: "center",
                              }}
                              title="Ver documento (PDF)"
                            >
                              <svg
                                width="24"
                                height="24"
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
                            </button>
                          ) : (
                            <div
                              style={{
                                display: "inline-flex",
                                alignItems: "center",
                                justifyContent: "center",
                                padding: "0.25rem",
                              }}
                              title="Documento no disponible"
                            >
                              <svg
                                width="24"
                                height="24"
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
                                <line
                                  x1="9"
                                  y1="12"
                                  x2="15"
                                  y2="18"
                                  stroke="var(--color-danger)"
                                />
                                <line
                                  x1="15"
                                  y1="12"
                                  x2="9"
                                  y2="18"
                                  stroke="var(--color-danger)"
                                />
                              </svg>
                            </div>
                          )}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>

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
                <button
                  onClick={() => handlePageChange(currentPage - 1)}
                  disabled={currentPage === 1}
                  style={{
                    padding: "0.5rem 1rem",
                    backgroundColor:
                      currentPage === 1
                        ? "var(--color-border)"
                        : "var(--color-primary)",
                    color:
                      currentPage === 1 ? "var(--color-text-muted)" : "white",
                    border: "none",
                    borderRadius: "var(--radius-sm)",
                    cursor: currentPage === 1 ? "not-allowed" : "pointer",
                    fontSize: "0.875rem",
                    fontWeight: "500",
                  }}
                >
                  Anterior
                </button>
                <span
                  style={{
                    padding: "0.5rem 1rem",
                    fontSize: "0.875rem",
                    color: "var(--color-text-primary)",
                  }}
                >
                  Página {currentPage} de {totalPages}
                </span>
                <button
                  onClick={() => handlePageChange(currentPage + 1)}
                  disabled={currentPage === totalPages}
                  style={{
                    padding: "0.5rem 1rem",
                    backgroundColor:
                      currentPage === totalPages
                        ? "var(--color-border)"
                        : "var(--color-primary)",
                    color:
                      currentPage === totalPages
                        ? "var(--color-text-muted)"
                        : "white",
                    border: "none",
                    borderRadius: "var(--radius-sm)",
                    cursor:
                      currentPage === totalPages ? "not-allowed" : "pointer",
                    fontSize: "0.875rem",
                    fontWeight: "500",
                  }}
                >
                  Siguiente
                </button>
              </div>
            )}
          </>
        )}

        {/* Support Document Form Modal */}
        {showForm && (
          <SupportDocumentForm
            onSubmit={handleFormSubmit}
            onCancel={handleCancel}
            isLoading={formLoading}
          />
        )}
      </div>
    </div>
  );
}
