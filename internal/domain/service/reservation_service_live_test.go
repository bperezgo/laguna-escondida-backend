package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// liveFeed records what staff screens would be told, and whether it was told inside the transaction.
type liveFeed struct {
	mu      sync.Mutex
	changes []*dto.Reservation
	inTx    []bool
}

func (l *liveFeed) ReservationChanged(ctx context.Context, reservation *dto.Reservation) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.changes = append(l.changes, reservation)
	l.inTx = append(l.inTx, inUnitOfWork(ctx))
}

func (l *liveFeed) statuses() []dto.ReservationStatus {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]dto.ReservationStatus, 0, len(l.changes))
	for _, r := range l.changes {
		out = append(out, r.Status)
	}
	return out
}

func TestReservationService_Live_ANewRequestReachesTheScreensAfterItIsSaved(t *testing.T) {
	now := crmAt(t, 8, 15, 0)
	f := newReservationFixture(t, now)
	f.acceptsCreate()

	result, err := f.service.Create(crmContext(), agentReservationRequest(now))

	require.NoError(t, err)
	require.Len(t, f.live.changes, 1)
	assert.Same(t, result, f.live.changes[0])
	assert.Equal(t, []bool{false}, f.live.inTx, "told after the commit, never before")
}

func TestReservationService_Live_EachTransitionReachesTheScreens(t *testing.T) {
	now := crmAt(t, 8, 15, 0)
	tests := []struct {
		name string
		from dto.ReservationStatus
		act  func(*ReservationService) (*dto.Reservation, error)
		want dto.ReservationStatus
	}{
		{"confirm", dto.ReservationStatusRequested, func(s *ReservationService) (*dto.Reservation, error) { return s.Confirm(crmContext(), "r1") }, dto.ReservationStatusConfirmed},
		{"decline", dto.ReservationStatusRequested, func(s *ReservationService) (*dto.Reservation, error) { return s.Decline(crmContext(), "r1", nil) }, dto.ReservationStatusDeclined},
		{"cancel", dto.ReservationStatusConfirmed, func(s *ReservationService) (*dto.Reservation, error) { return s.CancelByStaff(crmContext(), "r1") }, dto.ReservationStatusCancelled},
		{"no-show", dto.ReservationStatusConfirmed, func(s *ReservationService) (*dto.Reservation, error) { return s.MarkNoShow(crmContext(), "r1") }, dto.ReservationStatusNoShow},
		{"guest cancel", dto.ReservationStatusConfirmed, func(s *ReservationService) (*dto.Reservation, error) {
			return s.CancelByCustomer(crmContext(), "r1", crmPhone)
		}, dto.ReservationStatusCancelled},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newReservationFixture(t, now)
			f.holds(storedReservation(tt.from, now.Add(time.Hour)))

			_, err := tt.act(f.service)

			require.NoError(t, err)
			assert.Equal(t, []dto.ReservationStatus{tt.want}, f.live.statuses())
			assert.Equal(t, []bool{false}, f.live.inTx)
		})
	}
}

func TestReservationService_Live_NothingIsToldWhenTheChangeFails(t *testing.T) {
	now := crmAt(t, 8, 15, 0)
	t.Run("refused transition", func(t *testing.T) {
		f := newReservationFixture(t, now)
		f.holds(storedReservation(dto.ReservationStatusDeclined, now.Add(time.Hour)))
		_, err := f.service.Confirm(crmContext(), "r1")
		require.ErrorIs(t, err, domainError.ErrReservationTransition)
		assert.Empty(t, f.live.changes)
	})
	t.Run("event rolled back", func(t *testing.T) {
		f := newReservationFixture(t, now)
		f.outbox.appendErr = errors.New("outbox down")
		f.holds(storedReservation(dto.ReservationStatusRequested, now.Add(time.Hour)))
		_, err := f.service.Confirm(crmContext(), "r1")
		require.Error(t, err)
		assert.Empty(t, f.live.changes)
	})
	t.Run("create rolled back", func(t *testing.T) {
		f := newReservationFixture(t, now)
		f.contacts.EXPECT().Touch(mock.Anything, mock.Anything).Return(&dto.Contact{ID: "contact-1"}, nil).Once()
		f.repo.EXPECT().Create(mock.Anything, mock.Anything).Return(errors.New("db down")).Once()
		_, err := f.service.Create(crmContext(), agentReservationRequest(now))
		require.Error(t, err)
		assert.Empty(t, f.live.changes)
	})
}

func TestReservationService_Live_AVisitReachesTheScreens(t *testing.T) {
	dinner := storedReservation(dto.ReservationStatusConfirmed, crmAt(t, 8, 20, 0))
	f, _ := visitedFixture(t, crmAt(t, 8, 22, 15), dinner)

	_, err := f.service.MarkVisited(crmContext())

	require.NoError(t, err)
	assert.Equal(t, []dto.ReservationStatus{dto.ReservationStatusVisited}, f.live.statuses())
}

func TestReservationService_Live_ALostVisitRaceTellsNothing(t *testing.T) {
	f := newReservationFixture(t, crmAt(t, 8, 23, 0))
	f.repo.EXPECT().ListConfirmedStartedBefore(mock.Anything, mock.Anything).Return([]*dto.Reservation{
		storedReservation(dto.ReservationStatusConfirmed, crmAt(t, 8, 20, 0)),
	}, nil).Once()
	f.repo.EXPECT().Update(mock.Anything, mock.Anything, dto.ReservationStatusConfirmed).Return(domainError.ErrReservationTransition).Once()

	_, err := f.service.MarkVisited(crmContext())

	require.NoError(t, err)
	assert.Empty(t, f.live.changes)
}
