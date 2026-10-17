package service

import (
	"context"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	orderError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/permissions"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// The aggregates built by heldAggregate are created by userID1.
func agentUser(id string) dto.UserDomain {
	return dto.UserDomain{ID: id, RoleIDs: []int{permissions.RoleAgent}}
}

func TestOrderService_CreateOrder_AgentTakesOnlyTakeawayAndDelivery(t *testing.T) {
	tests := []struct {
		name    string
		contact dto.OrderContactRequest
		zones   bool
		wantErr error
	}{
		{name: "takeaway", contact: dto.OrderContactRequest{OrderType: dto.OrderTypeTakeaway, CustomerPhone: strPtr("3005551234")}},
		{name: "delivery", zones: true, contact: dto.OrderContactRequest{
			OrderType: dto.OrderTypeDelivery, CustomerPhone: strPtr("3005551234"), DeliveryAddress: strPtr("Cra 7 # 12-34"),
			DeliveryLat: strPtr(insideCentroLat), DeliveryLng: strPtr(insideCentroLng),
		}},
		{name: "explicit dine-in", contact: dto.OrderContactRequest{OrderType: dto.OrderTypeDineIn}, wantErr: orderError.ErrAgentOrderTypeNotAllowed},
		{name: "type omitted (defaults to dine-in)", contact: dto.OrderContactRequest{}, wantErr: orderError.ErrAgentOrderTypeNotAllowed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newDeliveryOrderFixture(t)
			if tt.zones {
				f.zones.EXPECT().ListActive(mock.Anything).Return([]*dto.DeliveryZone{centroZone()}, nil).Once()
			}

			_, err := f.service.CreateOrder(createTestContext(), contactRequest(tt.contact), agentUser(userID1))

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, f.created, "nothing is persisted")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.contact.OrderType, f.created.OrderType())
		})
	}
}

func TestOrderService_CreateOrder_StaffRoleAlongsideAgentIsNotRestricted(t *testing.T) {
	f := newDeliveryOrderFixture(t)
	user := dto.UserDomain{ID: userID1, RoleIDs: []int{permissions.RoleAgent, permissions.RoleServer}}

	result, err := f.service.CreateOrder(createTestContext(), contactRequest(dto.OrderContactRequest{}), user)

	require.NoError(t, err)
	assert.Equal(t, dto.OrderTypeDineIn, result.OrderType)
}

func TestOrderService_CancelLine_AgentScope(t *testing.T) {
	tests := []struct {
		name      string
		orderType dto.OrderType
		caller    string
		wantErr   error
	}{
		{name: "own takeaway", orderType: dto.OrderTypeTakeaway, caller: userID1},
		{name: "own delivery", orderType: dto.OrderTypeDelivery, caller: userID1},
		{name: "another user's takeaway", orderType: dto.OrderTypeTakeaway, caller: userID2, wantErr: orderError.ErrOrderNotOwnedByAgent},
		{name: "dine-in it somehow created", orderType: dto.OrderTypeDineIn, caller: userID1, wantErr: orderError.ErrOrderNotOwnedByAgent},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			aggregate := heldAggregate(t, tt.orderType, nil, dto.CommandStatusCreated)
			cancel := func(s *OrderService, ctx context.Context, openBillID, productID string) error {
				return s.CancelOpenBillProduct(ctx, openBillID, productID, agentUser(tt.caller))
			}

			if tt.wantErr == nil {
				runKitchenTransition(t, aggregate, cancel)
				assert.Equal(t, dto.CommandStatusCancelled, aggregate.GetProduct(obpID1).Status())
				return
			}
			ctx := createTestContext()
			service, openBills, _ := newStatusSyncService(t)
			openBills.On("FindAggregateForKitchen", ctx, openBillID1).Return(aggregate, nil)

			err := cancel(service, ctx, openBillID1, obpID1)

			require.ErrorIs(t, err, tt.wantErr)
			assert.Equal(t, dto.CommandStatusCreated, aggregate.GetProduct(obpID1).Status(), "the line is unchanged")
			openBills.AssertNotCalled(t, "UpdateProductStatus", mock.Anything, mock.Anything)
		})
	}
}

