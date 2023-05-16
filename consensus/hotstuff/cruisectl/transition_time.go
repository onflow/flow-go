package cruisectl

import (
	"fmt"
	"strings"
	"time"
)

// weekdays is a lookup from canonical weekday strings to the time package constant.
var weekdays = map[string]time.Weekday{
	strings.ToLower(time.Sunday.String()):    time.Sunday,
	strings.ToLower(time.Monday.String()):    time.Monday,
	strings.ToLower(time.Tuesday.String()):   time.Tuesday,
	strings.ToLower(time.Wednesday.String()): time.Wednesday,
	strings.ToLower(time.Thursday.String()):  time.Thursday,
	strings.ToLower(time.Friday.String()):    time.Friday,
	strings.ToLower(time.Saturday.String()):  time.Saturday,
}

var transitionFmt = "%s@%02d:%02d" // example: wednesday@08:00

// EpochTransitionTime represents the target epoch transition time.
// Epochs last one week, so the transition is defined in terms of a day-of-week and time-of-day.
// The target time is always in UTC to avoid confusion resulting from different
// representations of the same transition time and around daylight savings time.
type EpochTransitionTime struct {
	day    time.Weekday // day of every week to target epoch transition
	hour   uint8        // hour of the day to target epoch transition
	minute uint8        // minute of the hour to target epoch transition
}

// DefaultEpochTransitionTime is the default epoch transition target.
// The target switchover is Wednesday 12:00 PDT, which is 19:00 UTC.
// The string representation is `wednesday@19:00`.
func DefaultEpochTransitionTime() *EpochTransitionTime {
	return &EpochTransitionTime{
		day:    time.Wednesday,
		hour:   19,
		minute: 0,
	}
}

// String returns the canonical string representation of the transition time.
// This is the format expected as user input, when this value is configured manually.
// See ParseSwitchover for details of the format.
func (s *EpochTransitionTime) String() string {
	return fmt.Sprintf(transitionFmt, strings.ToLower(s.day.String()), s.hour, s.minute)
}

// newInvalidTransitionStrError returns an informational error about an invalid transition string.
func newInvalidTransitionStrError(s string, msg string, args ...any) error {
	args = append([]any{s}, args...)
	return fmt.Errorf("invalid transition string (%s): "+msg, args...)
}

// ParseTransition parses a transition time string.
// A transition string must be specified according to the format:
//
//	WD@HH:MM
//
// WD is the weekday string as defined by `strings.ToLower(time.Weekday.String)`
// HH is the 2-character hour of day, in the range [00-23]
// MM is the 2-character minute of hour, in the range [00-59]
// All times are in UTC.
//
// A generic error is returned if the input is an invalid transition string.
func ParseTransition(s string) (*EpochTransitionTime, error) {
	strs := strings.Split(s, "@")
	if len(strs) != 2 {
		return nil, newInvalidTransitionStrError(s, "split on @ yielded %d substrings - expected %d", len(strs), 2)
	}
	dayStr := strs[0]
	timeStr := strs[1]
	if len(timeStr) != 5 || timeStr[2] != ':' {
		return nil, newInvalidTransitionStrError(s, "time part must have form HH:MM")
	}

	var hour uint8
	_, err := fmt.Sscanf(timeStr[0:2], "%02d", &hour)
	if err != nil {
		return nil, newInvalidTransitionStrError(s, "error scanning hour part: %w", err)
	}
	var minute uint8
	_, err = fmt.Sscanf(timeStr[3:5], "%02d", &minute)
	if err != nil {
		return nil, newInvalidTransitionStrError(s, "error scanning minute part: %w", err)
	}

	day, ok := weekdays[strings.ToLower(dayStr)]
	if !ok {
		return nil, newInvalidTransitionStrError(s, "invalid weekday part %s", dayStr)
	}
	if hour > 23 {
		return nil, newInvalidTransitionStrError(s, "invalid hour part: %d>23", hour)
	}
	if minute > 59 {
		return nil, newInvalidTransitionStrError(s, "invalid minute part: %d>59", hour)
	}

	return &EpochTransitionTime{
		day:    day,
		hour:   hour,
		minute: minute,
	}, nil
}
