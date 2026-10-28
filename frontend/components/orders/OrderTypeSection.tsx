"use client";

import dynamic from "next/dynamic";
import { Input } from "@/components/ui";
import type { OrderType } from "@/types/commandItem";
import type { OrganizationSettingsResponse } from "@/types/organizationSettings";
import { describeAvailability } from "@/lib/deliveries/schedule";
import {
  ORDER_TYPE_OPTIONS,
  type DeliveryPin,
  type OrderContact,
} from "@/lib/orders/orderTypePayload";
import type { PinCheck } from "./useDeliveryPinCheck";

const DeliveryPinPicker = dynamic(() => import("./DeliveryPinPicker"), {
  ssr: false,
  loading: () => (
    <div
      style={{
        height: "220px",
        borderRadius: "var(--radius-md)",
        border: "1px solid var(--color-border)",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        color: "var(--color-text-muted)",
        fontSize: "0.85rem",
      }}
    >
      Cargando mapa…
    </div>
  ),
});

/** Why a type can't be chosen right now, or null when it can (or nothing is known). */
export function unavailableReason(
  type: OrderType,
  settings: OrganizationSettingsResponse | null,
): string | null {
  if (type === "dine_in" || !settings) return null;
  const availability = settings.availability[type];
  return availability.available
    ? null
    : describeAvailability(availability, settings.timezone);
}

function pinCheckLine(check: PinCheck): { text: string; color: string } | null {
  switch (check.status) {
    case "idle":
      return null;
    case "checking":
      return { text: "Verificando la ubicación…", color: "var(--color-text-secondary)" };
    case "inside":
      return { text: `Zona: ${check.zoneName}`, color: "var(--color-success)" };
    case "outside":
      return { text: "Fuera de la zona de entrega", color: "var(--color-danger)" };
    case "error":
      return {
        text: "No se pudo verificar la ubicación; se valida al crear la orden",
        color: "var(--color-warning)",
      };
  }
}

interface OrderTypeSectionProps {
  orderType: OrderType;
  onTypeChange: (type: OrderType) => void;
  contact: OrderContact;
  onContactChange: (contact: OrderContact) => void;
  onPinChange: (pin: DeliveryPin) => void;
  pinCheck: PinCheck;
  settings: OrganizationSettingsResponse | null;
  canReadZones: boolean;
}

export default function OrderTypeSection({
  orderType,
  onTypeChange,
  contact,
  onContactChange,
  onPinChange,
  pinCheck,
  settings,
  canReadZones,
}: OrderTypeSectionProps) {
  const checkLine = pinCheckLine(pinCheck);

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: "0.6rem" }}>
      <div
        role="radiogroup"
        aria-label="Tipo"
        style={{
          display: "flex",
          gap: "4px",
          padding: "4px",
          backgroundColor: "var(--color-bg)",
          borderRadius: "var(--radius-md)",
        }}
      >
        {ORDER_TYPE_OPTIONS.map(({ type, label }) => {
          const on = orderType === type;
          const reason = unavailableReason(type, settings);
          return (
            <button
              key={type}
              type="button"
              role="radio"
              aria-checked={on}
              disabled={reason !== null}
              title={reason ?? undefined}
              onClick={() => onTypeChange(type)}
              style={{
                flex: 1,
                border: "none",
                cursor: reason ? "not-allowed" : "pointer",
                fontFamily: "inherit",
                fontSize: "0.8rem",
                fontWeight: 700,
                padding: "0.45rem 0.3rem",
                borderRadius: "var(--radius-sm)",
                backgroundColor: on ? "var(--color-surface)" : "transparent",
                color: reason
                  ? "var(--color-text-muted)"
                  : on
                    ? "var(--color-text-primary)"
                    : "var(--color-text-secondary)",
                boxShadow: on ? "var(--shadow-sm)" : "none",
                display: "flex",
                flexDirection: "column",
                alignItems: "center",
                gap: "2px",
              }}
            >
              {label}
              {reason && (
                <span style={{ fontSize: "0.65rem", fontWeight: 500, lineHeight: 1.2 }}>
                  {reason}
                </span>
              )}
            </button>
          );
        })}
      </div>

      {orderType !== "dine_in" && (
        <details open>
          <summary
            style={{
              cursor: "pointer",
              fontSize: "0.85rem",
              fontWeight: 700,
              color: "var(--color-text-secondary)",
              marginBottom: "0.5rem",
            }}
          >
            Datos del cliente
          </summary>
          <div style={{ display: "flex", flexDirection: "column", gap: "0.6rem" }}>
            <Input
              label="Teléfono *"
              type="tel"
              inputMode="tel"
              value={contact.customerPhone}
              onChange={(e) => onContactChange({ ...contact, customerPhone: e.target.value })}
              placeholder="Ej: 300 555 1234"
            />
            <Input
              label="Nombre"
              type="text"
              value={contact.customerName}
              onChange={(e) => onContactChange({ ...contact, customerName: e.target.value })}
              placeholder="Opcional"
            />
            {orderType === "delivery" && (
              <>
                <Input
                  label="Dirección *"
                  type="text"
                  value={contact.deliveryAddress}
                  onChange={(e) =>
                    onContactChange({ ...contact, deliveryAddress: e.target.value })
                  }
                  placeholder="Calle, edificio, apartamento…"
                />
                <div style={{ fontSize: "0.8rem", color: "var(--color-text-secondary)" }}>
                  Ubicación * — toca el mapa o arrastra el pin
                </div>
                <DeliveryPinPicker
                  pin={contact.pin}
                  onPinChange={onPinChange}
                  showZones={canReadZones}
                />
                {checkLine && (
                  <div style={{ fontSize: "0.8rem", fontWeight: 600, color: checkLine.color }}>
                    {checkLine.text}
                  </div>
                )}
              </>
            )}
          </div>
        </details>
      )}
    </div>
  );
}
