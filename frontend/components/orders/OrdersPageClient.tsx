"use client";

import { useState, useEffect, useMemo, useRef } from "react";
import OpenBillCard from "./OpenBillCard";
import OpenBillSearch from "./OpenBillSearch";
import OrderViewToggle from "./OrderViewToggle";
import WaitressOrderGroup from "./WaitressOrderGroup";
import CreateOrderForm from "./CreateOrderForm";
import EditOrderForm from "./EditOrderForm";
import PaymentModal from "./PaymentModal";
import ClosedOrdersPanel from "./ClosedOrdersPanel";
import {
  getOpenBills,
  getOpenBillById,
  removeOpenBill,
} from "@/lib/api/orders";
import type { OpenBill, OpenBillWithProducts } from "@/types/order";
import { PermissionGate } from "@/components/permissions";
import { PinConfirmModal } from "@/components/ui";
import { isOrderActionPinRequired } from "@/lib/orders/actionPin";
import { PERMISSIONS, usePermissions } from "@/lib/permissions";
import {
  type OrderViewMode,
  billMatchesQuery,
  canDefaultToAllOrders,
  groupBillsByWaitress,
  isMyBill,
  sortBillsByRecency,
} from "@/lib/orders/grouping";
import { Button } from "@/components/ui";

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
  // Bill awaiting PIN-confirmed removal. Removing is irreversible, so a single
  // tap must never delete a bill — the confirm gate holds it here first.
  const [billPendingRemoval, setBillPendingRemoval] = useState<OpenBill | null>(
    null
  );
  const [isRemoving, setIsRemoving] = useState(false);
  const [viewMode, setViewMode] = useState<OrderViewMode>("mine");
  // Primary tab: live open bills vs. today's closed (paid) bills.
  const [mainView, setMainView] = useState<"open" | "closed">("open");

  const { user } = usePermissions();
  const defaultViewApplied = useRef(false);

  // Pick the initial view once the user resolves: admin/manager land on the
  // whole floor ("Todas"), everyone else on their own orders ("Mis órdenes").
  // Runs once so it never overrides a manual toggle.
  useEffect(() => {
    if (!defaultViewApplied.current && user) {
      defaultViewApplied.current = true;
      if (canDefaultToAllOrders(user)) {
        setViewMode("all");
      }
    }
  }, [user]);

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

  // Bills matching the search query (by table identifier OR waitress name).
  const searchedBills = useMemo(
    () => openBills.filter((bill) => billMatchesQuery(bill, searchQuery)),
    [openBills, searchQuery]
  );

  // "Mis órdenes" view: my bills only, sorted by identifier.
  const myBills = useMemo(
    () => sortBillsByRecency(searchedBills.filter((bill) => isMyBill(bill, user))),
    [searchedBills, user]
  );

  // "Todas" view: sections per waitress, my section pinned first.
  const waitressGroups = useMemo(
    () => groupBillsByWaitress(searchedBills, user),
    [searchedBills, user]
  );

  // Overview counts for the toggle (independent of the active search).
  const totalMineCount = useMemo(
    () => openBills.filter((bill) => isMyBill(bill, user)).length,
    [openBills, user]
  );

  const visibleBills = viewMode === "mine" ? myBills : searchedBills;

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

  const removeBill = async (bill: OpenBill) => {
    setIsRemoving(true);
    setError(null);
    try {
      await removeOpenBill(bill.id);
      setOpenBills((prev) => prev.filter((b) => b.id !== bill.id));
      setBillPendingRemoval(null);
      // If the bill was being edited, close that modal too.
      setEditingBill((current) =>
        current && current.id === bill.id ? null : current
      );
    } catch (err) {
      console.error("Error removing bill:", err);
      setError(err instanceof Error ? err.message : "Failed to remove bill");
      setBillPendingRemoval(null);
    } finally {
      setIsRemoving(false);
    }
  };

  // With a PIN configured (edge), stage the bill and open the confirm gate;
  // without one (cloud), remove immediately — no extra validation.
  const handleRemoveClick = (bill: OpenBill) => {
    if (isOrderActionPinRequired) {
      setBillPendingRemoval(bill);
    } else {
      removeBill(bill);
    }
  };

  const confirmRemoval = () => {
    if (billPendingRemoval) removeBill(billPendingRemoval);
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
            {mainView === "open" ? "Cuentas Abiertas" : "Órdenes Cerradas Hoy"}
          </h1>
          {mainView === "open" && (
            <PermissionGate permission={PERMISSIONS.ORDERS_CREATE}>
              <Button
                size="lg"
                onClick={() => setShowCreateForm(true)}
                style={{ boxShadow: "var(--shadow-md)" }}
              >
                + Crear Nueva Orden
              </Button>
            </PermissionGate>
          )}
        </div>

        {/* Primary tabs: open bills vs. today's closed bills */}
        <div
          role="tablist"
          aria-label="Abiertas o cerradas hoy"
          style={{
            display: "flex",
            gap: "0.25rem",
            padding: "0.25rem",
            backgroundColor: "var(--color-surface)",
            border: "1px solid var(--color-border)",
            borderRadius: "var(--radius-lg)",
            maxWidth: "420px",
            marginBottom: "1rem",
          }}
        >
          {(["open", "closed"] as const).map((mode) => {
            const active = mainView === mode;
            return (
              <button
                key={mode}
                type="button"
                role="tab"
                aria-selected={active}
                onClick={() => setMainView(mode)}
                style={{
                  flex: 1,
                  minHeight: "44px",
                  padding: "0.5rem 1rem",
                  border: "none",
                  borderRadius: "var(--radius-md)",
                  cursor: "pointer",
                  fontSize: "0.95rem",
                  fontWeight: 600,
                  backgroundColor: active
                    ? "var(--color-primary)"
                    : "transparent",
                  color: active
                    ? "var(--color-on-primary, #fff)"
                    : "var(--color-text-secondary)",
                  whiteSpace: "nowrap",
                }}
              >
                {mode === "open" ? "Abiertas" : "Cerradas hoy"}
              </button>
            );
          })}
        </div>

        {mainView === "closed" && <ClosedOrdersPanel />}

        {mainView === "open" && (
          <>
            {/* View toggle: my orders vs. the whole floor */}
            <div style={{ marginBottom: "1rem" }}>
              <OrderViewToggle
                value={viewMode}
                onChange={setViewMode}
                mineCount={totalMineCount}
                allCount={openBills.length}
              />
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

        {/* Open Bills */}
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
                    : viewMode === "mine"
                      ? "No tienes órdenes abiertas"
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
                  Mostrando {visibleBills.length}{" "}
                  {visibleBills.length === 1 ? "cuenta" : "cuentas"}
                  {searchQuery && ` que coinciden con "${searchQuery}"`}
                </div>
                {viewMode === "mine" ? (
                  <div
                    style={{
                      display: "grid",
                      gridTemplateColumns:
                        "repeat(auto-fill, minmax(min(320px, 100%), 1fr))",
                      gap: "1.25rem",
                    }}
                  >
                    {myBills.map((bill) => (
                      <OpenBillCard
                        key={bill.id}
                        openBill={bill}
                        isMine
                        onClick={() => handleBillClick(bill)}
                        onPayClick={() => handlePayClick(bill)}
                        onRemoveClick={() => handleRemoveClick(bill)}
                      />
                    ))}
                  </div>
                ) : (
                  waitressGroups.map((group) => (
                    <WaitressOrderGroup
                      key={group.key}
                      group={group}
                      onBillClick={handleBillClick}
                      onPayClick={handlePayClick}
                      onRemoveClick={handleRemoveClick}
                    />
                  ))
                )}
              </>
            )}
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
          onPayClick={() => setPaymentBill(editingBill)}
          onRemoveClick={() => handleRemoveClick(editingBill)}
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

      {/* Confirm + PIN gate before removing a bill (irreversible) */}
      <PinConfirmModal
        open={!!billPendingRemoval}
        onClose={() => setBillPendingRemoval(null)}
        onConfirm={confirmRemoval}
        isProcessing={isRemoving}
        title="Eliminar cuenta"
        confirmLabel="Eliminar"
        confirmVariant="danger"
        message={
          billPendingRemoval
            ? `Vas a eliminar la cuenta "${billPendingRemoval.temporal_identifier}". Esta acción no se puede deshacer.`
            : undefined
        }
      />

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
            <div
              style={{ color: "var(--color-text-primary)", fontWeight: "500" }}
            >
              Cargando detalles de la orden...
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
