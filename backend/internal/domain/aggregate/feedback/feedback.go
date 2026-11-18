package feedback

import (
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"

	"github.com/google/uuid"
)

const (
	minRating        = 1
	maxRating        = 5
	maxCommentLength = 1000
	// lowestHappyRating: anything below it needs a manager's attention.
	lowestHappyRating = 4
	// RequestOpenFor is how long a guest can answer a feedback request.
	RequestOpenFor = 7 * 24 * time.Hour
	// requestHour is the local hour, the day after the visit, from which feedback is asked.
	requestHour = 11
	// MaxVisitAge stops a backlog of old visits from being asked all at once.
	MaxVisitAge = 72 * time.Hour
)

// New validates a guest's answer to the request.
func New(requestID string, rating int, comment *string, now time.Time) (*dto.Feedback, error) {
	if rating < minRating || rating > maxRating {
		return nil, &domainError.ValidationError{Field: "rating", Reason: "must be a whole number from 1 to 5"}
	}
	cleaned := trimmedOrNil(comment)
	if cleaned != nil && utf8.RuneCountInString(*cleaned) > maxCommentLength {
		return nil, &domainError.ValidationError{Field: "comment", Reason: "must be at most 1000 characters"}
	}
	return &dto.Feedback{
		ID:                uuid.Must(uuid.NewV7()).String(),
		FeedbackRequestID: requestID,
		Rating:            rating,
		Comment:           cleaned,
		NeedsAttention:    NeedsAttention(rating),
		CreatedAt:         now,
		UpdatedAt:         now,
	}, nil
}

func NeedsAttention(rating int) bool {
	return rating < lowestHappyRating
}

// ReviewLink takes only the settings: the rating must never decide who is asked for a review.
func ReviewLink(settings dto.FeedbackSettings) *string {
	if !settings.ReviewLinkEnabled || settings.ReviewURL == "" {
		return nil
	}
	link := settings.ReviewURL
	return &link
}

// NewRequest opens a request for a visited reservation.
func NewRequest(reservation *dto.Reservation, now time.Time) *dto.FeedbackRequest {
	return &dto.FeedbackRequest{
		ID:            uuid.Must(uuid.NewV7()).String(),
		ReservationID: reservation.ID,
		ContactID:     reservation.ContactID,
		RequestedAt:   now,
		ExpiresAt:     now.Add(RequestOpenFor),
	}
}

// IsDue: from 11:00 local on the day after the reservation started, and only for a recent visit.
func IsDue(reservation *dto.Reservation, loc *time.Location, now time.Time) bool {
	if reservation.VisitedAt == nil || now.Sub(*reservation.VisitedAt) > MaxVisitAge {
		return false
	}
	local := reservation.StartsAt.In(loc)
	due := time.Date(local.Year(), local.Month(), local.Day()+1, requestHour, 0, 0, 0, loc)
	return !now.Before(due)
}

// ApplySettingsPatch validates and merges a patch of the feedback section.
func ApplySettingsPatch(current dto.FeedbackSettings, patch *dto.FeedbackSettingsPatch) (dto.FeedbackSettings, error) {
	if patch == nil {
		return current, nil
	}
	next := current
	if patch.ReviewLinkEnabled != nil {
		next.ReviewLinkEnabled = *patch.ReviewLinkEnabled
	}
	if patch.ReviewURL != nil {
		reviewURL := strings.TrimSpace(*patch.ReviewURL)
		if reviewURL != "" && !isAbsoluteHTTPS(reviewURL) {
			return current, &domainError.ValidationError{Field: "feedback.review_url", Reason: "must be an absolute https URL or empty"}
		}
		next.ReviewURL = reviewURL
	}
	return next, nil
}

func TouchesSettings(patch *dto.FeedbackSettingsPatch) bool {
	return patch != nil && (patch.ReviewLinkEnabled != nil || patch.ReviewURL != nil)
}

func isAbsoluteHTTPS(raw string) bool {
	parsed, err := url.Parse(raw)
	return err == nil && parsed.Scheme == "https" && parsed.Host != ""
}

func trimmedOrNil(value *string) *string {
	if value == nil {
		return nil
	}
	t := strings.TrimSpace(*value)
	if t == "" {
		return nil
	}
	return &t
}
