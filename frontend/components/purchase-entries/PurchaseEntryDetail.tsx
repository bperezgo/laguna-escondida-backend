"use client";

import { useState, useEffect } from "react";
import type { PurchaseEntry } from "@/types/purchaseEntry";
import { purchaseEntriesApi } from "@/lib/api/purchaseEntries";

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
