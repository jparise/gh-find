package timeparse

import (
	"fmt"
	"time"
)

// ParseTime parses various date/time formats in UTC.
// Supported formats:
//   - YYYY-MM-DD (assumes 00:00:00 UTC)
//   - YYYY-MM-DD HH:MM:SS (UTC)
//   - RFC3339: 2018-10-27T10:00:00Z (can specify any timezone)
//
// Returns the parsed time or an error if the format is invalid.
func ParseTime(s string) (time.Time, error) {
	// Try parsing as date only (YYYY-MM-DD) - assume UTC
	if t, err := time.Parse(time.DateOnly, s); err == nil {
		return t, nil
	}

	// Try parsing as date and time (YYYY-MM-DD HH:MM:SS) - assume UTC
	if t, err := time.Parse(time.DateTime, s); err == nil {
		return t, nil
	}

	// Try parsing as RFC3339 (can specify timezone explicitly)
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}

	return time.Time{}, fmt.Errorf("invalid time format %q (expected YYYY-MM-DD, YYYY-MM-DD HH:MM:SS, or RFC3339)", s)
}
