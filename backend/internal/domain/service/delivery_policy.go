package service

import (
	"strings"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
)

// DeliveryActor is who asks for a change: Manage for admins and managers, otherwise a courier
// carrying on their own behalf.
type DeliveryActor struct {
	UserID string
	Manage bool
}

// DeliveryRequest is a change as asked; CourierUserID/CourierName only apply to a dispatch.
type DeliveryRequest struct {
	Action        dto.DeliveryAction
	CourierUserID *string
	CourierName   *string
	Reason        dto.DeliveryFailureReason
	Note          *string
}

// DeliveryDecision is the change the policy allows.
type DeliveryDecision struct {
	Status        dto.DeliveryStatus
	CourierUserID *string
	CourierName   *string
	Reason        *dto.DeliveryFailureReason
	Note          *string
}

// normalizeDeliveryRequest validates the request's own fields; a courier's dispatch naming
// nobody means themselves.
func normalizeDeliveryRequest(req DeliveryRequest, actor DeliveryActor) (DeliveryRequest, error) {
	switch req.Action {
	case dto.DeliveryActionDispatch:
		req.CourierName = trimmedOrNilString(req.CourierName)
		req.CourierUserID = trimmedOrNilString(req.CourierUserID)
		if !actor.Manage && req.CourierUserID == nil && req.CourierName == nil {
			self := actor.UserID
			req.CourierUserID = &self
		}
		if (req.CourierUserID == nil) == (req.CourierName == nil) {
			return req, &domainError.ValidationError{Field: "courier", Reason: "name exactly one of courier_user_id or courier_name"}
		}
		if req.CourierName != nil && len([]rune(*req.CourierName)) > dto.MaxCourierNameLength {
			return req, &domainError.ValidationError{Field: "courier_name", Reason: "must be at most 80 characters"}
		}
	case dto.DeliveryActionFail:
		if !req.Reason.Valid() {
			return req, &domainError.ValidationError{Field: "reason", Reason: "must be customer_unreachable, wrong_address, refused or other"}
		}
		req.Note = trimmedOrNilString(req.Note)
		if req.Note != nil && len([]rune(*req.Note)) > dto.MaxDeliveryNoteLength {
			return req, &domainError.ValidationError{Field: "note", Reason: "must be at most 500 characters"}
		}
	case dto.DeliveryActionDeliver:
	default:
		return req, &domainError.ValidationError{Field: "action", Reason: "unknown delivery action"}
	}
	return req, nil
}

// decideDelivery applies the spec's transition table, then who may make it. The transition is
// checked first, so a courier asking the impossible of their own order gets 409, not 403.
func decideDelivery(state dto.DeliveryTrackingState, actor DeliveryActor, req DeliveryRequest) (DeliveryDecision, error) {
	if !deliveryTracked(state) {
		return DeliveryDecision{}, &domainError.DeliveryTransitionError{Action: string(req.Action)}
	}
	refuse := &domainError.DeliveryTransitionError{Current: string(state.Status), Action: string(req.Action)}
	isDelivery := state.OrderType == dto.OrderTypeDelivery
	notOut := state.Status == dto.DeliveryStatusPreparing || state.Status == dto.DeliveryStatusReady

	switch req.Action {
	case dto.DeliveryActionDispatch:
		if !isDelivery || (!notOut && state.Status != dto.DeliveryStatusFailed) {
			return DeliveryDecision{}, refuse
		}
		if !actor.Manage {
			claim := state.Status == dto.DeliveryStatusReady && state.CourierUserID == nil &&
				req.CourierName == nil && req.CourierUserID != nil && *req.CourierUserID == actor.UserID
			if !claim {
				return DeliveryDecision{}, domainError.ErrDeliveryForbidden
			}
		}
		return DeliveryDecision{Status: dto.DeliveryStatusOutForDelivery, CourierUserID: req.CourierUserID, CourierName: req.CourierName}, nil

	case dto.DeliveryActionDeliver:
		allowed := (isDelivery && state.Status == dto.DeliveryStatusOutForDelivery) || (!isDelivery && notOut)
		if !allowed {
			return DeliveryDecision{}, refuse
		}
		if !actor.Manage && !carriedBy(state, actor) {
			return DeliveryDecision{}, domainError.ErrDeliveryForbidden
		}
		return DeliveryDecision{Status: dto.DeliveryStatusDelivered, CourierUserID: state.CourierUserID, CourierName: state.CourierName}, nil

	case dto.DeliveryActionFail:
		if !isDelivery || state.Status != dto.DeliveryStatusOutForDelivery {
			return DeliveryDecision{}, refuse
		}
		if !actor.Manage && !carriedBy(state, actor) {
			return DeliveryDecision{}, domainError.ErrDeliveryForbidden
		}
		reason := req.Reason
		return DeliveryDecision{
			Status: dto.DeliveryStatusFailed, CourierUserID: state.CourierUserID, CourierName: state.CourierName,
			Reason: &reason, Note: req.Note,
		}, nil
	}
	return DeliveryDecision{}, refuse
}

func deliveryTracked(state dto.DeliveryTrackingState) bool {
	return state.Paid && !state.Voided && state.OrderType.IsHeldUntilPaid()
}

// carriedBy: only a delivery order on the road can be carried, and takeaway never is.
func carriedBy(state dto.DeliveryTrackingState, actor DeliveryActor) bool {
	return state.OrderType == dto.OrderTypeDelivery && state.Status == dto.DeliveryStatusOutForDelivery &&
		state.CourierUserID != nil && *state.CourierUserID == actor.UserID
}

func trimmedOrNilString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
