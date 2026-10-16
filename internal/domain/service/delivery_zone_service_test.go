package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	zoneError "laguna-escondida/backend/internal/domain/aggregate/delivery_zone/error"
	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	baseError "laguna-escondida/backend/internal/platform/shared/domain/error"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Centro is a square around Bogotá's center, [lng, lat], with a hole around (4.60, -74.08).
const (
	centroPolygon = `{"type":"Polygon","coordinates":[
		[[-74.10,4.58],[-74.06,4.58],[-74.06,4.62],[-74.10,4.62],[-74.10,4.58]],
		[[-74.085,4.595],[-74.075,4.595],[-74.075,4.605],[-74.085,4.605],[-74.085,4.595]]]}`
	norteMultiPolygon = `{"type":"MultiPolygon","coordinates":[
		[[[-74.06,4.68],[-74.02,4.68],[-74.02,4.72],[-74.06,4.72],[-74.06,4.68]]],
		[[[-74.20,4.50],[-74.16,4.50],[-74.16,4.54],[-74.20,4.54],[-74.20,4.50]]]]}`
	// Overlaps Centro's east half.
	overlapPolygon = `{"type":"Polygon","coordinates":[[[-74.08,4.58],[-74.04,4.58],[-74.04,4.62],[-74.08,4.62],[-74.08,4.58]]]}`
)

func zoneDTO(id, name, boundary string, active bool, createdAt time.Time) *dto.DeliveryZone {
	return &dto.DeliveryZone{ID: id, Name: name, Boundary: json.RawMessage(boundary), Active: active, CreatedAt: createdAt, UpdatedAt: createdAt}
}

func requireBaseErrorCode(t *testing.T, err error, code zoneError.DeliveryZoneErrorCode) {
	t.Helper()
	var baseErr *baseError.BaseError
	require.True(t, errors.As(err, &baseErr), "expected a validation error, got %v", err)
	assert.Equal(t, baseError.ErrorCode(code), baseErr.GetCode())
}

func TestDeliveryZoneService_Create_ValidPolygon(t *testing.T) {
	repo := mocks.NewMockDeliveryZoneRepository(t)
	repo.EXPECT().Create(mock.Anything, mock.MatchedBy(func(z *dto.DeliveryZone) bool {
		return z.Name == "Centro" && z.Active && z.ID != ""
	})).Return(nil).Once()

	zone, err := NewDeliveryZoneService(repo).Create(context.Background(), &dto.CreateDeliveryZoneRequest{
		Name: " Centro ", Boundary: json.RawMessage(centroPolygon),
	})

	require.NoError(t, err)
	assert.Equal(t, "Centro", zone.Name)
	assert.True(t, zone.Active)
	assert.JSONEq(t, centroPolygon, string(zone.Boundary), "the boundary is returned unchanged")
}

func TestDeliveryZoneService_Create_MultiPolygon(t *testing.T) {
	repo := mocks.NewMockDeliveryZoneRepository(t)
	repo.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Once()

	_, err := NewDeliveryZoneService(repo).Create(context.Background(), &dto.CreateDeliveryZoneRequest{
		Name: "Norte", Boundary: json.RawMessage(norteMultiPolygon),
	})

	require.NoError(t, err)
}

func TestDeliveryZoneService_Create_InvalidBoundary(t *testing.T) {
	tests := []struct {
		name     string
		boundary string
	}{
		{"open ring", `{"type":"Polygon","coordinates":[[[-74.10,4.58],[-74.06,4.58],[-74.06,4.62],[-74.10,4.62]]]}`},
		{"too few positions", `{"type":"Polygon","coordinates":[[[-74.10,4.58],[-74.06,4.58],[-74.10,4.58]]]}`},
		{"latitude 95", `{"type":"Polygon","coordinates":[[[-74.10,4.58],[-74.08,95],[-74.06,4.62],[-74.10,4.58]]]}`},
		{"unsupported type", `{"type":"Point","coordinates":[-74.08,4.60]}`},
		{"empty", ``},
		{"not json", `polygon`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := mocks.NewMockDeliveryZoneRepository(t)

			_, err := NewDeliveryZoneService(repo).Create(context.Background(), &dto.CreateDeliveryZoneRequest{
				Name: "Centro", Boundary: json.RawMessage(tt.boundary),
			})

			requireBaseErrorCode(t, err, zoneError.CodeInvalidBoundary)
			repo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
		})
	}
}

func TestDeliveryZoneService_Create_MissingName(t *testing.T) {
	repo := mocks.NewMockDeliveryZoneRepository(t)

	_, err := NewDeliveryZoneService(repo).Create(context.Background(), &dto.CreateDeliveryZoneRequest{
		Name: "  ", Boundary: json.RawMessage(centroPolygon),
	})

	requireBaseErrorCode(t, err, zoneError.CodeInvalidName)
}

