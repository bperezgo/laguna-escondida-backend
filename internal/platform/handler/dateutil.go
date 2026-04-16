package handler

import (
	"fmt"
	"time"
)

const simpleDateLayout = "2006-01-02"

var utcMinus5 = time.FixedZone("UTC-5", -5*60*60)

// parseStartDate parses a date string in RFC3339 or "2006-01-02" format.
// Simple dates are expanded to the start of the day (00:00:00 UTC-5).
func parseStartDate(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	if t, err := time.Parse(simpleDateLayout, s); err == nil {
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, utcMinus5), nil
	}
	return time.Time{}, fmt.Errorf("invalid date format %q, expected YYYY-MM-DD or RFC3339", s)
}

// parseEndDate parses a date string in RFC3339 or "2006-01-02" format.
// Simple dates are expanded to the end of the day (23:59:59 UTC-5).
func parseEndDate(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	if t, err := time.Parse(simpleDateLayout, s); err == nil {
		return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, utcMinus5), nil
	}
	return time.Time{}, fmt.Errorf("invalid date format %q, expected YYYY-MM-DD or RFC3339", s)
}
