"use client";

import { useState, useEffect } from "react";
import { invoicesApi } from "@/lib/api/invoices";
import type {
  CreateElectronicInvoiceRequest,
  InvoiceListItem,
  InvoiceFilters,
} from "@/types/invoice";
import InvoiceForm from "@/components/invoices/InvoiceForm";

export default function InvoicesPageClient() {
  const [showForm, setShowForm] = useState<boolean>(false);
  const [formLoading, setFormLoading] = useState<boolean>(false);
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

  // Handle document click - open PDF in new tab
  const handleDocumentClick = (url: string) => {
    window.open(url, "_blank", "noopener,noreferrer");
  };

  // Format date and time
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
        backgroundColor: "#f5f5f5",
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
                color: "#333",
              }}
            >
              Electronic Invoices
            </h1>
            <p
              style={{
                margin: "0.5rem 0 0 0",
                color: "#666",
                fontSize: "1rem",
              }}
            >
              Create and manage electronic invoices
            </p>
          </div>
          <button
            onClick={handleCreate}
            style={{
              padding: "0.75rem 1.5rem",
              backgroundColor: "#28a745",
              color: "white",
              border: "none",
              borderRadius: "4px",
              cursor: "pointer",
              fontSize: "1rem",
              fontWeight: "500",
              boxShadow: "0 2px 4px rgba(0,0,0,0.1)",
            }}
            onMouseEnter={(e) => {
              e.currentTarget.style.backgroundColor = "#218838";
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.backgroundColor = "#28a745";
            }}
          >
            + Create Invoice
          </button>
        </div>

        {/* Success Message */}
        {success && (
          <div
            style={{
              padding: "1rem",
              backgroundColor: "#d4edda",
              color: "#155724",
              border: "1px solid #c3e6cb",
              borderRadius: "4px",
              marginBottom: "1.5rem",
            }}
          >
            <strong>Success:</strong> {success}
          </div>
        )}

        {/* Error Message */}
        {error && (
          <div
            style={{
              padding: "1rem",
              backgroundColor: "#f8d7da",
              color: "#721c24",
              border: "1px solid #f5c6cb",
              borderRadius: "4px",
              marginBottom: "1.5rem",
            }}
          >
            <strong>Error:</strong> {error}
          </div>
        )}

        {/* Filters Section */}
        <div
          style={{
            backgroundColor: "white",
            padding: "1.5rem",
            borderRadius: "8px",
            border: "1px solid #e0e0e0",
            marginBottom: "1.5rem",
          }}
        >
          <h2
            style={{
              margin: "0 0 1rem 0",
              fontSize: "1.25rem",
              fontWeight: "bold",
              color: "#333",
            }}
          >
            Search Filters
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
                  color: "#555",
                }}
              >
                Start Date
              </label>
              <input
                type="date"
                value={createdAtStart}
                onChange={(e) => setCreatedAtStart(e.target.value)}
                style={{
                  width: "100%",
                  padding: "0.5rem",
                  border: "1px solid #ddd",
                  borderRadius: "4px",
                  fontSize: "1rem",
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
                  color: "#555",
                }}
              >
                End Date
              </label>
              <input
                type="date"
                value={createdAtEnd}
                onChange={(e) => setCreatedAtEnd(e.target.value)}
                style={{
                  width: "100%",
                  padding: "0.5rem",
                  border: "1px solid #ddd",
                  borderRadius: "4px",
                  fontSize: "1rem",
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
                  color: "#555",
                }}
              >
                National ID
              </label>
              <input
                type="text"
                value={nationalIdentification}
                onChange={(e) => setNationalIdentification(e.target.value)}
                placeholder="Enter ID number"
                style={{
                  width: "100%",
                  padding: "0.5rem",
                  border: "1px solid #ddd",
                  borderRadius: "4px",
                  fontSize: "1rem",
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
                backgroundColor: "#007bff",
                color: "white",
                border: "none",
                borderRadius: "4px",
                cursor: "pointer",
                fontSize: "1rem",
                fontWeight: "500",
              }}
              onMouseEnter={(e) => {
                e.currentTarget.style.backgroundColor = "#0056b3";
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.backgroundColor = "#007bff";
              }}
            >
              Apply Filters
            </button>
            <button
              onClick={handleClearFilters}
              style={{
                padding: "0.5rem 1.5rem",
                backgroundColor: "#6c757d",
                color: "white",
                border: "none",
                borderRadius: "4px",
                cursor: "pointer",
                fontSize: "1rem",
                fontWeight: "500",
              }}
              onMouseEnter={(e) => {
                e.currentTarget.style.backgroundColor = "#545b62";
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.backgroundColor = "#6c757d";
              }}
            >
              Clear Filters
            </button>
          </div>
        </div>

        {/* Invoice List */}
        {loading ? (
          <div
            style={{
              padding: "3rem",
              textAlign: "center",
              backgroundColor: "white",
              borderRadius: "8px",
              border: "1px solid #e0e0e0",
            }}
          >
            <p style={{ color: "#666", fontSize: "1rem", margin: 0 }}>
              Loading invoices...
            </p>
          </div>
        ) : invoices.length === 0 ? (
          <div
            style={{
              padding: "3rem",
              textAlign: "center",
              backgroundColor: "white",
              borderRadius: "8px",
              border: "1px solid #e0e0e0",
            }}
          >
            <p style={{ color: "#666", fontSize: "1rem", margin: 0 }}>
              No invoices found. Click "Create Invoice" to start creating a new
              electronic invoice.
            </p>
          </div>
        ) : (
          <>
            <div
              style={{
                backgroundColor: "white",
                borderRadius: "8px",
                border: "1px solid #e0e0e0",
                overflow: "hidden",
              }}
            >
              <div
                style={{
                  padding: "1rem 1.5rem",
                  borderBottom: "1px solid #e0e0e0",
                  backgroundColor: "#f8f9fa",
                }}
              >
                <p
                  style={{
                    margin: 0,
                    fontSize: "0.875rem",
                    color: "#666",
                  }}
                >
                  Showing {invoices.length} of {total} invoices
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
                        backgroundColor: "#f8f9fa",
                        borderBottom: "2px solid #e0e0e0",
                      }}
                    >
                      <th
                        style={{
                          padding: "1rem",
                          textAlign: "left",
                          fontSize: "0.875rem",
                          fontWeight: "bold",
                          color: "#333",
                        }}
                      >
                        Created At
                      </th>
                      <th
                        style={{
                          padding: "1rem",
                          textAlign: "left",
                          fontSize: "0.875rem",
                          fontWeight: "bold",
                          color: "#333",
                        }}
                      >
                        CUFE
                      </th>
                      <th
                        style={{
                          padding: "1rem",
                          textAlign: "left",
                          fontSize: "0.875rem",
                          fontWeight: "bold",
                          color: "#333",
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
                          color: "#333",
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
                          color: "#333",
                        }}
                      >
                        Discount
                      </th>
                      <th
                        style={{
                          padding: "1rem",
                          textAlign: "right",
                          fontSize: "0.875rem",
                          fontWeight: "bold",
                          color: "#333",
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
                          color: "#333",
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
                          color: "#333",
                        }}
                      >
                        Tip
                      </th>
                      <th
                        style={{
                          padding: "1rem",
                          textAlign: "center",
                          fontSize: "0.875rem",
                          fontWeight: "bold",
                          color: "#333",
                        }}
                      >
                        Document
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    {invoices.map((invoice, index) => (
                      <tr
                        key={invoice.id}
                        style={{
                          borderBottom: "1px solid #e0e0e0",
                          backgroundColor:
                            index % 2 === 0 ? "white" : "#f8f9fa",
                        }}
                      >
                        <td
                          style={{
                            padding: "1rem",
                            fontSize: "0.875rem",
                            color: "#333",
                          }}
                        >
                          {formatDateTime(invoice.created_at)}
                        </td>
                        <td
                          style={{
                            padding: "1rem",
                            fontSize: "0.75rem",
                            color: "#333",
                            maxWidth: "150px",
                            overflow: "hidden",
                            textOverflow: "ellipsis",
                            whiteSpace: "nowrap",
                          }}
                          title={invoice.cufe}
                        >
                          {invoice.cufe}
                        </td>
                        <td
                          style={{
                            padding: "1rem",
                            fontSize: "0.875rem",
                            color: "#333",
                          }}
                        >
                          {invoice.tascode}
                        </td>
                        <td
                          style={{
                            padding: "1rem",
                            fontSize: "0.875rem",
                            fontWeight: "bold",
                            color: "#28a745",
                            textAlign: "right",
                          }}
                        >
                          ${invoice.total_amount.toFixed(2)}
                        </td>
                        <td
                          style={{
                            padding: "1rem",
                            fontSize: "0.875rem",
                            color: "#333",
                            textAlign: "right",
                          }}
                        >
                          ${invoice.discount_amount.toFixed(2)}
                        </td>
                        <td
                          style={{
                            padding: "1rem",
                            fontSize: "0.875rem",
                            color: "#333",
                            textAlign: "right",
                          }}
                        >
                          ${invoice.vat.toFixed(2)}
                        </td>
                        <td
                          style={{
                            padding: "1rem",
                            fontSize: "0.875rem",
                            color: "#333",
                            textAlign: "right",
                          }}
                        >
                          ${invoice.ico.toFixed(2)}
                        </td>
                        <td
                          style={{
                            padding: "1rem",
                            fontSize: "0.875rem",
                            color: "#333",
                            textAlign: "right",
                          }}
                        >
                          ${invoice.tip.toFixed(2)}
                        </td>
                        <td
                          style={{
                            padding: "1rem",
                            textAlign: "center",
                          }}
                        >
                          {invoice.document_url ? (
                            <button
                              onClick={() =>
                                handleDocumentClick(invoice.document_url!)
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
                              title="View document (PDF)"
                            >
                              <svg
                                width="24"
                                height="24"
                                viewBox="0 0 24 24"
                                fill="none"
                                stroke="#28a745"
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
                              title="Document not available"
                            >
                              <svg
                                width="24"
                                height="24"
                                viewBox="0 0 24 24"
                                fill="none"
                                stroke="#dc3545"
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
                                  stroke="#dc3545"
                                />
                                <line
                                  x1="15"
                                  y1="12"
                                  x2="9"
                                  y2="18"
                                  stroke="#dc3545"
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
                    backgroundColor: currentPage === 1 ? "#e0e0e0" : "#007bff",
                    color: currentPage === 1 ? "#999" : "white",
                    border: "none",
                    borderRadius: "4px",
                    cursor: currentPage === 1 ? "not-allowed" : "pointer",
                    fontSize: "0.875rem",
                    fontWeight: "500",
                  }}
                >
                  Previous
                </button>
                <span
                  style={{
                    padding: "0.5rem 1rem",
                    fontSize: "0.875rem",
                    color: "#333",
                  }}
                >
                  Page {currentPage} of {totalPages}
                </span>
                <button
                  onClick={() => handlePageChange(currentPage + 1)}
                  disabled={currentPage === totalPages}
                  style={{
                    padding: "0.5rem 1rem",
                    backgroundColor:
                      currentPage === totalPages ? "#e0e0e0" : "#007bff",
                    color: currentPage === totalPages ? "#999" : "white",
                    border: "none",
                    borderRadius: "4px",
                    cursor:
                      currentPage === totalPages ? "not-allowed" : "pointer",
                    fontSize: "0.875rem",
                    fontWeight: "500",
                  }}
                >
                  Next
                </button>
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
