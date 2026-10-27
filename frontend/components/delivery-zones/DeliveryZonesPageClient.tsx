"use client";

import { useState } from "react";
import dynamic from "next/dynamic";
import type { DeliveryZone } from "@/types/deliveryZone";
import { deliveryZonesApi } from "@/lib/api/deliveryZones";
import { describeBoundary, geojsonIoUrl } from "@/lib/deliveryZones/geojson";
import { Badge, Button, Card, CardBody, Input } from "@/components/ui";
import { PermissionGate } from "@/components/permissions";
import { PERMISSIONS, usePermissions } from "@/lib/permissions";
import buttonStyles from "@/components/ui/Button.module.css";
import styles from "./DeliveryZones.module.css";
import ZoneEditor from "./ZoneEditor";
import type { CheckedPoint, ZoneEditState } from "./ZoneMap";

// Leaflet touches `window` on import.
const ZoneMap = dynamic(() => import("./ZoneMap"), {
  ssr: false,
  loading: () => (
    <div
      style={{
        height: "100%",
        minHeight: "420px",
        display: "grid",
        placeItems: "center",
        color: "var(--color-text-muted)",
        border: "1px solid var(--color-border)",
        borderRadius: "var(--radius-md)",
      }}
    >
      Cargando mapa…
    </div>
  ),
});

interface DeliveryZonesPageClientProps {
  initialZones: DeliveryZone[];
  loadError: string;
  /** Inside the Domicilios page's Zonas tab: that page owns the title and padding. */
  embedded?: boolean;
}

interface Draft extends ZoneEditState {
  name: string;
  active: boolean;
}

