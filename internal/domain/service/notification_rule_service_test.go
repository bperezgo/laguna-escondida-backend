package service

import (
	"errors"
	"strings"
	"testing"

	rule "laguna-escondida/backend/internal/domain/aggregate/notification_rule"
	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func newRuleService(t *testing.T) (*NotificationRuleService, *mocks.MockNotificationRuleRepository, *mocks.MockNotificationChannelRepository) {
	rules := mocks.NewMockNotificationRuleRepository(t)
	channels := mocks.NewMockNotificationChannelRepository(t)
	return NewNotificationRuleService(rules, channels), rules, channels
}

func ptr[T any](v T) *T { return &v }

func TestNotificationRuleService_Create_Defaults(t *testing.T) {
	tests := []struct {
		event   dto.NotificationEventType
		expires int
	}{
		{dto.NotificationEventOrderCreated, 60},
		{dto.NotificationEventOrderPaid, 60},
		{dto.NotificationEventReservationRequested, 720},
		{dto.NotificationEventReservationConfirmed, 720},
	}
	for _, tt := range tests {
		t.Run(string(tt.event), func(t *testing.T) {
			svc, rules, channels := newRuleService(t)
			channels.EXPECT().FindByID(mock.Anything, "c1").Return(storedChannel("c1", "A"), nil).Once()
			rules.EXPECT().Create(mock.Anything, mock.MatchedBy(func(r *dto.NotificationRule) bool {
				return r.OrganizationID == notifOrgID && r.ID != ""
			})).Return(nil).Once()

			got, err := svc.Create(notifCtx(), &dto.CreateNotificationRuleRequest{ChannelID: "c1", Event: tt.event})

			require.NoError(t, err)
			assert.Equal(t, tt.expires, got.ExpiresAfterMinutes)
			assert.True(t, got.Enabled)
		})
	}
}

func TestNotificationRuleService_Create_PaidDeliveries(t *testing.T) {
	svc, rules, channels := newRuleService(t)
	channels.EXPECT().FindByID(mock.Anything, "c1").Return(storedChannel("c1", "A"), nil).Once()
	rules.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Once()

	got, err := svc.Create(notifCtx(), &dto.CreateNotificationRuleRequest{
		ChannelID: "c1", Event: dto.NotificationEventOrderPaid,
		Conditions:  dto.NotificationConditions{OrderTypes: []dto.OrderType{dto.OrderTypeDelivery}},
		TitlePrefix: "<@&987>",
	})

	require.NoError(t, err)
	assert.Equal(t, 60, got.ExpiresAfterMinutes)
	assert.Equal(t, "<@&987>", got.TitlePrefix)
	assert.Equal(t, []dto.OrderType{dto.OrderTypeDelivery}, got.Conditions.OrderTypes)
}

func TestNotificationRuleService_Create_Validation(t *testing.T) {
	paid := dto.NotificationEventOrderPaid
	tests := []struct {
		name  string
		req   dto.CreateNotificationRuleRequest
		field string
	}{
		{"unknown event", dto.CreateNotificationRuleRequest{ChannelID: "c1", Event: "order.deleted"}, "event"},
		{"missing channel", dto.CreateNotificationRuleRequest{Event: paid}, "channel_id"},
		{"party size on order", dto.CreateNotificationRuleRequest{ChannelID: "c1", Event: paid, Conditions: dto.NotificationConditions{MinPartySize: ptr(6)}}, "conditions.min_party_size"},
		{"sources on order", dto.CreateNotificationRuleRequest{ChannelID: "c1", Event: paid, Conditions: dto.NotificationConditions{Sources: []dto.ReservationSource{dto.ReservationSourcePhone}}}, "conditions.sources"},
		{"order types on reservation", dto.CreateNotificationRuleRequest{ChannelID: "c1", Event: dto.NotificationEventReservationRequested, Conditions: dto.NotificationConditions{OrderTypes: []dto.OrderType{dto.OrderTypeDelivery}}}, "conditions.order_types"},
		{"confirmed_by on requested", dto.CreateNotificationRuleRequest{ChannelID: "c1", Event: dto.NotificationEventReservationRequested, Conditions: dto.NotificationConditions{ConfirmedBy: []dto.ConfirmedBy{dto.ConfirmedByAuto}}}, "conditions.confirmed_by"},
		{"unknown order type", dto.CreateNotificationRuleRequest{ChannelID: "c1", Event: paid, Conditions: dto.NotificationConditions{OrderTypes: []dto.OrderType{"drive_thru"}}}, "conditions.order_types"},
		{"unknown source", dto.CreateNotificationRuleRequest{ChannelID: "c1", Event: dto.NotificationEventReservationRequested, Conditions: dto.NotificationConditions{Sources: []dto.ReservationSource{"fax"}}}, "conditions.sources"},
		{"unknown confirmed_by", dto.CreateNotificationRuleRequest{ChannelID: "c1", Event: dto.NotificationEventReservationConfirmed, Conditions: dto.NotificationConditions{ConfirmedBy: []dto.ConfirmedBy{"robot"}}}, "conditions.confirmed_by"},
		{"zero party size", dto.CreateNotificationRuleRequest{ChannelID: "c1", Event: dto.NotificationEventReservationRequested, Conditions: dto.NotificationConditions{MinPartySize: ptr(0)}}, "conditions.min_party_size"},
		{"prefix too long", dto.CreateNotificationRuleRequest{ChannelID: "c1", Event: paid, TitlePrefix: strings.Repeat("a", 101)}, "title_prefix"},
		{"expiry zero", dto.CreateNotificationRuleRequest{ChannelID: "c1", Event: paid, ExpiresAfterMinutes: ptr(0)}, "expires_after_minutes"},
		{"expiry too large", dto.CreateNotificationRuleRequest{ChannelID: "c1", Event: paid, ExpiresAfterMinutes: ptr(10081)}, "expires_after_minutes"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, _, _ := newRuleService(t)
			_, err := svc.Create(notifCtx(), &tt.req)
			requireFieldError(t, err, tt.field)
		})
	}
}

