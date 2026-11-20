import type {
  DeliveryFailureReason,
  DeliveryOrder,
  DeliveryStatus,
} from "../../types/delivery.ts";
import { DeliveryApiError } from "../api/deliveries.ts";

export type DeliveriesTab = "pedidos" | "estado" | "horarios" | "zonas";

export const DELIVERY_PERMISSIONS = {
  read: "deliveries:read",
  manage: "deliveries:manage",
  carry: "deliveries:carry",
} as const;

/** "Pedidos" leads when the user can read the dashboard. */
export function deliveriesTabs(hasPermission: (permission: string) => boolean): DeliveriesTab[] {
  return [
    ...(hasPermission(DELIVERY_PERMISSIONS.read) ? (["pedidos"] as const) : []),
    ...(hasPermission("ordering:read") ? (["estado", "horarios"] as const) : []),
    ...(hasPermission("delivery-zones:read") ? (["zonas"] as const) : []),
  ];
}

export const STATUS_COLUMNS: { status: DeliveryStatus; label: string }[] = [
  { status: "preparing", label: "Preparando" },
  { status: "ready", label: "Listo" },
  { status: "out_for_delivery", label: "En camino" },
  { status: "failed", label: "Fallido" },
  { status: "delivered", label: "Entregado hoy" },
];

export const STATUS_LABEL: Record<DeliveryStatus, string> = {
  preparing: "Preparando",
  ready: "Listo",
  out_for_delivery: "En camino",
  delivered: "Entregado",
  failed: "Fallido",
};

export const FAILURE_REASONS: { reason: DeliveryFailureReason; label: string }[] = [
  { reason: "customer_unreachable", label: "No contesta / no está" },
  { reason: "wrong_address", label: "Dirección errada" },
  { reason: "refused", label: "Rechazó el pedido" },
  { reason: "other", label: "Otro" },
];

export function failureReasonLabel(reason: DeliveryFailureReason): string {
  return FAILURE_REASONS.find((r) => r.reason === reason)?.label ?? reason;
}

/** Each column oldest first in its status, so the order waiting longest is on top. */
export function groupByStatus(orders: DeliveryOrder[]): Record<DeliveryStatus, DeliveryOrder[]> {
  const groups: Record<DeliveryStatus, DeliveryOrder[]> = {
    preparing: [],
    ready: [],
    out_for_delivery: [],
    failed: [],
    delivered: [],
  };
  for (const order of orders) {
    groups[order.delivery_status]?.push(order);
  }
  for (const list of Object.values(groups)) {
    list.sort((a, b) => Date.parse(a.status_since) - Date.parse(b.status_since));
  }
  return groups;
}

export function timeInStatus(since: string, now: Date = new Date()): string {
  const minutes = Math.max(0, Math.floor((now.getTime() - Date.parse(since)) / 60000));
  if (minutes < 1) return "ahora";
  if (minutes < 60) return `${minutes} min`;
  const hours = Math.floor(minutes / 60);
  const rest = minutes % 60;
  if (hours >= 24) return `${Math.floor(hours / 24)} d`;
  return rest === 0 ? `${hours} h` : `${hours} h ${rest} min`;
}

export type BoardAction = "dispatch" | "redispatch" | "deliver" | "pickup" | "fail";

export const ACTION_LABEL: Record<BoardAction, string> = {
  dispatch: "Despachar",
  redispatch: "Reenviar",
  deliver: "Entregado",
  pickup: "Recogido",
  fail: "No se pudo entregar",
};

/** The transitions a manager may make from the order's status (the backend's table). */
export function actionsFor(order: DeliveryOrder): BoardAction[] {
  const notOut = order.delivery_status === "preparing" || order.delivery_status === "ready";
  if (order.order_type === "takeaway") {
    return notOut ? ["pickup"] : [];
  }
  switch (order.delivery_status) {
    case "preparing":
    case "ready":
      return ["dispatch"];
    case "out_for_delivery":
      return ["deliver", "fail"];
    case "failed":
      return ["redispatch"];
    default:
      return [];
  }
}

export const MAX_COURIER_NAME = 80;
export const MAX_NOTE = 500;

export type DispatchForm =
  | { kind: "courier"; courierUserId: string }
  | { kind: "third_party"; courierName: string };

/** Returns the request body, or the error to show. */
export function dispatchRequest(
  form: DispatchForm,
): { body: { courier_user_id: string } | { courier_name: string } } | { error: string } {
  if (form.kind === "courier") {
    if (!form.courierUserId) return { error: "Elige un repartidor." };
    return { body: { courier_user_id: form.courierUserId } };
  }
  const name = form.courierName.trim();
  if (!name) return { error: "Escribe el nombre de quien lo lleva." };
  if (Array.from(name).length > MAX_COURIER_NAME) {
    return { error: `El nombre puede tener hasta ${MAX_COURIER_NAME} caracteres.` };
  }
  return { body: { courier_name: name } };
}

export function failRequest(
  reason: DeliveryFailureReason | "",
  note: string,
): { body: { reason: DeliveryFailureReason; note?: string } } | { error: string } {
  if (!reason) return { error: "Elige el motivo." };
  const trimmed = note.trim();
  if (Array.from(trimmed).length > MAX_NOTE) {
    return { error: `La nota puede tener hasta ${MAX_NOTE} caracteres.` };
  }
  return { body: trimmed ? { reason, note: trimmed } : { reason } };
}

/** A lost claim reads as someone else's; any other 409 names where the order is now. */
export function deliveryErrorMessage(err: unknown, claiming = false): string {
  if (err instanceof DeliveryApiError) {
    if (err.status === 409 && err.code === "INVALID_DELIVERY_TRANSITION") {
      if (claiming && err.currentStatus === "out_for_delivery") return "Otro repartidor ya lo tomó.";
      return err.currentStatus
        ? `El pedido cambió: ahora está ${STATUS_LABEL[err.currentStatus].toLowerCase()}. Recarga la lista.`
        : "Este pedido ya no se puede mover.";
    }
    if (err.status === 403) return "No puedes hacer este cambio en este pedido.";
    if (err.status === 404) return "El pedido no existe o solo se gestiona en la nube.";
    if (err.status === 400 && err.field === "courier_user_id") return "Ese usuario no es un repartidor activo.";
    if (err.status === 400) return err.message;
  }
  return "No se pudo guardar el cambio. Intenta de nuevo.";
}

export function orderLabel(order: Pick<DeliveryOrder, "temporal_identifier" | "descriptor">): string {
  return order.descriptor || order.temporal_identifier.slice(-6).toUpperCase();
}

export function telLink(phone: string): string {
  return `tel:${phone.replace(/[^\d+]/g, "")}`;
}
