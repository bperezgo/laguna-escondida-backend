package mcpserver

import (
	"context"
	"net/url"
	"strconv"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"laguna-escondida/backend/internal/domain/dto"
)

type listReservationsInput struct {
	Date   *string `json:"date,omitempty" jsonschema:"the day as YYYY-MM-DD in the restaurant's timezone (default today)"`
	Status *string `json:"status,omitempty" jsonschema:"only these statuses, comma-separated: requested, confirmed, visited, declined, cancelled, no_show"`
}

type reservationStaffIDInput struct {
	ID string `json:"id" jsonschema:"the reservation id (UUID)"`
}

type createReservationInput struct {
	CustomerPhone string  `json:"customer_phone" jsonschema:"the guest's phone, national or +international"`
	Name          string  `json:"name" jsonschema:"the name the table is booked under"`
	PartySize     int     `json:"party_size" jsonschema:"how many people, 1 to 50"`
	StartsAt      string  `json:"starts_at" jsonschema:"date and time with offset (RFC 3339), e.g. 2026-11-09T20:00:00-05:00"`
	Notes         *string `json:"notes,omitempty" jsonschema:"optional notes"`
	Source        string  `json:"source" jsonschema:"how it was booked: phone, walk_in or staff"`
}

type declineReservationInput struct {
	ID     string  `json:"id" jsonschema:"the reservation id (UUID)"`
	Reason *string `json:"reason,omitempty" jsonschema:"why, told to the guest, e.g. Fully booked"`
}

type listFeedbackInput struct {
	From           *string `json:"from,omitempty" jsonschema:"first day, YYYY-MM-DD (default this week's Monday)"`
	To             *string `json:"to,omitempty" jsonschema:"last day, YYYY-MM-DD, inclusive (default six days after from)"`
	NeedsAttention *bool   `json:"needs_attention,omitempty" jsonschema:"only ratings of 1 to 3"`
}

type contactIDInput struct {
	ID string `json:"id" jsonschema:"the contact id (UUID)"`
}

func setIfPresent(q url.Values, key string, value *string) {
	if value != nil && *value != "" {
		q.Set(key, *value)
	}
}

func registerReservationTools(s *mcp.Server, c *Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_reservations",
		Description: "List one day's reservations (cloud only), in time order, in every status unless filtered, with each guest's phone and ad attribution.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in listReservationsInput) (*mcp.CallToolResult, any, error) {
		q := url.Values{}
		setIfPresent(q, "date", in.Date)
		setIfPresent(q, "status", in.Status)
		body, err := c.Get(ctx, "/api/reservations", q)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_reservation",
		Description: "Read one reservation by id (cloud only), with its origin and whether feedback was requested.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in reservationStaffIDInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Get(ctx, "/api/reservations/"+url.PathEscape(in.ID), nil)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "create_reservation",
		Description: "Enter a reservation taken by phone, at the door or by staff (cloud only). It is stored as confirmed.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in createReservationInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Post(ctx, "/api/reservations", dto.CreateReservationRequest{
			CustomerPhone: in.CustomerPhone, Name: in.Name, PartySize: in.PartySize, StartsAt: in.StartsAt,
			Notes: in.Notes, Source: dto.ReservationSource(in.Source),
		})
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	transition := func(name, path, description string) {
		mcp.AddTool(s, &mcp.Tool{Name: name, Description: description},
			func(ctx context.Context, _ *mcp.CallToolRequest, in reservationStaffIDInput) (*mcp.CallToolResult, any, error) {
				body, err := c.Post(ctx, "/api/reservations/"+url.PathEscape(in.ID)+path, struct{}{})
				if err != nil {
					return toolError(err)
				}
				return ok(body)
			})
	}
	transition("confirm_reservation", "/confirm", "Confirm a requested reservation (cloud only). A WhatsApp guest is messaged that it is confirmed.")
	transition("cancel_reservation", "/cancel", "Cancel a requested or confirmed reservation as staff (cloud only). A WhatsApp guest is told.")
	transition("mark_reservation_no_show", "/no-show",
		"Mark a confirmed reservation, or one marked visited before its feedback was asked, as a no-show (cloud only). No feedback is then requested.")

	mcp.AddTool(s, &mcp.Tool{
		Name:        "decline_reservation",
		Description: "Decline a requested reservation (cloud only), with an optional reason the WhatsApp guest is told.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in declineReservationInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Post(ctx, "/api/reservations/"+url.PathEscape(in.ID)+"/decline", dto.DeclineReservationRequest{Reason: in.Reason})
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_feedback",
		Description: "List guests' feedback recorded between two days (cloud only), newest first, each with rating, comment, phone and reservation. needs_attention lists ratings of 1 to 3 only.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in listFeedbackInput) (*mcp.CallToolResult, any, error) {
		q := url.Values{}
		setIfPresent(q, "from", in.From)
		setIfPresent(q, "to", in.To)
		if in.NeedsAttention != nil {
			q.Set("needs_attention", strconv.FormatBool(*in.NeedsAttention))
		}
		body, err := c.Get(ctx, "/api/feedback", q)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_contact",
		Description: "Read a guest contact by id (cloud only): phone, name, the ad that first brought them, their reservations and their feedback, newest first.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in contactIDInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Get(ctx, "/api/contacts/"+url.PathEscape(in.ID), nil)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})
}
