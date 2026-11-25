package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestPaymentSettingsService_Get_DefaultsToWompi(t *testing.T) {
	repo := mocks.NewMockPaymentSettingsRepository(t)
	repo.EXPECT().Get(mock.Anything).Return(nil, false, nil)

	got, err := NewPaymentSettingsService(repo).Get(context.Background())

	require.NoError(t, err)
	assert.Equal(t, &dto.PaymentSettings{Strategy: dto.PaymentStrategyWompi, ManualWindowMinutes: 60}, got)
}

func TestPaymentSettingsService_Get_Stored(t *testing.T) {
	stored := &dto.PaymentSettings{Strategy: dto.PaymentStrategyManual, ManualInstructions: "Llave 300", ManualWindowMinutes: 30}
	repo := mocks.NewMockPaymentSettingsRepository(t)
	repo.EXPECT().Get(mock.Anything).Return(stored, true, nil)

	got, err := NewPaymentSettingsService(repo).Get(context.Background())

	require.NoError(t, err)
	assert.Equal(t, stored, got)
}

func TestPaymentSettingsService_Get_ReadFailure(t *testing.T) {
	repo := mocks.NewMockPaymentSettingsRepository(t)
	repo.EXPECT().Get(mock.Anything).Return(nil, false, errors.New("db down"))

	_, err := NewPaymentSettingsService(repo).Get(context.Background())

	require.ErrorContains(t, err, "db down")
}

func TestPaymentSettingsService_Save_Manual(t *testing.T) {
	repo := mocks.NewMockPaymentSettingsRepository(t)
	want := &dto.PaymentSettings{Strategy: dto.PaymentStrategyManual, ManualInstructions: "Llave Bre-B 3001234567", ManualWindowMinutes: 45}
	repo.EXPECT().Upsert(mock.Anything, want).Return(nil)

	got, err := NewPaymentSettingsService(repo).Save(context.Background(), dto.PaymentSettingsRequest{
		Strategy: "manual", ManualInstructions: "  Llave Bre-B 3001234567 ", ManualWindowMinutes: intPtr(45),
	})

	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestPaymentSettingsService_Save_DefaultWindow(t *testing.T) {
	repo := mocks.NewMockPaymentSettingsRepository(t)
	repo.EXPECT().Upsert(mock.Anything, mock.Anything).Return(nil)

	got, err := NewPaymentSettingsService(repo).Save(context.Background(), dto.PaymentSettingsRequest{Strategy: "manual", ManualInstructions: "x"})

	require.NoError(t, err)
	assert.Equal(t, 60, got.ManualWindowMinutes)
}

func TestPaymentSettingsService_Save_WompiKeepsInstructions(t *testing.T) {
	repo := mocks.NewMockPaymentSettingsRepository(t)
	want := &dto.PaymentSettings{Strategy: dto.PaymentStrategyWompi, ManualInstructions: "Llave 300", ManualWindowMinutes: 90}
	repo.EXPECT().Upsert(mock.Anything, want).Return(nil)

	_, err := NewPaymentSettingsService(repo).Save(context.Background(), dto.PaymentSettingsRequest{
		Strategy: "wompi", ManualInstructions: "Llave 300", ManualWindowMinutes: intPtr(90),
	})

	require.NoError(t, err)
}

func TestPaymentSettingsService_Save_Refused(t *testing.T) {
	tests := []struct {
		name  string
		req   dto.PaymentSettingsRequest
		field string
	}{
		{"unknown strategy", dto.PaymentSettingsRequest{Strategy: "cash"}, "strategy"},
		{"no strategy", dto.PaymentSettingsRequest{}, "strategy"},
		{"manual without instructions", dto.PaymentSettingsRequest{Strategy: "manual", ManualInstructions: "  "}, "manual_instructions"},
		{"instructions too long", dto.PaymentSettingsRequest{Strategy: "wompi", ManualInstructions: strings.Repeat("a", 1001)}, "manual_instructions"},
		{"window too short", dto.PaymentSettingsRequest{Strategy: "manual", ManualInstructions: "x", ManualWindowMinutes: intPtr(9)}, "manual_window_minutes"},
		{"window too long", dto.PaymentSettingsRequest{Strategy: "manual", ManualInstructions: "x", ManualWindowMinutes: intPtr(1441)}, "manual_window_minutes"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := mocks.NewMockPaymentSettingsRepository(t)

			_, err := NewPaymentSettingsService(repo).Save(context.Background(), tt.req)

			assert.Equal(t, tt.field, validationField(t, err))
		})
	}
}

func TestPaymentSettingsService_Save_Boundaries(t *testing.T) {
	for _, window := range []int{10, 1440} {
		repo := mocks.NewMockPaymentSettingsRepository(t)
		repo.EXPECT().Upsert(mock.Anything, mock.Anything).Return(nil)

		_, err := NewPaymentSettingsService(repo).Save(context.Background(), dto.PaymentSettingsRequest{
			Strategy: "manual", ManualInstructions: strings.Repeat("a", 1000), ManualWindowMinutes: intPtr(window),
		})

		require.NoError(t, err)
	}
}

func TestPaymentSettingsService_Save_StoreFailure(t *testing.T) {
	repo := mocks.NewMockPaymentSettingsRepository(t)
	repo.EXPECT().Upsert(mock.Anything, mock.Anything).Return(errors.New("db down"))

	_, err := NewPaymentSettingsService(repo).Save(context.Background(), dto.PaymentSettingsRequest{Strategy: "wompi"})

	require.ErrorContains(t, err, "db down")
}
