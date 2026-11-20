"use client";

import { forwardRef, useEffect, useState } from "react";
import QRCode from "qrcode";
import type { DeliveryOrder } from "@/types/delivery";
import { orderLabel } from "@/lib/deliveries/board";

interface BagSlipProps {
  order: DeliveryOrder;
  /** Called once the slip, QR included, is on the page and can be printed. */
  onReady?: () => void;
}

/**
 * The bag label: browserPrint copies this node's innerHTML. The QR is drawn here, never by a
 * QR service, since the map link is the customer's address. No amounts: only that it's paid.
 */
const BagSlip = forwardRef<HTMLDivElement, BagSlipProps>(function BagSlip({ order, onReady }, ref) {
  const [qr, setQr] = useState<string | null>(null);
  const ready = !order.delivery_map_url || qr !== null;

  useEffect(() => {
    if (ready) onReady?.();
  }, [ready, onReady]);

  useEffect(() => {
    let cancelled = false;
    if (!order.delivery_map_url) return;
    QRCode.toDataURL(order.delivery_map_url, { margin: 1, width: 160 })
      .then((url) => {
        if (!cancelled) setQr(url);
      })
      .catch(() => {
        if (!cancelled) setQr("");
      });
    return () => {
      cancelled = true;
    };
  }, [order.delivery_map_url]);

  return (
    <div ref={ref}>
      <div className="header">
        <div className="bill-number">Pedido {orderLabel(order)}</div>
        <div className="date">{order.order_type === "delivery" ? "DOMICILIO" : "PARA LLEVAR"} · PAGADO</div>
      </div>
      <div className="items">
        {order.customer_name && <div className="item-name">{order.customer_name}</div>}
        {order.customer_phone && <div>{order.customer_phone}</div>}
        {order.delivery_address && <div>{order.delivery_address}</div>}
      </div>
      {qr && (
        <div style={{ textAlign: "center", margin: "8px 0" }}>
          {/* eslint-disable-next-line @next/next/no-img-element -- a data URL copied into the print window */}
          <img src={qr} alt="Mapa de la dirección" width={160} height={160} />
        </div>
      )}
      <div className="items">
        {order.lines.map((line, index) => (
          <div key={index} className="item">
            <div className="item-details">
              <span>{line.quantity} × {line.name}</span>
            </div>
            {line.notes && <div>{line.notes}</div>}
          </div>
        ))}
      </div>
    </div>
  );
});

export default BagSlip;
