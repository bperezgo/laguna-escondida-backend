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
	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/domain/ports/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const bogotaTZ = "America/Bogota"

func mustLocation(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	require.NoError(t, err)
	return loc
}

// 2026-10-26 is a Monday.
func bogotaAt(t *testing.T, day, hour, minute int) time.Time {
	return time.Date(2026, time.October, day, hour, minute, 0, 0, mustLocation(t, bogotaTZ))
}

func win(day, opens, closes string) dto.OpeningWindow {
	return dto.OpeningWindow{Day: day, Opens: opens, Closes: closes}
}

func boolPtr(b bool) *bool { return &b }

func schedulePtr(windows ...dto.OpeningWindow) *[]dto.OpeningWindow {
	if windows == nil {
		windows = []dto.OpeningWindow{}
	}
	return &windows
}

func rfc3339(tm *time.Time) string {
	if tm == nil {
		return ""
	}
	return tm.Format(time.RFC3339)
}

type settingsFixture struct {
	repo     *mocks.MockOrganizationSettingsRepository
	service  *OrganizationSettingsService
	current  dto.OrderingSettings
	timezone string
	writes   int
}

func newSettingsFixture(t *testing.T, now time.Time) *settingsFixture {
	repo := mocks.NewMockOrganizationSettingsRepository(t)
	svc := NewOrganizationSettingsService(repo)
	svc.now = func() time.Time { return now }
	return &settingsFixture{repo: repo, service: svc}
}

// stored makes the repository hold these settings; UpdateOrdering runs the merge on them and
// stores the result only when the merge succeeds, as the real row lock does.
func (f *settingsFixture) stored(ordering dto.OrderingSettings, timezone string) {
	f.timezone = timezone
	f.current = ordering
	f.repo.EXPECT().Get(mock.Anything).
		RunAndReturn(func(context.Context) (*dto.OrganizationSettingsView, error) {
			return &dto.OrganizationSettingsView{Ordering: f.current, Timezone: f.timezone}, nil
		}).Maybe()
	f.repo.EXPECT().UpdateOrdering(mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, merge ports.OrderingMerge) (*dto.OrganizationSettingsView, error) {
			next, err := merge(f.current)
			if err != nil {
				return nil, err
			}
			f.current = next
			f.writes++
			return &dto.OrganizationSettingsView{Ordering: next, Timezone: f.timezone}, nil
		}).Maybe()
}

func (f *settingsFixture) captureUpdate() *dto.OrderingSettings {
	return &f.current
}

func requireValidationField(t *testing.T, err error, field string) {
	t.Helper()
	var validation *domainError.OrderingSettingsValidationError
	require.True(t, errors.As(err, &validation), "expected a validation error, got %v", err)
	assert.Equal(t, field, validation.Field)
}

func patchSchedule(orderType dto.OrderType, windows ...dto.OpeningWindow) *dto.UpdateOrganizationSettingsRequest {
	p := &dto.OrderTypeSettingsPatch{Schedule: schedulePtr(windows...)}
	if orderType == dto.OrderTypeDelivery {
		return &dto.UpdateOrganizationSettingsRequest{Ordering: &dto.OrderingSettingsPatch{Delivery: p}}
	}
	return &dto.UpdateOrganizationSettingsRequest{Ordering: &dto.OrderingSettingsPatch{Takeaway: p}}
}

// --- schedule validation ---

