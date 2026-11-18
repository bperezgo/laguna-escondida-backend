package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type feedbackFixture struct {
	feedback     *mocks.MockFeedbackRepository
	reservations *mocks.MockReservationRepository
	outbox       *txOutbox
	service      *FeedbackService
	requests     []*dto.FeedbackRequest
}

func newFeedbackFixture(t *testing.T, now time.Time, settings dto.FeedbackSettings) *feedbackFixture {
	f := &feedbackFixture{
		feedback:     mocks.NewMockFeedbackRepository(t),
		reservations: mocks.NewMockReservationRepository(t),
		outbox:       &txOutbox{},
	}
	f.service = NewFeedbackService(f.feedback, f.reservations, f.outbox, crmSettings(t, settings), newRecordingEvents(t, f.outbox), "CO", fixedClock(now))
	f.feedback.EXPECT().CreateRequest(mock.Anything, mock.Anything).RunAndReturn(func(ctx context.Context, r *dto.FeedbackRequest) error {
		require.True(t, inUnitOfWork(ctx))
		f.requests = append(f.requests, r)
		return nil
	}).Maybe()
	return f
}

// awaiting makes the repository return these visits, filtered by visitedSince as the query does.
func (f *feedbackFixture) awaiting(visits ...*dto.Reservation) {
	f.reservations.EXPECT().ListAwaitingFeedback(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, since time.Time) ([]*dto.Reservation, error) {
		var out []*dto.Reservation
		for _, v := range visits {
			if v.Status == dto.ReservationStatusVisited && v.Origin.ChannelID != nil && v.VisitedAt != nil && !v.VisitedAt.Before(since) {
				out = append(out, v)
			}
		}
		return out, nil
	}).Once()
}

func visit(t *testing.T, startsAt time.Time, status dto.ReservationStatus, channel bool) *dto.Reservation {
	r := storedReservation(status, startsAt)
	if !channel {
		r.Origin = dto.Origin{}
		r.Source = dto.ReservationSourcePhone
	}
	if status == dto.ReservationStatusVisited {
		visitedAt := startsAt.Add(2*time.Hour + 5*time.Minute)
		r.VisitedAt = &visitedAt
	}
	return r
}

func TestFeedbackService_RequestDue_DinnerLastNight(t *testing.T) {
	now := crmAt(t, 9, 11, 5)
	f := newFeedbackFixture(t, now, dto.FeedbackSettings{})
	f.awaiting(visit(t, crmAt(t, 8, 20, 0), dto.ReservationStatusVisited, true))

	requested, err := f.service.RequestDue(crmContext())

	require.NoError(t, err)
	assert.Equal(t, 1, requested)
	require.Len(t, f.requests, 1)
	assert.Equal(t, "r1", f.requests[0].ReservationID)
	assert.Equal(t, now.Add(7*24*time.Hour), f.requests[0].ExpiresAt)
	events := f.outbox.events()
	require.Len(t, events, 1)
	assert.Equal(t, dto.MarketingEventFeedbackRequested, events[0].Type)
	assert.Equal(t, dto.MarketingEventsPathNotifications, events[0].Path)
}

func TestFeedbackService_RequestDue_Schedule(t *testing.T) {
	tests := []struct {
		name  string
		now   time.Time
		visit func(t *testing.T) *dto.Reservation
		want  int
	}{
		{name: "the morning after, before 11:00", now: crmAt(t, 9, 10, 59), visit: func(t *testing.T) *dto.Reservation {
			return visit(t, crmAt(t, 8, 20, 0), dto.ReservationStatusVisited, true)
		}},
		{name: "a late dinner past midnight UTC is still asked the next local day", now: crmAt(t, 9, 11, 0), want: 1, visit: func(t *testing.T) *dto.Reservation {
			return visit(t, crmAt(t, 8, 21, 30), dto.ReservationStatusVisited, true)
		}},
		{name: "lunch today is asked tomorrow", now: crmAt(t, 9, 18, 0), visit: func(t *testing.T) *dto.Reservation {
			return visit(t, crmAt(t, 9, 13, 0), dto.ReservationStatusVisited, true)
		}},
		{name: "a no-show", now: crmAt(t, 9, 11, 5), visit: func(t *testing.T) *dto.Reservation {
			return visit(t, crmAt(t, 8, 20, 0), dto.ReservationStatusNoShow, true)
		}},
		{name: "booked by phone", now: crmAt(t, 9, 11, 5), visit: func(t *testing.T) *dto.Reservation {
			return visit(t, crmAt(t, 8, 20, 0), dto.ReservationStatusVisited, false)
		}},
		{name: "an old backlog, visited 5 days ago", now: crmAt(t, 13, 11, 5), visit: func(t *testing.T) *dto.Reservation {
			return visit(t, crmAt(t, 8, 20, 0), dto.ReservationStatusVisited, true)
		}},
		{name: "visited just under 3 days ago", now: crmAt(t, 11, 22, 0), want: 1, visit: func(t *testing.T) *dto.Reservation {
			return visit(t, crmAt(t, 8, 20, 0), dto.ReservationStatusVisited, true)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFeedbackFixture(t, tt.now, dto.FeedbackSettings{})
			f.awaiting(tt.visit(t))

			requested, err := f.service.RequestDue(crmContext())

			require.NoError(t, err)
			assert.Equal(t, tt.want, requested)
			assert.Len(t, f.requests, tt.want)
			assert.Len(t, f.outbox.events(), tt.want)
		})
	}
}

