package mcpserver

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"laguna-escondida/backend/internal/domain/dto"
)

const testReservationID = "019a0000-0000-7000-8000-0000000000a1"

var channelForTest = "ch_1"

func callerHeaders(extra map[string]string) map[string]string {
	headers := map[string]string{customerPhoneHeader: "573005551234"}
	for k, v := range extra {
		headers[k] = v
	}
	return headers
}

// reservationBackend answers the reservation and feedback endpoints from a fixed store.
func reservationBackend(t *testing.T, stored []*dto.Reservation, feedback func(w http.ResponseWriter, b *fakeBackend)) *fakeBackend {
	b := newFakeBackend(t)
	b.override = func(w http.ResponseWriter, r *http.Request) bool {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/reservations":
			var req dto.CreateReservationRequest
			require.NoError(t, json.Unmarshal(b.lastBody(), &req))
			if req.PartySize > 50 {
				b.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid party_size: must be between 1 and 50", "code": "VALIDATION_ERROR", "field": "party_size"})
				return true
			}
			startsAt, _ := time.Parse(time.RFC3339, req.StartsAt)
			b.writeJSON(w, http.StatusCreated, dto.Reservation{
				ID: testReservationID, ContactPhone: req.CustomerPhone, Name: req.Name, PartySize: req.PartySize, StartsAt: startsAt,
				Source: req.Source, Status: dto.ReservationStatusRequested, Origin: dto.Origin{ChannelID: req.Origin.ChannelID},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/reservations":
			b.writeJSON(w, http.StatusOK, dto.ReservationListResponse{Reservations: stored, Total: len(stored)})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/cancel"):
			var req dto.CancelReservationRequest
			require.NoError(t, json.Unmarshal(b.lastBody(), &req))
			for _, s := range stored {
				if "/api/reservations/"+s.ID+"/cancel" == r.URL.Path && s.ContactPhone == *req.CustomerPhone {
					cancelled := *s
					cancelled.Status = dto.ReservationStatusCancelled
					b.writeJSON(w, http.StatusOK, cancelled)
					return true
				}
			}
			b.writeJSON(w, http.StatusNotFound, map[string]string{"error": "reservation not found"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/feedback":
			feedback(w, b)
		default:
			return false
		}
		return true
	}
	return b
}

func TestCreateMyReservation_BookingForSixAtEight(t *testing.T) {
	backend := reservationBackend(t, nil, nil)
	cs := connectCustomer(t, backend, callerHeaders(map[string]string{
		channelIDHeader: "ch_1", conversationIDHeader: "conv_9",
		attributionSourceHeader: "meta_ctwa", attributionAdIDHeader: "120210000000001", attributionClickedAtHeader: "2026-11-08T14:00:00Z",
	}))
	tomorrow := time.Now().Add(24 * time.Hour).Truncate(time.Hour).Format(time.RFC3339)

	text, isErr := call(t, cs, "create_my_reservation", map[string]any{"name": "Ana", "party_size": 6, "starts_at": tomorrow})

	require.False(t, isErr, text)
	posts := backend.requestsTo(http.MethodPost, "/api/reservations")
	require.Len(t, posts, 1)
	var sent dto.CreateReservationRequest
	require.NoError(t, json.Unmarshal(posts[0].Body, &sent))
	assert.Equal(t, "+573005551234", sent.CustomerPhone, "the header phone, not an argument")
	assert.Equal(t, dto.ReservationSourceWhatsApp, sent.Source)
	assert.Equal(t, "ch_1", *sent.Origin.ChannelID)
	assert.Equal(t, "conv_9", *sent.Origin.ConversationID)
	assert.Equal(t, "meta_ctwa", *sent.Origin.Attribution.Source)
	assert.Equal(t, "120210000000001", *sent.Origin.Attribution.AdID)
	assert.Equal(t, "2026-11-08T14:00:00Z", *sent.Origin.Attribution.ClickedAt)

	var view map[string]any
	require.NoError(t, json.Unmarshal([]byte(text), &view))
	assert.Equal(t, testReservationID, view["reservation_id"])
	assert.Equal(t, "Ana", view["name"])
	assert.EqualValues(t, 6, view["party_size"])
	assert.Equal(t, "requested", view["status"])
	for _, hidden := range []string{"ch_1", "conv_9", "meta_ctwa", "120210000000001", "origin"} {
		assert.NotContains(t, text, hidden, "the model never sees the origin")
	}
}

func TestCreateMyReservation_NoChannelHeader(t *testing.T) {
	backend := reservationBackend(t, nil, nil)
	cs := connectCustomer(t, backend, callerHeaders(map[string]string{conversationIDHeader: "conv_9"}))

	text, isErr := call(t, cs, "create_my_reservation", map[string]any{"name": "Ana", "party_size": 2, "starts_at": "2030-01-01T20:00:00-05:00"})

	assert.True(t, isErr)
	assert.Contains(t, text, "channel is missing")
	assert.Empty(t, backend.recorded(), "no reservation is created")
}

func TestCreateMyReservation_ValidationNamesTheField(t *testing.T) {
	backend := reservationBackend(t, nil, nil)
	cs := connectCustomer(t, backend, callerHeaders(map[string]string{channelIDHeader: "ch_1"}))

	text, isErr := call(t, cs, "create_my_reservation", map[string]any{"name": "Ana", "party_size": 80, "starts_at": "2030-01-01T20:00:00-05:00"})

	assert.True(t, isErr)
	assert.Contains(t, text, "party_size")
}

func TestCreateMyReservation_DescriptionSaysStaffConfirm(t *testing.T) {
	description := toolDescription(t, connectInMemory(t, NewMCPServer(testClient("http://backend.invalid"), ToolsetCustomer)), "create_my_reservation")

	assert.Contains(t, description, "staff will confirm")
}

func TestGetMyReservations_OnlyTheCallersNewestFirst(t *testing.T) {
	now := time.Now()
	stored := []*dto.Reservation{
		{ID: "r-old", ContactPhone: callerPhone, Name: "Ana", PartySize: 2, StartsAt: now.Add(-2 * time.Hour), Status: dto.ReservationStatusVisited, Origin: dto.Origin{ChannelID: &channelForTest}},
		{ID: "r-new", ContactPhone: callerPhone, Name: "Ana", PartySize: 4, StartsAt: now.Add(48 * time.Hour), Status: dto.ReservationStatusCancelled},
		{ID: "r-other", ContactPhone: otherPhone, Name: "Luis", PartySize: 3, StartsAt: now.Add(24 * time.Hour), Status: dto.ReservationStatusConfirmed},
	}
	backend := reservationBackend(t, stored, nil)
	cs := connectCustomer(t, backend, callerHeaders(nil))

	text, isErr := call(t, cs, "get_my_reservations", nil)

	require.False(t, isErr, text)
	var got myReservations
	require.NoError(t, json.Unmarshal([]byte(text), &got))
	require.Len(t, got.Reservations, 2)
	assert.Equal(t, "r-new", got.Reservations[0].ReservationID)
	assert.Equal(t, dto.ReservationStatusCancelled, got.Reservations[0].Status)
	assert.Equal(t, "r-old", got.Reservations[1].ReservationID)
	assert.NotContains(t, text, "Luis")
	assert.NotContains(t, text, "ch_1")

	gets := backend.requestsTo(http.MethodGet, "/api/reservations")
	require.Len(t, gets, 1)
	assert.Contains(t, gets[0].Query, "phone=%2B573005551234")
	assert.Contains(t, gets[0].Query, "from=")
}

func TestCancelMyReservation(t *testing.T) {
	stored := []*dto.Reservation{
		{ID: testReservationID, ContactPhone: callerPhone, Name: "Ana", PartySize: 2, StartsAt: time.Now().Add(24 * time.Hour), Status: dto.ReservationStatusConfirmed},
		{ID: "019a0000-0000-7000-8000-0000000000b2", ContactPhone: otherPhone, Status: dto.ReservationStatusConfirmed},
	}
	t.Run("tomorrow's table", func(t *testing.T) {
		backend := reservationBackend(t, stored, nil)
		cs := connectCustomer(t, backend, callerHeaders(nil))

		text, isErr := call(t, cs, "cancel_my_reservation", map[string]any{"reservation_id": testReservationID})

		require.False(t, isErr, text)
		assert.Contains(t, text, `"status":"cancelled"`)
		posts := backend.requestsTo(http.MethodPost, "/api/reservations/")
		require.Len(t, posts, 1)
		assert.JSONEq(t, `{"customer_phone":"+573005551234"}`, string(posts[0].Body))
	})
	t.Run("another guest's reservation", func(t *testing.T) {
		backend := reservationBackend(t, stored, nil)
		cs := connectCustomer(t, backend, callerHeaders(nil))

		text, isErr := call(t, cs, "cancel_my_reservation", map[string]any{"reservation_id": "019a0000-0000-7000-8000-0000000000b2"})

		assert.True(t, isErr)
		assert.Equal(t, errReservationNotFound.Error(), text)
	})
	t.Run("not an id", func(t *testing.T) {
		backend := reservationBackend(t, stored, nil)
		cs := connectCustomer(t, backend, callerHeaders(nil))

		text, isErr := call(t, cs, "cancel_my_reservation", map[string]any{"reservation_id": "r1; drop table"})

		assert.True(t, isErr)
		assert.Equal(t, errReservationNotFound.Error(), text, "the same answer, revealing nothing")
		assert.Empty(t, backend.recorded())
	})
}

func feedbackAnswer(status int, response dto.RecordFeedbackResponse) func(w http.ResponseWriter, b *fakeBackend) {
	return func(w http.ResponseWriter, b *fakeBackend) {
		if status != http.StatusOK {
			b.writeJSON(w, status, map[string]string{"error": "no visit awaiting feedback"})
			return
		}
		b.writeJSON(w, http.StatusOK, response)
	}
}

func TestRecordMyFeedback(t *testing.T) {
	link := "https://g.page/r/abc/review"
	tests := []struct {
		name     string
		rating   int
		answer   func(w http.ResponseWriter, b *fakeBackend)
		wantErr  string
		wantJSON string
	}{
		{name: "a happy guest, link switched off", rating: 5,
			answer: feedbackAnswer(http.StatusOK, dto.RecordFeedbackResponse{Feedback: &dto.Feedback{Rating: 5, ContactPhone: callerPhone}}), wantJSON: `{"escalate":false}`},
		{name: "an unhappy guest", rating: 2,
			answer: feedbackAnswer(http.StatusOK, dto.RecordFeedbackResponse{Feedback: &dto.Feedback{Rating: 2}, Escalate: true}), wantJSON: `{"escalate":true}`},
		{name: "link on, unhappy guest", rating: 2,
			answer:   feedbackAnswer(http.StatusOK, dto.RecordFeedbackResponse{Feedback: &dto.Feedback{Rating: 2}, Escalate: true, ReviewURL: &link}),
			wantJSON: `{"escalate":true,"review_url":"https://g.page/r/abc/review"}`},
		{name: "nothing to answer", rating: 5, answer: feedbackAnswer(http.StatusNotFound, dto.RecordFeedbackResponse{}), wantErr: "no visit awaiting feedback"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend := reservationBackend(t, nil, tt.answer)
			cs := connectCustomer(t, backend, callerHeaders(nil))

			text, isErr := call(t, cs, "record_my_feedback", map[string]any{"rating": tt.rating, "comment": "ok"})

			if tt.wantErr != "" {
				assert.True(t, isErr)
				assert.Contains(t, text, tt.wantErr)
				return
			}
			require.False(t, isErr, text)
			assert.JSONEq(t, tt.wantJSON, text)
			posts := backend.requestsTo(http.MethodPost, "/api/feedback")
			require.Len(t, posts, 1)
			assert.Contains(t, string(posts[0].Body), `"customer_phone":"+573005551234"`)
		})
	}
}

func TestRecordMyFeedback_DescriptionKeepsReviewsPolicySafe(t *testing.T) {
	description := toolDescription(t, connectInMemory(t, NewMCPServer(testClient("http://backend.invalid"), ToolsetCustomer)), "record_my_feedback")

	assert.Contains(t, description, "review_url, share it")
	assert.Contains(t, description, "hand the conversation to staff")
	assert.Contains(t, description, "Never offer")
}

func TestCreateDeliveryOrder_AnOrderFromAnAd(t *testing.T) {
	backend := newFakeBackend(t)
	cs := connectCustomer(t, backend, callerHeaders(map[string]string{
		channelIDHeader: "ch_1", attributionSourceHeader: "meta_ctwa", attributionAdIDHeader: "120210000000001",
	}))

	text, isErr := call(t, cs, "create_delivery_order", map[string]any{
		"order_type": "takeaway", "lines": []map[string]any{{"product_id": "019a0000-0000-7000-8000-000000000101", "quantity": 1}},
	})

	require.False(t, isErr, text)
	var sent dto.CreateOrderRequest
	require.NoError(t, json.Unmarshal(backend.requestsTo(http.MethodPost, "/api/orders")[0].Body, &sent))
	require.NotNil(t, sent.Origin)
	assert.Equal(t, "ch_1", *sent.Origin.ChannelID)
	assert.Equal(t, "meta_ctwa", *sent.Origin.Attribution.Source)
	assert.Equal(t, "120210000000001", *sent.Origin.Attribution.AdID)
	for _, hidden := range []string{"meta_ctwa", "120210000000001", "ch_1", "origin"} {
		assert.NotContains(t, text, hidden)
	}
}

func TestCreateDeliveryOrder_BeforeMarketingAppSendsTheHeaders(t *testing.T) {
	backend := newFakeBackend(t)
	cs := connectCustomer(t, backend, callerHeaders(nil))

	_, isErr := call(t, cs, "create_delivery_order", map[string]any{
		"order_type": "takeaway", "lines": []map[string]any{{"product_id": "019a0000-0000-7000-8000-000000000101", "quantity": 1}},
	})

	require.False(t, isErr)
	assert.NotContains(t, string(backend.requestsTo(http.MethodPost, "/api/orders")[0].Body), "origin")
}

func TestCustomerTools_NoToolTakesTheOriginAsAnArgument(t *testing.T) {
	cs := connectInMemory(t, NewMCPServer(testClient("http://backend.invalid"), ToolsetCustomer))
	res, err := cs.ListTools(context.Background(), nil)
	require.NoError(t, err)

	for _, tool := range res.Tools {
		schema, err := json.Marshal(tool.InputSchema)
		require.NoError(t, err)
		for _, forbidden := range []string{"channel", "conversation", "attribution", "ad_id", "clid", "customer_phone"} {
			assert.NotContains(t, strings.ToLower(string(schema)), forbidden, "%s must not take %s", tool.Name, forbidden)
		}
	}
}

func TestCustomerToolset_AStaffReservationToolIsUnknown(t *testing.T) {
	backend := newFakeBackend(t)
	cs := connectCustomer(t, backend, callerHeaders(nil))

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "confirm_reservation", Arguments: map[string]any{"id": testReservationID}})

	if err == nil {
		assert.True(t, res.IsError)
	}
	assert.Empty(t, backend.recorded(), "nothing is sent to the backend")
}

