"use client";

import { useState, useEffect, useMemo } from "react";
import OpenBillCard from "./OpenBillCard";
import OpenBillSearch from "./OpenBillSearch";
import CreateOrderForm from "./CreateOrderForm";
import EditOrderForm from "./EditOrderForm";
import PaymentModal from "./PaymentModal";
import { getOpenBills, getOpenBillById } from "@/lib/api/orders";
import type { OpenBill, OpenBillWithProducts } from "@/types/order";

export default function OrdersPageClient() {
  const [openBills, setOpenBills] = useState<OpenBill[]>([]);
  const [searchQuery, setSearchQuery] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [editingBill, setEditingBill] = useState<OpenBillWithProducts | null>(
    null
  );
  const [paymentBill, setPaymentBill] = useState<OpenBillWithProducts | null>(
    null
  );
  const [isLoadingBill, setIsLoadingBill] = useState(false);

  const fetchOpenBills = async () => {
    setIsLoading(true);
    setError(null);
    try {
      const response = await getOpenBills();
      setOpenBills(response.open_bills || []);
    } catch (err) {
      console.error("Error fetching open bills:", err);
      setError(
        err instanceof Error ? err.message : "Failed to load open bills"
      );
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    fetchOpenBills();
  }, []);

  // Filter open bills based on search query (by temporal_identifier)
  const filteredOpenBills = useMemo(() => {
    let bills = openBills;

    // Filter by search query if provided
    if (searchQuery.trim()) {
      const query = searchQuery.toLowerCase();
      bills = bills.filter((bill) =>
        bill.temporal_identifier.toLowerCase().includes(query)
      );
    }

    // Sort by temporal_identifier
    return [...bills].sort((a, b) =>
      a.temporal_identifier.localeCompare(b.temporal_identifier)
    );
  }, [openBills, searchQuery]);

  const handleCreateSuccess = () => {
    fetchOpenBills();
  };

  const handleBillClick = async (bill: OpenBill) => {
    setIsLoadingBill(true);
    setError(null);
    try {
      const fullBill = await getOpenBillById(bill.id);
      setEditingBill(fullBill);
    } catch (err) {
      console.error("Error fetching bill details:", err);
      setError(
        err instanceof Error ? err.message : "Failed to load bill details"
      );
    } finally {
      setIsLoadingBill(false);
    }
  };

  const handlePayClick = async (bill: OpenBill) => {
    setIsLoadingBill(true);
    setError(null);
    try {
      const fullBill = await getOpenBillById(bill.id);
      setPaymentBill(fullBill);
    } catch (err) {
      console.error("Error fetching bill details:", err);
      setError(
        err instanceof Error ? err.message : "Failed to load bill details"
      );
    } finally {
      setIsLoadingBill(false);
    }
  };

  return (
    <div
      style={{
        minHeight: "100vh",
        backgroundColor: "var(--color-bg)",
        padding: "1rem",
      }}
    >
      <div
        style={{
          maxWidth: "1400px",
          margin: "0 auto",
        }}
      >
        {/* Header */}
        <div
          style={{
            display: "flex",
            justifyContent: "space-between",
            alignItems: "center",
            marginBottom: "1.5rem",
            flexWrap: "wrap",
            gap: "1rem",
          }}
        >
          <h1
            style={{
              fontSize: "clamp(1.75rem, 4vw, 2.5rem)",
              fontWeight: "bold",
              margin: 0,
              color: "var(--color-text-primary)",
            }}
          >
            Cuentas Abiertas
          </h1>
          <button
            onClick={() => setShowCreateForm(true)}
            style={{
              padding: "0.875rem 1.75rem",
              fontSize: "1.05rem",
              fontWeight: "bold",
              backgroundColor: "var(--color-primary)",
              color: "white",
              border: "none",
              borderRadius: "var(--radius-md)",
              cursor: "pointer",
              transition: "background-color var(--transition-normal)",
              boxShadow: "var(--shadow-md)",
            }}
            onMouseEnter={(e) => {
              e.currentTarget.style.backgroundColor = "var(--color-primary-hover)";
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.backgroundColor = "var(--color-primary)";
            }}
          >
            + Crear Nueva Orden
          </button>
        </div>

        {/* Search Bar */}
        <OpenBillSearch
          searchQuery={searchQuery}
          onSearchChange={setSearchQuery}
        />

        {/* Error Message */}
        {error && (
          <div
            style={{
              padding: "1rem",
              backgroundColor: "var(--color-danger-light)",
              border: "1px solid var(--color-danger)",
              borderRadius: "var(--radius-md)",
              color: "var(--color-danger)",
              marginBottom: "1.5rem",
              fontWeight: "500",
            }}
          >
            {error}
          </div>
        )}

        {/* Loading State */}
        {isLoading && (
          <div
            style={{
              textAlign: "center",
              padding: "4rem",
              color: "var(--color-text-secondary)",
              fontSize: "1.1rem",
            }}
          >
            <div
              style={{
                display: "inline-block",
                width: "40px",
                height: "40px",
                border: "4px solid var(--color-border)",
                borderTop: "4px solid var(--color-primary)",
                borderRadius: "50%",
                animation: "spin 1s linear infinite",
                marginBottom: "1rem",
              }}
            />
            <div>Cargando cuentas abiertas...</div>
          </div>
        )}

        {/* Open Bills Grid */}
        {!isLoading && (
          <>
            {filteredOpenBills.length === 0 ? (
              <div
                style={{
                  textAlign: "center",
                  padding: "4rem",
                  backgroundColor: "var(--color-surface)",
                  borderRadius: "var(--radius-lg)",
                  border: "2px dashed var(--color-border)",
                }}
              >
                <div
                  style={{
                    fontSize: "3rem",
                    marginBottom: "1rem",
                  }}
                >
                  📋
                </div>
                <h3
                  style={{
                    margin: "0 0 0.5rem 0",
                    fontSize: "1.25rem",
                    color: "var(--color-text-primary)",
                  }}
                >
                  {searchQuery
                    ? "No se encontraron cuentas coincidentes"
                    : "Aún no hay cuentas abiertas"}
                </h3>
                <p
                  style={{
                    margin: 0,
                    color: "var(--color-text-secondary)",
                    fontSize: "1rem",
                  }}
                >
                  {searchQuery
                    ? "Intenta ajustar tu búsqueda"
                    : 'Haz clic en "Crear Nueva Orden" para comenzar'}
                </p>
              </div>
            ) : (
              <>
                <div
                  style={{
                    marginBottom: "1rem",
                    color: "var(--color-text-secondary)",
                    fontSize: "0.95rem",
                  }}
                >
                  Mostrando {filteredOpenBills.length}{" "}
                  {filteredOpenBills.length === 1 ? "cuenta" : "cuentas"}
                  {searchQuery && ` matching "${searchQuery}"`}
                </div>
                <div
                  style={{
                    display: "grid",
                    gridTemplateColumns:
                      "repeat(auto-fill, minmax(min(320px, 100%), 1fr))",
                    gap: "1.25rem",
                  }}
                >
                  {filteredOpenBills.map((bill) => (
                    <OpenBillCard
                      key={bill.id}
                      openBill={bill}
                      onClick={() => handleBillClick(bill)}
                      onPayClick={() => handlePayClick(bill)}
                    />
                  ))}
                </div>
              </>
            )}
          </>
        )}
      </div>

      {/* Create Order Form Modal */}
      {showCreateForm && (
        <CreateOrderForm
          onClose={() => setShowCreateForm(false)}
          onSuccess={handleCreateSuccess}
        />
      )}

      {/* Edit Order Form Modal */}
      {editingBill && (
        <EditOrderForm
          openBill={editingBill}
          onClose={() => setEditingBill(null)}
          onSuccess={handleCreateSuccess}
        />
      )}

      {/* Payment Modal */}
      {paymentBill && (
        <PaymentModal
          openBill={paymentBill}
          onClose={() => setPaymentBill(null)}
          onSuccess={handleCreateSuccess}
        />
      )}

      {/* Loading overlay when fetching bill details */}
      {isLoadingBill && (
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
            zIndex: 999,
          }}
        >
          <div
            style={{
              backgroundColor: "var(--color-surface)",
              padding: "2rem",
              borderRadius: "var(--radius-lg)",
              display: "flex",
              flexDirection: "column",
              alignItems: "center",
              gap: "1rem",
              border: "1px solid var(--color-border)",
            }}
          >
            <div
              style={{
                width: "40px",
                height: "40px",
                border: "4px solid var(--color-border)",
                borderTop: "4px solid var(--color-primary)",
                borderRadius: "50%",
                animation: "spin 1s linear infinite",
              }}
            />
            <div style={{ color: "var(--color-text-primary)", fontWeight: "500" }}>
              Cargando detalles de la orden...
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
