package ordering

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/domain/tenant"
	"laguna-escondida/backend/internal/platform/postgres/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func requireGate(t *testing.T) {
	t.Helper()
	if !gateEnabled() {
		t.Skip("Skipping acceptance test. Set RUN_ACCEPTANCE_TESTS=true to run (needs Docker or DB_HOST).")
	}
}

func settingsRepo(logs *bytes.Buffer) ports.OrganizationSettingsRepository {
	return repository.NewOrganizationSettingsRepository(cloudDB, slog.New(slog.NewTextHandler(logs, nil)))
}

func setOrdering(next dto.OrderingSettings) func(dto.OrderingSettings) (dto.OrderingSettings, error) {
	return func(dto.OrderingSettings) (dto.OrderingSettings, error) { return next, nil }
}

func pausedDelivery(reason string) dto.OrderingSettings {
	return dto.OrderingSettings{
		Delivery: dto.OrderTypeSettings{Paused: true, PausedReason: &reason, Schedule: []dto.OpeningWindow{}},
		Takeaway: dto.OrderTypeSettings{Schedule: []dto.OpeningWindow{}},
	}
}

// --- repository on Postgres ---

func TestOrganizationSettingsRepository_NothingStoredReadsAsDefaults(t *testing.T) {
	requireGate(t)
	ctx := tenant.WithOrg(context.Background(), seedOrganization(t))

	view, err := settingsRepo(&bytes.Buffer{}).Get(ctx)

	require.NoError(t, err)
	assert.Equal(t, dto.OrderingSettings{}, view.Ordering)
	assert.Equal(t, "America/Bogota", view.Timezone)
}

func TestOrganizationSettingsRepository_UpdateThenGet(t *testing.T) {
	requireGate(t)
	orgID := seedOrganization(t)
	ctx := tenant.WithOrg(context.Background(), orgID)
	repo := settingsRepo(&bytes.Buffer{})
	require.NoError(t, cloudDB.Exec(`UPDATE organizations SET settings = '{"other": {"kept": true}}' WHERE id = ?`, orgID).Error)

	saved, err := repo.UpdateOrdering(ctx, setOrdering(pausedDelivery("cocina saturada")))
	require.NoError(t, err)
	got, err := repo.Get(ctx)
	require.NoError(t, err)

	assert.Equal(t, pausedDelivery("cocina saturada"), saved.Ordering)
	assert.Equal(t, saved, got)
	var other string
	require.NoError(t, cloudDB.Raw(`SELECT settings->'other'->>'kept' FROM organizations WHERE id = ?`, orgID).Scan(&other).Error)
	assert.Equal(t, "true", other, "keys other than ordering are left alone")
}

func TestOrganizationSettingsRepository_FailedMergeStoresNothing(t *testing.T) {
	requireGate(t)
	orgID := seedOrganization(t)
	ctx := tenant.WithOrg(context.Background(), orgID)
	repo := settingsRepo(&bytes.Buffer{})

	_, err := repo.UpdateOrdering(ctx, func(dto.OrderingSettings) (dto.OrderingSettings, error) {
		return dto.OrderingSettings{}, errors.New("invalid schedule")
	})

	require.ErrorContains(t, err, "invalid schedule")
	var isNull bool
	require.NoError(t, cloudDB.Raw(`SELECT settings IS NULL FROM organizations WHERE id = ?`, orgID).Scan(&isNull).Error)
	assert.True(t, isNull)
}

func TestOrganizationSettingsRepository_UnparseableValueReadsAsDefaultsAndIsLogged(t *testing.T) {
	requireGate(t)
	orgID := seedOrganization(t)
	require.NoError(t, cloudDB.Exec(`UPDATE organizations SET settings = '{"ordering": "abierto"}' WHERE id = ?`, orgID).Error)
	var logs bytes.Buffer

	view, err := settingsRepo(&logs).Get(tenant.WithOrg(context.Background(), orgID))

	require.NoError(t, err)
	assert.Equal(t, dto.OrderingSettings{}, view.Ordering)
	assert.Contains(t, logs.String(), orgID)
}

