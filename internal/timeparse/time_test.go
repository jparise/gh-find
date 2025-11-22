package timeparse

import (
	"testing"
	"time"
)

func TestParseTime(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    time.Time
		wantErr bool
	}{
		{
			name:  "date only",
			input: "2018-10-27",
			want:  time.Date(2018, 10, 27, 0, 0, 0, 0, time.UTC),
		},
		{
			name:  "date and time",
			input: "2018-10-27 10:30:45",
			want:  time.Date(2018, 10, 27, 10, 30, 45, 0, time.UTC),
		},
		{
			name:  "RFC3339 with Z",
			input: "2018-10-27T10:00:00Z",
			want:  time.Date(2018, 10, 27, 10, 0, 0, 0, time.UTC),
		},
		{
			name:  "RFC3339 with offset",
			input: "2018-10-27T10:00:00-07:00",
			want:  time.Date(2018, 10, 27, 10, 0, 0, 0, time.FixedZone("", -7*3600)),
		},
		{
			name:    "invalid format",
			input:   "10/27/2018",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "invalid date",
			input:   "2018-13-45",
			wantErr: true,
		},
		{
			name:    "just text",
			input:   "not a date",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTime(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseTime(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && !got.Equal(tt.want) {
				t.Errorf("ParseTime(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