func TestNotificationRuleService_Create_BoundaryValues(t *testing.T) {
	for _, minutes := range []int{1, 10080} {
		svc, rules, channels := newRuleService(t)
		channels.EXPECT().FindByID(mock.Anything, "c1").Return(storedChannel("c1", "A"), nil).Once()
		rules.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Once()
		got, err := svc.Create(notifCtx(), &dto.CreateNotificationRuleRequest{
			ChannelID: "c1", Event: dto.NotificationEventOrderPaid, ExpiresAfterMinutes: ptr(minutes), TitlePrefix: strings.Repeat("a", 100), Enabled: ptr(false),
		})
		require.NoError(t, err)
		assert.Equal(t, minutes, got.ExpiresAfterMinutes)
		assert.False(t, got.Enabled)
	}
}

func TestNotificationRuleService_Create_ReservationConditionsAccepted(t *testing.T) {
	svc, rules, channels := newRuleService(t)
	channels.EXPECT().FindByID(mock.Anything, "c1").Return(storedChannel("c1", "A"), nil).Once()
	rules.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Once()

	_, err := svc.Create(notifCtx(), &dto.CreateNotificationRuleRequest{
		ChannelID: "c1", Event: dto.NotificationEventReservationConfirmed,
		Conditions: dto.NotificationConditions{
			Sources:      []dto.ReservationSource{dto.ReservationSourceWhatsApp},
			ConfirmedBy:  []dto.ConfirmedBy{dto.ConfirmedByAuto, dto.ConfirmedByStaff},
			MinPartySize: ptr(8),
		},
	})

	require.NoError(t, err)
}

func TestNotificationRuleService_Create_ChannelOfAnotherOrganization(t *testing.T) {
	svc, _, channels := newRuleService(t)
	channels.EXPECT().FindByID(mock.Anything, "foreign").Return(nil, domainError.ErrNotificationChannelNotFound).Once()

	_, err := svc.Create(notifCtx(), &dto.CreateNotificationRuleRequest{ChannelID: "foreign", Event: dto.NotificationEventOrderPaid})

	requireFieldError(t, err, "channel_id")
}

func TestNotificationRuleService_Create_Failures(t *testing.T) {
	req := &dto.CreateNotificationRuleRequest{ChannelID: "c1", Event: dto.NotificationEventOrderPaid}
	t.Run("no organization", func(t *testing.T) {
		svc, _, _ := newRuleService(t)
		_, err := svc.Create(t.Context(), req)
		require.Error(t, err)
	})
	t.Run("channel lookup fails", func(t *testing.T) {
		svc, _, channels := newRuleService(t)
		channels.EXPECT().FindByID(mock.Anything, "c1").Return(nil, errors.New("boom")).Once()
		_, err := svc.Create(notifCtx(), req)
		assert.ErrorIs(t, err, domainError.ErrNotificationRuleOperation)
	})
	t.Run("repo fails", func(t *testing.T) {
		svc, rules, channels := newRuleService(t)
		channels.EXPECT().FindByID(mock.Anything, "c1").Return(storedChannel("c1", "A"), nil).Once()
		rules.EXPECT().Create(mock.Anything, mock.Anything).Return(errors.New("boom")).Once()
		_, err := svc.Create(notifCtx(), req)
		assert.ErrorIs(t, err, domainError.ErrNotificationRuleOperation)
	})
}

