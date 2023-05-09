package cruisectl

import (
	"fmt"
	"math"
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

var switchoverFmt = "%s@%02d:%02d" // example: wednesday@08:00

// Switchover represents the target epoch switchover time.
// Epochs last one week, so the switchover is defined in terms of a day-of-week and time-of-day.
// The target time is always in UTC to avoid confusion resulting from different
// representations of the same switchover time and around daylight savings time.
type Switchover struct {
	day    time.Weekday // day of every week to target epoch switchover
	hour   uint8        // hour of the day to target epoch switchover
	minute uint8        // minute of the hour to target epoch switchover
}

// String returns the canonical string representation of the switchover time.
// This is the format expected as user input, when this value is configured manually.
// See ParseSwitchover for details of the format.
func (s *Switchover) String() string {
	return fmt.Sprintf(switchoverFmt, strings.ToLower(s.day.String()), s.hour, s.minute)
}

// newInvalidSwitchoverStringError returns an informational error about an invalid switchover string.
func newInvalidSwitchoverStringError(s string, msg string, args ...any) error {
	args = append([]any{s}, args...)
	return fmt.Errorf("invalid switchover string (%s): "+msg, args...)
}

// ParseSwitchover parses a switchover time string.
// A switchover string must be specified according to the format:
//
//	WD@HH:MM
//
// WD is the weekday string as defined by `strings.ToLower(time.Weekday.String)`
// HH is the 2-character hour of day, in the range [00-23]
// MM is the 2-character minute of hour, in the range [00-59]
// All times are in UTC.
//
// A generic error is returned if the input is an invalid switchover string.
func ParseSwitchover(s string) (*Switchover, error) {
	strs := strings.Split(s, "@")
	if len(strs) != 2 {
		return nil, newInvalidSwitchoverStringError(s, "split on @ yielded %d substrings - expected %d", len(strs), 2)
	}
	dayStr := strs[0]
	timeStr := strs[1]
	if len(timeStr) != 5 || timeStr[2] != ':' {
		return nil, newInvalidSwitchoverStringError(s, "time part must have form HH:MM")
	}

	var hour uint8
	_, err := fmt.Sscanf(timeStr[0:2], "%02d", &hour)
	if err != nil {
		return nil, newInvalidSwitchoverStringError(s, "error scanning hour part: %w", err)
	}
	var minute uint8
	_, err = fmt.Sscanf(timeStr[3:5], "%02d", &minute)
	if err != nil {
		return nil, newInvalidSwitchoverStringError(s, "error scanning minute part: %w", err)
	}

	day, ok := weekdays[dayStr]
	if !ok {
		return nil, newInvalidSwitchoverStringError(s, "invalid weekday part %s", dayStr)
	}
	if hour > 23 {
		return nil, newInvalidSwitchoverStringError(s, "invalid hour part: %d>23", hour)
	}
	if minute > 59 {
		return nil, newInvalidSwitchoverStringError(s, "invalid minute part: %d>59", hour)
	}

	return &Switchover{
		day:    day,
		hour:   hour,
		minute: hour,
	}, nil
}

// DefaultConfig returns the default config for the BlockRateController.
func DefaultConfig() *Config {
	return &Config{
		TargetSwitchover: Switchover{
			day:    time.Wednesday,
			hour:   19,
			minute: 0,
		},
		// TODO confirm default values
		DefaultBlockRateDelay: 500 * time.Millisecond,
		MaxDelay:              1000 * time.Millisecond,
		MinDelay:              250 * time.Millisecond,
		Enabled:               true,
		N:                     600, // 10 minutes @ 1 view/second
		KP:                    math.NaN(),
		KI:                    math.NaN(),
		KD:                    math.NaN(),
	}
}

// Config defines configuration for the BlockRateController.
type Config struct {
	// TargetSwitchover defines the target time to switchover epochs.
	// Options:
	TargetSwitchover Switchover
	// DefaultBlockRateDelay is the baseline block rate delay. It is used:
	//  - when Enabled is false
	//  - when epoch fallback has been triggered
	//  - as the initial block rate delay value, to which the compensation computed
	//    by the PID controller is added
	DefaultBlockRateDelay time.Duration
	// MaxDelay is a hard maximum on the block rate delay.
	// If the BlockRateController computes a larger desired block rate delay
	// based on the observed error and tuning, this value will be used instead.
	MaxDelay time.Duration
	// MinDelay is a hard minimum on the block rate delay.
	// If the BlockRateController computes a smaller desired block rate delay
	// based on the observed error and tuning, this value will be used instead.
	MinDelay time.Duration
	// Enabled defines whether responsive control of the block rate is enabled.
	// When disabled, the DefaultBlockRateDelay is used.
	Enabled bool

	// N is the number of views over which the view rate average is measured.
	N uint
	// KP, KI, KD, are the coefficients to the PID controller and define its response.
	// KP adjusts the proportional term (responds to the magnitude of instantaneous error).
	// KI adjusts the integral term (responds to the magnitude and duration of error over time).
	// KD adjusts the derivative term (responds to the instantaneous rate of change of the error).
	KP, KI, KD float64
}
