package timeoutcollector

import (
	"errors"
	"sync"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

var (
	// RepeatedTimeoutErr is emitted, when we receive a vote for the same block
	// from the same voter multiple times. This error does _not_ indicate
	// equivocation.
	RepeatedTimeoutErr              = errors.New("duplicated timeout")
	TimeoutForIncompatibleViewError = errors.New("timeout for incompatible view")
)

// TimeoutObjectsCache maintains a _concurrency safe_ cache of timeouts for one particular
// view. The cache memorizes the order in which the timeouts were received. Votes
// are de-duplicated based on the following rules:
//  * Vor each voter (i.e. SignerID), we store the _first_ vote v0.
//  * For any subsequent vote v, we check whether v.BlockID == v0.BlockID.
//    If this is the case, we consider the vote a duplicate and drop it.
//    If v and v0 have different BlockIDs, the voter is equivocating and
//    we return a model.DoubleVoteError
// .
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
//  * nil: if the vote was successfully added
//  * model.DoubleTimeoutError is returned if the voter is equivocating
//    (i.e. voting in the same view for different blocks).
//  * RepeatedTimeoutErr is returned when adding a vote for the same block from
//    the same voter multiple times.
//  * TimeoutForIncompatibleViewError is returned if the timeout is for a different view.
// When AddTimeoutObject returns an error, the timeout is _not_ stored.
func (vc *TimeoutObjectsCache) AddTimeoutObject(timeout *model.TimeoutObject) error {
	if timeout.View != vc.view {
		return TimeoutForIncompatibleViewError
	}
	vc.lock.Lock()
	defer vc.lock.Unlock()

	// De-duplicated timeouts based on the following rules:
	//  * Vor each voter (i.e. SignerID), we store the _first_ vote v0.
	//  * For any subsequent vote v, we check whether v.BlockID == v0.BlockID.
	//    If this is the case, we consider the vote a duplicate and drop it.
	//    If v and v0 have different BlockIDs, the voter is equivocating and
	//    we return a model.DoubleVoteError
	firstTimeout, exists := vc.timeouts[timeout.SignerID]
	if exists {
		if firstTimeout.ID() != timeout.ID() {
			return model.NewDoubleTimeoutErrorf(firstTimeout, timeout, "detected timeout equivocation at view: %d", vc.view)
		}
		return RepeatedTimeoutErr
	}
	vc.timeouts[timeout.SignerID] = timeout

	return nil
}

// GetTimeoutObject returns the stored vote for the given `signerID`. Returns:
//  - (vote, true) if a vote from signerID is known
//  - (false, nil) no vote from signerID is known
func (vc *TimeoutObjectsCache) GetTimeoutObject(signerID flow.Identifier) (*model.TimeoutObject, bool) {
	vc.lock.RLock()
	timeout, exists := vc.timeouts[signerID] // if signerID is unknown, its `Vote` pointer is nil
	vc.lock.RUnlock()
	return timeout, exists
}

// Size returns the number of cached timeouts
func (vc *TimeoutObjectsCache) Size() int {
	vc.lock.RLock()
	s := len(vc.timeouts)
	vc.lock.RUnlock()
	return s
}

// All returns all currently cached timeouts. Concurrency safe.
func (vc *TimeoutObjectsCache) All() []*model.TimeoutObject {
	vc.lock.Lock()
	defer vc.lock.Unlock()
	return vc.all()
}

// all returns all currently cached timeouts. NOT concurrency safe
func (vc *TimeoutObjectsCache) all() []*model.TimeoutObject {
	timeoutObjects := make([]*model.TimeoutObject, len(vc.timeouts))
	for _, t := range vc.timeouts {
		timeoutObjects = append(timeoutObjects, t)
	}
	return timeoutObjects
}
