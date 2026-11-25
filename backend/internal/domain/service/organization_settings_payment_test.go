package service

import (
	"context"
	"errors"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestOrganizationSettingsService_Get_CarriesThePaymentStrategy(t *testing.T) {
	repo := mocks.NewMockOrganizationSettingsRepository(t)
	repo.EXPECT().Get(mock.Anything).Return(&dto.OrganizationSettingsView{Timezone: bogotaTZ}, nil).Once()

	res, err := NewOrganizationSettingsService(repo).
		WithPaymentStrategy(paymentSettingsOf(t, dto.PaymentStrategyManual, 60)).Get(context.Background())

	require.NoError(t, err)
	require.NotNil(t, res.PaymentStrategy)
	assert.Equal(t, dto.PaymentStrategyManual, *res.PaymentStrategy)
}

func TestOrganizationSettingsService_Get_WithoutPaymentSettingsHasNoStrategy(t *testing.T) {
	repo := mocks.NewMockOrganizationSettingsRepository(t)
	repo.EXPECT().Get(mock.Anything).Return(&dto.OrganizationSettingsView{Timezone: bogotaTZ}, nil).Once()

	res, err := NewOrganizationSettingsService(repo).Get(context.Background())

	require.NoError(t, err)
	assert.Nil(t, res.PaymentStrategy, "the box has none")
}

func TestOrganizationSettingsService_Get_PaymentSettingsFailure(t *testing.T) {
	repo := mocks.NewMockOrganizationSettingsRepository(t)
	repo.EXPECT().Get(mock.Anything).Return(&dto.OrganizationSettingsView{Timezone: bogotaTZ}, nil).Once()
	payments := mocks.NewMockPaymentSettingsRepository(t)
	payments.EXPECT().Get(mock.Anything).Return(nil, false, errors.New("db down"))

	_, err := NewOrganizationSettingsService(repo).WithPaymentStrategy(NewPaymentSettingsService(payments)).Get(context.Background())

	require.ErrorContains(t, err, "db down")
}
