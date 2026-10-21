"use client";

import { useEffect, useRef } from "react";
import L from "leaflet";
import "@geoman-io/leaflet-geoman-free";
import "leaflet/dist/leaflet.css";
import "@geoman-io/leaflet-geoman-free/dist/leaflet-geoman.css";
import type {
  DeliveryPointCheck,
  DeliveryZone,
  ZoneBoundary,
} from "@/types/deliveryZone";
import { parseBoundary } from "@/lib/deliveryZones/geojson";

// Bogotá; only used until there is a zone to fit the view to.
const DEFAULT_CENTER: L.LatLngExpression = [4.6, -74.08];
const DEFAULT_ZOOM = 12;

const ACTIVE_STYLE: L.PathOptions = {
  color: "#0d9488",
  weight: 2,
  fillColor: "#14b8a6",
  fillOpacity: 0.2,
};
const INACTIVE_STYLE: L.PathOptions = {
  color: "#64748b",
  weight: 2,
  dashArray: "6 6",
  fillColor: "#94a3b8",
  fillOpacity: 0.1,
};
const EDITING_STYLE: L.PathOptions = {
  color: "#c9a227",
  weight: 3,
  fillColor: "#c9a227",
  fillOpacity: 0.25,
};

export interface ZoneEditState {
  zoneId: string | null;
  boundary: ZoneBoundary | null;
  /** Bumped when the boundary changes outside the map (e.g. pasted GeoJSON) so the map redraws it. */
  revision: number;
}

export interface CheckedPoint {
  lat: number;
  lng: number;
  result: DeliveryPointCheck | null;
}

interface ZoneMapProps {
  zones: DeliveryZone[];
  selectedZoneId: string | null;
  onSelectZone: (id: string) => void;
  editing: ZoneEditState | null;
  onBoundaryChange: (boundary: ZoneBoundary | null) => void;
  checkedPoint: CheckedPoint | null;
  onMapClick: (lat: number, lng: number) => void;
}

