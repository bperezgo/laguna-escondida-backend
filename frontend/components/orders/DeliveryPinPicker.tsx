"use client";

import { useEffect, useRef } from "react";
import L from "leaflet";
import "leaflet/dist/leaflet.css";
import { deliveryZonesApi } from "@/lib/api/deliveryZones";
import { DEFAULT_MAP_CENTER, DEFAULT_MAP_ZOOM } from "@/lib/config/map";
import type { DeliveryPin } from "@/lib/orders/orderTypePayload";

// A div icon: Leaflet's default marker images don't survive the bundler.
const PIN_ICON = L.divIcon({
  className: "",
  html: '<div style="width:22px;height:22px;border-radius:50%;background:#dc2626;border:3px solid #fff;box-shadow:0 1px 4px rgba(0,0,0,.45)"></div>',
  iconSize: [22, 22],
  iconAnchor: [11, 11],
});

interface DeliveryPinPickerProps {
  pin: DeliveryPin | null;
  onPinChange: (pin: DeliveryPin) => void;
  /** Waiters can't list zones; they still get the check, just no outlines. */
  showZones: boolean;
}

export default function DeliveryPinPicker({
  pin,
  onPinChange,
  showZones,
}: DeliveryPinPickerProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const mapRef = useRef<L.Map | null>(null);
  const markerRef = useRef<L.Marker | null>(null);
  const onPinChangeRef = useRef(onPinChange);
  const initialPinRef = useRef(pin);
  useEffect(() => {
    onPinChangeRef.current = onPinChange;
  });

  useEffect(() => {
    if (!containerRef.current) return;
    const initialPin = initialPinRef.current;
    const map = L.map(containerRef.current, {
      center: initialPin ? [initialPin.lat, initialPin.lng] : DEFAULT_MAP_CENTER,
      zoom: initialPin ? 16 : DEFAULT_MAP_ZOOM,
    });
    L.tileLayer("https://tile.openstreetmap.org/{z}/{x}/{y}.png", {
      maxZoom: 19,
      attribution:
        '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>',
    }).addTo(map);
    map.on("click", (e: L.LeafletMouseEvent) =>
      onPinChangeRef.current({ lat: e.latlng.lat, lng: e.latlng.lng }),
    );
    mapRef.current = map;

    let cancelled = false;
    if (showZones) {
      deliveryZonesApi
        .getAll()
        .then((zones) => {
          if (cancelled) return;
          const all = L.latLngBounds([]);
          for (const zone of zones.filter((z) => z.active)) {
            const layer = L.geoJSON(zone.boundary, {
              interactive: false,
              style: { color: "#0d9488", weight: 2, fillOpacity: 0.12 },
            }).addTo(map);
            all.extend(layer.getBounds());
          }
          if (!initialPin && all.isValid()) map.fitBounds(all, { padding: [16, 16] });
        })
        .catch(() => {
          // No outlines; the check still decides deliverability.
        });
    }

    return () => {
      cancelled = true;
      map.remove();
      mapRef.current = null;
      markerRef.current = null;
    };
  }, [showZones]);

  useEffect(() => {
    const map = mapRef.current;
    if (!map) return;
    if (!pin) {
      markerRef.current?.remove();
      markerRef.current = null;
      return;
    }
    if (markerRef.current) {
      markerRef.current.setLatLng([pin.lat, pin.lng]);
      return;
    }
    const marker = L.marker([pin.lat, pin.lng], { icon: PIN_ICON, draggable: true }).addTo(map);
    marker.on("dragend", () => {
      const { lat, lng } = marker.getLatLng();
      onPinChangeRef.current({ lat, lng });
    });
    markerRef.current = marker;
  }, [pin, showZones]);

  return (
    <div
      ref={containerRef}
      style={{
        width: "100%",
        height: "220px",
        borderRadius: "var(--radius-md)",
        border: "1px solid var(--color-border)",
        // Leaflet panes use z-index up to 1000; keep them inside the dialog.
        isolation: "isolate",
        cursor: "crosshair",
      }}
    />
  );
}
