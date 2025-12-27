"use client";

import { useState, useEffect, useRef, useCallback, useMemo } from "react";
import CommandItemCard from "./CommandItemCard";
import { completeCommandItem } from "@/lib/api/commandItems";
import type { CommandItemFromSSE } from "@/types/commandItem";

const PINNED_STORAGE_KEY = "pinned-command-items";
const COUNTDOWN_CONSTANT = 30;
const UTC_OFFSET_HOURS = 5; // Backend sends time in UTC-5

function calculateRemainingMs(priority: number, createdAt: string): number {
  const totalMinutes = COUNTDOWN_CONSTANT / (priority + 1);
  const totalMs = totalMinutes * 60 * 1000;
  // Backend sends time in UTC-5, so we add 5 hours to convert to UTC
  const createdTime =
    new Date(createdAt).getTime() + UTC_OFFSET_HOURS * 60 * 60 * 1000;
  const elapsed = Date.now() - createdTime;
  return totalMs - elapsed;
}

export default function KitchenCommandItemsView() {
  const [items, setItems] = useState<CommandItemFromSSE[]>([]);
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

      const eventSource = new EventSource("/api/sse/command-items/kitchen");
      eventSourceRef.current = eventSource;

      eventSource.onopen = () => {
        console.log("Command items EventSource connection opened");
        if (isMounted) {
          setIsConnecting(false);
          setConnectionError(null);
        }
      };

      const handleCreatedItem = (event: MessageEvent) => {
        try {
          console.log("command_item.created event:", event.data);
          const item: CommandItemFromSSE = JSON.parse(event.data);

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
          console.error("Error parsing command_item.created event:", error);
        }
      };

      const handleCompletedItem = (event: MessageEvent) => {
        try {
          const item: CommandItemFromSSE = JSON.parse(event.data);
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
          console.error("Error parsing command_item.completed event:", error);
        }
      };

      const handleCancelledItem = (event: MessageEvent) => {
        try {
          const item: CommandItemFromSSE = JSON.parse(event.data);
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
          console.error("Error parsing command_item.cancelled event:", error);
        }
      };

      eventSource.addEventListener("command_item.created", handleCreatedItem);
      eventSource.addEventListener(
        "command_item.completed",
        handleCompletedItem
      );
      eventSource.addEventListener(
        "command_item.cancelled",
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
      console.log("Cleaning up command items EventSource...");
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
    setCompletingIds((prev) => new Set(prev).add(openBillProductId));

    try {
      await completeCommandItem(openBillProductId);
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
      console.error("Error completing command item:", error);
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
    const pinned: CommandItemFromSSE[] = [];
    const pending: CommandItemFromSSE[] = [];

    items.forEach((item) => {
      if (pinnedIds.has(item.open_bill_product_id)) {
        pinned.push(item);
      } else {
        pending.push(item);
      }
    });

    const sortByCountdown = (a: CommandItemFromSSE, b: CommandItemFromSSE) => {
      const aRemaining = calculateRemainingMs(a.priority, a.created_at);
      const bRemaining = calculateRemainingMs(b.priority, b.created_at);
      return aRemaining - bRemaining;
    };

    pinned.sort(sortByCountdown);
    pending.sort(sortByCountdown);

    return { pinnedItems: pinned, pendingItems: pending };
  }, [items, pinnedIds]);

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