func storedRule(event dto.NotificationEventType, c dto.NotificationConditions) *dto.NotificationRule {
	return &dto.NotificationRule{ID: "r1", OrganizationID: notifOrgID, ChannelID: "c1", Event: event, Conditions: c, ExpiresAfterMinutes: 60, Enabled: true}
}

func TestNotificationRuleService_List_Get(t *testing.T) {
	svc, rules, _ := newRuleService(t)
	rules.EXPECT().List(mock.Anything, "c1").Return([]*dto.NotificationRule{storedRule(dto.NotificationEventOrderPaid, dto.NotificationConditions{})}, nil).Once()
	rules.EXPECT().List(mock.Anything, "").Return(nil, errors.New("boom")).Once()
	rules.EXPECT().FindByID(mock.Anything, "r1").Return(storedRule(dto.NotificationEventOrderPaid, dto.NotificationConditions{}), nil).Once()

	got, err := svc.List(notifCtx(), "c1")
	require.NoError(t, err)
	assert.Len(t, got, 1)

	_, err = svc.List(notifCtx(), "")
	assert.ErrorIs(t, err, domainError.ErrNotificationRuleOperation)

	rule, err := svc.Get(notifCtx(), "r1")
	require.NoError(t, err)
	assert.Equal(t, "r1", rule.ID)
}

func TestNotificationRuleService_Update(t *testing.T) {
	t.Run("changes fields", func(t *testing.T) {
		svc, rules, _ := newRuleService(t)
		rules.EXPECT().FindByID(mock.Anything, "r1").Return(storedRule(dto.NotificationEventOrderPaid, dto.NotificationConditions{}), nil).Once()
		rules.EXPECT().Update(mock.Anything, mock.Anything).Return(nil).Once()

		got, err := svc.Update(notifCtx(), "r1", &dto.UpdateNotificationRuleRequest{
			Conditions:          &dto.NotificationConditions{OrderTypes: []dto.OrderType{dto.OrderTypeTakeaway}},
			TitlePrefix:         ptr("@here"),
			ExpiresAfterMinutes: ptr(30),
			Enabled:             ptr(false),
		})

		require.NoError(t, err)
		assert.Equal(t, 30, got.ExpiresAfterMinutes)
		assert.Equal(t, "@here", got.TitlePrefix)
		assert.False(t, got.Enabled)
		assert.Equal(t, []dto.OrderType{dto.OrderTypeTakeaway}, got.Conditions.OrderTypes)
	})
	t.Run("moves to another channel", func(t *testing.T) {
		svc, rules, channels := newRuleService(t)
		rules.EXPECT().FindByID(mock.Anything, "r1").Return(storedRule(dto.NotificationEventOrderPaid, dto.NotificationConditions{}), nil).Once()
		channels.EXPECT().FindByID(mock.Anything, "c2").Return(storedChannel("c2", "B"), nil).Once()
		rules.EXPECT().Update(mock.Anything, mock.MatchedBy(func(r *dto.NotificationRule) bool { return r.ChannelID == "c2" })).Return(nil).Once()

		_, err := svc.Update(notifCtx(), "r1", &dto.UpdateNotificationRuleRequest{ChannelID: ptr("c2")})

		require.NoError(t, err)
	})
	t.Run("foreign channel", func(t *testing.T) {
		svc, rules, channels := newRuleService(t)
		rules.EXPECT().FindByID(mock.Anything, "r1").Return(storedRule(dto.NotificationEventOrderPaid, dto.NotificationConditions{}), nil).Once()
		channels.EXPECT().FindByID(mock.Anything, "foreign").Return(nil, domainError.ErrNotificationChannelNotFound).Once()
		_, err := svc.Update(notifCtx(), "r1", &dto.UpdateNotificationRuleRequest{ChannelID: ptr("foreign")})
		requireFieldError(t, err, "channel_id")
	})
	t.Run("not found", func(t *testing.T) {
		svc, rules, _ := newRuleService(t)
		rules.EXPECT().FindByID(mock.Anything, "x").Return(nil, domainError.ErrNotificationRuleNotFound).Once()
		_, err := svc.Update(notifCtx(), "x", &dto.UpdateNotificationRuleRequest{})
		assert.ErrorIs(t, err, domainError.ErrNotificationRuleNotFound)
	})
	invalid := []struct {
		name  string
		req   dto.UpdateNotificationRuleRequest
		field string
	}{
		{"empty channel", dto.UpdateNotificationRuleRequest{ChannelID: ptr("")}, "channel_id"},
		{"condition of another event", dto.UpdateNotificationRuleRequest{Conditions: &dto.NotificationConditions{MinPartySize: ptr(3)}}, "conditions.min_party_size"},
		{"prefix too long", dto.UpdateNotificationRuleRequest{TitlePrefix: ptr(strings.Repeat("a", 101))}, "title_prefix"},
		{"expiry out of range", dto.UpdateNotificationRuleRequest{ExpiresAfterMinutes: ptr(20000)}, "expires_after_minutes"},
	}
	for _, tt := range invalid {
		t.Run(tt.name, func(t *testing.T) {
			svc, rules, _ := newRuleService(t)
			rules.EXPECT().FindByID(mock.Anything, "r1").Return(storedRule(dto.NotificationEventOrderPaid, dto.NotificationConditions{}), nil).Once()
			_, err := svc.Update(notifCtx(), "r1", &tt.req)
			requireFieldError(t, err, tt.field)
		})
	}
	t.Run("repo fails", func(t *testing.T) {
		svc, rules, _ := newRuleService(t)
		rules.EXPECT().FindByID(mock.Anything, "r1").Return(storedRule(dto.NotificationEventOrderPaid, dto.NotificationConditions{}), nil).Once()
		rules.EXPECT().Update(mock.Anything, mock.Anything).Return(errors.New("boom")).Once()
		_, err := svc.Update(notifCtx(), "r1", &dto.UpdateNotificationRuleRequest{})
		assert.ErrorIs(t, err, domainError.ErrNotificationRuleOperation)
	})
}

