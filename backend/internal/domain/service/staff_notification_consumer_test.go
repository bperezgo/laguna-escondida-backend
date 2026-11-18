package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/domain/tenant"
)

type staffConsumerFixture struct {
	rules         *mocks.MockNotificationRuleRepository
	notifications *mocks.MockStaffNotificationRepository
	organizations *mocks.MockOrganizationRepository
	zones         *mocks.MockDeliveryZoneRepository
	consumer      *StaffNotificationConsumer
	ctx           context.Context
	now           time.Time
}

func newStaffConsumerFixture(t *testing.T) *staffConsumerFixture {
	f := &staffConsumerFixture{
		rules:         mocks.NewMockNotificationRuleRepository(t),
		notifications: mocks.NewMockStaffNotificationRepository(t),
		organizations: mocks.NewMockOrganizationRepository(t),
		zones:         mocks.NewMockDeliveryZoneRepository(t),
		ctx:           tenant.WithOrg(context.Background(), "org-1"),
		now:           crmAt(t, 8, 20, 0),
	}
	f.consumer = NewStaffNotificationConsumer(f.rules, f.notifications, f.organizations, f.zones, fixedClock(f.now))
	return f
}

func (f *staffConsumerFixture) org(t *testing.T) {
	f.organizations.EXPECT().Get(mock.Anything, "org-1").Return(&dto.Organization{ID: "org-1", Timezone: "America/Bogota", Currency: "COP"}, nil).Maybe()
}

func storedEvent(t *testing.T, name string, payload any) dto.StoredEvent {
	body, err := json.Marshal(payload)
	require.NoError(t, err)
	return dto.StoredEvent{ID: "evt-1", Name: name, OrganizationID: "org-1", Payload: body}
}

func paidDelivery(t *testing.T, at time.Time) dto.StoredEvent {
	name, phone, zone := "Ana Pérez", "3001234567", "zone-1"
	event := storedEvent(t, dto.DomainEventOrderPaid, dto.OrderPaid{
		DomainEventMeta: dto.DomainEventMeta{ID: "evt-1", Name: dto.DomainEventOrderPaid, OrganizationID: "org-1", Subject: "0190aaaa-bbbb-cccc-dddd-eeeeeeabc123", Time: at},
		OrderType:       dto.OrderTypeDelivery, Total: decimal.NewFromInt(58000), AmountPaid: decimal.NewFromInt(58000),
		CustomerName: &name, CustomerPhone: &phone, DeliveryZoneID: &zone,
	})
	event.OccurredAt = at
	return event
}

func ruleWith(id, channelID, prefix string, event dto.NotificationEventType, c dto.NotificationConditions, channelEnabled bool) *dto.RuleWithChannel {
	return &dto.RuleWithChannel{
		Rule:    dto.NotificationRule{ID: id, OrganizationID: "org-1", ChannelID: channelID, Event: event, Conditions: c, TitlePrefix: prefix, ExpiresAfterMinutes: 60, Enabled: true},
		Channel: dto.NotificationChannel{ID: channelID, Enabled: channelEnabled},
	}
}

func fieldValue(t *testing.T, m dto.StaffMessage, name string) string {
	for _, f := range m.Fields {
		if f.Name == name {
			return f.Value
		}
	}
	return "<missing>"
}

func TestStaffNotificationConsumer_Handle_TwoRulesTwoRows(t *testing.T) {
	f := newStaffConsumerFixture(t)
	f.org(t)
	at := crmAt(t, 8, 19, 30)
	f.rules.EXPECT().ListEnabledForEvent(mock.Anything, dto.NotificationEventOrderPaid).Return([]*dto.RuleWithChannel{
		ruleWith("r1", "c1", "<@&987>", dto.NotificationEventOrderPaid, dto.NotificationConditions{OrderTypes: []dto.OrderType{dto.OrderTypeDelivery}}, true),
		ruleWith("r2", "c2", "", dto.NotificationEventOrderPaid, dto.NotificationConditions{}, true),
	}, nil).Once()
	f.zones.EXPECT().FindByID(mock.Anything, "zone-1").Return(&dto.DeliveryZone{ID: "zone-1", Name: "Norte"}, nil).Once()
	var rows []*dto.StaffNotification
	f.notifications.EXPECT().Insert(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, n *dto.StaffNotification) (bool, error) {
		rows = append(rows, n)
		return true, nil
	}).Twice()

	require.NoError(t, f.consumer.Handle(f.ctx, paidDelivery(t, at)))

	require.Len(t, rows, 2)
	assert.Equal(t, "r1", rows[0].RuleID)
	assert.Equal(t, "c1", rows[0].ChannelID)
	assert.Equal(t, "evt-1", rows[0].EventID)
	assert.Equal(t, dto.StaffNotificationPending, rows[0].Status)
	assert.Equal(t, at.Add(60*time.Minute), rows[0].ExpiresAt)
	assert.Equal(t, "<@&987>", rows[0].Message.Prefix)
	assert.Equal(t, "", rows[1].Message.Prefix)
	assert.NotEqual(t, rows[0].ID, rows[1].ID)
	assert.Equal(t, "c2", rows[1].ChannelID)
}

