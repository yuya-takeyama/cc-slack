package web

import (
	"testing"
	"time"
)

func TestConvertThreadTsToTime(t *testing.T) {
	tests := []struct {
		name      string
		threadTs  string
		wantError bool
		wantUnix  int64
		wantNanos int64
	}{
		{
			name:      "valid timestamp",
			threadTs:  "1753663466.387799",
			wantUnix:  1753663466,
			wantNanos: 387799000, // Expect microseconds converted to nanoseconds
		},
		{
			name:      "timestamp without decimal",
			threadTs:  "1753663466",
			wantUnix:  1753663466,
			wantNanos: 0,
		},
		{
			name:      "invalid timestamp",
			threadTs:  "invalid",
			wantError: true,
		},
		{
			name:      "empty timestamp",
			threadTs:  "",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ConvertThreadTsToTime(tt.threadTs)
			if (err != nil) != tt.wantError {
				t.Errorf("ConvertThreadTsToTime() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if !tt.wantError {
				// Check Unix seconds
				if got.Unix() != tt.wantUnix {
					t.Errorf("ConvertThreadTsToTime() Unix = %v, want %v", got.Unix(), tt.wantUnix)
				}
				// Check nanoseconds within reasonable precision (allow for floating point errors)
				gotNanos := got.UnixNano() - (got.Unix() * 1e9)
				if gotNanos < tt.wantNanos-1000 || gotNanos > tt.wantNanos+1000 {
					t.Errorf("ConvertThreadTsToTime() Nanos = %v, want ~%v", gotNanos, tt.wantNanos)
				}
			}
		})
	}
}

func TestFormatThreadTime(t *testing.T) {
	// Create a test time with a specific timezone
	loc, _ := time.LoadLocation("Asia/Tokyo")
	testTime := time.Date(2024, 12, 27, 14, 24, 26, 0, loc)

	tests := []struct {
		name     string
		time     time.Time
		expected string
	}{
		{
			name:     "format JST time",
			time:     testTime,
			expected: "2024-12-27 14:24:26 JST",
		},
		{
			name:     "format UTC time",
			time:     testTime.UTC(),
			expected: "2024-12-27 05:24:26 UTC",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatThreadTime(tt.time)
			if got != tt.expected {
				t.Errorf("FormatThreadTime() = %v, want %v", got, tt.expected)
			}
		})
	}
}
