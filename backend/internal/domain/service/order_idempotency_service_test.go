package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	orderError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports/mocks"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type fakeOrderCreator struct {
	mock.Mock
}

func (f *fakeOrderCreator) CreateOrder(ctx context.Context, req *dto.CreateOrderRequest, user dto.UserDomain) (*dto.OpenBill, error) {
	args := f.Called(ctx, req, user)
	order, _ := args.Get(0).(*dto.OpenBill)
	return order, args.Error(1)
}

const idemKey = "4f1c2a9e-key"

func idemFixture(t *testing.T) (*OrderIdempotencyService, *fakeOrderCreator, *mocks.MockIdempotencyKeyRepository) {
	t.Helper()
	orders := &fakeOrderCreator{}
	keys := mocks.NewMockIdempotencyKeyRepository(t)
	t.Cleanup(func() { orders.AssertExpectations(t) })
	return NewOrderIdempotencyService(orders, keys), orders, keys
}

func idemRequest() *dto.CreateOrderRequest {
	return &dto.CreateOrderRequest{
		OpenBillID:         "019a0000-0000-7000-8000-000000000001",
		TemporalIdentifier: "019a0000-0000-7000-8000-000000000002",
		Products:           []dto.OrderProductItem{{ProductID: "019a0000-0000-7000-8000-000000000003", Quantity: 2}},
	}
}

func idemOrder() *dto.OpenBill {
	return &dto.OpenBill{ID: "019a0000-0000-7000-8000-000000000001", TotalAmount: decimal.RequireFromString("24000"), Status: "pending", CreatedByID: "user-1"}
}

func storedKey(t *testing.T, userID string, req *dto.CreateOrderRequest, response *dto.OpenBill) *dto.IdempotencyKey {
	t.Helper()
	hash, err := requestHash(req)
	require.NoError(t, err)
	key := &dto.IdempotencyKey{Scope: orderCreateIdempotencyScope, Key: idemKey, UserID: userID, RequestHash: hash}
	if response != nil {
		key.Response, err = json.Marshal(response)
		require.NoError(t, err)
	}
	return key
}

var idemUser = dto.UserDomain{ID: "user-1"}

func TestOrderIdempotencyService_CreateOrder_FirstRequestCreatesAndStoresResponse(t *testing.T) {
	svc, orders, keys := idemFixture(t)
	req, order := idemRequest(), idemOrder()

	keys.EXPECT().Claim(mock.Anything, mock.MatchedBy(func(k *dto.IdempotencyKey) bool {
		return k.Scope == orderCreateIdempotencyScope && k.Key == idemKey && k.UserID == "user-1" && k.RequestHash != ""
	})).Return(true, nil)
	orders.On("CreateOrder", mock.Anything, req, idemUser).Return(order, nil)
	keys.EXPECT().Complete(mock.Anything, orderCreateIdempotencyScope, idemKey, mock.MatchedBy(func(body []byte) bool {
		var stored dto.OpenBill
		return json.Unmarshal(body, &stored) == nil && stored.ID == order.ID
	})).Return(nil)

	got, replayed, err := svc.CreateOrder(context.Background(), idemKey, req, idemUser)

	require.NoError(t, err)
	assert.False(t, replayed)
	assert.Same(t, order, got)
}

func TestOrderIdempotencyService_CreateOrder_RepeatReturnsStoredOrderWithoutCreating(t *testing.T) {
	svc, _, keys := idemFixture(t)
	req, order := idemRequest(), idemOrder()

	keys.EXPECT().Claim(mock.Anything, mock.Anything).Return(false, nil)
	keys.EXPECT().Get(mock.Anything, orderCreateIdempotencyScope, idemKey).Return(storedKey(t, "user-1", req, order), nil)

	got, replayed, err := svc.CreateOrder(context.Background(), idemKey, req, idemUser)

	require.NoError(t, err)
	assert.True(t, replayed)
	assert.Equal(t, order.ID, got.ID)
	assert.True(t, order.TotalAmount.Equal(got.TotalAmount))
}

