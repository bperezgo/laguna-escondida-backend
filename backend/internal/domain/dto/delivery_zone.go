package dto

import (
	"encoding/json"
	"time"

	"github.com/shopspring/decimal"
)

// DeliveryZone.Boundary is a GeoJSON Polygon or MultiPolygon geometry, positions as [lng, lat].
type DeliveryZone struct {
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	Boundary json.RawMessage `json:"boundary"`
	Active   bool            `json:"active"`
	DeliveryFeeAmounts
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// DeliveryFeeAmounts is a delivery fee with ICO included, and the base and ICO derived from it.
// Base + ICO always equals the fee.
type DeliveryFeeAmounts struct {
	DeliveryFee     decimal.Decimal `json:"delivery_fee"`
	DeliveryFeeBase decimal.Decimal `json:"delivery_fee_base"`
	DeliveryFeeICO  decimal.Decimal `json:"delivery_fee_ico"`
}

// DeliveryFee is a string, like the other numeric request fields; omitted means 0 on create
// and unchanged on update.
type CreateDeliveryZoneRequest struct {
	Name        string          `json:"name"`
	Boundary    json.RawMessage `json:"boundary"`
	Active      *bool           `json:"active,omitempty"`
	DeliveryFee *string         `json:"delivery_fee,omitempty"`
}

type UpdateDeliveryZoneRequest struct {
	Name        *string         `json:"name,omitempty"`
	Boundary    json.RawMessage `json:"boundary,omitempty"`
	Active      *bool           `json:"active,omitempty"`
	DeliveryFee *string         `json:"delivery_fee,omitempty"`
}

type DeliveryZoneCheckResult struct {
	Deliverable bool             `json:"deliverable"`
	ZoneID      *string          `json:"zone_id,omitempty"`
	ZoneName    *string          `json:"zone_name,omitempty"`
	DeliveryFee *decimal.Decimal `json:"delivery_fee,omitempty"`
}
