package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/domain/tenant"
	baseError "laguna-escondida/backend/internal/platform/shared/domain/error"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	notifOrgID     = "org-a"
	validWebhook   = "https://discord.com/api/webhooks/123/abcd1234"
	sealedByCipher = "sealed"
)

type channelFixture struct {
	repo    *mocks.MockNotificationChannelRepository
	cipher  *mocks.MockSecretCipher
	senders *mocks.MockStaffNotificationSenders
	sender  *mocks.MockStaffNotificationSender
	svc     *NotificationChannelService
}

func newChannelFixture(t *testing.T) *channelFixture {
	f := &channelFixture{
		repo:    mocks.NewMockNotificationChannelRepository(t),
		cipher:  mocks.NewMockSecretCipher(t),
		senders: mocks.NewMockStaffNotificationSenders(t),
		sender:  mocks.NewMockStaffNotificationSender(t),
	}
	f.svc = NewNotificationChannelService(f.repo, f.cipher, f.senders)
	return f
}

func notifCtx() context.Context { return tenant.WithOrg(context.Background(), notifOrgID) }

func requireFieldError(t *testing.T, err error, field string) {
	t.Helper()
	var baseErr *baseError.BaseError
	require.True(t, errors.As(err, &baseErr), "expected a validation error, got %v", err)
	assert.Equal(t, field, baseErr.GetFieldValue())
}

func discordCreateRequest(url string) *dto.CreateNotificationChannelRequest {
	return &dto.CreateNotificationChannelRequest{
		Name: "Domicilios", Type: dto.NotificationChannelDiscord,
		Settings: dto.ChannelSettingsInput{WebhookURL: &url},
	}
}

func storedChannel(id, name string) *dto.NotificationChannel {
	return &dto.NotificationChannel{
		ID: id, OrganizationID: notifOrgID, Name: name, Type: dto.NotificationChannelDiscord,
		Enabled: true, Secret: []byte(sealedByCipher), SecretHint: "…1234",
	}
}

func TestNotificationChannelService_Create_EncryptsAndHidesSecret(t *testing.T) {
	f := newChannelFixture(t)
	f.repo.EXPECT().List(mock.Anything).Return(nil, nil).Once()
	f.cipher.EXPECT().Encrypt(mock.MatchedBy(func(plain []byte) bool {
		var s dto.ChannelSettings
		return json.Unmarshal(plain, &s) == nil && s.Discord != nil && s.Discord.WebhookURL == validWebhook
	})).Return([]byte(sealedByCipher), nil).Once()
	f.repo.EXPECT().Create(mock.Anything, mock.MatchedBy(func(c *dto.NotificationChannel) bool {
		return c.OrganizationID == notifOrgID && c.ID != "" && c.Enabled &&
			string(c.Secret) == sealedByCipher && c.SecretHint == "…1234"
	})).Return(nil).Once()

	view, err := f.svc.Create(notifCtx(), discordCreateRequest(validWebhook))

	require.NoError(t, err)
	assert.Equal(t, "Domicilios", view.Name)
	assert.True(t, view.Enabled)
	assert.Equal(t, "…1234", view.Settings.WebhookURLHint)
	raw, _ := json.Marshal(view)
	assert.NotContains(t, string(raw), "webhook_url\"")
	assert.NotContains(t, string(raw), "abcd1234")
}

func TestNotificationChannelService_Create_Validation(t *testing.T) {
	long := make([]byte, 81)
	for i := range long {
		long[i] = 'a'
	}
	tests := []struct {
		name  string
		req   *dto.CreateNotificationChannelRequest
		field string
	}{
		{"webhook outside discord", discordCreateRequest("https://example.com/hook"), "settings.webhook_url"},
		{"http scheme", discordCreateRequest("http://discord.com/api/webhooks/1/a"), "settings.webhook_url"},
		{"wrong path", discordCreateRequest("https://discord.com/other/1/a"), "settings.webhook_url"},
		{"lookalike host", discordCreateRequest("https://discord.com.evil.io/api/webhooks/1/a"), "settings.webhook_url"},
		{"missing webhook", &dto.CreateNotificationChannelRequest{Name: "x", Type: dto.NotificationChannelDiscord}, "settings.webhook_url"},
		{"unknown type", &dto.CreateNotificationChannelRequest{Name: "x", Type: "slack"}, "type"},
		{"empty name", &dto.CreateNotificationChannelRequest{Name: "  ", Type: dto.NotificationChannelDiscord}, "name"},
		{"name too long", &dto.CreateNotificationChannelRequest{Name: string(long), Type: dto.NotificationChannelDiscord}, "name"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newChannelFixture(t)
			_, err := f.svc.Create(notifCtx(), tt.req)
			requireFieldError(t, err, tt.field)
		})
	}
}