func TestOrganizationSettingsRepository_TenantScoped(t *testing.T) {
	requireGate(t)
	a, b := seedOrganization(t), seedOrganization(t)
	repo := settingsRepo(&bytes.Buffer{})

	_, err := repo.UpdateOrdering(tenant.WithOrg(context.Background(), a), setOrdering(pausedDelivery("A")))
	require.NoError(t, err)
	viewB, err := repo.Get(tenant.WithOrg(context.Background(), b))
	require.NoError(t, err)

	assert.False(t, viewB.Ordering.Delivery.Paused, "B is untouched by A's write")
	_, err = repo.Get(context.Background())
	assert.Error(t, err, "no organization in the context reads nothing")
}

// The row lock serializes the merges: the second one sees the first one's write.
func TestOrganizationSettingsRepository_ConcurrentMergesDontOverwriteEachOther(t *testing.T) {
	requireGate(t)
	ctx := tenant.WithOrg(context.Background(), seedOrganization(t))
	repo := settingsRepo(&bytes.Buffer{})
	reason := "lleno"
	firstInside := make(chan struct{})

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, err := repo.UpdateOrdering(ctx, func(current dto.OrderingSettings) (dto.OrderingSettings, error) {
			close(firstInside)
			time.Sleep(200 * time.Millisecond)
			current.Delivery.Paused, current.Delivery.PausedReason = true, &reason
			return current, nil
		})
		assert.NoError(t, err)
	}()
	go func() {
		defer wg.Done()
		<-firstInside
		_, err := repo.UpdateOrdering(ctx, func(current dto.OrderingSettings) (dto.OrderingSettings, error) {
			current.Takeaway.Schedule = []dto.OpeningWindow{{Day: "mon", Opens: "11:30", Closes: "22:30"}}
			return current, nil
		})
		assert.NoError(t, err)
	}()
	wg.Wait()

	view, err := repo.Get(ctx)
	require.NoError(t, err)
	assert.True(t, view.Ordering.Delivery.Paused, "the pause survives")
	assert.Len(t, view.Ordering.Takeaway.Schedule, 1, "and so does the schedule")
}

func TestOrganizationRepository_PlatformReadIncludesSettings(t *testing.T) {
	requireGate(t)
	orgID := seedOrganization(t)
	_, err := settingsRepo(&bytes.Buffer{}).UpdateOrdering(tenant.WithOrg(context.Background(), orgID), setOrdering(pausedDelivery("x")))
	require.NoError(t, err)

	org, err := repository.NewOrganizationRepository(cloudDB).Get(context.Background(), orgID)

	require.NoError(t, err)
	assert.True(t, org.Settings.Ordering.Delivery.Paused)
	assert.Equal(t, []dto.OpeningWindow{}, org.Settings.Ordering.Takeaway.Schedule)
}

// --- end to end through the cloud ---

type settingsBody struct {
	Ordering     dto.OrderingSettings     `json:"ordering"`
	Availability dto.OrderingAvailability `json:"availability"`
	Timezone     string                   `json:"timezone"`
}

func TestOrdering_PauseRefusesDeliveryUntilResumed(t *testing.T) {
	c := newCloud(t)
	r := c.newRestaurant()

	resp := c.withKey(r.managerKey, http.MethodPatch, "/api/organization/settings",
		map[string]any{"ordering": map[string]any{"delivery": map[string]any{"paused": true, "paused_reason": "cocina saturada"}}})
	require.Equal(t, http.StatusOK, resp.status, string(resp.body))
	var patched settingsBody
	resp.decode(t, &patched)
	assert.False(t, patched.Availability.Delivery.Available)

	refused := c.createOrder(r.agentKey, r, delivery())
	require.Equal(t, http.StatusUnprocessableEntity, refused.status, string(refused.body))
	assert.Equal(t, "ORDERING_PAUSED", errorCode(t, refused))
	assert.Contains(t, string(refused.body), `"paused_reason":"cocina saturada"`)

	assert.Equal(t, http.StatusCreated, c.createOrder(r.agentKey, r, takeaway()).status, "takeaway is still taken")

	resp = c.withKey(r.managerKey, http.MethodPatch, "/api/organization/settings",
		map[string]any{"ordering": map[string]any{"delivery": map[string]any{"paused": false}}})
	require.Equal(t, http.StatusOK, resp.status, string(resp.body))
	resumed := c.createOrder(r.agentKey, r, delivery())
	require.Equal(t, http.StatusUnprocessableEntity, resumed.status, string(resumed.body))
	assert.Equal(t, "OUTSIDE_DELIVERY_ZONE", errorCode(t, resumed), "past the gate, the zone check decides")
}

