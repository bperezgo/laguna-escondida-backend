package shared

import (
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
)

const (
	maxOriginIDLength       = 128
	maxAdIDLength           = 64
	maxCampaignNameLength   = 255
	maxClickedAtClockSkew   = 5 * time.Minute
	originFieldClickedAt    = "origin.attribution.clicked_at"
	originFieldSource       = "origin.attribution.source"
	originFieldAdID         = "origin.attribution.ad_id"
	originFieldCampaignID   = "origin.attribution.campaign_id"
	originFieldCampaign     = "origin.attribution.campaign_name"
	originFieldChannel      = "origin.channel_id"
	originFieldConversation = "origin.conversation_id"
)

var attributionSourcePattern = regexp.MustCompile(`^[a-z0-9_]{1,32}$`)

// NewOrigin validates a create request's origin. Each attribution field stands alone: a bad one is
// dropped and named in dropped, and never fails the booking. Only an over-long channel or
// conversation id is an error, since those route messages back to the guest.
func NewOrigin(req *dto.OriginRequest, now time.Time) (origin dto.Origin, dropped []string, err error) {
	if req == nil {
		return dto.Origin{}, nil, nil
	}
	if origin.ChannelID, err = originID(req.ChannelID, originFieldChannel); err != nil {
		return dto.Origin{}, nil, err
	}
	if origin.ConversationID, err = originID(req.ConversationID, originFieldConversation); err != nil {
		return dto.Origin{}, nil, err
	}
	if req.Attribution == nil {
		return origin, nil, nil
	}

	a := req.Attribution
	var attribution dto.Attribution
	keep := func(raw *string, field string, valid func(string) bool) *string {
		value := trimmed(raw)
		if value == nil {
			return nil
		}
		if !valid(*value) {
			dropped = append(dropped, field)
			return nil
		}
		return value
	}
	attribution.Source = keep(a.Source, originFieldSource, attributionSourcePattern.MatchString)
	attribution.AdID = keep(a.AdID, originFieldAdID, maxRunes(maxAdIDLength))
	attribution.CampaignID = keep(a.CampaignID, originFieldCampaignID, maxRunes(maxAdIDLength))
	attribution.CampaignName = keep(a.CampaignName, originFieldCampaign, maxRunes(maxCampaignNameLength))
	if clicked := trimmed(a.ClickedAt); clicked != nil {
		at, parseErr := time.Parse(time.RFC3339, *clicked)
		if parseErr != nil || at.After(now.Add(maxClickedAtClockSkew)) {
			dropped = append(dropped, originFieldClickedAt)
		} else {
			attribution.ClickedAt = &at
		}
	}
	if !AttributionIsEmpty(&attribution) {
		origin.Attribution = &attribution
	}
	return origin, dropped, nil
}

func AttributionIsEmpty(a *dto.Attribution) bool {
	return a == nil || (a.Source == nil && a.AdID == nil && a.ClickedAt == nil && a.CampaignID == nil && a.CampaignName == nil)
}

func originID(raw *string, field string) (*string, error) {
	value := trimmed(raw)
	if value == nil {
		return nil, nil
	}
	if utf8.RuneCountInString(*value) > maxOriginIDLength {
		return nil, &domainError.ValidationError{Field: field, Reason: "must be at most 128 characters"}
	}
	return value, nil
}

func maxRunes(limit int) func(string) bool {
	return func(value string) bool { return utf8.RuneCountInString(value) <= limit }
}

func trimmed(value *string) *string {
	if value == nil {
		return nil
	}
	t := strings.TrimSpace(*value)
	if t == "" {
		return nil
	}
	return &t
}