func TestNotificationChannelService_Create_AcceptsDiscordappHost(t *testing.T) {
	f := newChannelFixture(t)
	f.repo.EXPECT().List(mock.Anything).Return(nil, nil).Once()
	f.cipher.EXPECT().Encrypt(mock.Anything).Return([]byte(sealedByCipher), nil).Once()
	f.repo.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Once()

	_, err := f.svc.Create(notifCtx(), discordCreateRequest("https://discordapp.com/api/webhooks/1/zzzz"))

	require.NoError(t, err)
}

func TestNotificationChannelService_Create_DisabledFlag(t *testing.T) {
	f := newChannelFixture(t)
	f.repo.EXPECT().List(mock.Anything).Return(nil, nil).Once()
	f.cipher.EXPECT().Encrypt(mock.Anything).Return([]byte(sealedByCipher), nil).Once()
	f.repo.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Once()
	req := discordCreateRequest(validWebhook)
	off := false
	req.Enabled = &off

	view, err := f.svc.Create(notifCtx(), req)

	require.NoError(t, err)
	assert.False(t, view.Enabled)
}

func TestNotificationChannelService_Create_DuplicateName(t *testing.T) {
	f := newChannelFixture(t)
	f.repo.EXPECT().List(mock.Anything).Return([]*dto.NotificationChannel{storedChannel("c1", "Domicilios")}, nil).Once()

	_, err := f.svc.Create(notifCtx(), discordCreateRequest(validWebhook))

	requireFieldError(t, err, "name")
}

func TestNotificationChannelService_Create_NoOrganization(t *testing.T) {
	f := newChannelFixture(t)

	_, err := f.svc.Create(context.Background(), discordCreateRequest(validWebhook))

	require.Error(t, err)
}

func TestNotificationChannelService_Create_Failures(t *testing.T) {
	boom := errors.New("boom")
	t.Run("list fails", func(t *testing.T) {
		f := newChannelFixture(t)
		f.repo.EXPECT().List(mock.Anything).Return(nil, boom).Once()
		_, err := f.svc.Create(notifCtx(), discordCreateRequest(validWebhook))
		assert.ErrorIs(t, err, domainError.ErrNotificationChannelOperation)
	})
	t.Run("encrypt fails", func(t *testing.T) {
		f := newChannelFixture(t)
		f.repo.EXPECT().List(mock.Anything).Return(nil, nil).Once()
		f.cipher.EXPECT().Encrypt(mock.Anything).Return(nil, boom).Once()
		_, err := f.svc.Create(notifCtx(), discordCreateRequest(validWebhook))
		assert.ErrorIs(t, err, domainError.ErrNotificationChannelOperation)
	})
	t.Run("repo fails", func(t *testing.T) {
		f := newChannelFixture(t)
		f.repo.EXPECT().List(mock.Anything).Return(nil, nil).Once()
		f.cipher.EXPECT().Encrypt(mock.Anything).Return([]byte(sealedByCipher), nil).Once()
		f.repo.EXPECT().Create(mock.Anything, mock.Anything).Return(boom).Once()
		_, err := f.svc.Create(notifCtx(), discordCreateRequest(validWebhook))
		assert.ErrorIs(t, err, domainError.ErrNotificationChannelOperation)
	})
}

func TestNotificationChannelService_List_And_Get_ShowHintOnly(t *testing.T) {
	f := newChannelFixture(t)
	f.repo.EXPECT().List(mock.Anything).Return([]*dto.NotificationChannel{storedChannel("c1", "Cocina")}, nil).Once()
	f.repo.EXPECT().FindByID(mock.Anything, "c1").Return(storedChannel("c1", "Cocina"), nil).Once()

	views, err := f.svc.List(notifCtx())
	require.NoError(t, err)
	require.Len(t, views, 1)
	assert.Equal(t, "…1234", views[0].Settings.WebhookURLHint)

	view, err := f.svc.Get(notifCtx(), "c1")
	require.NoError(t, err)
	assert.Equal(t, "…1234", view.Settings.WebhookURLHint)
	raw, _ := json.Marshal(view)
	assert.NotContains(t, string(raw), sealedByCipher)
}

