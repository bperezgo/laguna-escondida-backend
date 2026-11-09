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

type reservationFixture struct {
	t        *testing.T
	repo     *mocks.MockReservationRepository
	contacts *mocks.MockContactRecorder
	outbox   *txOutbox
	live     *liveFeed
	service  *ReservationService
	now      time.Time
	created  *dto.Reservation
	updated  *dto.Reservation
}

func newReservationFixture(t *testing.T, now time.Time) *reservationFixture {
	f := &reservationFixture{
		t:        t,
		repo:     mocks.NewMockReservationRepository(t),
		contacts: mocks.NewMockContactRecorder(t),
		outbox:   &txOutbox{},
		live:     &liveFeed{},
		now:      now,
	}
	f.service = NewReservationService(f.repo, f.contacts, f.outbox, crmSettings(t, dto.FeedbackSettings{}),
		newRecordingEvents(t, f.outbox), f.live, "CO", discardLogger(), fixedClock(now))
	return f
}

// acceptsCreate expects one contact touch and one stored reservation.
func (f *reservationFixture) acceptsCreate() {
	f.contacts.EXPECT().Touch(mock.Anything, mock.Anything).RunAndReturn(func(ctx context.Context, touch dto.ContactTouch) (*dto.Contact, error) {
		require.True(f.t, inUnitOfWork(ctx), "the contact is written with the reservation")
		return &dto.Contact{ID: "contact-1", Phone: touch.Phone}, nil
	}).Once()
	f.repo.EXPECT().Create(mock.Anything, mock.Anything).RunAndReturn(func(ctx context.Context, r *dto.Reservation) error {
		require.True(f.t, inUnitOfWork(ctx))
		f.created = r
		return nil
	}).Once()
}

// holds makes FindByID return r and accepts the update that moves it on.
func (f *reservationFixture) holds(r *dto.Reservation) {
	f.repo.EXPECT().FindByID(mock.Anything, r.ID).Return(r, nil).Maybe()
	f.repo.EXPECT().Update(mock.Anything, mock.Anything, r.Status).RunAndReturn(func(_ context.Context, u *dto.Reservation, _ dto.ReservationStatus) error {
		f.updated = u
		return nil
	}).Maybe()
}

func agentReservationRequest(now time.Time) *dto.CreateReservationRequest {
	return &dto.CreateReservationRequest{
		CustomerPhone: crmPhone,
		Name:          "Ana",
		PartySize:     6,
		StartsAt:      now.Add(24 * time.Hour).Format(time.RFC3339),
		Source:        dto.ReservationSourceWhatsApp,
		Origin: &dto.OriginRequest{ChannelID: strPtr(crmChannel), ConversationID: strPtr(crmConvID), Attribution: &dto.AttributionRequest{
			Source: strPtr("meta_ctwa"), AdID: strPtr("120210000000001"), ClickedAt: strPtr(now.Add(-time.Hour).Format(time.RFC3339)),
		}},
	}
}

func storedReservation(status dto.ReservationStatus, startsAt time.Time) *dto.Reservation {
	return &dto.Reservation{
		ID: "r1", ContactID: "contact-1", ContactPhone: crmPhone, Name: "Ana", PartySize: 6, StartsAt: startsAt,
		Source: dto.ReservationSourceWhatsApp, Status: status,
		Origin: dto.Origin{ChannelID: strPtr(crmChannel), ConversationID: strPtr(crmConvID)},
	}
}

func TestReservationService_Create_FromTheAgentIsRequested(t *testing.T) {
	now := crmAt(t, 8, 15, 0)
	f := newReservationFixture(t, now)
	f.acceptsCreate()

	result, err := f.service.Create(crmContext(), agentReservationRequest(now))

	require.NoError(t, err)
	assert.Equal(t, dto.ReservationStatusRequested, result.Status)
	assert.Equal(t, dto.ReservationSourceWhatsApp, result.Source)
	assert.Equal(t, crmPhone, result.ContactPhone)
	assert.Equal(t, "contact-1", result.ContactID)
	assert.Equal(t, crmChannel, *result.Origin.ChannelID)
	assert.Equal(t, crmConvID, *result.Origin.ConversationID)
	assert.Equal(t, "120210000000001", *result.Origin.Attribution.AdID)
	assert.Same(t, f.created, result)
	assert.Empty(t, f.outbox.events(), "a request is not reported until staff answer it")
}

