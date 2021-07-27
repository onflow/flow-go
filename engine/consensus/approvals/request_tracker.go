package approvals

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/storage"
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

// Update creates a _new_ RequestTrackerItem with incremented request number and updated NextTimeout.
func (i RequestTrackerItem) Update() RequestTrackerItem {
	i.Requests++
	i.NextTimeout = randBlackout(i.blackoutPeriodMin, i.blackoutPeriodMax)
	return i
}

func (i RequestTrackerItem) IsBlackout() bool {
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
	headers           storage.Headers
	index             map[flow.Identifier]map[flow.Identifier]map[uint64]RequestTrackerItem
	blackoutPeriodMin int
	blackoutPeriodMax int
	lock              sync.Mutex
	byHeight          map[uint64]flow.IdentifierList
	lowestHeight      uint64
}

// NewRequestTracker instantiates a new RequestTracker with blackout periods
// between min and max seconds.
func NewRequestTracker(headers storage.Headers, blackoutPeriodMin, blackoutPeriodMax int) *RequestTracker {
	return &RequestTracker{
		headers:           headers,
		index:             make(map[flow.Identifier]map[flow.Identifier]map[uint64]RequestTrackerItem),
		byHeight:          make(map[uint64]flow.IdentifierList),
		blackoutPeriodMin: blackoutPeriodMin,
		blackoutPeriodMax: blackoutPeriodMax,
	}
}

// TryUpdate tries to update tracker item if it's not in blackout period. Returns the tracker item for a specific chunk
// (creates it if it doesn't exists) and whenever request item was successfully updated or not.
// Since RequestTracker prunes items by height it can't accept items for height lower than cached lowest height.
// If height of executed block pointed by execution result is smaller than the lowest height, sentinel mempool.DecreasingPruningHeightError is returned.
// In case execution result points to unknown executed block exception will be returned.
func (rt *RequestTracker) TryUpdate(result *flow.ExecutionResult, incorporatedBlockID flow.Identifier, chunkIndex uint64) (RequestTrackerItem, bool, error) {
	resultID := result.ID()
	rt.lock.Lock()
	defer rt.lock.Unlock()
	item, ok := rt.index[resultID][incorporatedBlockID][chunkIndex]

	if !ok {
		item = NewRequestTrackerItem(rt.blackoutPeriodMin, rt.blackoutPeriodMax)
		err := rt.set(resultID, result.BlockID, incorporatedBlockID, chunkIndex, item)
		if err != nil {
			return item, false, fmt.Errorf("could not set created tracker item: %w", err)
		}
	}

	canUpdate := !item.IsBlackout()
	if canUpdate {
		item = item.Update()
		rt.index[resultID][incorporatedBlockID][chunkIndex] = item
	}

	return item, canUpdate, nil
}

// set inserts or updates the tracker item for a specific chunk.
func (rt *RequestTracker) set(resultID, executedBlockID, incorporatedBlockID flow.Identifier, chunkIndex uint64, item RequestTrackerItem) error {
	executedBlock, err := rt.headers.ByBlockID(executedBlockID)
	if err != nil {
		return fmt.Errorf("could not retrieve block by id %v: %w", executedBlockID, err)
	}

	if executedBlock.Height < rt.lowestHeight {
		return mempool.NewDecreasingPruningHeightErrorf(
			"adding height: %d, existing height: %d", executedBlock.Height, rt.lowestHeight)
	}

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

	// update secondary height based index for correct pruning
	rt.byHeight[executedBlock.Height] = append(rt.byHeight[executedBlock.Height], resultID)

	return nil
}

// GetAllIds returns all result IDs that we are indexing
func (rt *RequestTracker) GetAllIds() []flow.Identifier {
	rt.lock.Lock()
	defer rt.lock.Unlock()
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

// PruneUpToHeight remove all tracker items for blocks whose height is strictly
// smaller that height. Note: items for blocks at height are retained.
// After pruning, items for blocks below the given height are dropped.
//
// Monotonicity Requirement:
// The pruned height cannot decrease, as we cannot recover already pruned elements.
// If `height` is smaller than the previous value, the previous value is kept
// and the sentinel mempool.DecreasingPruningHeightError is returned.
func (rt *RequestTracker) PruneUpToHeight(height uint64) error {
	rt.lock.Lock()
	defer rt.lock.Unlock()
	if height < rt.lowestHeight {
		return mempool.NewDecreasingPruningHeightErrorf(
			"pruning height: %d, existing height: %d", height, rt.lowestHeight)
	}

	if len(rt.index) == 0 {
		rt.lowestHeight = height
		return nil
	}

	// Optimization: if there are less elements in the `byHeight` map
	// than the height range to prune: inspect each map element.
	// Otherwise, go through each height to prune.
	if uint64(len(rt.byHeight)) < height-rt.lowestHeight {
		for h := range rt.byHeight {
			if h < height {
				rt.removeByHeight(h)
			}
		}
	} else {
		for h := rt.lowestHeight; h < height; h++ {
			rt.removeByHeight(h)
		}
	}
	rt.lowestHeight = height
	return nil
}

func (rt *RequestTracker) removeByHeight(height uint64) {
	for _, resultID := range rt.byHeight[height] {
		delete(rt.index, resultID)
	}
	delete(rt.byHeight, height)
}
