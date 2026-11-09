package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
)

func TestReservationService_MarkNoShow_UnansweredRequestOnceItsTimeCame(t *testing.T) {
	now := crmAt(t, 9, 8, 0)
	f := newReservationFixture(t, now)
	f.holds(storedReservation(dto.ReservationStatusRequested, now.Add(-12*time.Hour)))

	result, err := f.service.MarkNoShow(crmContext(), "r1")

	require.NoError(t, err)
	assert.Equal(t, dto.ReservationStatusNoShow, result.Status)
	assert.Empty(t, f.outbox.events(), "no-shows are never messaged")
}

func TestReservationService_MarkNoShow_RequestBeforeItsTimeIsRejected(t *testing.T) {
	now := crmAt(t, 8, 15, 0)
	f := newReservationFixture(t, now)
	f.holds(storedReservation(dto.ReservationStatusRequested, now.Add(time.Minute)))

	_, err := f.service.MarkNoShow(crmContext(), "r1")

	var transition *domainError.ReservationTransitionError
	require.ErrorAs(t, err, &transition)
	assert.Equal(t, string(dto.ReservationStatusRequested), transition.From)
	assert.Equal(t, string(dto.ReservationStatusNoShow), transition.To)
	assert.Nil(t, f.updated)
}

func TestReservationService_LateChangesAreNotMessaged(t *testing.T) {
	tests := []struct {
		action string
		from   dto.ReservationStatus
		want   dto.ReservationStatus
	}{
		{action: "confirm", from: dto.ReservationStatusRequested, want: dto.ReservationStatusConfirmed},
		{action: "decline", from: dto.ReservationStatusRequested, want: dto.ReservationStatusDeclined},
		{action: "cancel", from: dto.ReservationStatusRequested, want: dto.ReservationStatusCancelled},
		{action: "cancel", from: dto.ReservationStatusConfirmed, want: dto.ReservationStatusCancelled},
	}
	for _, tt := range tests {
		t.Run(string(tt.from)+" "+tt.action, func(t *testing.T) {
			now := crmAt(t, 9, 8, 0)
			f := newReservationFixture(t, now)
			f.holds(storedReservation(tt.from, now))

			result, err := reservationActions[tt.action].apply(f.service, crmContext(), "r1")

			require.NoError(t, err)
			assert.Equal(t, tt.want, result.Status)
			assert.Empty(t, f.outbox.events())
			assert.Equal(t, []dto.ReservationStatus{tt.want}, f.live.statuses(), "staff screens still hear about it")
		})
	}
}
