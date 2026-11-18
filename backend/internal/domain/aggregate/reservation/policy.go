package reservation

import (
	"time"

	"laguna-escondida/backend/internal/domain/aggregate/ordering"
	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
)

const (
	defaultMaxPartySize     = 8
	defaultMinNoticeMinutes = 120
	maxNoticeMinutes        = 7 * 24 * 60
)

// Decision is the status a new reservation is stored in, and why it waits for staff when it does.
type Decision struct {
	Status        dto.ReservationStatus
	PendingReason *dto.ReservationPendingReason
}

// DefaultSettings is what an organization that never saved the section reads.
func DefaultSettings() dto.ReservationSettings {
	return dto.ReservationSettings{
		MaxPartySize:     defaultMaxPartySize,
		MinNoticeMinutes: defaultMinNoticeMinutes,
		Schedule:         []dto.OpeningWindow{},
	}
}

func NormalizeSettings(settings dto.ReservationSettings) dto.ReservationSettings {
	if settings.Schedule == nil {
		settings.Schedule = []dto.OpeningWindow{}
	}
	return settings
}

// ApplySettingsPatch validates and merges a patch of the reservations section.
func ApplySettingsPatch(current dto.ReservationSettings, patch *dto.ReservationSettingsPatch) (dto.ReservationSettings, error) {
	next := NormalizeSettings(current)
	if patch == nil {
		return next, nil
	}
	if patch.AutoConfirmEnabled != nil {
		next.AutoConfirmEnabled = *patch.AutoConfirmEnabled
	}
	if patch.MaxPartySize != nil {
		if *patch.MaxPartySize < minPartySize || *patch.MaxPartySize > maxPartySize {
			return current, &domainError.ValidationError{Field: "reservations.max_party_size", Reason: "must be between 1 and 50"}
		}
		next.MaxPartySize = *patch.MaxPartySize
	}
	if patch.MinNoticeMinutes != nil {
		if *patch.MinNoticeMinutes < 0 || *patch.MinNoticeMinutes > maxNoticeMinutes {
			return current, &domainError.ValidationError{Field: "reservations.min_notice_minutes", Reason: "must be between 0 and 10080"}
		}
		next.MinNoticeMinutes = *patch.MinNoticeMinutes
	}
	if patch.DailyGuestCap != nil {
		if *patch.DailyGuestCap < 1 {
			return current, &domainError.ValidationError{Field: "reservations.daily_guest_cap", Reason: "must be a positive number of guests"}
		}
		guestCap := *patch.DailyGuestCap
		next.DailyGuestCap = &guestCap
	}
	if patch.Schedule != nil {
		if invalid := ordering.ValidateWindows("reservations.schedule", *patch.Schedule); invalid != nil {
			return current, invalid
		}
		next.Schedule = append([]dto.OpeningWindow{}, *patch.Schedule...)
	}
	if next.AutoConfirmEnabled && next.DailyGuestCap == nil {
		return current, &domainError.ValidationError{Field: "reservations.daily_guest_cap", Reason: "is required while auto-confirm is enabled"}
	}
	return next, nil
}

func TouchesSettings(patch *dto.ReservationSettingsPatch) bool {
	return patch != nil && (patch.AutoConfirmEnabled != nil || patch.MaxPartySize != nil ||
		patch.MinNoticeMinutes != nil || patch.DailyGuestCap != nil || patch.Schedule != nil)
}

// Decide checks a WhatsApp request against the policy, rule by rule, and never refuses it.
// confirmedGuests is called only when every other rule passed, so a disabled or failing policy
// never takes the day's lock.
func Decide(
	policy dto.ReservationSettings,
	req *dto.CreateReservationRequest,
	now time.Time,
	loc *time.Location,
	confirmedGuests func(dayStart, dayEnd time.Time) (int, error),
) (Decision, error) {
	if !policy.AutoConfirmEnabled {
		return pending(dto.PendingReasonDisabled), nil
	}
	if req.PartySize > policy.MaxPartySize {
		return pending(dto.PendingReasonPartySize), nil
	}
	startsAt, err := validateStartsAt(req.StartsAt, now)
	if err != nil {
		return Decision{}, err
	}
	if startsAt.Before(now.Add(time.Duration(policy.MinNoticeMinutes) * time.Minute)) {
		return pending(dto.PendingReasonNotice), nil
	}
	if len(policy.Schedule) > 0 && !ordering.Evaluate(dto.OrderTypeSettings{Schedule: policy.Schedule}, loc, startsAt).Available {
		return pending(dto.PendingReasonOutsideSchedule), nil
	}
	if policy.DailyGuestCap == nil {
		return pending(dto.PendingReasonDailyCap), nil
	}
	local := startsAt.In(loc)
	dayStart := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	guests, err := confirmedGuests(dayStart, dayStart.AddDate(0, 0, 1))
	if err != nil {
		return Decision{}, err
	}
	if guests+req.PartySize > *policy.DailyGuestCap {
		return pending(dto.PendingReasonDailyCap), nil
	}
	return Decision{Status: dto.ReservationStatusConfirmed}, nil
}

func pending(reason dto.ReservationPendingReason) Decision {
	return Decision{Status: dto.ReservationStatusRequested, PendingReason: &reason}
}
