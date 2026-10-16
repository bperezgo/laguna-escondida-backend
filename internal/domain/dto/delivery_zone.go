package dto

import (
	"encoding/json"
	"time"
)

// DeliveryZone.Boundary is a GeoJSON Polygon or MultiPolygon geometry, positions as [lng, lat].
type DeliveryZone struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Boundary  json.RawMessage `json:"boundary"`
	Active    bool            `json:"active"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

type CreateDeliveryZoneRequest struct {
	Name     string          `json:"name"`
	Boundary json.RawMessage `json:"boundary"`
	Active   *bool           `json:"active,omitempty"`
}

type UpdateDeliveryZoneRequest struct {
	Name     *string         `json:"name,omitempty"`
	Boundary json.RawMessage `json:"boundary,omitempty"`
	Active   *bool           `json:"active,omitempty"`
}

type DeliveryZoneCheckResult struct {
	Deliverable bool    `json:"deliverable"`
	ZoneID      *string `json:"zone_id,omitempty"`
	ZoneName    *string `json:"zone_name,omitempty"`
}
