package flowmc

import "time"

type TimeoutMode int

const (
	ReplicaTimeout        TimeoutMode = iota
	VoteCollectionTimeoutTimeout TimeoutMode = iota
)

type activatedTimeout struct {
	view  uint64
	mode  TimeoutMode
	timer *time.Timer
}

type Timeout struct {
	TimoutController
	activatedTimeout activatedTimeout
}

func DefaultTimout() Timeout {
	return Timeout{
		TimoutController: DefaultTimoutController(),
		activatedTimeout: activatedTimeout{
			view: 0,
			mode: ReplicaTimeout,
		},
	}
}

func (t *Timeout) StartTimeout(view uint64, mode TimeoutMode) {
	if t.activatedTimeout.view > view {
		panic("View for Timers must be strictly monotonously increasing")
	}
	if t.activatedTimeout.view == view {
		if !(t.activatedTimeout.mode == ReplicaTimeout && mode == VoteCollectionTimeoutTimeout) {
			panic("For same view: can only transition from ReplicaTimeout to VoteCollectionTimeoutTimeout")
		}
		// we are transitioning from ReplicaTimeout to VoteCollectionTimeoutTimeout
		t.initTimeout(view, mode)
	}
	// t.activatedTimeout.view < view
	t.onProgressBeforeTimeout()
	t.initTimeout(view, mode)
}

func (t *Timeout) Channel() <-chan time.Time { return t.activatedTimeout.timer.C }
func (t *Timeout) Mode() TimeoutMode         { return t.activatedTimeout.mode }
func (t *Timeout) View() uint64              { return t.activatedTimeout.view }

func (t *Timeout) initTimeout(view uint64, mode TimeoutMode) {
	t.activatedTimeout.view = view
	t.activatedTimeout.mode = mode
	switch mode {
	case VoteCollectionTimeoutTimeout:
		t.activatedTimeout.timer = time.NewTimer(t.VoteCollectionTimeoutTimeout())
	case ReplicaTimeout:
		t.activatedTimeout.timer = time.NewTimer(t.ReplicaTimeout())
	default:
		panic("unknown timeout mode")
	}
}

// TimoutController implements a timout with:
// - on timeout: increase timeout by multiplicative factor `timeoutIncrease` (user-specified)
//   this results in exponential growing timeout duration on multiple subsequent timeouts
// - on progress: decrease timeout by subtrahend `timeoutDecrease`
type TimoutController struct {
	// replicaTimeout is the duration of a view before we time out [Milliseconds]
	// replicaTimeout is the only variable quantity
	replicaTimeout float64

	// replicaTimeout is the duration of a view before we time out [Milliseconds]
	minReplicaTimeout float64
	// voteAggregationTimeoutFraction is a fraction of replicaTimeout which is reserved for aggregating votes
	voteAggregationTimeoutFraction float64
	// timeoutDecrease: multiplicative factor for increasing timeout
	timeoutIncrease float64
	// timeoutDecrease linear subtrahend for timeout decrease [Milliseconds]
	timeoutDecrease float64
}

func DefaultTimoutController() TimoutController {
	return TimoutController{
		replicaTimeout:                 800,
		minReplicaTimeout:              800,
		voteAggregationTimeoutFraction: 0.5,
		timeoutIncrease:                1.5,
		timeoutDecrease:                500,
	}
}

// NewTimoutController creates a new TimoutController.
// minReplicaTimeout: minimal timeout value for replica round [Milliseconds], also used as starting value;
// voteAggregationTimeoutFraction: fraction of replicaTimeout which is reserved for aggregating votes;
// timeoutIncrease: multiplicative factor for increasing timeout;
// timeoutDecrease: linear subtrahend for timeout decrease [Milliseconds]
func NewTimoutController(minReplicaTimeout, voteAggregationTimeoutFraction, timeoutIncrease, timeoutDecrease float64) *TimoutController {
	if !((0 < voteAggregationTimeoutFraction) && (voteAggregationTimeoutFraction <= 1)) {
		panic("VoteAggregationTimeoutFraction must be in range (0,1]")
	}
	if timeoutIncrease <= 1 {
		panic("TimeoutIncrease must be strictly bigger than 1")
	}
	return &TimoutController{
		replicaTimeout:                 minReplicaTimeout,
		minReplicaTimeout:              ensurePositive(minReplicaTimeout, "MinReplicaTimeout"),
		voteAggregationTimeoutFraction: voteAggregationTimeoutFraction,
		timeoutIncrease:                timeoutIncrease,
		timeoutDecrease:                ensurePositive(timeoutDecrease, "TimeoutDecrease"),
	}
}

// ReplicaTimeout returns the duration of the current view before we time out
func (t *TimoutController) ReplicaTimeout() time.Duration {
	return time.Duration(t.replicaTimeout * 1E6)
}

// VoteCollectionTimeoutTimeout returns the duration of Vote aggregation _after_ receiving a block
// during which the primary tries to aggregate votes for the view where it is leader
func (t *TimoutController) VoteCollectionTimeoutTimeout() time.Duration {
	// time.Duration expects an int64 as input which specifies the duration in units of nanoseconds (1E-9)
	return time.Duration(t.replicaTimeout * 1E6 * t.voteAggregationTimeoutFraction)
}

// OnTimeout indicates to the TimoutController that the timeout was reached
func (t *TimoutController) OnTimeout() {
	t.replicaTimeout *= t.timeoutIncrease
}

// OnProgressBeforeTimeout indicates to the TimoutController that progress was made _before_ the timeout was reached
func (t *TimoutController) onProgressBeforeTimeout() {
	t.replicaTimeout -= t.timeoutDecrease
	if t.replicaTimeout < t.minReplicaTimeout {
		t.replicaTimeout = t.minReplicaTimeout
	}
}

func ensurePositive(value float64, varName string) float64 {
	if value <= 0 {
		panic(varName + " must be positive")
	}
	return value
}
