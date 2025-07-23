package torrent

import (
	"testing"
	"time"
)

func TestFormatDuration(t *testing.T) {
	formatter := NewFormatter(false)

	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{
			name:     "milliseconds",
			duration: 500 * time.Millisecond,
			expected: "500ms",
		},
		{
			name:     "seconds only",
			duration: 45 * time.Second,
			expected: "45.0s",
		},
		{
			name:     "minutes and seconds",
			duration: 5*time.Minute + 30*time.Second,
			expected: "5m 30s",
		},
		{
			name:     "exactly one hour",
			duration: 1 * time.Hour,
			expected: "1h 0m 0s",
		},
		{
			name:     "1 hour 44 minutes (user's reported case)",
			duration: 1*time.Hour + 44*time.Minute,
			expected: "1h 44m 0s",
		},
		{
			name:     "1 hour 44 minutes with seconds",
			duration: 1*time.Hour + 44*time.Minute + 12*time.Second,
			expected: "1h 44m 12s",
		},
		{
			name:     "2 hours 32 minutes",
			duration: 2*time.Hour + 32*time.Minute,
			expected: "2h 32m 0s",
		},
		{
			name:     "hours minutes and seconds",
			duration: 3*time.Hour + 15*time.Minute + 45*time.Second,
			expected: "3h 15m 45s",
		},
		{
			name:     "24 hours",
			duration: 24 * time.Hour,
			expected: "24h 0m 0s",
		},
		{
			name:     "complex duration",
			duration: 5*time.Hour + 59*time.Minute + 59*time.Second,
			expected: "5h 59m 59s",
		},
		{
			name:     "zero duration",
			duration: 0,
			expected: "0ms",
		},
		{
			name:     "1 second exact",
			duration: 1 * time.Second,
			expected: "1.0s",
		},
		{
			name:     "59 seconds",
			duration: 59 * time.Second,
			expected: "59.0s",
		},
		{
			name:     "3.5 seconds",
			duration: 3*time.Second + 500*time.Millisecond,
			expected: "3.5s",
		},
		{
			name:     "10.123 seconds (rounds to 1 decimal)",
			duration: 10*time.Second + 123*time.Millisecond,
			expected: "10.1s",
		},
		{
			name:     "1 minute exact",
			duration: 1 * time.Minute,
			expected: "1m 0s",
		},
		{
			name:     "59 minutes 59 seconds",
			duration: 59*time.Minute + 59*time.Second,
			expected: "59m 59s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatter.FormatDuration(tt.duration)
			if result != tt.expected {
				t.Errorf("FormatDuration(%v) = %q, want %q", tt.duration, result, tt.expected)
			}
		})
	}
}