func TestReservationService_Create_TouchesTheContactWithNameAndAttribution(t *testing.T) {
	now := crmAt(t, 8, 15, 0)
	f := newReservationFixture(t, now)
	f.contacts.EXPECT().Touch(mock.Anything, mock.MatchedBy(func(touch dto.ContactTouch) bool {
		return touch.Phone == crmPhone && *touch.Name == "Ana" && *touch.Attribution.Source == "meta_ctwa" && touch.At.Equal(now)
	})).Return(&dto.Contact{ID: "contact-1"}, nil).Once()
	f.repo.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Once()

	_, err := f.service.Create(crmContext(), agentReservationRequest(now))

	require.NoError(t, err)
}

func TestReservationService_Create_StaffBookingsAreConfirmed(t *testing.T) {
	for _, source := range []dto.ReservationSource{dto.ReservationSourcePhone, dto.ReservationSourceWalkIn, dto.ReservationSourceStaff} {
		t.Run(string(source), func(t *testing.T) {
			now := crmAt(t, 8, 15, 0)
			f := newReservationFixture(t, now)
			f.acceptsCreate()
			req := agentReservationRequest(now)
			req.Source, req.Origin, req.CustomerPhone = source, nil, "300 555 1234"

			result, err := f.service.Create(crmContext(), req)

			require.NoError(t, err)
			assert.Equal(t, dto.ReservationStatusConfirmed, result.Status)
			assert.Equal(t, crmPhone, result.ContactPhone, "a national number is read in the home region")
			assert.Empty(t, f.outbox.events(), "no channel, nothing to tell marketing-app")
		})
	}
}

func TestReservationService_Create_InvalidInputStoresNothing(t *testing.T) {
	now := crmAt(t, 8, 15, 0)
	tests := []struct {
		name  string
		edit  func(*dto.CreateReservationRequest)
		field string
	}{
		{name: "party too large", edit: func(r *dto.CreateReservationRequest) { r.PartySize = 80 }, field: "party_size"},
		{name: "party empty", edit: func(r *dto.CreateReservationRequest) { r.PartySize = 0 }, field: "party_size"},
		{name: "an hour ago", edit: func(r *dto.CreateReservationRequest) { r.StartsAt = now.Add(-time.Hour).Format(time.RFC3339) }, field: "starts_at"},
		{name: "61 days ahead", edit: func(r *dto.CreateReservationRequest) { r.StartsAt = now.AddDate(0, 0, 61).Format(time.RFC3339) }, field: "starts_at"},
		{name: "no offset", edit: func(r *dto.CreateReservationRequest) { r.StartsAt = "2026-11-09T20:00:00" }, field: "starts_at"},
		{name: "blank name", edit: func(r *dto.CreateReservationRequest) { r.Name = "   " }, field: "name"},
		{name: "long name", edit: func(r *dto.CreateReservationRequest) { r.Name = strings.Repeat("a", 101) }, field: "name"},
		{name: "long notes", edit: func(r *dto.CreateReservationRequest) { r.Notes = strPtr(strings.Repeat("n", 501)) }, field: "notes"},
		{name: "unknown source", edit: func(r *dto.CreateReservationRequest) { r.Source = "instagram" }, field: "source"},
		{name: "bad phone", edit: func(r *dto.CreateReservationRequest) { r.CustomerPhone = "12" }, field: "customer_phone"},
		{name: "long channel", edit: func(r *dto.CreateReservationRequest) { r.Origin.ChannelID = strPtr(strings.Repeat("c", 129)) }, field: "origin.channel_id"},
		{name: "long conversation", edit: func(r *dto.CreateReservationRequest) { r.Origin.ConversationID = strPtr(strings.Repeat("c", 129)) }, field: "origin.conversation_id"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newReservationFixture(t, now)
			req := agentReservationRequest(now)
			tt.edit(req)

			_, err := f.service.Create(crmContext(), req)

			var validation *domainError.ValidationError
			require.ErrorAs(t, err, &validation)
			assert.Equal(t, tt.field, validation.Field)
			f.contacts.AssertNotCalled(t, "Touch", mock.Anything, mock.Anything)
			f.repo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
		})
	}
}

