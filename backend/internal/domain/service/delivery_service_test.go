package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	deliveryOrderID = "019a0000-0000-7000-8000-0000000000e1"
	courierJuanID   = "019a0000-0000-7000-8000-0000000000a1"
	courierPedroID  = "019a0000-0000-7000-8000-0000000000a2"
	deliveryMgrID   = "019a0000-0000-7000-8000-0000000000a9"
)

var deliveryNow = time.Date(2026, 11, 20, 17, 30, 0, 0, time.UTC)

type deliveryRig struct {
	repo    *mocks.MockDeliveryRepository
	outbox  *txOutbox
	service *DeliveryService
}

func newDeliveryRig(t *testing.T) *deliveryRig {
	outbox := &txOutbox{}
	repo := mocks.NewMockDeliveryRepository(t)
	return &deliveryRig{
		repo:    repo,
		outbox:  outbox,
		service: NewDeliveryService(repo, outbox, newRecordingEvents(t, outbox), func() time.Time { return deliveryNow }),
	}
}

func agentDeliveryState(orderType dto.OrderType, status dto.DeliveryStatus, courier *string) *dto.DeliveryTrackingState {
	return &dto.DeliveryTrackingState{
		OrderID: deliveryOrderID, OrderType: orderType, Paid: true, Status: status, CourierUserID: courier,
		Fulfillment: dto.OrderFulfillment{
			OrderType: orderType, CustomerPhone: strPtr(crmPhone),
			Origin: &dto.Origin{ChannelID: strPtr(crmChannel), ConversationID: strPtr(crmConvID)},
		},
	}
}

func (r *deliveryRig) expectApplied(t *testing.T, check func(*dto.DeliveryStatusChange)) {
	r.repo.EXPECT().ApplyStatusChange(mock.Anything, mock.Anything).RunAndReturn(func(ctx context.Context, c *dto.DeliveryStatusChange) (bool, error) {
		require.True(t, inUnitOfWork(ctx), "the change is written inside the unit of work")
		check(c)
		return true, nil
	}).Once()
	r.repo.EXPECT().FindOrder(mock.Anything, deliveryOrderID).Return(&dto.DeliveryOrder{ID: deliveryOrderID}, nil).Once()
}

func notificationData(t *testing.T, event *dto.MarketingEvent) map[string]string {
	t.Helper()
	var body struct {
		Type string            `json:"type"`
		Data map[string]string `json:"data"`
	}
	require.NoError(t, json.Unmarshal(event.Body, &body))
	body.Data["type"] = body.Type
	return body.Data
}

func TestDeliveryService_Dispatch_ManagerAssignsACourier(t *testing.T) {
	rig := newDeliveryRig(t)
	rig.repo.EXPECT().FindTrackingState(mock.Anything, deliveryOrderID).Return(agentDeliveryState(dto.OrderTypeDelivery, dto.DeliveryStatusReady, nil), nil)
	rig.repo.EXPECT().FindActiveCourier(mock.Anything, courierJuanID).Return(&dto.CourierUser{ID: courierJuanID, Name: "Juan"}, true, nil)
	rig.expectApplied(t, func(c *dto.DeliveryStatusChange) {
		assert.Nil(t, c.ExpectedStatus, "a ready order has no recorded status")
		assert.Nil(t, c.ExpectedCourierUserID)
		assert.Equal(t, dto.DeliveryStatusOutForDelivery, c.Status)
		assert.Equal(t, courierJuanID, *c.CourierUserID)
		assert.Equal(t, "Juan", *c.CourierName, "the courier's display name is kept")
		assert.Equal(t, deliveryMgrID, c.ActorUserID)
		assert.Equal(t, deliveryNow, c.OccurredAt)
		assert.NotEmpty(t, c.EventID)
	})

	_, err := rig.service.Dispatch(crmContext(), deliveryOrderID, DeliveryActor{UserID: deliveryMgrID, Manage: true},
		dto.DispatchDeliveryRequest{CourierUserID: strPtr(courierJuanID)})
	require.NoError(t, err)

	events := rig.outbox.events()
	require.Len(t, events, 1, "one order.out_for_delivery")
	assert.Equal(t, map[string]string{"type": "order.out_for_delivery", "order_id": deliveryOrderID, "order_type": "delivery", "courier_name": "Juan"},
		notificationData(t, events[0]))
}

