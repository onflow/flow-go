package cruisectl

import (
	"time"

	"go.uber.org/atomic"
)

// DefaultConfig returns the default config for the BlockTimeController.
func DefaultConfig() *Config {
	return &Config{
		TimingConfig{
			FallbackProposalDelay: atomic.NewDuration(250 * time.Millisecond),
			MinViewDuration:       atomic.NewDuration(600 * time.Millisecond),
			MaxViewDuration:       atomic.NewDuration(1600 * time.Millisecond),
			Enabled:               atomic.NewBool(false),
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
	// FallbackProposalDelay is the minimal block construction delay. When used, it behaves like the
	// old command line flag `block-rate-delay`. Specifically, the primary measures the duration from
	// starting to construct its proposal to the proposal being ready to be published. If this
	// duration is _less_ than FallbackProposalDelay, the primary delays broadcasting its proposal
	// by the remainder needed to reach `FallbackProposalDelay`
	// It is used:
	//  - when Enabled is false
	//  - when epoch fallback has been triggered
	FallbackProposalDelay *atomic.Duration

	// MaxViewDuration is a hard maximum on the total view time targeted by ProposalTiming.
	// If the BlockTimeController computes a larger desired ProposalTiming value
	// based on the observed error and tuning, this value will be used instead.
	MaxViewDuration *atomic.Duration

	// MinViewDuration  is a hard maximum on the total view time targeted by ProposalTiming.
	// If the BlockTimeController computes a smaller desired ProposalTiming value
	// based on the observed error and tuning, this value will be used instead.
	MinViewDuration *atomic.Duration

	// Enabled defines whether responsive control of the GetProposalTiming is enabled.
	// When disabled, the FallbackProposalDelay is used.
	Enabled *atomic.Bool
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

// GetFallbackProposalDuration returns the proposal duration used when Cruise Control is not active.
func (ctl TimingConfig) GetFallbackProposalDuration() time.Duration {
	return ctl.FallbackProposalDelay.Load()
}

// GetMaxViewDuration returns the max view duration returned by the controller.
func (ctl TimingConfig) GetMaxViewDuration() time.Duration {
	return ctl.MaxViewDuration.Load()
}

// GetMinViewDuration returns the min view duration returned by the controller.
func (ctl TimingConfig) GetMinViewDuration() time.Duration {
	return ctl.MinViewDuration.Load()
}

// GetEnabled returns whether the controller is enabled.
func (ctl TimingConfig) GetEnabled() bool {
	return ctl.Enabled.Load()
}

// SetFallbackProposalDuration sets the proposal duration used when Cruise Control is not active.
func (ctl TimingConfig) SetFallbackProposalDuration(dur time.Duration) error {
	ctl.FallbackProposalDelay.Store(dur)
	return nil
}

// SetMaxViewDuration sets the max view duration returned by the controller.
func (ctl TimingConfig) SetMaxViewDuration(dur time.Duration) error {
	ctl.MaxViewDuration.Store(dur)
	return nil
}

// SetMinViewDuration sets the min view duration returned by the controller.
func (ctl TimingConfig) SetMinViewDuration(dur time.Duration) error {
	ctl.MinViewDuration.Store(dur)
	return nil
}

// SetEnabled sets whether the controller is enabled.
func (ctl TimingConfig) SetEnabled(enabled bool) error {
	ctl.Enabled.Store(enabled)
	return nil
}
