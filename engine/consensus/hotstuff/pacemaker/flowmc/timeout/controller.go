package timeout

import (
	"time"
)

// Controller implements a timout with:
// - on timeout: increase timeout by multiplicative factor `timeoutIncrease` (user-specified)
//   this results in exponential growing timeout duration on multiple subsequent timeouts
// - on progress: decrease timeout by subtrahend `timeoutDecrease`
type Controller struct {
	Config
	mode  TimeoutMode
	timer *time.Timer
}

type TimeoutMode int

const (
	ReplicaTimeout TimeoutMode = iota
	VoteCollection TimeoutMode = iota
)

// timeoutCap this is an internal cap on the timeout to avoid numerical overflows.
// Its value is large enough to be of no practical implication.
// We use 1E9 milliseconds which is about 11 days for a single timout (i.e. more than a full epoch)
const timeoutCap float64 = 1E9

// NewController creates a new Controller.
func NewController(timeoutConfig Config) *Controller {
	tc := Controller{
		Config: timeoutConfig,
	}
	return &tc
}

func DefaultController() *Controller {
	return NewController(*DefaultConfig())
}

func (t *Controller) StartTimeout(mode TimeoutMode) {
	t.mode = mode
	switch mode {
	case VoteCollection:
		t.timer = time.NewTimer(t.VoteCollectionTimeout())
	case ReplicaTimeout:
		t.timer = time.NewTimer(t.ReplicaTimeout())
	default:
		panic("unknown timeout mode")
	}

}

func (t *Controller) Channel() <-chan time.Time { return t.timer.C }
func (t *Controller) Mode() TimeoutMode         { return t.mode }

// ReplicaTimeout returns the duration of the current view before we time out
func (t *Controller) ReplicaTimeout() time.Duration {
	return time.Duration(t.replicaTimeout * 1E6)
}

// VoteCollection returns the duration of Vote aggregation _after_ receiving a block
// during which the primary tries to aggregate votes for the view where it is leader
func (t *Controller) VoteCollectionTimeout() time.Duration {
	// time.Duration expects an int64 as input which specifies the duration in units of nanoseconds (1E-9)
	return time.Duration(t.replicaTimeout * 1E6 * t.voteAggregationTimeoutFraction)
}

// OnTimeout indicates to the Controller that the timeout was reached
func (t *Controller) OnTimeout() {
	t.replicaTimeout *= t.timeoutIncrease
	if t.replicaTimeout > timeoutCap {
		t.replicaTimeout = timeoutCap
	}
}

// OnProgressBeforeTimeout indicates to the Controller that progress was made _before_ the timeout was reached
func (t *Controller) OnProgressBeforeTimeout() {
	t.replicaTimeout -= t.timeoutDecrease
	if t.replicaTimeout < t.minReplicaTimeout {
		t.replicaTimeout = t.minReplicaTimeout
	}
}
