import type { OrderType } from "@/types/commandItem";
import type { CreateOrderRequest } from "@/types/order";

export const ORDER_TYPE_OPTIONS: { type: OrderType; label: string }[] = [
  { type: "dine_in", label: "En el restaurante" },
  { type: "takeaway", label: "Para llevar" },
  { type: "delivery", label: "Domicilio" },
];

// Frontend-only convention: the backend accepts any temporal identifier.
const IDENTIFIER_PREFIX: Record<OrderType, string> = {
  dine_in: "MESA-",
  takeaway: "LLEVAR-",
  delivery: "DOM-",
};

export function identifierPrefix(type: OrderType): string {
  return IDENTIFIER_PREFIX[type];
}

/** Follows the new type only while the user hasn't typed past a prefix. */
export function retypeIdentifier(current: string, to: OrderType): string {
  const trimmed = current.trim();
  const untouched =
    trimmed === "" || Object.values(IDENTIFIER_PREFIX).includes(trimmed);
  return untouched ? IDENTIFIER_PREFIX[to] : current;
}

export interface DeliveryPin {
  lat: number;
  lng: number;
}

export interface OrderContact {
  customerName: string;
  customerPhone: string;
  deliveryAddress: string;
  pin: DeliveryPin | null;
}

export const EMPTY_CONTACT: OrderContact = {
  customerName: "",
  customerPhone: "",
  deliveryAddress: "",
  pin: null,
};

type BaseOrder = Pick<
  CreateOrderRequest,
  "open_bill_id" | "temporal_identifier" | "descriptor" | "products"
>;

/** Sends only the selected type's fields; dine-in stays exactly today's payload. */
export function buildCreateOrderRequest(
  base: BaseOrder,
  type: OrderType,
  contact: OrderContact,
): CreateOrderRequest {
  if (type === "dine_in") return base;

  const request: CreateOrderRequest = {
    ...base,
    order_type: type,
    customer_phone: contact.customerPhone.trim(),
  };
  const name = contact.customerName.trim();
  if (name) request.customer_name = name;
  if (type === "delivery") {
    request.delivery_address = contact.deliveryAddress.trim();
    if (contact.pin) {
      request.delivery_lat = contact.pin.lat.toFixed(6);
      request.delivery_lng = contact.pin.lng.toFixed(6);
    }
  }
  return request;
}

/** Spanish names of the required fields still empty for this type. */
export function missingOrderFields(
  type: OrderType,
  contact: OrderContact,
): string[] {
  if (type === "dine_in") return [];
  const missing: string[] = [];
  if (!contact.customerPhone.trim()) missing.push("teléfono");
  if (type === "delivery") {
    if (!contact.deliveryAddress.trim()) missing.push("dirección");
    if (!contact.pin) missing.push("ubicación en el mapa");
  }
  return missing;
}
