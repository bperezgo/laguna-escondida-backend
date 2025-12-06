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
        backgroundColor: "#f5f5f5",
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
              color: "#333",
            }}
          >
            Open Bills
          </h1>
          <button
            onClick={() => setShowCreateForm(true)}
            style={{
              padding: "0.875rem 1.75rem",
              fontSize: "1.05rem",
              fontWeight: "bold",
              backgroundColor: "#007bff",
              color: "white",
              border: "none",
              borderRadius: "8px",
              cursor: "pointer",
              transition: "background-color 0.2s",
              boxShadow: "0 2px 6px rgba(0, 123, 255, 0.3)",
            }}
            onMouseEnter={(e) => {
              e.currentTarget.style.backgroundColor = "#0056b3";
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.backgroundColor = "#007bff";
            }}
          >
            + Create New Order
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
              backgroundColor: "#fee",
              border: "2px solid #fcc",
              borderRadius: "8px",
              color: "#c00",
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
              color: "#666",
              fontSize: "1.1rem",
            }}
          >
            <div
              style={{
                display: "inline-block",
                width: "40px",
                height: "40px",
                border: "4px solid #e0e0e0",
                borderTop: "4px solid #007bff",
                borderRadius: "50%",
                animation: "spin 1s linear infinite",
                marginBottom: "1rem",
              }}
            />
            <div>Loading open bills...</div>
            <style>{`
              @keyframes spin {
                0% { transform: rotate(0deg); }
                100% { transform: rotate(360deg); }
              }
            `}</style>
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
                  backgroundColor: "white",
                  borderRadius: "12px",
                  border: "2px dashed #e0e0e0",
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
                    color: "#333",
                  }}
                >
                  {searchQuery
                    ? "No matching open bills found"
                    : "No open bills yet"}
                </h3>
                <p
                  style={{
                    margin: 0,
                    color: "#666",
                    fontSize: "1rem",
                  }}
                >
                  {searchQuery
                    ? "Try adjusting your search query"
                    : 'Click "Create New Order" to get started'}
                </p>
              </div>
            ) : (
              <>
                <div
                  style={{
                    marginBottom: "1rem",
                    color: "#666",
                    fontSize: "0.95rem",
                  }}
                >
                  Showing {filteredOpenBills.length}{" "}
                  {filteredOpenBills.length === 1 ? "bill" : "bills"}
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
            backgroundColor: "rgba(0, 0, 0, 0.5)",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            zIndex: 999,
          }}
        >
          <div
            style={{
              backgroundColor: "white",
              padding: "2rem",
              borderRadius: "12px",
              display: "flex",
              flexDirection: "column",
              alignItems: "center",
              gap: "1rem",
            }}
          >
            <div
              style={{
                width: "40px",
                height: "40px",
                border: "4px solid #e0e0e0",
                borderTop: "4px solid #007bff",
                borderRadius: "50%",
                animation: "spin 1s linear infinite",
              }}
            />
            <div style={{ color: "#333", fontWeight: "500" }}>
              Loading order details...
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
