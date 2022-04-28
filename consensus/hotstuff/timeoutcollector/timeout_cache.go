package timeoutcollector

import (
	"errors"
	"sync"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

var (
	// ErrRepeatedTimeout is emitted, when we receive an identical timeout object for the same block
	// from the same voter multiple times. This error does _not_ indicate
	// equivocation.
	ErrRepeatedTimeout            = errors.New("duplicated timeout")
	ErrTimeoutForIncompatibleView = errors.New("timeout for incompatible view")
)

// TimeoutObjectsCache maintains a _concurrency safe_ cache of timeouts for one particular
// view. The cache memorizes the order in which the timeouts were received. Timeouts
// are de-duplicated based on the following rules:
//  * For each voter (i.e. SignerID), we store the _first_ timeout t0.
//  * For any subsequent timeout t, we check whether t.ID() == t0.ID().
//    If this is the case, we consider the timeout a duplicate and drop it.
//    If t and t0 have different checksums, the voter is equivocating, and
//    we return a model.DoubleTimeoutError.
type TimeoutObjectsCache struct {
	lock     sync.RWMutex
	view     uint64
	timeouts map[flow.Identifier]*model.TimeoutObject // signerID -> first timeout
}

// NewTimeoutObjectsCache instantiates a TimeoutObjectsCache for the given view
func NewTimeoutObjectsCache(view uint64) *TimeoutObjectsCache {
	return &TimeoutObjectsCache{
		view:     view,
		timeouts: make(map[flow.Identifier]*model.TimeoutObject),
	}
}

func (vc *TimeoutObjectsCache) View() uint64 { return vc.view }

// AddTimeoutObject stores a timeout in the cache. The following errors are expected during
// normal operations:
//  * nil: if the timeout was successfully added
//  * model.DoubleTimeoutError is returned if the replica is equivocating
//  * RepeatedTimeoutErr is returned when adding an _identical_ timeout for the same view from
//    the same voter multiple times.
//  * TimeoutForIncompatibleViewError is returned if the timeout is for a different view.
// When AddTimeoutObject returns an error, the timeout is _not_ stored.
func (vc *TimeoutObjectsCache) AddTimeoutObject(timeout *model.TimeoutObject) error {
	if timeout.View != vc.view {
		return ErrTimeoutForIncompatibleView
	}
	vc.lock.Lock()
	defer vc.lock.Unlock()

	// De-duplicated timeouts based on the following rules:
	//  * For each voter (i.e. SignerID), we store the _first_  t0.
	//  * For any subsequent timeout t, we check whether t.ID() == t0.ID().
	//    If this is the case, we consider the timeout a duplicate and drop it.
	//    If t and t0 have different checksums, the voter is equivocating, and
	//    we return a model.DoubleTimeoutError.
	firstTimeout, exists := vc.timeouts[timeout.SignerID]
	if exists {
		// TODO: once we have signer indices, implement Equals methods for QC, TC
		// and TimeoutObjects, to avoid the comparatively very expensive ID computation.
		if firstTimeout.ID() != timeout.ID() {
			return model.NewDoubleTimeoutErrorf(firstTimeout, timeout, "detected timeout equivocation by replica %x at view: %d", timeout.SignerID, vc.view)
		}
		return ErrRepeatedTimeout
	}
	vc.timeouts[timeout.SignerID] = timeout

	return nil
}

// GetTimeoutObject returns the stored timeout for the given `signerID`. Returns:
//  - (timeout, true) if a timeout object from signerID is known
//  - (nil, false) no timeout object from signerID is known
func (vc *TimeoutObjectsCache) GetTimeoutObject(signerID flow.Identifier) (*model.TimeoutObject, bool) {
	vc.lock.RLock()
	timeout, exists := vc.timeouts[signerID] // if signerID is unknown, its `Vote` pointer is nil
	vc.lock.RUnlock()
	return timeout, exists
}

// Size returns the number of cached timeout objects
func (vc *TimeoutObjectsCache) Size() int {
	vc.lock.RLock()
	s := len(vc.timeouts)
	vc.lock.RUnlock()
	return s
}

// All returns all currently cached timeout objects. Concurrency safe.
func (vc *TimeoutObjectsCache) All() []*model.TimeoutObject {
	vc.lock.RLock()
	defer vc.lock.RUnlock()
	return vc.all()
}

// all returns all currently cached timeout objects. NOT concurrency safe
func (vc *TimeoutObjectsCache) all() []*model.TimeoutObject {
	timeoutObjects := make([]*model.TimeoutObject, 0, len(vc.timeouts))
	for _, t := range vc.timeouts {
		timeoutObjects = append(timeoutObjects, t)
	}
	return timeoutObjects
}