func TestDeliveryZoneService_Check(t *testing.T) {
	base := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	centro := zoneDTO("zone-centro", "Centro", centroPolygon, true, base)
	norte := zoneDTO("zone-norte", "Norte", norteMultiPolygon, true, base.Add(time.Hour))
	inactive := zoneDTO("zone-off", "Apagada", overlapPolygon, false, base.Add(-time.Hour))
	newerOverlap := zoneDTO("zone-newer", "Nueva", overlapPolygon, true, base.Add(2*time.Hour))
	olderOverlap := zoneDTO("zone-older", "Vieja", overlapPolygon, true, base.Add(-2*time.Hour))

	tests := []struct {
		name     string
		zones    []*dto.DeliveryZone
		lat, lng float64
		wantZone string
	}{
		{"point inside", []*dto.DeliveryZone{centro}, 4.590, -74.095, "Centro"},
		{"point in a hole", []*dto.DeliveryZone{centro}, 4.600, -74.080, ""},
		{"point outside", []*dto.DeliveryZone{centro}, 4.700, -74.200, ""},
		{"inactive zone ignored", []*dto.DeliveryZone{inactive}, 4.600, -74.050, ""},
		{"overlapping zones pick the oldest", []*dto.DeliveryZone{centro, newerOverlap}, 4.590, -74.070, "Centro"},
		{"oldest wins whatever the list order", []*dto.DeliveryZone{centro, olderOverlap}, 4.590, -74.070, "Vieja"},
		{"MultiPolygon second part", []*dto.DeliveryZone{centro, norte}, 4.520, -74.180, "Norte"},
		{"no zones", nil, 4.590, -74.095, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := mocks.NewMockDeliveryZoneRepository(t)
			repo.EXPECT().ListActive(mock.Anything).Return(tt.zones, nil).Once()

			result, err := NewDeliveryZoneService(repo).Check(context.Background(), tt.lat, tt.lng)

			require.NoError(t, err)
			if tt.wantZone == "" {
				assert.False(t, result.Deliverable)
				assert.Nil(t, result.ZoneName)
				return
			}
			assert.True(t, result.Deliverable)
			require.NotNil(t, result.ZoneName)
			assert.Equal(t, tt.wantZone, *result.ZoneName)
		})
	}
}

func TestDeliveryZoneService_Check_InvalidPoint(t *testing.T) {
	repo := mocks.NewMockDeliveryZoneRepository(t)

	_, err := NewDeliveryZoneService(repo).Check(context.Background(), 95, -74.08)

	requireBaseErrorCode(t, err, zoneError.CodeInvalidPoint)
}

func TestDeliveryZoneService_Update_Redraw(t *testing.T) {
	existing := zoneDTO("zone-centro", "Centro", centroPolygon, true, time.Now().Add(-time.Hour))
	repo := mocks.NewMockDeliveryZoneRepository(t)
	repo.EXPECT().FindByID(mock.Anything, "zone-centro").Return(existing, nil).Once()
	repo.EXPECT().Update(mock.Anything, mock.MatchedBy(func(z *dto.DeliveryZone) bool {
		return z.ID == "zone-centro" && string(z.Boundary) == overlapPolygon && !z.Active
	})).Return(nil).Once()

	inactive := false
	zone, err := NewDeliveryZoneService(repo).Update(context.Background(), "zone-centro", &dto.UpdateDeliveryZoneRequest{
		Boundary: json.RawMessage(overlapPolygon), Active: &inactive,
	})

	require.NoError(t, err)
	assert.Equal(t, "Centro", zone.Name, "an omitted field keeps its value")
}

func TestDeliveryZoneService_Update_InvalidBoundaryNotSaved(t *testing.T) {
	existing := zoneDTO("zone-centro", "Centro", centroPolygon, true, time.Now())
	repo := mocks.NewMockDeliveryZoneRepository(t)
	repo.EXPECT().FindByID(mock.Anything, "zone-centro").Return(existing, nil).Once()

	_, err := NewDeliveryZoneService(repo).Update(context.Background(), "zone-centro", &dto.UpdateDeliveryZoneRequest{
		Boundary: json.RawMessage(`{"type":"Point","coordinates":[0,0]}`),
	})

	requireBaseErrorCode(t, err, zoneError.CodeInvalidBoundary)
}

func TestDeliveryZoneService_Update_NotFound(t *testing.T) {
	repo := mocks.NewMockDeliveryZoneRepository(t)
	repo.EXPECT().FindByID(mock.Anything, "missing").Return(nil, domainError.ErrDeliveryZoneNotFound).Once()

	_, err := NewDeliveryZoneService(repo).Update(context.Background(), "missing", &dto.UpdateDeliveryZoneRequest{})

	assert.ErrorIs(t, err, domainError.ErrDeliveryZoneNotFound)
}

func TestDeliveryZoneService_Delete(t *testing.T) {
	repo := mocks.NewMockDeliveryZoneRepository(t)
	repo.EXPECT().FindByID(mock.Anything, "zone-centro").Return(zoneDTO("zone-centro", "Centro", centroPolygon, true, time.Now()), nil).Once()
	repo.EXPECT().Delete(mock.Anything, "zone-centro").Return(nil).Once()

	require.NoError(t, NewDeliveryZoneService(repo).Delete(context.Background(), "zone-centro"))
}

func TestDeliveryZoneService_Delete_NotFound(t *testing.T) {
	repo := mocks.NewMockDeliveryZoneRepository(t)
	repo.EXPECT().FindByID(mock.Anything, "missing").Return(nil, domainError.ErrDeliveryZoneNotFound).Once()

	err := NewDeliveryZoneService(repo).Delete(context.Background(), "missing")

	assert.ErrorIs(t, err, domainError.ErrDeliveryZoneNotFound)
}

func TestDeliveryZoneService_List_RepositoryError(t *testing.T) {
	repo := mocks.NewMockDeliveryZoneRepository(t)
	repo.EXPECT().FindAll(mock.Anything).Return(nil, errors.New("db down")).Once()

	_, err := NewDeliveryZoneService(repo).List(context.Background())

	assert.ErrorIs(t, err, domainError.ErrDeliveryZoneOperation)
}
