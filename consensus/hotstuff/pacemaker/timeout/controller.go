package timeout

import (
	"math"
	"time"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
)

// Controller implements a timout with:
//   - on timeout: increase timeout by multiplicative factor `timeoutIncrease` (user-specified)
//     this results in exponential growing timeout duration on multiple subsequent timeouts
//   - on progress: decrease timeout by subtrahend `timeoutDecrease`
type Controller struct {
	cfg            Config
	timer          *time.Timer
	timerInfo      *model.TimerInfo
	timeoutChannel <-chan time.Time
}

// NewController creates a new Controller.
func NewController(timeoutConfig Config) *Controller {
	// the initial value for the timeout channel is a closed channel which returns immediately
	// this prevents indefinite blocking when no timeout has been started
	startChannel := make(chan time.Time)
	close(startChannel)

	tc := Controller{
		cfg:            timeoutConfig,
		timeoutChannel: startChannel,
	}
	return &tc
}

func DefaultController() *Controller {
	return NewController(DefaultConfig)
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

// StartTimeout starts the timeout of the specified type and returns the
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
	return time.Duration(t.cfg.ReplicaTimeout * 1e6)
}

// OnTimeout indicates to the Controller that a view change was triggered by a TC (unhappy path).
func (t *Controller) OnTimeout() {
	t.cfg.ReplicaTimeout = math.Min(t.cfg.ReplicaTimeout*t.cfg.TimeoutIncrease, t.cfg.MaxReplicaTimeout)
}

// OnProgressBeforeTimeout indicates to the Controller that progress was made _before_ the timeout was reached
func (t *Controller) OnProgressBeforeTimeout() {
	t.cfg.ReplicaTimeout = math.Max(t.cfg.ReplicaTimeout*t.cfg.TimeoutDecrease, t.cfg.MinReplicaTimeout)
}

// BlockRateDelay is a delay to broadcast the proposal in order to control block production rate
func (t *Controller) BlockRateDelay() time.Duration {
	return time.Duration(t.cfg.BlockRateDelayMS * float64(time.Millisecond))
}
