package logic

import (
	"testing"
	"time"
)

func TestShouldBeAsleep(t *testing.T) {
	// Define the test cases
	tests := []struct {
		name     string
		now      time.Time
		wakeAt   string
		sleepAt  string
		timezone string
		want     bool
		wantErr  bool
	}{
		{
			name:     "Daytime (Should be Awake)",
			now:      parseTime("2025-11-22T12:00:00Z"), // 12:00 UTC
			wakeAt:   "08:00",
			sleepAt:  "20:00",
			timezone: "UTC",
			want:     false,
			wantErr:  false,
		},
		{
			name:     "Nighttime (Should be Asleep)",
			now:      parseTime("2025-11-22T22:00:00Z"), // 22:00 UTC
			wakeAt:   "08:00",
			sleepAt:  "20:00",
			timezone: "UTC",
			want:     true,
			wantErr:  false,
		},
		{
			name:     "Morning before Wake (Should be Asleep)",
			now:      parseTime("2025-11-22T05:00:00Z"), // 05:00 UTC
			wakeAt:   "08:00",
			sleepAt:  "20:00",
			timezone: "UTC",
			want:     true,
			wantErr:  false,
		},
		{
			name:     "Overnight Schedule - Late Night (Should be Asleep)",
			now:      parseTime("2025-11-22T02:00:00Z"),
			wakeAt:   "08:00",
			sleepAt:  "22:00", // Sleeps at 10 PM, Wakes at 8 AM next day
			timezone: "UTC",
			want:     true,
			wantErr:  false,
		},
		{
			name:     "Overnight Schedule - Evening (Should be Asleep)",
			now:      parseTime("2025-11-22T23:00:00Z"),
			wakeAt:   "08:00",
			sleepAt:  "22:00",
			timezone: "UTC",
			want:     true,
			wantErr:  false,
		},
		{
			name:     "Invalid Timezone",
			now:      time.Now(),
			wakeAt:   "08:00",
			sleepAt:  "20:00",
			timezone: "Mars/Crater",
			want:     false,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ShouldBeAsleep(tt.now, tt.wakeAt, tt.sleepAt, tt.timezone)
			if (err != nil) != tt.wantErr {
				t.Errorf("ShouldBeAsleep() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ShouldBeAsleep() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Helper to parse time strings for tests
func parseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}
