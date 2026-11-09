package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports/mocks"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const marketingOrderID = "019a0000-0000-7000-8000-0000000000d1"

type recordFunc func(ctx context.Context, s *MarketingEventService) error

// recordInTx runs record inside the fake unit of work, as every caller does.
func recordInTx(t *testing.T, outbox *txOutbox, record recordFunc, events *MarketingEventService) []*dto.MarketingEvent {
	t.Helper()
	require.NoError(t, outbox.Do(crmContext(), func(ctx context.Context) error { return record(ctx, events) }))
	return outbox.events()
}

// requireFixture compares the stored bytes to testdata, with the generated event_id substituted.
func requireFixture(t *testing.T, name string, event *dto.MarketingEvent) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "marketing_events", name)) //nolint:gosec // G304: fixture names are in-test constants
	require.NoError(t, err)
	want := strings.ReplaceAll(strings.TrimSpace(string(raw)), "{{event_id}}", event.EventID)
	assert.Equal(t, want, string(event.Body), "the exact bytes signed and sent")
	_, err = uuid.Parse(event.EventID)
	assert.NoError(t, err, "event_id is a UUID")
}

func orderSubject(t *testing.T) dto.MarketingOrderSubject {
	return dto.MarketingOrderSubject{
		OrderID:       marketingOrderID,
		CustomerPhone: strPtr(crmPhone),
		Origin:        &dto.Origin{ChannelID: strPtr(crmChannel), ConversationID: strPtr(crmConvID)},
		Amount:        decimal.RequireFromString("45000.0000"),
		OccurredAt:    crmAt(t, 8, 20, 30).UTC(),
	}
}

func fixtureReservation(t *testing.T, status dto.ReservationStatus) *dto.Reservation {
	r := storedReservation(status, crmAt(t, 9, 20, 0).UTC())
	return r
}

func TestMarketingEventService_EventBodiesMatchTheContract(t *testing.T) {
	declined := fixtureReservation(t, dto.ReservationStatusDeclined)
	declined.DeclineReason = strPtr("Fully booked")
	cancelled := fixtureReservation(t, dto.ReservationStatusCancelled)
	by := dto.CancelledByCustomer
	cancelled.CancelledBy, cancelled.Origin.ConversationID = &by, nil
	visited := storedReservation(dto.ReservationStatusVisited, crmAt(t, 8, 20, 0))
	feedbackRequest := &dto.FeedbackRequest{ID: "fr1", ReservationID: "r1", RequestedAt: crmAt(t, 9, 11, 5), ExpiresAt: crmAt(t, 16, 11, 5)}
	confirmedAt := crmAt(t, 8, 15, 0)

	tests := []struct {
		fixture  string
		path     string
		wantType dto.MarketingEventType
		record   recordFunc
	}{
		{fixture: "order_paid.json", path: "/backend/events", wantType: dto.MarketingEventOrderPaid, record: func(ctx context.Context, s *MarketingEventService) error {
			return s.RecordOrder(ctx, dto.MarketingEventOrderPaid, orderSubject(t))
		}},
		{fixture: "order_cancelled.json", path: "/backend/events", wantType: dto.MarketingEventOrderCancelled, record: func(ctx context.Context, s *MarketingEventService) error {
			return s.RecordOrder(ctx, dto.MarketingEventOrderCancelled, orderSubject(t))
		}},
		{fixture: "reservation_confirmed.json", path: "/backend/notifications", wantType: dto.MarketingEventReservationConfirmed, record: func(ctx context.Context, s *MarketingEventService) error {
			return s.RecordReservation(ctx, fixtureReservation(t, dto.ReservationStatusConfirmed), confirmedAt)
		}},
		{fixture: "reservation_declined.json", path: "/backend/notifications", wantType: dto.MarketingEventReservationDeclined, record: func(ctx context.Context, s *MarketingEventService) error {
			return s.RecordReservation(ctx, declined, confirmedAt)
		}},
		{fixture: "reservation_cancelled.json", path: "/backend/notifications", wantType: dto.MarketingEventReservationCancelled, record: func(ctx context.Context, s *MarketingEventService) error {
			return s.RecordReservation(ctx, cancelled, confirmedAt)
		}},
		{fixture: "feedback_requested.json", path: "/backend/notifications", wantType: dto.MarketingEventFeedbackRequested, record: func(ctx context.Context, s *MarketingEventService) error {
			return s.RecordFeedbackRequested(ctx, feedbackRequest, visited)
		}},
	}
	for _, tt := range tests {
		t.Run(string(tt.wantType), func(t *testing.T) {
			outbox := &txOutbox{}
			events := recordInTx(t, outbox, tt.record, newRecordingEvents(t, outbox))

			require.Len(t, events, 1)
			assert.Equal(t, tt.wantType, events[0].Type)
			assert.Equal(t, tt.path, events[0].Path)
			assert.Equal(t, dto.MarketingEventPending, events[0].Status)
			requireFixture(t, tt.fixture, events[0])
		})
	}
}

