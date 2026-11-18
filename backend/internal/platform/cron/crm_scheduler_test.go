package cron

import (
	"bytes"
	"log/slog"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/domain/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type slugSecrets map[string]string

func (s slugSecrets) SecretFor(slug string) (string, bool) {
	secret, ok := s[slug]
	return secret, ok
}

func newCRMScheduler(t *testing.T, delivery bool, visitedCron string) (*CRMScheduler, *bytes.Buffer) {
	t.Helper()
	logs := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	orgs := mocks.NewMockOrganizationRepository(t)
	orgs.EXPECT().List(mock.Anything).Return([]*dto.Organization{
		{ID: cronOrgA, Slug: "laguna-escondida", Status: dto.OrganizationStatusActive},
		{ID: cronOrgB, Slug: "restaurante-x", Status: dto.OrganizationStatusActive},
		{ID: cronOrgSuspended, Slug: "cerrado", Status: dto.OrganizationStatusSuspended},
	}, nil).Maybe()
	jobs := CRMJobs{}
	if delivery {
		jobs.Delivery = &service.MarketingEventDeliveryService{}
	}
	scheduler, err := NewCRMScheduler(cronOrganizations(t), orgs, slugSecrets{"laguna-escondida": "s3cret"}, jobs,
		visitedCron, "5 * * * *", logger)
	require.NoError(t, err)
	t.Cleanup(func() { _ = scheduler.Stop() })
	return scheduler, logs
}

func TestCRMScheduler_RegistersItsJobs(t *testing.T) {
	scheduler, logs := newCRMScheduler(t, true, "*/15 * * * *")

	require.NoError(t, scheduler.registerJobs())

	assert.Len(t, scheduler.scheduler.Jobs(), 3)
	assert.Contains(t, logs.String(), "Registered cron job: markReservationsVisited")
	assert.Contains(t, logs.String(), "Registered cron job: requestFeedback")
	assert.Contains(t, logs.String(), "Registered job: deliverMarketingEvents")
}

func TestCRMScheduler_WithoutTheEventsURLSendsNothing(t *testing.T) {
	scheduler, logs := newCRMScheduler(t, false, "*/15 * * * *")

	require.NoError(t, scheduler.registerJobs())
	scheduler.warnOrganizationsWithoutSecret(t.Context())

	assert.Len(t, scheduler.scheduler.Jobs(), 2)
	assert.Contains(t, logs.String(), "Marketing events disabled")
	assert.NotContains(t, logs.String(), "no marketing event signing secret")
}

func TestCRMScheduler_WarnsForEachActiveOrganizationWithoutASecret(t *testing.T) {
	scheduler, logs := newCRMScheduler(t, true, "*/15 * * * *")

	scheduler.warnOrganizationsWithoutSecret(t.Context())

	assert.Contains(t, logs.String(), "slug=restaurante-x")
	assert.NotContains(t, logs.String(), "slug=laguna-escondida")
	assert.NotContains(t, logs.String(), "slug=cerrado", "a suspended organization is not warned about")
}

func TestCRMScheduler_RejectsAnInvalidCron(t *testing.T) {
	scheduler, _ := newCRMScheduler(t, false, "not a cron")

	assert.Error(t, scheduler.registerJobs())
}
