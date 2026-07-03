"use client";

import { useState, useEffect, useRef, useCallback, useMemo } from "react";
import CommandItemCard from "./CommandItemCard";
import { completeOpenBillProduct } from "@/lib/api/openBillProducts";
import type { OpenBillProductFromSSE } from "@/types/commandItem";
import { calculateRemainingMs } from "@/lib/kitchen/countdown";
import { useNow } from "@/lib/kitchen/useNow";

const PINNED_STORAGE_KEY = "pinned-command-items";

export default function KitchenCommandItemsView() {
  const now = useNow();
  const [items, setItems] = useState<OpenBillProductFromSSE[]>([]);
  const [pinnedIds, setPinnedIds] = useState<Set<string>>(new Set());
  const [isConnecting, setIsConnecting] = useState(true);
  const [connectionError, setConnectionError] = useState<string | null>(null);
  const [completingIds, setCompletingIds] = useState<Set<string>>(new Set());
  const eventSourceRef = useRef<EventSource | null>(null);
  const reconnectTimeoutRef = useRef<NodeJS.Timeout | null>(null);

  // Load pinned IDs from localStorage on mount
  useEffect(() => {
    try {
      const stored = localStorage.getItem(PINNED_STORAGE_KEY);
      if (stored) {
        const parsed = JSON.parse(stored);
        if (Array.isArray(parsed)) {
          setPinnedIds(new Set(parsed));
        }
      }
    } catch (error) {
      console.error("Error loading pinned items from localStorage:", error);
    }
  }, []);

  const savePinnedIds = useCallback((ids: Set<string>) => {
    try {
      localStorage.setItem(PINNED_STORAGE_KEY, JSON.stringify(Array.from(ids)));
    } catch (error) {
      console.error("Error saving pinned items to localStorage:", error);
    }
  }, []);

  // SSE connection
  useEffect(() => {
    let isMounted = true;

    const connectToSSE = () => {
      if (eventSourceRef.current) {
        eventSourceRef.current.close();
      }
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current);
      }

      setIsConnecting(true);
      setConnectionError(null);

      const eventSource = new EventSource("/api/sse/open-bill-products/kitchen");
      eventSourceRef.current = eventSource;

      eventSource.onopen = () => {
        if (isMounted) {
          setIsConnecting(false);
          setConnectionError(null);
        }
      };

      const handleCreatedItem = (event: MessageEvent) => {
        try {
          const item: OpenBillProductFromSSE = JSON.parse(event.data);

          // The kitchen feed now includes completed lines (struck-through in the
          // grouped "Comandas" view). The individual view only shows pending work,
          // so ignore any finalized lines that arrive in the snapshot.
          if (item.status === "completed" || item.status === "cancelled") {
            return;
          }

          setItems((prev) => {
            const existingIndex = prev.findIndex(
              (i) => i.open_bill_product_id === item.open_bill_product_id
            );

            if (existingIndex >= 0) {
              const updated = [...prev];
              updated[existingIndex] = item;
              return updated;
            } else {
              return [...prev, item];
            }
          });
        } catch (error) {
          console.error("Error parsing open_bill_product.created event:", error);
        }
      };

      const handleUpdatedItem = (event: MessageEvent) => {
        try {
          const item: OpenBillProductFromSSE = JSON.parse(event.data);
          
          // If the item is completed or cancelled, remove it from the list
          if (item.status === "completed" || item.status === "cancelled") {
            setItems((prev) =>
              prev.filter(
                (i) => i.open_bill_product_id !== item.open_bill_product_id
              )
            );
            setPinnedIds((prev) => {
              if (prev.has(item.open_bill_product_id)) {
                const updated = new Set(prev);
                updated.delete(item.open_bill_product_id);
                savePinnedIds(updated);
                return updated;
              }
              return prev;
            });
          } else {
            // Otherwise, update the item in place
            setItems((prev) => {
              const existingIndex = prev.findIndex(
                (i) => i.open_bill_product_id === item.open_bill_product_id
              );
              if (existingIndex >= 0) {
                const updated = [...prev];
                updated[existingIndex] = item;
                return updated;
              }
              return prev;
            });
          }
        } catch (error) {
          console.error("Error parsing open_bill_product.updated event:", error);
        }
      };

      const handleCancelledItem = (event: MessageEvent) => {
        try {
          const item: OpenBillProductFromSSE = JSON.parse(event.data);
          setItems((prev) =>
            prev.filter(
              (i) => i.open_bill_product_id !== item.open_bill_product_id
            )
          );
          setPinnedIds((prev) => {
            if (prev.has(item.open_bill_product_id)) {
              const updated = new Set(prev);
              updated.delete(item.open_bill_product_id);
              savePinnedIds(updated);
              return updated;
            }
            return prev;
          });
        } catch (error) {
          console.error("Error parsing open_bill_product.cancelled event:", error);
        }
      };

      eventSource.addEventListener("open_bill_product.created", handleCreatedItem);
      eventSource.addEventListener(
        "open_bill_product.updated",
        handleUpdatedItem
      );
      eventSource.addEventListener(
        "open_bill_product.cancelled",
        handleCancelledItem
      );

      eventSource.onerror = () => {
        eventSource.close();

        if (isMounted) {
          setIsConnecting(false);
          setConnectionError("Conexión perdida. Reconectando...");

          reconnectTimeoutRef.current = setTimeout(() => {
            connectToSSE();
          }, 3000);
        }
      };
    };

    connectToSSE();

    return () => {
      isMounted = false;
      if (eventSourceRef.current) {
        eventSourceRef.current.close();
      }
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current);
      }
    };
  }, [savePinnedIds]);

  const handleComplete = async (openBillProductId: string) => {
    // Find the item to get the open_bill_id (order ID)
    const item = items.find((i) => i.open_bill_product_id === openBillProductId);
    if (!item) return;

    setCompletingIds((prev) => new Set(prev).add(openBillProductId));

    try {
      await completeOpenBillProduct(item.open_bill_id, openBillProductId);
      setItems((prev) =>
        prev.filter((i) => i.open_bill_product_id !== openBillProductId)
      );
      setPinnedIds((prev) => {
        if (prev.has(openBillProductId)) {
          const updated = new Set(prev);
          updated.delete(openBillProductId);
          savePinnedIds(updated);
          return updated;
        }
        return prev;
      });
    } catch (error) {
      console.error("Error completing open bill product:", error);
      setConnectionError(
        error instanceof Error
          ? error.message
          : "Error al marcar como completado"
      );
      setTimeout(() => setConnectionError(null), 3000);
    } finally {
      setCompletingIds((prev) => {
        const updated = new Set(prev);
        updated.delete(openBillProductId);
        return updated;
      });
    }
  };

  const handleTogglePin = (openBillProductId: string) => {
    setPinnedIds((prev) => {
      const updated = new Set(prev);
      if (updated.has(openBillProductId)) {
        updated.delete(openBillProductId);
      } else {
        updated.add(openBillProductId);
      }
      savePinnedIds(updated);
      return updated;
    });
  };

  const { pinnedItems, pendingItems } = useMemo(() => {
    const pinned: OpenBillProductFromSSE[] = [];
    const pending: OpenBillProductFromSSE[] = [];

    items.forEach((item) => {
      if (pinnedIds.has(item.open_bill_product_id)) {
        pinned.push(item);
      } else {
        pending.push(item);
      }
    });

    const sortByCountdown = (a: OpenBillProductFromSSE, b: OpenBillProductFromSSE) => {
      const aRemaining = calculateRemainingMs(a.priority, a.created_at, now);
      const bRemaining = calculateRemainingMs(b.priority, b.created_at, now);
      return aRemaining - bRemaining;
    };

    pinned.sort(sortByCountdown);
    pending.sort(sortByCountdown);

    return { pinnedItems: pinned, pendingItems: pending };
  }, [items, pinnedIds, now]);

  const totalItems = items.length;

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
          maxWidth: "1600px",
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
          <div style={{ display: "flex", alignItems: "center", gap: "1rem" }}>
            <h1
              style={{
                fontSize: "clamp(1.75rem, 4vw, 2.5rem)",
                fontWeight: "bold",
                margin: 0,
                color: "var(--color-text-primary)",
              }}
            >
              🍳 Comandas Individuales
            </h1>

            {/* Connection status indicator */}
            <div
              style={{
                display: "flex",
                alignItems: "center",
                gap: "0.5rem",
                padding: "0.5rem 1rem",
                borderRadius: "var(--radius-md)",
                backgroundColor: isConnecting
                  ? "var(--color-warning-light)"
                  : connectionError
                  ? "var(--color-danger-light)"
                  : "var(--color-success-light)",
                fontSize: "0.875rem",
                fontWeight: "500",
              }}
            >
              <div
                style={{
                  width: "8px",
                  height: "8px",
                  borderRadius: "50%",
                  backgroundColor: isConnecting
                    ? "var(--color-warning)"
                    : connectionError
                    ? "var(--color-danger)"
                    : "var(--color-success)",
                  animation: isConnecting ? "pulse 1.5s infinite" : "none",
                }}
              />
              <span
                style={{
                  color: isConnecting
                    ? "var(--color-warning)"
                    : connectionError
                    ? "var(--color-danger)"
                    : "var(--color-success)",
                }}
              >
                {isConnecting
                  ? "Conectando..."
                  : connectionError
                  ? connectionError
                  : "En vivo"}
              </span>
            </div>
          </div>

          {/* Items count */}
          <div style={{ display: "flex", gap: "1rem", flexWrap: "wrap" }}>
            <div
              style={{
                padding: "0.75rem 1.25rem",
                borderRadius: "var(--radius-md)",
                backgroundColor: "var(--color-surface)",
                border: "1px solid var(--color-border)",
                fontSize: "1.1rem",
                fontWeight: "600",
                color: "var(--color-text-primary)",
              }}
            >
              {totalItems} {totalItems === 1 ? "item" : "items"} pendiente
              {totalItems !== 1 ? "s" : ""}
            </div>
            {pinnedItems.length > 0 && (
              <div
                style={{
                  padding: "0.75rem 1.25rem",
                  borderRadius: "var(--radius-md)",
                  backgroundColor: "var(--color-primary-light)",
                  border: "1px solid var(--color-primary)",
                  fontSize: "1.1rem",
                  fontWeight: "600",
                  color: "var(--color-primary)",
                }}
              >
                📌 {pinnedItems.length} en progreso
              </div>
            )}
          </div>
        </div>

        {/* Loading State */}
        {isConnecting && items.length === 0 && (
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
                width: "50px",
                height: "50px",
                border: "4px solid var(--color-border)",
                borderTop: "4px solid var(--color-primary)",
                borderRadius: "50%",
                animation: "spin 1s linear infinite",
                marginBottom: "1rem",
              }}
            />
            <div>Conectando a comandas individuales...</div>
          </div>
        )}

        {/* Empty State */}
        {!isConnecting && items.length === 0 && (
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
                fontSize: "4rem",
                marginBottom: "1rem",
              }}
            >
              ✨
            </div>
            <h3
              style={{
                margin: "0 0 0.5rem 0",
                fontSize: "1.5rem",
                color: "var(--color-text-primary)",
              }}
            >
              ¡Todo al día!
            </h3>
            <p
              style={{
                margin: 0,
                color: "var(--color-text-secondary)",
                fontSize: "1.1rem",
              }}
            >
              No hay comandas individuales pendientes en este momento
            </p>
          </div>
        )}

        {/* Pinned Items Section */}
        {pinnedItems.length > 0 && (
          <div style={{ marginBottom: "2rem" }}>
            <h2
              style={{
                fontSize: "1.25rem",
                fontWeight: "bold",
                color: "var(--color-primary)",
                marginBottom: "1rem",
                display: "flex",
                alignItems: "center",
                gap: "0.5rem",
              }}
            >
              📌 En Progreso ({pinnedItems.length})
            </h2>
            <div
              style={{
                display: "grid",
                gridTemplateColumns:
                  "repeat(auto-fill, minmax(min(320px, 100%), 1fr))",
                gap: "1.25rem",
              }}
            >
              {pinnedItems.map((item) => (
                <CommandItemCard
                  key={item.open_bill_product_id}
                  item={item}
                  isPinned={true}
                  onComplete={handleComplete}
                  onTogglePin={handleTogglePin}
                  isCompleting={completingIds.has(item.open_bill_product_id)}
                  now={now}
                />
              ))}
            </div>
          </div>
        )}

        {/* Pending Items Section */}
        {pendingItems.length > 0 && (
          <div>
            <h2
              style={{
                fontSize: "1.25rem",
                fontWeight: "bold",
                color: "var(--color-text-primary)",
                marginBottom: "1rem",
                display: "flex",
                alignItems: "center",
                gap: "0.5rem",
              }}
            >
              ⏳ Pendientes ({pendingItems.length})
            </h2>
            <div
              style={{
                display: "grid",
                gridTemplateColumns:
                  "repeat(auto-fill, minmax(min(320px, 100%), 1fr))",
                gap: "1.25rem",
              }}
            >
              {pendingItems.map((item) => (
                <CommandItemCard
                  key={item.open_bill_product_id}
                  item={item}
                  isPinned={false}
                  onComplete={handleComplete}
                  onTogglePin={handleTogglePin}
                  isCompleting={completingIds.has(item.open_bill_product_id)}
                  now={now}
                />
              ))}
            </div>
          </div>
        )}
      </div>

      {/* Animations */}
      <style jsx global>{`
        @keyframes pulse {
          0%,
          100% {
            opacity: 1;
          }
          50% {
            opacity: 0.4;
          }
        }
        @keyframes spin {
          from {
            transform: rotate(0deg);
          }
          to {
            transform: rotate(360deg);
          }
        }
      `}</style>
    </div>
  );
}
