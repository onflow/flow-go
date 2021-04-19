package sealing

import (
	"math/rand"
	"time"

	"github.com/onflow/flow-go/model/flow"
)

/*~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
RequestTrackerItem
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~*/

// RequestTrackerItem is an object that keeps track of how many times a request
// has been made, as well as the time until a new request can be made.
// It is not concurrency-safe.
type RequestTrackerItem struct {
	Requests          uint
	NextTimeout       time.Time
	blackoutPeriodMin int
	blackoutPeriodMax int
}

// NewRequestTrackerItem instantiates a new RequestTrackerItem where the
// NextTimeout is evaluated to the current time plus a random blackout period
// contained between min and max.
func NewRequestTrackerItem(blackoutPeriodMin, blackoutPeriodMax int) *RequestTrackerItem {
	item := &RequestTrackerItem{
		blackoutPeriodMin: blackoutPeriodMin,
		blackoutPeriodMax: blackoutPeriodMax,
	}
	item.NextTimeout = randBlackout(blackoutPeriodMin, blackoutPeriodMax)
	return item
}

// Update increments the number of requests and recomputes the NextTimeout.
func (i *RequestTrackerItem) Update() {
	i.Requests++
	i.NextTimeout = randBlackout(i.blackoutPeriodMin, i.blackoutPeriodMax)
}

func (i *RequestTrackerItem) IsBlackout() bool {
	return time.Now().Before(i.NextTimeout)
}

func randBlackout(min int, max int) time.Time {
	blackoutSeconds := rand.Intn(max-min+1) + min
	blackout := time.Now().Add(time.Duration(blackoutSeconds) * time.Second)
	return blackout
}

/*~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
RequestTracker
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~*/

// RequestTracker is an index of RequestTrackerItems indexed by execution result
// ID, incorporated block ID and chunk index.
// It is not concurrency-safe.
type RequestTracker struct {
	index             map[flow.Identifier]map[flow.Identifier]map[uint64]*RequestTrackerItem
	blackoutPeriodMin int
	blackoutPeriodMax int
}

// NewRequestTracker instantiates a new RequestTracker with blackout periods
// between min and max seconds.
func NewRequestTracker(blackoutPeriodMin, blackoutPeriodMax int) *RequestTracker {
	return &RequestTracker{
		index:             make(map[flow.Identifier]map[flow.Identifier]map[uint64]*RequestTrackerItem),
		blackoutPeriodMin: blackoutPeriodMin,
		blackoutPeriodMax: blackoutPeriodMax,
	}
}

// GetAll returns a map of all the items in the tracker indexed by execution
// result ID, incorporated block ID and chunk index.
func (rt *RequestTracker) GetAll() map[flow.Identifier]map[flow.Identifier]map[uint64]*RequestTrackerItem {
	return rt.index
}

// Get returns the tracker item for a specific chunk, and creates a new one if
// it doesn't exist.
func (rt *RequestTracker) Get(resultID, incorporatedBlockID flow.Identifier, chunkIndex uint64) *RequestTrackerItem {
	item, ok := rt.index[resultID][incorporatedBlockID][chunkIndex]
	if !ok {
		item = NewRequestTrackerItem(rt.blackoutPeriodMin, rt.blackoutPeriodMax)
		rt.Set(resultID, incorporatedBlockID, chunkIndex, item)
	}
	return item
}

// Set inserts or updates the tracker item for a specific chunk.
func (rt *RequestTracker) Set(resultID, incorporatedBlockID flow.Identifier, chunkIndex uint64, item *RequestTrackerItem) {
	_, ok := rt.index[resultID]
	if !ok {
		rt.index[resultID] = make(map[flow.Identifier]map[uint64]*RequestTrackerItem)
	}
	_, ok = rt.index[resultID][incorporatedBlockID]
	if !ok {
		rt.index[resultID][incorporatedBlockID] = make(map[uint64]*RequestTrackerItem)
	}

	rt.index[resultID][incorporatedBlockID][chunkIndex] = item
}

// Remove removes all entries pertaining to an execution result
func (rt *RequestTracker) Remove(resultID, incorporatedBlockID flow.Identifier) {
	index, ok := rt.index[resultID]
	if !ok {
		return
	}
	delete(index, incorporatedBlockID)
	if len(index) == 0 {
		delete(rt.index, resultID)
	}
}
