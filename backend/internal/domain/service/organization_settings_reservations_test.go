package service

import (
	"context"
	"errors"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/domain/ports/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func intPtr(i int) *int { return &i }

func reservationSettingsDefaults() dto.ReservationSettings {
	return dto.ReservationSettings{MaxPartySize: 8, MinNoticeMinutes: 120, Schedule: []dto.OpeningWindow{}}
}

// newReservationSettingsFixture stores the reservations section in memory, as the repository's merge does.
func newReservationSettingsFixture(t *testing.T, stored dto.ReservationSettings) (*OrganizationSettingsService, *mocks.MockOrganizationSettingsRepository, *dto.ReservationSettings) {
	repo := mocks.NewMockOrganizationSettingsRepository(t)
	current := stored
	repo.EXPECT().Get(mock.Anything).RunAndReturn(func(context.Context) (*dto.OrganizationSettingsView, error) {
		return &dto.OrganizationSettingsView{Reservations: current, Timezone: bogotaTZ}, nil
	}).Maybe()
	repo.EXPECT().UpdateReservations(mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, merge ports.ReservationsMerge) (*dto.OrganizationSettingsView, error) {
			next, err := merge(current)
			if err != nil {
				return nil, err
			}
			current = next
			return &dto.OrganizationSettingsView{Reservations: next, Timezone: bogotaTZ}, nil
		}).Maybe()
	return NewOrganizationSettingsService(repo), repo, &current
}

func TestOrganizationSettingsService_Reservations_TurningAutoConfirmOn(t *testing.T) {
	svc, repo, current := newReservationSettingsFixture(t, reservationSettingsDefaults())

	res, err := svc.Update(context.Background(), &dto.UpdateOrganizationSettingsRequest{Reservations: &dto.ReservationSettingsPatch{
		AutoConfirmEnabled: boolPtr(true), DailyGuestCap: intPtr(120),
	}})

	require.NoError(t, err)
	want := dto.ReservationSettings{AutoConfirmEnabled: true, MaxPartySize: 8, MinNoticeMinutes: 120, DailyGuestCap: intPtr(120), Schedule: []dto.OpeningWindow{}}
	assert.Equal(t, want, res.Reservations)
	assert.Equal(t, want, *current)
	repo.AssertNotCalled(t, "UpdateOrdering", mock.Anything, mock.Anything)
}

func TestOrganizationSettingsService_Reservations_EnabledWithoutACap(t *testing.T) {
	svc, repo, current := newReservationSettingsFixture(t, reservationSettingsDefaults())

	_, err := svc.Update(context.Background(), &dto.UpdateOrganizationSettingsRequest{
		Feedback:     &dto.FeedbackSettingsPatch{ReviewLinkEnabled: boolPtr(true)},
		Reservations: &dto.ReservationSettingsPatch{AutoConfirmEnabled: boolPtr(true)},
	})

	var validation *domainError.ValidationError
	require.ErrorAs(t, err, &validation)
	assert.Equal(t, "reservations.daily_guest_cap", validation.Field)
	assert.Equal(t, reservationSettingsDefaults(), *current)
	repo.AssertNotCalled(t, "UpdateFeedback", mock.Anything, mock.Anything)
	repo.AssertNotCalled(t, "UpdateReservations", mock.Anything, mock.Anything)
}

func TestOrganizationSettingsService_Reservations_TheSavedCapCountsWhenEnabling(t *testing.T) {
	stored := reservationSettingsDefaults()
	stored.DailyGuestCap = intPtr(80)
	svc, _, current := newReservationSettingsFixture(t, stored)

	_, err := svc.Update(context.Background(), &dto.UpdateOrganizationSettingsRequest{Reservations: &dto.ReservationSettingsPatch{AutoConfirmEnabled: boolPtr(true)}})

	require.NoError(t, err)
	assert.True(t, current.AutoConfirmEnabled)
	assert.Equal(t, 80, *current.DailyGuestCap)
}

