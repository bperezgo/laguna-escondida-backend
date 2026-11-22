package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	zoneError "laguna-escondida/backend/internal/domain/aggregate/delivery_zone/error"
	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports/mocks"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func feeOf(fee, base, ico string) dto.DeliveryFeeAmounts {
	return feeAt(fee, base, ico, "0.08")
}

func feeAt(fee, base, ico, rate string) dto.DeliveryFeeAmounts {
	return dto.DeliveryFeeAmounts{
		DeliveryFee:        decimal.RequireFromString(fee),
		DeliveryFeeBase:    decimal.RequireFromString(base),
		DeliveryFeeICO:     decimal.RequireFromString(ico),
		DeliveryFeeICORate: decimal.RequireFromString(rate),
	}
}

func assertFee(t *testing.T, want, got dto.DeliveryFeeAmounts) {
	t.Helper()
	assert.True(t, want.DeliveryFee.Equal(got.DeliveryFee), "fee: want %s, got %s", want.DeliveryFee, got.DeliveryFee)
	assert.True(t, want.DeliveryFeeBase.Equal(got.DeliveryFeeBase), "base: want %s, got %s", want.DeliveryFeeBase, got.DeliveryFeeBase)
	assert.True(t, want.DeliveryFeeICO.Equal(got.DeliveryFeeICO), "ico: want %s, got %s", want.DeliveryFeeICO, got.DeliveryFeeICO)
	assert.True(t, want.DeliveryFeeICORate.Equal(got.DeliveryFeeICORate), "rate: want %s, got %s", want.DeliveryFeeICORate, got.DeliveryFeeICORate)
}

func TestDeliveryZoneService_Create_DerivesFeeBaseAndICO(t *testing.T) {
	tests := []struct {
		fee, base, ico string
	}{
		{"5000", "4629.63", "370.37"},
		{"8000", "7407.41", "592.59"},
		{"0", "0", "0"},
		{"1234.56", "1143.11", "91.45"},
	}
	for _, tc := range tests {
		t.Run(tc.fee, func(t *testing.T) {
			repo := mocks.NewMockDeliveryZoneRepository(t)
			var stored *dto.DeliveryZone
			repo.EXPECT().Create(mock.Anything, mock.Anything).
				Run(func(_ context.Context, z *dto.DeliveryZone) { stored = z }).Return(nil).Once()

			fee := tc.fee
			zone, err := NewDeliveryZoneService(repo).Create(context.Background(), &dto.CreateDeliveryZoneRequest{
				Name: "Centro", Boundary: json.RawMessage(centroPolygon), DeliveryFee: &fee,
			})

			require.NoError(t, err)
			assertFee(t, feeOf(tc.fee, tc.base, tc.ico), zone.DeliveryFeeAmounts)
			assertFee(t, feeOf(tc.fee, tc.base, tc.ico), stored.DeliveryFeeAmounts)
			assert.True(t, zone.DeliveryFeeBase.Add(zone.DeliveryFeeICO).Equal(zone.DeliveryFee), "base + ico = fee")
		})
	}
}

func TestDeliveryZoneService_Create_NoFeeIsFree(t *testing.T) {
	repo := mocks.NewMockDeliveryZoneRepository(t)
	repo.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Once()

	zone, err := NewDeliveryZoneService(repo).Create(context.Background(), &dto.CreateDeliveryZoneRequest{
		Name: "Centro", Boundary: json.RawMessage(centroPolygon),
	})

	require.NoError(t, err)
	assertFee(t, feeOf("0", "0", "0"), zone.DeliveryFeeAmounts)
}

func TestDeliveryZoneService_Create_InvalidFee(t *testing.T) {
	for _, fee := range []string{"-1000", "5000.123", "abc", ""} {
		t.Run(fee, func(t *testing.T) {
			repo := mocks.NewMockDeliveryZoneRepository(t)
			value := fee

			_, err := NewDeliveryZoneService(repo).Create(context.Background(), &dto.CreateDeliveryZoneRequest{
				Name: "Centro", Boundary: json.RawMessage(centroPolygon), DeliveryFee: &value,
			})

			requireBaseErrorCode(t, err, zoneError.CodeInvalidFee)
		})
	}
}