func TestOrganizationSettingsService_Update_InvalidSchedule(t *testing.T) {
	tooMany := make([]dto.OpeningWindow, 0, 22)
	for _, hours := range [][2]string{{"08:00", "09:00"}, {"11:00", "12:00"}, {"14:00", "15:00"}} {
		for _, day := range []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"} {
			tooMany = append(tooMany, win(day, hours[0], hours[1]))
		}
	}
	tooMany = append(tooMany, win("mon", "20:00", "21:00"))

	tests := []struct {
		name    string
		windows []dto.OpeningWindow
		field   string
	}{
		{"unknown day code", []dto.OpeningWindow{win("monday", "11:00", "15:00")}, "ordering.takeaway.schedule[0]"},
		{"hour 24", []dto.OpeningWindow{win("mon", "11:00", "24:00")}, "ordering.takeaway.schedule[0]"},
		{"single-digit hour", []dto.OpeningWindow{win("mon", "7:00", "15:00")}, "ordering.takeaway.schedule[0]"},
		{"minute 60", []dto.OpeningWindow{win("mon", "11:60", "15:00")}, "ordering.takeaway.schedule[0]"},
		{"empty time", []dto.OpeningWindow{win("mon", "", "15:00")}, "ordering.takeaway.schedule[0]"},
		{"equal times", []dto.OpeningWindow{win("mon", "12:00", "12:00")}, "ordering.takeaway.schedule[0]"},
		{"overlap on one day", []dto.OpeningWindow{win("mon", "11:00", "15:00"), win("mon", "14:00", "18:00")}, "ordering.takeaway.schedule[1]"},
		{"one window inside another", []dto.OpeningWindow{win("mon", "11:00", "22:00"), win("mon", "12:00", "13:00")}, "ordering.takeaway.schedule[1]"},
		{"overlap across midnight", []dto.OpeningWindow{win("fri", "18:00", "02:00"), win("sat", "01:00", "04:00")}, "ordering.takeaway.schedule[1]"},
		{"overlap across the week's end", []dto.OpeningWindow{win("sun", "22:00", "02:00"), win("mon", "01:00", "03:00")}, "ordering.takeaway.schedule[1]"},
		{"more than 21 windows", tooMany, "ordering.takeaway.schedule"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newSettingsFixture(t, bogotaAt(t, 26, 12, 0))
			f.stored(dto.OrderingSettings{Takeaway: dto.OrderTypeSettings{Schedule: []dto.OpeningWindow{win("tue", "10:00", "11:00")}}}, bogotaTZ)

			_, err := f.service.Update(context.Background(), patchSchedule(dto.OrderTypeTakeaway, tt.windows...))

			requireValidationField(t, err, tt.field)
			assert.Zero(t, f.writes, "nothing is stored")
		})
	}
}

func TestOrganizationSettingsService_Update_ValidSchedules(t *testing.T) {
	twentyOne := make([]dto.OpeningWindow, 0, 21)
	for _, day := range []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"} {
		twentyOne = append(twentyOne, win(day, "08:00", "09:00"), win(day, "11:00", "12:00"), win(day, "14:00", "15:00"))
	}

	tests := []struct {
		name    string
		windows []dto.OpeningWindow
	}{
		{"past midnight", []dto.OpeningWindow{win("fri", "18:00", "02:00")}},
		{"back to back", []dto.OpeningWindow{win("mon", "11:00", "15:00"), win("mon", "15:00", "18:00")}},
		{"past midnight then the next day after it closes", []dto.OpeningWindow{win("fri", "18:00", "02:00"), win("sat", "02:00", "04:00")}},
		{"sunday past midnight and a monday lunch", []dto.OpeningWindow{win("sun", "20:00", "01:00"), win("mon", "11:30", "15:00")}},
		{"exactly 21 windows", twentyOne},
		{"00:00 and 23:59", []dto.OpeningWindow{win("wed", "00:00", "23:59")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newSettingsFixture(t, bogotaAt(t, 26, 12, 0))
			f.stored(dto.OrderingSettings{}, bogotaTZ)
			saved := f.captureUpdate()

			_, err := f.service.Update(context.Background(), patchSchedule(dto.OrderTypeDelivery, tt.windows...))

			require.NoError(t, err)
			assert.Equal(t, tt.windows, saved.Delivery.Schedule)
		})
	}
}

func TestOrganizationSettingsService_Update_PausedReasonTooLong(t *testing.T) {
	f := newSettingsFixture(t, bogotaAt(t, 26, 12, 0))
	f.stored(dto.OrderingSettings{}, bogotaTZ)

	_, err := f.service.Update(context.Background(), &dto.UpdateOrganizationSettingsRequest{Ordering: &dto.OrderingSettingsPatch{
		Delivery: &dto.OrderTypeSettingsPatch{Paused: boolPtr(true), PausedReason: strPtr(strings.Repeat("ñ", 256))},
	}})

	requireValidationField(t, err, "ordering.delivery.paused_reason")
	assert.Zero(t, f.writes, "nothing is stored")
}