func TestFeedbackService_RequestDue_OnePerReservation(t *testing.T) {
	f := &feedbackFixture{feedback: mocks.NewMockFeedbackRepository(t), reservations: mocks.NewMockReservationRepository(t), outbox: &txOutbox{}}
	f.service = NewFeedbackService(f.feedback, f.reservations, f.outbox, crmSettings(t, dto.FeedbackSettings{}), newRecordingEvents(t, f.outbox), "CO", fixedClock(crmAt(t, 9, 12, 0)))
	f.awaiting(visit(t, crmAt(t, 8, 20, 0), dto.ReservationStatusVisited, true))
	f.feedback.EXPECT().CreateRequest(mock.Anything, mock.Anything).Return(domainError.ErrFeedbackRequestExists).Once()

	requested, err := f.service.RequestDue(crmContext())

	require.NoError(t, err)
	assert.Zero(t, requested)
	assert.Empty(t, f.outbox.events(), "no second feedback.requested")
}

func TestFeedbackService_RequestDue_AnEventFailureRollsBackTheRequest(t *testing.T) {
	f := newFeedbackFixture(t, crmAt(t, 9, 12, 0), dto.FeedbackSettings{})
	f.outbox.appendErr = errors.New("outbox down")
	f.awaiting(visit(t, crmAt(t, 8, 20, 0), dto.ReservationStatusVisited, true))

	_, err := f.service.RequestDue(crmContext())

	require.ErrorIs(t, err, domainError.ErrFeedbackOperation)
	assert.Empty(t, f.outbox.events())
}

func TestFeedbackService_RequestDue_Failures(t *testing.T) {
	t.Run("list", func(t *testing.T) {
		f := newFeedbackFixture(t, crmAt(t, 9, 12, 0), dto.FeedbackSettings{})
		f.reservations.EXPECT().ListAwaitingFeedback(mock.Anything, mock.Anything).Return(nil, errors.New("db down")).Once()
		_, err := f.service.RequestDue(crmContext())
		require.ErrorIs(t, err, domainError.ErrFeedbackOperation)
	})
	t.Run("settings", func(t *testing.T) {
		settings := mocks.NewMockOrganizationSettingsRepository(t)
		settings.EXPECT().Get(mock.Anything).Return(nil, errors.New("db down")).Once()
		svc := NewFeedbackService(mocks.NewMockFeedbackRepository(t), mocks.NewMockReservationRepository(t), &txOutbox{}, settings, nil, "CO", nil)
		_, err := svc.RequestDue(crmContext())
		require.ErrorIs(t, err, domainError.ErrFeedbackOperation)
	})
	t.Run("timezone", func(t *testing.T) {
		settings := mocks.NewMockOrganizationSettingsRepository(t)
		settings.EXPECT().Get(mock.Anything).Return(&dto.OrganizationSettingsView{Timezone: "Mars/Olympus"}, nil).Once()
		svc := NewFeedbackService(mocks.NewMockFeedbackRepository(t), mocks.NewMockReservationRepository(t), &txOutbox{}, settings, nil, "CO", nil)
		_, err := svc.RequestDue(crmContext())
		require.ErrorIs(t, err, domainError.ErrFeedbackOperation)
	})
}

// recording makes the latest open request "req-1" and saves what is recorded on it, replacing.
func (f *feedbackFixture) recording(t *testing.T) *[]*dto.Feedback {
	saved := &[]*dto.Feedback{}
	f.feedback.EXPECT().FindLatestOpenRequest(mock.Anything, crmPhone, mock.Anything).Return(&dto.FeedbackRequest{ID: "req-1", ReservationID: "r1"}, nil).Maybe()
	f.feedback.EXPECT().Save(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, fb *dto.Feedback) (*dto.Feedback, error) {
		require.Equal(t, "req-1", fb.FeedbackRequestID)
		*saved = append(*saved, fb)
		stored := *fb
		stored.ContactPhone = crmPhone
		return &stored, nil
	}).Maybe()
	return saved
}

