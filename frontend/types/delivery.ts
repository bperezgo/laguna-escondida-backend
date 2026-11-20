import type { OrderType } from "./commandItem.ts";

/** preparing and ready are derived from the kitchen lines; the others are recorded changes. */
export type DeliveryStatus = "preparing" | "ready" | "out_for_delivery" | "delivered" | "failed";

export type DeliveryFailureReason = "customer_unreachable" | "wrong_address" | "refused" | "other";

export type DeliveryAction = "dispatch" | "deliver" | "fail";

export interface DeliveryCourier {
  /** Absent for a third party. */
  user_id?: string;
  name: string;
}

export interface DeliveryOrderLine {
  name: string;
  quantity: number;
  notes?: string;
  status: string;
}

/** Carries no amounts: couriers read it too. */
export interface DeliveryOrder {
  id: string;
  temporal_identifier: string;
  descriptor?: string;
  order_type: OrderType;
  customer_name?: string;
  customer_phone?: string;
  delivery_address?: string;
  delivery_lat?: string;
  delivery_lng?: string;
  delivery_map_url?: string;
  delivery_status: DeliveryStatus;
  status_since: string;
  courier?: DeliveryCourier;
  failure_reason?: DeliveryFailureReason;
  paid: boolean;
  paid_at: string;
  ready_at?: string;
  created_at: string;
  lines: DeliveryOrderLine[];
}

export interface DeliveryOrderListResponse {
  orders: DeliveryOrder[];
}

export interface CourierQueue {
  claimable: DeliveryOrder[];
  on_the_road: DeliveryOrder[];
}

export interface CourierUser {
  id: string;
  name: string;
}

export interface CourierListResponse {
  couriers: CourierUser[];
}

export interface DeliveryHistoryEntry {
  status: DeliveryStatus;
  occurred_at: string;
  actor?: { id: string; name: string };
  courier?: DeliveryCourier;
  reason?: DeliveryFailureReason;
  note?: string;
}

export interface DeliveryHistory {
  order_id: string;
  paid_at: string;
  ready_at?: string;
  entries: DeliveryHistoryEntry[];
}

/** Exactly one of the two; a courier may send neither to mean themselves. */
export interface DispatchDeliveryRequest {
  courier_user_id?: string;
  courier_name?: string;
}

export interface FailDeliveryRequest {
  reason: DeliveryFailureReason;
  note?: string;
}

export interface DeliveryListFilter {
  status?: DeliveryStatus;
  order_type?: OrderType;
}