// --- availability ---

func TestOrganizationSettingsService_Get_Availability(t *testing.T) {
	mondayEvening := []dto.OpeningWindow{win("mon", "18:00", "22:30")}
	outside := dto.AvailabilityReasonOutsideHours

	tests := []struct {
		name        string
		delivery    dto.OrderTypeSettings
		now         time.Time
		available   bool
		reason      *dto.AvailabilityReason
		closesAt    string
		nextOpensAt string
	}{
		{
			name:     "inside a window",
			delivery: dto.OrderTypeSettings{Schedule: mondayEvening},
			now:      bogotaAt(t, 26, 19, 0), available: true, closesAt: "2026-10-26T22:30:00-05:00",
		},
		{
			name:     "exactly at opening",
			delivery: dto.OrderTypeSettings{Schedule: mondayEvening},
			now:      bogotaAt(t, 26, 18, 0), available: true, closesAt: "2026-10-26T22:30:00-05:00",
		},
		{
			name:     "before opening",
			delivery: dto.OrderTypeSettings{Schedule: mondayEvening},
			now:      bogotaAt(t, 26, 17, 0), reason: &outside, nextOpensAt: "2026-10-26T18:00:00-05:00",
		},
		{
			name:     "exactly at closing",
			delivery: dto.OrderTypeSettings{Schedule: mondayEvening},
			now:      bogotaAt(t, 26, 22, 30), reason: &outside, nextOpensAt: "2026-11-02T18:00:00-05:00",
		},
		{
			name:     "empty schedule",
			delivery: dto.OrderTypeSettings{},
			now:      bogotaAt(t, 27, 3, 0), available: true,
		},
		{
			name:     "early hours of a past-midnight window",
			delivery: dto.OrderTypeSettings{Schedule: []dto.OpeningWindow{win("fri", "18:00", "02:00")}},
			now:      bogotaAt(t, 31, 1, 30), available: true, closesAt: "2026-10-31T02:00:00-05:00",
		},
		{
			name:     "after a past-midnight window closed",
			delivery: dto.OrderTypeSettings{Schedule: []dto.OpeningWindow{win("fri", "18:00", "02:00")}},
			now:      bogotaAt(t, 31, 2, 0), reason: &outside, nextOpensAt: "2026-11-06T18:00:00-05:00",
		},
		{
			name:     "next opening a few days ahead",
			delivery: dto.OrderTypeSettings{Schedule: []dto.OpeningWindow{win("wed", "12:00", "14:00"), win("thu", "12:00", "14:00")}},
			now:      bogotaAt(t, 26, 10, 0), reason: &outside, nextOpensAt: "2026-10-28T12:00:00-05:00",
		},
		{
			name:     "next opening across the week's end",
			delivery: dto.OrderTypeSettings{Schedule: []dto.OpeningWindow{win("mon", "11:30", "22:30")}},
			now:      time.Date(2026, time.November, 1, 23, 0, 0, 0, mustLocation(t, bogotaTZ)),
			reason:   &outside, nextOpensAt: "2026-11-02T11:30:00-05:00",
		},
		{
			name:     "takeaway after closing on its only day opens next week",
			delivery: dto.OrderTypeSettings{Schedule: []dto.OpeningWindow{win("mon", "11:30", "22:30")}},
			now:      bogotaAt(t, 26, 23, 0), reason: &outside, nextOpensAt: "2026-11-02T11:30:00-05:00",
		},
		{
			name:     "a hand-edited window that doesn't parse is ignored",
			delivery: dto.OrderTypeSettings{Schedule: []dto.OpeningWindow{win("xyz", "18:00", "22:30")}},
			now:      bogotaAt(t, 26, 3, 0), available: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newSettingsFixture(t, tt.now)
			f.stored(dto.OrderingSettings{Delivery: tt.delivery}, bogotaTZ)

			res, err := f.service.Get(context.Background())

			require.NoError(t, err)
			got := res.Availability.Delivery
			assert.Equal(t, tt.available, got.Available)
			assert.Equal(t, tt.reason, got.Reason)
			assert.Equal(t, tt.closesAt, rfc3339(got.ClosesAt))
			assert.Equal(t, tt.nextOpensAt, rfc3339(got.NextOpensAt))
			assert.Nil(t, got.PausedReason)
		})
	}
}