func TestOrderIdempotencyService_CreateOrder_ReusedKeyIsRejected(t *testing.T) {
	otherBody := idemRequest()
	otherBody.Products[0].Quantity = 5

	tests := []struct {
		name   string
		stored *dto.IdempotencyKey
	}{
		{"different request body", nil},
		{"different user", nil},
	}
	tests[0].stored = storedKey(t, "user-1", otherBody, idemOrder())
	tests[1].stored = storedKey(t, "user-2", idemRequest(), idemOrder())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, _, keys := idemFixture(t)
			keys.EXPECT().Claim(mock.Anything, mock.Anything).Return(false, nil)
			keys.EXPECT().Get(mock.Anything, orderCreateIdempotencyScope, idemKey).Return(tt.stored, nil)

			_, _, err := svc.CreateOrder(context.Background(), idemKey, idemRequest(), idemUser)

			assert.ErrorIs(t, err, orderError.ErrIdempotencyKeyReused)
		})
	}
}

func TestOrderIdempotencyService_CreateOrder_InFlightRequestIsRejected(t *testing.T) {
	svc, _, keys := idemFixture(t)
	req := idemRequest()

	keys.EXPECT().Claim(mock.Anything, mock.Anything).Return(false, nil)
	keys.EXPECT().Get(mock.Anything, orderCreateIdempotencyScope, idemKey).Return(storedKey(t, "user-1", req, nil), nil)
	keys.EXPECT().ReleaseStale(mock.Anything, orderCreateIdempotencyScope, idemKey, staleIdempotencyClaim).Return(false, nil)

	_, _, err := svc.CreateOrder(context.Background(), idemKey, req, idemUser)

	assert.ErrorIs(t, err, orderError.ErrIdempotencyRequestInProgress)
}

func TestOrderIdempotencyService_CreateOrder_StaleClaimIsTakenOver(t *testing.T) {
	svc, orders, keys := idemFixture(t)
	req, order := idemRequest(), idemOrder()

	keys.EXPECT().Claim(mock.Anything, mock.Anything).Return(false, nil).Once()
	keys.EXPECT().Get(mock.Anything, orderCreateIdempotencyScope, idemKey).Return(storedKey(t, "user-1", req, nil), nil)
	keys.EXPECT().ReleaseStale(mock.Anything, orderCreateIdempotencyScope, idemKey, staleIdempotencyClaim).Return(true, nil)
	keys.EXPECT().Claim(mock.Anything, mock.Anything).Return(true, nil).Once()
	orders.On("CreateOrder", mock.Anything, req, idemUser).Return(order, nil)
	keys.EXPECT().Complete(mock.Anything, orderCreateIdempotencyScope, idemKey, mock.Anything).Return(nil)

	got, replayed, err := svc.CreateOrder(context.Background(), idemKey, req, idemUser)

	require.NoError(t, err)
	assert.False(t, replayed)
	assert.Same(t, order, got)
}

func TestOrderIdempotencyService_CreateOrder_LostTakeoverRaceIsInProgress(t *testing.T) {
	svc, _, keys := idemFixture(t)
	req := idemRequest()

	keys.EXPECT().Claim(mock.Anything, mock.Anything).Return(false, nil).Twice()
	keys.EXPECT().Get(mock.Anything, orderCreateIdempotencyScope, idemKey).Return(nil, nil)

	_, _, err := svc.CreateOrder(context.Background(), idemKey, req, idemUser)

	assert.ErrorIs(t, err, orderError.ErrIdempotencyRequestInProgress)
}

func TestOrderIdempotencyService_CreateOrder_FailedCreateReleasesKey(t *testing.T) {
	svc, orders, keys := idemFixture(t)
	req := idemRequest()

	keys.EXPECT().Claim(mock.Anything, mock.Anything).Return(true, nil)
	orders.On("CreateOrder", mock.Anything, req, idemUser).Return(nil, orderError.ErrProductNotFound)
	keys.EXPECT().Release(mock.Anything, orderCreateIdempotencyScope, idemKey).Return(nil)

	_, _, err := svc.CreateOrder(context.Background(), idemKey, req, idemUser)

	assert.ErrorIs(t, err, orderError.ErrProductNotFound)
}