func TestDeliveryZoneService_Update_ChangesFee(t *testing.T) {
	created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	existing := zoneDTO("z1", "Centro", centroPolygon, true, created)
	existing.DeliveryFeeAmounts = feeOf("5000", "4629.63", "370.37")
	repo := mocks.NewMockDeliveryZoneRepository(t)
	repo.EXPECT().FindByID(mock.Anything, "z1").Return(existing, nil).Once()
	repo.EXPECT().Update(mock.Anything, mock.Anything).Return(nil).Once()

	fee := "6000"
	zone, err := NewDeliveryZoneService(repo).Update(context.Background(), "z1", &dto.UpdateDeliveryZoneRequest{DeliveryFee: &fee})

	require.NoError(t, err)
	assertFee(t, feeOf("6000", "5555.56", "444.44"), zone.DeliveryFeeAmounts)
}

func TestDeliveryZoneService_Update_WithoutFeeKeepsIt(t *testing.T) {
	created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	existing := zoneDTO("z1", "Centro", centroPolygon, true, created)
	existing.DeliveryFeeAmounts = feeOf("5000", "4629.63", "370.37")
	repo := mocks.NewMockDeliveryZoneRepository(t)
	repo.EXPECT().FindByID(mock.Anything, "z1").Return(existing, nil).Once()
	repo.EXPECT().Update(mock.Anything, mock.Anything).Return(nil).Once()

	name := "Centro histórico"
	zone, err := NewDeliveryZoneService(repo).Update(context.Background(), "z1", &dto.UpdateDeliveryZoneRequest{Name: &name})

	require.NoError(t, err)
	assertFee(t, feeOf("5000", "4629.63", "370.37"), zone.DeliveryFeeAmounts)
}

func TestDeliveryZoneService_Update_NegativeFeeChangesNothing(t *testing.T) {
	created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	repo := mocks.NewMockDeliveryZoneRepository(t)
	repo.EXPECT().FindByID(mock.Anything, "z1").Return(zoneDTO("z1", "Centro", centroPolygon, true, created), nil).Once()

	fee := "-1000"
	_, err := NewDeliveryZoneService(repo).Update(context.Background(), "z1", &dto.UpdateDeliveryZoneRequest{DeliveryFee: &fee})

	requireBaseErrorCode(t, err, zoneError.CodeInvalidFee)
}

func TestDeliveryZoneService_Check_ReturnsZoneFee(t *testing.T) {
	created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	paid := zoneDTO("z1", "Centro", centroPolygon, true, created)
	paid.DeliveryFeeAmounts = feeOf("5000", "4629.63", "370.37")
	free := zoneDTO("z2", "Norte", norteMultiPolygon, true, created.Add(time.Hour))

	tests := []struct {
		name        string
		lat, lng    float64
		deliverable bool
		fee         *string
	}{
		{"paid zone", 4.61, -74.09, true, strPtr("5000")},
		{"free zone", 4.70, -74.04, true, strPtr("0")},
		{"outside every zone", 4.90, -74.30, false, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := mocks.NewMockDeliveryZoneRepository(t)
			repo.EXPECT().ListActive(mock.Anything).Return([]*dto.DeliveryZone{paid, free}, nil).Once()

			result, err := NewDeliveryZoneService(repo).Check(context.Background(), tc.lat, tc.lng)

			require.NoError(t, err)
			assert.Equal(t, tc.deliverable, result.Deliverable)
			if tc.fee == nil {
				assert.Nil(t, result.DeliveryFee)
				return
			}
			require.NotNil(t, result.DeliveryFee)
			assert.True(t, decimal.RequireFromString(*tc.fee).Equal(*result.DeliveryFee))
		})
	}
}

