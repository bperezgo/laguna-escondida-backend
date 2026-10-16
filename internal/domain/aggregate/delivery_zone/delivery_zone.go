package delivery_zone

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	zoneError "laguna-escondida/backend/internal/domain/aggregate/delivery_zone/error"
	"laguna-escondida/backend/internal/domain/dto"

	"github.com/google/uuid"
)

const (
	geoJSONPolygon      = "Polygon"
	geoJSONMultiPolygon = "MultiPolygon"
	maxNameLength       = 100
	minRingPositions    = 4
)

type ring [][]float64

// polygon is an outer ring followed by its holes.
type polygon []ring

type Aggregate struct {
	id        string
	name      string
	boundary  json.RawMessage
	polygons  []polygon
	active    bool
	createdAt time.Time
	updatedAt time.Time
}

func NewAggregateFromCreateRequest(req *dto.CreateDeliveryZoneRequest) (*Aggregate, error) {
	name, err := validateName(req.Name)
	if err != nil {
		return nil, err
	}
	polygons, err := parseBoundary(req.Boundary)
	if err != nil {
		return nil, err
	}
	active := true
	if req.Active != nil {
		active = *req.Active
	}
	now := time.Now()
	return &Aggregate{
		id:        uuid.Must(uuid.NewV7()).String(),
		name:      name,
		boundary:  req.Boundary,
		polygons:  polygons,
		active:    active,
		createdAt: now,
		updatedAt: now,
	}, nil
}

// NewAggregateFromDTO re-validates a stored zone, so a row edited by hand can never contain points.
func NewAggregateFromDTO(zone *dto.DeliveryZone) (*Aggregate, error) {
	polygons, err := parseBoundary(zone.Boundary)
	if err != nil {
		return nil, err
	}
	return &Aggregate{
		id:        zone.ID,
		name:      zone.Name,
		boundary:  zone.Boundary,
		polygons:  polygons,
		active:    zone.Active,
		createdAt: zone.CreatedAt,
		updatedAt: zone.UpdatedAt,
	}, nil
}

func (a *Aggregate) Update(req *dto.UpdateDeliveryZoneRequest) error {
	if req.Name != nil {
		name, err := validateName(*req.Name)
		if err != nil {
			return err
		}
		a.name = name
	}
	if len(req.Boundary) > 0 {
		polygons, err := parseBoundary(req.Boundary)
		if err != nil {
			return err
		}
		a.boundary, a.polygons = req.Boundary, polygons
	}
	if req.Active != nil {
		a.active = *req.Active
	}
	a.updatedAt = time.Now()
	return nil
}

func (a *Aggregate) ID() string {
	return a.id
}

func (a *Aggregate) Name() string {
	return a.name
}

func (a *Aggregate) IsActive() bool {
	return a.active
}

// Contains: inside an outer ring and in none of that polygon's holes. Inactive zones contain nothing.
func (a *Aggregate) Contains(lat, lng float64) bool {
	if !a.active {
		return false
	}
	for _, p := range a.polygons {
		if !p[0].contains(lng, lat) {
			continue
		}
		inHole := false
		for _, hole := range p[1:] {
			if hole.contains(lng, lat) {
				inHole = true
				break
			}
		}
		if !inHole {
			return true
		}
	}
	return false
}

func (a *Aggregate) ToDTO() *dto.DeliveryZone {
	return &dto.DeliveryZone{
		ID:        a.id,
		Name:      a.name,
		Boundary:  a.boundary,
		Active:    a.active,
		CreatedAt: a.createdAt,
		UpdatedAt: a.updatedAt,
	}
}

// ValidatePoint checks a lat/lng pair is on Earth before it is tested against zones.
func ValidatePoint(lat, lng float64) error {
	if lat < -90 || lat > 90 || lng < -180 || lng > 180 {
		return zoneError.NewInvalidPointError(fmt.Sprintf("%v,%v", lat, lng))
	}
	return nil
}

// contains is ray casting over (x=lng, y=lat).
func (r ring) contains(x, y float64) bool {
	inside := false
	for i, j := 0, len(r)-1; i < len(r); j, i = i, i+1 {
		xi, yi := r[i][0], r[i][1]
		xj, yj := r[j][0], r[j][1]
		if (yi > y) != (yj > y) && x < (xj-xi)*(y-yi)/(yj-yi)+xi {
			inside = !inside
		}
	}
	return inside
}

func validateName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" || utf8.RuneCountInString(name) > maxNameLength {
		return "", zoneError.NewInvalidNameError(raw)
	}
	return name, nil
}

func parseBoundary(raw json.RawMessage) ([]polygon, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, zoneError.NewInvalidBoundaryError("boundary is required")
	}
	var geometry struct {
		Type        string          `json:"type"`
		Coordinates json.RawMessage `json:"coordinates"`
	}
	if err := json.Unmarshal(raw, &geometry); err != nil {
		return nil, zoneError.NewInvalidBoundaryError("not a JSON geometry object")
	}

	var polygons []polygon
	switch geometry.Type {
	case geoJSONPolygon:
		var p polygon
		if err := json.Unmarshal(geometry.Coordinates, &p); err != nil {
			return nil, zoneError.NewInvalidBoundaryError("Polygon coordinates must be an array of rings")
		}
		polygons = []polygon{p}
	case geoJSONMultiPolygon:
		if err := json.Unmarshal(geometry.Coordinates, &polygons); err != nil {
			return nil, zoneError.NewInvalidBoundaryError("MultiPolygon coordinates must be an array of polygons")
		}
	default:
		return nil, zoneError.NewInvalidBoundaryError(fmt.Sprintf("unsupported geometry type %q", geometry.Type))
	}

	if len(polygons) == 0 {
		return nil, zoneError.NewInvalidBoundaryError("no polygon")
	}
	for _, p := range polygons {
		if len(p) == 0 {
			return nil, zoneError.NewInvalidBoundaryError("a polygon has no ring")
		}
		for _, r := range p {
			if err := validateRing(r); err != nil {
				return nil, err
			}
		}
	}
	return polygons, nil
}

func validateRing(r ring) error {
	if len(r) < minRingPositions {
		return zoneError.NewInvalidBoundaryError(fmt.Sprintf("a ring needs at least %d positions", minRingPositions))
	}
	for _, position := range r {
		if len(position) < 2 {
			return zoneError.NewInvalidBoundaryError("a position needs a longitude and a latitude")
		}
		lng, lat := position[0], position[1]
		if lng < -180 || lng > 180 || lat < -90 || lat > 90 {
			return zoneError.NewInvalidBoundaryError(fmt.Sprintf("position [%v, %v] is out of range", lng, lat))
		}
	}
	first, last := r[0], r[len(r)-1]
	if first[0] != last[0] || first[1] != last[1] {
		return zoneError.NewInvalidBoundaryError("a ring's first and last positions must be equal")
	}
	return nil
}
