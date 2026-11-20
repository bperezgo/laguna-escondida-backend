package service

import (
	"errors"
	"strings"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	juanID  = "juan-id"
	pedroID = "pedro-id"
)

var (
	manager = DeliveryActor{UserID: "manager-id", Manage: true}
	juan    = DeliveryActor{UserID: juanID}
	pedro   = DeliveryActor{UserID: pedroID}
)

func trackedOrder(orderType dto.OrderType, status dto.DeliveryStatus, courier *string) dto.DeliveryTrackingState {
	return dto.DeliveryTrackingState{OrderID: "order-1", OrderType: orderType, Paid: true, Status: status, CourierUserID: courier}
}

func TestDeliveryPolicy_Decide(t *testing.T) {
	dispatchTo := func(userID string) DeliveryRequest {
		return DeliveryRequest{Action: dto.DeliveryActionDispatch, CourierUserID: strPtr(userID)}
	}
	deliver := DeliveryRequest{Action: dto.DeliveryActionDeliver}
	fail := DeliveryRequest{Action: dto.DeliveryActionFail, Reason: dto.DeliveryFailureWrongAddress, Note: strPtr("no existe")}
	unpaid := trackedOrder(dto.OrderTypeDelivery, dto.DeliveryStatusReady, nil)
	unpaid.Paid = false
	voided := trackedOrder(dto.OrderTypeDelivery, dto.DeliveryStatusReady, nil)
	voided.Voided = true

	tests := []struct {
		name       string
		state      dto.DeliveryTrackingState
		actor      DeliveryActor
		req        DeliveryRequest
		wantStatus dto.DeliveryStatus
		wantErr    string // "409" or "403"
	}{
		// Transitions
		{name: "ready delivery goes out", state: trackedOrder(dto.OrderTypeDelivery, dto.DeliveryStatusReady, nil), actor: manager, req: dispatchTo(juanID), wantStatus: dto.DeliveryStatusOutForDelivery},
		{name: "preparing delivery goes out with a manager", state: trackedOrder(dto.OrderTypeDelivery, dto.DeliveryStatusPreparing, nil), actor: manager, req: dispatchTo(juanID), wantStatus: dto.DeliveryStatusOutForDelivery},
		{name: "a third-party courier", state: trackedOrder(dto.OrderTypeDelivery, dto.DeliveryStatusReady, nil), actor: manager, req: DeliveryRequest{Action: dto.DeliveryActionDispatch, CourierName: strPtr("Mensajeros Ya")}, wantStatus: dto.DeliveryStatusOutForDelivery},
		{name: "on the road is delivered", state: trackedOrder(dto.OrderTypeDelivery, dto.DeliveryStatusOutForDelivery, strPtr(juanID)), actor: manager, req: deliver, wantStatus: dto.DeliveryStatusDelivered},
		{name: "on the road fails", state: trackedOrder(dto.OrderTypeDelivery, dto.DeliveryStatusOutForDelivery, strPtr(juanID)), actor: manager, req: fail, wantStatus: dto.DeliveryStatusFailed},
		{name: "failed is re-dispatched", state: trackedOrder(dto.OrderTypeDelivery, dto.DeliveryStatusFailed, strPtr(juanID)), actor: manager, req: dispatchTo(pedroID), wantStatus: dto.DeliveryStatusOutForDelivery},
		{name: "takeaway picked up", state: trackedOrder(dto.OrderTypeTakeaway, dto.DeliveryStatusReady, nil), actor: manager, req: deliver, wantStatus: dto.DeliveryStatusDelivered},
		{name: "takeaway picked up while preparing", state: trackedOrder(dto.OrderTypeTakeaway, dto.DeliveryStatusPreparing, nil), actor: manager, req: deliver, wantStatus: dto.DeliveryStatusDelivered},
		{name: "a third party reports back", state: dto.DeliveryTrackingState{OrderType: dto.OrderTypeDelivery, Paid: true, Status: dto.DeliveryStatusOutForDelivery, CourierName: strPtr("Mensajeros Ya")}, actor: manager, req: deliver, wantStatus: dto.DeliveryStatusDelivered},

		// Refused transitions
		{name: "delivered without leaving", state: trackedOrder(dto.OrderTypeDelivery, dto.DeliveryStatusReady, nil), actor: manager, req: deliver, wantErr: "409"},
		{name: "takeaway on the road", state: trackedOrder(dto.OrderTypeTakeaway, dto.DeliveryStatusReady, nil), actor: manager, req: dispatchTo(juanID), wantErr: "409"},
		{name: "takeaway cannot fail", state: trackedOrder(dto.OrderTypeTakeaway, dto.DeliveryStatusReady, nil), actor: manager, req: fail, wantErr: "409"},
		{name: "an unpaid order", state: unpaid, actor: manager, req: dispatchTo(juanID), wantErr: "409"},
		{name: "a voided order", state: voided, actor: manager, req: dispatchTo(juanID), wantErr: "409"},
		{name: "a dine-in order", state: dto.DeliveryTrackingState{OrderType: dto.OrderTypeDineIn, Paid: true}, actor: manager, req: deliver, wantErr: "409"},
		{name: "delivered is final", state: trackedOrder(dto.OrderTypeDelivery, dto.DeliveryStatusDelivered, strPtr(juanID)), actor: manager, req: dispatchTo(juanID), wantErr: "409"},
		{name: "delivered takeaway is final", state: trackedOrder(dto.OrderTypeTakeaway, dto.DeliveryStatusDelivered, nil), actor: manager, req: deliver, wantErr: "409"},
		{name: "already on the road cannot go out again", state: trackedOrder(dto.OrderTypeDelivery, dto.DeliveryStatusOutForDelivery, strPtr(juanID)), actor: manager, req: dispatchTo(pedroID), wantErr: "409"},
		{name: "a failed order cannot be delivered", state: trackedOrder(dto.OrderTypeDelivery, dto.DeliveryStatusFailed, strPtr(juanID)), actor: manager, req: deliver, wantErr: "409"},
		{name: "a ready order cannot fail", state: trackedOrder(dto.OrderTypeDelivery, dto.DeliveryStatusReady, nil), actor: manager, req: fail, wantErr: "409"},

		// Couriers
		{name: "a courier claims an order", state: trackedOrder(dto.OrderTypeDelivery, dto.DeliveryStatusReady, nil), actor: juan, req: dispatchTo(juanID), wantStatus: dto.DeliveryStatusOutForDelivery},
		{name: "a courier delivers their own", state: trackedOrder(dto.OrderTypeDelivery, dto.DeliveryStatusOutForDelivery, strPtr(juanID)), actor: juan, req: deliver, wantStatus: dto.DeliveryStatusDelivered},
		{name: "a courier fails their own", state: trackedOrder(dto.OrderTypeDelivery, dto.DeliveryStatusOutForDelivery, strPtr(juanID)), actor: juan, req: fail, wantStatus: dto.DeliveryStatusFailed},
		{name: "another courier's order", state: trackedOrder(dto.OrderTypeDelivery, dto.DeliveryStatusOutForDelivery, strPtr(juanID)), actor: pedro, req: deliver, wantErr: "403"},
		{name: "another courier's order cannot fail", state: trackedOrder(dto.OrderTypeDelivery, dto.DeliveryStatusOutForDelivery, strPtr(juanID)), actor: pedro, req: fail, wantErr: "403"},
		{name: "a third party's order", state: dto.DeliveryTrackingState{OrderType: dto.OrderTypeDelivery, Paid: true, Status: dto.DeliveryStatusOutForDelivery, CourierName: strPtr("Mensajeros Ya")}, actor: juan, req: deliver, wantErr: "403"},
		{name: "a courier re-dispatches", state: trackedOrder(dto.OrderTypeDelivery, dto.DeliveryStatusFailed, strPtr(juanID)), actor: juan, req: dispatchTo(juanID), wantErr: "403"},
		{name: "a courier claims a preparing order", state: trackedOrder(dto.OrderTypeDelivery, dto.DeliveryStatusPreparing, nil), actor: juan, req: dispatchTo(juanID), wantErr: "403"},
		{name: "a courier names another courier", state: trackedOrder(dto.OrderTypeDelivery, dto.DeliveryStatusReady, nil), actor: juan, req: dispatchTo(pedroID), wantErr: "403"},
		{name: "a courier names a third party", state: trackedOrder(dto.OrderTypeDelivery, dto.DeliveryStatusReady, nil), actor: juan, req: DeliveryRequest{Action: dto.DeliveryActionDispatch, CourierName: strPtr("Mensajeros Ya")}, wantErr: "403"},
		{name: "a courier hands over a takeaway", state: trackedOrder(dto.OrderTypeTakeaway, dto.DeliveryStatusReady, nil), actor: juan, req: deliver, wantErr: "403"},
		{name: "a courier asks the impossible of their own order", state: trackedOrder(dto.OrderTypeDelivery, dto.DeliveryStatusOutForDelivery, strPtr(juanID)), actor: juan, req: dispatchTo(juanID), wantErr: "409"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision, err := decideDelivery(tt.state, tt.actor, tt.req)
			switch tt.wantErr {
			case "409":
				var transition *domainError.DeliveryTransitionError
				require.True(t, errors.As(err, &transition), "want a transition error, got %v", err)
				assert.Equal(t, string(tt.req.Action), transition.Action)
				if deliveryTracked(tt.state) {
					assert.Equal(t, string(tt.state.Status), transition.Current)
				} else {
					assert.Empty(t, transition.Current, "an untracked order has no status")
				}
			case "403":
				require.ErrorIs(t, err, domainError.ErrDeliveryForbidden)
			default:
				require.NoError(t, err)
				assert.Equal(t, tt.wantStatus, decision.Status)
			}
		})
	}
}

