package logic

import (
	"fmt"
	"time"
)

// ShouldBeAsleep checks if the current time falls within the sleep window.
// It returns true if we should be sleeping, false otherwise.
// It returns an error if the timezone is invalid or time formats are wrong.
func ShouldBeAsleep(now time.Time, wakeAt, sleepAt, timezone string) (bool, error) {
	state, _, _, err := GetTimeState(now, wakeAt, sleepAt, timezone)
	return state, err
}

// GetNextEvent returns the time and type of the next scheduled event.
func GetNextEvent(now time.Time, wakeAt, sleepAt, timezone string) (time.Time, string, error) {
	_, nextTime, nextEvent, err := GetTimeState(now, wakeAt, sleepAt, timezone)
	return nextTime, nextEvent, err
}

// GetTimeState calculates both the current state (isSleep) and the next event.
// This ensures consistency as both are derived from the exact same parsed time and schedule.
func GetTimeState(now time.Time, wakeAt, sleepAt, timezone string) (bool, time.Time, string, error) {
	targetNow, todayWake, todaySleep, err := parseSchedule(now, wakeAt, sleepAt, timezone)
	if err != nil {
		return false, time.Time{}, "", err
	}

	// 1. Determine Current State
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
	shouldSleep := !isAwake

	// 2. Determine Next Event
	var nextWake, nextSleep time.Time
	if todayWake.Before(targetNow) {
		nextWake = todayWake.AddDate(0, 0, 1)
	} else {
		nextWake = todayWake
	}
	if todaySleep.Before(targetNow) {
		nextSleep = todaySleep.AddDate(0, 0, 1)
	} else {
		nextSleep = todaySleep
	}

	if nextWake.Before(nextSleep) {
		return shouldSleep, nextWake, "Wake", nil
	}
	return shouldSleep, nextSleep, "Sleep", nil
}

func parseSchedule(now time.Time, wakeAt, sleepAt, timezone string) (time.Time, time.Time, time.Time, error) {
	targetNow, err := parseTimeToLocation(now, timezone)
	if err != nil {
		return time.Time{}, time.Time{}, time.Time{}, err
	}

	layout := "15:04"
	wakeParsed, err := time.Parse(layout, wakeAt)
	if err != nil {
		return time.Time{}, time.Time{}, time.Time{}, fmt.Errorf("invalid wakeAt format: %w", err)
	}
	sleepParsed, err := time.Parse(layout, sleepAt)
	if err != nil {
		return time.Time{}, time.Time{}, time.Time{}, fmt.Errorf("invalid sleepAt format: %w", err)
	}

	todayWake := time.Date(targetNow.Year(), targetNow.Month(), targetNow.Day(), wakeParsed.Hour(), wakeParsed.Minute(), 0, 0, targetNow.Location())
	todaySleep := time.Date(targetNow.Year(), targetNow.Month(), targetNow.Day(), sleepParsed.Hour(), sleepParsed.Minute(), 0, 0, targetNow.Location())

	return targetNow, todayWake, todaySleep, nil
}

func parseTimeToLocation(now time.Time, timezone string) (time.Time, error) {
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return now, fmt.Errorf("invalid timezone: %w", err)
	}
	return now.In(location), nil
}
