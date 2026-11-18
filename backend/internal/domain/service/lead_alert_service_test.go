package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const leadWebhook = "https://discord.com/api/webhooks/1/x"

type leadAlertFixture struct {
	repo    *mocks.MockLeadRepository
	sender  *mocks.MockStaffNotificationSender
	service *LeadAlertService
	now     time.Time
}

func newLeadAlertFixture(t *testing.T) *leadAlertFixture {
	f := &leadAlertFixture{
		repo:   mocks.NewMockLeadRepository(t),
		sender: mocks.NewMockStaffNotificationSender(t),
		now:    time.Date(2026, 11, 18, 15, 0, 0, 0, time.UTC),
	}
	f.service = NewLeadAlertService(f.repo, f.sender, leadWebhook, discardLogger(), fixedClock(f.now))
	return f
}

func (f *leadAlertFixture) claim(leads ...*dto.Lead) {
	f.repo.EXPECT().ClaimDueAlerts(mock.Anything, f.now, f.now.Add(2*time.Minute), 50).Return(leads, nil).Once()
}

func (f *leadAlertFixture) lead(attempts int, age time.Duration) *dto.Lead {
	return &dto.Lead{
		ID: "l1", Name: "Ana Ruiz", RestaurantName: "El Fogón", Phone: "+573001234567",
		LeadAttribution: dto.LeadAttribution{UTMSource: "facebook", UTMCampaign: "lanzamiento"},
		AlertStatus:     dto.LeadAlertPending, AlertAttempts: attempts, CreatedAt: f.now.Add(-age),
	}
}

func TestLeadAlertService_DeliverDue_SendsTheLeadAndMarksItSent(t *testing.T) {
	f := newLeadAlertFixture(t)
	f.claim(f.lead(1, time.Minute))
	f.sender.EXPECT().Send(mock.Anything, mock.MatchedBy(func(s dto.ChannelSecret) bool {
		return s.Type == dto.NotificationChannelDiscord && s.Settings.Discord != nil && s.Settings.Discord.WebhookURL == leadWebhook
	}), mock.MatchedBy(func(m dto.StaffMessage) bool {
		values := map[string]string{}
		for _, field := range m.Fields {
			values[field.Name] = field.Value
		}
		return values["Nombre"] == "Ana Ruiz" && values["Restaurante"] == "El Fogón" && values["Teléfono"] == "+573001234567" &&
			values["utm_source"] == "facebook" && values["utm_campaign"] == "lanzamiento"
	})).Return(dto.SendOutcome{Status: dto.SendSent, StatusCode: 204}, nil).Once()
	f.repo.EXPECT().MarkAlert(mock.Anything, "l1", dto.LeadAlertSent, time.Time{}, "").Return(nil).Once()

	sent, err := f.service.DeliverDue(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 1, sent)
}

func TestLeadAlertService_DeliverDue_EmptyAttributionShowsADash(t *testing.T) {
	f := newLeadAlertFixture(t)
	lead := f.lead(1, time.Minute)
	lead.LeadAttribution = dto.LeadAttribution{}
	f.claim(lead)
	f.sender.EXPECT().Send(mock.Anything, mock.Anything, mock.MatchedBy(func(m dto.StaffMessage) bool {
		for _, field := range m.Fields {
			if field.Value == "" {
				return false
			}
		}
		return true
	})).Return(dto.SendOutcome{Status: dto.SendSent}, nil).Once()
	f.repo.EXPECT().MarkAlert(mock.Anything, "l1", dto.LeadAlertSent, time.Time{}, "").Return(nil).Once()

	_, err := f.service.DeliverDue(context.Background())

	require.NoError(t, err)
}