func TestStaffNotificationConsumer_Handle_PaidDeliveryMessageFields(t *testing.T) {
	f := newStaffConsumerFixture(t)
	f.org(t)
	at := crmAt(t, 8, 19, 30)
	f.rules.EXPECT().ListEnabledForEvent(mock.Anything, dto.NotificationEventOrderPaid).Return([]*dto.RuleWithChannel{
		ruleWith("r1", "c1", "<@&987>", dto.NotificationEventOrderPaid, dto.NotificationConditions{}, true),
	}, nil).Once()
	f.zones.EXPECT().FindByID(mock.Anything, "zone-1").Return(&dto.DeliveryZone{Name: "Norte"}, nil).Once()
	var row *dto.StaffNotification
	f.notifications.EXPECT().Insert(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, n *dto.StaffNotification) (bool, error) {
		row = n
		return true, nil
	}).Once()

	require.NoError(t, f.consumer.Handle(f.ctx, paidDelivery(t, at)))

	m := row.Message
	assert.Equal(t, "<@&987>", m.Prefix)
	assert.Equal(t, "Pedido pagado", m.Title)
	assert.Equal(t, "abc123", fieldValue(t, m, "Pedido"))
	assert.Equal(t, "Domicilio", fieldValue(t, m, "Tipo"))
	assert.Equal(t, "$58.000 COP", fieldValue(t, m, "Total"))
	assert.Equal(t, "Ana Pérez", fieldValue(t, m, "Cliente"))
	assert.Equal(t, "3001234567", fieldValue(t, m, "Teléfono"))
	assert.Equal(t, "Norte", fieldValue(t, m, "Zona"))
	assert.Equal(t, "19:30", fieldValue(t, m, "Hora"))
}

func TestStaffNotificationConsumer_Handle_OrderWithoutCustomerOrZone(t *testing.T) {
	f := newStaffConsumerFixture(t)
	f.org(t)
	f.rules.EXPECT().ListEnabledForEvent(mock.Anything, dto.NotificationEventOrderCreated).Return([]*dto.RuleWithChannel{
		ruleWith("r1", "c1", "", dto.NotificationEventOrderCreated, dto.NotificationConditions{}, true),
	}, nil).Once()
	var row *dto.StaffNotification
	f.notifications.EXPECT().Insert(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, n *dto.StaffNotification) (bool, error) {
		row = n
		return true, nil
	}).Once()
	event := storedEvent(t, dto.DomainEventOrderCreated, dto.OrderCreated{
		DomainEventMeta: dto.DomainEventMeta{ID: "evt-1", Subject: "order-xyz789", Time: crmAt(t, 8, 12, 5)},
		OrderType:       dto.OrderTypeDineIn, Total: decimal.NewFromInt(12500),
	})

	require.NoError(t, f.consumer.Handle(f.ctx, event))

	assert.Equal(t, "Nuevo pedido", row.Message.Title)
	assert.Equal(t, "xyz789", fieldValue(t, row.Message, "Pedido"))
	assert.Equal(t, "En mesa", fieldValue(t, row.Message, "Tipo"))
	assert.Equal(t, "<missing>", fieldValue(t, row.Message, "Cliente"))
	assert.Equal(t, "<missing>", fieldValue(t, row.Message, "Zona"))
}

func TestStaffNotificationConsumer_Handle_ReservationOverPartyLimit(t *testing.T) {
	f := newStaffConsumerFixture(t)
	f.org(t)
	min := 8
	f.rules.EXPECT().ListEnabledForEvent(mock.Anything, dto.NotificationEventReservationRequested).Return([]*dto.RuleWithChannel{
		ruleWith("r1", "c1", "", dto.NotificationEventReservationRequested, dto.NotificationConditions{MinPartySize: &min}, true),
	}, nil).Once()
	var row *dto.StaffNotification
	f.notifications.EXPECT().Insert(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, n *dto.StaffNotification) (bool, error) {
		row = n
		return true, nil
	}).Once()
	reason := dto.PendingReasonPartySize
	event := storedEvent(t, dto.DomainEventReservationRequested, dto.ReservationRequested{
		DomainEventMeta: dto.DomainEventMeta{ID: "evt-1", Subject: "res-1", Time: crmAt(t, 8, 15, 0)},
		GuestName:       "Luis", GuestPhone: "300", PartySize: 10, StartsAt: crmAt(t, 14, 20, 30),
		Source: dto.ReservationSourceWhatsApp, PendingReason: &reason,
	})

	require.NoError(t, f.consumer.Handle(f.ctx, event))

	m := row.Message
	assert.Equal(t, "Reserva por confirmar", m.Title)
	assert.Equal(t, "Luis", fieldValue(t, m, "Huésped"))
	assert.Equal(t, "10", fieldValue(t, m, "Personas"))
	assert.Equal(t, "14/11/2026 20:30", fieldValue(t, m, "Fecha y hora"))
	assert.Equal(t, "WhatsApp", fieldValue(t, m, "Origen"))
	assert.Equal(t, "El grupo supera el límite de personas", fieldValue(t, m, "Motivo"))
}

