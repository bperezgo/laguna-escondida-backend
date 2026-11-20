"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { Button, Input, Modal, Select, Textarea } from "@/components/ui";
import { deliveriesApi, DeliveryApiError } from "@/lib/api/deliveries";
import {
  ACTION_LABEL,
  FAILURE_REASONS,
  MAX_COURIER_NAME,
  MAX_NOTE,
  STATUS_COLUMNS,
  STATUS_LABEL,
  actionsFor,
  deliveryErrorMessage,
  dispatchRequest,
  failRequest,
  failureReasonLabel,
  groupByStatus,
  orderLabel,
  telLink,
  timeInStatus,
  type BoardAction,
} from "@/lib/deliveries/board";
import { printSlip } from "@/lib/deliveries/printSlip";
import { usePolling } from "@/lib/deliveries/usePolling";
import { useNow } from "@/lib/kitchen/useNow";
import { formatDateTime } from "@/lib/format";
import { PERMISSIONS, usePermissions } from "@/lib/permissions";
import type {
  CourierUser,
  DeliveryFailureReason,
  DeliveryHistory,
  DeliveryOrder,
} from "@/types/delivery";
import BagSlip from "./BagSlip";

function loadErrorMessage(err: unknown): string {
  if (err instanceof DeliveryApiError && err.status === 404) return "Los pedidos solo se gestionan en la nube.";
  if (err instanceof DeliveryApiError && err.status === 403) return "No tienes permiso para ver los pedidos.";
  return "Error al cargar los pedidos.";
}

export default function DeliveryOrdersTab() {
  const { hasPermission } = usePermissions();
  const canManage = hasPermission(PERMISSIONS.DELIVERIES_MANAGE);
  const now = useNow(30_000);
  const [orders, setOrders] = useState<DeliveryOrder[]>([]);
  const [loadError, setLoadError] = useState("");
  const [loaded, setLoaded] = useState(false);
  const [actionError, setActionError] = useState("");
  const [busyId, setBusyId] = useState<string | null>(null);
  const [dispatching, setDispatching] = useState<DeliveryOrder | null>(null);
  const [failing, setFailing] = useState<DeliveryOrder | null>(null);
  const [historyOf, setHistoryOf] = useState<DeliveryOrder | null>(null);
  const [slipOf, setSlipOf] = useState<DeliveryOrder | null>(null);
  const slipRef = useRef<HTMLDivElement>(null);

  const refresh = usePolling(
    useCallback(async () => {
      try {
        const list = await deliveriesApi.list();
        setOrders(list.orders);
        setLoadError("");
      } catch (err) {
        setLoadError(loadErrorMessage(err));
      } finally {
        setLoaded(true);
      }
    }, []),
  );

  const run = async (order: DeliveryOrder, change: () => Promise<unknown>): Promise<boolean> => {
    setBusyId(order.id);
    setActionError("");
    try {
      await change();
      return true;
    } catch (err) {
      setActionError(`Pedido ${orderLabel(order)}: ${deliveryErrorMessage(err)}`);
      return false;
    } finally {
      setBusyId(null);
      await refresh();
    }
  };

  const onAction = (order: DeliveryOrder, action: BoardAction) => {
    if (action === "dispatch" || action === "redispatch") setDispatching(order);
    else if (action === "fail") setFailing(order);
    else void run(order, () => deliveriesApi.deliver(order.id));
  };

  const onSlipReady = useCallback(() => {
    if (!slipOf) return;
    printSlip(slipRef.current, `Pedido ${orderLabel(slipOf)}`);
    setSlipOf(null);
  }, [slipOf]);

  const groups = groupByStatus(orders);

  return (
    <div>
      {loadError && <p role="alert" style={{ color: "var(--color-danger)" }}>{loadError}</p>}
      {actionError && <p role="alert" style={{ color: "var(--color-danger)" }}>{actionError}</p>}
      {loaded && !loadError && orders.length === 0 && (
        <p style={{ color: "var(--color-text-secondary)" }}>No hay pedidos para llevar ni domicilios pendientes.</p>
      )}

      <div
        style={{
          display: "grid",
          gridTemplateColumns: "repeat(auto-fit, minmax(240px, 1fr))",
          gap: "1rem",
          alignItems: "start",
        }}
      >
        {STATUS_COLUMNS.map(({ status, label }) => (
          <section key={status} aria-label={label}>
            <h2 style={{ fontSize: "1rem", margin: "0 0 0.5rem 0", color: "var(--color-text-primary)" }}>
              {label} ({groups[status].length})
            </h2>
            <div style={{ display: "flex", flexDirection: "column", gap: "0.5rem" }}>
              {groups[status].map((order) => (
                <OrderCard
                  key={order.id}
                  order={order}
                  now={now}
                  busy={busyId === order.id}
                  canManage={canManage}
                  onAction={(action) => onAction(order, action)}
                  onHistory={() => setHistoryOf(order)}
                  onPrint={() => setSlipOf(order)}
                />
              ))}
            </div>
          </section>
        ))}
      </div>

      {dispatching && (
        <DispatchDialog
          order={dispatching}
          onClose={() => setDispatching(null)}
          onDone={() => {
            setDispatching(null);
            void refresh();
          }}
        />
      )}
      {failing && (
        <FailDialog
          order={failing}
          onClose={() => setFailing(null)}
          onDone={() => {
            setFailing(null);
            void refresh();
          }}
        />
      )}
      {historyOf && <HistoryDialog order={historyOf} onClose={() => setHistoryOf(null)} />}
      {slipOf && (
        <div style={{ display: "none" }}>
          <BagSlip ref={slipRef} order={slipOf} onReady={onSlipReady} />
        </div>
      )}
    </div>
  );
}

