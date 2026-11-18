package lead

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"laguna-escondida/backend/internal/domain/aggregate/shared"
	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"

	"github.com/google/uuid"
)

const (
	maxNameLength           = 100
	maxRestaurantNameLength = 120
	maxNotesLength          = 2000
	maxUTMLength            = 255
	maxClickIDLength        = 512
	maxURLLength            = 2048
	homeRegion              = "CO"
)

var eventIDPattern = regexp.MustCompile(`^[A-Za-z0-9-]{8,64}$`)

// NewFromSubmission validates a public submission. Required fields fail the whole lead;
// invalid attribution fields are dropped and named in dropped so the caller can log them.
func NewFromSubmission(req *dto.SubmitLeadRequest, now time.Time) (lead *dto.Lead, dropped []string, err error) {
	name, err := requiredText("name", req.Name, maxNameLength)
	if err != nil {
		return nil, nil, err
	}
	restaurant, err := requiredText("restaurant_name", req.RestaurantName, maxRestaurantNameLength)
	if err != nil {
		return nil, nil, err
	}
	phone, err := shared.NewPhone(req.Phone, homeRegion)
	if err != nil {
		return nil, nil, &domainError.ValidationError{Field: "phone", Reason: "is not a valid phone number"}
	}
	if !req.Consent {
		return nil, nil, &domainError.ValidationError{Field: "consent", Reason: "is required"}
	}

	attribution, dropped := cleanAttribution(req.LeadAttribution)
	return &dto.Lead{
		ID:              uuid.Must(uuid.NewV7()).String(),
		Name:            name,
		RestaurantName:  restaurant,
		Phone:           phone.Value(),
		Status:          dto.LeadStatusNew,
		ConsentAt:       now,
		LeadAttribution: attribution,
		CreatedAt:       now,
		UpdatedAt:       now,
	}, dropped, nil
}

// ApplyUpdate changes status and notes only; nothing else about a lead is editable.
func ApplyUpdate(lead *dto.Lead, req *dto.UpdateLeadRequest, now time.Time) error {
	if req.Status != nil {
		status := dto.LeadStatus(*req.Status)
		if !status.Valid() {
			return &domainError.ValidationError{Field: "status", Reason: "must be one of new, contacted, won, lost"}
		}
		lead.Status = status
	}
	if req.Notes != nil {
		notes := strings.TrimSpace(*req.Notes)
		if utf8.RuneCountInString(notes) > maxNotesLength {
			return &domainError.ValidationError{Field: "notes", Reason: "must be at most 2000 characters"}
		}
		lead.Notes = notes
	}
	lead.UpdatedAt = now
	return nil
}

func requiredText(field, raw string, max int) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" || utf8.RuneCountInString(value) > max {
		return "", &domainError.ValidationError{Field: field, Reason: "is required and must be at most " + strconv.Itoa(max) + " characters"}
	}
	return value, nil
}

func cleanAttribution(in dto.LeadAttribution) (dto.LeadAttribution, []string) {
	var dropped []string
	keep := func(field, value string, ok bool) string {
		if value == "" {
			return ""
		}
		if !ok {
			dropped = append(dropped, field)
			return ""
		}
		return value
	}
	text := func(field, raw string, max int) string {
		v := strings.TrimSpace(raw)
		return keep(field, v, utf8.RuneCountInString(v) <= max)
	}
	link := func(field, raw string) string {
		v := strings.TrimSpace(raw)
		return keep(field, v, utf8.RuneCountInString(v) <= maxURLLength && isAbsoluteHTTPURL(v))
	}
	eventID := strings.TrimSpace(in.EventID)
	return dto.LeadAttribution{
		UTMSource:   text("utm_source", in.UTMSource, maxUTMLength),
		UTMMedium:   text("utm_medium", in.UTMMedium, maxUTMLength),
		UTMCampaign: text("utm_campaign", in.UTMCampaign, maxUTMLength),
		UTMTerm:     text("utm_term", in.UTMTerm, maxUTMLength),
		UTMContent:  text("utm_content", in.UTMContent, maxUTMLength),
		FBCLID:      text("fbclid", in.FBCLID, maxClickIDLength),
		GCLID:       text("gclid", in.GCLID, maxClickIDLength),
		LandingURL:  link("landing_url", in.LandingURL),
		Referrer:    link("referrer", in.Referrer),
		EventID:     keep("event_id", eventID, eventIDPattern.MatchString(eventID)),
	}, dropped
}

func isAbsoluteHTTPURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}
