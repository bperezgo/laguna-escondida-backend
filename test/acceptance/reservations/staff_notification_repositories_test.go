package reservations

import (
	"context"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/tenant"
	"laguna-escondida/backend/internal/platform/postgres/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedChannelAndRule(t *testing.T, ctx context.Context, name string) (*dto.NotificationChannel, *dto.NotificationRule) {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Microsecond)
	channel := &dto.NotificationChannel{
		ID: uuid.NewString(), Name: name, Type: dto.NotificationChannelDiscord, Enabled: true,
		Secret: []byte{1, 2, 3}, SecretHint: "…abcd", CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, repository.NewNotificationChannelRepository(cloudDB).Create(ctx, channel))
	rule := &dto.NotificationRule{
		ID: uuid.NewString(), ChannelID: channel.ID, Event: dto.NotificationEventOrderPaid,
		Conditions:  dto.NotificationConditions{OrderTypes: []dto.OrderType{dto.OrderTypeDelivery}},
		TitlePrefix: "<@&1>", ExpiresAfterMinutes: 30, Enabled: true, CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, repository.NewNotificationRuleRepository(cloudDB).Create(ctx, rule))
	return channel, rule
}

func TestNotificationRepositories_OrganizationIsolation(t *testing.T) {
	newCloud(t)
	orgA, orgB := seedOrganization(t), seedOrganization(t)
	ctxA, ctxB := tenant.WithOrg(context.Background(), orgA), tenant.WithOrg(context.Background(), orgB)
	channels, rules := repository.NewNotificationChannelRepository(cloudDB), repository.NewNotificationRuleRepository(cloudDB)

	channel, rule := seedChannelAndRule(t, ctxA, "Domicilios")
	seedChannelAndRule(t, ctxB, "Domicilios")

	got, err := channels.FindByID(ctxA, channel.ID)
	require.NoError(t, err)
	assert.Equal(t, orgA, got.OrganizationID)
	assert.Equal(t, []byte{1, 2, 3}, got.Secret)

	_, err = channels.FindByID(ctxB, channel.ID)
	assert.ErrorIs(t, err, domainError.ErrNotificationChannelNotFound)
	_, err = rules.FindByID(ctxB, rule.ID)
	assert.ErrorIs(t, err, domainError.ErrNotificationRuleNotFound)

	listA, err := channels.List(ctxA)
	require.NoError(t, err)
	assert.Len(t, listA, 1)
	ruleList, err := rules.List(ctxA, channel.ID)
	require.NoError(t, err)
	require.Len(t, ruleList, 1)
	assert.Equal(t, []dto.OrderType{dto.OrderTypeDelivery}, ruleList[0].Conditions.OrderTypes)

	enabledA, err := rules.ListEnabledForEvent(ctxA, dto.NotificationEventOrderPaid)
	require.NoError(t, err)
	require.Len(t, enabledA, 1)
	assert.Equal(t, rule.ID, enabledA[0].Rule.ID)
	assert.Equal(t, channel.ID, enabledA[0].Channel.ID)

	require.NoError(t, channels.Delete(ctxB, channel.ID))
	_, err = channels.FindByID(ctxA, channel.ID)
	assert.NoError(t, err, "another organization's delete leaves the channel alone")
}

func TestNotificationRepositories_ChannelDeleteCascadesToRules(t *testing.T) {
	newCloud(t)
	ctx := tenant.WithOrg(context.Background(), seedOrganization(t))
	channels, rules := repository.NewNotificationChannelRepository(cloudDB), repository.NewNotificationRuleRepository(cloudDB)
	channel, rule := seedChannelAndRule(t, ctx, "Cocina")

	require.NoError(t, channels.Delete(ctx, channel.ID))

	_, err := channels.FindByID(ctx, channel.ID)
	assert.ErrorIs(t, err, domainError.ErrNotificationChannelNotFound)
	_, err = rules.FindByID(ctx, rule.ID)
	assert.ErrorIs(t, err, domainError.ErrNotificationRuleNotFound)
	enabled, err := rules.ListEnabledForEvent(ctx, dto.NotificationEventOrderPaid)
	require.NoError(t, err)
	assert.Empty(t, enabled)

	var deleted int64
	require.NoError(t, cloudDB.Table("notification_rules").Where("id = ? AND deleted_at IS NOT NULL", rule.ID).Count(&deleted).Error)
	assert.EqualValues(t, 1, deleted, "soft delete keeps the row")

	require.NoError(t, cloudDB.Exec("DELETE FROM notification_channels WHERE id = ?", channel.ID).Error)
	require.NoError(t, cloudDB.Table("notification_rules").Where("id = ?", rule.ID).Count(&deleted).Error)
	assert.Zero(t, deleted, "a hard delete cascades through the foreign key")
}

