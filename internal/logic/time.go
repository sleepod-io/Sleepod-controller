package logic

import (
	"fmt"
	"time"
)

// ShouldBeAsleep checks if the current time falls within the sleep window.
// It returns true if we should be sleeping, false otherwise.
// It returns an error if the timezone is invalid or time formats are wrong.
func ShouldBeAsleep(now time.Time, wakeAt, sleepAt, timezone string) (bool, error) {
	targetNow, err := parseTimeToLocation(now, timezone)
	if err != nil {
		return false, err
	}

	layout := "15:04"
	wakeParsed, err := time.Parse(layout, wakeAt)
	if err != nil {
		return false, fmt.Errorf("invalid wakeAt format: %w", err)
	}
	sleepParsed, err := time.Parse(layout, sleepAt)
	if err != nil {
		return false, fmt.Errorf("invalid sleepAt format: %w", err)
	}

	todayWake := time.Date(targetNow.Year(), targetNow.Month(), targetNow.Day(), wakeParsed.Hour(), wakeParsed.Minute(), 0, 0, targetNow.Location())
	todaySleep := time.Date(targetNow.Year(), targetNow.Month(), targetNow.Day(), sleepParsed.Hour(), sleepParsed.Minute(), 0, 0, targetNow.Location())
	isAwake := false

	if todayWake.Before(todaySleep) {
		// Awake if: Wake <= Now < Sleep
		if (targetNow.Equal(todayWake) || targetNow.After(todayWake)) && targetNow.Before(todaySleep) {
			isAwake = true
		}
	} else {
		// Awake if: Now >= Wake OR Now < Sleep
		if (targetNow.Equal(todayWake) || targetNow.After(todayWake)) || targetNow.Before(todaySleep) {
			isAwake = true
		}
	}

	return !isAwake, nil
}

func GetNextEvent(now time.Time, wakeAt, sleepAt, timezone string) (time.Time, string, error) {
	targetNow, err := parseTimeToLocation(now, timezone)
	if err != nil {
		return time.Time{}, "", err
	}

	layout := "15:04"
	wakeParsed, err := time.Parse(layout, wakeAt)
	if err != nil {
		return time.Time{}, "", fmt.Errorf("invalid wakeAt format: %w", err)
	}
	sleepParsed, err := time.Parse(layout, sleepAt)
	if err != nil {
		return time.Time{}, "", fmt.Errorf("invalid sleepAt format: %w", err)
	}

	todayWake := time.Date(targetNow.Year(), targetNow.Month(), targetNow.Day(), wakeParsed.Hour(), wakeParsed.Minute(), 0, 0, targetNow.Location())
	todaySleep := time.Date(targetNow.Year(), targetNow.Month(), targetNow.Day(), sleepParsed.Hour(), sleepParsed.Minute(), 0, 0, targetNow.Location())

	var nextWake, nextSleep time.Time
	if todayWake.Before(targetNow) {
		// todayWake is already in the past, so it's tomorrow
		nextWake = todayWake.AddDate(0, 0, 1)
	} else {
		// todayWake is in the future, so it's today
		nextWake = todayWake
	}
	if todaySleep.Before(targetNow) {
		// todaySleep is already in the past, so it's tomorrow
		nextSleep = todaySleep.AddDate(0, 0, 1)
	} else {
		// todaySleep is in the future, so it's today
		nextSleep = todaySleep
	}

	// pick the minimum of nextWake and nextSleep
	if nextWake.Before(nextSleep) {
		return nextWake, "Wake", nil
	}
	return nextSleep, "Sleep", nil
}

func parseTimeToLocation(now time.Time, timezone string) (time.Time, error) {
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return now, fmt.Errorf("invalid timezone: %w", err)
	}
	return now.In(location), nil
}
