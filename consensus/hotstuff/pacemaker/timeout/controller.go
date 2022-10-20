package timeout

import (
	"context"
	"math"
	"time"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
)

// Controller implements the following truncated exponential backoff:
//
//	duration = t_min * min(b ^ ((r-k) * θ(r-k)), t_max)
//
// For practical purpose we will transform this formula into:
//
//	duration(r) = t_min * b ^ (min((r-k) * θ(r-k)), c), where c = log_b (t_max / t_min).
//
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
	timeoutChannel chan model.TimerInfo
	stopTicker     context.CancelFunc
	maxExponent    float64 // max exponent for exponential function, derived from maximum round duration
	r              uint64  // failed rounds counter, higher value results in longer round duration
}

// NewController creates a new Controller.
func NewController(timeoutConfig Config) *Controller {
	// the initial value for the timeout channel is a closed channel which returns immediately
	// this prevents indefinite blocking when no timeout has been started
	startChannel := make(chan model.TimerInfo)
	close(startChannel)

	// we need to calculate log_b(t_max/t_min), golang doesn't support logarithm with custom base
	// we will apply change of base logarithm transformation to get around this:
	// log_b(x) = log_e(x) / log_e(b)
	maxExponent := math.Log(timeoutConfig.MaxReplicaTimeout/timeoutConfig.MinReplicaTimeout) /
		math.Log(timeoutConfig.TimeoutAdjustmentFactor)

	tc := Controller{
		cfg:            timeoutConfig,
		timeoutChannel: startChannel,
		stopTicker:     func() {},
		maxExponent:    maxExponent,
	}
	return &tc
}

// Channel returns a channel that will receive the specific timeout.
// A new channel is created on each call of `StartTimeout`.
// Returns closed channel if no timer has been started.
func (t *Controller) Channel() <-chan model.TimerInfo {
	return t.timeoutChannel
}

// StartTimeout starts the timeout of the specified type and returns the timer info
func (t *Controller) StartTimeout(ctx context.Context, view uint64) model.TimerInfo {
	t.stopTicker() // stop old timeout

	// setup new timer:
	// when round duration is small react with faster timeout rebroadcasts
	duration := t.ReplicaTimeout()
	tickInterval := time.Duration(math.Min(float64(duration.Milliseconds()), t.cfg.MaxTimeoutObjectRebroadcastInterval))
	timerInfo := model.TimerInfo{View: view, StartTime: time.Now().UTC(), Duration: duration}
	t.timeoutChannel = make(chan model.TimerInfo, 1)

	// start timeout logic for (re-)broadcasting timeout objects on regular basis as long as we are in the same round.
	var childContext context.Context
	childContext, t.stopTicker = context.WithCancel(ctx)
	go tickAfterTimeout(childContext, timerInfo, tickInterval, t.timeoutChannel)

	return timerInfo
}

// tickAfterTimeout is a utility function which:
//  1. waits for the initial timeout and then sends the current time to `timeoutChannel`
//  2. and subsequently sends the current time every `tickInterval` to `timeoutChannel`
//
// If the receiver from the `timeoutChannel` falls behind and does not pick up the events,
// we drop ticks until the receiver catches up. When cancelling `ctx`, all timing logic stops.
// This approach allows to have a concurrent-safe implementation, where there is no unsafe state sharing between caller and
// ticking logic.
func tickAfterTimeout(ctx context.Context, timerInfo model.TimerInfo, tickInterval time.Duration, timeoutChannel chan<- model.TimerInfo) {
	// wait for initial timeout
	timer := time.NewTimer(timerInfo.Duration)
	select {
	case <-timer.C:
		timeoutChannel <- timerInfo // forward initial timeout to the sink
	case <-ctx.Done():
		timer.Stop() // allows timer to be garbage collected (before it expires)
		return
	}

	// after we have reached the initial timeout, sent to `tickSink` every `tickInterval` until cancelled
	ticker := time.NewTicker(tickInterval)
	for {
		select {
		case <-ticker.C:
			timerInfo.Tick++
			timeoutChannel <- timerInfo // forward ticks to the sink
		case <-ctx.Done():
			ticker.Stop() // critical for ticker to be garbage collected
			return
		}
	}
}

// ReplicaTimeout returns the duration of the current view before we time out
func (t *Controller) ReplicaTimeout() time.Duration {
	if t.r <= t.cfg.HappyPathMaxRoundFailures {
		return time.Duration(t.cfg.MinReplicaTimeout * float64(time.Millisecond))
	}
	r := float64(t.r - t.cfg.HappyPathMaxRoundFailures)
	if r >= t.maxExponent {
		return time.Duration(t.cfg.MaxReplicaTimeout * float64(time.Millisecond))
	}
	// compute timeout duration [in milliseconds]:
	duration := t.cfg.MinReplicaTimeout * math.Pow(t.cfg.TimeoutAdjustmentFactor, r)
	return time.Duration(duration * float64(time.Millisecond))
}

// OnTimeout indicates to the Controller that a view change was triggered by a TC (unhappy path).
func (t *Controller) OnTimeout() {
	if float64(t.r) >= t.maxExponent+float64(t.cfg.HappyPathMaxRoundFailures) {
		return
	}
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
