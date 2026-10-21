/** GeoJSON position, `[longitude, latitude]` — the opposite of how maps show it. */
export type Position = [number, number];

export interface PolygonGeometry {
  type: "Polygon";
  /** First ring is the outer boundary; further rings are holes. */
  coordinates: Position[][];
}

export interface MultiPolygonGeometry {
  type: "MultiPolygon";
  coordinates: Position[][][];
}

export type ZoneBoundary = PolygonGeometry | MultiPolygonGeometry;

export interface DeliveryZone {
  id: string;
  name: string;
  boundary: ZoneBoundary;
  active: boolean;
  created_at: string;
  updated_at: string;
}

export interface DeliveryZoneListResponse {
  delivery_zones: DeliveryZone[];
}

export interface CreateDeliveryZoneRequest {
  name: string;
  boundary: ZoneBoundary;
  active?: boolean;
}

export interface UpdateDeliveryZoneRequest {
  name?: string;
  boundary?: ZoneBoundary;
  active?: boolean;
}

export interface DeliveryPointCheck {
  deliverable: boolean;
  zone_id?: string;
  zone_name?: string;
}
