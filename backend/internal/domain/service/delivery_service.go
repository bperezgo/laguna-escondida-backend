package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// DeliveryService moves paid takeaway and delivery orders from the kitchen to the customer.
type DeliveryService struct {
	repo            ports.DeliveryRepository
	unitOfWork      ports.UnitOfWork
	marketingEvents ports.MarketingEventRecorder
	now             func() time.Time
}

// NewDeliveryService: marketingEvents may be nil, and then nothing is notified.
func NewDeliveryService(repo ports.DeliveryRepository, unitOfWork ports.UnitOfWork, marketingEvents ports.MarketingEventRecorder, now func() time.Time) *DeliveryService {
	if now == nil {
		now = time.Now
	}
	return &DeliveryService{repo: repo, unitOfWork: unitOfWork, marketingEvents: marketingEvents, now: now}
}

var errDeliveryChangedMeanwhile = errors.New("the order changed meanwhile")

func (s *DeliveryService) List(ctx context.Context, filter dto.DeliveryListFilter, deliveredSince time.Time) (*dto.DeliveryOrderList, error) {
	if filter.Status != "" && !filter.Status.Valid() {
		return nil, &domainError.ValidationError{Field: "status", Reason: "must be preparing, ready, out_for_delivery, delivered or failed"}
	}
	if filter.OrderType != "" && !filter.OrderType.IsHeldUntilPaid() {
		return nil, &domainError.ValidationError{Field: "order_type", Reason: "must be takeaway or delivery"}
	}
	orders, err := s.repo.List(ctx, filter, deliveredSince)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrDeliveryOperation, err)
	}
	return &dto.DeliveryOrderList{Orders: orders}, nil
}

func (s *DeliveryService) History(ctx context.Context, orderID string) (*dto.DeliveryHistory, error) {
	if !isUUID(orderID) {
		return nil, domainError.ErrDeliveryOrderNotFound
	}
	history, err := s.repo.History(ctx, orderID)
	if err != nil {
		return nil, wrapDeliveryErr(err)
	}
	return history, nil
}

func (s *DeliveryService) Couriers(ctx context.Context) (*dto.CourierList, error) {
	couriers, err := s.repo.ActiveCouriers(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrDeliveryOperation, err)
	}
	return &dto.CourierList{Couriers: couriers}, nil
}

func (s *DeliveryService) Mine(ctx context.Context, courierUserID string) (*dto.CourierQueue, error) {
	claimable, err := s.repo.Claimable(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrDeliveryOperation, err)
	}
	onTheRoad, err := s.repo.OnTheRoad(ctx, courierUserID)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrDeliveryOperation, err)
	}
	return &dto.CourierQueue{Claimable: claimable, OnTheRoad: onTheRoad}, nil
}

func (s *DeliveryService) Dispatch(ctx context.Context, orderID string, actor DeliveryActor, req dto.DispatchDeliveryRequest) (*dto.DeliveryOrder, error) {
	return s.change(ctx, orderID, actor, DeliveryRequest{Action: dto.DeliveryActionDispatch, CourierUserID: req.CourierUserID, CourierName: req.CourierName})
}

func (s *DeliveryService) Deliver(ctx context.Context, orderID string, actor DeliveryActor) (*dto.DeliveryOrder, error) {
	return s.change(ctx, orderID, actor, DeliveryRequest{Action: dto.DeliveryActionDeliver})
}

func (s *DeliveryService) Fail(ctx context.Context, orderID string, actor DeliveryActor, req dto.FailDeliveryRequest) (*dto.DeliveryOrder, error) {
	return s.change(ctx, orderID, actor, DeliveryRequest{Action: dto.DeliveryActionFail, Reason: req.Reason, Note: req.Note})
}

