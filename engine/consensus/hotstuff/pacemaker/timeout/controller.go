package timeout

import (
	"math"
	"time"

	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
)

// Controller implements a timout with:
// - on timeout: increase timeout by multiplicative factor `timeoutIncrease` (user-specified)
//   this results in exponential growing timeout duration on multiple subsequent timeouts
// - on progress: decrease timeout by subtrahend `timeoutDecrease`
type Controller struct {
	Config
	timerInfo      *types.TimerInfo
	timeoutChannel <-chan *types.Timeout
}

// timeoutCap this is an internal cap on the timeout to avoid numerical overflows.
// Its value is large enough to be of no practical implication.
// We use 1E9 milliseconds which is about 11 days for a single timout (i.e. more than a full epoch)
const timeoutCap float64 = 1E9

// NewController creates a new Controller.
func NewController(timeoutConfig Config) *Controller {
	// the initial value for the timeout channel is a closed channel which returns immediately
	// this prevents indefinite blocking when no timeout has been started
	startChannel := make(chan *types.Timeout)
	close(startChannel)

	tc := Controller{
		Config:         timeoutConfig,
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
func (t *Controller) TimerInfo() *types.TimerInfo { return t.timerInfo }

// Channel returns a channel that will receive the specific timeout.
// New channel is created for each timer.
// in the event the timeout is reached (specified as TimerInfo).
// returns closed channel if no timer has been started.
func (t *Controller) Channel() <-chan *types.Timeout { return t.timeoutChannel }

// StartTimeout starts the timeout of the specified type and returns the
func (t *Controller) StartTimeout(mode types.TimeoutMode, view uint64) *types.TimerInfo {
	duration := t.computeTimeoutDuration(mode)

	startTime := time.Now()
	timer := time.NewTimer(duration)
	timerInfo := types.TimerInfo{Mode: mode, View: view, StartTime: startTime, Duration: duration}

	timeoutChannel := make(chan *types.Timeout)
	go func() {
		time := <-timer.C
		timeoutChannel <- &types.Timeout{TimerInfo: timerInfo, CreatedAt: time}
	}()
	t.timeoutChannel = timeoutChannel
	t.timerInfo = &timerInfo

	return &timerInfo
}

func (t *Controller) computeTimeoutDuration(mode types.TimeoutMode) time.Duration {
	var duration time.Duration
	switch mode {
	case types.VoteCollectionTimeout:
		duration = t.VoteCollectionTimeoutTimeout()
	case types.ReplicaTimeout:
		duration = t.ReplicaTimeout()
	default:
		// This should never happen; Only protects code from future inconsistent modifications.
		// There are only the two timeout modes explicitly handled above. Unless the enum
		// containing the timeout mode is extended, the default case will never be reached.
		panic("unknown timeout mode")
	}
	return duration
}

// ReplicaTimeout returns the duration of the current view before we time out
func (t *Controller) ReplicaTimeout() time.Duration {
	return time.Duration(t.replicaTimeout * 1E6)
}

// VoteCollectionTimeout returns the duration of Vote aggregation _after_ receiving a block
// during which the primary tries to aggregate votes for the view where it is leader
func (t *Controller) VoteCollectionTimeoutTimeout() time.Duration {
	// time.Duration expects an int64 as input which specifies the duration in units of nanoseconds (1E-9)
	return time.Duration(t.replicaTimeout * 1E6 * t.voteAggregationTimeoutFraction)
}

// OnTimeout indicates to the Controller that the timeout was reached
func (t *Controller) OnTimeout() {
	t.replicaTimeout = math.Min(t.replicaTimeout*t.timeoutIncrease, timeoutCap)
}

// OnProgressBeforeTimeout indicates to the Controller that progress was made _before_ the timeout was reached
func (t *Controller) OnProgressBeforeTimeout() {
	t.replicaTimeout = math.Max(t.replicaTimeout-t.timeoutDecrease, t.minReplicaTimeout)
}