func TestFeedbackService_Record_RatingAndReviewLink(t *testing.T) {
	const reviewURL = "https://g.page/r/abc/review"
	links := []struct {
		name     string
		settings dto.FeedbackSettings
		wantLink bool
	}{
		{name: "link off", settings: dto.FeedbackSettings{ReviewURL: reviewURL}},
		{name: "link on", settings: dto.FeedbackSettings{ReviewLinkEnabled: true, ReviewURL: reviewURL}, wantLink: true},
		{name: "on without a URL", settings: dto.FeedbackSettings{ReviewLinkEnabled: true}},
	}
	for _, link := range links {
		for rating := 1; rating <= 5; rating++ {
			t.Run(link.name+" rating "+string(rune('0'+rating)), func(t *testing.T) {
				f := newFeedbackFixture(t, crmAt(t, 9, 12, 0), link.settings)
				saved := f.recording(t)

				result, err := f.service.Record(crmContext(), &dto.RecordFeedbackRequest{CustomerPhone: "573005551234", Rating: rating})

				require.NoError(t, err)
				wantEscalate := rating <= 3
				assert.Equal(t, wantEscalate, result.Escalate)
				assert.Equal(t, wantEscalate, result.Feedback.NeedsAttention)
				assert.Equal(t, wantEscalate, (*saved)[0].NeedsAttention)
				if link.wantLink {
					require.NotNil(t, result.ReviewURL, "every rating gets the link")
					assert.Equal(t, reviewURL, *result.ReviewURL)
				} else {
					assert.Nil(t, result.ReviewURL)
				}
			})
		}
	}
}

func TestFeedbackService_Record_AFiveStarAnswer(t *testing.T) {
	f := newFeedbackFixture(t, crmAt(t, 9, 12, 0), dto.FeedbackSettings{})
	saved := f.recording(t)

	result, err := f.service.Record(crmContext(), &dto.RecordFeedbackRequest{CustomerPhone: crmPhone, Rating: 5, Comment: strPtr(" Todo delicioso ")})

	require.NoError(t, err)
	assert.Equal(t, 5, result.Feedback.Rating)
	assert.Equal(t, "Todo delicioso", *(*saved)[0].Comment)
	body, err := json.Marshal(result)
	require.NoError(t, err)
	assert.NotContains(t, string(body), "review_url", "no link while it is off")
}

func TestFeedbackService_Record_TheGuestAddsACommentLater(t *testing.T) {
	f := newFeedbackFixture(t, crmAt(t, 9, 12, 0), dto.FeedbackSettings{})
	saved := f.recording(t)

	_, err := f.service.Record(crmContext(), &dto.RecordFeedbackRequest{CustomerPhone: crmPhone, Rating: 5})
	require.NoError(t, err)
	result, err := f.service.Record(crmContext(), &dto.RecordFeedbackRequest{CustomerPhone: crmPhone, Rating: 4, Comment: strPtr("La música muy alta")})

	require.NoError(t, err)
	require.Len(t, *saved, 2, "both answers land on the same request")
	assert.Equal(t, 4, result.Feedback.Rating)
	assert.Equal(t, "La música muy alta", *result.Feedback.Comment)
}

func TestFeedbackService_Record_NothingToAnswer(t *testing.T) {
	f := newFeedbackFixture(t, crmAt(t, 17, 12, 0), dto.FeedbackSettings{})
	f.feedback.EXPECT().FindLatestOpenRequest(mock.Anything, crmPhone, crmAt(t, 17, 12, 0)).Return(nil, domainError.ErrNoVisitAwaitingFeedback).Once()

	_, err := f.service.Record(crmContext(), &dto.RecordFeedbackRequest{CustomerPhone: crmPhone, Rating: 5})

	require.ErrorIs(t, err, domainError.ErrNoVisitAwaitingFeedback)
	f.feedback.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
}

