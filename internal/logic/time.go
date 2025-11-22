package logic

import (
	"fmt"
	"time"
)

// ShouldBeAsleep checks if the current time falls within the sleep window.
// It returns true if we should be sleeping, false otherwise.
// It returns an error if the timezone is invalid or time formats are wrong.
func ShouldBeAsleep(now time.Time, wakeAt, sleepAt, timezone string) (bool, error) {
	// 1. Load the target timezone
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return false, fmt.Errorf("invalid timezone: %w", err)
	}

	// 2. Convert 'now' to the target timezone
	targetNow := now.In(location)

	// 3. Parse the schedule strings (only hour:minute matters)
	layout := "15:04"
	wakeParsed, err := time.Parse(layout, wakeAt)
	if err != nil {
		return false, fmt.Errorf("invalid wakeAt format: %w", err)
	}
	sleepParsed, err := time.Parse(layout, sleepAt)
	if err != nil {
		return false, fmt.Errorf("invalid sleepAt format: %w", err)
	}

	// 4. Construct 'today' versions of wake and sleep times
	// We use the Year, Month, Day from targetNow, but Hour, Min from the schedule.
	todayWake := time.Date(targetNow.Year(), targetNow.Month(), targetNow.Day(), wakeParsed.Hour(), wakeParsed.Minute(), 0, 0, location)
	todaySleep := time.Date(targetNow.Year(), targetNow.Month(), targetNow.Day(), sleepParsed.Hour(), sleepParsed.Minute(), 0, 0, location)

	// 5. Determine if we are in the "Awake" window
	isAwake := false

	if todayWake.Before(todaySleep) {
		// Standard Day Schedule (e.g., Wake 09:00, Sleep 17:00)
		// Awake if: Wake <= Now < Sleep
		if (targetNow.Equal(todayWake) || targetNow.After(todayWake)) && targetNow.Before(todaySleep) {
			isAwake = true
		}
	} else {
		// Overnight Schedule (e.g., Wake 22:00, Sleep 08:00)
		// Awake if: Now >= Wake OR Now < Sleep
		if (targetNow.Equal(todayWake) || targetNow.After(todayWake)) || targetNow.Before(todaySleep) {
			isAwake = true
		}
	}

	return !isAwake, nil
}
