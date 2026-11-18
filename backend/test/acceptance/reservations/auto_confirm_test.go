package reservations

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/tenant"
	"laguna-escondida/backend/internal/platform/postgres/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (c *cloud) staffReservationFor(r *restaurant, partySize int, startsAt string) dto.Reservation {
	c.t.Helper()
	resp := c.withKey(r.managerKey, http.MethodPost, "/api/reservations", map[string]any{
		"customer_phone": "3005559876", "name": "Luis", "party_size": partySize, "starts_at": startsAt, "source": "phone",
	})
	require.Equal(c.t, http.StatusCreated, resp.status, string(resp.body))
	var created dto.Reservation
	resp.decode(c.t, &created)
	return created
}

func agentBody(partySize int, startsAt string) map[string]any {
	return map[string]any{
		"customer_phone": "573005551234", "name": "Ana", "party_size": partySize, "starts_at": startsAt, "source": "whatsapp",
		"origin": map[string]any{"channel_id": "ch_1", "conversation_id": "conv_9"},
	}
}

func (c *cloud) agentReservationFor(r *restaurant, partySize int, startsAt string) dto.Reservation {
	c.t.Helper()
	resp := c.withKey(r.agentKey, http.MethodPost, "/api/reservations", agentBody(partySize, startsAt))
	require.Equal(c.t, http.StatusCreated, resp.status, string(resp.body))
	var created dto.Reservation
	resp.decode(c.t, &created)
	return created
}

func (c *cloud) enableAutoConfirm(r *restaurant, guestCap int) {
	c.t.Helper()
	resp := c.withKey(r.managerKey, http.MethodPatch, "/api/organization/settings", map[string]any{
		"reservations": map[string]any{"auto_confirm_enabled": true, "daily_guest_cap": guestCap},
	})
	require.Equal(c.t, http.StatusOK, resp.status, string(resp.body))
}

func (c *cloud) reservationSettings(r *restaurant) dto.ReservationSettings {
	c.t.Helper()
	resp := c.withKey(r.managerKey, http.MethodGet, "/api/organization/settings", nil)
	require.Equal(c.t, http.StatusOK, resp.status, string(resp.body))
	var settings dto.OrganizationSettingsResponse
	resp.decode(c.t, &settings)
	return settings.Reservations
}

func TestReservationSettings_AnUnsavedSectionReadsTheDefaults(t *testing.T) {
	c := newCloud(t)
	r := c.newRestaurant()
	want := dto.ReservationSettings{MaxPartySize: 8, MinNoticeMinutes: 120, Schedule: []dto.OpeningWindow{}}

	repo := repository.NewOrganizationSettingsRepository(cloudDB, slog.New(slog.NewTextHandler(io.Discard, nil)))
	view, err := repo.Get(tenant.WithOrg(context.Background(), r.orgID))
	require.NoError(t, err)
	assert.Equal(t, want, view.Reservations)

	require.NoError(t, cloudDB.Exec(`UPDATE organizations SET settings = '{"feedback":{"review_link_enabled":true}}' WHERE id = ?`, r.orgID).Error)
	view, err = repo.Get(tenant.WithOrg(context.Background(), r.orgID))
	require.NoError(t, err)
	assert.Equal(t, want, view.Reservations, "other sections saved, but not this one")

	resp := c.withKey(r.managerKey, http.MethodGet, "/api/organization/settings", nil)
	assert.Contains(t, string(resp.body),
		`"reservations":{"auto_confirm_enabled":false,"max_party_size":8,"min_notice_minutes":120,"daily_guest_cap":null,"schedule":[]}`)
}

func TestReservationSettings_ManagersSetThePolicy(t *testing.T) {
	c := newCloud(t)
	r := c.newRestaurant()

	noCap := c.withKey(r.managerKey, http.MethodPatch, "/api/organization/settings", map[string]any{
		"reservations": map[string]any{"auto_confirm_enabled": true},
	})
	assert.Equal(t, http.StatusBadRequest, noCap.status)
	assert.Contains(t, string(noCap.body), `"field":"reservations.daily_guest_cap"`)
	assert.False(t, c.reservationSettings(r).AutoConfirmEnabled, "nothing saved")

	forbidden := c.withKey(r.agentKey, http.MethodPatch, "/api/organization/settings", map[string]any{
		"reservations": map[string]any{"auto_confirm_enabled": true, "daily_guest_cap": 120},
	})
	assert.Equal(t, http.StatusForbidden, forbidden.status, string(forbidden.body))
	assert.False(t, c.reservationSettings(r).AutoConfirmEnabled, "nothing saved")

	c.enableAutoConfirm(r, 120)

	settings := c.reservationSettings(r)
	assert.True(t, settings.AutoConfirmEnabled)
	assert.Equal(t, 8, settings.MaxPartySize)
	assert.Equal(t, 120, settings.MinNoticeMinutes)
	require.NotNil(t, settings.DailyGuestCap)
	assert.Equal(t, 120, *settings.DailyGuestCap)
}