var staffReservationTools = []string{
	"list_reservations", "get_reservation", "create_reservation", "confirm_reservation", "decline_reservation",
	"cancel_reservation", "mark_reservation_no_show", "list_feedback", "get_contact",
}

func TestNewMCPServer_FullToolsetHasTheStaffReservationTools(t *testing.T) {
	names := toolNames(t, connectInMemory(t, NewMCPServer(testClient("http://backend.invalid"), ToolsetFull)))

	for _, name := range staffReservationTools {
		assert.Contains(t, names, name)
	}
}

func TestStaffReservationTools_PathsAndBodies(t *testing.T) {
	var got []recordedRequest
	mux := http.NewServeMux()
	mux.HandleFunc("/api/auth/signin", func(w http.ResponseWriter, _ *http.Request) { writeSignIn(w) })
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got = append(got, recordedRequest{Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery, Body: body})
		_, _ = w.Write([]byte(`{}`))
	})
	backend := httptest.NewServer(mux)
	defer backend.Close()
	cs := connectInMemory(t, NewMCPServer(testClient(backend.URL), ToolsetFull))

	calls := []struct {
		tool string
		args map[string]any
		want string
		body string
	}{
		{"list_reservations", map[string]any{"date": "2026-11-08", "status": "requested"}, "GET /api/reservations?date=2026-11-08&status=requested", ""},
		{"get_reservation", map[string]any{"id": "r1"}, "GET /api/reservations/r1", ""},
		{"create_reservation", map[string]any{"customer_phone": "3005551234", "name": "Ana", "party_size": 4, "starts_at": "2026-11-09T20:00:00-05:00", "source": "phone"},
			"POST /api/reservations", `{"customer_phone":"3005551234","name":"Ana","party_size":4,"starts_at":"2026-11-09T20:00:00-05:00","source":"phone"}`},
		{"confirm_reservation", map[string]any{"id": "r1"}, "POST /api/reservations/r1/confirm", `{}`},
		{"decline_reservation", map[string]any{"id": "r1", "reason": "Fully booked"}, "POST /api/reservations/r1/decline", `{"reason":"Fully booked"}`},
		{"cancel_reservation", map[string]any{"id": "r1"}, "POST /api/reservations/r1/cancel", `{}`},
		{"mark_reservation_no_show", map[string]any{"id": "r1"}, "POST /api/reservations/r1/no-show", `{}`},
		{"list_feedback", map[string]any{"from": "2026-11-02", "needs_attention": true}, "GET /api/feedback?from=2026-11-02&needs_attention=true", ""},
		{"get_contact", map[string]any{"id": "c1"}, "GET /api/contacts/c1", ""},
	}
	for _, c := range calls {
		text, isErr := call(t, cs, c.tool, c.args)
		require.False(t, isErr, "%s: %s", c.tool, text)
	}

	require.Len(t, got, len(calls))
	for i, c := range calls {
		line := got[i].Method + " " + got[i].Path
		if got[i].Query != "" {
			line += "?" + got[i].Query
		}
		assert.Equal(t, c.want, line, c.tool)
		if c.body != "" {
			assert.JSONEq(t, c.body, string(got[i].Body), c.tool)
		}
	}
}

func toolDescription(t *testing.T, cs *mcp.ClientSession, name string) string {
	t.Helper()
	res, err := cs.ListTools(context.Background(), nil)
	require.NoError(t, err)
	for _, tool := range res.Tools {
		if tool.Name == name {
			return tool.Description
		}
	}
	t.Fatalf("tool %s not listed", name)
	return ""
}
