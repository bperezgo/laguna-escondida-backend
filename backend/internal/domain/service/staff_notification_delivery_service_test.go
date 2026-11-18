package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/domain/tenant"
)

type staffDeliveryFixture struct {
	repo    *mocks.MockStaffNotificationRepository
	senders *mocks.MockStaffNotificationSenders
	sender  *mocks.MockStaffNotificationSender
	cipher  *mocks.MockSecretCipher
	service *StaffNotificationDeliveryService
	now     time.Time
	ctx     context.Context
}

func newStaffDeliveryFixture(t *testing.T) *staffDeliveryFixture {
	f := &staffDeliveryFixture{
		repo:    mocks.NewMockStaffNotificationRepository(t),
		senders: mocks.NewMockStaffNotificationSenders(t),
		sender:  mocks.NewMockStaffNotificationSender(t),
		cipher:  mocks.NewMockSecretCipher(t),
		now:     crmAt(t, 8, 20, 0),
		ctx:     tenant.WithOrg(context.Background(), "org-1"),
	}
	f.service = NewStaffNotificationDeliveryService(f.repo, f.senders, f.cipher, discardLogger(), fixedClock(f.now))
	return f
}

func (f *staffDeliveryFixture) claim(c ...*dto.ClaimedStaffNotification) {
	f.repo.EXPECT().ClaimDue(mock.Anything, f.now, f.now.Add(2*time.Minute), 50).Return(c, nil).Once()
}

func (f *staffDeliveryFixture) claimed(attempts int, live bool, expiresIn time.Duration) *dto.ClaimedStaffNotification {
	return &dto.ClaimedStaffNotification{
		Notification: &dto.StaffNotification{ID: "n1", OrganizationID: "org-1", RuleID: "r1", ChannelID: "c1", EventID: "e1",
			Message: dto.StaffMessage{Title: "Pedido pagado"}, Attempts: attempts, ExpiresAt: f.now.Add(expiresIn)},
		Live: live, ChannelType: dto.NotificationChannelDiscord, Secret: []byte("sealed"),
	}
}

func (f *staffDeliveryFixture) sends(outcome dto.SendOutcome, err error) {
	f.cipher.EXPECT().Decrypt([]byte("sealed")).Return([]byte(`{"discord":{"webhook_url":"https://discord.com/api/webhooks/1/x"}}`), nil).Once()
	f.senders.EXPECT().For(dto.NotificationChannelDiscord).Return(f.sender, true).Once()
	f.sender.EXPECT().Send(mock.Anything, mock.MatchedBy(func(s dto.ChannelSecret) bool {
		return s.Type == dto.NotificationChannelDiscord && s.Settings.Discord != nil && s.Settings.Discord.WebhookURL == "https://discord.com/api/webhooks/1/x"
	}), dto.StaffMessage{Title: "Pedido pagado"}).Return(outcome, err).Once()
}

func TestStaffNotificationDeliveryService_DeliverDue_SentOn2xx(t *testing.T) {
	f := newStaffDeliveryFixture(t)
	f.claim(f.claimed(1, true, time.Hour))
	f.sends(dto.SendOutcome{Status: dto.SendSent, StatusCode: 204}, nil)
	f.repo.EXPECT().MarkSent(mock.Anything, "n1", f.now).Return(nil).Once()

	sent, err := f.service.DeliverDue(f.ctx)

	require.NoError(t, err)
	assert.Equal(t, 1, sent)
}

func TestStaffNotificationDeliveryService_DeliverDue_RejectedStatuses(t *testing.T) {
	for _, code := range []int{401, 403, 404} {
		t.Run(string(rune('0'+code/100))+"xx", func(t *testing.T) {
			f := newStaffDeliveryFixture(t)
			f.claim(f.claimed(1, true, time.Hour))
			f.sends(dto.SendOutcome{Status: dto.SendRejected, StatusCode: code}, nil)
			f.repo.EXPECT().MarkFailed(mock.Anything, "n1", dto.StaffNotificationRejected, f.now, mock.Anything).Return(nil).Once()

			sent, err := f.service.DeliverDue(f.ctx)

			require.NoError(t, err)
			assert.Zero(t, sent)
		})
	}
}

func TestStaffNotificationDeliveryService_DeliverDue_RetryBackoff(t *testing.T) {
	tests := []struct {
		attempts   int
		retryAfter time.Duration
		want       time.Duration
	}{
		{1, 0, 10 * time.Second},
		{2, 0, 20 * time.Second},
		{3, 0, 40 * time.Second},
		{6, 0, 5 * time.Minute},
		{20, 0, 5 * time.Minute},
		{1, 45 * time.Second, 45 * time.Second},
		{3, 5 * time.Second, 40 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.want.String(), func(t *testing.T) {
			f := newStaffDeliveryFixture(t)
			f.claim(f.claimed(tt.attempts, true, 24*time.Hour))
			f.sends(dto.SendOutcome{Status: dto.SendRetry, StatusCode: 503, RetryAfter: tt.retryAfter}, nil)
			f.repo.EXPECT().MarkFailed(mock.Anything, "n1", dto.StaffNotificationPending, f.now.Add(tt.want), "HTTP 503").Return(nil).Once()

			_, err := f.service.DeliverDue(f.ctx)

			require.NoError(t, err)
		})
	}
}

