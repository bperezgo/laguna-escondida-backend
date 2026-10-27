// Package ordering holds the rules for when an organization takes delivery and takeaway
// orders: schedule validation, applying a settings patch, and evaluating availability.
package ordering

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
)

const (
	MaxWindowsPerType     = 21
	maxPausedReasonLength = 255
	minutesPerDay         = 24 * 60
	minutesPerWeek        = 7 * minutesPerDay
	lookaheadDays         = 7
)

var clockPattern = regexp.MustCompile(`^([01]\d|2[0-3]):([0-5]\d)$`)

var dayCodes = map[string]time.Weekday{
	"mon": time.Monday, "tue": time.Tuesday, "wed": time.Wednesday, "thu": time.Thursday,
	"fri": time.Friday, "sat": time.Saturday, "sun": time.Sunday,
}

type window struct {
	day    time.Weekday
	opens  int
	closes int
}

func (w window) pastMidnight() bool {
	return w.closes < w.opens
}

// weekStart and weekEnd place the window on a Monday-based week; weekEnd may pass the week's end.
func (w window) weekStart() int {
	return ((int(w.day)+6)%7)*minutesPerDay + w.opens
}

func (w window) weekEnd() int {
	length := w.closes - w.opens
	if w.pastMidnight() {
		length += minutesPerDay
	}
	return w.weekStart() + length
}

func parseClock(value string) (int, bool) {
	m := clockPattern.FindStringSubmatch(value)
	if m == nil {
		return 0, false
	}
	h, _ := strconv.Atoi(m[1])
	mins, _ := strconv.Atoi(m[2])
	return h*60 + mins, true
}

func parseWindow(w dto.OpeningWindow) (window, string) {
	day, ok := dayCodes[w.Day]
	if !ok {
		return window{}, "day must be one of mon, tue, wed, thu, fri, sat, sun"
	}
	opens, ok := parseClock(w.Opens)
	if !ok {
		return window{}, "opens must be a 24-hour HH:MM time"
	}
	closes, ok := parseClock(w.Closes)
	if !ok {
		return window{}, "closes must be a 24-hour HH:MM time"
	}
	if opens == closes {
		return window{}, "opens and closes must differ"
	}
	return window{day: day, opens: opens, closes: closes}, ""
}

func overlaps(a, b window) bool {
	for _, shift := range []int{-minutesPerWeek, 0, minutesPerWeek} {
		if a.weekStart() < b.weekEnd()+shift && b.weekStart()+shift < a.weekEnd() {
			return true
		}
	}
	return false
}

// ValidateSchedule checks a type's whole schedule; a past-midnight window counts against the next day's.
func ValidateSchedule(orderType dto.OrderType, schedule []dto.OpeningWindow) error {
	field := "ordering." + string(orderType) + ".schedule"
	if len(schedule) > MaxWindowsPerType {
		return &domainError.OrderingSettingsValidationError{
			Field: field, Reason: fmt.Sprintf("at most %d windows", MaxWindowsPerType),
		}
	}
	parsed := make([]window, 0, len(schedule))
	for i, w := range schedule {
		pw, reason := parseWindow(w)
		if reason != "" {
			return &domainError.OrderingSettingsValidationError{Field: fmt.Sprintf("%s[%d]", field, i), Reason: reason}
		}
		for j, earlier := range parsed {
			if overlaps(earlier, pw) {
				return &domainError.OrderingSettingsValidationError{
					Field: fmt.Sprintf("%s[%d]", field, i), Reason: fmt.Sprintf("overlaps %s[%d]", field, j),
				}
			}
		}
		parsed = append(parsed, pw)
	}
	return nil
}

// Normalize gives every type a non-nil schedule, so an unconfigured one reads as [].
func Normalize(settings dto.OrderingSettings) dto.OrderingSettings {
	for _, s := range []*dto.OrderTypeSettings{&settings.Delivery, &settings.Takeaway} {
		if s.Schedule == nil {
			s.Schedule = []dto.OpeningWindow{}
		}
		if !s.Paused {
			s.PausedReason = nil
		}
	}
	return settings
}