func TestReservationService_Create_InvalidAttributionIsDroppedNotFatal(t *testing.T) {
	now := crmAt(t, 8, 15, 0)
	tests := []struct {
		name  string
		edit  func(*dto.AttributionRequest)
		check func(t *testing.T, a *dto.Attribution)
	}{
		{name: "a garbled click time", edit: func(a *dto.AttributionRequest) { a.ClickedAt = strPtr("yesterday") }, check: func(t *testing.T, a *dto.Attribution) {
			assert.Nil(t, a.ClickedAt)
			assert.Equal(t, "meta_ctwa", *a.Source)
		}},
		{name: "a click time ten minutes ahead", edit: func(a *dto.AttributionRequest) { a.ClickedAt = strPtr(now.Add(10 * time.Minute).Format(time.RFC3339)) }, check: func(t *testing.T, a *dto.Attribution) {
			assert.Nil(t, a.ClickedAt)
		}},
		{name: "a click time within the clock skew", edit: func(a *dto.AttributionRequest) { a.ClickedAt = strPtr(now.Add(2 * time.Minute).Format(time.RFC3339)) }, check: func(t *testing.T, a *dto.Attribution) {
			assert.NotNil(t, a.ClickedAt)
		}},
		{name: "an upper-case source", edit: func(a *dto.AttributionRequest) { a.Source = strPtr("Meta CTWA") }, check: func(t *testing.T, a *dto.Attribution) {
			assert.Nil(t, a.Source)
			assert.Equal(t, "120210000000001", *a.AdID)
		}},
		{name: "a long ad id", edit: func(a *dto.AttributionRequest) { a.AdID = strPtr(strings.Repeat("9", 65)) }, check: func(t *testing.T, a *dto.Attribution) {
			assert.Nil(t, a.AdID)
		}},
		{name: "a long campaign id", edit: func(a *dto.AttributionRequest) { a.CampaignID = strPtr(strings.Repeat("9", 65)) }, check: func(t *testing.T, a *dto.Attribution) {
			assert.Nil(t, a.CampaignID)
		}},
		{name: "a long campaign name", edit: func(a *dto.AttributionRequest) { a.CampaignName = strPtr(strings.Repeat("x", 256)) }, check: func(t *testing.T, a *dto.Attribution) {
			assert.Nil(t, a.CampaignName)
		}},
		{name: "a campaign kept", edit: func(a *dto.AttributionRequest) { a.CampaignID, a.CampaignName = strPtr("c-1"), strPtr("Navidad") }, check: func(t *testing.T, a *dto.Attribution) {
			assert.Equal(t, "c-1", *a.CampaignID)
			assert.Equal(t, "Navidad", *a.CampaignName)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newReservationFixture(t, now)
			f.acceptsCreate()
			req := agentReservationRequest(now)
			tt.edit(req.Origin.Attribution)

			result, err := f.service.Create(crmContext(), req)

			require.NoError(t, err, "the booking is never failed by attribution")
			require.NotNil(t, result.Origin.Attribution)
			tt.check(t, result.Origin.Attribution)
		})
	}
}

func TestReservationService_Create_AllAttributionInvalidLeavesNone(t *testing.T) {
	now := crmAt(t, 8, 15, 0)
	f := newReservationFixture(t, now)
	f.acceptsCreate()
	req := agentReservationRequest(now)
	req.Origin.Attribution = &dto.AttributionRequest{Source: strPtr("!!"), ClickedAt: strPtr("soon")}

	result, err := f.service.Create(crmContext(), req)

	require.NoError(t, err)
	assert.Nil(t, result.Origin.Attribution)
}

func TestReservationService_Create_AClickIdentifierIsIgnored(t *testing.T) {
	now := crmAt(t, 8, 15, 0)
	f := newReservationFixture(t, now)
	f.acceptsCreate()
	var req dto.CreateReservationRequest
	require.NoError(t, json.Unmarshal([]byte(`{
		"customer_phone": "+573005551234", "name": "Ana", "party_size": 2, "source": "whatsapp",
		"starts_at": "`+now.Add(48*time.Hour).Format(time.RFC3339)+`",
		"origin": {"channel_id": "ch_1", "ctwa_clid": "ARAkLkA8rmlFeiCktEJQ", "attribution": {"source": "meta_ctwa", "ctwa_clid": "ARAkLkA8rmlFeiCktEJQ"}}
	}`), &req))

	result, err := f.service.Create(crmContext(), &req)

	require.NoError(t, err)
	stored, err := json.Marshal(f.created)
	require.NoError(t, err)
	assert.NotContains(t, string(stored), "ARAkLkA8rmlFeiCktEJQ")
	assert.Equal(t, "meta_ctwa", *result.Origin.Attribution.Source)
}

