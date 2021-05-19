package sealing

import (
	"math/rand"
	"sync"
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
func NewRequestTrackerItem(blackoutPeriodMin, blackoutPeriodMax int) RequestTrackerItem {
	item := RequestTrackerItem{
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
// Index on result ID, incorporated block ID and chunk index.
// Is concurrency-safe.
type RequestTracker struct {
	index             map[flow.Identifier]map[flow.Identifier]map[uint64]RequestTrackerItem
	blackoutPeriodMin int
	blackoutPeriodMax int
	lock              sync.RWMutex
}

// NewRequestTracker instantiates a new RequestTracker with blackout periods
// between min and max seconds.
func NewRequestTracker(blackoutPeriodMin, blackoutPeriodMax int) *RequestTracker {
	return &RequestTracker{
		index:             make(map[flow.Identifier]map[flow.Identifier]map[uint64]RequestTrackerItem),
		blackoutPeriodMin: blackoutPeriodMin,
		blackoutPeriodMax: blackoutPeriodMax,
	}
}

// Get returns the tracker item for a specific chunk, and creates a new one if
// it doesn't exist.
func (rt *RequestTracker) Get(resultID, incorporatedBlockID flow.Identifier, chunkIndex uint64) RequestTrackerItem {
	rt.lock.RLock()
	item, ok := rt.index[resultID][incorporatedBlockID][chunkIndex]
	rt.lock.RUnlock()

	if !ok {
		item = NewRequestTrackerItem(rt.blackoutPeriodMin, rt.blackoutPeriodMax)
		rt.Set(resultID, incorporatedBlockID, chunkIndex, item)
	}
	return item
}

// Set inserts or updates the tracker item for a specific chunk.
func (rt *RequestTracker) Set(resultID, incorporatedBlockID flow.Identifier, chunkIndex uint64, item RequestTrackerItem) {
	rt.lock.Lock()
	defer rt.lock.Unlock()
	level1, level1found := rt.index[resultID]
	if !level1found {
		level1 = make(map[flow.Identifier]map[uint64]RequestTrackerItem)
		rt.index[resultID] = level1
	}
	level2, level2found := level1[incorporatedBlockID]
	if !level2found {
		level2 = make(map[uint64]RequestTrackerItem)
		level1[incorporatedBlockID] = level2
	}
	level2[chunkIndex] = item
}

// GetAllIds returns all result IDs that we are indexing
func (rt *RequestTracker) GetAllIds() []flow.Identifier {
	rt.lock.RLock()
	defer rt.lock.RUnlock()
	ids := make([]flow.Identifier, 0, len(rt.index))
	for resultID := range rt.index {
		ids = append(ids, resultID)
	}
	return ids
}

// Remove removes all entries pertaining to an execution result
func (rt *RequestTracker) Remove(resultIDs ...flow.Identifier) {
	if len(resultIDs) == 0 {
		return
	}
	rt.lock.Lock()
	defer rt.lock.Unlock()
	for _, resultID := range resultIDs {
		delete(rt.index, resultID)
	}
}