func TestReservations_ThePendingReasonIsStored(t *testing.T) {
	c := newCloud(t)
	r := c.newRestaurant()
	tomorrow := localDay(t, 1)

	requested := c.agentReservationFor(r, 4, at(tomorrow, 12, 0))
	staff := c.staffReservationFor(r, 4, at(tomorrow, 13, 0))

	repo := repository.NewReservationRepository(cloudDB)
	stored, err := repo.FindByID(tenant.WithOrg(context.Background(), r.orgID), requested.ID)
	require.NoError(t, err)
	require.NotNil(t, stored.PendingReason)
	assert.Equal(t, dto.PendingReasonDisabled, *stored.PendingReason)
	assert.Nil(t, staff.PendingReason)

	confirmed := c.withKey(r.managerKey, http.MethodPost, "/api/reservations/"+requested.ID+"/confirm", nil)
	require.Equal(t, http.StatusOK, confirmed.status)
	var after dto.Reservation
	confirmed.decode(t, &after)
	require.NotNil(t, after.PendingReason, "set once and never changed")
	assert.Equal(t, dto.PendingReasonDisabled, *after.PendingReason)
}

func TestReservations_ThePolicyConfirmsAnOrdinaryTable(t *testing.T) {
	c := newCloud(t)
	r := c.newRestaurant()
	tomorrow := localDay(t, 1)
	c.enableAutoConfirm(r, 120)
	c.staffReservationFor(r, 40, at(tomorrow, 20, 0))

	ordinary := c.agentReservationFor(r, 4, at(tomorrow, 12, 0))
	large := c.agentReservationFor(r, 10, at(tomorrow, 12, 0))

	assert.Equal(t, dto.ReservationStatusConfirmed, ordinary.Status)
	assert.Nil(t, ordinary.PendingReason)
	assert.Equal(t, dto.ReservationStatusRequested, large.Status)
	require.NotNil(t, large.PendingReason)
	assert.Equal(t, dto.PendingReasonPartySize, *large.PendingReason)
}

func TestReservations_TwoRequestsForTheLastSeats(t *testing.T) {
	c := newCloud(t)
	r := c.newRestaurant()
	c.enableAutoConfirm(r, 120)

	// Several days, so a lucky interleaving on one doesn't hide a race.
	for daysAhead := 2; daysAhead <= 6; daysAhead++ {
		day := localDay(t, daysAhead)
		for _, partySize := range []int{50, 50, 16} {
			c.staffReservationFor(r, partySize, at(day, 19, 0))
		}

		statuses := c.createConcurrently(r, 2, agentBody(4, at(day, 13, 0)))

		var confirmed, full int
		for _, s := range statuses {
			switch {
			case s.Status == dto.ReservationStatusConfirmed:
				confirmed++
			case s.Status == dto.ReservationStatusRequested && s.PendingReason != nil && *s.PendingReason == dto.PendingReasonDailyCap:
				full++
			}
		}
		assert.Equal(t, 1, confirmed, "day +%d: exactly one takes the last seats", daysAhead)
		assert.Equal(t, 1, full, "day +%d: the other waits for staff", daysAhead)
	}
}

// createConcurrently fires n agent creates at once, each from its own guest so the contact row
// doesn't serialize them, and returns what each stored.
func (c *cloud) createConcurrently(r *restaurant, n int, body map[string]any) []dto.Reservation {
	c.t.Helper()
	bodies := make([][]byte, n)
	for i := range n {
		body["customer_phone"] = fmt.Sprintf("57300555%04d", i)
		raw, err := json.Marshal(body)
		require.NoError(c.t, err)
		bodies[i] = raw
	}
	start := make(chan struct{})
	results := make([]dto.Reservation, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			req, reqErr := http.NewRequest(http.MethodPost, c.server.URL+"/api/reservations", bytes.NewReader(bodies[i]))
			if reqErr != nil {
				errs[i] = reqErr
				return
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+r.agentKey)
			<-start
			resp, doErr := http.DefaultClient.Do(req)
			if doErr != nil {
				errs[i] = doErr
				return
			}
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode != http.StatusCreated {
				payload, _ := io.ReadAll(resp.Body)
				errs[i] = &unexpectedStatus{status: resp.StatusCode, body: string(payload)}
				return
			}
			errs[i] = json.NewDecoder(resp.Body).Decode(&results[i])
		}(i)
	}
	time.Sleep(10 * time.Millisecond)
	close(start)
	wg.Wait()
	for _, e := range errs {
		require.NoError(c.t, e)
	}
	return results
}

type unexpectedStatus struct {
	status int
	body   string
}

func (e *unexpectedStatus) Error() string {
	return http.StatusText(e.status) + ": " + e.body
}