func TestReservationService_Create_Failures(t *testing.T) {
	now := crmAt(t, 8, 15, 0)
	t.Run("contact", func(t *testing.T) {
		f := newReservationFixture(t, now)
		f.contacts.EXPECT().Touch(mock.Anything, mock.Anything).Return(nil, errors.New("db down")).Once()
		_, err := f.service.Create(crmContext(), agentReservationRequest(now))
		require.ErrorIs(t, err, domainError.ErrReservationOperation)
	})
	t.Run("store", func(t *testing.T) {
		f := newReservationFixture(t, now)
		f.contacts.EXPECT().Touch(mock.Anything, mock.Anything).Return(&dto.Contact{ID: "contact-1"}, nil).Once()
		f.repo.EXPECT().Create(mock.Anything, mock.Anything).Return(errors.New("db down")).Once()
		_, err := f.service.Create(crmContext(), agentReservationRequest(now))
		require.ErrorIs(t, err, domainError.ErrReservationOperation)
	})
}

type reservationAction struct {
	name  string
	apply func(s *ReservationService, ctx context.Context, id string) (*dto.Reservation, error)
	to    dto.ReservationStatus
}

var reservationActions = map[string]reservationAction{
	"confirm": {name: "confirm", to: dto.ReservationStatusConfirmed, apply: func(s *ReservationService, ctx context.Context, id string) (*dto.Reservation, error) {
		return s.Confirm(ctx, id)
	}},
	"decline": {name: "decline", to: dto.ReservationStatusDeclined, apply: func(s *ReservationService, ctx context.Context, id string) (*dto.Reservation, error) {
		return s.Decline(ctx, id, strPtr("Fully booked"))
	}},
	"cancel": {name: "cancel", to: dto.ReservationStatusCancelled, apply: func(s *ReservationService, ctx context.Context, id string) (*dto.Reservation, error) {
		return s.CancelByStaff(ctx, id)
	}},
	"no_show": {name: "no_show", to: dto.ReservationStatusNoShow, apply: func(s *ReservationService, ctx context.Context, id string) (*dto.Reservation, error) {
		return s.MarkNoShow(ctx, id)
	}},
}

func TestReservationService_Transitions(t *testing.T) {
	tests := []struct {
		from              dto.ReservationStatus
		feedbackRequested bool
		allowed           []string
	}{
		{from: dto.ReservationStatusRequested, allowed: []string{"confirm", "decline", "cancel"}},
		{from: dto.ReservationStatusConfirmed, allowed: []string{"cancel", "no_show"}},
		{from: dto.ReservationStatusVisited, allowed: []string{"no_show"}},
		{from: dto.ReservationStatusVisited, feedbackRequested: true},
		{from: dto.ReservationStatusDeclined},
		{from: dto.ReservationStatusCancelled},
		{from: dto.ReservationStatusNoShow},
	}
	for _, tt := range tests {
		for _, action := range reservationActions {
			allowed := false
			for _, a := range tt.allowed {
				allowed = allowed || a == action.name
			}
			name := string(tt.from) + " " + action.name
			if tt.feedbackRequested {
				name += " after feedback was requested"
			}
			t.Run(name, func(t *testing.T) {
				now := crmAt(t, 8, 15, 0)
				f := newReservationFixture(t, now)
				current := storedReservation(tt.from, now.Add(24*time.Hour))
				current.FeedbackRequested = tt.feedbackRequested
				f.holds(current)

				result, err := action.apply(f.service, crmContext(), current.ID)

				if allowed {
					require.NoError(t, err)
					assert.Equal(t, action.to, result.Status)
					assert.Equal(t, now, result.UpdatedAt)
					return
				}
				var transition *domainError.ReservationTransitionError
				require.ErrorAs(t, err, &transition)
				assert.ErrorIs(t, err, domainError.ErrReservationTransition)
				assert.Equal(t, string(tt.from), transition.From)
				assert.Equal(t, string(action.to), transition.To)
				assert.Nil(t, f.updated, "nothing changes")
				assert.Empty(t, f.outbox.events())
			})
		}
	}
}