export default function ZoneMap({
  zones,
  selectedZoneId,
  onSelectZone,
  editing,
  onBoundaryChange,
  checkedPoint,
  onMapClick,
}: ZoneMapProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const mapRef = useRef<L.Map | null>(null);
  const zonesLayerRef = useRef<L.LayerGroup | null>(null);
  const pointLayerRef = useRef<L.LayerGroup | null>(null);
  const zoneBoundsRef = useRef<Map<string, L.LatLngBounds>>(new Map());
  const didFitRef = useRef(false);
  const watchLayerRef = useRef<(layer: L.Layer) => void>(() => {});

  // Handlers change every render; the map's listeners are bound once.
  const callbacksRef = useRef({ onSelectZone, onBoundaryChange, onMapClick });
  const isEditingRef = useRef(false);
  const editBoundaryRef = useRef<ZoneBoundary | null>(null);
  useEffect(() => {
    callbacksRef.current = { onSelectZone, onBoundaryChange, onMapClick };
    isEditingRef.current = editing !== null;
    editBoundaryRef.current = editing?.boundary ?? null;
  });

  useEffect(() => {
    if (!containerRef.current) return;
    const map = L.map(containerRef.current, {
      center: DEFAULT_CENTER,
      zoom: DEFAULT_ZOOM,
    });
    L.tileLayer("https://tile.openstreetmap.org/{z}/{x}/{y}.png", {
      maxZoom: 19,
      attribution:
        '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>',
    }).addTo(map);

    zonesLayerRef.current = L.layerGroup().addTo(map);
    pointLayerRef.current = L.layerGroup().addTo(map);
    map.pm.setLang("es");
    map.pm.setGlobalOptions({ allowSelfIntersection: false });

    const emitBoundary = () => {
      const features = map.pm
        .getGeomanLayers()
        .filter((layer): layer is L.Polygon => layer instanceof L.Polygon)
        .map((layer) => layer.toGeoJSON());
      const parsed = parseBoundary({ type: "FeatureCollection", features });
      callbacksRef.current.onBoundaryChange(parsed.ok ? parsed.boundary : null);
    };
    const watchLayer = (layer: L.Layer) => {
      layer.on("pm:edit pm:dragend pm:rotateend", emitBoundary);
    };

    map.on("pm:create", (e) => {
      if (e.layer instanceof L.Path) e.layer.setStyle(EDITING_STYLE);
      watchLayer(e.layer);
      emitBoundary();
    });
    map.on("pm:cut", (e) => {
      watchLayer(e.layer);
      emitBoundary();
    });
    map.on("pm:remove", emitBoundary);
    map.on("click", (e: L.LeafletMouseEvent) => {
      if (!isEditingRef.current) {
        callbacksRef.current.onMapClick(e.latlng.lat, e.latlng.lng);
      }
    });

    mapRef.current = map;
    watchLayerRef.current = watchLayer;

    return () => {
      map.remove();
      mapRef.current = null;
    };
  }, []);

  // Read-only zones. The one being edited is drawn by the editing effect instead.
  useEffect(() => {
    const map = mapRef.current;
    const group = zonesLayerRef.current;
    if (!map || !group) return;

    group.clearLayers();
    zoneBoundsRef.current.clear();
    const all = L.latLngBounds([]);

    for (const zone of zones) {
      if (editing && editing.zoneId === zone.id) continue;
      const selected = zone.id === selectedZoneId;
      const base = zone.active ? ACTIVE_STYLE : INACTIVE_STYLE;
      const layer = L.geoJSON(zone.boundary, {
        pmIgnore: true,
        style: {
          ...base,
          weight: selected ? 4 : base.weight,
          fillOpacity: editing ? 0.05 : selected ? 0.35 : base.fillOpacity,
        },
      });
      layer.bindTooltip(
        zone.active ? zone.name : `${zone.name} (inactiva)`,
        { sticky: true },
      );
      layer.on("click", (e: L.LeafletMouseEvent) => {
        if (isEditingRef.current) return;
        L.DomEvent.stopPropagation(e);
        callbacksRef.current.onSelectZone(zone.id);
      });
      layer.addTo(group);

      const bounds = layer.getBounds();
      if (bounds.isValid()) {
        zoneBoundsRef.current.set(zone.id, bounds);
        all.extend(bounds);
      }
    }

    if (!didFitRef.current && !editing && all.isValid()) {
      map.fitBounds(all, { padding: [24, 24] });
      didFitRef.current = true;
    }
  }, [zones, selectedZoneId, editing]);

  useEffect(() => {
    const map = mapRef.current;
    if (!map || !selectedZoneId) return;
    const bounds = zoneBoundsRef.current.get(selectedZoneId);
    if (bounds) map.fitBounds(bounds, { padding: [24, 24], maxZoom: 16 });
  }, [selectedZoneId]);

  // Editable layers: rebuilt only when the edit session starts or the revision bumps,
  // never on the map's own changes, so in-progress vertex drags aren't reset.
  const editKey = editing ? `${editing.zoneId ?? "new"}:${editing.revision}` : null;

  useEffect(() => {
    const map = mapRef.current;
    if (!map) return;

    map.pm.getGeomanLayers().forEach((layer) => layer.remove());

    if (editKey === null) {
      map.pm.disableDraw();
      map.pm.disableGlobalEditMode();
      map.pm.disableGlobalRemovalMode();
      map.pm.disableGlobalCutMode();
      map.pm.disableGlobalDragMode();
      map.pm.removeControls();
      return;
    }

    map.pm.addControls({
      position: "topleft",
      drawMarker: false,
      drawCircleMarker: false,
      drawPolyline: false,
      drawCircle: false,
      drawText: false,
      rotateMode: false,
    });

    const boundary = editBoundaryRef.current;
    if (boundary) {
      const group = L.geoJSON(boundary, { style: EDITING_STYLE });
      group.eachLayer((layer) => {
        layer.addTo(map);
        watchLayerRef.current(layer);
      });
      const bounds = group.getBounds();
      if (bounds.isValid()) map.fitBounds(bounds, { padding: [24, 24] });
    }
  }, [editKey]);

  useEffect(() => {
    const group = pointLayerRef.current;
    if (!group) return;
    group.clearLayers();
    if (!checkedPoint) return;

    const { lat, lng, result } = checkedPoint;
    const color = !result ? "#64748b" : result.deliverable ? "#16a34a" : "#dc2626";
    const label = !result
      ? "Verificando…"
      : result.deliverable
        ? `Se entrega · ${result.zone_name ?? "zona"}`
        : "Fuera de zona: no se entrega";
    L.circleMarker([lat, lng], {
      radius: 9,
      color: "#ffffff",
      weight: 2,
      fillColor: color,
      fillOpacity: 1,
      pmIgnore: true,
    })
      .bindTooltip(label, { permanent: true, direction: "top", offset: [0, -10] })
      .addTo(group);
  }, [checkedPoint]);

  return (
    <div
      ref={containerRef}
      style={{
        width: "100%",
        height: "100%",
        minHeight: "420px",
        borderRadius: "var(--radius-md)",
        border: "1px solid var(--color-border)",
        // Leaflet panes use z-index up to 1000; keep them under the sticky header.
        isolation: "isolate",
        cursor: editing ? undefined : "crosshair",
      }}
    />
  );
}