func TestStaffNotificationConsumer_Handle_ConfirmedReservationHasNoReason(t *testing.T) {
	f := newStaffConsumerFixture(t)
	f.org(t)
	f.rules.EXPECT().ListEnabledForEvent(mock.Anything, dto.NotificationEventReservationConfirmed).Return([]*dto.RuleWithChannel{
		ruleWith("r1", "c1", "", dto.NotificationEventReservationConfirmed, dto.NotificationConditions{ConfirmedBy: []dto.ConfirmedBy{dto.ConfirmedByAuto}}, true),
		ruleWith("r2", "c1", "", dto.NotificationEventReservationConfirmed, dto.NotificationConditions{ConfirmedBy: []dto.ConfirmedBy{dto.ConfirmedByStaff}}, true),
	}, nil).Once()
	var rows []*dto.StaffNotification
	f.notifications.EXPECT().Insert(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, n *dto.StaffNotification) (bool, error) {
		rows = append(rows, n)
		return true, nil
	}).Once()
	event := storedEvent(t, dto.DomainEventReservationConfirmed, dto.ReservationConfirmed{
		DomainEventMeta: dto.DomainEventMeta{ID: "evt-1", Subject: "res-1", Time: crmAt(t, 8, 15, 0)},
		GuestName:       "Luis", PartySize: 4, StartsAt: crmAt(t, 9, 19, 0), Source: dto.ReservationSourceWhatsApp, ConfirmedBy: dto.ConfirmedByAuto,
	})

	require.NoError(t, f.consumer.Handle(f.ctx, event))

	require.Len(t, rows, 1)
	assert.Equal(t, "r1", rows[0].RuleID)
	assert.Equal(t, "Reserva confirmada", rows[0].Message.Title)
	assert.Equal(t, "<missing>", fieldValue(t, rows[0].Message, "Motivo"))
}

func TestStaffNotificationConsumer_Handle_NoMatchRecordsNothing(t *testing.T) {
	f := newStaffConsumerFixture(t)
	f.rules.EXPECT().ListEnabledForEvent(mock.Anything, dto.NotificationEventOrderPaid).Return([]*dto.RuleWithChannel{
		ruleWith("r1", "c1", "", dto.NotificationEventOrderPaid, dto.NotificationConditions{OrderTypes: []dto.OrderType{dto.OrderTypeTakeaway}}, true),
	}, nil).Once()

	require.NoError(t, f.consumer.Handle(f.ctx, paidDelivery(t, f.now)))
}

func TestStaffNotificationConsumer_Handle_NoRulesRecordsNothing(t *testing.T) {
	f := newStaffConsumerFixture(t)
	f.rules.EXPECT().ListEnabledForEvent(mock.Anything, dto.NotificationEventOrderPaid).Return(nil, nil).Once()

	require.NoError(t, f.consumer.Handle(f.ctx, paidDelivery(t, f.now)))
}

func TestStaffNotificationConsumer_Handle_DeliveredTwiceOneRowPerRule(t *testing.T) {
	f := newStaffConsumerFixture(t)
	f.org(t)
	f.rules.EXPECT().ListEnabledForEvent(mock.Anything, dto.NotificationEventOrderPaid).Return([]*dto.RuleWithChannel{
		ruleWith("r1", "c1", "", dto.NotificationEventOrderPaid, dto.NotificationConditions{}, true),
	}, nil).Twice()
	f.zones.EXPECT().FindByID(mock.Anything, "zone-1").Return(&dto.DeliveryZone{Name: "Norte"}, nil).Twice()
	seen := map[string]bool{}
	stored := 0
	f.notifications.EXPECT().Insert(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, n *dto.StaffNotification) (bool, error) {
		key := n.RuleID + "/" + n.EventID
		if seen[key] {
			return false, nil
		}
		seen[key] = true
		stored++
		return true, nil
	}).Twice()
	event := paidDelivery(t, f.now)

	require.NoError(t, f.consumer.Handle(f.ctx, event))
	require.NoError(t, f.consumer.Handle(f.ctx, event))

	assert.Equal(t, 1, stored)
}