func TestReservationService_Transitions_EmitTheirEvents(t *testing.T) {
	tests := []struct {
		action      string
		from        dto.ReservationStatus
		wantType    dto.MarketingEventType
		wantInData  map[string]string
		wantNoEvent bool
	}{
		{action: "confirm", from: dto.ReservationStatusRequested, wantType: dto.MarketingEventReservationConfirmed, wantInData: map[string]string{"status": "confirmed"}},
		{action: "decline", from: dto.ReservationStatusRequested, wantType: dto.MarketingEventReservationDeclined, wantInData: map[string]string{"status": "declined", "reason": "Fully booked"}},
		{action: "cancel", from: dto.ReservationStatusConfirmed, wantType: dto.MarketingEventReservationCancelled, wantInData: map[string]string{"status": "cancelled", "cancelled_by": "staff"}},
		{action: "no_show", from: dto.ReservationStatusConfirmed, wantNoEvent: true},
	}
	for _, tt := range tests {
		t.Run(tt.action, func(t *testing.T) {
			now := crmAt(t, 8, 15, 0)
			f := newReservationFixture(t, now)
			f.holds(storedReservation(tt.from, now.Add(24*time.Hour)))

			_, err := reservationActions[tt.action].apply(f.service, crmContext(), "r1")

			require.NoError(t, err)
			events := f.outbox.events()
			if tt.wantNoEvent {
				assert.Empty(t, events)
				return
			}
			require.Len(t, events, 1)
			assert.Equal(t, tt.wantType, events[0].Type)
			assert.Equal(t, dto.MarketingEventsPathNotifications, events[0].Path)
			var body struct {
				Data map[string]any `json:"data"`
			}
			require.NoError(t, json.Unmarshal(events[0].Body, &body))
			for key, want := range tt.wantInData {
				assert.Equal(t, want, body.Data[key], key)
			}
		})
	}
}

func TestReservationService_Transitions_NoChannelNoEvent(t *testing.T) {
	now := crmAt(t, 8, 15, 0)
	f := newReservationFixture(t, now)
	current := storedReservation(dto.ReservationStatusRequested, now.Add(24*time.Hour))
	current.Origin = dto.Origin{}
	f.holds(current)

	_, err := f.service.Confirm(crmContext(), "r1")

	require.NoError(t, err)
	assert.Empty(t, f.outbox.events())
}

func TestReservationService_Transitions_LostRaceIsAConflict(t *testing.T) {
	now := crmAt(t, 8, 15, 0)
	f := newReservationFixture(t, now)
	f.repo.EXPECT().FindByID(mock.Anything, "r1").Return(storedReservation(dto.ReservationStatusRequested, now.Add(time.Hour)), nil).Once()
	f.repo.EXPECT().Update(mock.Anything, mock.Anything, dto.ReservationStatusRequested).Return(domainError.ErrReservationTransition).Once()

	_, err := f.service.Confirm(crmContext(), "r1")

	require.ErrorIs(t, err, domainError.ErrReservationTransition)
	assert.Empty(t, f.outbox.events(), "the event rolls back with the change")
}

func TestReservationService_Transitions_Failures(t *testing.T) {
	now := crmAt(t, 8, 15, 0)
	t.Run("not found", func(t *testing.T) {
		f := newReservationFixture(t, now)
		f.repo.EXPECT().FindByID(mock.Anything, "nope").Return(nil, domainError.ErrReservationNotFound).Once()
		_, err := f.service.Confirm(crmContext(), "nope")
		require.ErrorIs(t, err, domainError.ErrReservationNotFound)
	})
	t.Run("store", func(t *testing.T) {
		f := newReservationFixture(t, now)
		f.repo.EXPECT().FindByID(mock.Anything, "r1").Return(storedReservation(dto.ReservationStatusRequested, now.Add(time.Hour)), nil).Once()
		f.repo.EXPECT().Update(mock.Anything, mock.Anything, mock.Anything).Return(errors.New("db down")).Once()
		_, err := f.service.Confirm(crmContext(), "r1")
		require.ErrorIs(t, err, domainError.ErrReservationOperation)
	})
	t.Run("event", func(t *testing.T) {
		f := newReservationFixture(t, now)
		f.outbox.appendErr = errors.New("outbox down")
		f.holds(storedReservation(dto.ReservationStatusRequested, now.Add(time.Hour)))
		_, err := f.service.Confirm(crmContext(), "r1")
		require.ErrorIs(t, err, domainError.ErrMarketingEventRecordError)
	})
	t.Run("long decline reason", func(t *testing.T) {
		f := newReservationFixture(t, now)
		f.holds(storedReservation(dto.ReservationStatusRequested, now.Add(time.Hour)))
		_, err := f.service.Decline(crmContext(), "r1", strPtr(strings.Repeat("r", 256)))
		var validation *domainError.ValidationError
		require.ErrorAs(t, err, &validation)
		assert.Equal(t, "reason", validation.Field)
		assert.Nil(t, f.updated)
	})
}