func TestNotificationRuleService_Delete(t *testing.T) {
	t.Run("deletes", func(t *testing.T) {
		svc, rules, _ := newRuleService(t)
		rules.EXPECT().FindByID(mock.Anything, "r1").Return(storedRule(dto.NotificationEventOrderPaid, dto.NotificationConditions{}), nil).Once()
		rules.EXPECT().Delete(mock.Anything, "r1").Return(nil).Once()
		require.NoError(t, svc.Delete(notifCtx(), "r1"))
	})
	t.Run("not found", func(t *testing.T) {
		svc, rules, _ := newRuleService(t)
		rules.EXPECT().FindByID(mock.Anything, "r1").Return(nil, domainError.ErrNotificationRuleNotFound).Once()
		assert.ErrorIs(t, svc.Delete(notifCtx(), "r1"), domainError.ErrNotificationRuleNotFound)
	})
	t.Run("repo fails", func(t *testing.T) {
		svc, rules, _ := newRuleService(t)
		rules.EXPECT().FindByID(mock.Anything, "r1").Return(storedRule(dto.NotificationEventOrderPaid, dto.NotificationConditions{}), nil).Once()
		rules.EXPECT().Delete(mock.Anything, "r1").Return(errors.New("boom")).Once()
		assert.ErrorIs(t, svc.Delete(notifCtx(), "r1"), domainError.ErrNotificationRuleOperation)
	})
}