// ApplyPatch returns current with only the patch's present fields changed, or a validation error.
func ApplyPatch(current dto.OrderingSettings, patch *dto.OrderingSettingsPatch) (dto.OrderingSettings, error) {
	next := Normalize(current)
	if patch == nil {
		return next, nil
	}
	var err error
	if next.Delivery, err = applyTypePatch(dto.OrderTypeDelivery, next.Delivery, patch.Delivery); err != nil {
		return current, err
	}
	if next.Takeaway, err = applyTypePatch(dto.OrderTypeTakeaway, next.Takeaway, patch.Takeaway); err != nil {
		return current, err
	}
	return next, nil
}

func applyTypePatch(orderType dto.OrderType, current dto.OrderTypeSettings, patch *dto.OrderTypeSettingsPatch) (dto.OrderTypeSettings, error) {
	if patch == nil {
		return current, nil
	}
	next := current
	if patch.Paused != nil {
		next.Paused = *patch.Paused
	}
	if patch.PausedReason != nil {
		reason := strings.TrimSpace(*patch.PausedReason)
		if utf8.RuneCountInString(reason) > maxPausedReasonLength {
			return current, &domainError.OrderingSettingsValidationError{
				Field:  "ordering." + string(orderType) + ".paused_reason",
				Reason: fmt.Sprintf("at most %d characters", maxPausedReasonLength),
			}
		}
		next.PausedReason = nil
		if reason != "" {
			next.PausedReason = &reason
		}
	}
	if !next.Paused {
		next.PausedReason = nil
	}
	if patch.Schedule != nil {
		schedule := *patch.Schedule
		if err := ValidateSchedule(orderType, schedule); err != nil {
			return current, err
		}
		next.Schedule = append([]dto.OpeningWindow{}, schedule...)
	}
	return next, nil
}

// TouchesPause and TouchesSchedule tell which permissions a patch needs.
func TouchesPause(patch *dto.OrderingSettingsPatch) bool {
	if patch == nil {
		return false
	}
	for _, p := range []*dto.OrderTypeSettingsPatch{patch.Delivery, patch.Takeaway} {
		if p != nil && (p.Paused != nil || p.PausedReason != nil) {
			return true
		}
	}
	return false
}

func TouchesSchedule(patch *dto.OrderingSettingsPatch) bool {
	if patch == nil {
		return false
	}
	for _, p := range []*dto.OrderTypeSettingsPatch{patch.Delivery, patch.Takeaway} {
		if p != nil && p.Schedule != nil {
			return true
		}
	}
	return false
}

// Evaluate places now on loc's wall clock, so a window keeps its local hours across DST changes.
// A stored window that doesn't parse (hand-edited JSON) is ignored rather than closing the type.
func Evaluate(settings dto.OrderTypeSettings, loc *time.Location, now time.Time) dto.Availability {
	if settings.Paused {
		reason := dto.AvailabilityReasonPaused
		return dto.Availability{Reason: &reason, PausedReason: settings.PausedReason}
	}
	if len(settings.Schedule) == 0 {
		return dto.Availability{Available: true}
	}

	windows := make([]window, 0, len(settings.Schedule))
	for _, w := range settings.Schedule {
		if pw, reason := parseWindow(w); reason == "" {
			windows = append(windows, pw)
		}
	}
	if len(windows) == 0 {
		return dto.Availability{Available: true}
	}

	local := now.In(loc)
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	horizon := now.Add(lookaheadDays * 24 * time.Hour)
	var nextOpens *time.Time
	// From yesterday, for a past-midnight window still running in today's early hours.
	for offset := -1; offset <= lookaheadDays; offset++ {
		date := today.AddDate(0, 0, offset)
		for _, w := range windows {
			if w.day != date.Weekday() {
				continue
			}
			opensAt := atMinute(date, w.opens, loc)
			closeDate := date
			if w.pastMidnight() {
				closeDate = date.AddDate(0, 0, 1)
			}
			closesAt := atMinute(closeDate, w.closes, loc)
			if !now.Before(opensAt) && now.Before(closesAt) {
				return dto.Availability{Available: true, ClosesAt: &closesAt}
			}
			if opensAt.After(now) && !opensAt.After(horizon) && (nextOpens == nil || opensAt.Before(*nextOpens)) {
				t := opensAt
				nextOpens = &t
			}
		}
	}
	reason := dto.AvailabilityReasonOutsideHours
	return dto.Availability{Reason: &reason, NextOpensAt: nextOpens}
}

func atMinute(date time.Time, minute int, loc *time.Location) time.Time {
	return time.Date(date.Year(), date.Month(), date.Day(), minute/60, minute%60, 0, 0, loc)
}
