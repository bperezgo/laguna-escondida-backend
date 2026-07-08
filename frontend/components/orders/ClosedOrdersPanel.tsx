"use client";

import { useEffect, useMemo, useState } from "react";
import OpenBillCard from "./OpenBillCard";
import OpenBillSearch from "./OpenBillSearch";
import ClosedOrderDetailModal from "./ClosedOrderDetailModal";
import { getClosedBills, getClosedBillById } from "@/lib/api/orders";
import type { OpenBill, OpenBillWithProducts } from "@/types/order";
import { billMatchesQuery, sortBillsByRecency } from "@/lib/orders/grouping";

/**
 * Today's closed (paid) orders, shown inside the Orders page under the "Cerradas hoy"
 * tab. Read-only: tap a card to view the itemized cuenta and reprint it. Fetches on
 * mount, i.e. lazily when the user first switches to this tab.
 */
export default function ClosedOrdersPanel() {
  const [closedBills, setClosedBills] = useState<OpenBill[]>([]);
  const [searchQuery, setSearchQuery] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [detailBill, setDetailBill] = useState<OpenBillWithProducts | null>(
    null,
  );
  const [isLoadingBill, setIsLoadingBill] = useState(false);

  const fetchClosedBills = async () => {
    setIsLoading(true);
    setError(null);
    try {
      const response = await getClosedBills();
      setClosedBills(response.open_bills || []);
    } catch (err) {
      console.error("Error fetching closed bills:", err);
      setError(
        err instanceof Error ? err.message : "Failed to load closed bills",
      );
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    fetchClosedBills();
  }, []);

  const visibleBills = useMemo(
    () =>
      sortBillsByRecency(
        closedBills.filter((bill) => billMatchesQuery(bill, searchQuery)),
      ),
    [closedBills, searchQuery],
  );

  const handleBillClick = async (bill: OpenBill) => {
    setIsLoadingBill(true);
    setError(null);
    try {
      const fullBill = await getClosedBillById(bill.id);
      setDetailBill(fullBill);
    } catch (err) {
      console.error("Error fetching closed bill details:", err);
      setError(
        err instanceof Error ? err.message : "Failed to load bill details",
      );
    } finally {
      setIsLoadingBill(false);
    }
  };

  return (
    <>
      {/* Search Bar */}
      <OpenBillSearch searchQuery={searchQuery} onSearchChange={setSearchQuery} />

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
          <div>Cargando órdenes cerradas...</div>
        </div>
      )}

      {/* Closed Bills */}
      {!isLoading && (
        <>
          {visibleBills.length === 0 ? (
            <div
              style={{
                textAlign: "center",
                padding: "4rem",
                backgroundColor: "var(--color-surface)",
                borderRadius: "var(--radius-lg)",
                border: "2px dashed var(--color-border)",
              }}
            >
              <div style={{ fontSize: "3rem", marginBottom: "1rem" }}>🧾</div>
              <h3
                style={{
                  margin: "0 0 0.5rem 0",
                  fontSize: "1.25rem",
                  color: "var(--color-text-primary)",
                }}
              >
                {searchQuery
                  ? "No se encontraron cuentas coincidentes"
                  : "Aún no hay órdenes cerradas hoy"}
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
                  : "Las órdenes pagadas hoy aparecerán aquí"}
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
                Mostrando {visibleBills.length}{" "}
                {visibleBills.length === 1 ? "cuenta" : "cuentas"}
                {searchQuery && ` que coinciden con "${searchQuery}"`}
              </div>
              <div
                style={{
                  display: "grid",
                  gridTemplateColumns:
                    "repeat(auto-fill, minmax(min(320px, 100%), 1fr))",
                  gap: "1.25rem",
                }}
              >
                {visibleBills.map((bill) => (
                  <OpenBillCard
                    key={bill.id}
                    openBill={bill}
                    onClick={() => handleBillClick(bill)}
                  />
                ))}
              </div>
            </>
          )}
        </>
      )}

      {/* Closed Order Detail / Reprint Modal */}
      {detailBill && (
        <ClosedOrderDetailModal
          openBill={detailBill}
          onClose={() => setDetailBill(null)}
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
    </>
  );
}