func TestOrganizationSettingsService_Get_PausedWinsOverAWindow(t *testing.T) {
	f := newSettingsFixture(t, bogotaAt(t, 26, 19, 0))
	f.stored(dto.OrderingSettings{Delivery: dto.OrderTypeSettings{
		Paused: true, PausedReason: strPtr("cocina saturada"), Schedule: []dto.OpeningWindow{win("mon", "18:00", "22:30")},
	}}, bogotaTZ)

	res, err := f.service.Get(context.Background())

	require.NoError(t, err)
	paused := dto.AvailabilityReasonPaused
	assert.Equal(t, dto.Availability{Reason: &paused, PausedReason: strPtr("cocina saturada")}, res.Availability.Delivery)
	assert.True(t, res.Availability.Takeaway.Available, "takeaway is evaluated on its own")
}

func TestOrganizationSettingsService_Get_PausedOutsideHoursHasNoNextOpening(t *testing.T) {
	f := newSettingsFixture(t, bogotaAt(t, 26, 3, 0))
	f.stored(dto.OrderingSettings{Takeaway: dto.OrderTypeSettings{Paused: true, Schedule: []dto.OpeningWindow{win("mon", "18:00", "22:30")}}}, bogotaTZ)

	res, err := f.service.Get(context.Background())

	require.NoError(t, err)
	assert.Equal(t, dto.AvailabilityReasonPaused, *res.Availability.Takeaway.Reason)
	assert.Nil(t, res.Availability.Takeaway.NextOpensAt)
}

func TestOrganizationSettingsService_Get_InstantsCarryTheOrganizationOffset(t *testing.T) {
	f := newSettingsFixture(t, time.Date(2026, time.October, 27, 0, 0, 0, 0, time.UTC)) // Mon 19:00 in Bogotá
	f.stored(dto.OrderingSettings{Delivery: dto.OrderTypeSettings{Schedule: []dto.OpeningWindow{win("mon", "18:00", "22:30")}}}, bogotaTZ)

	res, err := f.service.Get(context.Background())
	require.NoError(t, err)
	body, err := json.Marshal(res.Availability.Delivery)
	require.NoError(t, err)

	assert.JSONEq(t, `{"available":true,"reason":null,"paused_reason":null,
		"closes_at":"2026-10-26T22:30:00-05:00","next_opens_at":null}`, string(body))
}

func TestOrganizationSettingsService_Get_KeepsWallClockHoursAcrossDST(t *testing.T) {
	newYork := mustLocation(t, "America/New_York")
	// US daylight saving ends on Sunday 2026-11-01 at 02:00.
	f := newSettingsFixture(t, time.Date(2026, time.October, 31, 23, 0, 0, 0, newYork))
	f.stored(dto.OrderingSettings{Delivery: dto.OrderTypeSettings{Schedule: []dto.OpeningWindow{win("sun", "12:00", "14:00")}}}, "America/New_York")

	res, err := f.service.Get(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "2026-11-01T12:00:00-05:00", rfc3339(res.Availability.Delivery.NextOpensAt))
}

func TestOrganizationSettingsService_Get_TimezoneChangeMovesTheHours(t *testing.T) {
	now := time.Date(2026, time.October, 26, 23, 30, 0, 0, time.UTC) // 18:30 Bogotá, 17:30 Mexico City
	schedule := dto.OrderingSettings{Delivery: dto.OrderTypeSettings{Schedule: []dto.OpeningWindow{win("mon", "18:00", "22:30")}}}

	bogota := newSettingsFixture(t, now)
	bogota.stored(schedule, bogotaTZ)
	mexico := newSettingsFixture(t, now)
	mexico.stored(schedule, "America/Mexico_City")

	inBogota, err := bogota.service.Get(context.Background())
	require.NoError(t, err)
	inMexico, err := mexico.service.Get(context.Background())
	require.NoError(t, err)

	assert.True(t, inBogota.Availability.Delivery.Available)
	assert.False(t, inMexico.Availability.Delivery.Available)
	assert.Equal(t, "2026-10-26T18:00:00-06:00", rfc3339(inMexico.Availability.Delivery.NextOpensAt))
}