interface OrderCardProps {
  order: DeliveryOrder;
  now: number;
  busy: boolean;
  canManage: boolean;
  onAction: (action: BoardAction) => void;
  onHistory: () => void;
  onPrint: () => void;
}

function OrderCard({ order, now, busy, canManage, onAction, onHistory, onPrint }: OrderCardProps) {
  return (
    <article
      style={{
        border: "1px solid var(--color-border)",
        borderRadius: "8px",
        padding: "0.75rem",
        backgroundColor: "var(--color-surface)",
        display: "flex",
        flexDirection: "column",
        gap: "0.35rem",
      }}
    >
      <div style={{ display: "flex", justifyContent: "space-between", gap: "0.5rem" }}>
        <strong>{orderLabel(order)}</strong>
        <span style={{ color: "var(--color-text-secondary)", fontSize: "0.85rem" }}>
          {order.order_type === "delivery" ? "Domicilio" : "Para llevar"} · {timeInStatus(order.status_since, new Date(now))}
        </span>
      </div>
      {order.customer_name && <div>{order.customer_name}</div>}
      {order.customer_phone && (
        <a href={telLink(order.customer_phone)} style={{ color: "var(--color-primary)" }}>
          {order.customer_phone}
        </a>
      )}
      {order.delivery_address && (
        <div style={{ fontSize: "0.9rem" }}>
          {order.delivery_address}
          {order.delivery_map_url && (
            <>
              {" · "}
              <a href={order.delivery_map_url} target="_blank" rel="noreferrer" style={{ color: "var(--color-primary)" }}>
                Mapa
              </a>
            </>
          )}
        </div>
      )}
      {order.courier && order.delivery_status !== "ready" && order.delivery_status !== "preparing" && (
        <div style={{ fontSize: "0.9rem" }}>Repartidor: {order.courier.name}</div>
      )}
      {order.failure_reason && (
        <div style={{ fontSize: "0.9rem", color: "var(--color-danger)" }}>
          {order.delivery_status === "failed" ? "Motivo" : "Último fallo"}: {failureReasonLabel(order.failure_reason)}
        </div>
      )}
      <div style={{ display: "flex", flexWrap: "wrap", gap: "0.35rem", marginTop: "0.25rem" }}>
        {canManage &&
          actionsFor(order).map((action) => (
            <Button
              key={action}
              size="sm"
              variant={action === "fail" ? "danger" : "primary"}
              disabled={busy}
              onClick={() => onAction(action)}
            >
              {ACTION_LABEL[action]}
            </Button>
          ))}
        <Button size="sm" variant="ghost" onClick={onHistory}>
          Historial
        </Button>
        <Button size="sm" variant="ghost" onClick={onPrint}>
          Imprimir
        </Button>
      </div>
    </article>
  );
}

