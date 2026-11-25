package service

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	orderError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type fakeManualVoider struct {
	voided []string
	errs   map[string]error
}

func (v *fakeManualVoider) ExpireManualOrder(_ context.Context, orderID string) error {
	if err := v.errs[orderID]; err != nil {
		return err
	}
	v.voided = append(v.voided, orderID)
	return nil
}

func newManualExpiry(t *testing.T) (*PaymentExpiryService, *expiryFixture, *mocks.MockOpenBillRepository, *fakeManualVoider) {
	t.Helper()
	f := newExpiryFixture(t)
	f.links.links.EXPECT().ListDue(mock.Anything, f.links.now).Return(nil, nil)
	orders := mocks.NewMockOpenBillRepository(t)
	voider := &fakeManualVoider{errs: map[string]error{}}
	service := NewPaymentExpiryService(nil, f.links.links, f.transactions, f.links.service, f.orders,
		slog.New(slog.DiscardHandler), func() time.Time { return f.links.now }, WithManualExpiry(orders, voider))
	return service, f, orders, voider
}

func TestPaymentExpiryService_ManualOrderNotPaidInTime(t *testing.T) {
	service, f, orders, voider := newManualExpiry(t)
	orders.EXPECT().ListManualDue(mock.Anything, f.links.now).Return([]string{"m1", "m2"}, nil).Once()

	require.NoError(t, service.ExpireDueForOrganization(crmContext()))

	assert.Equal(t, []string{"m1", "m2"}, voider.voided)
	assert.Empty(t, f.orders.expired, "no link was involved")
}

func TestPaymentExpiryService_ManualOrderUnderReviewIsHeldThenVoided(t *testing.T) {
	service, f, orders, voider := newManualExpiry(t)
	// An order under review is not listed; once the mark is cleared, the next run lists it.
	orders.EXPECT().ListManualDue(mock.Anything, f.links.now).Return(nil, nil).Once()
	require.NoError(t, service.ExpireDueForOrganization(crmContext()))
	assert.Empty(t, voider.voided)

	orders.EXPECT().ListManualDue(mock.Anything, f.links.now).Return([]string{"m1"}, nil).Once()
	require.NoError(t, service.ExpireDueForOrganization(crmContext()))
	assert.Equal(t, []string{"m1"}, voider.voided)
}

func TestPaymentExpiryService_ManualOrderPaidMeanwhileIsSkipped(t *testing.T) {
	service, f, orders, voider := newManualExpiry(t)
	orders.EXPECT().ListManualDue(mock.Anything, f.links.now).Return([]string{"m1", "m2"}, nil).Once()
	voider.errs["m1"] = orderError.ErrOrderNotFound

	require.NoError(t, service.ExpireDueForOrganization(crmContext()))

	assert.Equal(t, []string{"m2"}, voider.voided, "the next one still runs")
}

func TestPaymentExpiryService_ManualOrderFailuresAreReported(t *testing.T) {
	service, f, orders, voider := newManualExpiry(t)
	orders.EXPECT().ListManualDue(mock.Anything, f.links.now).Return([]string{"m1", "m2"}, nil).Once()
	voider.errs["m1"] = errors.New("db down")

	err := service.ExpireDueForOrganization(crmContext())

	require.ErrorContains(t, err, "manual order m1")
	assert.Equal(t, []string{"m2"}, voider.voided)
}

func TestPaymentExpiryService_ManualListFailure(t *testing.T) {
	service, f, orders, _ := newManualExpiry(t)
	orders.EXPECT().ListManualDue(mock.Anything, f.links.now).Return(nil, errors.New("db down")).Once()

	require.ErrorContains(t, service.ExpireDueForOrganization(crmContext()), "list expired manual orders")
}
