package timeparse

import (
	"testing"
	"time"
)

func TestParseDuration(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    time.Duration
		wantErr bool
	}{
		// Basic time units
		{"seconds", "10s", 10 * time.Second, false},
		{"minutes", "5m", 5 * time.Minute, false},
		{"hours", "2h", 2 * time.Hour, false},

		// Days
		{"days short", "1d", 24 * time.Hour, false},
		{"days plural", "2days", 48 * time.Hour, false},
		{"day singular", "1day", 24 * time.Hour, false},

		// Weeks
		{"weeks short", "1w", 7 * 24 * time.Hour, false},
		{"weeks plural", "2weeks", 2 * 7 * 24 * time.Hour, false},
		{"week singular", "1week", 7 * 24 * time.Hour, false},

		// With whitespace
		{"with spaces", " 10h ", 10 * time.Hour, false},

		// Error cases
		{"empty string", "", 0, true},
		{"no unit", "123", 0, true},
		{"invalid unit", "10x", 0, true},
		{"sub-second not supported", "100ms", 0, true},
		{"no number", "s", 0, true},
		{"invalid format", "abc", 0, true},
		{"negative", "-10s", 0, true},
		{"combined units not supported", "1h30m", 0, true},
		{"fractional not supported", "1.5h", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDuration(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDuration(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ParseDuration(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