func TestDeliveryZoneService_Create_UsesItsOwnICORate(t *testing.T) {
	repo := mocks.NewMockDeliveryZoneRepository(t)
	var stored *dto.DeliveryZone
	repo.EXPECT().Create(mock.Anything, mock.Anything).
		Run(func(_ context.Context, z *dto.DeliveryZone) { stored = z }).Return(nil).Once()

	fee, rate := "5000", "0.19"
	zone, err := NewDeliveryZoneService(repo).Create(context.Background(), &dto.CreateDeliveryZoneRequest{
		Name: "Norte", Boundary: json.RawMessage(centroPolygon), DeliveryFee: &fee, DeliveryFeeICORate: &rate,
	})

	require.NoError(t, err)
	assertFee(t, feeAt("5000", "4201.68", "798.32", "0.19"), zone.DeliveryFeeAmounts)
	assertFee(t, feeAt("5000", "4201.68", "798.32", "0.19"), stored.DeliveryFeeAmounts)
}

func TestDeliveryZoneService_Create_InvalidICORate(t *testing.T) {
	for _, rate := range []string{"-0.01", "1", "8", "0.08125", "abc"} {
		t.Run(rate, func(t *testing.T) {
			repo := mocks.NewMockDeliveryZoneRepository(t)
			fee, value := "5000", rate

			_, err := NewDeliveryZoneService(repo).Create(context.Background(), &dto.CreateDeliveryZoneRequest{
				Name: "Centro", Boundary: json.RawMessage(centroPolygon), DeliveryFee: &fee, DeliveryFeeICORate: &value,
			})

			requireBaseErrorCode(t, err, zoneError.CodeInvalidICORate)
		})
	}
}

func TestDeliveryZoneService_Update_InvalidICORateChangesNothing(t *testing.T) {
	for _, rate := range []string{"-0.01", "1", "8"} {
		t.Run(rate, func(t *testing.T) {
			created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
			existing := zoneDTO("z1", "Centro", centroPolygon, true, created)
			existing.DeliveryFeeAmounts = feeOf("5000", "4629.63", "370.37")
			repo := mocks.NewMockDeliveryZoneRepository(t)
			repo.EXPECT().FindByID(mock.Anything, "z1").Return(existing, nil).Once()

			value := rate
			_, err := NewDeliveryZoneService(repo).Update(context.Background(), "z1", &dto.UpdateDeliveryZoneRequest{DeliveryFeeICORate: &value})

			requireBaseErrorCode(t, err, zoneError.CodeInvalidICORate)
		})
	}
}

func TestDeliveryZoneService_Update_RateOnlyRecomputesBaseAndICO(t *testing.T) {
	created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	existing := zoneDTO("z1", "Centro", centroPolygon, true, created)
	existing.DeliveryFeeAmounts = feeOf("5000", "4629.63", "370.37")
	repo := mocks.NewMockDeliveryZoneRepository(t)
	repo.EXPECT().FindByID(mock.Anything, "z1").Return(existing, nil).Once()
	var stored *dto.DeliveryZone
	repo.EXPECT().Update(mock.Anything, mock.Anything).
		Run(func(_ context.Context, z *dto.DeliveryZone) { stored = z }).Return(nil).Once()

	rate := "0.19"
	zone, err := NewDeliveryZoneService(repo).Update(context.Background(), "z1", &dto.UpdateDeliveryZoneRequest{DeliveryFeeICORate: &rate})

	require.NoError(t, err)
	assertFee(t, feeAt("5000", "4201.68", "798.32", "0.19"), zone.DeliveryFeeAmounts)
	assertFee(t, feeAt("5000", "4201.68", "798.32", "0.19"), stored.DeliveryFeeAmounts)
}

func TestDeliveryZoneService_Update_FeeOnlyKeepsTheRate(t *testing.T) {
	created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	existing := zoneDTO("z1", "Norte", centroPolygon, true, created)
	existing.DeliveryFeeAmounts = feeAt("5000", "4201.68", "798.32", "0.19")
	repo := mocks.NewMockDeliveryZoneRepository(t)
	repo.EXPECT().FindByID(mock.Anything, "z1").Return(existing, nil).Once()
	repo.EXPECT().Update(mock.Anything, mock.Anything).Return(nil).Once()

	fee := "6000"
	zone, err := NewDeliveryZoneService(repo).Update(context.Background(), "z1", &dto.UpdateDeliveryZoneRequest{DeliveryFee: &fee})

	require.NoError(t, err)
	assertFee(t, feeAt("6000", "5042.02", "957.98", "0.19"), zone.DeliveryFeeAmounts)
}
