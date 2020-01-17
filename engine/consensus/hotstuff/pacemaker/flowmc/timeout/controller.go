package timeout

import (
	"time"
)

// Controller implements a timout with:
// - on timeout: increase timeout by multiplicative factor `timeoutIncrease` (user-specified)
//   this results in exponential growing timeout duration on multiple subsequent timeouts
// - on progress: decrease timeout by subtrahend `timeoutDecrease`
type Controller struct {
	TimoutConfig
	mode  TimeoutMode
	timer *time.Timer
}

type TimeoutMode int

const (
	ReplicaTimeout        TimeoutMode = iota
	VoteCollectionTimeout TimeoutMode = iota
)

// NewController creates a new Controller.
func NewController(timeoutConfig TimoutConfig) *Controller {
	tc := Controller{
		TimoutConfig: timeoutConfig,
	}
	return &tc
}

func DefaultController() *Controller {
	return NewController(*DefaultConfig())
}

func (t *Controller) StartTimeout(mode TimeoutMode) {
	t.mode = mode
	switch mode {
	case VoteCollectionTimeout:
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

// VoteCollectionTimeout returns the duration of Vote aggregation _after_ receiving a block
// during which the primary tries to aggregate votes for the view where it is leader
func (t *Controller) VoteCollectionTimeout() time.Duration {
	// time.Duration expects an int64 as input which specifies the duration in units of nanoseconds (1E-9)
	return time.Duration(t.replicaTimeout * 1E6 * t.voteAggregationTimeoutFraction)
}

// OnTimeout indicates to the Controller that the timeout was reached
func (t *Controller) OnTimeout() {
	t.replicaTimeout *= t.timeoutIncrease
}

// OnProgressBeforeTimeout indicates to the Controller that progress was made _before_ the timeout was reached
func (t *Controller) OnProgressBeforeTimeout() {
	t.replicaTimeout -= t.timeoutDecrease
	if t.replicaTimeout < t.minReplicaTimeout {
		t.replicaTimeout = t.minReplicaTimeout
	}
}
