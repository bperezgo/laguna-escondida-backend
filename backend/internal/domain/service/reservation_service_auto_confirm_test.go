package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// autoConfirmPolicy is the defaults with the policy on and a cap of 120.
func autoConfirmPolicy() dto.ReservationSettings {
	policy := reservationSettingsDefaults()
	policy.AutoConfirmEnabled = true
	policy.DailyGuestCap = intPtr(120)
	return policy
}

func newAutoConfirmFixture(t *testing.T, now time.Time, policy dto.ReservationSettings) *reservationFixture {
	f := &reservationFixture{
		t:        t,
		repo:     mocks.NewMockReservationRepository(t),
		contacts: mocks.NewMockContactRecorder(t),
		outbox:   &txOutbox{},
		live:     &liveFeed{},
		now:      now,
	}
	settings := mocks.NewMockOrganizationSettingsRepository(t)
	settings.EXPECT().Get(mock.Anything).Return(&dto.OrganizationSettingsView{Timezone: crmTimezone, Reservations: policy}, nil).Maybe()
	f.service = NewReservationService(f.repo, f.contacts, f.outbox, settings,
		newRecordingEvents(t, f.outbox), f.live, "CO", discardLogger(), fixedClock(now))
	return f
}

// countsGuests answers the day's count with guests, and checks it asks for one Bogotá calendar day
// from inside the create's unit of work.
func (f *reservationFixture) countsGuests(guests int, day int) {
	f.repo.EXPECT().LockDayAndCountGuests(mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(ctx context.Context, dayStart, dayEnd time.Time) (int, error) {
			require.True(f.t, inUnitOfWork(ctx), "the day stays locked until the reservation is stored")
			assert.True(f.t, dayStart.Equal(crmAt(f.t, day, 0, 0)), "day starts at local midnight, got %s", dayStart)
			assert.True(f.t, dayEnd.Equal(crmAt(f.t, day+1, 0, 0)), "day ends at the next local midnight, got %s", dayEnd)
			return guests, nil
		}).Once()
}

func agentRequestFor(now time.Time, partySize int, startsAt time.Time) *dto.CreateReservationRequest {
	req := agentReservationRequest(now)
	req.PartySize = partySize
	req.StartsAt = startsAt.Format(time.RFC3339)
	return req
}

func TestReservationService_Create_AutoConfirmPolicy(t *testing.T) {
	// Sunday 8 November 2026, 15:00 in Bogotá; the 14th is a Saturday.
	now := crmAt(t, 8, 15, 0)
	saturdayOnly := autoConfirmPolicy()
	saturdayOnly.Schedule = []dto.OpeningWindow{{Day: "sat", Opens: "12:00", Closes: "23:00"}}
	fridayNight := autoConfirmPolicy()
	fridayNight.Schedule = []dto.OpeningWindow{{Day: "fri", Opens: "18:00", Closes: "02:00"}}
	disabled := reservationSettingsDefaults()

	tests := []struct {
		name       string
		policy     dto.ReservationSettings
		partySize  int
		startsAt   time.Time
		guests     *int
		countDay   int
		wantStatus dto.ReservationStatus
		wantReason dto.ReservationPendingReason
	}{
		{name: "an ordinary table", policy: autoConfirmPolicy(), partySize: 4, startsAt: crmAt(t, 9, 12, 0),
			guests: intPtr(40), countDay: 9, wantStatus: dto.ReservationStatusConfirmed},
		{name: "a large party", policy: autoConfirmPolicy(), partySize: 10, startsAt: crmAt(t, 9, 12, 0),
			wantStatus: dto.ReservationStatusRequested, wantReason: dto.PendingReasonPartySize},
		{name: "exactly the largest party", policy: autoConfirmPolicy(), partySize: 8, startsAt: crmAt(t, 9, 12, 0),
			guests: intPtr(0), countDay: 9, wantStatus: dto.ReservationStatusConfirmed},
		{name: "too little notice", policy: autoConfirmPolicy(), partySize: 4, startsAt: now.Add(90 * time.Minute),
			wantStatus: dto.ReservationStatusRequested, wantReason: dto.PendingReasonNotice},
		{name: "exactly the notice", policy: autoConfirmPolicy(), partySize: 4, startsAt: now.Add(120 * time.Minute),
			guests: intPtr(0), countDay: 8, wantStatus: dto.ReservationStatusConfirmed},
		{name: "outside the schedule", policy: saturdayOnly, partySize: 4, startsAt: crmAt(t, 9, 12, 0),
			wantStatus: dto.ReservationStatusRequested, wantReason: dto.PendingReasonOutsideSchedule},
		{name: "inside the schedule", policy: saturdayOnly, partySize: 4, startsAt: crmAt(t, 14, 20, 0),
			guests: intPtr(0), countDay: 14, wantStatus: dto.ReservationStatusConfirmed},
		{name: "inside a window past midnight", policy: fridayNight, partySize: 4, startsAt: crmAt(t, 14, 1, 0),
			guests: intPtr(0), countDay: 14, wantStatus: dto.ReservationStatusConfirmed},
		{name: "after a window past midnight closes", policy: fridayNight, partySize: 4, startsAt: crmAt(t, 14, 3, 0),
			wantStatus: dto.ReservationStatusRequested, wantReason: dto.PendingReasonOutsideSchedule},
		{name: "the day is full", policy: autoConfirmPolicy(), partySize: 4, startsAt: crmAt(t, 14, 20, 0),
			guests: intPtr(118), countDay: 14, wantStatus: dto.ReservationStatusRequested, wantReason: dto.PendingReasonDailyCap},
		{name: "one guest over the cap", policy: autoConfirmPolicy(), partySize: 4, startsAt: crmAt(t, 14, 20, 0),
			guests: intPtr(117), countDay: 14, wantStatus: dto.ReservationStatusRequested, wantReason: dto.PendingReasonDailyCap},
		{name: "the last seats exactly", policy: autoConfirmPolicy(), partySize: 4, startsAt: crmAt(t, 14, 20, 0),
			guests: intPtr(116), countDay: 14, wantStatus: dto.ReservationStatusConfirmed},
		{name: "auto-confirm is off", policy: disabled, partySize: 2, startsAt: crmAt(t, 9, 12, 0),
			wantStatus: dto.ReservationStatusRequested, wantReason: dto.PendingReasonDisabled},
		{name: "party size is checked before notice", policy: autoConfirmPolicy(), partySize: 12, startsAt: now.Add(30 * time.Minute),
			wantStatus: dto.ReservationStatusRequested, wantReason: dto.PendingReasonPartySize},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newAutoConfirmFixture(t, now, tt.policy)
			f.acceptsCreate()
			if tt.guests != nil {
				f.countsGuests(*tt.guests, tt.countDay)
			}

			result, err := f.service.Create(crmContext(), agentRequestFor(now, tt.partySize, tt.startsAt))

			require.NoError(t, err)
			assert.Equal(t, tt.wantStatus, result.Status)
			assert.Same(t, f.created, result)
			if tt.wantReason == "" {
				assert.Nil(t, result.PendingReason)
			} else {
				require.NotNil(t, result.PendingReason)
				assert.Equal(t, tt.wantReason, *result.PendingReason)
			}
			if tt.guests == nil {
				f.repo.AssertNotCalled(t, "LockDayAndCountGuests", mock.Anything, mock.Anything, mock.Anything)
			}
			assert.Empty(t, f.outbox.events(), "the agent tells the guest; no event is recorded at creation")
			assert.Len(t, f.live.statuses(), 1, "staff screens hear of it either way")
		})
	}
}

