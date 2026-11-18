package notification_rule

import (
	"fmt"
	"slices"
	"time"
	"unicode/utf8"

	ruleError "laguna-escondida/backend/internal/domain/aggregate/notification_rule/error"
	"laguna-escondida/backend/internal/domain/dto"

	"github.com/google/uuid"
)

const (
	maxTitlePrefixLength  = 100
	minExpiresAfter       = 1
	maxExpiresAfter       = 10080
	orderExpiresAfterMin  = 60
	reservationExpiresMin = 720
)

type Aggregate struct {
	id          string
	orgID       string
	channelID   string
	event       dto.NotificationEventType
	conditions  dto.NotificationConditions
	titlePrefix string
	expiresMin  int
	enabled     bool
	createdAt   time.Time
	updatedAt   time.Time
}

// NewAggregateFromCreateRequest does not check that the channel exists; the service owns that.
func NewAggregateFromCreateRequest(orgID string, req *dto.CreateNotificationRuleRequest) (*Aggregate, error) {
	if req.ChannelID == "" {
		return nil, ruleError.NewInvalidFieldError("channel_id", "is required")
	}
	if !isKnownEvent(req.Event) {
		return nil, ruleError.NewInvalidFieldError("event", "must be one of: order.created, order.paid, reservation.requested, reservation.confirmed")
	}
	if err := validateConditions(req.Event, &req.Conditions); err != nil {
		return nil, err
	}
	if err := validateTitlePrefix(req.TitlePrefix); err != nil {
		return nil, err
	}
	expires := defaultExpiresAfter(req.Event)
	if req.ExpiresAfterMinutes != nil {
		if err := validateExpiresAfter(*req.ExpiresAfterMinutes); err != nil {
			return nil, err
		}
		expires = *req.ExpiresAfterMinutes
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	now := time.Now()
	return &Aggregate{
		id:          uuid.Must(uuid.NewV7()).String(),
		orgID:       orgID,
		channelID:   req.ChannelID,
		event:       req.Event,
		conditions:  req.Conditions,
		titlePrefix: req.TitlePrefix,
		expiresMin:  expires,
		enabled:     enabled,
		createdAt:   now,
		updatedAt:   now,
	}, nil
}

func NewAggregateFromDTO(rule *dto.NotificationRule) *Aggregate {
	return &Aggregate{
		id:          rule.ID,
		orgID:       rule.OrganizationID,
		channelID:   rule.ChannelID,
		event:       rule.Event,
		conditions:  rule.Conditions,
		titlePrefix: rule.TitlePrefix,
		expiresMin:  rule.ExpiresAfterMinutes,
		enabled:     rule.Enabled,
		createdAt:   rule.CreatedAt,
		updatedAt:   rule.UpdatedAt,
	}
}

// Update leaves the event alone: its conditions and default expiry depend on it.
func (a *Aggregate) Update(req *dto.UpdateNotificationRuleRequest) error {
	if req.ChannelID != nil {
		if *req.ChannelID == "" {
			return ruleError.NewInvalidFieldError("channel_id", "is required")
		}
		a.channelID = *req.ChannelID
	}
	if req.Conditions != nil {
		if err := validateConditions(a.event, req.Conditions); err != nil {
			return err
		}
		a.conditions = *req.Conditions
	}
	if req.TitlePrefix != nil {
		if err := validateTitlePrefix(*req.TitlePrefix); err != nil {
			return err
		}
		a.titlePrefix = *req.TitlePrefix
	}
	if req.ExpiresAfterMinutes != nil {
		if err := validateExpiresAfter(*req.ExpiresAfterMinutes); err != nil {
			return err
		}
		a.expiresMin = *req.ExpiresAfterMinutes
	}
	if req.Enabled != nil {
		a.enabled = *req.Enabled
	}
	a.updatedAt = time.Now()
	return nil
}

func (a *Aggregate) ChannelID() string { return a.channelID }

// Matches holds when the event is the rule's and every condition the rule sets holds.
func (a *Aggregate) Matches(subject dto.NotificationSubject) bool {
	if subject.Event != a.event {
		return false
	}
	c := a.conditions
	if len(c.OrderTypes) > 0 && (subject.OrderType == nil || !slices.Contains(c.OrderTypes, *subject.OrderType)) {
		return false
	}
	if len(c.Sources) > 0 && (subject.Source == nil || !slices.Contains(c.Sources, *subject.Source)) {
		return false
	}
	if len(c.ConfirmedBy) > 0 && (subject.ConfirmedBy == nil || !slices.Contains(c.ConfirmedBy, *subject.ConfirmedBy)) {
		return false
	}
	if c.MinPartySize != nil && (subject.PartySize == nil || *subject.PartySize < *c.MinPartySize) {
		return false
	}
	return true
}

func (a *Aggregate) ToDTO() *dto.NotificationRule {
	return &dto.NotificationRule{
		ID:                  a.id,
		OrganizationID:      a.orgID,
		ChannelID:           a.channelID,
		Event:               a.event,
		Conditions:          a.conditions,
		TitlePrefix:         a.titlePrefix,
		ExpiresAfterMinutes: a.expiresMin,
		Enabled:             a.enabled,
		CreatedAt:           a.createdAt,
		UpdatedAt:           a.updatedAt,
	}
}

func isKnownEvent(e dto.NotificationEventType) bool {
	switch e {
	case dto.NotificationEventOrderCreated, dto.NotificationEventOrderPaid,
		dto.NotificationEventReservationRequested, dto.NotificationEventReservationConfirmed:
		return true
	}
	return false
}

func isOrderEvent(e dto.NotificationEventType) bool {
	return e == dto.NotificationEventOrderCreated || e == dto.NotificationEventOrderPaid
}

func defaultExpiresAfter(e dto.NotificationEventType) int {
	if isOrderEvent(e) {
		return orderExpiresAfterMin
	}
	return reservationExpiresMin
}

func validateTitlePrefix(prefix string) error {
	if utf8.RuneCountInString(prefix) > maxTitlePrefixLength {
		return ruleError.NewInvalidFieldError("title_prefix", "must be at most 100 characters")
	}
	return nil
}

func validateExpiresAfter(minutes int) error {
	if minutes < minExpiresAfter || minutes > maxExpiresAfter {
		return ruleError.NewInvalidFieldError("expires_after_minutes", "must be between 1 and 10080")
	}
	return nil
}

func validateConditions(event dto.NotificationEventType, c *dto.NotificationConditions) error {
	orderEvent := isOrderEvent(event)
	confirmed := event == dto.NotificationEventReservationConfirmed

	if c.OrderTypes != nil {
		if !orderEvent {
			return notAllowed("order_types")
		}
		for _, v := range c.OrderTypes {
			if v != dto.OrderTypeDineIn && v != dto.OrderTypeTakeaway && v != dto.OrderTypeDelivery {
				return invalidValue("order_types", v)
			}
		}
	}
	if c.Sources != nil {
		if orderEvent {
			return notAllowed("sources")
		}
		for _, v := range c.Sources {
			switch v {
			case dto.ReservationSourceWhatsApp, dto.ReservationSourcePhone, dto.ReservationSourceWalkIn, dto.ReservationSourceStaff:
			default:
				return invalidValue("sources", v)
			}
		}
	}
	if c.ConfirmedBy != nil {
		if !confirmed {
			return notAllowed("confirmed_by")
		}
		for _, v := range c.ConfirmedBy {
			if v != dto.ConfirmedByStaff && v != dto.ConfirmedByAuto {
				return invalidValue("confirmed_by", v)
			}
		}
	}
	if c.MinPartySize != nil {
		if orderEvent {
			return notAllowed("min_party_size")
		}
		if *c.MinPartySize <= 0 {
			return ruleError.NewInvalidFieldError("conditions.min_party_size", "must be a positive integer")
		}
	}
	return nil
}

func notAllowed(condition string) error {
	return ruleError.NewInvalidFieldError("conditions."+condition, "is not a condition of this event")
}

func invalidValue[T ~string](condition string, v T) error {
	return ruleError.NewInvalidFieldError("conditions."+condition, fmt.Sprintf("unknown value %q", string(v)))
}