func TestNotificationChannelService_ListGet_Errors(t *testing.T) {
	f := newChannelFixture(t)
	f.repo.EXPECT().List(mock.Anything).Return(nil, errors.New("boom")).Once()
	f.repo.EXPECT().FindByID(mock.Anything, "other-org").Return(nil, domainError.ErrNotificationChannelNotFound).Once()

	_, err := f.svc.List(notifCtx())
	assert.ErrorIs(t, err, domainError.ErrNotificationChannelOperation)

	_, err = f.svc.Get(notifCtx(), "other-org")
	assert.ErrorIs(t, err, domainError.ErrNotificationChannelNotFound)
}

func TestNotificationChannelService_Update_RenameKeepsSecret(t *testing.T) {
	f := newChannelFixture(t)
	f.repo.EXPECT().FindByID(mock.Anything, "c1").Return(storedChannel("c1", "Domicilios"), nil).Once()
	f.repo.EXPECT().List(mock.Anything).Return([]*dto.NotificationChannel{storedChannel("c1", "Domicilios")}, nil).Once()
	f.repo.EXPECT().Update(mock.Anything, mock.MatchedBy(func(c *dto.NotificationChannel) bool {
		return c.Name == "Cocina" && string(c.Secret) == sealedByCipher && c.SecretHint == "…1234"
	})).Return(nil).Once()
	name := "Cocina"

	view, err := f.svc.Update(notifCtx(), "c1", &dto.UpdateNotificationChannelRequest{Name: &name})

	require.NoError(t, err)
	assert.Equal(t, "Cocina", view.Name)
}

func TestNotificationChannelService_Update_EmptySettingsKeepSecret(t *testing.T) {
	f := newChannelFixture(t)
	f.repo.EXPECT().FindByID(mock.Anything, "c1").Return(storedChannel("c1", "Domicilios"), nil).Once()
	f.repo.EXPECT().Update(mock.Anything, mock.MatchedBy(func(c *dto.NotificationChannel) bool {
		return string(c.Secret) == sealedByCipher && !c.Enabled
	})).Return(nil).Once()
	off := false

	_, err := f.svc.Update(notifCtx(), "c1", &dto.UpdateNotificationChannelRequest{Enabled: &off, Settings: &dto.ChannelSettingsInput{}})

	require.NoError(t, err)
}

func TestNotificationChannelService_Update_NewWebhookIsReEncrypted(t *testing.T) {
	f := newChannelFixture(t)
	url := "https://discord.com/api/webhooks/9/wxyz"
	f.repo.EXPECT().FindByID(mock.Anything, "c1").Return(storedChannel("c1", "Domicilios"), nil).Once()
	f.cipher.EXPECT().Encrypt(mock.Anything).Return([]byte("sealed2"), nil).Once()
	f.repo.EXPECT().Update(mock.Anything, mock.MatchedBy(func(c *dto.NotificationChannel) bool {
		return string(c.Secret) == "sealed2" && c.SecretHint == "…wxyz"
	})).Return(nil).Once()

	view, err := f.svc.Update(notifCtx(), "c1", &dto.UpdateNotificationChannelRequest{Settings: &dto.ChannelSettingsInput{WebhookURL: &url}})

	require.NoError(t, err)
	assert.Equal(t, "…wxyz", view.Settings.WebhookURLHint)
}

