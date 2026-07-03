"use client";

import { useState, useEffect, useRef, useCallback, useMemo } from "react";
import ReadyCommandCard, { type ReadyCommand } from "./ReadyCommandCard";
import { getCompletedOpenBillProducts } from "@/lib/api/openBillProducts";
import type { OpenBillProductFromSSE } from "@/types/commandItem";

const REFRESH_INTERVAL_MS = 30000;

function timeOf(dateString: string | null): number {
  return dateString ? new Date(dateString).getTime() : 0;
}

export default function KitchenReadyView() {
  const [items, setItems] = useState<OpenBillProductFromSSE[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const pollRef = useRef<NodeJS.Timeout | null>(null);

  const load = useCallback(async (isManual: boolean) => {
    if (isManual) setIsRefreshing(true);
    try {
      const products = await getCompletedOpenBillProducts("kitchen");
      setItems(products);
      setError(null);
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Error al cargar comandas listas"
      );
    } finally {
      setIsLoading(false);
      if (isManual) setIsRefreshing(false);
    }
  }, []);

  useEffect(() => {
    load(false);
    pollRef.current = setInterval(() => load(false), REFRESH_INTERVAL_MS);
    return () => {
      if (pollRef.current) clearInterval(pollRef.current);
    };
  }, [load]);

  // Group completed lines by comanda, newest-completed first.
  const commands = useMemo((): ReadyCommand[] => {
    const groupMap = new Map<string, OpenBillProductFromSSE[]>();
    items.forEach((product) => {
      const key = product.temporal_identifier;
      if (!groupMap.has(key)) groupMap.set(key, []);
      groupMap.get(key)!.push(product);
    });

    const grouped: ReadyCommand[] = [];
    groupMap.forEach((groupProducts, temporalId) => {
      groupProducts.sort(
        (a, b) => timeOf(a.created_at) - timeOf(b.created_at)
      );
      const firstProduct = groupProducts[0];
      const completedAt = groupProducts.reduce<string | null>((latest, p) => {
        if (!p.completed_at) return latest;
        if (!latest) return p.completed_at;
        return timeOf(p.completed_at) > timeOf(latest) ? p.completed_at : latest;
      }, null);

      grouped.push({
        id: temporalId,
        temporal_identifier: temporalId,
        created_by_name: firstProduct.created_by_name,
        completed_at: completedAt,
        created_at: firstProduct.created_at,
        items: groupProducts.map((p) => ({
          id: p.open_bill_product_id,
          product_name: p.product_name,
          quantity: p.quantity,
          notes: p.notes,
        })),
      });
    });

    grouped.sort(
      (a, b) =>
        (timeOf(b.completed_at) || timeOf(b.created_at)) -
        (timeOf(a.completed_at) || timeOf(a.created_at))
    );

    return grouped;
  }, [items]);

  return (
    <div
      style={{
        minHeight: "100vh",
        backgroundColor: "var(--color-bg)",
        padding: "1rem",
      }}
    >
      <div style={{ maxWidth: "1600px", margin: "0 auto" }}>
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
              ✅ Comandas Listas
            </h1>
            <span
              style={{
                fontSize: "0.9rem",
                color: "var(--color-text-secondary)",
              }}
            >
              Hoy
            </span>
          </div>

          <div
            style={{
              display: "flex",
              gap: "1rem",
              flexWrap: "wrap",
              alignItems: "center",
            }}
          >
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
              {commands.length}{" "}
              {commands.length === 1 ? "comanda" : "comandas"}
            </div>
            <button
              onClick={() => load(true)}
              disabled={isRefreshing}
              style={{
                padding: "0.75rem 1.25rem",
                borderRadius: "var(--radius-md)",
                backgroundColor: "var(--color-surface)",
                border: "1px solid var(--color-border)",
                fontSize: "1rem",
                fontWeight: "600",
                color: "var(--color-text-primary)",
                cursor: isRefreshing ? "not-allowed" : "pointer",
                transition: "all 0.2s ease",
              }}
            >
              {isRefreshing ? "Actualizando…" : "↻ Actualizar"}
            </button>
          </div>
        </div>

        {/* Error */}
        {error && (
          <div
            style={{
              padding: "1rem",
              marginBottom: "1rem",
              borderRadius: "var(--radius-md)",
              backgroundColor: "var(--color-danger-light)",
              color: "var(--color-danger)",
              fontWeight: "500",
            }}
          >
            {error}
          </div>
        )}

        {/* Loading */}
        {isLoading && commands.length === 0 && (
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
            <div>Cargando comandas listas…</div>
          </div>
        )}

        {/* Empty */}
        {!isLoading && commands.length === 0 && (
          <div
            style={{
              textAlign: "center",
              padding: "4rem",
              backgroundColor: "var(--color-surface)",
              borderRadius: "var(--radius-lg)",
              border: "2px dashed var(--color-border)",
            }}
          >
            <div style={{ fontSize: "4rem", marginBottom: "1rem" }}>🍽️</div>
            <h3
              style={{
                margin: "0 0 0.5rem 0",
                fontSize: "1.5rem",
                color: "var(--color-text-primary)",
              }}
            >
              Aún no hay comandas listas hoy
            </h3>
            <p
              style={{
                margin: 0,
                color: "var(--color-text-secondary)",
                fontSize: "1.1rem",
              }}
            >
              Las comandas completadas aparecerán aquí
            </p>
          </div>
        )}

        {/* Grid */}
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
              <ReadyCommandCard key={command.id} command={command} />
            ))}
          </div>
        )}
      </div>

      <style jsx global>{`
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