func TestFeedbackService_Record_InvalidAnswer(t *testing.T) {
	tests := []struct {
		name  string
		req   dto.RecordFeedbackRequest
		field string
	}{
		{name: "zero", req: dto.RecordFeedbackRequest{CustomerPhone: crmPhone, Rating: 0}, field: "rating"},
		{name: "six", req: dto.RecordFeedbackRequest{CustomerPhone: crmPhone, Rating: 6}, field: "rating"},
		{name: "long comment", req: dto.RecordFeedbackRequest{CustomerPhone: crmPhone, Rating: 3, Comment: strPtr(strings.Repeat("c", 1001))}, field: "comment"},
		{name: "phone", req: dto.RecordFeedbackRequest{CustomerPhone: "12", Rating: 3}, field: "customer_phone"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFeedbackFixture(t, crmAt(t, 9, 12, 0), dto.FeedbackSettings{})
			f.feedback.EXPECT().FindLatestOpenRequest(mock.Anything, mock.Anything, mock.Anything).Return(&dto.FeedbackRequest{ID: "req-1"}, nil).Maybe()

			_, err := f.service.Record(crmContext(), &tt.req)

			var validation *domainError.ValidationError
			require.ErrorAs(t, err, &validation)
			assert.Equal(t, tt.field, validation.Field)
			f.feedback.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
		})
	}
}

func TestFeedbackService_Record_SaveFails(t *testing.T) {
	f := newFeedbackFixture(t, crmAt(t, 9, 12, 0), dto.FeedbackSettings{})
	f.feedback.EXPECT().FindLatestOpenRequest(mock.Anything, mock.Anything, mock.Anything).Return(&dto.FeedbackRequest{ID: "req-1"}, nil).Once()
	f.feedback.EXPECT().Save(mock.Anything, mock.Anything).Return(nil, errors.New("db down")).Once()

	_, err := f.service.Record(crmContext(), &dto.RecordFeedbackRequest{CustomerPhone: crmPhone, Rating: 2})

	require.ErrorIs(t, err, domainError.ErrFeedbackOperation)
}

func TestFeedbackService_List_ThisWeeksUnhappyGuests(t *testing.T) {
	// Sunday 8 November: the week began Monday 2 November.
	f := newFeedbackFixture(t, crmAt(t, 8, 12, 0), dto.FeedbackSettings{})
	f.feedback.EXPECT().List(mock.Anything, mock.MatchedBy(func(got dto.FeedbackListFilter) bool {
		return got.NeedsAttention && got.From.Equal(crmAt(t, 2, 0, 0)) && got.To.Equal(crmAt(t, 9, 0, 0))
	})).Return([]*dto.Feedback{{ID: "f1", Rating: 2, NeedsAttention: true}}, nil).Once()

	result, err := f.service.List(crmContext(), dto.FeedbackListQuery{NeedsAttention: true})

	require.NoError(t, err)
	assert.Equal(t, 1, result.Total)
}

func TestFeedbackService_List_ADateRange(t *testing.T) {
	f := newFeedbackFixture(t, crmAt(t, 8, 12, 0), dto.FeedbackSettings{})
	f.feedback.EXPECT().List(mock.Anything, mock.MatchedBy(func(got dto.FeedbackListFilter) bool {
		return !got.NeedsAttention && got.From.Equal(crmAt(t, 1, 0, 0)) && got.To.Equal(crmAt(t, 16, 0, 0))
	})).Return(nil, nil).Once()

	result, err := f.service.List(crmContext(), dto.FeedbackListQuery{From: "2026-11-01", To: "2026-11-15"})

	require.NoError(t, err)
	assert.NotNil(t, result.Feedback)
}

func TestFeedbackService_List_InvalidQuery(t *testing.T) {
	for _, tt := range []struct {
		query dto.FeedbackListQuery
		field string
	}{
		{query: dto.FeedbackListQuery{From: "1 Nov"}, field: "from"},
		{query: dto.FeedbackListQuery{To: "15 Nov"}, field: "to"},
		{query: dto.FeedbackListQuery{From: "2026-11-15", To: "2026-11-01"}, field: "to"},
	} {
		t.Run(tt.field, func(t *testing.T) {
			f := newFeedbackFixture(t, crmAt(t, 8, 12, 0), dto.FeedbackSettings{})
			_, err := f.service.List(crmContext(), tt.query)
			var validation *domainError.ValidationError
			require.ErrorAs(t, err, &validation)
			assert.Equal(t, tt.field, validation.Field)
		})
	}
}

func TestFeedbackService_List_RepositoryError(t *testing.T) {
	f := newFeedbackFixture(t, crmAt(t, 8, 12, 0), dto.FeedbackSettings{})
	f.feedback.EXPECT().List(mock.Anything, mock.Anything).Return(nil, errors.New("db down")).Once()

	_, err := f.service.List(crmContext(), dto.FeedbackListQuery{})

	require.ErrorIs(t, err, domainError.ErrFeedbackOperation)
}
