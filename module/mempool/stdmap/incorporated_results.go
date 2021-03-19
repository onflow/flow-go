package stdmap

import (
	"errors"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool/model"
	"github.com/onflow/flow-go/storage"
)

// IncorporatedResults implements the incorporated results memory pool of the
// consensus nodes, used to store results that need to be sealed.
type IncorporatedResults struct {
	// Concurrency: the mempool internally re-uses the backend's lock

	backend *Backend
	size    uint
}

// NewIncorporatedResults creates a mempool for the incorporated results.
func NewIncorporatedResults(limit uint, opts ...OptionFunc) (*IncorporatedResults, error) {
	mempool := &IncorporatedResults{
		size:    0,
		backend: NewBackend(append(opts, WithLimit(limit))...),
	}

	adjustSizeOnEjection := func(entity flow.Entity) {
		// uncaught type assertion; should never panic as the mempool only stores IncorporatedResultMap:
		incorporatedResultMap := entity.(*model.IncorporatedResultMap)
		mempool.size -= uint(len(incorporatedResultMap.IncorporatedResults))
	}
	mempool.backend.RegisterEjectionCallbacks(adjustSizeOnEjection)

	return mempool, nil
}

// Add adds an IncorporatedResult to the mempool.
func (ir *IncorporatedResults) Add(incorporatedResult *flow.IncorporatedResult) (bool, error) {

	key := incorporatedResult.Result.ID()

	appended := false
	err := ir.backend.Run(func(backdata map[flow.Identifier]flow.Entity) error {

		var incResults map[flow.Identifier]*flow.IncorporatedResult

		entity, ok := backdata[key]
		if !ok {
			// no record with key is available in the mempool, initialise
			// incResults.
			incResults = make(map[flow.Identifier]*flow.IncorporatedResult)
			// add the new map to mempool for holding all incorporated results for the same result.ID
			backdata[key] = &model.IncorporatedResultMap{
				ExecutionResult:     incorporatedResult.Result,
				IncorporatedResults: incResults,
			}
		} else {
			// uncaught type assertion; should never panic as the mempool only stores IncorporatedResultMap:
			incResults = entity.(*model.IncorporatedResultMap).IncorporatedResults
			if _, ok := incResults[incorporatedResult.IncorporatedBlockID]; ok {
				// incorporated result is already associated with result and
				// incorporated block.
				return nil
			}
		}

		// appends incorporated result to the map
		incResults[incorporatedResult.IncorporatedBlockID] = incorporatedResult
		appended = true
		ir.size++
		return nil
	})

	return appended, err
}

// All returns all the items in the mempool.
func (ir *IncorporatedResults) All() flow.IncorporatedResultList {
	// To guarantee concurrency safety, we need to copy the map via a locked operation in the backend.
	// Otherwise, another routine might concurrently modify the maps stored as mempool entities.
	res := make([]*flow.IncorporatedResult, 0)
	_ = ir.backend.Run(func(backdata map[flow.Identifier]flow.Entity) error {
		for _, entity := range backdata {
			// uncaught type assertion; should never panic as the mempool only stores IncorporatedResultMap:
			for _, ir := range entity.(*model.IncorporatedResultMap).IncorporatedResults {
				res = append(res, ir)
			}
		}
		return nil
	}) // error return impossible

	return res
}

// ByResultID returns all the IncorporatedResults that contain a specific
// ExecutionResult, indexed by IncorporatedBlockID.
func (ir *IncorporatedResults) ByResultID(resultID flow.Identifier) (*flow.ExecutionResult, map[flow.Identifier]*flow.IncorporatedResult, bool) {
	// To guarantee concurrency safety, we need to copy the map via a locked operation in the backend.
	// Otherwise, another routine might concurrently modify the map stored for the same resultID.
	var result *flow.ExecutionResult
	incResults := make(map[flow.Identifier]*flow.IncorporatedResult)
	err := ir.backend.Run(func(backdata map[flow.Identifier]flow.Entity) error {
		entity, exists := backdata[resultID]
		if !exists {
			return storage.ErrNotFound
		}
		// uncaught type assertion; should never panic as the mempool only stores IncorporatedResultMap:
		irMap := entity.(*model.IncorporatedResultMap)
		result = irMap.ExecutionResult
		for i, res := range irMap.IncorporatedResults {
			incResults[i] = res
		}
		return nil
	})
	if errors.Is(err, storage.ErrNotFound) {
		return nil, nil, false
	} else if err != nil {
		// The current implementation never reaches this path
		panic("unexpected internal error in IncorporatedResults mempool: " + err.Error())
	}

	return result, incResults, true
}

// Rem removes an IncorporatedResult from the mempool.
func (ir *IncorporatedResults) Rem(incorporatedResult *flow.IncorporatedResult) bool {
	key := incorporatedResult.Result.ID()

	removed := false
	_ = ir.backend.Run(func(backdata map[flow.Identifier]flow.Entity) error {
		var incResults map[flow.Identifier]*flow.IncorporatedResult

		entity, ok := backdata[key]
		if !ok {
			// there are no items for this result
			return nil
		}
		// uncaught type assertion; should never panic as the mempool only stores IncorporatedResultMap:
		incResults = entity.(*model.IncorporatedResultMap).IncorporatedResults
		if _, ok := incResults[incorporatedResult.IncorporatedBlockID]; !ok {
			// there are no items for this IncorporatedBlockID
			return nil
		}
		if len(incResults) == 1 {
			// special case: there is only a single Incorporated result stored for this Result.ID()
			// => remove entire map
			delete(backdata, key)
		} else {
			// remove item from map
			delete(incResults, incorporatedResult.IncorporatedBlockID)
		}

		removed = true
		ir.size--
		return nil
	}) // error return impossible

	return removed
}

// Size returns the number of incorporated results in the mempool.
func (ir *IncorporatedResults) Size() uint {
	// To guarantee concurrency safety, i.e. that the read retrieves the latest size value,
	// we need run the read through a locked operation in the backend.
	// To guarantee concurrency safety, i.e. that the read retrieves the latest size value,
	// we need run utilize the backend's lock.
	ir.backend.RLock()
	defer ir.backend.RUnlock()
	return ir.size
}