// --- reading and patching ---

func TestOrganizationSettingsService_Get_NothingStoredReadsAsDefaults(t *testing.T) {
	f := newSettingsFixture(t, bogotaAt(t, 27, 3, 0))
	f.stored(dto.OrderingSettings{}, bogotaTZ)

	res, err := f.service.Get(context.Background())

	require.NoError(t, err)
	body, err := json.Marshal(res)
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"ordering": {
			"delivery": {"paused": false, "paused_reason": null, "schedule": []},
			"takeaway": {"paused": false, "paused_reason": null, "schedule": []}
		},
		"availability": {
			"delivery": {"available": true, "reason": null, "paused_reason": null, "closes_at": null, "next_opens_at": null},
			"takeaway": {"available": true, "reason": null, "paused_reason": null, "closes_at": null, "next_opens_at": null}
		},
		"timezone": "America/Bogota"
	}`, string(body))
}

func TestOrganizationSettingsService_Get_RepositoryError(t *testing.T) {
	f := newSettingsFixture(t, bogotaAt(t, 26, 12, 0))
	f.repo.EXPECT().Get(mock.Anything).Return(nil, errors.New("db down")).Once()

	_, err := f.service.Get(context.Background())

	require.ErrorContains(t, err, "db down")
}

func TestOrganizationSettingsService_Get_UnknownTimezone(t *testing.T) {
	f := newSettingsFixture(t, bogotaAt(t, 26, 12, 0))
	f.stored(dto.OrderingSettings{Delivery: dto.OrderTypeSettings{Schedule: []dto.OpeningWindow{win("mon", "18:00", "22:30")}}}, "Mars/Olympus")

	_, err := f.service.Get(context.Background())

	require.Error(t, err)
}

func TestOrganizationSettingsService_Update_PauseLeavesEverythingElse(t *testing.T) {
	f := newSettingsFixture(t, bogotaAt(t, 26, 19, 0))
	current := dto.OrderingSettings{
		Delivery: dto.OrderTypeSettings{Schedule: []dto.OpeningWindow{win("mon", "18:00", "22:30")}},
		Takeaway: dto.OrderTypeSettings{Schedule: []dto.OpeningWindow{win("mon", "11:30", "22:30")}},
	}
	f.stored(current, bogotaTZ)
	saved := f.captureUpdate()

	res, err := f.service.Update(context.Background(), &dto.UpdateOrganizationSettingsRequest{Ordering: &dto.OrderingSettingsPatch{
		Delivery: &dto.OrderTypeSettingsPatch{Paused: boolPtr(true), PausedReason: strPtr(" cocina saturada ")},
	}})

	require.NoError(t, err)
	assert.Equal(t, dto.OrderingSettings{
		Delivery: dto.OrderTypeSettings{Paused: true, PausedReason: strPtr("cocina saturada"), Schedule: current.Delivery.Schedule},
		Takeaway: current.Takeaway,
	}, *saved)
	assert.Equal(t, dto.AvailabilityReasonPaused, *res.Availability.Delivery.Reason, "the response shows the new state")
	assert.True(t, res.Availability.Takeaway.Available)
}

func TestOrganizationSettingsService_Update_ResumingClearsTheReason(t *testing.T) {
	f := newSettingsFixture(t, bogotaAt(t, 26, 19, 0))
	f.stored(dto.OrderingSettings{Delivery: dto.OrderTypeSettings{Paused: true, PausedReason: strPtr("cocina saturada")}}, bogotaTZ)
	saved := f.captureUpdate()

	res, err := f.service.Update(context.Background(), &dto.UpdateOrganizationSettingsRequest{Ordering: &dto.OrderingSettingsPatch{
		Delivery: &dto.OrderTypeSettingsPatch{Paused: boolPtr(false)},
	}})

	require.NoError(t, err)
	assert.False(t, saved.Delivery.Paused)
	assert.Nil(t, saved.Delivery.PausedReason)
	assert.True(t, res.Availability.Delivery.Available)
}

func TestOrganizationSettingsService_Update_ReasonWithoutPauseIsDropped(t *testing.T) {
	f := newSettingsFixture(t, bogotaAt(t, 26, 19, 0))
	f.stored(dto.OrderingSettings{}, bogotaTZ)
	saved := f.captureUpdate()

	_, err := f.service.Update(context.Background(), &dto.UpdateOrganizationSettingsRequest{Ordering: &dto.OrderingSettingsPatch{
		Takeaway: &dto.OrderTypeSettingsPatch{PausedReason: strPtr("sin gas")},
	}})

	require.NoError(t, err)
	assert.Nil(t, saved.Takeaway.PausedReason, "paused_reason is null when not paused")
}

func TestOrganizationSettingsService_Update_EmptyScheduleClearsIt(t *testing.T) {
	f := newSettingsFixture(t, bogotaAt(t, 27, 3, 0))
	f.stored(dto.OrderingSettings{Takeaway: dto.OrderTypeSettings{Schedule: []dto.OpeningWindow{win("mon", "11:30", "22:30")}}}, bogotaTZ)
	saved := f.captureUpdate()

	res, err := f.service.Update(context.Background(), patchSchedule(dto.OrderTypeTakeaway))

	require.NoError(t, err)
	assert.Empty(t, saved.Takeaway.Schedule)
	assert.True(t, res.Availability.Takeaway.Available, "no schedule means open whenever not paused")
}

func TestOrganizationSettingsService_Update_EmptyBodyRewritesNothingNew(t *testing.T) {
	f := newSettingsFixture(t, bogotaAt(t, 26, 12, 0))
	current := dto.OrderingSettings{Delivery: dto.OrderTypeSettings{Schedule: []dto.OpeningWindow{win("mon", "18:00", "22:30")}}}
	f.stored(current, bogotaTZ)
	saved := f.captureUpdate()

	_, err := f.service.Update(context.Background(), &dto.UpdateOrganizationSettingsRequest{})

	require.NoError(t, err)
	assert.Equal(t, current.Delivery, saved.Delivery)
	assert.Equal(t, []dto.OpeningWindow{}, saved.Takeaway.Schedule)
}

func TestOrganizationSettingsService_Update_RepositoryError(t *testing.T) {
	f := newSettingsFixture(t, bogotaAt(t, 26, 12, 0))
	f.repo.EXPECT().UpdateOrdering(mock.Anything, mock.Anything).Return(nil, errors.New("lock timeout")).Once()

	_, err := f.service.Update(context.Background(), patchSchedule(dto.OrderTypeTakeaway))

	require.ErrorContains(t, err, "lock timeout")
}

func TestOrganizationSettingsService_RequiredPermissions(t *testing.T) {
	tests := []struct {
		name  string
		patch *dto.OrderingSettingsPatch
		want  []permissions.Permission
	}{
		{"nothing", nil, nil},
		{"empty types", &dto.OrderingSettingsPatch{Delivery: &dto.OrderTypeSettingsPatch{}}, nil},
		{"pause", &dto.OrderingSettingsPatch{Delivery: &dto.OrderTypeSettingsPatch{Paused: boolPtr(true)}}, []permissions.Permission{permissions.OrderingPause}},
		{"reason only", &dto.OrderingSettingsPatch{Takeaway: &dto.OrderTypeSettingsPatch{PausedReason: strPtr("x")}}, []permissions.Permission{permissions.OrderingPause}},
		{"schedule", &dto.OrderingSettingsPatch{Takeaway: &dto.OrderTypeSettingsPatch{Schedule: schedulePtr()}}, []permissions.Permission{permissions.OrderingManage}},
		{
			"schedule on one type and pause on the other",
			&dto.OrderingSettingsPatch{
				Delivery: &dto.OrderTypeSettingsPatch{Schedule: schedulePtr(win("mon", "18:00", "22:00"))},
				Takeaway: &dto.OrderTypeSettingsPatch{Paused: boolPtr(true)},
			},
			[]permissions.Permission{permissions.OrderingPause, permissions.OrderingManage},
		},
	}
	svc := NewOrganizationSettingsService(mocks.NewMockOrganizationSettingsRepository(t))
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, svc.RequiredPermissions(&dto.UpdateOrganizationSettingsRequest{Ordering: tt.patch}))
		})
	}
}
