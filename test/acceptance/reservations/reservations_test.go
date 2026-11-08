package reservations

import (
	"context"
	"net/http"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/tenant"
	"laguna-escondida/backend/internal/platform/postgres/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const guestPhone = "+573005551234"

func localDay(t *testing.T, daysAhead int) time.Time {
	t.Helper()
	loc, err := time.LoadLocation(bogota)
	require.NoError(t, err)
	now := time.Now().In(loc)
	return time.Date(now.Year(), now.Month(), now.Day()+daysAhead, 0, 0, 0, 0, loc)
}

func at(day time.Time, hour, minute int) string {
	return day.Add(time.Duration(hour)*time.Hour + time.Duration(minute)*time.Minute).Format(time.RFC3339)
}

func (c *cloud) staffReservation(r *restaurant, phone, name, startsAt string) dto.Reservation {
	c.t.Helper()
	resp := c.withKey(r.managerKey, http.MethodPost, "/api/reservations", map[string]any{
		"customer_phone": phone, "name": name, "party_size": 4, "starts_at": startsAt, "source": "phone",
	})
	require.Equal(c.t, http.StatusCreated, resp.status, string(resp.body))
	var created dto.Reservation
	resp.decode(c.t, &created)
	return created
}

func (c *cloud) agentReservation(r *restaurant, phone, name, startsAt string) dto.Reservation {
	c.t.Helper()
	resp := c.withKey(r.agentKey, http.MethodPost, "/api/reservations", map[string]any{
		"customer_phone": phone, "name": name, "party_size": 6, "starts_at": startsAt, "source": "whatsapp",
		"origin": map[string]any{"channel_id": "ch_1", "conversation_id": "conv_9", "ctwa_clid": "ARAkLk",
			"attribution": map[string]any{"source": "meta_ctwa", "ad_id": "120210000000001", "clicked_at": "yesterday"}},
	})
	require.Equal(c.t, http.StatusCreated, resp.status, string(resp.body))
	var created dto.Reservation
	resp.decode(c.t, &created)
	return created
}

func TestReservations_TheAgentRequestsAndStaffConfirm(t *testing.T) {
	c := newCloud(t)
	r := c.newRestaurant()
	tomorrow := localDay(t, 1)

	requested := c.agentReservation(r, "573005551234", "Ana", at(tomorrow, 20, 0))
	assert.Equal(t, dto.ReservationStatusRequested, requested.Status)
	assert.Equal(t, guestPhone, requested.ContactPhone)

	refused := c.withKey(r.agentKey, http.MethodPost, "/api/reservations/"+requested.ID+"/confirm", nil)
	assert.Equal(t, http.StatusForbidden, refused.status, "the agent cannot confirm")

	confirmed := c.withKey(r.managerKey, http.MethodPost, "/api/reservations/"+requested.ID+"/confirm", nil)
	require.Equal(t, http.StatusOK, confirmed.status, string(confirmed.body))
	again := c.withKey(r.managerKey, http.MethodPost, "/api/reservations/"+requested.ID+"/decline", map[string]any{"reason": "late"})
	assert.Equal(t, http.StatusConflict, again.status)

	read := c.withKey(r.managerKey, http.MethodGet, "/api/reservations/"+requested.ID, nil)
	require.Equal(t, http.StatusOK, read.status)
	var got dto.Reservation
	read.decode(t, &got)
	assert.Equal(t, dto.ReservationStatusConfirmed, got.Status)
	assert.Equal(t, "ch_1", *got.Origin.ChannelID)
	assert.Equal(t, "conv_9", *got.Origin.ConversationID)
	assert.Equal(t, "120210000000001", *got.Origin.Attribution.AdID)
	assert.Nil(t, got.Origin.Attribution.ClickedAt, "a garbled click time is dropped")
	assert.NotContains(t, string(read.body), "ARAkLk", "no click identifier is stored")
}

func TestReservations_TonightsListInTimeOrder(t *testing.T) {
	c := newCloud(t)
	r := c.newRestaurant()
	day := localDay(t, 1)
	late := c.staffReservation(r, "3005551234", "Ana", at(day, 21, 30))
	early := c.staffReservation(r, "3005559876", "Luis", at(day, 19, 0))
	middle := c.agentReservation(r, "573001112233", "Marta", at(day, 20, 0))
	c.staffReservation(r, "3005551234", "Ana", at(localDay(t, 2), 0, 30))
	declined := c.withKey(r.managerKey, http.MethodPost, "/api/reservations/"+middle.ID+"/decline", map[string]any{"reason": "Fully booked"})
	require.Equal(t, http.StatusOK, declined.status, string(declined.body))

	resp := c.withKey(r.managerKey, http.MethodGet, "/api/reservations?date="+day.Format(time.DateOnly), nil)

	require.Equal(t, http.StatusOK, resp.status, string(resp.body))
	var list dto.ReservationListResponse
	resp.decode(t, &list)
	require.Len(t, list.Reservations, 3, "only that local day, in every state")
	assert.Equal(t, []string{early.ID, middle.ID, late.ID},
		[]string{list.Reservations[0].ID, list.Reservations[1].ID, list.Reservations[2].ID})
	assert.Equal(t, "+573005559876", list.Reservations[0].ContactPhone)
	assert.Equal(t, dto.ReservationStatusDeclined, list.Reservations[1].Status)
	assert.Equal(t, "Fully booked", *list.Reservations[1].DeclineReason)

	confirmedOnly := c.withKey(r.managerKey, http.MethodGet, "/api/reservations?status=confirmed&date="+day.Format(time.DateOnly), nil)
	var filtered dto.ReservationListResponse
	confirmedOnly.decode(t, &filtered)
	assert.Len(t, filtered.Reservations, 2)
}

func TestReservations_TheGuestCancelsOnlyTheirOwn(t *testing.T) {
	c := newCloud(t)
	r := c.newRestaurant()
	mine := c.agentReservation(r, "573005551234", "Ana", at(localDay(t, 1), 20, 0))
	theirs := c.agentReservation(r, "573119998877", "Luis", at(localDay(t, 1), 20, 0))

	foreign := c.withKey(r.agentKey, http.MethodPost, "/api/reservations/"+theirs.ID+"/cancel", map[string]any{"customer_phone": guestPhone})
	assert.Equal(t, http.StatusNotFound, foreign.status)
	notAnID := c.withKey(r.agentKey, http.MethodPost, "/api/reservations/not-a-uuid/cancel", map[string]any{"customer_phone": guestPhone})
	assert.Equal(t, http.StatusNotFound, notAnID.status)

	own := c.withKey(r.agentKey, http.MethodPost, "/api/reservations/"+mine.ID+"/cancel", map[string]any{"customer_phone": guestPhone})
	require.Equal(t, http.StatusOK, own.status, string(own.body))
	var cancelled dto.Reservation
	own.decode(t, &cancelled)
	assert.Equal(t, dto.CancelledByCustomer, *cancelled.CancelledBy)

	listed := c.withKey(r.agentKey, http.MethodGet, "/api/reservations?phone=573005551234&from="+time.Now().Add(-24*time.Hour).Format(time.RFC3339), nil)
	var list dto.ReservationListResponse
	listed.decode(t, &list)
	require.Len(t, list.Reservations, 1)
	assert.Equal(t, dto.ReservationStatusCancelled, list.Reservations[0].Status)

	var theirsNow dto.Reservation
	c.withKey(r.managerKey, http.MethodGet, "/api/reservations/"+theirs.ID, nil).decode(t, &theirsNow)
	assert.Equal(t, dto.ReservationStatusRequested, theirsNow.Status, "unchanged")
}

// seedFeedback marks reservationID visited and stores a request and an answer recorded at recordedAt.
func seedFeedback(t *testing.T, orgID, reservationID, contactID string, rating int, recordedAt time.Time) {
	t.Helper()
	requestID := uuid.NewString()
	require.NoError(t, cloudDB.Exec(`UPDATE reservations SET status = 'visited', visited_at = ? WHERE id = ?`, recordedAt, reservationID).Error)
	require.NoError(t, cloudDB.Exec(`INSERT INTO feedback_requests (id, organization_id, reservation_id, contact_id, requested_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?)`, requestID, orgID, reservationID, contactID, recordedAt, recordedAt.Add(7*24*time.Hour)).Error)
	require.NoError(t, cloudDB.Exec(`INSERT INTO feedback (id, organization_id, feedback_request_id, rating, comment, needs_attention, created_at, updated_at)
		VALUES (?, ?, ?, ?, 'comentario', ?, ?, ?)`, uuid.NewString(), orgID, requestID, rating, rating <= 3, recordedAt, recordedAt).Error)
}

func TestContacts_ARegularsHistory(t *testing.T) {
	c := newCloud(t)
	r := c.newRestaurant()
	first := c.agentReservation(r, "573005551234", "Ana", at(localDay(t, 1), 20, 0))
	second := c.staffReservation(r, "3005551234", "Ana María", at(localDay(t, 3), 20, 0))
	third := c.staffReservation(r, "3005551234", "Ana María", at(localDay(t, 5), 20, 0))
	require.Equal(t, first.ContactID, second.ContactID, "one contact per phone")
	seedFeedback(t, r.orgID, first.ID, first.ContactID, 5, time.Now().Add(-2*time.Hour))
	seedFeedback(t, r.orgID, second.ID, first.ContactID, 2, time.Now().Add(-time.Hour))

	resp := c.withKey(r.managerKey, http.MethodGet, "/api/contacts/"+first.ContactID, nil)

	require.Equal(t, http.StatusOK, resp.status, string(resp.body))
	var detail dto.ContactDetail
	resp.decode(t, &detail)
	assert.Equal(t, guestPhone, detail.Phone)
	assert.Equal(t, "Ana María", *detail.Name, "the latest non-empty name")
	require.NotNil(t, detail.FirstTouch)
	assert.Equal(t, "120210000000001", *detail.FirstTouch.AdID, "the ad that first brought them")
	require.Len(t, detail.Reservations, 3)
	assert.Equal(t, []string{third.ID, second.ID, first.ID},
		[]string{detail.Reservations[0].ID, detail.Reservations[1].ID, detail.Reservations[2].ID}, "newest first")
	require.Len(t, detail.Feedback, 2)
	assert.Equal(t, 2, detail.Feedback[0].Rating)

	search := c.withKey(r.managerKey, http.MethodGet, "/api/contacts?search=maría", nil)
	var list dto.ContactListResponse
	search.decode(t, &list)
	require.Len(t, list.Contacts, 1)
	assert.Equal(t, first.ContactID, list.Contacts[0].ID)
}

func TestContacts_OrganizationBCannotReadOrganizationAs(t *testing.T) {
	c := newCloud(t)
	a, b := c.newRestaurant(), c.newRestaurant()
	inA := c.staffReservation(a, "3005551234", "Ana", at(localDay(t, 1), 20, 0))
	inB := c.staffReservation(b, "3005551234", "Ana", at(localDay(t, 1), 20, 0))
	assert.NotEqual(t, inA.ContactID, inB.ContactID, "the same phone in two restaurants is two contacts")

	assert.Equal(t, http.StatusNotFound, c.withKey(b.managerKey, http.MethodGet, "/api/contacts/"+inA.ContactID, nil).status)
	assert.Equal(t, http.StatusNotFound, c.withKey(b.managerKey, http.MethodGet, "/api/reservations/"+inA.ID, nil).status)

	contacts := repository.NewContactRepository(cloudDB)
	_, err := contacts.FindByID(tenant.WithOrg(context.Background(), b.orgID), inA.ContactID)
	require.ErrorIs(t, err, domainError.ErrContactNotFound)
	found, err := contacts.FindByPhone(tenant.WithOrg(context.Background(), b.orgID), guestPhone)
	require.NoError(t, err)
	assert.Equal(t, inB.ContactID, found.ID)
	page, total, err := contacts.List(tenant.WithOrg(context.Background(), b.orgID), dto.ContactListFilter{Limit: 50})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Equal(t, inB.ContactID, page[0].ID)
}

func TestFeedback_ThisWeeksUnhappyGuests(t *testing.T) {
	c := newCloud(t)
	r := c.newRestaurant()
	reservations := make([]dto.Reservation, 3)
	for i := range reservations {
		reservations[i] = c.staffReservation(r, "300555123"+string(rune('0'+i)), "Guest", at(localDay(t, 1), 19+i, 0))
	}
	seedFeedback(t, r.orgID, reservations[0].ID, reservations[0].ContactID, 2, time.Now().Add(-time.Minute))
	seedFeedback(t, r.orgID, reservations[1].ID, reservations[1].ContactID, 5, time.Now().Add(-time.Minute))
	seedFeedback(t, r.orgID, reservations[2].ID, reservations[2].ContactID, 1, time.Now().AddDate(0, 0, -8))

	resp := c.withKey(r.managerKey, http.MethodGet, "/api/feedback?needs_attention=true", nil)

	require.Equal(t, http.StatusOK, resp.status, string(resp.body))
	var list dto.FeedbackListResponse
	resp.decode(t, &list)
	require.Len(t, list.Feedback, 1, "only this week's 1–3 ratings")
	assert.Equal(t, 2, list.Feedback[0].Rating)
	assert.Equal(t, reservations[0].ID, list.Feedback[0].Reservation.ID)
	assert.Equal(t, reservations[0].ContactPhone, list.Feedback[0].ContactPhone)
	assert.Equal(t, http.StatusForbidden, c.withKey(r.agentKey, http.MethodGet, "/api/feedback", nil).status)
}

func TestFeedback_TheGuestAnswersAndAddsACommentLater(t *testing.T) {
	c := newCloud(t)
	r := c.newRestaurant()
	visit := c.agentReservation(r, "573005551234", "Ana", at(localDay(t, 1), 20, 0))
	requestID := uuid.NewString()
	require.NoError(t, cloudDB.Exec(`INSERT INTO feedback_requests (id, organization_id, reservation_id, contact_id, requested_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?)`, requestID, r.orgID, visit.ID, visit.ContactID, time.Now(), time.Now().Add(7*24*time.Hour)).Error)
	patch := c.withKey(r.managerKey, http.MethodPatch, "/api/organization/settings",
		map[string]any{"feedback": map[string]any{"review_link_enabled": true, "review_url": "https://g.page/r/abc/review"}})
	require.Equal(t, http.StatusOK, patch.status, string(patch.body))

	first := c.withKey(r.agentKey, http.MethodPost, "/api/feedback", map[string]any{"customer_phone": guestPhone, "rating": 5})
	require.Equal(t, http.StatusOK, first.status, string(first.body))
	second := c.withKey(r.agentKey, http.MethodPost, "/api/feedback", map[string]any{"customer_phone": guestPhone, "rating": 2, "comment": "La música muy alta"})
	require.Equal(t, http.StatusOK, second.status, string(second.body))

	var answer dto.RecordFeedbackResponse
	second.decode(t, &answer)
	assert.True(t, answer.Escalate)
	require.NotNil(t, answer.ReviewURL, "every rating gets the link while it is on")
	assert.Equal(t, "https://g.page/r/abc/review", *answer.ReviewURL)
	var stored []struct {
		Rating  int
		Comment string
	}
	require.NoError(t, cloudDB.Raw(`SELECT rating, comment FROM feedback WHERE feedback_request_id = ?`, requestID).Scan(&stored).Error)
	require.Len(t, stored, 1, "the second answer replaced the first")
	assert.Equal(t, 2, stored[0].Rating)
	assert.Equal(t, "La música muy alta", stored[0].Comment)

	nothing := c.withKey(r.agentKey, http.MethodPost, "/api/feedback", map[string]any{"customer_phone": "573119998877", "rating": 5})
	assert.Equal(t, http.StatusNotFound, nothing.status)
}

func TestOrders_AnAgentOrderKeepsItsOrigin(t *testing.T) {
	c := newCloud(t)
	r := c.newRestaurant()
	orderID := uuid.NewString()
	created := c.withKey(r.agentKey, http.MethodPost, "/api/orders", map[string]any{
		"open_bill_id": orderID, "temporal_identifier": uuid.NewString(),
		"order_type": "takeaway", "customer_phone": "3005551234", "customer_name": "Ana",
		"products": []map[string]any{{"open_bill_product_id": uuid.NewString(), "product_id": r.productID, "quantity": 1}},
		"origin":   map[string]any{"channel_id": "ch_1", "attribution": map[string]any{"source": "meta_ctwa", "ad_id": "120210000000001"}},
	})
	require.Equal(t, http.StatusCreated, created.status, string(created.body))

	updated := c.withKey(r.agentKey, http.MethodPut, "/api/orders/"+orderID, map[string]any{
		"products": []map[string]any{{"open_bill_product_id": uuid.NewString(), "product_id": r.productID, "quantity": 3}},
	})
	require.Equal(t, http.StatusOK, updated.status, string(updated.body))

	read := c.withKey(r.managerKey, http.MethodGet, "/api/orders/"+orderID, nil)
	require.Equal(t, http.StatusOK, read.status, string(read.body))
	var order dto.OpenBillWithProducts
	read.decode(t, &order)
	require.NotNil(t, order.Origin, "staff read the origin")
	assert.Equal(t, "ch_1", *order.Origin.ChannelID)
	assert.Equal(t, "120210000000001", *order.Origin.Attribution.AdID, "unchanged by the update")

	contact, err := repository.NewContactRepository(cloudDB).FindByPhone(tenant.WithOrg(context.Background(), r.orgID), guestPhone)
	require.NoError(t, err, "the agent order created the guest's contact")
	assert.Equal(t, "Ana", *contact.Name)
	assert.Equal(t, "meta_ctwa", *contact.FirstTouch.Source)
}

func TestOrders_AStaffOrderHasNoOrigin(t *testing.T) {
	c := newCloud(t)
	r := c.newRestaurant()
	orderID := uuid.NewString()
	created := c.withKey(r.managerKey, http.MethodPost, "/api/orders", map[string]any{
		"open_bill_id": orderID, "temporal_identifier": uuid.NewString(),
		"products": []map[string]any{{"open_bill_product_id": uuid.NewString(), "product_id": r.productID, "quantity": 1}},
	})
	require.Equal(t, http.StatusCreated, created.status, string(created.body))

	read := c.withKey(r.managerKey, http.MethodGet, "/api/orders/"+orderID, nil)
	assert.NotContains(t, string(read.body), `"origin"`)
}
