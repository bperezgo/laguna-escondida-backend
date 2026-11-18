// Package marketing_event builds the bodies this backend sends to marketing-app. The bytes are
// built once, stored, and signed and sent unchanged on every retry.
package marketing_event

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"laguna-escondida/backend/internal/domain/dto"

	"github.com/google/uuid"
)

// orderBody is marketing-app's existing /backend/events contract; field order is the wire order.
type orderBody struct {
	EventID       string  `json:"event_id"`
	Type          string  `json:"type"`
	OccurredAt    string  `json:"occurred_at"`
	ChannelID     string  `json:"channel_id"`
	OrderID       string  `json:"order_id"`
	CustomerPhone string  `json:"customer_phone"`
	Amount        *string `json:"amount,omitempty"`
	Currency      *string `json:"currency,omitempty"`
}

type notificationBody struct {
	EventID        string          `json:"event_id"`
	Type           string          `json:"type"`
	OccurredAt     string          `json:"occurred_at"`
	ChannelID      string          `json:"channel_id"`
	ConversationID *string         `json:"conversation_id"`
	CustomerPhone  string          `json:"customer_phone"`
	Data           json.RawMessage `json:"data"`
}

type reservationData struct {
	ReservationID string  `json:"reservation_id"`
	Name          string  `json:"name"`
	PartySize     int     `json:"party_size"`
	StartsAt      string  `json:"starts_at"`
	Status        string  `json:"status"`
	Reason        *string `json:"reason,omitempty"`
	CancelledBy   *string `json:"cancelled_by,omitempty"`
}

type feedbackRequestedData struct {
	FeedbackRequestID string `json:"feedback_request_id"`
	ReservationID     string `json:"reservation_id"`
	Name              string `json:"name"`
	StartsAt          string `json:"starts_at"`
	ExpiresAt         string `json:"expires_at"`
}

// NewOrderEvent: amount and currency go only on order.paid.
func NewOrderEvent(eventType dto.MarketingEventType, subject dto.MarketingOrderSubject, currency string, loc *time.Location) (*dto.MarketingEvent, error) {
	if subject.Origin == nil || subject.Origin.ChannelID == nil || subject.CustomerPhone == nil {
		return nil, fmt.Errorf("order %s has no channel or phone to report", subject.OrderID)
	}
	eventID := uuid.Must(uuid.NewV7()).String()
	body := orderBody{
		EventID:       eventID,
		Type:          string(eventType),
		OccurredAt:    formatTime(subject.OccurredAt, loc),
		ChannelID:     *subject.Origin.ChannelID,
		OrderID:       subject.OrderID,
		CustomerPhone: phoneDigits(*subject.CustomerPhone),
	}
	if eventType == dto.MarketingEventOrderPaid {
		amount := subject.Amount.String()
		body.Amount, body.Currency = &amount, &currency
	}
	return newEvent(eventID, eventType, dto.MarketingEventsPathOrders, body, subject.OccurredAt)
}

// NewReservationEvent reports the status the reservation just entered.
func NewReservationEvent(eventType dto.MarketingEventType, r *dto.Reservation, occurredAt time.Time, loc *time.Location) (*dto.MarketingEvent, error) {
	data := reservationData{
		ReservationID: r.ID,
		Name:          r.Name,
		PartySize:     r.PartySize,
		StartsAt:      formatTime(r.StartsAt, loc),
		Status:        string(r.Status),
		Reason:        r.DeclineReason,
	}
	if r.CancelledBy != nil {
		by := string(*r.CancelledBy)
		data.CancelledBy = &by
	}
	return newNotification(eventType, r, data, occurredAt, loc)
}

func NewFeedbackRequestedEvent(req *dto.FeedbackRequest, r *dto.Reservation, loc *time.Location) (*dto.MarketingEvent, error) {
	data := feedbackRequestedData{
		FeedbackRequestID: req.ID,
		ReservationID:     r.ID,
		Name:              r.Name,
		StartsAt:          formatTime(r.StartsAt, loc),
		ExpiresAt:         formatTime(req.ExpiresAt, loc),
	}
	return newNotification(dto.MarketingEventFeedbackRequested, r, data, req.RequestedAt, loc)
}

func newNotification[T reservationData | feedbackRequestedData](eventType dto.MarketingEventType, r *dto.Reservation, data T, occurredAt time.Time, loc *time.Location) (*dto.MarketingEvent, error) {
	if r.Origin.ChannelID == nil {
		return nil, fmt.Errorf("reservation %s has no channel to report", r.ID)
	}
	rawData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("encode %s data: %w", eventType, err)
	}
	eventID := uuid.Must(uuid.NewV7()).String()
	body := notificationBody{
		EventID:        eventID,
		Type:           string(eventType),
		OccurredAt:     formatTime(occurredAt, loc),
		ChannelID:      *r.Origin.ChannelID,
		ConversationID: r.Origin.ConversationID,
		CustomerPhone:  phoneDigits(r.ContactPhone),
		Data:           rawData,
	}
	return newEvent(eventID, eventType, dto.MarketingEventsPathNotifications, body, occurredAt)
}

func newEvent[T orderBody | notificationBody](eventID string, eventType dto.MarketingEventType, path string, body T, at time.Time) (*dto.MarketingEvent, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encode %s: %w", eventType, err)
	}
	return &dto.MarketingEvent{
		EventID:       eventID,
		Type:          eventType,
		Path:          path,
		Body:          raw,
		Status:        dto.MarketingEventPending,
		NextAttemptAt: at,
		CreatedAt:     at,
	}, nil
}

// formatTime is RFC 3339 with the organization's offset, to the second.
func formatTime(t time.Time, loc *time.Location) string {
	return t.In(loc).Format(time.RFC3339)
}

// phoneDigits is E.164 without "+", the form marketing-app keys conversations by.
func phoneDigits(phone string) string {
	return strings.TrimPrefix(phone, "+")
}

// ReservationEventType is the event a reservation's new status produces, if any.
func ReservationEventType(status dto.ReservationStatus) (dto.MarketingEventType, bool) {
	switch status {
	case dto.ReservationStatusConfirmed:
		return dto.MarketingEventReservationConfirmed, true
	case dto.ReservationStatusDeclined:
		return dto.MarketingEventReservationDeclined, true
	case dto.ReservationStatusCancelled:
		return dto.MarketingEventReservationCancelled, true
	}
	return "", false
}