func TestOrderService_CancelLine_StaffMayCancelAnyOrder(t *testing.T) {
	aggregate := heldAggregate(t, dto.OrderTypeDineIn, nil, dto.CommandStatusCreated)
	staff := dto.UserDomain{ID: userID2, RoleIDs: []int{permissions.RoleServer}}

	runKitchenTransition(t, aggregate, func(s *OrderService, ctx context.Context, openBillID, productID string) error {
		return s.CancelOpenBillProduct(ctx, openBillID, productID, staff)
	})

	assert.Equal(t, dto.CommandStatusCancelled, aggregate.GetProduct(obpID1).Status())
}

func TestOrderService_UpdateOrder_AgentRefusedOnOrdersItDidNotCreate(t *testing.T) {
	for _, tt := range []struct {
		name      string
		orderType dto.OrderType
		caller    string
	}{
		{name: "another user's delivery", orderType: dto.OrderTypeDelivery, caller: userID2},
		{name: "dine-in", orderType: dto.OrderTypeDineIn, caller: userID1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := createTestContext()
			service, openBills, _ := newStatusSyncService(t)
			openBills.On("FindByIDWithProducts", ctx, openBillID1).Return(&dto.OpenBillWithProducts{ID: openBillID1}, nil)
			openBills.On("FindAggregateByID", ctx, openBillID1).Return(heldAggregate(t, tt.orderType, nil, dto.CommandStatusCreated), nil)

			_, err := service.UpdateOrder(ctx, openBillID1, &dto.UpdateOrderRequest{Products: []dto.OrderProductItem{}}, agentUser(tt.caller))

			require.ErrorIs(t, err, orderError.ErrOrderNotOwnedByAgent)
			openBills.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
		})
	}
}

func TestOrderService_DeleteOrder_AgentScope(t *testing.T) {
	order := func(orderType dto.OrderType, createdBy string) *dto.OpenBillWithProducts {
		return &dto.OpenBillWithProducts{
			ID:               openBillID1,
			TotalAmount:      decimal.NewFromFloat(100.0),
			CreatedBy:        dto.OpenBillCreator{ID: createdBy},
			CreatedAt:        time.Now(),
			OrderFulfillment: dto.OrderFulfillment{OrderType: orderType},
		}
	}

	t.Run("own takeaway is deleted", func(t *testing.T) {
		ctx := createTestContext()
		service, openBills, outbox := newStatusSyncService(t)
		openBills.On("FindByIDWithProducts", ctx, openBillID1).Return(order(dto.OrderTypeTakeaway, userID1), nil)
		openBills.On("Delete", ctx, openBillID1).Return(nil)
		outbox.EXPECT().Append(mock.Anything, mock.Anything).Return(nil).Once()

		require.NoError(t, service.DeleteOrder(ctx, openBillID1, agentUser(userID1)))
	})

	for _, tt := range []struct {
		name  string
		order *dto.OpenBillWithProducts
	}{
		{name: "another user's takeaway", order: order(dto.OrderTypeTakeaway, userID2)},
		{name: "a dine-in table", order: order(dto.OrderTypeDineIn, userID2)},
		{name: "a dine-in replicated without a type", order: order("", userID1)},
	} {
		t.Run(tt.name+" is refused", func(t *testing.T) {
			ctx := createTestContext()
			service, openBills, _ := newStatusSyncService(t)
			openBills.On("FindByIDWithProducts", ctx, openBillID1).Return(tt.order, nil)

			err := service.DeleteOrder(ctx, openBillID1, agentUser(userID1))

			require.ErrorIs(t, err, orderError.ErrOrderNotOwnedByAgent)
			openBills.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything)
		})
	}
}