func TestReservationService_CorrectingANoShowTheNextMorning(t *testing.T) {
	now := crmAt(t, 9, 8, 0)
	f := newReservationFixture(t, now)
	visitedAt := crmAt(t, 8, 22, 15)
	current := storedReservation(dto.ReservationStatusVisited, crmAt(t, 8, 20, 0))
	current.VisitedAt = &visitedAt
	f.holds(current)

	result, err := f.service.MarkNoShow(crmContext(), "r1")

	require.NoError(t, err)
	assert.Equal(t, dto.ReservationStatusNoShow, result.Status)
}

func TestReservationService_CancelByCustomer(t *testing.T) {
	now := crmAt(t, 8, 15, 0)
	tests := []struct {
		name     string
		stored   *dto.Reservation
		phone    string
		wantOK   bool
		wantByCx bool
	}{
		{name: "tomorrow's confirmed table", stored: storedReservation(dto.ReservationStatusConfirmed, now.Add(24*time.Hour)), phone: "573005551234", wantOK: true},
		{name: "a pending request", stored: storedReservation(dto.ReservationStatusRequested, now.Add(time.Hour)), phone: crmPhone, wantOK: true},
		{name: "another guest's reservation", stored: storedReservation(dto.ReservationStatusConfirmed, now.Add(24*time.Hour)), phone: "+573009998877"},
		{name: "already started", stored: storedReservation(dto.ReservationStatusConfirmed, now.Add(-time.Minute)), phone: crmPhone},
		{name: "already declined", stored: storedReservation(dto.ReservationStatusDeclined, now.Add(24*time.Hour)), phone: crmPhone},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newReservationFixture(t, now)
			f.holds(tt.stored)

			result, err := f.service.CancelByCustomer(crmContext(), "r1", tt.phone)

			if !tt.wantOK {
				require.ErrorIs(t, err, domainError.ErrReservationNotFound, "the same answer as an unknown id")
				assert.NotErrorIs(t, err, domainError.ErrReservationTransition)
				assert.Nil(t, f.updated, "unchanged")
				assert.Empty(t, f.outbox.events())
				return
			}
			require.NoError(t, err)
			assert.Equal(t, dto.ReservationStatusCancelled, result.Status)
			assert.Equal(t, dto.CancelledByCustomer, *result.CancelledBy)
			events := f.outbox.events()
			require.Len(t, events, 1)
			assert.Equal(t, dto.MarketingEventReservationCancelled, events[0].Type)
			assert.Contains(t, string(events[0].Body), `"cancelled_by":"customer"`)
		})
	}
}

func TestReservationService_CancelByCustomer_UnknownIDAndBadPhone(t *testing.T) {
	now := crmAt(t, 8, 15, 0)
	f := newReservationFixture(t, now)
	f.repo.EXPECT().FindByID(mock.Anything, "nope").Return(nil, domainError.ErrReservationNotFound).Once()

	_, err := f.service.CancelByCustomer(crmContext(), "nope", crmPhone)
	require.ErrorIs(t, err, domainError.ErrReservationNotFound)

	_, err = f.service.CancelByCustomer(crmContext(), "r1", "12")
	var validation *domainError.ValidationError
	require.ErrorAs(t, err, &validation)
}

func TestReservationService_Get(t *testing.T) {
	f := newReservationFixture(t, crmAt(t, 8, 15, 0))
	want := storedReservation(dto.ReservationStatusRequested, time.Now())
	f.repo.EXPECT().FindByID(mock.Anything, "r1").Return(want, nil).Once()

	got, err := f.service.Get(crmContext(), "r1")

	require.NoError(t, err)
	assert.Same(t, want, got)
}