func TestNotificationRepositories_ChannelNameIsUniquePerOrganization(t *testing.T) {
	newCloud(t)
	ctxA := tenant.WithOrg(context.Background(), seedOrganization(t))
	ctxB := tenant.WithOrg(context.Background(), seedOrganization(t))
	channels := repository.NewNotificationChannelRepository(cloudDB)
	channel, _ := seedChannelAndRule(t, ctxA, "Domicilios")
	seedChannelAndRule(t, ctxB, "Domicilios")

	dup := *channel
	dup.ID = uuid.NewString()
	assert.Error(t, channels.Create(ctxA, &dup))

	require.NoError(t, channels.Delete(ctxA, channel.ID))
	assert.NoError(t, channels.Create(ctxA, &dup), "a deleted channel frees its name")
}

func TestStaffNotificationOutbox_InsertIsIdempotentAndClaimReportsLiveness(t *testing.T) {
	newCloud(t)
	orgA, orgB := seedOrganization(t), seedOrganization(t)
	ctxA, ctxB := tenant.WithOrg(context.Background(), orgA), tenant.WithOrg(context.Background(), orgB)
	outbox := repository.NewStaffNotificationOutboxRepository(cloudDB)
	channels, rules := repository.NewNotificationChannelRepository(cloudDB), repository.NewNotificationRuleRepository(cloudDB)
	channel, rule := seedChannelAndRule(t, ctxA, "Domicilios")
	now := time.Now().UTC().Truncate(time.Microsecond)
	eventID := uuid.NewString()
	notification := func() *dto.StaffNotification {
		return &dto.StaffNotification{
			ID: uuid.NewString(), RuleID: rule.ID, ChannelID: channel.ID, EventID: eventID,
			Message: dto.StaffMessage{Prefix: "<@&1>", Title: "Pedido", Fields: []dto.MessageField{{Name: "Total", Value: "10", Inline: true}}, At: now},
			Status:  dto.StaffNotificationPending, NextAttemptAt: now, ExpiresAt: now.Add(time.Hour), CreatedAt: now,
		}
	}

	inserted, err := outbox.Insert(ctxA, notification())
	require.NoError(t, err)
	assert.True(t, inserted)
	inserted, err = outbox.Insert(ctxA, notification())
	require.NoError(t, err)
	assert.False(t, inserted, "the same rule and event stores one row")
	var rows int64
	require.NoError(t, cloudDB.Table("staff_notifications").Where("rule_id = ? AND event_id = ?", rule.ID, eventID).Count(&rows).Error)
	assert.EqualValues(t, 1, rows)

	other, err := outbox.ClaimDue(ctxB, now.Add(time.Second), now.Add(time.Minute), 10)
	require.NoError(t, err)
	assert.Empty(t, other, "another organization claims nothing")

	claimed, err := outbox.ClaimDue(ctxA, now.Add(time.Second), now.Add(time.Minute), 10)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	c := claimed[0]
	assert.True(t, c.Live)
	assert.Equal(t, dto.NotificationChannelDiscord, c.ChannelType)
	assert.Equal(t, []byte{1, 2, 3}, c.Secret)
	assert.Equal(t, 1, c.Notification.Attempts)
	assert.Equal(t, "Pedido", c.Notification.Message.Title)
	assert.Equal(t, []dto.MessageField{{Name: "Total", Value: "10", Inline: true}}, c.Notification.Message.Fields)

	again, err := outbox.ClaimDue(ctxA, now.Add(2*time.Second), now.Add(time.Minute), 10)
	require.NoError(t, err)
	assert.Empty(t, again, "a leased notification is not claimed again")

	require.NoError(t, outbox.MarkFailed(ctxA, c.Notification.ID, dto.StaffNotificationPending, now.Add(2*time.Second), "HTTP 502"))
	enabled := false
	disabledRule := *rule
	disabledRule.Enabled = enabled
	require.NoError(t, rules.Update(ctxA, &disabledRule))
	retried, err := outbox.ClaimDue(ctxA, now.Add(3*time.Second), now.Add(time.Minute), 10)
	require.NoError(t, err)
	require.Len(t, retried, 1)
	assert.False(t, retried[0].Live, "a disabled rule is reported not live")
	assert.Equal(t, 2, retried[0].Notification.Attempts)

	require.NoError(t, outbox.MarkSent(ctxA, c.Notification.ID, now))
	var status string
	require.NoError(t, cloudDB.Table("staff_notifications").Select("status").Where("id = ?", c.Notification.ID).Scan(&status).Error)
	assert.Equal(t, string(dto.StaffNotificationSent), status)

	require.NoError(t, channels.Delete(ctxA, channel.ID))
}