func TestOrdering_TakeawayOutsideItsHoursIsRefused(t *testing.T) {
	c := newCloud(t)
	r := c.newRestaurant()
	bogota, err := time.LoadLocation("America/Bogota")
	require.NoError(t, err)
	// Three days from now, so the window is never open while the test runs.
	opensOn := time.Now().In(bogota).AddDate(0, 0, 3)
	day := map[time.Weekday]string{
		time.Monday: "mon", time.Tuesday: "tue", time.Wednesday: "wed", time.Thursday: "thu",
		time.Friday: "fri", time.Saturday: "sat", time.Sunday: "sun",
	}[opensOn.Weekday()]

	resp := c.withKey(r.managerKey, http.MethodPatch, "/api/organization/settings", map[string]any{"ordering": map[string]any{
		"takeaway": map[string]any{"schedule": []map[string]string{{"day": day, "opens": "10:00", "closes": "11:00"}}},
	}})
	require.Equal(t, http.StatusOK, resp.status, string(resp.body))

	refused := c.createOrder(r.agentKey, r, takeaway())
	require.Equal(t, http.StatusUnprocessableEntity, refused.status, string(refused.body))
	assert.Equal(t, "OUTSIDE_ORDERING_HOURS", errorCode(t, refused))
	var body struct {
		NextOpensAt time.Time `json:"next_opens_at"`
	}
	refused.decode(t, &body)
	want := time.Date(opensOn.Year(), opensOn.Month(), opensOn.Day(), 10, 0, 0, 0, bogota)
	assert.True(t, want.Equal(body.NextOpensAt), "next opens %s, want %s", body.NextOpensAt, want)

	assert.Equal(t, http.StatusCreated, c.createOrder(r.managerKey, r, dineIn()).status, "dine-in is never gated")

	var read settingsBody
	agentRead := c.withKey(r.agentKey, http.MethodGet, "/api/organization/settings", nil)
	require.Equal(t, http.StatusOK, agentRead.status, string(agentRead.body))
	agentRead.decode(t, &read)
	assert.Equal(t, dto.AvailabilityReasonOutsideHours, *read.Availability.Takeaway.Reason)
	assert.Equal(t, "America/Bogota", read.Timezone)
}

func TestOrdering_AgentReadsButCannotChange(t *testing.T) {
	c := newCloud(t)
	r := c.newRestaurant()

	assert.Equal(t, http.StatusOK, c.withKey(r.agentKey, http.MethodGet, "/api/organization/settings", nil).status)
	resp := c.withKey(r.agentKey, http.MethodPatch, "/api/organization/settings",
		map[string]any{"ordering": map[string]any{"delivery": map[string]any{"paused": true}}})
	assert.Equal(t, http.StatusForbidden, resp.status)
	assert.Equal(t, http.StatusCreated, c.createOrder(r.agentKey, r, takeaway()).status, "nothing was paused")
}

func TestOrdering_OrganizationsDontSeeEachOther(t *testing.T) {
	c := newCloud(t)
	a, b := c.newRestaurant(), c.newRestaurant()

	resp := c.withKey(a.managerKey, http.MethodPatch, "/api/organization/settings",
		map[string]any{"ordering": map[string]any{"takeaway": map[string]any{"paused": true}}})
	require.Equal(t, http.StatusOK, resp.status, string(resp.body))

	var readB settingsBody
	c.withKey(b.agentKey, http.MethodGet, "/api/organization/settings", nil).decode(t, &readB)
	assert.True(t, readB.Availability.Takeaway.Available)
	assert.Equal(t, http.StatusCreated, c.createOrder(b.agentKey, b, takeaway()).status)
	assert.Equal(t, http.StatusUnprocessableEntity, c.createOrder(a.agentKey, a, takeaway()).status)
}