func TestStaffNotificationDeliveryService_DeliverDue_TransportErrorRetries(t *testing.T) {
	f := newStaffDeliveryFixture(t)
	f.claim(f.claimed(1, true, time.Hour))
	f.sends(dto.SendOutcome{}, errors.New("connection reset"))
	f.repo.EXPECT().MarkFailed(mock.Anything, "n1", dto.StaffNotificationPending, f.now.Add(10*time.Second), "connection reset").Return(nil).Once()

	_, err := f.service.DeliverDue(f.ctx)

	require.NoError(t, err)
}

func TestStaffNotificationDeliveryService_DeliverDue_DroppedWhenNotLive(t *testing.T) {
	f := newStaffDeliveryFixture(t)
	f.claim(f.claimed(1, false, time.Hour))
	f.repo.EXPECT().MarkFailed(mock.Anything, "n1", dto.StaffNotificationDropped, f.now, mock.Anything).Return(nil).Once()

	sent, err := f.service.DeliverDue(f.ctx)

	require.NoError(t, err)
	assert.Zero(t, sent)
}

func TestStaffNotificationDeliveryService_DeliverDue_Expiry(t *testing.T) {
	tests := []struct {
		name      string
		expiresIn time.Duration
		expired   bool
	}{
		{"past expiry", -time.Minute, true},
		{"exactly at expiry", 0, true},
		{"before expiry", time.Second, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newStaffDeliveryFixture(t)
			f.claim(f.claimed(1, true, tt.expiresIn))
			if tt.expired {
				f.repo.EXPECT().MarkFailed(mock.Anything, "n1", dto.StaffNotificationExpired, f.now, mock.Anything).Return(nil).Once()
			} else {
				f.sends(dto.SendOutcome{Status: dto.SendSent, StatusCode: 204}, nil)
				f.repo.EXPECT().MarkSent(mock.Anything, "n1", f.now).Return(nil).Once()
			}

			_, err := f.service.DeliverDue(f.ctx)

			require.NoError(t, err)
		})
	}
}

func TestStaffNotificationDeliveryService_DeliverDue_UnusableSecretIsRejected(t *testing.T) {
	f := newStaffDeliveryFixture(t)
	f.claim(f.claimed(1, true, time.Hour))
	f.cipher.EXPECT().Decrypt([]byte("sealed")).Return(nil, errors.New("wrong key")).Once()
	f.repo.EXPECT().MarkFailed(mock.Anything, "n1", dto.StaffNotificationRejected, f.now, mock.Anything).Return(nil).Once()

	_, err := f.service.DeliverDue(f.ctx)

	require.NoError(t, err)
}

func TestStaffNotificationDeliveryService_DeliverDue_UndecodableSecretIsRejected(t *testing.T) {
	f := newStaffDeliveryFixture(t)
	f.claim(f.claimed(1, true, time.Hour))
	f.cipher.EXPECT().Decrypt([]byte("sealed")).Return([]byte("not json"), nil).Once()
	f.repo.EXPECT().MarkFailed(mock.Anything, "n1", dto.StaffNotificationRejected, f.now, mock.Anything).Return(nil).Once()

	_, err := f.service.DeliverDue(f.ctx)

	require.NoError(t, err)
}

func TestStaffNotificationDeliveryService_DeliverDue_NoSenderIsRejected(t *testing.T) {
	f := newStaffDeliveryFixture(t)
	f.claim(f.claimed(1, true, time.Hour))
	f.cipher.EXPECT().Decrypt([]byte("sealed")).Return([]byte(`{}`), nil).Once()
	f.senders.EXPECT().For(dto.NotificationChannelDiscord).Return(nil, false).Once()
	f.repo.EXPECT().MarkFailed(mock.Anything, "n1", dto.StaffNotificationRejected, f.now, "no sender for channel type").Return(nil).Once()

	_, err := f.service.DeliverDue(f.ctx)

	require.NoError(t, err)
}

func TestStaffNotificationDeliveryService_DeliverDue_ClaimError(t *testing.T) {
	f := newStaffDeliveryFixture(t)
	f.repo.EXPECT().ClaimDue(mock.Anything, f.now, f.now.Add(2*time.Minute), 50).Return(nil, errors.New("db down")).Once()

	_, err := f.service.DeliverDue(f.ctx)

	assert.Error(t, err)
}

func TestStaffNotificationDeliveryService_DeliverDue_RequiresOrganization(t *testing.T) {
	f := newStaffDeliveryFixture(t)

	_, err := f.service.DeliverDue(context.Background())

	assert.Error(t, err)
}

func TestStaffNotificationDeliveryService_DeliverDue_StoreErrorStopsBatch(t *testing.T) {
	f := newStaffDeliveryFixture(t)
	f.claim(f.claimed(1, false, time.Hour))
	f.repo.EXPECT().MarkFailed(mock.Anything, "n1", dto.StaffNotificationDropped, f.now, mock.Anything).Return(errors.New("db down")).Once()

	_, err := f.service.DeliverDue(f.ctx)

	assert.Error(t, err)
}