func TestMarketingEventService_RecordsNothingWhen(t *testing.T) {
	noChannel := orderSubject(t)
	noChannel.Origin = &dto.Origin{ConversationID: strPtr(crmConvID)}
	noPhone := orderSubject(t)
	noPhone.CustomerPhone = nil

	tests := []struct {
		name    string
		events  func(t *testing.T, outbox *txOutbox) *MarketingEventService
		subject dto.MarketingOrderSubject
	}{
		{name: "an order taken at the till", events: newRecordingEvents, subject: dto.MarketingOrderSubject{OrderID: marketingOrderID, CustomerPhone: strPtr(crmPhone)}},
		{name: "an origin without a channel", events: newRecordingEvents, subject: noChannel},
		{name: "no phone to tell", events: newRecordingEvents, subject: noPhone},
		{name: "the events URL is unset", subject: orderSubject(t), events: func(t *testing.T, outbox *txOutbox) *MarketingEventService {
			return NewMarketingEventService(outbox, mocks.NewMockOrganizationRepository(t), staticSecrets{crmOrgSlug: crmSecret}, false)
		}},
		{name: "a restaurant not yet onboarded", subject: orderSubject(t), events: func(t *testing.T, outbox *txOutbox) *MarketingEventService {
			return NewMarketingEventService(outbox, crmOrganizations(t), staticSecrets{"restaurante-x": crmSecret}, true)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outbox := &txOutbox{}
			events := recordInTx(t, outbox, func(ctx context.Context, s *MarketingEventService) error {
				return s.RecordOrder(ctx, dto.MarketingEventOrderPaid, tt.subject)
			}, tt.events(t, outbox))

			assert.Empty(t, events)
		})
	}
}

func TestMarketingEventService_ReservationStatesThatAreNotReported(t *testing.T) {
	for _, status := range []dto.ReservationStatus{dto.ReservationStatusRequested, dto.ReservationStatusVisited, dto.ReservationStatusNoShow} {
		t.Run(string(status), func(t *testing.T) {
			outbox := &txOutbox{}
			events := recordInTx(t, outbox, func(ctx context.Context, s *MarketingEventService) error {
				return s.RecordReservation(ctx, fixtureReservation(t, status), time.Now())
			}, newRecordingEvents(t, outbox))

			assert.Empty(t, events)
		})
	}
}

func TestMarketingEventService_FeedbackWithoutChannelIsNotReported(t *testing.T) {
	outbox := &txOutbox{}
	r := storedReservation(dto.ReservationStatusVisited, time.Now())
	r.Origin = dto.Origin{}

	events := recordInTx(t, outbox, func(ctx context.Context, s *MarketingEventService) error {
		return s.RecordFeedbackRequested(ctx, &dto.FeedbackRequest{ID: "fr1"}, r)
	}, newRecordingEvents(t, outbox))

	assert.Empty(t, events)
}

func TestMarketingEventService_Failures(t *testing.T) {
	t.Run("organization", func(t *testing.T) {
		orgs := mocks.NewMockOrganizationRepository(t)
		orgs.EXPECT().Get(mock.Anything, crmOrgID).Return(nil, errors.New("db down")).Once()
		svc := NewMarketingEventService(&txOutbox{}, orgs, staticSecrets{}, true)
		err := svc.RecordOrder(crmContext(), dto.MarketingEventOrderPaid, orderSubject(t))
		require.ErrorIs(t, err, domainError.ErrMarketingEventRecordError)
	})
	t.Run("no organization in context", func(t *testing.T) {
		svc := NewMarketingEventService(&txOutbox{}, mocks.NewMockOrganizationRepository(t), staticSecrets{}, true)
		err := svc.RecordOrder(t.Context(), dto.MarketingEventOrderPaid, orderSubject(t))
		require.ErrorIs(t, err, domainError.ErrNoOrganization)
	})
	t.Run("outbox", func(t *testing.T) {
		outbox := &txOutbox{appendErr: errors.New("outbox down")}
		err := outbox.Do(crmContext(), func(ctx context.Context) error {
			return newRecordingEvents(t, outbox).RecordOrder(ctx, dto.MarketingEventOrderPaid, orderSubject(t))
		})
		require.ErrorIs(t, err, domainError.ErrMarketingEventRecordError)
		assert.Empty(t, outbox.events())
	})
}

func TestMarketingEventService_OrganizationFailuresFailEveryRecord(t *testing.T) {
	visited := storedReservation(dto.ReservationStatusVisited, time.Now())
	records := map[string]recordFunc{
		"reservation": func(ctx context.Context, s *MarketingEventService) error {
			return s.RecordReservation(ctx, fixtureReservation(t, dto.ReservationStatusConfirmed), time.Now())
		},
		"feedback": func(ctx context.Context, s *MarketingEventService) error {
			return s.RecordFeedbackRequested(ctx, &dto.FeedbackRequest{ID: "fr1"}, visited)
		},
	}
	for name, record := range records {
		t.Run(name, func(t *testing.T) {
			orgs := mocks.NewMockOrganizationRepository(t)
			orgs.EXPECT().Get(mock.Anything, crmOrgID).Return(nil, errors.New("db down")).Once()
			outbox := &txOutbox{}

			err := outbox.Do(crmContext(), func(ctx context.Context) error {
				return record(ctx, NewMarketingEventService(outbox, orgs, staticSecrets{crmOrgSlug: crmSecret}, true))
			})

			require.ErrorIs(t, err, domainError.ErrMarketingEventRecordError)
			assert.Empty(t, outbox.events())
		})
	}
}

func TestMarketingEventService_AnUnknownTimezoneFallsBackToUTC(t *testing.T) {
	orgs := mocks.NewMockOrganizationRepository(t)
	orgs.EXPECT().Get(mock.Anything, crmOrgID).Return(&dto.Organization{ID: crmOrgID, Slug: crmOrgSlug, Timezone: "Mars/Olympus", Currency: "COP"}, nil).Once()
	outbox := &txOutbox{}

	events := recordInTx(t, outbox, func(ctx context.Context, s *MarketingEventService) error {
		return s.RecordOrder(ctx, dto.MarketingEventOrderPaid, orderSubject(t))
	}, NewMarketingEventService(outbox, orgs, staticSecrets{crmOrgSlug: crmSecret}, true))

	require.Len(t, events, 1)
	assert.Contains(t, string(events[0].Body), `"occurred_at":"2026-11-09T01:30:00Z"`)
}
