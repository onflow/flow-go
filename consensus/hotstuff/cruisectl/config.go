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
		DefaultProposalDelay: 500 * time.Millisecond,
		MaxProposalDelay:     1000 * time.Millisecond,
		MinProposalDelay:     250 * time.Millisecond,
		Enabled:              true,
		N:                    600, // 10 minutes @ 1 view/second
		KP:                   math.NaN(),
		KI:                   math.NaN(),
		KD:                   math.NaN(),
	}
}

// Config defines configuration for the BlockRateController.
type Config struct {
	// TargetTransition defines the target time to transition epochs each week.
	TargetTransition *EpochTransitionTime
	// DefaultProposalDelay is the baseline ProposalDelay value. It is used:
	//  - when Enabled is false
	//  - when epoch fallback has been triggered
	//  - as the initial ProposalDelay value, to which the compensation computed by the PID controller is added
	DefaultProposalDelay time.Duration
	// MaxProposalDelay is a hard maximum on the ProposalDelay.
	// If the BlockRateController computes a larger desired ProposalDelay value
	// based on the observed error and tuning, this value will be used instead.
	MaxProposalDelay time.Duration
	// MinProposalDelay is a hard minimum on the ProposalDelay.
	// If the BlockRateController computes a smaller desired ProposalDelay value
	// based on the observed error and tuning, this value will be used instead.
	MinProposalDelay time.Duration
	// Enabled defines whether responsive control of the block rate is enabled.
	// When disabled, the DefaultProposalDelay is used.
	Enabled bool

	// N is the number of views over which the view rate average is measured.
	// Per convention, this must be a _positive_ integer.
	N uint
	// KP, KI, KD, are the coefficients to the PID controller and define its response.
	// KP adjusts the proportional term (responds to the magnitude of error).
	// KI adjusts the integral term (responds to the error sum over a recent time interval).
	// KD adjusts the derivative term (responds to the rate of change, i.e. time derivative, of the error).
	KP, KI, KD float64
}

// alpha returns the sample inclusion proportion used when calculating the exponentially moving average.
func (c Config) alpha() float64 {
	return 2.0 / float64(c.N+1)
}