func TestDeliveryService_Dispatch_CourierClaimsWithoutNamingThemselves(t *testing.T) {
	rig := newDeliveryRig(t)
	rig.repo.EXPECT().FindTrackingState(mock.Anything, deliveryOrderID).Return(agentDeliveryState(dto.OrderTypeDelivery, dto.DeliveryStatusReady, nil), nil)
	rig.repo.EXPECT().FindActiveCourier(mock.Anything, courierJuanID).Return(&dto.CourierUser{ID: courierJuanID, Name: "Juan"}, true, nil)
	rig.expectApplied(t, func(c *dto.DeliveryStatusChange) {
		assert.Equal(t, courierJuanID, *c.CourierUserID)
	})

	_, err := rig.service.Dispatch(crmContext(), deliveryOrderID, DeliveryActor{UserID: courierJuanID}, dto.DispatchDeliveryRequest{})
	require.NoError(t, err)
	assert.Len(t, rig.outbox.events(), 1)
}

func TestDeliveryService_Dispatch_ThirdPartyAndRedispatch(t *testing.T) {
	rig := newDeliveryRig(t)
	failed := agentDeliveryState(dto.OrderTypeDelivery, dto.DeliveryStatusFailed, strPtr(courierJuanID))
	rig.repo.EXPECT().FindTrackingState(mock.Anything, deliveryOrderID).Return(failed, nil)
	rig.expectApplied(t, func(c *dto.DeliveryStatusChange) {
		require.NotNil(t, c.ExpectedStatus)
		assert.Equal(t, dto.DeliveryStatusFailed, *c.ExpectedStatus)
		assert.Equal(t, courierJuanID, *c.ExpectedCourierUserID, "the update is conditional on what was seen")
		assert.Nil(t, c.CourierUserID)
		assert.Equal(t, "Mensajeros Ya", *c.CourierName)
	})

	_, err := rig.service.Dispatch(crmContext(), deliveryOrderID, DeliveryActor{UserID: deliveryMgrID, Manage: true},
		dto.DispatchDeliveryRequest{CourierName: strPtr("  Mensajeros Ya ")})
	require.NoError(t, err)
	events := rig.outbox.events()
	require.Len(t, events, 1, "a re-dispatch notifies again")
	assert.Equal(t, "Mensajeros Ya", notificationData(t, events[0])["courier_name"])
}

func TestDeliveryService_Dispatch_AWaiterAsCourier(t *testing.T) {
	rig := newDeliveryRig(t)
	rig.repo.EXPECT().FindTrackingState(mock.Anything, deliveryOrderID).Return(agentDeliveryState(dto.OrderTypeDelivery, dto.DeliveryStatusReady, nil), nil)
	rig.repo.EXPECT().FindActiveCourier(mock.Anything, courierPedroID).Return(nil, false, nil)

	_, err := rig.service.Dispatch(crmContext(), deliveryOrderID, DeliveryActor{UserID: deliveryMgrID, Manage: true},
		dto.DispatchDeliveryRequest{CourierUserID: strPtr(courierPedroID)})

	var validation *domainError.ValidationError
	require.True(t, errors.As(err, &validation))
	assert.Equal(t, "courier_user_id", validation.Field)
	assert.Empty(t, rig.outbox.events())
}

func TestDeliveryService_Deliver_Takeaway(t *testing.T) {
	rig := newDeliveryRig(t)
	rig.repo.EXPECT().FindTrackingState(mock.Anything, deliveryOrderID).Return(agentDeliveryState(dto.OrderTypeTakeaway, dto.DeliveryStatusReady, nil), nil)
	rig.expectApplied(t, func(c *dto.DeliveryStatusChange) {
		assert.Equal(t, dto.DeliveryStatusDelivered, c.Status)
		assert.Nil(t, c.CourierName)
	})

	_, err := rig.service.Deliver(crmContext(), deliveryOrderID, DeliveryActor{UserID: deliveryMgrID, Manage: true})
	require.NoError(t, err)
	events := rig.outbox.events()
	require.Len(t, events, 1)
	assert.Equal(t, map[string]string{"type": "order.delivered", "order_id": deliveryOrderID, "order_type": "takeaway"}, notificationData(t, events[0]))
}

