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
	timer          *time.Timer
	timerInfo      *model.TimerInfo
	timeoutChannel chan time.Time
	stopTicker     context.CancelFunc
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
func (t *Controller) Channel() <-chan time.Time {
	return t.timeoutChannel
}

// StartTimeout starts the timeout of the specified type and returns the timer info
func (t *Controller) StartTimeout(view uint64) *model.TimerInfo {
	// stop old timer and schedule stop of ticker
	if t.timer != nil {
		t.timer.Stop()
		t.stopTicker()
	}
	duration := t.ReplicaTimeout()

	// start round duration timer
	startTime := time.Now().UTC()
	timer := time.NewTimer(duration)
	timerInfo := model.TimerInfo{View: view, StartTime: startTime, Duration: duration}
	t.timer = timer
	t.timeoutChannel = make(chan time.Time, 1)
	t.timerInfo = &timerInfo

	var ctx context.Context
	ctx, t.stopTicker = context.WithCancel(context.Background())

	// start a ticker to rebroadcast timeout objects on regular basis as long as we are in the same round.
	go tickAfterTimeout(ctx, t.timeoutChannel, t.timer.C)

	return &timerInfo
}

// tickAfterTimeout is a utility function which starts ticking after observing timeout from single-shot timer
// The idea is that after observing single-shot event we create a ticker which will send tick events to `tickSink` channel
// note that first timeout event is sent as well. We use context to track when to stop.
// This approach allows to have a concurrent-safe implementation where there is no unsafe state sharing between caller and
// ticking logic.
func tickAfterTimeout(ctx context.Context, tickSink chan time.Time, timeout <-chan time.Time) {
	var tickerChannel <-chan time.Time
	defer close(tickSink)
	for {
		select {
		case val, ok := <-timeout:
			// since this channel is single-shot we know this section will be called once
			if !ok {
				return
			}
			// create a ticker and schedule it to stop when we are done
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			tickerChannel = ticker.C
			// don't forget to send value to the sink
			tickSink <- val
		case val := <-tickerChannel:
			// forward ticks to the sink
			tickSink <- val
		case <-ctx.Done():
			// exit when asked
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