export default function DeliveryZonesPageClient({
  initialZones,
  loadError,
  embedded = false,
}: DeliveryZonesPageClientProps) {
  const { hasPermission } = usePermissions();
  const canManage = hasPermission(PERMISSIONS.DELIVERY_ZONES_MANAGE);

  const [zones, setZones] = useState<DeliveryZone[]>(initialZones);
  const [error, setError] = useState(loadError);
  const [selectedZoneId, setSelectedZoneId] = useState<string | null>(null);
  const [draft, setDraft] = useState<Draft | null>(null);
  const [saving, setSaving] = useState(false);
  const [checkedPoint, setCheckedPoint] = useState<CheckedPoint | null>(null);
  const [checkLat, setCheckLat] = useState("");
  const [checkLng, setCheckLng] = useState("");

  const reload = async () => {
    setZones(await deliveryZonesApi.getAll());
  };

  const startCreate = () => {
    setError("");
    setSelectedZoneId(null);
    setCheckedPoint(null);
    setDraft({ zoneId: null, name: "", active: true, boundary: null, revision: 0 });
  };

  const startEdit = (zone: DeliveryZone) => {
    setError("");
    setSelectedZoneId(zone.id);
    setCheckedPoint(null);
    setDraft({
      zoneId: zone.id,
      name: zone.name,
      active: zone.active,
      boundary: zone.boundary,
      revision: 0,
    });
  };

  const handleSave = async () => {
    if (!draft?.boundary) return;
    try {
      setSaving(true);
      setError("");
      const body = {
        name: draft.name.trim(),
        boundary: draft.boundary,
        active: draft.active,
      };
      const saved = draft.zoneId
        ? await deliveryZonesApi.update(draft.zoneId, body)
        : await deliveryZonesApi.create(body);
      await reload();
      setDraft(null);
      setSelectedZoneId(saved.id);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Error al guardar la zona");
    } finally {
      setSaving(false);
    }
  };

  const handleToggleActive = async (zone: DeliveryZone) => {
    try {
      setError("");
      await deliveryZonesApi.update(zone.id, { active: !zone.active });
      await reload();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Error al actualizar la zona");
    }
  };

  const handleDelete = async (zone: DeliveryZone) => {
    if (
      !confirm(
        `¿Eliminar la zona "${zone.name}"? Las órdenes ya registradas en ella no cambian.`,
      )
    ) {
      return;
    }
    try {
      setError("");
      await deliveryZonesApi.delete(zone.id);
      if (selectedZoneId === zone.id) setSelectedZoneId(null);
      await reload();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Error al eliminar la zona");
    }
  };

  const checkPoint = async (lat: number, lng: number) => {
    setCheckedPoint({ lat, lng, result: null });
    setCheckLat(lat.toFixed(6));
    setCheckLng(lng.toFixed(6));
    try {
      const result = await deliveryZonesApi.check(lat, lng);
      setCheckedPoint({ lat, lng, result });
    } catch (err) {
      setCheckedPoint(null);
      setError(err instanceof Error ? err.message : "Error al verificar el punto");
    }
  };

  const handleCheckSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    const lat = Number(checkLat);
    const lng = Number(checkLng);
    if (!checkLat || !checkLng || !Number.isFinite(lat) || !Number.isFinite(lng)) {
      setError("Ingresa latitud y longitud en grados decimales");
      return;
    }
    checkPoint(lat, lng);
  };

  const activeCount = zones.filter((z) => z.active).length;

  return (
    <div
      style={
        embedded
          ? undefined
          : { minHeight: "100vh", backgroundColor: "var(--color-bg)", padding: "2rem" }
      }
    >
      <div style={{ maxWidth: "1400px", margin: "0 auto" }}>
        <div
          style={{
            display: "flex",
            justifyContent: "space-between",
            alignItems: "center",
            marginBottom: "1.5rem",
            flexWrap: "wrap",
            gap: "1rem",
          }}
        >
          <div>
            {!embedded && (
              <h1
                style={{
                  margin: 0,
                  fontSize: "2rem",
                  fontWeight: "bold",
                  color: "var(--color-text-primary)",
                }}
              >
                Zonas de Domicilio
              </h1>
            )}
            <p
              style={{
                margin: embedded ? 0 : "0.5rem 0 0 0",
                color: "var(--color-text-secondary)",
                fontSize: "1rem",
              }}
            >
              Un domicilio se acepta solo si su ubicación cae dentro de una zona activa.
            </p>
          </div>
          <div style={{ display: "flex", gap: "0.5rem", flexWrap: "wrap" }}>
            {zones.length > 0 && (
              <a
                href={geojsonIoUrl(zones)}
                target="_blank"
                rel="noopener noreferrer"
                className={`${buttonStyles.btn} ${buttonStyles.secondary} ${buttonStyles.md}`}
                style={{ textDecoration: "none" }}
              >
                Ver todas en geojson.io ↗
              </a>
            )}
            <PermissionGate permission={PERMISSIONS.DELIVERY_ZONES_MANAGE}>
              <Button onClick={startCreate} disabled={draft !== null}>
                + Nueva zona
              </Button>
            </PermissionGate>
          </div>
        </div>

        {error && (
          <div
            style={{
              padding: "1rem",
              backgroundColor: "var(--color-danger-light)",
              color: "var(--color-danger)",
              border: "1px solid var(--color-danger)",
              borderRadius: "var(--radius-sm)",
              marginBottom: "1.5rem",
            }}
          >
            <strong>Error:</strong> {error}
          </div>
        )}

        {!loadError && activeCount === 0 && !draft && (
          <div
            style={{
              padding: "1rem",
              backgroundColor: "var(--color-warning-bg)",
              color: "var(--color-warning-text)",
              borderRadius: "var(--radius-sm)",
              marginBottom: "1.5rem",
            }}
          >
            No hay zonas activas: ahora mismo no se acepta ningún domicilio.
          </div>
        )}

        <div className={styles.layout}>
          <div className={styles.sidebar}>
            {draft ? (
              <ZoneEditor
                key={draft.zoneId ?? "new"}
                isNew={draft.zoneId === null}
                name={draft.name}
                active={draft.active}
                boundary={draft.boundary}
                onNameChange={(name) => setDraft((d) => d && { ...d, name })}
                onActiveChange={(active) => setDraft((d) => d && { ...d, active })}
                onBoundaryImport={(boundary) =>
                  setDraft((d) => d && { ...d, boundary, revision: d.revision + 1 })
                }
                onSave={handleSave}
                onCancel={() => setDraft(null)}
                isSaving={saving}
              />
            ) : (
              <>
                {zones.length === 0 && !loadError && (
                  <Card>
                    <CardBody>
                      <p style={{ margin: 0, color: "var(--color-text-secondary)" }}>
                        Aún no hay zonas.
                        {canManage && " Crea una con “+ Nueva zona” y dibújala en el mapa."}
                      </p>
                    </CardBody>
                  </Card>
                )}
                {zones.map((zone) => (
                  <ZoneCard
                    key={zone.id}
                    zone={zone}
                    selected={zone.id === selectedZoneId}
                    canManage={canManage}
                    onSelect={() => setSelectedZoneId(zone.id)}
                    onEdit={() => startEdit(zone)}
                    onToggleActive={() => handleToggleActive(zone)}
                    onDelete={() => handleDelete(zone)}
                  />
                ))}

                {!loadError && (
                  <Card>
                    <CardBody>
                      <form
                        onSubmit={handleCheckSubmit}
                        style={{ display: "flex", flexDirection: "column", gap: "0.75rem" }}
                      >
                        <div>
                          <h2
                            style={{
                              margin: 0,
                              fontSize: "1rem",
                              fontWeight: 600,
                              color: "var(--color-text-primary)",
                            }}
                          >
                            Verificar un punto
                          </h2>
                          <p
                            style={{
                              margin: "0.25rem 0 0",
                              fontSize: "0.8125rem",
                              color: "var(--color-text-secondary)",
                            }}
                          >
                            Haz clic en el mapa o ingresa coordenadas. Tras cambiar una
                            zona, verifica la ubicación del restaurante.
                          </p>
                        </div>
                        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "0.5rem" }}>
                          <Input
                            label="Latitud"
                            value={checkLat}
                            onChange={(e) => setCheckLat(e.target.value)}
                            placeholder="4.6097"
                            inputMode="decimal"
                          />
                          <Input
                            label="Longitud"
                            value={checkLng}
                            onChange={(e) => setCheckLng(e.target.value)}
                            placeholder="-74.0817"
                            inputMode="decimal"
                          />
                        </div>
                        <div style={{ display: "flex", alignItems: "center", gap: "0.75rem", flexWrap: "wrap" }}>
                          <Button type="submit" variant="secondary" size="sm">
                            Verificar
                          </Button>
                          {checkedPoint?.result && (
                            <Badge tone={checkedPoint.result.deliverable ? "success" : "danger"}>
                              {checkedPoint.result.deliverable
                                ? `Se entrega · ${checkedPoint.result.zone_name ?? ""}`
                                : "No se entrega"}
                            </Badge>
                          )}
                        </div>
                      </form>
                    </CardBody>
                  </Card>
                )}
              </>
            )}
          </div>

          <div className={styles.map}>
            <ZoneMap
              zones={zones}
              selectedZoneId={selectedZoneId}
              onSelectZone={setSelectedZoneId}
              editing={draft}
              onBoundaryChange={(boundary) => setDraft((d) => d && { ...d, boundary })}
              checkedPoint={checkedPoint}
              onMapClick={checkPoint}
            />
          </div>
        </div>
      </div>
    </div>
  );
}