func TestDeliveryService_Fail_CourierReportsWithAReasonAndNote(t *testing.T) {
	rig := newDeliveryRig(t)
	rig.repo.EXPECT().FindTrackingState(mock.Anything, deliveryOrderID).
		Return(agentDeliveryState(dto.OrderTypeDelivery, dto.DeliveryStatusOutForDelivery, strPtr(courierJuanID)), nil)
	rig.expectApplied(t, func(c *dto.DeliveryStatusChange) {
		assert.Equal(t, dto.DeliveryStatusFailed, c.Status)
		assert.Equal(t, dto.DeliveryFailureCustomerUnreachable, *c.Reason)
		assert.Equal(t, "portería no deja subir, no contesta", *c.Note)
		assert.Equal(t, courierJuanID, *c.CourierUserID)
	})

	_, err := rig.service.Fail(crmContext(), deliveryOrderID, DeliveryActor{UserID: courierJuanID},
		dto.FailDeliveryRequest{Reason: dto.DeliveryFailureCustomerUnreachable, Note: strPtr("portería no deja subir, no contesta")})
	require.NoError(t, err)
	events := rig.outbox.events()
	require.Len(t, events, 1)
	data := notificationData(t, events[0])
	assert.Equal(t, "customer_unreachable", data["reason"])
	assert.NotContains(t, string(events[0].Body), "portería", "the note is never sent")
}

func TestDeliveryService_Fail_WithoutAReasonRecordsNothing(t *testing.T) {
	rig := newDeliveryRig(t)

	_, err := rig.service.Fail(crmContext(), deliveryOrderID, DeliveryActor{UserID: courierJuanID}, dto.FailDeliveryRequest{})

	var validation *domainError.ValidationError
	require.True(t, errors.As(err, &validation))
	assert.Equal(t, "reason", validation.Field)
}

func TestDeliveryService_RefusedChangesRecordNothing(t *testing.T) {
	tests := []struct {
		name    string
		state   *dto.DeliveryTrackingState
		actor   DeliveryActor
		call    func(s *DeliveryService, actor DeliveryActor) error
		want403 bool
	}{
		{name: "delivered without leaving", state: agentDeliveryState(dto.OrderTypeDelivery, dto.DeliveryStatusReady, nil),
			actor: DeliveryActor{UserID: deliveryMgrID, Manage: true},
			call: func(s *DeliveryService, a DeliveryActor) error {
				_, err := s.Deliver(crmContext(), deliveryOrderID, a)
				return err
			}},
		{name: "another courier's order", state: agentDeliveryState(dto.OrderTypeDelivery, dto.DeliveryStatusOutForDelivery, strPtr(courierJuanID)),
			actor: DeliveryActor{UserID: courierPedroID}, want403: true,
			call: func(s *DeliveryService, a DeliveryActor) error {
				_, err := s.Deliver(crmContext(), deliveryOrderID, a)
				return err
			}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rig := newDeliveryRig(t)
			rig.repo.EXPECT().FindTrackingState(mock.Anything, deliveryOrderID).Return(tt.state, nil)

			err := tt.call(rig.service, tt.actor)
			if tt.want403 {
				require.ErrorIs(t, err, domainError.ErrDeliveryForbidden)
			} else {
				var transition *domainError.DeliveryTransitionError
				require.True(t, errors.As(err, &transition))
				assert.Equal(t, "ready", transition.Current)
			}
			assert.Empty(t, rig.outbox.events())
		})
	}
}

func TestDeliveryService_LostRaceAnswers409WithTheFreshStatus(t *testing.T) {
	rig := newDeliveryRig(t)
	rig.repo.EXPECT().FindTrackingState(mock.Anything, deliveryOrderID).Return(agentDeliveryState(dto.OrderTypeDelivery, dto.DeliveryStatusReady, nil), nil).Once()
	rig.repo.EXPECT().FindActiveCourier(mock.Anything, courierJuanID).Return(&dto.CourierUser{ID: courierJuanID, Name: "Juan"}, true, nil)
	rig.repo.EXPECT().ApplyStatusChange(mock.Anything, mock.Anything).Return(false, nil)
	rig.repo.EXPECT().FindTrackingState(mock.Anything, deliveryOrderID).
		Return(agentDeliveryState(dto.OrderTypeDelivery, dto.DeliveryStatusOutForDelivery, strPtr(courierPedroID)), nil).Once()

	_, err := rig.service.Dispatch(crmContext(), deliveryOrderID, DeliveryActor{UserID: courierJuanID}, dto.DispatchDeliveryRequest{})

	var transition *domainError.DeliveryTransitionError
	require.True(t, errors.As(err, &transition))
	assert.Equal(t, "out_for_delivery", transition.Current)
	assert.Empty(t, rig.outbox.events(), "the loser notifies nothing")
}

func TestDeliveryService_RollbackLeavesNoEvent(t *testing.T) {
	rig := newDeliveryRig(t)
	rig.outbox.appendErr = errors.New("outbox down")
	rig.repo.EXPECT().FindTrackingState(mock.Anything, deliveryOrderID).
		Return(agentDeliveryState(dto.OrderTypeDelivery, dto.DeliveryStatusOutForDelivery, strPtr(courierJuanID)), nil)
	rig.repo.EXPECT().ApplyStatusChange(mock.Anything, mock.Anything).Return(true, nil)

	_, err := rig.service.Deliver(crmContext(), deliveryOrderID, DeliveryActor{UserID: deliveryMgrID, Manage: true})

	require.ErrorIs(t, err, domainError.ErrDeliveryOperation, "the unit of work fails, so the history entry rolls back with it")
	assert.Empty(t, rig.outbox.events())
}

