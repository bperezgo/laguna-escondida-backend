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
  /** Flat fee per delivery with ICO included, as a decimal string; "0" is free delivery. */
  delivery_fee: string;
  delivery_fee_base: string;
  delivery_fee_ico: string;
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
  delivery_fee?: string;
}

export interface UpdateDeliveryZoneRequest {
  name?: string;
  boundary?: ZoneBoundary;
  active?: boolean;
  delivery_fee?: string;
}

export interface DeliveryPointCheck {
  deliverable: boolean;
  zone_id?: string;
  zone_name?: string;
  /** Set when deliverable: what an order at this point would be charged. */
  delivery_fee?: string;
}
