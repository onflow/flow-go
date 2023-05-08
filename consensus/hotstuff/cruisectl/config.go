package cruisectl

import (
	"go.uber.org/atomic"
	"time"
)

type switchover struct {
	day  time.Weekday // day of every week to target epoch switchover
	hour uint8        // hour of the day to target epoch switchover
	zone time.Location
}

// ParseSwitcho
func ParseSwitchover(s string) (switchover, error) {

}

// Config defines configuration for the BlockRateController.
type Config struct {
	// TargetSwitchoverTime defines the target time to switchover epochs.
	// Options:
	TargetSwitchoverTime time.Time
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
	KP, KI, KD *atomic.Float64
}
