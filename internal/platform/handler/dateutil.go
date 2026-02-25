package handler

import (
	"fmt"
	"time"
)

const simpleDateLayout = "2006-01-02"

// parseStartDate parses a date string in RFC3339 or "2006-01-02" format.
// Simple dates are expanded to the start of the day (00:00:00 UTC).
func parseStartDate(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	if t, err := time.Parse(simpleDateLayout, s); err == nil {
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), nil
	}
	return time.Time{}, fmt.Errorf("invalid date format %q, expected YYYY-MM-DD or RFC3339", s)
}

// parseEndDate parses a date string in RFC3339 or "2006-01-02" format.
// Simple dates are expanded to the end of the day (23:59:59 UTC).
func parseEndDate(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	if t, err := time.Parse(simpleDateLayout, s); err == nil {
		return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, time.UTC), nil
	}
	return time.Time{}, fmt.Errorf("invalid date format %q, expected YYYY-MM-DD or RFC3339", s)
}