interface ZoneCardProps {
  zone: DeliveryZone;
  selected: boolean;
  canManage: boolean;
  onSelect: () => void;
  onEdit: () => void;
  onToggleActive: () => void;
  onDelete: () => void;
}

function ZoneCard({
  zone,
  selected,
  canManage,
  onSelect,
  onEdit,
  onToggleActive,
  onDelete,
}: ZoneCardProps) {
  return (
    <Card
      interactive
      onClick={onSelect}
      style={selected ? { borderColor: "var(--color-primary)", boxShadow: "var(--shadow-md)" } : undefined}
    >
      <CardBody>
        <div style={{ display: "flex", justifyContent: "space-between", gap: "0.5rem", alignItems: "flex-start" }}>
          <div style={{ minWidth: 0 }}>
            <div
              style={{
                fontWeight: 600,
                color: "var(--color-text-primary)",
                overflowWrap: "anywhere",
              }}
            >
              {zone.name}
            </div>
            <div style={{ fontSize: "0.8125rem", color: "var(--color-text-secondary)", marginTop: "0.25rem" }}>
              {describeBoundary(zone.boundary)}
            </div>
          </div>
          <Badge tone={zone.active ? "success" : "neutral"}>
            {zone.active ? "Activa" : "Inactiva"}
          </Badge>
        </div>
        {canManage && (
          <div
            style={{ display: "flex", gap: "0.5rem", marginTop: "0.75rem", flexWrap: "wrap" }}
            onClick={(e) => e.stopPropagation()}
          >
            <Button size="sm" variant="secondary" onClick={onEdit}>
              Editar
            </Button>
            <Button size="sm" variant="ghost" onClick={onToggleActive}>
              {zone.active ? "Desactivar" : "Activar"}
            </Button>
            <Button size="sm" variant="danger" onClick={onDelete}>
              Eliminar
            </Button>
          </div>
        )}
      </CardBody>
    </Card>
  );
}