interface DialogProps {
  order: DeliveryOrder;
  onClose: () => void;
  onDone: () => void;
}

function DispatchDialog({ order, onClose, onDone }: DialogProps) {
  const [couriers, setCouriers] = useState<CourierUser[]>([]);
  const [kind, setKind] = useState<"courier" | "third_party">("courier");
  const [courierUserId, setCourierUserId] = useState("");
  const [courierName, setCourierName] = useState("");
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    deliveriesApi
      .couriers()
      .then((list) => setCouriers(list.couriers))
      .catch(() => setError("No se pudieron cargar los repartidores."));
  }, []);

  const submit = async () => {
    const result =
      kind === "courier" ? dispatchRequest({ kind, courierUserId }) : dispatchRequest({ kind, courierName });
    if ("error" in result) {
      setError(result.error);
      return;
    }
    setSaving(true);
    setError("");
    try {
      await deliveriesApi.dispatch(order.id, result.body);
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
      title={`${order.delivery_status === "failed" ? "Reenviar" : "Despachar"} pedido ${orderLabel(order)}`}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>Cancelar</Button>
          <Button onClick={submit} disabled={saving}>Despachar</Button>
        </>
      }
    >
      <div style={{ display: "flex", flexDirection: "column", gap: "0.75rem" }}>
        <div role="radiogroup" style={{ display: "flex", gap: "1rem" }}>
          <label>
            <input type="radio" checked={kind === "courier"} onChange={() => setKind("courier")} /> Repartidor
          </label>
          <label>
            <input type="radio" checked={kind === "third_party"} onChange={() => setKind("third_party")} /> Tercero
          </label>
        </div>
        {kind === "courier" ? (
          <Select label="Repartidor" value={courierUserId} onChange={(e) => setCourierUserId(e.target.value)}>
            <option value="">Elige…</option>
            {couriers.map((c) => (
              <option key={c.id} value={c.id}>{c.name}</option>
            ))}
          </Select>
        ) : (
          <Input
            label="Quién lo lleva"
            placeholder="Mensajeros Ya"
            value={courierName}
            maxLength={MAX_COURIER_NAME}
            onChange={(e) => setCourierName(e.target.value)}
          />
        )}
        {error && <p role="alert" style={{ color: "var(--color-danger)", margin: 0 }}>{error}</p>}
      </div>
    </Modal>
  );
}

function FailDialog({ order, onClose, onDone }: DialogProps) {
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
    setError("");
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
        <Textarea
          label="Nota interna (opcional)"
          helper="Solo la ve el equipo; no se le envía al cliente."
          value={note}
          maxLength={MAX_NOTE}
          onChange={(e) => setNote(e.target.value)}
        />
        {error && <p role="alert" style={{ color: "var(--color-danger)", margin: 0 }}>{error}</p>}
      </div>
    </Modal>
  );
}

function HistoryDialog({ order, onClose }: { order: DeliveryOrder; onClose: () => void }) {
  const [history, setHistory] = useState<DeliveryHistory | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    deliveriesApi
      .history(order.id)
      .then(setHistory)
      .catch((err) => setError(deliveryErrorMessage(err)));
  }, [order.id]);

  return (
    <Modal open onClose={onClose} title={`Historial del pedido ${orderLabel(order)}`}>
      {error && <p role="alert" style={{ color: "var(--color-danger)" }}>{error}</p>}
      {history && (
        <ol style={{ margin: 0, paddingLeft: "1.25rem", display: "flex", flexDirection: "column", gap: "0.5rem" }}>
          <li>Pagado · {formatDateTime(history.paid_at)}</li>
          {history.ready_at && <li>Listo · {formatDateTime(history.ready_at)}</li>}
          {history.entries.map((entry, index) => (
            <li key={index}>
              {STATUS_LABEL[entry.status]} · {formatDateTime(entry.occurred_at)}
              {entry.actor && ` · por ${entry.actor.name}`}
              {entry.courier && entry.status === "out_for_delivery" && ` · con ${entry.courier.name}`}
              {entry.reason && ` · ${failureReasonLabel(entry.reason)}`}
              {entry.note && <div style={{ color: "var(--color-text-secondary)" }}>{entry.note}</div>}
            </li>
          ))}
        </ol>
      )}
    </Modal>
  );
}