func TestReservationService_List_Filters(t *testing.T) {
	now := crmAt(t, 8, 15, 0)
	from := crmAt(t, 7, 15, 0)
	tests := []struct {
		name  string
		query dto.ReservationListQuery
		want  dto.ReservationListFilter
	}{
		{name: "tonight by default", query: dto.ReservationListQuery{}, want: dto.ReservationListFilter{From: ptrTime(crmAt(t, 8, 0, 0)), To: ptrTime(crmAt(t, 9, 0, 0))}},
		{name: "a local day", query: dto.ReservationListQuery{Date: "2026-11-24"}, want: dto.ReservationListFilter{From: ptrTime(crmAt(t, 24, 0, 0)), To: ptrTime(crmAt(t, 25, 0, 0))}},
		{name: "by status", query: dto.ReservationListQuery{Date: "2026-11-08", Status: "requested, confirmed"}, want: dto.ReservationListFilter{
			From: ptrTime(crmAt(t, 8, 0, 0)), To: ptrTime(crmAt(t, 9, 0, 0)),
			Statuses: []dto.ReservationStatus{dto.ReservationStatusRequested, dto.ReservationStatusConfirmed},
		}},
		{name: "a guest's from a time on", query: dto.ReservationListQuery{Phone: "573005551234", From: from.Format(time.RFC3339)}, want: dto.ReservationListFilter{
			Phone: crmPhone, From: &from,
		}},
		{name: "a guest's on a day", query: dto.ReservationListQuery{Phone: crmPhone, Date: "2026-11-08"}, want: dto.ReservationListFilter{
			Phone: crmPhone, From: ptrTime(crmAt(t, 8, 0, 0)), To: ptrTime(crmAt(t, 9, 0, 0)),
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newReservationFixture(t, now)
			f.repo.EXPECT().List(mock.Anything, mock.MatchedBy(func(got dto.ReservationListFilter) bool {
				return sameReservationFilter(got, tt.want)
			})).Return([]*dto.Reservation{{ID: "r1"}}, nil).Once()

			result, err := f.service.List(crmContext(), tt.query)

			require.NoError(t, err)
			assert.Equal(t, 1, result.Total)
		})
	}
}

func TestReservationService_List_InvalidQuery(t *testing.T) {
	tests := []struct {
		query dto.ReservationListQuery
		field string
	}{
		{query: dto.ReservationListQuery{Status: "seated"}, field: "status"},
		{query: dto.ReservationListQuery{Date: "08/11/2026"}, field: "date"},
		{query: dto.ReservationListQuery{Phone: "12"}, field: "phone"},
		{query: dto.ReservationListQuery{Phone: crmPhone, From: "yesterday"}, field: "from"},
	}
	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			f := newReservationFixture(t, crmAt(t, 8, 15, 0))

			_, err := f.service.List(crmContext(), tt.query)

			var validation *domainError.ValidationError
			require.ErrorAs(t, err, &validation)
			assert.Equal(t, tt.field, validation.Field)
		})
	}
}

func TestReservationService_List_EmptyAndFailing(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		f := newReservationFixture(t, crmAt(t, 8, 15, 0))
		f.repo.EXPECT().List(mock.Anything, mock.Anything).Return(nil, nil).Once()
		result, err := f.service.List(crmContext(), dto.ReservationListQuery{})
		require.NoError(t, err)
		assert.NotNil(t, result.Reservations)
	})
	t.Run("repository", func(t *testing.T) {
		f := newReservationFixture(t, crmAt(t, 8, 15, 0))
		f.repo.EXPECT().List(mock.Anything, mock.Anything).Return(nil, errors.New("db down")).Once()
		_, err := f.service.List(crmContext(), dto.ReservationListQuery{})
		require.ErrorIs(t, err, domainError.ErrReservationOperation)
	})
	t.Run("timezone", func(t *testing.T) {
		settings := mocks.NewMockOrganizationSettingsRepository(t)
		settings.EXPECT().Get(mock.Anything).Return(&dto.OrganizationSettingsView{Timezone: "Mars/Olympus"}, nil).Once()
		svc := NewReservationService(mocks.NewMockReservationRepository(t), mocks.NewMockContactRecorder(t), &txOutbox{}, settings,
			newRecordingEvents(t, &txOutbox{}), nil, "CO", discardLogger(), nil)
		_, err := svc.List(crmContext(), dto.ReservationListQuery{})
		require.ErrorIs(t, err, domainError.ErrReservationOperation)
	})
}

// visitedFixture stores reservations and answers ListConfirmedStartedBefore the way the repository does.
func visitedFixture(t *testing.T, now time.Time, stored ...*dto.Reservation) (*reservationFixture, map[string]dto.ReservationStatus) {
	f := newReservationFixture(t, now)
	statuses := map[string]dto.ReservationStatus{}
	f.repo.EXPECT().ListConfirmedStartedBefore(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, cutoff time.Time) ([]*dto.Reservation, error) {
		var due []*dto.Reservation
		for _, r := range stored {
			if r.Status == dto.ReservationStatusConfirmed && r.StartsAt.Before(cutoff) {
				due = append(due, r)
			}
		}
		return due, nil
	}).Once()
	f.repo.EXPECT().Update(mock.Anything, mock.Anything, dto.ReservationStatusConfirmed).RunAndReturn(func(_ context.Context, r *dto.Reservation, _ dto.ReservationStatus) error {
		statuses[r.ID] = r.Status
		require.NotNil(t, r.VisitedAt)
		return nil
	}).Maybe()
	return f, statuses
}

