package matching

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
// contained by the provided min and max.
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

func randBlackout(min int, max int) time.Time {
	blackoutSeconds := rand.Intn(max-min) + min
	blackout := time.Now().Add(time.Duration(blackoutSeconds) * time.Second)
	return blackout
}

/*~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
RequestTracker
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~*/

// RequestTracker is an index of RequestTrackerItems indexed by execution result
// ID and chunk index.
// It is not concurrency-safe.
type RequestTracker struct {
	index map[flow.Identifier]map[uint64]*RequestTrackerItem
}

// NewRequestTracker instantiates a new RequestTracker.
func NewRequestTracker() *RequestTracker {
	return &RequestTracker{
		index: make(map[flow.Identifier]map[uint64]*RequestTrackerItem),
	}
}

// GetAll returns a map of all the items in the tracker indexed by execution
// result ID and chunk index.
func (rt *RequestTracker) GetAll() map[flow.Identifier]map[uint64]*RequestTrackerItem {
	return rt.index
}

// Get returns the tracker item for a specific chunk.
func (rt *RequestTracker) Get(resultID flow.Identifier, chunkIndex uint64) (*RequestTrackerItem, bool) {
	item, ok := rt.index[resultID][chunkIndex]
	return item, ok
}

// Set inserts or updates the tracker item for a specific chunk.
func (rt *RequestTracker) Set(resultID flow.Identifier, chunkIndex uint64, item *RequestTrackerItem) {
	_, ok := rt.index[resultID]
	if !ok {
		rt.index[resultID] = make(map[uint64]*RequestTrackerItem)
	}
	rt.index[resultID][chunkIndex] = item
}

// Remove removes all entries pertaining to an execution result
func (rt *RequestTracker) Remove(resultID flow.Identifier) {
	delete(rt.index, resultID)
}
