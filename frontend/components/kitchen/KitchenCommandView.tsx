"use client";

import { useState, useEffect, useRef, useMemo } from "react";
import CommandCard from "./CommandCard";
import {
  completeOpenBillProduct,
  uncompleteOpenBillProduct,
} from "@/lib/api/openBillProducts";
import type {
  OpenBillProductFromSSE,
  OpenBillProductStatus,
} from "@/types/commandItem";
import { useNow } from "@/lib/kitchen/useNow";

// How long a fully-completed comanda flashes "✓ Lista" before it leaves the board.
const READY_FLASH_MS = 3000;

// Grouped command structure for display (derived from open bill products)
export interface GroupedCommand {
  id: string; // temporal_identifier used as group ID
  temporal_identifier: string;
  created_by_name: string;
  area: string;
  status: "created" | "completed" | "cancelled";
  items: {
    id: string;
    open_bill_id: string;
    product_name: string;
    quantity: number;
    notes?: string | null;
    // Per-line countdown inputs (see lib/kitchen/countdown).
    priority: number;
    created_at: string;
    // Per-line status drives the strike-through (completed = struck).
    status: OpenBillProductStatus;
    // Stamped client-side when the cook strikes the line.
    completed_at?: string | null;
  }[];
  created_at: string;
}

export default function KitchenCommandView() {
  const now = useNow();
  const [products, setProducts] = useState<OpenBillProductFromSSE[]>([]);
  const [isConnecting, setIsConnecting] = useState(true);
  const [connectionError, setConnectionError] = useState<string | null>(null);
  const [completingIds, setCompletingIds] = useState<Set<string>>(new Set());
  // Comandas whose last line was just struck: flashing "✓ Lista" before removal.
  const [readyGroups, setReadyGroups] = useState<Set<string>>(new Set());
  const readyTimersRef = useRef<Map<string, ReturnType<typeof setTimeout>>>(
    new Map()
  );
  const eventSourceRef = useRef<EventSource | null>(null);
  const reconnectTimeoutRef = useRef<NodeJS.Timeout | null>(null);

  // Group products by temporal_identifier to create command-like groups
  const commands = useMemo((): GroupedCommand[] => {
    const groupMap = new Map<string, OpenBillProductFromSSE[]>();
    
    products.forEach((product) => {
      const key = product.temporal_identifier;
      if (!groupMap.has(key)) {
        groupMap.set(key, []);
      }
      groupMap.get(key)!.push(product);
    });

    const grouped: GroupedCommand[] = [];
    
    groupMap.forEach((groupProducts, temporalId) => {
      // Sort products by created_at within the group
      groupProducts.sort(
        (a, b) => new Date(a.created_at).getTime() - new Date(b.created_at).getTime()
      );

      const firstProduct = groupProducts[0];

      // The comanda is "completed" only once every line is struck; until then it
      // stays on the board so cooks see the whole ticket and its progress.
      const allCompleted = groupProducts.every(
        (p) => p.status === "completed"
      );

      grouped.push({
        id: temporalId,
        temporal_identifier: temporalId,
        created_by_name: firstProduct.created_by_name,
        area: firstProduct.area,
        status: allCompleted ? "completed" : "created",
        items: groupProducts.map((p) => ({
          id: p.open_bill_product_id,
          open_bill_id: p.open_bill_id,
          product_name: p.product_name,
          quantity: p.quantity,
          notes: p.notes,
          priority: p.priority,
          created_at: p.created_at,
          status: p.status,
          completed_at: p.completed_at,
        })),
        created_at: firstProduct.created_at,
      });
    });

    // Sort groups by oldest created_at
    grouped.sort(
      (a, b) => new Date(a.created_at).getTime() - new Date(b.created_at).getTime()
    );

    return grouped;
  }, [products]);

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

      const handleCreatedProduct = (event: MessageEvent) => {
        try {
          const product: OpenBillProductFromSSE = JSON.parse(event.data);

          setProducts((prev) => {
            const existingIndex = prev.findIndex(
              (p) => p.open_bill_product_id === product.open_bill_product_id
            );

            if (existingIndex >= 0) {
              const updated = [...prev];
              updated[existingIndex] = product;
              return updated;
            } else {
              return [...prev, product];
            }
          });
        } catch (error) {
          console.error("Error parsing open_bill_product.created event:", error);
        }
      };

      const handleUpdatedProduct = (event: MessageEvent) => {
        try {
          const product: OpenBillProductFromSSE = JSON.parse(event.data);

          // Cancelled lines leave the board. Completed lines STAY (rendered
          // struck-through) so the comanda keeps showing its progress until every
          // line is done.
          if (product.status === "cancelled") {
            setProducts((prev) =>
              prev.filter(
                (p) => p.open_bill_product_id !== product.open_bill_product_id
              )
            );
            return;
          }

          setProducts((prev) => {
            const existingIndex = prev.findIndex(
              (p) => p.open_bill_product_id === product.open_bill_product_id
            );
            if (existingIndex >= 0) {
              const updated = [...prev];
              updated[existingIndex] = product;
              return updated;
            }
            return prev;
          });
        } catch (error) {
          console.error("Error parsing open_bill_product.updated event:", error);
        }
      };

      const handleCancelledProduct = (event: MessageEvent) => {
        try {
          const product: OpenBillProductFromSSE = JSON.parse(event.data);
          setProducts((prev) =>
            prev.filter((p) => p.open_bill_product_id !== product.open_bill_product_id)
          );
        } catch (error) {
          console.error("Error parsing open_bill_product.cancelled event:", error);
        }
      };

      eventSource.addEventListener("open_bill_product.created", handleCreatedProduct);
      eventSource.addEventListener("open_bill_product.updated", handleUpdatedProduct);
      eventSource.addEventListener("open_bill_product.cancelled", handleCancelledProduct);

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
  }, []);

  const setProductStatus = (
    productId: string,
    status: OpenBillProductStatus,
    completed_at?: string | null
  ) => {
    setProducts((prev) =>
      prev.map((p) =>
        p.open_bill_product_id === productId ? { ...p, status, completed_at } : p
      )
    );
  };

  const markBusy = (id: string, busy: boolean) => {
    setCompletingIds((prev) => {
      const updated = new Set(prev);
      if (busy) updated.add(id);
      else updated.delete(id);
      return updated;
    });
  };

  const showError = (error: unknown, fallback: string) => {
    console.error(fallback, error);
    setConnectionError(error instanceof Error ? error.message : fallback);
    setTimeout(() => setConnectionError(null), 3000);
  };

  // Strike a single line. Stays visible (struck) until the whole comanda is done.
  const handleCompleteLine = async (openBillId: string, productId: string) => {
    markBusy(productId, true);
    setProductStatus(productId, "completed", new Date().toISOString());
    try {
      await completeOpenBillProduct(openBillId, productId);
    } catch (error) {
      setProductStatus(productId, "created", null);
      showError(error, "Error al marcar como completado");
    } finally {
      markBusy(productId, false);
    }
  };

  const handleUndoLine = async (openBillId: string, productId: string) => {
    markBusy(productId, true);
    setProductStatus(productId, "created", null);
    try {
      await uncompleteOpenBillProduct(openBillId, productId);
    } catch (error) {
      setProductStatus(productId, "completed"); // revert — no timestamp recovery needed
      showError(error, "Error al deshacer");
    } finally {
      markBusy(productId, false);
    }
  };

  // When every line of a comanda is struck, flash "✓ Lista" then drop the card.
  // If an undo reopens a line mid-flash, cancel the removal.
  useEffect(() => {
    const timers = readyTimersRef.current;

    commands.forEach((command) => {
      const allCompleted =
        command.items.length > 0 &&
        command.items.every((item) => item.status === "completed");

      if (allCompleted && !timers.has(command.id)) {
        setReadyGroups((prev) => new Set(prev).add(command.id));
        const timer = setTimeout(() => {
          setProducts((prev) =>
            prev.filter((p) => p.temporal_identifier !== command.id)
          );
          timers.delete(command.id);
          setReadyGroups((prev) => {
            const next = new Set(prev);
            next.delete(command.id);
            return next;
          });
        }, READY_FLASH_MS);
        timers.set(command.id, timer);
      } else if (!allCompleted && timers.has(command.id)) {
        clearTimeout(timers.get(command.id)!);
        timers.delete(command.id);
        setReadyGroups((prev) => {
          const next = new Set(prev);
          next.delete(command.id);
          return next;
        });
      }
    });
  }, [commands]);

  // Clear any pending flash timers on unmount.
  useEffect(() => {
    const timers = readyTimersRef.current;
    return () => {
      timers.forEach((timer) => clearTimeout(timer));
      timers.clear();
    };
  }, []);

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
              🍳 Comandas de Cocina
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

          {/* Commands count */}
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
            {commands.length} {commands.length === 1 ? "comanda" : "comandas"}{" "}
            pendiente{commands.length !== 1 ? "s" : ""}
          </div>
        </div>

        {/* Loading State */}
        {isConnecting && commands.length === 0 && (
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
            <div>Conectando a comandas de cocina...</div>
          </div>
        )}

        {/* Empty State */}
        {!isConnecting && commands.length === 0 && (
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
              No hay comandas pendientes en este momento
            </p>
          </div>
        )}

        {/* Commands Grid */}
        {commands.length > 0 && (
          <div
            style={{
              display: "grid",
              gridTemplateColumns:
                "repeat(auto-fill, minmax(min(380px, 100%), 1fr))",
              gap: "1.25rem",
            }}
          >
            {commands.map((command) => (
              <CommandCard
                key={command.id}
                command={command}
                onCompleteLine={handleCompleteLine}
                onUndoLine={handleUndoLine}
                completingIds={completingIds}
                isReady={readyGroups.has(command.id)}
                now={now}
              />
            ))}
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