// A stored rule is rebuilt from its DTO and matched against the subject of a decoded event, as the consumer does.
func TestNotificationRule_Matches(t *testing.T) {
	order := func(t dto.OrderType) dto.NotificationSubject {
		return dto.NotificationSubject{Event: dto.NotificationEventOrderPaid, OrderType: &t}
	}
	reservation := func(ev dto.NotificationEventType, src dto.ReservationSource, by *dto.ConfirmedBy, size int) dto.NotificationSubject {
		return dto.NotificationSubject{Event: ev, Source: &src, ConfirmedBy: by, PartySize: &size}
	}
	auto, staff := dto.ConfirmedByAuto, dto.ConfirmedByStaff
	tests := []struct {
		name    string
		rule    *dto.NotificationRule
		subject dto.NotificationSubject
		want    bool
	}{
		{"delivery rule, takeaway order", storedRule(dto.NotificationEventOrderPaid, dto.NotificationConditions{OrderTypes: []dto.OrderType{dto.OrderTypeDelivery}}), order(dto.OrderTypeTakeaway), false},
		{"delivery rule, delivery order", storedRule(dto.NotificationEventOrderPaid, dto.NotificationConditions{OrderTypes: []dto.OrderType{dto.OrderTypeDelivery}}), order(dto.OrderTypeDelivery), true},
		{"no conditions", storedRule(dto.NotificationEventOrderPaid, dto.NotificationConditions{}), order(dto.OrderTypeDineIn), true},
		{"empty list matches any", storedRule(dto.NotificationEventOrderPaid, dto.NotificationConditions{OrderTypes: []dto.OrderType{}}), order(dto.OrderTypeDineIn), true},
		{"other event", storedRule(dto.NotificationEventOrderCreated, dto.NotificationConditions{}), order(dto.OrderTypeDineIn), false},
		{"condition without subject value", storedRule(dto.NotificationEventOrderPaid, dto.NotificationConditions{OrderTypes: []dto.OrderType{dto.OrderTypeDelivery}}), dto.NotificationSubject{Event: dto.NotificationEventOrderPaid}, false},
		{"auto confirmed matches auto", storedRule(dto.NotificationEventReservationConfirmed, dto.NotificationConditions{ConfirmedBy: []dto.ConfirmedBy{dto.ConfirmedByAuto}}), reservation(dto.NotificationEventReservationConfirmed, dto.ReservationSourceWhatsApp, &auto, 2), true},
		{"staff confirmed against auto rule", storedRule(dto.NotificationEventReservationConfirmed, dto.NotificationConditions{ConfirmedBy: []dto.ConfirmedBy{dto.ConfirmedByAuto}}), reservation(dto.NotificationEventReservationConfirmed, dto.ReservationSourcePhone, &staff, 2), false},
		{"confirmed_by without value", storedRule(dto.NotificationEventReservationConfirmed, dto.NotificationConditions{ConfirmedBy: []dto.ConfirmedBy{dto.ConfirmedByAuto}}), reservation(dto.NotificationEventReservationConfirmed, dto.ReservationSourcePhone, nil, 2), false},
		{"source in list", storedRule(dto.NotificationEventReservationRequested, dto.NotificationConditions{Sources: []dto.ReservationSource{dto.ReservationSourceWhatsApp, dto.ReservationSourcePhone}}), reservation(dto.NotificationEventReservationRequested, dto.ReservationSourcePhone, nil, 2), true},
		{"source not in list", storedRule(dto.NotificationEventReservationRequested, dto.NotificationConditions{Sources: []dto.ReservationSource{dto.ReservationSourceWhatsApp}}), reservation(dto.NotificationEventReservationRequested, dto.ReservationSourceStaff, nil, 2), false},
		{"source without value", storedRule(dto.NotificationEventReservationRequested, dto.NotificationConditions{Sources: []dto.ReservationSource{dto.ReservationSourceWhatsApp}}), dto.NotificationSubject{Event: dto.NotificationEventReservationRequested}, false},
		{"party of 10 against min 8", storedRule(dto.NotificationEventReservationRequested, dto.NotificationConditions{MinPartySize: ptr(8)}), reservation(dto.NotificationEventReservationRequested, dto.ReservationSourceWhatsApp, nil, 10), true},
		{"party of exactly min", storedRule(dto.NotificationEventReservationRequested, dto.NotificationConditions{MinPartySize: ptr(8)}), reservation(dto.NotificationEventReservationRequested, dto.ReservationSourceWhatsApp, nil, 8), true},
		{"party below min", storedRule(dto.NotificationEventReservationRequested, dto.NotificationConditions{MinPartySize: ptr(8)}), reservation(dto.NotificationEventReservationRequested, dto.ReservationSourceWhatsApp, nil, 7), false},
		{"min party without size", storedRule(dto.NotificationEventReservationRequested, dto.NotificationConditions{MinPartySize: ptr(8)}), dto.NotificationSubject{Event: dto.NotificationEventReservationRequested}, false},
		{"all conditions together", storedRule(dto.NotificationEventReservationConfirmed, dto.NotificationConditions{Sources: []dto.ReservationSource{dto.ReservationSourceWhatsApp}, ConfirmedBy: []dto.ConfirmedBy{dto.ConfirmedByAuto}, MinPartySize: ptr(4)}), reservation(dto.NotificationEventReservationConfirmed, dto.ReservationSourceWhatsApp, &auto, 4), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, rule.NewAggregateFromDTO(tt.rule).Matches(tt.subject))
		})
	}
}
