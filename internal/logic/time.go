package logic

import (
	"fmt"
	"strings"
	"time"
)

// ShouldBeAsleep checks if the current time falls within the sleep window.
// It returns true if we should be sleeping, false otherwise.
// It returns an error if the timezone is invalid or time formats are wrong.
// ShouldBeAsleep checks if the current time falls within the sleep window.
// It returns true if we should be sleeping, false otherwise.
// It returns an error if the timezone is invalid or time formats are wrong.
func ShouldBeAsleep(now time.Time, wakeAt, sleepAt, timezone, workingDays string) (bool, error) {
	state, _, _, err := GetTimeState(now, wakeAt, sleepAt, timezone, workingDays)
	return state, err
}

// GetNextEvent returns the time and type of the next scheduled event.
func GetNextEvent(now time.Time, wakeAt, sleepAt, timezone, workingDays string) (time.Time, string, error) {
	_, nextTime, nextEvent, err := GetTimeState(now, wakeAt, sleepAt, timezone, workingDays)
	return nextTime, nextEvent, err
}

// GetTimeState calculates both the current state (isSleep) and the next event.
// This ensures consistency as both are derived from the exact same parsed time and schedule.
func GetTimeState(now time.Time, wakeAt, sleepAt, timezone, workingDays string) (bool, time.Time, string, error) {
	targetNow, todayWake, todaySleep, err := parseSchedule(now, wakeAt, sleepAt, timezone)
	if err != nil {
		return false, time.Time{}, "", err
	}

	// 1. Determine Current State
	isAwake := false

	// Check Working Days logic
	isWorkingDay := isDayWorking(targetNow, workingDays)
	if todayWake.After(todaySleep) {
		// Overnight shift (e.g. Wake 22:00, Sleep 06:00)
		// If we are in the morning hours (upto Sleep), the shift started yesterday.
		if targetNow.Before(todaySleep) {
			isWorkingDay = isDayWorking(targetNow.AddDate(0, 0, -1), workingDays)
		}
	}

	if isWorkingDay {
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

func isDayWorking(t time.Time, workingDays string) bool {
	if workingDays == "" {
		return true
	}

	currentDay := t.Weekday()
	// normalise
	workingDays = strings.ToLower(workingDays)

	parts := strings.Split(workingDays, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "-") {
			// Range
			rangeParts := strings.Split(part, "-")
			if len(rangeParts) == 2 {
				start := parseDay(rangeParts[0])
				end := parseDay(rangeParts[1])
				if start != -1 && end != -1 {
					// Check if currentDay is in range [start, end]
					// Handle wrap around: Fri-Mon means Fri, Sat, Sun, Mon.
					if start <= end {
						if currentDay >= start && currentDay <= end {
							return true
						}
					} else {
						// Wrap around
						if currentDay >= start || currentDay <= end {
							return true
						}
					}
				}
			}
		} else {
			// Single day
			d := parseDay(part)
			if d != -1 && currentDay == d {
				return true
			}
		}
	}
	return false
}

func parseDay(day string) time.Weekday {
	day = strings.ToLower(strings.TrimSpace(day))
	switch day {
	case "sunday", "sun":
		return time.Sunday
	case "monday", "mon":
		return time.Monday
	case "tuesday", "tue":
		return time.Tuesday
	case "wednesday", "wed":
		return time.Wednesday
	case "thursday", "thu":
		return time.Thursday
	case "friday", "fri":
		return time.Friday
	case "saturday", "sat":
		return time.Saturday
	}
	return -1
}

func ParseDateFromStr(date string) (time.Time, error) {
	layout := "02/01/2006"
	dateParsed, err := time.Parse(layout, date)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date format: %w", err)
	}
	return dateParsed, nil
}
