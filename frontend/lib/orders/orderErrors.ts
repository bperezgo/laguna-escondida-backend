import type { ApiErrorBody } from "@/lib/api/config";
import { formatInstant } from "../deliveries/schedule.ts";
import { formatSettings } from "../format.ts";

const PAUSED: Record<string, string> = {
  delivery: "Los domicilios están pausados",
  takeaway: "Los pedidos para llevar están pausados",
};

const CLOSED: Record<string, string> = {
  delivery: "Los domicilios están fuera de horario",
  takeaway: "Los pedidos para llevar están fuera de horario",
};

// The backend names the field only in its English message.
const MISSING_FIELD: [string, string][] = [
  ["customer_phone", "el teléfono"],
  ["delivery_address", "la dirección"],
  ["delivery_lat", "la ubicación en el mapa"],
  ["delivery_lng", "la ubicación en el mapa"],
];

/** Spanish message for a refused takeaway/delivery order, or null for any other error. */
export function orderErrorMessage(
  body: ApiErrorBody,
  now: Date = new Date(),
): string | null {
  switch (body.code) {
    case "ORDERING_PAUSED": {
      const label = PAUSED[body.order_type ?? ""] ?? "Este tipo de pedido está pausado";
      return body.paused_reason ? `${label}: "${body.paused_reason}"` : `${label}.`;
    }
    case "OUTSIDE_ORDERING_HOURS": {
      const label = CLOSED[body.order_type ?? ""] ?? "Este tipo de pedido está fuera de horario";
      return body.next_opens_at
        ? `${label}. Abre ${formatInstant(body.next_opens_at, formatSettings.timeZone, now)}.`
        : `${label} y no abre en los próximos 7 días.`;
    }
    case "OUTSIDE_DELIVERY_ZONE":
      return "La ubicación está fuera de la zona de entrega.";
    case "INVALID_PHONE":
      return "El teléfono no es válido.";
    case "ORDER_MISSING_FIELD": {
      const field = MISSING_FIELD.find(([key]) => body.error?.includes(key));
      return field ? `Falta ${field[1]}.` : "Falta un dato obligatorio del pedido.";
    }
    default:
      return null;
  }
}
