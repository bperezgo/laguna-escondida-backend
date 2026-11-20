"use client";

import { useCallback, useState } from "react";
import { Button, Modal, Select, Textarea } from "@/components/ui";
import { deliveriesApi, DeliveryApiError } from "@/lib/api/deliveries";
import {
  FAILURE_REASONS,
  MAX_NOTE,
  deliveryErrorMessage,
  failRequest,
  orderLabel,
  telLink,
} from "@/lib/deliveries/board";
import { usePolling } from "@/lib/deliveries/usePolling";
import { PERMISSIONS, usePermissions } from "@/lib/permissions";
import type { CourierQueue, DeliveryFailureReason, DeliveryOrder } from "@/types/delivery";

export default function MyDeliveriesClient() {
  const { hasPermission, isLoading } = usePermissions();
  const [queue, setQueue] = useState<CourierQueue>({ claimable: [], on_the_road: [] });
  const [loadError, setLoadError] = useState("");
  const [message, setMessage] = useState("");
  const [busyId, setBusyId] = useState<string | null>(null);
  const [failing, setFailing] = useState<DeliveryOrder | null>(null);
  const allowed = hasPermission(PERMISSIONS.DELIVERIES_CARRY);

  const refresh = usePolling(
    useCallback(async () => {
      try {
        setQueue(await deliveriesApi.mine());
        setLoadError("");
      } catch (err) {
        setLoadError(
          err instanceof DeliveryApiError && err.status === 404
            ? "Las entregas solo se gestionan en la nube."
            : "Error al cargar tus entregas.",
        );
      }
    }, []),
  );

  const run = async (order: DeliveryOrder, change: () => Promise<unknown>, claiming = false) => {
    setBusyId(order.id);
    setMessage("");
    try {
      await change();
    } catch (err) {
      setMessage(`Pedido ${orderLabel(order)}: ${deliveryErrorMessage(err, claiming)}`);
    } finally {
      setBusyId(null);
      await refresh();
    }
  };

  if (!isLoading && !allowed) {
    return <p style={{ padding: "1rem", color: "var(--color-text-secondary)" }}>No tienes permiso para ver esta sección.</p>;
  }

  return (
    <div style={{ minHeight: "100vh", backgroundColor: "var(--color-bg)", padding: "1rem" }}>
      <div style={{ maxWidth: "640px", margin: "0 auto", display: "flex", flexDirection: "column", gap: "1rem" }}>
        <h1 style={{ margin: 0, fontSize: "1.5rem", color: "var(--color-text-primary)" }}>Mis entregas</h1>
        {loadError && <p role="alert" style={{ color: "var(--color-danger)", margin: 0 }}>{loadError}</p>}
        {message && <p role="alert" style={{ color: "var(--color-danger)", margin: 0 }}>{message}</p>}

        <section aria-label="En camino">
          <h2 style={{ fontSize: "1.1rem", margin: "0 0 0.5rem 0" }}>En camino ({queue.on_the_road.length})</h2>
          {queue.on_the_road.length === 0 && <p style={{ color: "var(--color-text-secondary)", margin: 0 }}>No llevas pedidos.</p>}
          {queue.on_the_road.map((order) => (
            <CourierCard key={order.id} order={order}>
              <Button fullWidth size="lg" disabled={busyId === order.id} onClick={() => run(order, () => deliveriesApi.deliver(order.id))}>
                Entregado
              </Button>
              <Button fullWidth size="lg" variant="danger" disabled={busyId === order.id} onClick={() => setFailing(order)}>
                No se pudo entregar
              </Button>
            </CourierCard>
          ))}
        </section>

        <section aria-label="Para llevar">
          <h2 style={{ fontSize: "1.1rem", margin: "0 0 0.5rem 0" }}>Para llevar ({queue.claimable.length})</h2>
          {queue.claimable.length === 0 && <p style={{ color: "var(--color-text-secondary)", margin: 0 }}>No hay pedidos listos.</p>}
          {queue.claimable.map((order) => (
            <CourierCard key={order.id} order={order}>
              <Button fullWidth size="lg" disabled={busyId === order.id} onClick={() => run(order, () => deliveriesApi.dispatch(order.id), true)}>
                Lo llevo
              </Button>
            </CourierCard>
          ))}
        </section>
      </div>

      {failing && (
        <CourierFailDialog
          order={failing}
          onClose={() => setFailing(null)}
          onDone={() => {
            setFailing(null);
            void refresh();
          }}
        />
      )}
    </div>
  );
}