func TestDeliveryService_TillOrderNotifiesNothing(t *testing.T) {
	rig := newDeliveryRig(t)
	till := agentDeliveryState(dto.OrderTypeDelivery, dto.DeliveryStatusOutForDelivery, nil)
	till.CourierName = strPtr("Mensajeros Ya")
	till.Fulfillment.Origin = nil
	rig.repo.EXPECT().FindTrackingState(mock.Anything, deliveryOrderID).Return(till, nil)
	rig.expectApplied(t, func(*dto.DeliveryStatusChange) {})

	_, err := rig.service.Deliver(crmContext(), deliveryOrderID, DeliveryActor{UserID: deliveryMgrID, Manage: true})
	require.NoError(t, err)
	assert.Empty(t, rig.outbox.events())
}

func TestDeliveryService_UnknownOrder(t *testing.T) {
	rig := newDeliveryRig(t)
	rig.repo.EXPECT().FindTrackingState(mock.Anything, deliveryOrderID).Return(nil, domainError.ErrDeliveryOrderNotFound)

	_, err := rig.service.Deliver(crmContext(), deliveryOrderID, DeliveryActor{UserID: deliveryMgrID, Manage: true})
	require.ErrorIs(t, err, domainError.ErrDeliveryOrderNotFound)

	_, err = rig.service.Deliver(crmContext(), "not-a-uuid", DeliveryActor{UserID: deliveryMgrID, Manage: true})
	require.ErrorIs(t, err, domainError.ErrDeliveryOrderNotFound)
	_, err = rig.service.History(crmContext(), "not-a-uuid")
	require.ErrorIs(t, err, domainError.ErrDeliveryOrderNotFound)
}

func TestDeliveryService_List_ValidatesTheFilters(t *testing.T) {
	rig := newDeliveryRig(t)
	since := deliveryNow.Add(-time.Hour)

	_, err := rig.service.List(crmContext(), dto.DeliveryListFilter{Status: "lost"}, since)
	var validation *domainError.ValidationError
	require.True(t, errors.As(err, &validation))
	assert.Equal(t, "status", validation.Field)
	_, err = rig.service.List(crmContext(), dto.DeliveryListFilter{OrderType: dto.OrderTypeDineIn}, since)
	require.True(t, errors.As(err, &validation))
	assert.Equal(t, "order_type", validation.Field)

	filter := dto.DeliveryListFilter{Status: dto.DeliveryStatusOutForDelivery, OrderType: dto.OrderTypeDelivery}
	rig.repo.EXPECT().List(mock.Anything, filter, since).Return([]dto.DeliveryOrder{{ID: deliveryOrderID}}, nil)
	list, err := rig.service.List(crmContext(), filter, since)
	require.NoError(t, err)
	assert.Len(t, list.Orders, 1)
}

func TestDeliveryService_Mine(t *testing.T) {
	rig := newDeliveryRig(t)
	rig.repo.EXPECT().Claimable(mock.Anything).Return([]dto.DeliveryOrder{{ID: "a"}}, nil)
	rig.repo.EXPECT().OnTheRoad(mock.Anything, courierJuanID).Return([]dto.DeliveryOrder{{ID: "b"}, {ID: "c"}}, nil)

	queue, err := rig.service.Mine(crmContext(), courierJuanID)
	require.NoError(t, err)
	assert.Len(t, queue.Claimable, 1)
	assert.Len(t, queue.OnTheRoad, 2)
}

func TestDeliveryService_CouriersAndHistory(t *testing.T) {
	rig := newDeliveryRig(t)
	rig.repo.EXPECT().ActiveCouriers(mock.Anything).Return([]dto.CourierUser{{ID: courierJuanID, Name: "Juan"}}, nil)
	rig.repo.EXPECT().History(mock.Anything, deliveryOrderID).Return(&dto.DeliveryHistory{OrderID: deliveryOrderID}, nil)

	couriers, err := rig.service.Couriers(crmContext())
	require.NoError(t, err)
	assert.Len(t, couriers.Couriers, 1)
	history, err := rig.service.History(crmContext(), deliveryOrderID)
	require.NoError(t, err)
	assert.Equal(t, deliveryOrderID, history.OrderID)
}