func TestOrganizationSettingsService_Reservations_OutOfRange(t *testing.T) {
	tests := []struct {
		name  string
		patch dto.ReservationSettingsPatch
		field string
	}{
		{"party size 0", dto.ReservationSettingsPatch{MaxPartySize: intPtr(0)}, "reservations.max_party_size"},
		{"party size 51", dto.ReservationSettingsPatch{MaxPartySize: intPtr(51)}, "reservations.max_party_size"},
		{"negative notice", dto.ReservationSettingsPatch{MinNoticeMinutes: intPtr(-1)}, "reservations.min_notice_minutes"},
		{"notice over a week", dto.ReservationSettingsPatch{MinNoticeMinutes: intPtr(10081)}, "reservations.min_notice_minutes"},
		{"zero cap", dto.ReservationSettingsPatch{DailyGuestCap: intPtr(0)}, "reservations.daily_guest_cap"},
		{"bad window", dto.ReservationSettingsPatch{Schedule: &[]dto.OpeningWindow{{Day: "lun", Opens: "12:00", Closes: "15:00"}}}, "reservations.schedule[0]"},
		{"overlapping windows", dto.ReservationSettingsPatch{Schedule: &[]dto.OpeningWindow{
			{Day: "sat", Opens: "12:00", Closes: "16:00"}, {Day: "sat", Opens: "15:00", Closes: "23:00"},
		}}, "reservations.schedule[1]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, _, current := newReservationSettingsFixture(t, reservationSettingsDefaults())

			_, err := svc.Update(context.Background(), &dto.UpdateOrganizationSettingsRequest{Reservations: &tt.patch})

			var validation *domainError.ValidationError
			require.ErrorAs(t, err, &validation)
			assert.Equal(t, tt.field, validation.Field)
			assert.Equal(t, reservationSettingsDefaults(), *current, "nothing is saved")
		})
	}
}

func TestOrganizationSettingsService_Reservations_EdgesOfTheRanges(t *testing.T) {
	svc, _, current := newReservationSettingsFixture(t, reservationSettingsDefaults())
	schedule := []dto.OpeningWindow{{Day: "fri", Opens: "18:00", Closes: "02:00"}}

	_, err := svc.Update(context.Background(), &dto.UpdateOrganizationSettingsRequest{Reservations: &dto.ReservationSettingsPatch{
		MaxPartySize: intPtr(50), MinNoticeMinutes: intPtr(0), DailyGuestCap: intPtr(1), Schedule: &schedule,
	}})

	require.NoError(t, err)
	assert.Equal(t, dto.ReservationSettings{MaxPartySize: 50, MinNoticeMinutes: 0, DailyGuestCap: intPtr(1), Schedule: schedule}, *current)
}

func TestOrganizationSettingsService_Reservations_ReadNeverReturnsANilSchedule(t *testing.T) {
	repo := mocks.NewMockOrganizationSettingsRepository(t)
	repo.EXPECT().Get(mock.Anything).Return(&dto.OrganizationSettingsView{
		Timezone: bogotaTZ, Reservations: dto.ReservationSettings{MaxPartySize: 8, MinNoticeMinutes: 120},
	}, nil).Once()

	res, err := NewOrganizationSettingsService(repo).Get(context.Background())

	require.NoError(t, err)
	assert.Equal(t, reservationSettingsDefaults(), res.Reservations)
}

func TestOrganizationSettingsService_Reservations_ReadFailureSavesNothing(t *testing.T) {
	repo := mocks.NewMockOrganizationSettingsRepository(t)
	repo.EXPECT().Get(mock.Anything).Return(nil, errors.New("db down")).Once()

	_, err := NewOrganizationSettingsService(repo).Update(context.Background(), &dto.UpdateOrganizationSettingsRequest{
		Reservations: &dto.ReservationSettingsPatch{MaxPartySize: intPtr(10)},
	})

	require.Error(t, err)
	repo.AssertNotCalled(t, "UpdateReservations", mock.Anything, mock.Anything)
}

func TestOrganizationSettingsService_Reservations_RequiredPermissions(t *testing.T) {
	svc := NewOrganizationSettingsService(mocks.NewMockOrganizationSettingsRepository(t))

	assert.Equal(t, []permissions.Permission{permissions.SettingsReservationsUpdate},
		svc.RequiredPermissions(&dto.UpdateOrganizationSettingsRequest{Reservations: &dto.ReservationSettingsPatch{MaxPartySize: intPtr(6)}}))
	assert.Empty(t, svc.RequiredPermissions(&dto.UpdateOrganizationSettingsRequest{Reservations: &dto.ReservationSettingsPatch{}}))
	for _, role := range []int{permissions.RoleAdmin, permissions.RoleManager} {
		assert.True(t, permissions.HasPermission([]int{role}, permissions.SettingsReservationsUpdate), "role %d", role)
	}
	assert.False(t, permissions.HasPermission([]int{permissions.RoleAgent}, permissions.SettingsReservationsUpdate))
}
