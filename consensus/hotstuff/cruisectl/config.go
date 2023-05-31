package cruisectl

import (
	"time"
)

// DefaultConfig returns the default config for the BlockTimeController.
func DefaultConfig() *Config {
	return &Config{
		TimingConfig{
			TargetTransition:         DefaultEpochTransitionTime(),
			FallbackProposalDuration: 500 * time.Millisecond,
			MinProposalDuration:      250 * time.Millisecond,
			MaxProposalDuration:      1800 * time.Millisecond,
			Enabled:                  true,
		},
		ControllerParams{
			N_ewma: 5,
			N_itg:  50,
			KP:     2.0,
			KI:     0.6,
			KD:     3.0,
		},
	}
}

// Config defines configuration for the BlockTimeController.
type Config struct {
	TimingConfig
	ControllerParams
}

// TimingConfig specifies the BlockTimeController's limits of authority.
type TimingConfig struct {
	// TargetTransition defines the target time to transition epochs each week.
	TargetTransition EpochTransitionTime
	// FallbackProposalDuration is the baseline GetProposalTiming value. It is used:
	//  - when Enabled is false
	//  - when epoch fallback has been triggered
	//  - as the initial GetProposalTiming value, to which the compensation computed by the PID controller is added
	FallbackProposalDuration time.Duration
	// MaxProposalDuration is a hard maximum on the GetProposalTiming.
	// If the BlockTimeController computes a larger desired GetProposalTiming value
	// based on the observed error and tuning, this value will be used instead.
	MaxProposalDuration time.Duration
	// MinProposalDuration is a hard minimum on the GetProposalTiming.
	// If the BlockTimeController computes a smaller desired GetProposalTiming value
	// based on the observed error and tuning, this value will be used instead.
	MinProposalDuration time.Duration
	// Enabled defines whether responsive control of the GetProposalTiming is enabled.
	// When disabled, the FallbackProposalDuration is used.
	Enabled bool
}

// ControllerParams specifies the BlockTimeController's internal parameters.
type ControllerParams struct {
	// N_ewma defines how historical measurements are incorporated into the EWMA for the proportional error term.
	// Intuition: Suppose the input changes from x to y instantaneously:
	//  - N_ewma is the number of samples required to move the EWMA output about 2/3 of the way from x to y
	// Per convention, this must be a _positive_ integer.
	N_ewma uint

	// N_itg defines how historical measurements are incorporated into the integral error term.
	// Intuition: For a constant error x:
	//  - the integrator value will saturate at `x•N_itg`
	//  - an integrator initialized at 0 reaches 2/3 of the saturation value after N_itg samples
	// Per convention, this must be a _positive_ integer.
	N_itg uint

	// KP, KI, KD, are the coefficients to the PID controller and define its response.
	// KP adjusts the proportional term (responds to the magnitude of error).
	// KI adjusts the integral term (responds to the error sum over a recent time interval).
	// KD adjusts the derivative term (responds to the rate of change, i.e. time derivative, of the error).
	KP, KI, KD float64
}

// alpha returns α, the inclusion parameter for the error EWMA. See N_ewma for details.
func (c *ControllerParams) alpha() float64 {
	return 1.0 / float64(c.N_ewma)
}

// beta returns ß, the memory parameter of the leaky error integrator. See N_itg for details.
func (c *ControllerParams) beta() float64 {
	return 1.0 / float64(c.N_itg)
}
