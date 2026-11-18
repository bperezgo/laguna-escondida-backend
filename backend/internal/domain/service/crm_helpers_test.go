package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/domain/tenant"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	crmOrgID    = "019a0000-0000-7000-8000-0000000000c1"
	crmOrgSlug  = "laguna-escondida"
	crmSecret   = "s3cret"
	crmPhone    = "+573005551234"
	crmChannel  = "ch_1"
	crmConvID   = "conv_9"
	crmTimezone = "America/Bogota"
)

func crmContext() context.Context {
	return tenant.WithOrg(context.Background(), crmOrgID)
}

func crmBogota(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(crmTimezone)
	require.NoError(t, err)
	return loc
}

// crmAt is a Bogotá wall-clock time on 2026-11-dd.
func crmAt(t *testing.T, day, hour, minute int) time.Time {
	return time.Date(2026, time.November, day, hour, minute, 0, 0, crmBogota(t))
}

func fixedClock(at time.Time) func() time.Time {
	return func() time.Time { return at }
}

// txOutbox is a unit of work and a marketing outbox in one: rows appended inside Do are kept only
// when fn succeeds, as a real transaction keeps them.
type txOutbox struct {
	mu        sync.Mutex
	committed []*dto.MarketingEvent
	pending   []*dto.MarketingEvent
	inTx      bool
	appendErr error
}

func (o *txOutbox) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	o.mu.Lock()
	o.inTx, o.pending = true, nil
	o.mu.Unlock()
	err := fn(context.WithValue(ctx, inUnitOfWorkKey{}, true))
	o.mu.Lock()
	defer o.mu.Unlock()
	if err == nil {
		o.committed = append(o.committed, o.pending...)
	}
	o.inTx, o.pending = false, nil
	return err
}

func (o *txOutbox) Append(ctx context.Context, event *dto.MarketingEvent) error {
	if o.appendErr != nil {
		return o.appendErr
	}
	if !inUnitOfWork(ctx) {
		return errors.New("marketing event appended outside a unit of work")
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	o.pending = append(o.pending, event)
	return nil
}

func (o *txOutbox) events() []*dto.MarketingEvent {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]*dto.MarketingEvent(nil), o.committed...)
}

type staticSecrets map[string]string

func (s staticSecrets) SecretFor(slug string) (string, bool) {
	secret, ok := s[slug]
	return secret, ok
}

// crmOrganizations answers Get for the test organization, in Bogotá and in COP.
func crmOrganizations(t *testing.T) *mocks.MockOrganizationRepository {
	orgs := mocks.NewMockOrganizationRepository(t)
	orgs.EXPECT().Get(mock.Anything, crmOrgID).Return(&dto.Organization{
		ID: crmOrgID, Slug: crmOrgSlug, Status: dto.OrganizationStatusActive, Timezone: crmTimezone, Currency: "COP",
	}, nil).Maybe()
	return orgs
}

// newRecordingEvents is the real recorder over outbox, enabled with the test organization's secret.
func newRecordingEvents(t *testing.T, outbox *txOutbox) *MarketingEventService {
	return NewMarketingEventService(outbox, crmOrganizations(t), staticSecrets{crmOrgSlug: crmSecret}, true)
}

func crmSettings(t *testing.T, feedback dto.FeedbackSettings) *mocks.MockOrganizationSettingsRepository {
	settings := mocks.NewMockOrganizationSettingsRepository(t)
	settings.EXPECT().Get(mock.Anything).Return(&dto.OrganizationSettingsView{Timezone: crmTimezone, Feedback: feedback}, nil).Maybe()
	return settings
}
