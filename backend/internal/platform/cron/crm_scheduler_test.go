package cron

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/domain/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const cronOrgFailing = "019a0000-0000-7000-8000-0000000000ff"

type orgSecrets map[string]string

func (s orgSecrets) SecretFor(_ context.Context, organizationID string) (string, bool, error) {
	if organizationID == cronOrgFailing {
		return "", false, errors.New("db down")
	}
	secret, ok := s[organizationID]
	return secret, ok, nil
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
		{ID: cronOrgFailing, Slug: "sin-lectura", Status: dto.OrganizationStatusActive},
	}, nil).Maybe()
	jobs := CRMJobs{}
	if delivery {
		jobs.Delivery = &service.MarketingEventDeliveryService{}
	}
	scheduler, err := NewCRMScheduler(cronOrganizations(t), orgs, orgSecrets{cronOrgA: "s3cret"}, jobs,
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
	assert.Contains(t, logs.String(), "Could not read an organization's marketing event signing secret")
	assert.NotContains(t, logs.String(), "s3cret")
}

func TestCRMScheduler_RejectsAnInvalidCron(t *testing.T) {
	scheduler, _ := newCRMScheduler(t, false, "not a cron")

	assert.Error(t, scheduler.registerJobs())
}