func TestOrderIdempotencyService_CreateOrder_FailedReleaseKeepsCreateError(t *testing.T) {
	svc, orders, keys := idemFixture(t)
	req := idemRequest()
	releaseErr := errors.New("db down")

	keys.EXPECT().Claim(mock.Anything, mock.Anything).Return(true, nil)
	orders.On("CreateOrder", mock.Anything, req, idemUser).Return(nil, orderError.ErrProductNotFound)
	keys.EXPECT().Release(mock.Anything, orderCreateIdempotencyScope, idemKey).Return(releaseErr)

	_, _, err := svc.CreateOrder(context.Background(), idemKey, req, idemUser)

	assert.ErrorIs(t, err, orderError.ErrProductNotFound)
	assert.ErrorIs(t, err, releaseErr)
}

func TestOrderIdempotencyService_CreateOrder_FailedCompleteStillReturnsOrder(t *testing.T) {
	svc, orders, keys := idemFixture(t)
	req, order := idemRequest(), idemOrder()

	keys.EXPECT().Claim(mock.Anything, mock.Anything).Return(true, nil)
	orders.On("CreateOrder", mock.Anything, req, idemUser).Return(order, nil)
	keys.EXPECT().Complete(mock.Anything, orderCreateIdempotencyScope, idemKey, mock.Anything).Return(errors.New("db down"))

	got, _, err := svc.CreateOrder(context.Background(), idemKey, req, idemUser)

	require.NoError(t, err)
	assert.Same(t, order, got)
}

func TestOrderIdempotencyService_CreateOrder_InvalidKey(t *testing.T) {
	for name, key := range map[string]string{"empty": "", "too long": strings.Repeat("k", maxIdempotencyKeyLength+1)} {
		t.Run(name, func(t *testing.T) {
			svc, _, _ := idemFixture(t)

			_, _, err := svc.CreateOrder(context.Background(), key, idemRequest(), idemUser)

			assert.ErrorIs(t, err, orderError.ErrIdempotencyKeyInvalid)
		})
	}
}

func TestOrderIdempotencyService_CreateOrder_RepositoryErrors(t *testing.T) {
	dbErr := errors.New("db down")

	t.Run("claim", func(t *testing.T) {
		svc, _, keys := idemFixture(t)
		keys.EXPECT().Claim(mock.Anything, mock.Anything).Return(false, dbErr)

		_, _, err := svc.CreateOrder(context.Background(), idemKey, idemRequest(), idemUser)
		assert.ErrorIs(t, err, dbErr)
	})

	t.Run("get", func(t *testing.T) {
		svc, _, keys := idemFixture(t)
		keys.EXPECT().Claim(mock.Anything, mock.Anything).Return(false, nil)
		keys.EXPECT().Get(mock.Anything, orderCreateIdempotencyScope, idemKey).Return(nil, dbErr)

		_, _, err := svc.CreateOrder(context.Background(), idemKey, idemRequest(), idemUser)
		assert.ErrorIs(t, err, dbErr)
	})

	t.Run("release stale", func(t *testing.T) {
		svc, _, keys := idemFixture(t)
		req := idemRequest()
		keys.EXPECT().Claim(mock.Anything, mock.Anything).Return(false, nil)
		keys.EXPECT().Get(mock.Anything, orderCreateIdempotencyScope, idemKey).Return(storedKey(t, "user-1", req, nil), nil)
		keys.EXPECT().ReleaseStale(mock.Anything, orderCreateIdempotencyScope, idemKey, staleIdempotencyClaim).Return(false, dbErr)

		_, _, err := svc.CreateOrder(context.Background(), idemKey, req, idemUser)
		assert.ErrorIs(t, err, dbErr)
	})

	t.Run("stored response is corrupt", func(t *testing.T) {
		svc, _, keys := idemFixture(t)
		req := idemRequest()
		stored := storedKey(t, "user-1", req, nil)
		stored.Response = json.RawMessage(`{"id":`)
		keys.EXPECT().Claim(mock.Anything, mock.Anything).Return(false, nil)
		keys.EXPECT().Get(mock.Anything, orderCreateIdempotencyScope, idemKey).Return(stored, nil)

		_, _, err := svc.CreateOrder(context.Background(), idemKey, req, idemUser)
		assert.ErrorContains(t, err, "decode stored order")
	})
}
