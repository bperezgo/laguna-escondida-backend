import type { Position, ZoneBoundary } from "@/types/deliveryZone";

export type BoundaryParseResult =
  | { ok: true; boundary: ZoneBoundary }
  | { ok: false; error: string };

type Json = Record<string, unknown>;

function isObject(value: unknown): value is Json {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function isPosition(value: unknown): value is Position {
  return (
    Array.isArray(value) &&
    value.length >= 2 &&
    typeof value[0] === "number" &&
    typeof value[1] === "number" &&
    Number.isFinite(value[0]) &&
    Number.isFinite(value[1])
  );
}

function readPolygon(value: unknown): Position[][] | null {
  if (!Array.isArray(value) || value.length === 0) return null;
  const rings: Position[][] = [];
  for (const ring of value) {
    if (!Array.isArray(ring) || !ring.every(isPosition)) return null;
    // Drop altitude so the backend gets plain [lng, lat] pairs.
    rings.push(ring.map((p: Position) => [p[0], p[1]]));
  }
  return rings;
}

/** Polygons of a geometry, or null when it is not a Polygon/MultiPolygon. */
function polygonsOfGeometry(geometry: unknown): Position[][][] | null {
  if (!isObject(geometry)) return null;
  if (geometry.type === "Polygon") {
    const polygon = readPolygon(geometry.coordinates);
    return polygon ? [polygon] : null;
  }
  if (geometry.type === "MultiPolygon") {
    if (!Array.isArray(geometry.coordinates)) return null;
    const polygons: Position[][][] = [];
    for (const raw of geometry.coordinates) {
      const polygon = readPolygon(raw);
      if (!polygon) return null;
      polygons.push(polygon);
    }
    return polygons;
  }
  return null;
}

export function polygonsOf(boundary: ZoneBoundary): Position[][][] {
  return boundary.type === "Polygon"
    ? [boundary.coordinates]
    : boundary.coordinates;
}

export function combinePolygons(polygons: Position[][][]): ZoneBoundary | null {
  if (polygons.length === 0) return null;
  if (polygons.length === 1) {
    return { type: "Polygon", coordinates: polygons[0] };
  }
  return { type: "MultiPolygon", coordinates: polygons };
}

/** Mirrors the backend rules so the user sees the problem before saving. */
export function validateBoundary(boundary: ZoneBoundary): string | null {
  const polygons = polygonsOf(boundary);
  if (polygons.length === 0) return "La zona no tiene ningún polígono";
  for (const polygon of polygons) {
    if (polygon.length === 0) return "Hay un polígono sin anillos";
    for (const ring of polygon) {
      if (ring.length < 4) {
        return "Cada anillo necesita al menos 4 posiciones";
      }
      const first = ring[0];
      const last = ring[ring.length - 1];
      if (first[0] !== last[0] || first[1] !== last[1]) {
        return "Cada anillo debe estar cerrado: la última posición debe ser igual a la primera";
      }
      for (const [lng, lat] of ring) {
        if (lng < -180 || lng > 180 || lat < -90 || lat > 90) {
          return `Coordenada fuera de rango: [${lng}, ${lat}]. El orden es [longitud, latitud]`;
        }
      }
    }
  }
  return null;
}

/**
 * Accepts what people usually copy out of geojson.io or a KML converter: a
 * bare geometry, a Feature, or a FeatureCollection. Several polygons are
 * merged into one MultiPolygon.
 */
export function parseBoundary(value: unknown): BoundaryParseResult {
  let geometries: unknown[];
  if (isObject(value) && value.type === "FeatureCollection") {
    if (!Array.isArray(value.features)) {
      return { ok: false, error: "FeatureCollection sin \"features\"" };
    }
    geometries = value.features.map((f) => (isObject(f) ? f.geometry : null));
  } else if (isObject(value) && value.type === "Feature") {
    geometries = [value.geometry];
  } else {
    geometries = [value];
  }

  const polygons: Position[][][] = [];
  for (const geometry of geometries) {
    const found = polygonsOfGeometry(geometry);
    if (!found) {
      return {
        ok: false,
        error:
          "Solo se admiten geometrías Polygon o MultiPolygon (las líneas y puntos no delimitan un área)",
      };
    }
    polygons.push(...found);
  }

  const boundary = combinePolygons(polygons);
  if (!boundary) return { ok: false, error: "No se encontró ningún polígono" };

  const invalid = validateBoundary(boundary);
  if (invalid) return { ok: false, error: invalid };
  return { ok: true, boundary };
}

export function parseBoundaryText(text: string): BoundaryParseResult {
  let value: unknown;
  try {
    value = JSON.parse(text);
  } catch {
    return { ok: false, error: "El texto no es JSON válido" };
  }
  return parseBoundary(value);
}

export function formatBoundary(boundary: ZoneBoundary): string {
  return JSON.stringify(boundary, null, 2);
}

export function describeBoundary(boundary: ZoneBoundary): string {
  const polygons = polygonsOf(boundary);
  const holes = polygons.reduce((sum, p) => sum + Math.max(0, p.length - 1), 0);
  const areas = polygons.length === 1 ? "1 área" : `${polygons.length} áreas`;
  if (holes === 0) return areas;
  return `${areas} · ${holes === 1 ? "1 exclusión" : `${holes} exclusiones`}`;
}

const GEOJSON_IO_BASE = "https://geojson.io/#data=data:application/json,";

/** Opens the zones in geojson.io; the data travels in the URL fragment, never to a server. */
export function geojsonIoUrl(
  zones: { name: string; boundary: ZoneBoundary; active?: boolean }[],
): string {
  const collection = {
    type: "FeatureCollection",
    features: zones.map((zone) => ({
      type: "Feature",
      properties: {
        name: zone.name,
        ...(zone.active === undefined ? {} : { active: zone.active }),
        // simplestyle-spec, honoured by geojson.io.
        fill: zone.active === false ? "#94a3b8" : "#14b8a6",
        stroke: zone.active === false ? "#64748b" : "#0d9488",
      },
      geometry: zone.boundary,
    })),
  };
  return GEOJSON_IO_BASE + encodeURIComponent(JSON.stringify(collection));
}