func TestReservationService_Create_StaffReservationsIgnoreThePolicy(t *testing.T) {
	now := crmAt(t, 8, 15, 0)
	f := newAutoConfirmFixture(t, now, autoConfirmPolicy())
	f.acceptsCreate()
	req := agentRequestFor(now, 20, now.Add(30*time.Minute))
	req.Source, req.Origin = dto.ReservationSourcePhone, nil

	result, err := f.service.Create(crmContext(), req)

	require.NoError(t, err)
	assert.Equal(t, dto.ReservationStatusConfirmed, result.Status)
	assert.Nil(t, result.PendingReason)
	f.repo.AssertNotCalled(t, "LockDayAndCountGuests", mock.Anything, mock.Anything, mock.Anything)
}

func TestReservationService_Create_CountFailureStoresNothing(t *testing.T) {
	now := crmAt(t, 8, 15, 0)
	f := newAutoConfirmFixture(t, now, autoConfirmPolicy())
	f.contacts.EXPECT().Touch(mock.Anything, mock.Anything).Return(&dto.Contact{ID: "contact-1"}, nil).Once()
	f.repo.EXPECT().LockDayAndCountGuests(mock.Anything, mock.Anything, mock.Anything).Return(0, errors.New("db down")).Once()

	_, err := f.service.Create(crmContext(), agentRequestFor(now, 4, crmAt(t, 9, 12, 0)))

	require.ErrorIs(t, err, domainError.ErrReservationOperation)
	f.repo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	assert.Empty(t, f.live.statuses())
}

func TestReservationService_Create_SettingsFailureStoresNothing(t *testing.T) {
	now := crmAt(t, 8, 15, 0)
	f := newReservationFixture(t, now)
	settings := mocks.NewMockOrganizationSettingsRepository(t)
	settings.EXPECT().Get(mock.Anything).Return(nil, errors.New("db down")).Once()
	f.service = NewReservationService(f.repo, f.contacts, f.outbox, settings,
		newRecordingEvents(t, f.outbox), f.live, "CO", discardLogger(), fixedClock(now))
	f.contacts.EXPECT().Touch(mock.Anything, mock.Anything).Return(&dto.Contact{ID: "contact-1"}, nil).Once()

	_, err := f.service.Create(crmContext(), agentReservationRequest(now))

	require.ErrorIs(t, err, domainError.ErrReservationOperation)
	f.repo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestReservationService_Confirm_StaffConfirmingAnAgentRequestRecordsTheEvent(t *testing.T) {
	now := crmAt(t, 8, 15, 0)
	f := newAutoConfirmFixture(t, now, autoConfirmPolicy())
	stored := storedReservation(dto.ReservationStatusRequested, crmAt(t, 9, 20, 0))
	partySize := dto.PendingReasonPartySize
	stored.PendingReason = &partySize
	f.holds(stored)

	result, err := f.service.Confirm(crmContext(), stored.ID)

	require.NoError(t, err)
	assert.Equal(t, dto.ReservationStatusConfirmed, result.Status)
	assert.Equal(t, &partySize, result.PendingReason, "why it waited is kept")
	events := f.outbox.events()
	require.Len(t, events, 1)
	assert.Equal(t, dto.MarketingEventReservationConfirmed, events[0].Type)
}
