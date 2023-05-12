package cruisectl

import (
	"math"
	"time"
)

// DefaultConfig returns the default config for the BlockRateController.
func DefaultConfig() *Config {
	return &Config{
		TargetTransition: DefaultEpochTransitionTime(),
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
	// TargetTransition defines the target time to transition epochs each week.
	TargetTransition *EpochTransitionTime
	// DefaultBlockRateDelay is the baseline block rate delay. It is used:
	//  - when Enabled is false
	//  - when epochInfo fallback has been triggered
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

// alpha returns the sample inclusion proportion used when calculating the exponentially moving average.
func (c Config) alpha() float64 {
	return 2.0 / float64(c.N+1)
}