func TestReservationService_MarkVisited_TwoHoursAfterItStarts(t *testing.T) {
	tests := []struct {
		name        string
		now         time.Time
		wantVisited bool
	}{
		{name: "dinner at 8 pm, now 22:15", now: crmAt(t, 8, 22, 15), wantVisited: true},
		{name: "dinner at 8 pm, now 21:59", now: crmAt(t, 8, 21, 59), wantVisited: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dinner := storedReservation(dto.ReservationStatusConfirmed, crmAt(t, 8, 20, 0))
			f, statuses := visitedFixture(t, tt.now, dinner)

			marked, err := f.service.MarkVisited(crmContext())

			require.NoError(t, err)
			if tt.wantVisited {
				assert.Equal(t, 1, marked)
				assert.Equal(t, dto.ReservationStatusVisited, statuses["r1"])
				return
			}
			assert.Zero(t, marked)
			assert.Empty(t, statuses)
		})
	}
}

func TestReservationService_MarkVisited_LeavesARequestAlone(t *testing.T) {
	never := storedReservation(dto.ReservationStatusRequested, crmAt(t, 8, 20, 0))
	f, statuses := visitedFixture(t, crmAt(t, 8, 23, 0), never)

	marked, err := f.service.MarkVisited(crmContext())

	require.NoError(t, err)
	assert.Zero(t, marked)
	assert.Empty(t, statuses)
	assert.Empty(t, f.outbox.events(), "a visit is not reported")
}

func TestReservationService_MarkVisited_SkipsWhatStaffChangedMeanwhile(t *testing.T) {
	f := newReservationFixture(t, crmAt(t, 8, 23, 0))
	f.repo.EXPECT().ListConfirmedStartedBefore(mock.Anything, mock.Anything).Return([]*dto.Reservation{
		storedReservation(dto.ReservationStatusConfirmed, crmAt(t, 8, 20, 0)),
		// A stale row the repository should not return; the aggregate still refuses it.
		storedReservation(dto.ReservationStatusCancelled, crmAt(t, 8, 20, 0)),
	}, nil).Once()
	f.repo.EXPECT().Update(mock.Anything, mock.Anything, dto.ReservationStatusConfirmed).Return(domainError.ErrReservationTransition).Once()

	marked, err := f.service.MarkVisited(crmContext())

	require.NoError(t, err)
	assert.Zero(t, marked)
}

func TestReservationService_MarkVisited_Failures(t *testing.T) {
	t.Run("list", func(t *testing.T) {
		f := newReservationFixture(t, crmAt(t, 8, 23, 0))
		f.repo.EXPECT().ListConfirmedStartedBefore(mock.Anything, mock.Anything).Return(nil, errors.New("db down")).Once()
		_, err := f.service.MarkVisited(crmContext())
		require.ErrorIs(t, err, domainError.ErrReservationOperation)
	})
	t.Run("update", func(t *testing.T) {
		f := newReservationFixture(t, crmAt(t, 8, 23, 0))
		f.repo.EXPECT().ListConfirmedStartedBefore(mock.Anything, mock.Anything).Return([]*dto.Reservation{
			storedReservation(dto.ReservationStatusConfirmed, crmAt(t, 8, 20, 0)),
		}, nil).Once()
		f.repo.EXPECT().Update(mock.Anything, mock.Anything, mock.Anything).Return(errors.New("db down")).Once()
		_, err := f.service.MarkVisited(crmContext())
		require.ErrorIs(t, err, domainError.ErrReservationOperation)
	})
}

func ptrTime(t time.Time) *time.Time { return &t }

func sameReservationFilter(got, want dto.ReservationListFilter) bool {
	sameTime := func(a, b *time.Time) bool { return (a == nil) == (b == nil) && (a == nil || a.Equal(*b)) }
	if !sameTime(got.From, want.From) || !sameTime(got.To, want.To) || got.Phone != want.Phone || got.ContactID != want.ContactID {
		return false
	}
	if len(got.Statuses) != len(want.Statuses) {
		return false
	}
	for i := range got.Statuses {
		if got.Statuses[i] != want.Statuses[i] {
			return false
		}
	}
	return true
}
