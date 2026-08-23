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
	for _, layout := range []string{time.DateOnly, time.DateTime, time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid time format %q (expected YYYY-MM-DD, YYYY-MM-DD HH:MM:SS, or RFC3339)", s)
}