function CourierCard({ order, children }: { order: DeliveryOrder; children: React.ReactNode }) {
  return (
    <article
      style={{
        border: "1px solid var(--color-border)",
        borderRadius: "10px",
        padding: "1rem",
        marginBottom: "0.75rem",
        backgroundColor: "var(--color-surface)",
        display: "flex",
        flexDirection: "column",
        gap: "0.5rem",
      }}
    >
      <div style={{ display: "flex", justifyContent: "space-between" }}>
        <strong>Pedido {orderLabel(order)}</strong>
        <span style={{ color: "var(--color-success-text)" }}>Pagado</span>
      </div>
      {order.customer_name && <div>{order.customer_name}</div>}
      {order.delivery_address && <div style={{ fontSize: "1.05rem" }}>{order.delivery_address}</div>}
      <div style={{ display: "flex", gap: "0.75rem", flexWrap: "wrap" }}>
        {order.delivery_map_url && (
          <a href={order.delivery_map_url} target="_blank" rel="noreferrer" style={{ color: "var(--color-primary)", fontWeight: 600 }}>
            Abrir mapa
          </a>
        )}
        {order.customer_phone && (
          <a href={telLink(order.customer_phone)} style={{ color: "var(--color-primary)", fontWeight: 600 }}>
            Llamar {order.customer_phone}
          </a>
        )}
      </div>
      <ul style={{ margin: 0, paddingLeft: "1.25rem" }}>
        {order.lines.map((line, index) => (
          <li key={index}>
            {line.quantity} × {line.name}
            {line.notes && <span style={{ color: "var(--color-text-secondary)" }}> ({line.notes})</span>}
          </li>
        ))}
      </ul>
      <div style={{ display: "flex", flexDirection: "column", gap: "0.5rem" }}>{children}</div>
    </article>
  );
}

function CourierFailDialog({ order, onClose, onDone }: { order: DeliveryOrder; onClose: () => void; onDone: () => void }) {
  const [reason, setReason] = useState<DeliveryFailureReason | "">("");
  const [note, setNote] = useState("");
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);

  const submit = async () => {
    const result = failRequest(reason, note);
    if ("error" in result) {
      setError(result.error);
      return;
    }
    setSaving(true);
    try {
      await deliveriesApi.fail(order.id, result.body);
      onDone();
    } catch (err) {
      setError(deliveryErrorMessage(err));
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal
      open
      onClose={onClose}
      title={`No se pudo entregar: pedido ${orderLabel(order)}`}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>Cancelar</Button>
          <Button variant="danger" onClick={submit} disabled={saving}>Guardar</Button>
        </>
      }
    >
      <div style={{ display: "flex", flexDirection: "column", gap: "0.75rem" }}>
        <Select label="Motivo" value={reason} onChange={(e) => setReason(e.target.value as DeliveryFailureReason | "")}>
          <option value="">Elige…</option>
          {FAILURE_REASONS.map((r) => (
            <option key={r.reason} value={r.reason}>{r.label}</option>
          ))}
        </Select>
        <Textarea label="Nota (opcional)" value={note} maxLength={MAX_NOTE} onChange={(e) => setNote(e.target.value)} />
        {error && <p role="alert" style={{ color: "var(--color-danger)", margin: 0 }}>{error}</p>}
      </div>
    </Modal>
  );
}
