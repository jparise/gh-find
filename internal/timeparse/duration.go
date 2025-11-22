// Package timeparse provides extended time and duration parsing utilities.
package timeparse

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

var units = map[string]time.Duration{
	"s": time.Second,
	"m": time.Minute,
	"h": time.Hour,
	"d": 24 * time.Hour,
	"w": 7 * 24 * time.Hour,
	// Aliases
	"day":   24 * time.Hour,
	"days":  24 * time.Hour,
	"week":  7 * 24 * time.Hour,
	"weeks": 7 * 24 * time.Hour,
}

// ParseDuration parses a simple duration string for file modification times.
// Supports: s (seconds), m (minutes), h (hours), d/day/days, w/week/weeks.
// Examples: "10h", "2d", "3weeks", "30days".
func ParseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty duration string")
	}

	// Find where the unit starts (first non-digit)
	i := 0
	for i < len(s) && (s[i] >= '0' && s[i] <= '9') {
		i++
	}

	if i == 0 {
		return 0, fmt.Errorf("invalid duration %q: missing number", s)
	}
	if i == len(s) {
		return 0, fmt.Errorf("invalid duration %q: missing unit", s)
	}

	// Parse the number
	numStr := s[:i]
	num, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q: %w", s, err)
	}
	if num < 0 {
		return 0, fmt.Errorf("invalid duration %q: negative durations not supported", s)
	}

	// Parse the unit
	unitStr := strings.TrimSpace(s[i:])
	unit, ok := units[unitStr]
	if !ok {
		return 0, fmt.Errorf("invalid duration %q: unknown unit %q", s, unitStr)
	}

	// Check for overflow: num * unit must fit in time.Duration (int64)
	if num > math.MaxInt64/int64(unit) {
		return 0, fmt.Errorf("invalid duration %q: value too large", s)
	}

	return time.Duration(num) * unit, nil
}