func (s *DeliveryService) change(ctx context.Context, orderID string, actor DeliveryActor, req DeliveryRequest) (*dto.DeliveryOrder, error) {
	req, err := normalizeDeliveryRequest(req, actor)
	if err != nil {
		return nil, err
	}
	if !isUUID(orderID) {
		return nil, domainError.ErrDeliveryOrderNotFound
	}
	state, err := s.repo.FindTrackingState(ctx, orderID)
	if err != nil {
		return nil, wrapDeliveryErr(err)
	}
	decision, err := decideDelivery(*state, actor, req)
	if err != nil {
		return nil, err
	}
	if req.Action == dto.DeliveryActionDispatch && decision.CourierUserID != nil {
		courierName, courierErr := s.activeCourierName(ctx, *decision.CourierUserID)
		if courierErr != nil {
			return nil, courierErr
		}
		decision.CourierName = &courierName
	}

	now := s.now()
	change := &dto.DeliveryStatusChange{
		EventID:               uuid.Must(uuid.NewV7()).String(),
		OrderID:               orderID,
		ExpectedCourierUserID: state.CourierUserID,
		Status:                decision.Status,
		CourierUserID:         decision.CourierUserID,
		CourierName:           decision.CourierName,
		Reason:                decision.Reason,
		Note:                  decision.Note,
		ActorUserID:           actor.UserID,
		OccurredAt:            now,
	}
	if state.Status.IsRecorded() {
		seen := state.Status
		change.ExpectedStatus = &seen
	}

	err = s.unitOfWork.Do(ctx, func(ctx context.Context) error {
		changed, applyErr := s.repo.ApplyStatusChange(ctx, change)
		if applyErr != nil {
			return fmt.Errorf("%w: %w", domainError.ErrDeliveryOperation, applyErr)
		}
		if !changed {
			return errDeliveryChangedMeanwhile
		}
		return s.notify(ctx, state, decision, now)
	})
	if errors.Is(err, errDeliveryChangedMeanwhile) {
		return nil, s.lostRace(ctx, orderID, req.Action)
	}
	if err != nil {
		return nil, err
	}
	order, err := s.repo.FindOrder(ctx, orderID)
	if err != nil {
		return nil, wrapDeliveryErr(err)
	}
	return order, nil
}

func (s *DeliveryService) activeCourierName(ctx context.Context, userID string) (string, error) {
	invalid := &domainError.ValidationError{Field: "courier_user_id", Reason: "must be an active courier of the organization"}
	if !isUUID(userID) {
		return "", invalid
	}
	courier, ok, err := s.repo.FindActiveCourier(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("%w: %w", domainError.ErrDeliveryOperation, err)
	}
	if !ok {
		return "", invalid
	}
	return courier.Name, nil
}

// lostRace answers the loser of a concurrent change with the status the winner left.
func (s *DeliveryService) lostRace(ctx context.Context, orderID string, action dto.DeliveryAction) error {
	fresh, err := s.repo.FindTrackingState(ctx, orderID)
	if err != nil {
		return wrapDeliveryErr(err)
	}
	return &domainError.DeliveryTransitionError{Current: string(fresh.Status), Action: string(action)}
}

func (s *DeliveryService) notify(ctx context.Context, state *dto.DeliveryTrackingState, decision DeliveryDecision, at time.Time) error {
	if s.marketingEvents == nil || state.Fulfillment.Origin == nil {
		return nil
	}
	progress := dto.MarketingDeliveryProgress{OrderType: state.OrderType}
	var eventType dto.MarketingEventType
	switch decision.Status {
	case dto.DeliveryStatusOutForDelivery:
		eventType, progress.CourierName = dto.MarketingEventOrderOutForDelivery, decision.CourierName
	case dto.DeliveryStatusDelivered:
		eventType = dto.MarketingEventOrderDelivered
	case dto.DeliveryStatusFailed:
		eventType, progress.Reason = dto.MarketingEventOrderDeliveryFailed, decision.Reason
	default:
		return nil
	}
	subject := dto.MarketingOrderSubject{
		OrderID: state.OrderID, CustomerPhone: state.Fulfillment.CustomerPhone, Origin: state.Fulfillment.Origin,
		Amount: decimal.Zero, OccurredAt: at,
	}
	if err := s.marketingEvents.RecordDelivery(ctx, eventType, subject, progress); err != nil {
		return fmt.Errorf("%w: %w", domainError.ErrDeliveryOperation, err)
	}
	return nil
}

func wrapDeliveryErr(err error) error {
	if errors.Is(err, domainError.ErrDeliveryOrderNotFound) {
		return err
	}
	return fmt.Errorf("%w: %w", domainError.ErrDeliveryOperation, err)
}

func isUUID(value string) bool {
	_, err := uuid.Parse(value)
	return err == nil
}
