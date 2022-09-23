package timeout

import (
	"math"
	"time"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
)

// Controller implements a truncated exponential backoff which can be described with next formula:
// duration = t_min * min(b ^ (r * θ(r-k)), t_max)
// for practical purpose we will transform this formula into:
// duration(r) = t_min * b ^ (min(r * θ(r-k)), c), where c = b(t_max / t_min).
// In described formula:
//
//	  k - is number of rounds we expect during hot path, after failing this many rounds,
//	      we will start increasing timeouts.
//	  b - timeout increase factor
//	  r - failed rounds counter
//	  θ - Heaviside step function
//		 t_min/t_max - minimum/maximum round duration
//
// By manipulating `r` after observing progress or lack thereof, we are achieving exponential increase/decrease
// of round durations.
//   - on timeout: increase number of failed rounds, this results in exponential growing round duration
//     on multiple subsequent timeouts, after exceeding k.
//   - on progress: decrease number of failed rounds, this results in exponential decrease of round duration.
type Controller struct {
	cfg            Config
	timer          *time.Timer
	timerInfo      *model.TimerInfo
	timeoutChannel <-chan time.Time
	maxExponent    float64 // max exponent for exponential function, derived from maximum round duration
	r              uint64  // failed rounds counter, higher value results in longer round duration
}

// NewController creates a new Controller.
func NewController(timeoutConfig Config) *Controller {
	// the initial value for the timeout channel is a closed channel which returns immediately
	// this prevents indefinite blocking when no timeout has been started
	startChannel := make(chan time.Time)
	close(startChannel)

	// we need to calculate log_b(t_max/t_min), golang doesn't support logarithm with custom base
	// we will apply change of base logarithm transformation to get around this:
	// log_b(x) = log_e(x) / log_e(b)
	maxExponent := math.Log(timeoutConfig.MaxReplicaTimeout/timeoutConfig.MinReplicaTimeout) /
		math.Log(timeoutConfig.TimeoutAdjustmentFactor)

	tc := Controller{
		cfg:            timeoutConfig,
		timeoutChannel: startChannel,
		maxExponent:    maxExponent,
	}
	return &tc
}

// TimerInfo returns TimerInfo for the current timer.
// New struct is created for each timer.
// Is nil if no timer has been started.
func (t *Controller) TimerInfo() *model.TimerInfo { return t.timerInfo }

// Channel returns a channel that will receive the specific timeout.
// New channel is created for each timer.
// in the event the timeout is reached (specified as TimerInfo).
// returns closed channel if no timer has been started.
func (t *Controller) Channel() <-chan time.Time { return t.timeoutChannel }

// StartTimeout starts the timeout of the specified type and returns the timer info
func (t *Controller) StartTimeout(view uint64) *model.TimerInfo {
	if t.timer != nil { // stop old timer
		t.timer.Stop()
	}
	duration := t.ReplicaTimeout()

	startTime := time.Now().UTC()
	timer := time.NewTimer(duration)
	timerInfo := model.TimerInfo{View: view, StartTime: startTime, Duration: duration}
	t.timer = timer
	t.timeoutChannel = t.timer.C
	t.timerInfo = &timerInfo

	return &timerInfo
}

// ReplicaTimeout returns the duration of the current view before we time out
func (t *Controller) ReplicaTimeout() time.Duration {
	// piecewise function
	step := uint64(0)
	if t.r > t.cfg.HappyPathRounds {
		step = 1
	}

	exponent := math.Min(float64(t.r*step), t.maxExponent)
	duration := t.cfg.MinReplicaTimeout * math.Pow(t.cfg.TimeoutAdjustmentFactor, exponent)
	return time.Duration(duration * float64(time.Millisecond))
}

// OnTimeout indicates to the Controller that a view change was triggered by a TC (unhappy path).
func (t *Controller) OnTimeout() {
	t.r++
}

// OnProgressBeforeTimeout indicates to the Controller that progress was made _before_ the timeout was reached
func (t *Controller) OnProgressBeforeTimeout() {
	if t.r > 0 {
		t.r--
	}
}

// BlockRateDelay is a delay to broadcast the proposal in order to control block production rate
func (t *Controller) BlockRateDelay() time.Duration {
	return time.Duration(t.cfg.BlockRateDelayMS * float64(time.Millisecond))
}