func TestLeadAlertService_DeliverDue_ServiceUnavailableIsRetriedWithBackoff(t *testing.T) {
	tests := []struct {
		attempts int
		delay    time.Duration
	}{
		{1, 15 * time.Second},
		{2, 30 * time.Second},
		{3, time.Minute},
		{6, 8 * time.Minute},
		{8, 30 * time.Minute},
		{20, 30 * time.Minute},
	}
	for _, tt := range tests {
		f := newLeadAlertFixture(t)
		f.claim(f.lead(tt.attempts, time.Minute))
		f.sender.EXPECT().Send(mock.Anything, mock.Anything, mock.Anything).
			Return(dto.SendOutcome{Status: dto.SendRetry, StatusCode: 503}, nil).Once()
		want := tt.delay
		f.repo.EXPECT().MarkAlert(mock.Anything, "l1", dto.LeadAlertPending, f.now.Add(want), "HTTP 503").Return(nil).Once()

		sent, err := f.service.DeliverDue(context.Background())

		require.NoError(t, err)
		assert.Zero(t, sent)
	}
}

func TestLeadAlertService_DeliverDue_RetryAfterWinsWhenLonger(t *testing.T) {
	f := newLeadAlertFixture(t)
	f.claim(f.lead(1, time.Minute))
	f.sender.EXPECT().Send(mock.Anything, mock.Anything, mock.Anything).
		Return(dto.SendOutcome{Status: dto.SendRetry, StatusCode: 429, RetryAfter: 5 * time.Minute}, nil).Once()
	f.repo.EXPECT().MarkAlert(mock.Anything, "l1", dto.LeadAlertPending, f.now.Add(5*time.Minute), "HTTP 429").Return(nil).Once()

	_, err := f.service.DeliverDue(context.Background())

	require.NoError(t, err)
}

func TestLeadAlertService_DeliverDue_TransportErrorIsRetried(t *testing.T) {
	f := newLeadAlertFixture(t)
	f.claim(f.lead(1, time.Minute))
	f.sender.EXPECT().Send(mock.Anything, mock.Anything, mock.Anything).
		Return(dto.SendOutcome{Status: dto.SendRetry}, errors.New("connection reset")).Once()
	f.repo.EXPECT().MarkAlert(mock.Anything, "l1", dto.LeadAlertPending, f.now.Add(15*time.Second), "connection reset").Return(nil).Once()

	_, err := f.service.DeliverDue(context.Background())

	require.NoError(t, err)
}

func TestLeadAlertService_DeliverDue_FailsAfter24HoursWithoutSending(t *testing.T) {
	f := newLeadAlertFixture(t)
	f.claim(f.lead(40, 24*time.Hour+time.Minute))
	f.repo.EXPECT().MarkAlert(mock.Anything, "l1", dto.LeadAlertFailed, time.Time{}, "not sent within 24 hours").Return(nil).Once()

	sent, err := f.service.DeliverDue(context.Background())

	require.NoError(t, err)
	assert.Zero(t, sent)
}

func TestLeadAlertService_DeliverDue_FailsWhenTheNextTryWouldPassTheDeadline(t *testing.T) {
	f := newLeadAlertFixture(t)
	f.claim(f.lead(40, 24*time.Hour-10*time.Minute))
	f.sender.EXPECT().Send(mock.Anything, mock.Anything, mock.Anything).
		Return(dto.SendOutcome{Status: dto.SendRetry, StatusCode: 503}, nil).Once()
	f.repo.EXPECT().MarkAlert(mock.Anything, "l1", dto.LeadAlertFailed, time.Time{}, "HTTP 503").Return(nil).Once()

	_, err := f.service.DeliverDue(context.Background())

	require.NoError(t, err)
}

func TestLeadAlertService_DeliverDue_NothingDue(t *testing.T) {
	f := newLeadAlertFixture(t)
	f.claim()

	sent, err := f.service.DeliverDue(context.Background())

	require.NoError(t, err)
	assert.Zero(t, sent)
}

func TestLeadAlertService_DeliverDue_ClaimErrorIsReturned(t *testing.T) {
	f := newLeadAlertFixture(t)
	f.repo.EXPECT().ClaimDueAlerts(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, assert.AnError).Once()

	_, err := f.service.DeliverDue(context.Background())

	assert.ErrorIs(t, err, assert.AnError)
}