func TestStaffNotificationConsumer_Handle_Errors(t *testing.T) {
	boom := errors.New("boom")
	tests := []struct {
		name  string
		event func(t *testing.T, f *staffConsumerFixture) dto.StoredEvent
	}{
		{"rules fail", func(t *testing.T, f *staffConsumerFixture) dto.StoredEvent {
			f.rules.EXPECT().ListEnabledForEvent(mock.Anything, mock.Anything).Return(nil, boom).Once()
			return paidDelivery(t, f.now)
		}},
		{"organization fails", func(t *testing.T, f *staffConsumerFixture) dto.StoredEvent {
			f.rules.EXPECT().ListEnabledForEvent(mock.Anything, mock.Anything).Return([]*dto.RuleWithChannel{
				ruleWith("r1", "c1", "", dto.NotificationEventOrderPaid, dto.NotificationConditions{}, true)}, nil).Once()
			f.organizations.EXPECT().Get(mock.Anything, "org-1").Return(nil, boom).Once()
			return paidDelivery(t, f.now)
		}},
		{"zone lookup fails", func(t *testing.T, f *staffConsumerFixture) dto.StoredEvent {
			f.org(t)
			f.rules.EXPECT().ListEnabledForEvent(mock.Anything, mock.Anything).Return([]*dto.RuleWithChannel{
				ruleWith("r1", "c1", "", dto.NotificationEventOrderPaid, dto.NotificationConditions{}, true)}, nil).Once()
			f.zones.EXPECT().FindByID(mock.Anything, "zone-1").Return(nil, boom).Once()
			return paidDelivery(t, f.now)
		}},
		{"insert fails", func(t *testing.T, f *staffConsumerFixture) dto.StoredEvent {
			f.org(t)
			f.rules.EXPECT().ListEnabledForEvent(mock.Anything, mock.Anything).Return([]*dto.RuleWithChannel{
				ruleWith("r1", "c1", "", dto.NotificationEventOrderPaid, dto.NotificationConditions{}, true)}, nil).Once()
			f.zones.EXPECT().FindByID(mock.Anything, "zone-1").Return(&dto.DeliveryZone{Name: "N"}, nil).Once()
			f.notifications.EXPECT().Insert(mock.Anything, mock.Anything).Return(false, boom).Once()
			return paidDelivery(t, f.now)
		}},
		{"malformed payload", func(t *testing.T, f *staffConsumerFixture) dto.StoredEvent {
			return dto.StoredEvent{ID: "e", Name: dto.DomainEventOrderPaid, OrganizationID: "org-1", Payload: []byte("{")}
		}},
		{"unknown event", func(t *testing.T, f *staffConsumerFixture) dto.StoredEvent {
			return dto.StoredEvent{ID: "e", Name: "other.event", OrganizationID: "org-1"}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newStaffConsumerFixture(t)
			event := tt.event(t, f)
			assert.Error(t, f.consumer.Handle(f.ctx, event))
		})
	}
}

func TestStaffNotificationConsumer_Handle_MissingZoneOmitsField(t *testing.T) {
	f := newStaffConsumerFixture(t)
	f.org(t)
	f.rules.EXPECT().ListEnabledForEvent(mock.Anything, mock.Anything).Return([]*dto.RuleWithChannel{
		ruleWith("r1", "c1", "", dto.NotificationEventOrderPaid, dto.NotificationConditions{}, true)}, nil).Once()
	f.zones.EXPECT().FindByID(mock.Anything, "zone-1").Return(nil, domainError.ErrDeliveryZoneNotFound).Once()
	var row *dto.StaffNotification
	f.notifications.EXPECT().Insert(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, n *dto.StaffNotification) (bool, error) {
		row = n
		return true, nil
	}).Once()

	require.NoError(t, f.consumer.Handle(f.ctx, paidDelivery(t, f.now)))

	assert.Equal(t, "<missing>", fieldValue(t, row.Message, "Zona"))
}

func TestStaffNotificationConsumer_NameAndHandles(t *testing.T) {
	c := NewStaffNotificationConsumer(nil, nil, nil, nil, nil)
	assert.Equal(t, "staff-notifications", c.Name())
	assert.ElementsMatch(t, []string{"order.created", "order.paid", "reservation.requested", "reservation.confirmed"}, c.Handles())
}

func TestFormatMoney(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"58000", "$58.000 COP"},
		{"999", "$999 COP"},
		{"1234567", "$1.234.567 COP"},
		{"1500.5", "$1.500,50 COP"},
		{"0", "$0 COP"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			assert.Equal(t, tt.want, formatMoney(decimal.RequireFromString(tt.in), "COP"))
		})
	}
}