func TestDeliveryPolicy_DecisionCarriesTheCourierAndReason(t *testing.T) {
	out, err := decideDelivery(trackedOrder(dto.OrderTypeDelivery, dto.DeliveryStatusReady, nil), manager,
		DeliveryRequest{Action: dto.DeliveryActionDispatch, CourierName: strPtr("Mensajeros Ya")})
	require.NoError(t, err)
	assert.Nil(t, out.CourierUserID)
	assert.Equal(t, "Mensajeros Ya", *out.CourierName)

	failed, err := decideDelivery(trackedOrder(dto.OrderTypeDelivery, dto.DeliveryStatusOutForDelivery, strPtr(juanID)), juan,
		DeliveryRequest{Action: dto.DeliveryActionFail, Reason: dto.DeliveryFailureCustomerUnreachable, Note: strPtr("portería no deja subir, no contesta")})
	require.NoError(t, err)
	assert.Equal(t, juanID, *failed.CourierUserID, "the failed entry keeps the courier")
	assert.Equal(t, dto.DeliveryFailureCustomerUnreachable, *failed.Reason)
	assert.Equal(t, "portería no deja subir, no contesta", *failed.Note)
}

func TestDeliveryPolicy_NormalizeRequest(t *testing.T) {
	long := strings.Repeat("a", 81)
	longNote := strings.Repeat("n", 501)
	tests := []struct {
		name      string
		actor     DeliveryActor
		req       DeliveryRequest
		wantField string
	}{
		{name: "both courier user and name", actor: manager, req: DeliveryRequest{Action: dto.DeliveryActionDispatch, CourierUserID: strPtr(juanID), CourierName: strPtr("X")}, wantField: "courier"},
		{name: "neither as a manager", actor: manager, req: DeliveryRequest{Action: dto.DeliveryActionDispatch}, wantField: "courier"},
		{name: "a blank name is none", actor: manager, req: DeliveryRequest{Action: dto.DeliveryActionDispatch, CourierName: strPtr("   ")}, wantField: "courier"},
		{name: "a name over 80", actor: manager, req: DeliveryRequest{Action: dto.DeliveryActionDispatch, CourierName: &long}, wantField: "courier_name"},
		{name: "failed without a reason", actor: manager, req: DeliveryRequest{Action: dto.DeliveryActionFail}, wantField: "reason"},
		{name: "an unknown reason", actor: manager, req: DeliveryRequest{Action: dto.DeliveryActionFail, Reason: "lost"}, wantField: "reason"},
		{name: "a note over 500", actor: manager, req: DeliveryRequest{Action: dto.DeliveryActionFail, Reason: dto.DeliveryFailureOther, Note: &longNote}, wantField: "note"},
		{name: "a courier naming nobody means themselves", actor: juan, req: DeliveryRequest{Action: dto.DeliveryActionDispatch}},
		{name: "a name of exactly 80", actor: manager, req: DeliveryRequest{Action: dto.DeliveryActionDispatch, CourierName: strPtr(strings.Repeat("ñ", 80))}},
		{name: "deliver needs nothing", actor: juan, req: DeliveryRequest{Action: dto.DeliveryActionDeliver}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := normalizeDeliveryRequest(tt.req, tt.actor)
			if tt.wantField == "" {
				require.NoError(t, err)
				return
			}
			var validation *domainError.ValidationError
			require.True(t, errors.As(err, &validation), "want a validation error, got %v", err)
			assert.Equal(t, tt.wantField, validation.Field)

		})
	}

	self, err := normalizeDeliveryRequest(DeliveryRequest{Action: dto.DeliveryActionDispatch}, juan)
	require.NoError(t, err)
	assert.Equal(t, juanID, *self.CourierUserID)
}

func TestDeliveryTransitionError_MapsToTheSentinel(t *testing.T) {
	err := error(&domainError.DeliveryTransitionError{Current: "ready", Action: "deliver"})
	assert.ErrorIs(t, err, domainError.ErrDeliveryTransition)
	assert.Equal(t, "the order is ready and cannot deliver", err.Error())
	assert.Equal(t, "an order without a delivery status cannot dispatch",
		(&domainError.DeliveryTransitionError{Action: "dispatch"}).Error())
}
