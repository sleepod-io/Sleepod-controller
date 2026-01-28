package logic

import (
	"testing"
	"time"
)

// Helper to parse time strings for tests
func parseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}
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
			got, err := ShouldBeAsleep(tt.now, tt.wakeAt, tt.sleepAt, tt.timezone, "")
			if (err != nil) != tt.wantErr {
				t.Errorf("ShouldBeAsleep() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ShouldBeAsleep() = %v, want %v", got, tt.want)
			}
		})
	}

	// Additional Tests for Working Days
	t.Run("Working Days Logic", func(t *testing.T) {
		// 2025-11-22 is Saturday
		saturday := parseTime("2025-11-22T12:00:00Z")
		// 2025-11-24 is Monday
		monday := parseTime("2025-11-24T12:00:00Z")

		// Case 1: Weekend, M-F working days -> Should Sleep
		shouldSleep, err := ShouldBeAsleep(saturday, "08:00", "20:00", "UTC", "Monday-Friday")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !shouldSleep {
			t.Errorf("Expected to sleep on weekend, but got awake")
		}

		// Case 2: Monday, M-F working days -> Should Awake (12:00 is between 08-20)
		shouldSleep, err = ShouldBeAsleep(monday, "08:00", "20:00", "UTC", "Monday-Friday")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if shouldSleep {
			t.Errorf("Expected to be awake on Monday, but got sleep")
		}

		// Case 3: Weekend Overnight Shift (Friday Night to Saturday Morning)
		// Friday 22:00 -> Shift Started -> Working Day -> Awake
		fridayNight := parseTime("2025-11-21T23:00:00Z") // Friday
		shouldSleep, _ = ShouldBeAsleep(fridayNight, "20:00", "06:00", "UTC", "Monday-Friday")
		if shouldSleep {
			t.Errorf("Expected to be awake on Friday Night")
		}

		// Saturday 04:00 -> Shift continued -> Effective Day Friday -> Working Day -> Awake
		saturdayMorning := parseTime("2025-11-22T04:00:00Z") // Saturday
		shouldSleep, _ = ShouldBeAsleep(saturdayMorning, "20:00", "06:00", "UTC", "Monday-Friday")
		if shouldSleep {
			t.Errorf("Expected to be awake on Saturday Morning (Friday shift continuation)")
		}

		// Saturday 23:00 -> Shift Starts -> Saturday is NOT working day -> Sleep
		saturdayNight := parseTime("2025-11-22T23:00:00Z")
		shouldSleep, _ = ShouldBeAsleep(saturdayNight, "20:00", "06:00", "UTC", "Monday-Friday")
		if !shouldSleep {
			t.Errorf("Expected to be asleep on Saturday Night")
		}
	})
}

func TestGetNextEvent(t *testing.T) {
	tests := []struct {
		name      string
		now       time.Time
		wakeAt    string
		sleepAt   string
		timezone  string
		wantTime  time.Time
		wantEvent string
		wantErr   bool
	}{
		{
			name:      "Day Schedule - Currently Awake -> Next is Sleep (Today)",
			now:       parseTime("2025-11-22T10:00:00Z"), // 10:00
			wakeAt:    "08:00",
			sleepAt:   "20:00",
			timezone:  "UTC",
			wantTime:  parseTime("2025-11-22T20:00:00Z"), // Sleep at 20:00 today
			wantEvent: "Sleep",
		},
		{
			name:      "Day Schedule - Currently Asleep (Before Wake) -> Next is Wake (Today)",
			now:       parseTime("2025-11-22T05:00:00Z"), // 05:00
			wakeAt:    "08:00",
			sleepAt:   "20:00",
			timezone:  "UTC",
			wantTime:  parseTime("2025-11-22T08:00:00Z"), // Wake at 08:00 today
			wantEvent: "Wake",
		},
		{
			name:      "Day Schedule - Currently Asleep (After Sleep) -> Next is Wake (Tomorrow)",
			now:       parseTime("2025-11-22T22:00:00Z"), // 22:00
			wakeAt:    "08:00",
			sleepAt:   "20:00",
			timezone:  "UTC",
			wantTime:  parseTime("2025-11-23T08:00:00Z"), // Wake at 08:00 TOMORROW
			wantEvent: "Wake",
		},
		{
			name:      "Overnight Schedule - Currently Awake (Late) -> Next is Sleep (Tomorrow)",
			now:       parseTime("2025-11-22T23:00:00Z"), // 23:00
			wakeAt:    "20:00",
			sleepAt:   "08:00",
			timezone:  "UTC",
			wantTime:  parseTime("2025-11-23T08:00:00Z"), // Sleep at 08:00 tomorrow
			wantEvent: "Sleep",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTime, gotEvent, err := GetNextEvent(tt.now, tt.wakeAt, tt.sleepAt, tt.timezone, "")
			if (err != nil) != tt.wantErr {
				t.Errorf("GetNextEvent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !gotTime.Equal(tt.wantTime) {
				t.Errorf("GetNextEvent() gotTime = %v, want %v", gotTime, tt.wantTime)
			}
			if gotEvent != tt.wantEvent {
				t.Errorf("GetNextEvent() gotEvent = %v, want %v", gotEvent, tt.wantEvent)
			}
		})
	}
}
