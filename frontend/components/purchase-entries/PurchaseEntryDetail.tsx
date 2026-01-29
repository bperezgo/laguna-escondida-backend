"use client";

import { useState, useEffect, useRef } from "react";
import type { PurchaseEntry } from "@/types/purchaseEntry";
import { purchaseEntriesApi } from "@/lib/api/purchaseEntries";
import { PermissionGate } from "@/components/permissions";
import { PERMISSIONS } from "@/lib/permissions";

interface PurchaseEntryDetailProps {
  entryId: string;
  onClose: () => void;
}

export default function PurchaseEntryDetail({
  entryId,
  onClose,
}: PurchaseEntryDetailProps) {
  const [entry, setEntry] = useState<PurchaseEntry | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [uploadError, setUploadError] = useState("");
  const [uploadSuccess, setUploadSuccess] = useState("");
  const [uploading, setUploading] = useState(false);
  
  const pdfInputRef = useRef<HTMLInputElement>(null);
  const xmlInputRef = useRef<HTMLInputElement>(null);
  const zipInputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    loadEntry();
  }, [entryId]);

  const loadEntry = async () => {
    try {
      setLoading(true);
      setError("");
      const data = await purchaseEntriesApi.getById(entryId);
      setEntry(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Error al cargar la entrada");
    } finally {
      setLoading(false);
    }
  };

  const handleUploadPdf = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    if (file.type !== "application/pdf") {
      setUploadError("El archivo debe ser PDF");
      return;
    }

    try {
      setUploading(true);
      setUploadError("");
      setUploadSuccess("");
      await purchaseEntriesApi.uploadDocument(entryId, "pdf", file);
      setUploadSuccess("PDF subido correctamente");
      await loadEntry();
    } catch (err) {
      setUploadError(err instanceof Error ? err.message : "Error al subir PDF");
    } finally {
      setUploading(false);
      if (pdfInputRef.current) pdfInputRef.current.value = "";
    }
  };

  const handleUploadXml = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    if (
      file.type !== "text/xml" &&
      file.type !== "application/xml" &&
      !file.name.endsWith(".xml")
    ) {
      setUploadError("El archivo debe ser XML");
      return;
    }

    try {
      setUploading(true);
      setUploadError("");
      setUploadSuccess("");
      await purchaseEntriesApi.uploadDocument(entryId, "xml", file);
      setUploadSuccess("XML subido correctamente");
      await loadEntry();
    } catch (err) {
      setUploadError(err instanceof Error ? err.message : "Error al subir XML");
    } finally {
      setUploading(false);
      if (xmlInputRef.current) xmlInputRef.current.value = "";
    }
  };

  const handleUploadZip = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    if (
      file.type !== "application/zip" &&
      file.type !== "application/x-zip-compressed" &&
      !file.name.toLowerCase().endsWith(".zip")
    ) {
      setUploadError("El archivo debe ser ZIP");
      return;
    }

    try {
      setUploading(true);
      setUploadError("");
      setUploadSuccess("");
      await purchaseEntriesApi.uploadZipDocument(entryId, file);
      setUploadSuccess("ZIP subido correctamente (PDF y XML extraídos)");
      await loadEntry();
    } catch (err) {
      setUploadError(err instanceof Error ? err.message : "Error al subir ZIP");
    } finally {
      setUploading(false);
      if (zipInputRef.current) zipInputRef.current.value = "";
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
          maxWidth: "800px",
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
            Detalle de Entrada de Compra
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

        {/* Entry Details */}
        {entry && !loading && (
          <>
            {/* Summary */}
            <div
              style={{
                display: "grid",
                gridTemplateColumns: "repeat(auto-fit, minmax(200px, 1fr))",
                gap: "1.5rem",
                marginBottom: "2rem",
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
                  Proveedor
                </p>
                <p
                  style={{
                    margin: 0,
                    fontSize: "1.1rem",
                    fontWeight: "600",
                    color: "var(--color-text-primary)",
                  }}
                >
                  {entry.supplier_name || "Desconocido"}
                </p>
              </div>
              <div>
                <p
                  style={{
                    margin: "0 0 0.25rem 0",
                    color: "var(--color-text-muted)",
                    fontSize: "0.875rem",
                  }}
                >
                  Fecha de Entrada
                </p>
                <p
                  style={{
                    margin: 0,
                    fontSize: "1.1rem",
                    fontWeight: "600",
                    color: "var(--color-text-primary)",
                  }}
                >
                  {formatDate(entry.entry_date)}
                </p>
              </div>
              {entry.invoice_reference && (
                <div>
                  <p
                    style={{
                      margin: "0 0 0.25rem 0",
                      color: "var(--color-text-muted)",
                      fontSize: "0.875rem",
                    }}
                  >
                    Referencia de Factura
                  </p>
                  <p
                    style={{
                      margin: 0,
                      fontSize: "1.1rem",
                      fontWeight: "600",
                      color: "var(--color-text-primary)",
                      fontFamily: "monospace",
                    }}
                  >
                    {entry.invoice_reference}
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
                  Total
                </p>
                <p
                  style={{
                    margin: 0,
                    fontSize: "1.25rem",
                    fontWeight: "bold",
                    color: "var(--color-success)",
                  }}
                >
                  {formatCurrency(entry.total_amount)}
                </p>
              </div>
            </div>

            {/* Notes */}
            {entry.notes && (
              <div
                style={{
                  marginBottom: "2rem",
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
                  }}
                >
                  {entry.notes}
                </p>
              </div>
            )}

            {/* Documents Section */}
            <div
              style={{
                marginBottom: "2rem",
                padding: "1rem",
                backgroundColor: "var(--color-bg)",
                borderRadius: "var(--radius-sm)",
                border: "1px solid var(--color-border)",
              }}
            >
              <p
                style={{
                  margin: "0 0 1rem 0",
                  color: "var(--color-text-muted)",
                  fontSize: "0.875rem",
                  fontWeight: "600",
                }}
              >
                Documentos Adjuntos
              </p>

              {/* Current Documents */}
              {(entry.pdf_storage_path || entry.xml_storage_path) && (
                <div style={{ display: "flex", gap: "0.5rem", flexWrap: "wrap", marginBottom: "1rem" }}>
                  {entry.pdf_storage_path && (
                    <span
                      style={{
                        padding: "0.5rem 1rem",
                        backgroundColor: "var(--color-danger-light)",
                        color: "var(--color-danger)",
                        borderRadius: "var(--radius-sm)",
                        fontSize: "0.875rem",
                        fontWeight: "600",
                      }}
                    >
                      PDF adjunto
                    </span>
                  )}
                  {entry.xml_storage_path && (
                    <span
                      style={{
                        padding: "0.5rem 1rem",
                        backgroundColor: "var(--color-success-light)",
                        color: "var(--color-success)",
                        borderRadius: "var(--radius-sm)",
                        fontSize: "0.875rem",
                        fontWeight: "600",
                      }}
                    >
                      XML adjunto
                    </span>
                  )}
                </div>
              )}

              {/* Upload Messages */}
              {uploadError && (
                <div
                  style={{
                    padding: "0.75rem",
                    backgroundColor: "var(--color-danger-light)",
                    color: "var(--color-danger)",
                    border: "1px solid var(--color-danger)",
                    borderRadius: "var(--radius-sm)",
                    marginBottom: "1rem",
                    fontSize: "0.875rem",
                  }}
                >
                  {uploadError}
                </div>
              )}
              {uploadSuccess && (
                <div
                  style={{
                    padding: "0.75rem",
                    backgroundColor: "var(--color-success-light)",
                    color: "var(--color-success)",
                    border: "1px solid var(--color-success)",
                    borderRadius: "var(--radius-sm)",
                    marginBottom: "1rem",
                    fontSize: "0.875rem",
                  }}
                >
                  {uploadSuccess}
                </div>
              )}

              {/* Upload Section */}
              <PermissionGate permission={PERMISSIONS.PURCHASE_ENTRIES_UPLOAD}>
                <div
                  style={{
                    borderTop: entry.pdf_storage_path || entry.xml_storage_path ? "1px solid var(--color-border)" : "none",
                    paddingTop: entry.pdf_storage_path || entry.xml_storage_path ? "1rem" : "0",
                  }}
                >
                  <p
                    style={{
                      margin: "0 0 0.75rem 0",
                      color: "var(--color-text-secondary)",
                      fontSize: "0.875rem",
                    }}
                  >
                    {entry.pdf_storage_path && entry.xml_storage_path
                      ? "Reemplazar documentos:"
                      : "Subir documentos:"}
                  </p>

                  <div style={{ display: "flex", gap: "0.5rem", flexWrap: "wrap" }}>
                    {/* PDF Upload */}
                    <label
                      style={{
                        padding: "0.5rem 1rem",
                        backgroundColor: entry.pdf_storage_path ? "var(--color-surface-hover)" : "var(--color-danger)",
                        color: entry.pdf_storage_path ? "var(--color-text-primary)" : "white",
                        border: entry.pdf_storage_path ? "1px solid var(--color-border)" : "none",
                        borderRadius: "var(--radius-sm)",
                        cursor: uploading ? "not-allowed" : "pointer",
                        fontSize: "0.875rem",
                        fontWeight: "500",
                        opacity: uploading ? 0.6 : 1,
                      }}
                    >
                      {entry.pdf_storage_path ? "Reemplazar PDF" : "Subir PDF"}
                      <input
                        ref={pdfInputRef}
                        type="file"
                        accept=".pdf,application/pdf"
                        onChange={handleUploadPdf}
                        disabled={uploading}
                        style={{ display: "none" }}
                      />
                    </label>

                    {/* XML Upload */}
                    <label
                      style={{
                        padding: "0.5rem 1rem",
                        backgroundColor: entry.xml_storage_path ? "var(--color-surface-hover)" : "var(--color-success)",
                        color: entry.xml_storage_path ? "var(--color-text-primary)" : "white",
                        border: entry.xml_storage_path ? "1px solid var(--color-border)" : "none",
                        borderRadius: "var(--radius-sm)",
                        cursor: uploading ? "not-allowed" : "pointer",
                        fontSize: "0.875rem",
                        fontWeight: "500",
                        opacity: uploading ? 0.6 : 1,
                      }}
                    >
                      {entry.xml_storage_path ? "Reemplazar XML" : "Subir XML"}
                      <input
                        ref={xmlInputRef}
                        type="file"
                        accept=".xml,text/xml,application/xml"
                        onChange={handleUploadXml}
                        disabled={uploading}
                        style={{ display: "none" }}
                      />
                    </label>

                    {/* ZIP Upload */}
                    <label
                      style={{
                        padding: "0.5rem 1rem",
                        backgroundColor: "var(--color-primary)",
                        color: "white",
                        border: "none",
                        borderRadius: "var(--radius-sm)",
                        cursor: uploading ? "not-allowed" : "pointer",
                        fontSize: "0.875rem",
                        fontWeight: "500",
                        opacity: uploading ? 0.6 : 1,
                      }}
                    >
                      Subir ZIP
                      <input
                        ref={zipInputRef}
                        type="file"
                        accept=".zip,application/zip,application/x-zip-compressed"
                        onChange={handleUploadZip}
                        disabled={uploading}
                        style={{ display: "none" }}
                      />
                    </label>
                  </div>

                  {uploading && (
                    <p
                      style={{
                        margin: "0.75rem 0 0 0",
                        color: "var(--color-text-secondary)",
                        fontSize: "0.875rem",
                      }}
                    >
                      Subiendo documento...
                    </p>
                  )}

                  <p
                    style={{
                      margin: "0.75rem 0 0 0",
                      color: "var(--color-text-muted)",
                      fontSize: "0.75rem",
                    }}
                  >
                    El ZIP debe contener exactamente 1 PDF y 1 XML.
                  </p>
                </div>
              </PermissionGate>

              {/* No documents message */}
              {!entry.pdf_storage_path && !entry.xml_storage_path && (
                <p
                  style={{
                    margin: "0 0 1rem 0",
                    color: "var(--color-text-muted)",
                    fontSize: "0.875rem",
                    fontStyle: "italic",
                  }}
                >
                  No hay documentos adjuntos.
                </p>
              )}
            </div>

            {/* Items Table */}
            <h3
              style={{
                margin: "0 0 1rem 0",
                fontSize: "1.1rem",
                fontWeight: "600",
                color: "var(--color-text-primary)",
              }}
            >
              Productos Recibidos
            </h3>
            {entry.items && entry.items.length > 0 ? (
              <table
                style={{
                  width: "100%",
                  borderCollapse: "collapse",
                }}
              >
                <thead>
                  <tr
                    style={{
                      borderBottom: "2px solid var(--color-border)",
                    }}
                  >
                    <th
                      style={{
                        textAlign: "left",
                        padding: "0.75rem",
                        color: "var(--color-text-secondary)",
                        fontSize: "0.875rem",
                        fontWeight: "600",
                      }}
                    >
                      Producto
                    </th>
                    <th
                      style={{
                        textAlign: "right",
                        padding: "0.75rem",
                        color: "var(--color-text-secondary)",
                        fontSize: "0.875rem",
                        fontWeight: "600",
                      }}
                    >
                      Cantidad
                    </th>
                    <th
                      style={{
                        textAlign: "right",
                        padding: "0.75rem",
                        color: "var(--color-text-secondary)",
                        fontSize: "0.875rem",
                        fontWeight: "600",
                      }}
                    >
                      Costo Unit.
                    </th>
                    <th
                      style={{
                        textAlign: "right",
                        padding: "0.75rem",
                        color: "var(--color-text-secondary)",
                        fontSize: "0.875rem",
                        fontWeight: "600",
                      }}
                    >
                      Total
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {entry.items.map((item) => (
                    <tr
                      key={item.id}
                      style={{
                        borderBottom: "1px solid var(--color-border)",
                      }}
                    >
                      <td
                        style={{
                          padding: "0.75rem",
                          color: "var(--color-text-primary)",
                        }}
                      >
                        {item.product_name || item.product_id}
                      </td>
                      <td
                        style={{
                          padding: "0.75rem",
                          color: "var(--color-text-primary)",
                          textAlign: "right",
                        }}
                      >
                        {item.quantity}
                      </td>
                      <td
                        style={{
                          padding: "0.75rem",
                          color: "var(--color-text-primary)",
                          textAlign: "right",
                        }}
                      >
                        {formatCurrency(item.unit_cost)}
                      </td>
                      <td
                        style={{
                          padding: "0.75rem",
                          color: "var(--color-text-primary)",
                          textAlign: "right",
                          fontWeight: "600",
                        }}
                      >
                        {formatCurrency(item.total_cost)}
                      </td>
                    </tr>
                  ))}
                </tbody>
                <tfoot>
                  <tr
                    style={{
                      borderTop: "2px solid var(--color-border)",
                    }}
                  >
                    <td
                      colSpan={3}
                      style={{
                        padding: "0.75rem",
                        textAlign: "right",
                        fontWeight: "600",
                        color: "var(--color-text-primary)",
                      }}
                    >
                      Total General:
                    </td>
                    <td
                      style={{
                        padding: "0.75rem",
                        textAlign: "right",
                        fontWeight: "bold",
                        fontSize: "1.1rem",
                        color: "var(--color-success)",
                      }}
                    >
                      {formatCurrency(entry.total_amount)}
                    </td>
                  </tr>
                </tfoot>
              </table>
            ) : (
              <p style={{ color: "var(--color-text-secondary)" }}>
                No hay items en esta entrada.
              </p>
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
