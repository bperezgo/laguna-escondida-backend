"use client";

import { useState } from "react";
import type { ZoneBoundary } from "@/types/deliveryZone";
import { Button, Card, CardBody, Input, Textarea } from "@/components/ui";
import {
  describeBoundary,
  formatBoundary,
  geojsonIoUrl,
  parseBoundaryText,
} from "@/lib/deliveryZones/geojson";

interface ZoneEditorProps {
  isNew: boolean;
  name: string;
  active: boolean;
  boundary: ZoneBoundary | null;
  onNameChange: (name: string) => void;
  onActiveChange: (active: boolean) => void;
  /** A boundary that came from outside the map (pasted GeoJSON). */
  onBoundaryImport: (boundary: ZoneBoundary) => void;
  onSave: () => void;
  onCancel: () => void;
  isSaving: boolean;
}

export default function ZoneEditor({
  isNew,
  name,
  active,
  boundary,
  onNameChange,
  onActiveChange,
  onBoundaryImport,
  onSave,
  onCancel,
  isSaving,
}: ZoneEditorProps) {
  // null = mirror the map; a string = the user's unapplied edit.
  const [typedText, setTypedText] = useState<string | null>(null);
  const textDirty = typedText !== null;
  const geojsonText = typedText ?? (boundary ? formatBoundary(boundary) : "");
  const [importError, setImportError] = useState("");
  const [nameError, setNameError] = useState("");

  const handleImport = () => {
    const result = parseBoundaryText(geojsonText);
    if (!result.ok) {
      setImportError(result.error);
      return;
    }
    setImportError("");
    setTypedText(null);
    onBoundaryImport(result.boundary);
  };

  const handleSave = () => {
    const trimmed = name.trim();
    if (!trimmed) {
      setNameError("El nombre es requerido");
      return;
    }
    if (trimmed.length > 100) {
      setNameError("El nombre debe tener 100 caracteres o menos");
      return;
    }
    setNameError("");
    onSave();
  };

  return (
    <Card>
      <CardBody>
        <div style={{ display: "flex", flexDirection: "column", gap: "1rem" }}>
          <h2
            style={{
              margin: 0,
              fontSize: "1.125rem",
              fontWeight: 600,
              color: "var(--color-text-primary)",
            }}
          >
            {isNew ? "Nueva zona" : "Editar zona"}
          </h2>

          <Input
            label="Nombre *"
            value={name}
            onChange={(e) => {
              onNameChange(e.target.value);
              if (nameError) setNameError("");
            }}
            error={nameError}
            placeholder="Ej: Centro"
            maxLength={100}
          />

          <label
            style={{
              display: "flex",
              alignItems: "center",
              gap: "0.5rem",
              fontSize: "0.875rem",
              color: "var(--color-text-primary)",
              cursor: "pointer",
            }}
          >
            <input
              type="checkbox"
              checked={active}
              onChange={(e) => onActiveChange(e.target.checked)}
            />
            Activa (acepta domicilios)
          </label>

          <div
            style={{
              padding: "0.75rem",
              borderRadius: "var(--radius-sm)",
              backgroundColor: "var(--color-secondary-50)",
              fontSize: "0.8125rem",
              color: "var(--color-text-secondary)",
              lineHeight: 1.5,
            }}
          >
            <strong style={{ color: "var(--color-text-primary)" }}>
              Dibuja en el mapa
            </strong>{" "}
            con la barra de la izquierda: polígono o rectángulo para el área,
            el modo <em>editar</em> para mover vértices y <em>cortar</em> para
            excluir un sector dentro de la zona.
            <div style={{ marginTop: "0.5rem", color: "var(--color-text-primary)" }}>
              {boundary ? describeBoundary(boundary) : "Sin área dibujada"}
            </div>
          </div>

          <details>
            <summary
              style={{
                cursor: "pointer",
                fontSize: "0.875rem",
                color: "var(--color-text-secondary)",
              }}
            >
              GeoJSON (pegar desde geojson.io)
            </summary>
            <div
              style={{
                display: "flex",
                flexDirection: "column",
                gap: "0.5rem",
                marginTop: "0.75rem",
              }}
            >
              <Textarea
                value={geojsonText}
                onChange={(e) => {
                  setTypedText(e.target.value);
                  if (importError) setImportError("");
                }}
                error={importError}
                rows={8}
                spellCheck={false}
                placeholder='{"type": "Polygon", "coordinates": [[[lng, lat], ...]]}'
                style={{ fontFamily: "monospace", fontSize: "0.75rem" }}
              />
              <p
                style={{
                  margin: 0,
                  fontSize: "0.75rem",
                  color: "var(--color-text-muted)",
                }}
              >
                Acepta una geometría, un Feature o un FeatureCollection.
                Posiciones en orden [longitud, latitud].
              </p>
              <div style={{ display: "flex", gap: "0.5rem", flexWrap: "wrap" }}>
                <Button
                  variant="secondary"
                  size="sm"
                  onClick={handleImport}
                  disabled={!textDirty}
                >
                  Aplicar al mapa
                </Button>
                {boundary && (
                  <a
                    href={geojsonIoUrl([{ name: name || "Zona", boundary, active }])}
                    target="_blank"
                    rel="noopener noreferrer"
                    style={{
                      fontSize: "0.875rem",
                      color: "var(--color-primary)",
                      alignSelf: "center",
                    }}
                  >
                    Abrir en geojson.io ↗
                  </a>
                )}
              </div>
            </div>
          </details>

          <div style={{ display: "flex", gap: "0.5rem", justifyContent: "flex-end" }}>
            <Button variant="secondary" onClick={onCancel} disabled={isSaving}>
              Cancelar
            </Button>
            <Button onClick={handleSave} disabled={isSaving || !boundary}>
              {isSaving ? "Guardando..." : isNew ? "Crear zona" : "Guardar"}
            </Button>
          </div>
        </div>
      </CardBody>
    </Card>
  );
}