func TestNotificationChannelService_Update_Errors(t *testing.T) {
	bad := "https://example.com/x"
	name := "Cocina"
	empty := ""
	t.Run("not found", func(t *testing.T) {
		f := newChannelFixture(t)
		f.repo.EXPECT().FindByID(mock.Anything, "x").Return(nil, domainError.ErrNotificationChannelNotFound).Once()
		_, err := f.svc.Update(notifCtx(), "x", &dto.UpdateNotificationChannelRequest{})
		assert.ErrorIs(t, err, domainError.ErrNotificationChannelNotFound)
	})
	t.Run("bad webhook", func(t *testing.T) {
		f := newChannelFixture(t)
		f.repo.EXPECT().FindByID(mock.Anything, "c1").Return(storedChannel("c1", "A"), nil).Once()
		_, err := f.svc.Update(notifCtx(), "c1", &dto.UpdateNotificationChannelRequest{Settings: &dto.ChannelSettingsInput{WebhookURL: &bad}})
		requireFieldError(t, err, "settings.webhook_url")
	})
	t.Run("empty name", func(t *testing.T) {
		f := newChannelFixture(t)
		f.repo.EXPECT().FindByID(mock.Anything, "c1").Return(storedChannel("c1", "A"), nil).Once()
		_, err := f.svc.Update(notifCtx(), "c1", &dto.UpdateNotificationChannelRequest{Name: &empty})
		requireFieldError(t, err, "name")
	})
	t.Run("duplicate name", func(t *testing.T) {
		f := newChannelFixture(t)
		f.repo.EXPECT().FindByID(mock.Anything, "c1").Return(storedChannel("c1", "A"), nil).Once()
		f.repo.EXPECT().List(mock.Anything).Return([]*dto.NotificationChannel{storedChannel("c1", "A"), storedChannel("c2", "Cocina")}, nil).Once()
		_, err := f.svc.Update(notifCtx(), "c1", &dto.UpdateNotificationChannelRequest{Name: &name})
		requireFieldError(t, err, "name")
	})
	t.Run("list fails", func(t *testing.T) {
		f := newChannelFixture(t)
		f.repo.EXPECT().FindByID(mock.Anything, "c1").Return(storedChannel("c1", "A"), nil).Once()
		f.repo.EXPECT().List(mock.Anything).Return(nil, errors.New("boom")).Once()
		_, err := f.svc.Update(notifCtx(), "c1", &dto.UpdateNotificationChannelRequest{Name: &name})
		assert.ErrorIs(t, err, domainError.ErrNotificationChannelOperation)
	})
	t.Run("encrypt fails", func(t *testing.T) {
		f := newChannelFixture(t)
		url := validWebhook
		f.repo.EXPECT().FindByID(mock.Anything, "c1").Return(storedChannel("c1", "A"), nil).Once()
		f.cipher.EXPECT().Encrypt(mock.Anything).Return(nil, errors.New("boom")).Once()
		_, err := f.svc.Update(notifCtx(), "c1", &dto.UpdateNotificationChannelRequest{Settings: &dto.ChannelSettingsInput{WebhookURL: &url}})
		assert.ErrorIs(t, err, domainError.ErrNotificationChannelOperation)
	})
	t.Run("repo fails", func(t *testing.T) {
		f := newChannelFixture(t)
		f.repo.EXPECT().FindByID(mock.Anything, "c1").Return(storedChannel("c1", "A"), nil).Once()
		f.repo.EXPECT().Update(mock.Anything, mock.Anything).Return(errors.New("boom")).Once()
		_, err := f.svc.Update(notifCtx(), "c1", &dto.UpdateNotificationChannelRequest{})
		assert.ErrorIs(t, err, domainError.ErrNotificationChannelOperation)
	})
}

func TestNotificationChannelService_Delete(t *testing.T) {
	t.Run("deletes", func(t *testing.T) {
		f := newChannelFixture(t)
		f.repo.EXPECT().FindByID(mock.Anything, "c1").Return(storedChannel("c1", "A"), nil).Once()
		f.repo.EXPECT().Delete(mock.Anything, "c1").Return(nil).Once()
		require.NoError(t, f.svc.Delete(notifCtx(), "c1"))
	})
	t.Run("not found", func(t *testing.T) {
		f := newChannelFixture(t)
		f.repo.EXPECT().FindByID(mock.Anything, "c1").Return(nil, domainError.ErrNotificationChannelNotFound).Once()
		assert.ErrorIs(t, f.svc.Delete(notifCtx(), "c1"), domainError.ErrNotificationChannelNotFound)
	})
	t.Run("repo fails", func(t *testing.T) {
		f := newChannelFixture(t)
		f.repo.EXPECT().FindByID(mock.Anything, "c1").Return(storedChannel("c1", "A"), nil).Once()
		f.repo.EXPECT().Delete(mock.Anything, "c1").Return(errors.New("boom")).Once()
		assert.ErrorIs(t, f.svc.Delete(notifCtx(), "c1"), domainError.ErrNotificationChannelOperation)
	})
}

