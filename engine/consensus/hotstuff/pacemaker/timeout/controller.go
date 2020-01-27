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
	mode           types.TimeoutMode
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

func (t *Controller) Channel() <-chan *types.Timeout { return t.timeoutChannel }
func (t *Controller) Mode() types.TimeoutMode        { return t.mode }

func (t *Controller) StartTimeout(mode types.TimeoutMode, view uint64) {
	t.mode = mode
	var timerChannel <-chan time.Time
	switch mode {
	case types.VoteCollectionTimeout:
		timerChannel = time.NewTimer(t.VoteCollectionTimeoutTimeout()).C
	case types.ReplicaTimeout:
		timerChannel = time.NewTimer(t.ReplicaTimeout()).C
	default:
		// This should never happen; Only protects code from future inconsistent modifications.
		// There are only the two timeout modes explicitly handled above. Unless the enum
		// containing the timeout mode is extended, the default case will never be reached.
		panic("unknown timeout mode")
	}
	timeoutChannel := make(chan *types.Timeout)
	go func() {
		time := <-timerChannel
		timeoutChannel <- &types.Timeout{
			Mode:         mode,
			View:         view,
			TimeoutFired: time,
		}
	}()
	t.timeoutChannel = timeoutChannel
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
