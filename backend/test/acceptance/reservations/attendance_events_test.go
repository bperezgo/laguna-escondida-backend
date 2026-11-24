package reservations

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/domain/tenant"
	"laguna-escondida/backend/internal/platform/postgres"
	"laguna-escondida/backend/internal/platform/postgres/repository"
	"laguna-escondida/backend/test/acceptance/testsupport"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var signingSecret = strings.Repeat("s", 43)

func configureMarketingApp(t *testing.T, r *restaurant) {
	t.Helper()
	_, err := testsupport.IntegrationService(t, cloudDB, "http://127.0.0.1/v1").
		SetMarketingApp(context.Background(), r.orgID, dto.MarketingAppCredentials{SigningSecret: signingSecret})
	require.NoError(t, err)
}

type outboxRow struct {
	Type string
	Body []byte
}

func attendanceRows(t *testing.T, orgID string) []outboxRow {
	t.Helper()
	var rows []outboxRow
	require.NoError(t, cloudDB.Raw(`SELECT type, body FROM marketing_event_outbox
		WHERE organization_id = ? AND type IN ('reservation.visited', 'reservation.no_show') ORDER BY created_at, event_id`, orgID).Scan(&rows).Error)
	return rows
}

func (row outboxRow) data(t *testing.T) map[string]any {
	t.Helper()
	var body struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(row.Body, &body))
	return body.Data
}

func startedHoursAgo(t *testing.T, id string, hours int) {
	t.Helper()
	require.NoError(t, cloudDB.Exec(`UPDATE reservations SET starts_at = ? WHERE id = ?`, time.Now().Add(-time.Duration(hours)*time.Hour), id).Error)
}

func runVisitedJob(t *testing.T, r *restaurant) int {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	reservationRepo := repository.NewReservationRepository(cloudDB)
	events := service.NewMarketingEventService(repository.NewMarketingEventOutboxRepository(cloudDB), repository.NewOrganizationRepository(cloudDB),
		service.MarketingAppSecrets{Integrations: testsupport.IntegrationService(t, cloudDB, "http://127.0.0.1/v1")}, true)
	reservations := service.NewReservationService(reservationRepo,
		service.NewContactService(repository.NewContactRepository(cloudDB), reservationRepo, repository.NewFeedbackRepository(cloudDB)),
		postgres.NewUnitOfWork(cloudDB), repository.NewOrganizationSettingsRepository(cloudDB, logger), events, nil, "CO", logger, nil)
	marked, err := reservations.MarkVisited(tenant.WithOrg(context.Background(), r.orgID))
	require.NoError(t, err)
	return marked
}

func TestReservationAttendance_TheJobMarksVisitedAndStaffCorrectToNoShow(t *testing.T) {
	c := newCloud(t)
	r := c.newRestaurant()
	configureMarketingApp(t, r)
	reservation := c.agentReservation(r, "573005551234", "Ana", at(localDay(t, 1), 20, 0))
	confirmed := c.withKey(r.managerKey, http.MethodPost, "/api/reservations/"+reservation.ID+"/confirm", nil)
	require.Equal(t, http.StatusOK, confirmed.status, string(confirmed.body))
	startedHoursAgo(t, reservation.ID, 3)

	require.Equal(t, 1, runVisitedJob(t, r))

	rows := attendanceRows(t, r.orgID)
	require.Len(t, rows, 1)
	assert.Equal(t, "reservation.visited", rows[0].Type)
	assert.Equal(t, "visited", rows[0].data(t)["status"])
	assert.Equal(t, "confirmed", rows[0].data(t)["previous_status"])

	corrected := c.withKey(r.managerKey, http.MethodPost, "/api/reservations/"+reservation.ID+"/no-show", nil)
	require.Equal(t, http.StatusOK, corrected.status, string(corrected.body))

	rows = attendanceRows(t, r.orgID)
	require.Len(t, rows, 2, "the visited event is neither withdrawn nor changed")
	assert.Equal(t, "reservation.visited", rows[0].Type)
	assert.Equal(t, "reservation.no_show", rows[1].Type)
	assert.Equal(t, "visited", rows[1].data(t)["previous_status"])
}

func TestReservationAttendance_AStaffEnteredReservationRecordsNone(t *testing.T) {
	c := newCloud(t)
	r := c.newRestaurant()
	configureMarketingApp(t, r)
	reservation := c.staffReservation(r, "3005559876", "Luis", at(localDay(t, 1), 20, 0))
	startedHoursAgo(t, reservation.ID, 3)

	require.Equal(t, 1, runVisitedJob(t, r))

	read := c.withKey(r.managerKey, http.MethodGet, "/api/reservations/"+reservation.ID, nil)
	var got dto.Reservation
	read.decode(t, &got)
	assert.Equal(t, dto.ReservationStatusVisited, got.Status)
	assert.Empty(t, attendanceRows(t, r.orgID))
}