func channelSecretJSON(t *testing.T) []byte {
	t.Helper()
	raw, err := json.Marshal(dto.ChannelSettings{Discord: &dto.DiscordSettings{WebhookURL: validWebhook}})
	require.NoError(t, err)
	return raw
}

func TestNotificationChannelService_Test_Outcomes(t *testing.T) {
	tests := []struct {
		name       string
		outcome    dto.SendOutcome
		sendErr    error
		wantStatus int
		wantErr    bool
	}{
		{"accepted", dto.SendOutcome{Status: dto.SendSent, StatusCode: 204}, nil, 0, false},
		{"webhook deleted", dto.SendOutcome{Status: dto.SendRejected, StatusCode: 404}, nil, 404, true},
		{"destination down", dto.SendOutcome{Status: dto.SendRetry, StatusCode: 503}, nil, 503, true},
		{"rate limited", dto.SendOutcome{Status: dto.SendRetry, StatusCode: 429}, nil, 429, true},
		{"transport error", dto.SendOutcome{}, errors.New("dial tcp"), 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newChannelFixture(t)
			f.repo.EXPECT().FindByID(mock.Anything, "c1").Return(storedChannel("c1", "Domicilios"), nil).Once()
			f.senders.EXPECT().For(dto.NotificationChannelDiscord).Return(f.sender, true).Once()
			f.cipher.EXPECT().Decrypt([]byte(sealedByCipher)).Return(channelSecretJSON(t), nil).Once()
			f.sender.EXPECT().Send(mock.Anything,
				mock.MatchedBy(func(s dto.ChannelSecret) bool {
					return s.Type == dto.NotificationChannelDiscord && s.Settings.Discord.WebhookURL == validWebhook
				}),
				mock.MatchedBy(func(m dto.StaffMessage) bool { return m.Title != "" && !m.At.IsZero() }),
			).Return(tt.outcome, tt.sendErr).Once()

			err := f.svc.Test(notifCtx(), "c1")

			if !tt.wantErr {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, domainError.ErrNotificationChannelUnreachable)
			var unreachable *domainError.NotificationChannelUnreachableError
			require.True(t, errors.As(err, &unreachable))
			assert.Equal(t, tt.wantStatus, unreachable.DestinationStatus)
		})
	}
}

func TestNotificationChannelService_Test_Failures(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		f := newChannelFixture(t)
		f.repo.EXPECT().FindByID(mock.Anything, "c1").Return(nil, domainError.ErrNotificationChannelNotFound).Once()
		assert.ErrorIs(t, f.svc.Test(notifCtx(), "c1"), domainError.ErrNotificationChannelNotFound)
	})
	t.Run("no sender", func(t *testing.T) {
		f := newChannelFixture(t)
		f.repo.EXPECT().FindByID(mock.Anything, "c1").Return(storedChannel("c1", "A"), nil).Once()
		f.senders.EXPECT().For(dto.NotificationChannelDiscord).Return(nil, false).Once()
		assert.ErrorIs(t, f.svc.Test(notifCtx(), "c1"), domainError.ErrNotificationChannelOperation)
	})
	t.Run("decrypt fails", func(t *testing.T) {
		f := newChannelFixture(t)
		f.repo.EXPECT().FindByID(mock.Anything, "c1").Return(storedChannel("c1", "A"), nil).Once()
		f.senders.EXPECT().For(dto.NotificationChannelDiscord).Return(f.sender, true).Once()
		f.cipher.EXPECT().Decrypt(mock.Anything).Return(nil, errors.New("bad key")).Once()
		assert.ErrorIs(t, f.svc.Test(notifCtx(), "c1"), domainError.ErrNotificationChannelOperation)
	})
	t.Run("corrupt secret", func(t *testing.T) {
		f := newChannelFixture(t)
		f.repo.EXPECT().FindByID(mock.Anything, "c1").Return(storedChannel("c1", "A"), nil).Once()
		f.senders.EXPECT().For(dto.NotificationChannelDiscord).Return(f.sender, true).Once()
		f.cipher.EXPECT().Decrypt(mock.Anything).Return([]byte("not json"), nil).Once()
		assert.ErrorIs(t, f.svc.Test(notifCtx(), "c1"), domainError.ErrNotificationChannelOperation)
	})
}
