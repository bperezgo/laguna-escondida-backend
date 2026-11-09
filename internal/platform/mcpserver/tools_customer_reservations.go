package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"laguna-escondida/backend/internal/domain/dto"
)

// customerReservationLookback: a guest still sees yesterday's table as theirs.
const customerReservationLookback = 24 * time.Hour

// errReservationNotFound is the same for another guest's reservation and one that can no longer be
// cancelled, so the tool never reveals whether an id exists.
var errReservationNotFound = errors.New("reservation not found: it is not one of this customer's upcoming reservations")

var errNoVisitAwaitingFeedback = errors.New("no visit awaiting feedback: this customer has no recent visit to rate")

type createMyReservationInput struct {
	Name      string  `json:"name" jsonschema:"the name the table is booked under"`
	PartySize int     `json:"party_size" jsonschema:"how many people, 1 to 50"`
	StartsAt  string  `json:"starts_at" jsonschema:"date and time with offset (RFC 3339), e.g. 2026-11-09T20:00:00-05:00"`
	Notes     *string `json:"notes,omitempty" jsonschema:"optional notes for the restaurant, e.g. a birthday or a high chair"`
}

type reservationIDInput struct {
	ReservationID string `json:"reservation_id" jsonschema:"the reservation id from create_my_reservation or get_my_reservations"`
}

type recordMyFeedbackInput struct {
	Rating  int     `json:"rating" jsonschema:"how the visit went, 1 (bad) to 5 (great)"`
	Comment *string `json:"comment,omitempty" jsonschema:"what the customer said, in their words"`
}

type reservationView struct {
	ReservationID string                `json:"reservation_id"`
	Name          string                `json:"name"`
	PartySize     int                   `json:"party_size"`
	StartsAt      time.Time             `json:"starts_at"`
	Status        dto.ReservationStatus `json:"status"`
}

type myReservations struct {
	Reservations []reservationView `json:"reservations"`
}

type feedbackView struct {
	Escalate  bool    `json:"escalate"`
	ReviewURL *string `json:"review_url,omitempty"`
}

func viewReservation(r *dto.Reservation) reservationView {
	return reservationView{ReservationID: r.ID, Name: r.Name, PartySize: r.PartySize, StartsAt: r.StartsAt, Status: r.Status}
}

func decodeReservation(body []byte) (*dto.Reservation, error) {
	var r dto.Reservation
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("decode reservation: %w", err)
	}
	return &r, nil
}

func registerCustomerReservationTools(s *mcp.Server, c *Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name: "create_my_reservation",
		Description: "Request a table for the customer in this chat. Confirm the name, party size, date and time with the customer first. " +
			"The reservation is created as requested, not confirmed: tell the customer that the restaurant's staff will confirm it, " +
			"and that they will be messaged once it is confirmed or declined.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in createMyReservationInput) (*mcp.CallToolResult, any, error) {
		phone, err := customerPhone(req)
		if err != nil {
			return toolError(err)
		}
		origin := customerOrigin(req)
		if origin == nil || origin.ChannelID == nil {
			return toolError(fmt.Errorf("the channel is missing: the channel did not send %s, so no confirmation could reach the customer; no reservation was created", channelIDHeader))
		}
		body, err := c.Post(ctx, "/api/reservations", dto.CreateReservationRequest{
			CustomerPhone: phone,
			Name:          in.Name,
			PartySize:     in.PartySize,
			StartsAt:      in.StartsAt,
			Notes:         in.Notes,
			Source:        dto.ReservationSourceWhatsApp,
			Origin:        origin,
		})
		if err != nil {
			return backendFailure("the reservation was not created", err)
		}
		created, err := decodeReservation(body)
		if err != nil {
			return toolError(err)
		}
		return jsonOK(json.Marshal(viewReservation(created)))
	})

	mcp.AddTool(s, &mcp.Tool{
		Name: "get_my_reservations",
		Description: "List this customer's reservations from yesterday on, newest first, each with its status: " +
			"requested (waiting for staff), confirmed, declined, cancelled, visited or no_show.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ noInput) (*mcp.CallToolResult, any, error) {
		phone, err := customerPhone(req)
		if err != nil {
			return toolError(err)
		}
		from := time.Now().Add(-customerReservationLookback).Format(time.RFC3339)
		body, err := c.Get(ctx, "/api/reservations", query("phone", phone, "from", from))
		if err != nil {
			return backendFailure("the reservations could not be loaded", err)
		}
		var list dto.ReservationListResponse
		if err := json.Unmarshal(body, &list); err != nil {
			return toolError(fmt.Errorf("decode reservations: %w", err))
		}
		views := make([]reservationView, 0, len(list.Reservations))
		for _, r := range list.Reservations {
			// Only the caller's, whatever the backend returned.
			if r != nil && r.ContactPhone == phone {
				views = append(views, viewReservation(r))
			}
		}
		sort.SliceStable(views, func(i, j int) bool { return views[i].StartsAt.After(views[j].StartsAt) })
		return jsonOK(json.Marshal(myReservations{Reservations: views}))
	})

	mcp.AddTool(s, &mcp.Tool{
		Name: "cancel_my_reservation",
		Description: "Cancel one of this customer's requested or confirmed reservations that has not started yet. " +
			"Fails with reservation not found for any other id; then only staff can help.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in reservationIDInput) (*mcp.CallToolResult, any, error) {
		phone, err := customerPhone(req)
		if err != nil {
			return toolError(err)
		}
		if _, parseErr := uuid.Parse(in.ReservationID); parseErr != nil {
			return toolError(errReservationNotFound)
		}
		body, err := c.Post(ctx, "/api/reservations/"+url.PathEscape(in.ReservationID)+"/cancel", dto.CancelReservationRequest{CustomerPhone: &phone})
		var apiErr *APIError
		if errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound {
			return toolError(errReservationNotFound)
		}
		if err != nil {
			return backendFailure("the reservation was not cancelled", err)
		}
		cancelled, err := decodeReservation(body)
		if err != nil {
			return toolError(err)
		}
		return jsonOK(json.Marshal(viewReservation(cancelled)))
	})

	mcp.AddTool(s, &mcp.Tool{
		Name: "record_my_feedback",
		Description: "Record how the customer's recent visit went, after the restaurant asked them: a rating from 1 to 5 and their comment. " +
			"If the result has review_url, share it and invite the customer to leave a review there. " +
			"If escalate is true, apologise and hand the conversation to staff. " +
			"Never offer a discount, gift or anything else in exchange for a review or a rating.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in recordMyFeedbackInput) (*mcp.CallToolResult, any, error) {
		phone, err := customerPhone(req)
		if err != nil {
			return toolError(err)
		}
		body, err := c.Post(ctx, "/api/feedback", dto.RecordFeedbackRequest{CustomerPhone: phone, Rating: in.Rating, Comment: in.Comment})
		var apiErr *APIError
		if errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound {
			return toolError(errNoVisitAwaitingFeedback)
		}
		if err != nil {
			return backendFailure("the feedback was not recorded", err)
		}
		var recorded dto.RecordFeedbackResponse
		if err := json.Unmarshal(body, &recorded); err != nil {
			return toolError(fmt.Errorf("decode feedback: %w", err))
		}
		return jsonOK(json.Marshal(feedbackView{Escalate: recorded.Escalate, ReviewURL: recorded.ReviewURL}))
	})
}
