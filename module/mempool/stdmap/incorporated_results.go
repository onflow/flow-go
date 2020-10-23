package stdmap

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool/model"
	"github.com/onflow/flow-go/storage"
)

// IncorporatedResults implements the incorporated results memory pool of the
// consensus nodes, used to store results that need to be sealed.
type IncorporatedResults struct {
	backend *Backend
	size    *uint
}

// NewIncorporatedResults creates a mempool for the incorporated results.
func NewIncorporatedResults(limit uint) *IncorporatedResults {
	var size uint
	ejector := NewSizeEjector(&size)
	return &IncorporatedResults{
		size: &size,
		backend: NewBackend(
			WithLimit(limit),
			WithEject(ejector.Eject),
		),
	}
}

// Add adds an IncorporatedResult to the mempool.
func (ir *IncorporatedResults) Add(incorporatedResult *flow.IncorporatedResult) bool {

	key := incorporatedResult.Result.ID()

	appended := false
	err := ir.backend.Run(func(backdata map[flow.Identifier]flow.Entity) error {

		var incResults map[flow.Identifier]*flow.IncorporatedResult

		entity, ok := backdata[key]
		if !ok {
			// no record with key is available in the mempool,
			// initialise incResults.
			incResults = make(map[flow.Identifier]*flow.IncorporatedResult)
			// add the new map to mempool for holding all incorporated results for the same result.ID
			backdata[key] = model.IncorporatedResultMap{
				ExecutionResult:     incorporatedResult.Result,
				IncorporatedResults: incResults,
			}
		} else {
			incorporatedResultMap, ok := entity.(model.IncorporatedResultMap)
			if !ok {
				return fmt.Errorf("unexpected entity type %T", entity)
			}

			incResults = incorporatedResultMap.IncorporatedResults
			if _, ok := incResults[incorporatedResult.IncorporatedBlockID]; ok {
				// incorporated result is already associated with result and
				// incorporated block.
				return nil
			}
		}

		// appends incorporated result to the map
		incResults[incorporatedResult.IncorporatedBlockID] = incorporatedResult
		appended = true
		*ir.size++
		return nil
	})
	if err != nil {
		// The current implementation never reaches this path, as it only stores
		// IncorporatedResultMap as entities in the mempool. Reaching this error
		// condition implies this code was inconsistently modified.
		panic("unexpected internal error in IncorporatedResults mempool: " + err.Error())
	}

	return appended
}

// All returns all the items in the mempool.
func (ir *IncorporatedResults) All() []*flow.IncorporatedResult {
	// To guarantee concurrency safety, we need to copy the map via a locked operation in the backend.
	// Otherwise, another routine might concurrently modify the maps stored as mempool entities.
	res := make([]*flow.IncorporatedResult, 0)
	err := ir.backend.Run(func(backdata map[flow.Identifier]flow.Entity) error {
		for _, entity := range backdata {
			irMap, ok := entity.(model.IncorporatedResultMap)
			if !ok {
				// should never happen: as the mempoo
				return fmt.Errorf("unexpected entity type %T", entity)
			}
			for _, ir := range irMap.IncorporatedResults {
				res = append(res, ir)
			}
		}
		return nil
	})
	if err != nil {
		// The current implementation never reaches this path, as it only stores
		// IncorporatedResultMap as entities in the mempool. Reaching this error
		// condition implies this code was inconsistently modified.
		panic("unexpected internal error in IncorporatedResults mempool: " + err.Error())
	}

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
		irMap, ok := entity.(model.IncorporatedResultMap)
		if !ok {
			// should never happen: as the mempoo
			return fmt.Errorf("unexpected entity type %T", entity)
		}
		result = irMap.ExecutionResult
		for i, res := range irMap.IncorporatedResults {
			incResults[i] = res
		}
		return nil
	})
	if errors.Is(err, storage.ErrNotFound) {
		return nil, nil, false
	} else if err != nil {
		// The current implementation never reaches this path, as it only stores
		// IncorporatedResultMap as entities in the mempool. Reaching this error
		// condition implies this code was inconsistently modified.
		panic("unexpected internal error in IncorporatedResults mempool: " + err.Error())
	}

	return result, incResults, true
}

// Rem removes an IncorporatedResult from the mempool.
func (ir *IncorporatedResults) Rem(incorporatedResult *flow.IncorporatedResult) bool {
	key := incorporatedResult.Result.ID()

	removed := false
	err := ir.backend.Run(func(backdata map[flow.Identifier]flow.Entity) error {
		var incResults map[flow.Identifier]*flow.IncorporatedResult

		entity, ok := backdata[key]
		if !ok {
			// there are no items for this result
			return nil
		}
		incorporatedResultMap, ok := entity.(model.IncorporatedResultMap)
		if !ok {
			return fmt.Errorf("unexpected entity type %T", entity)
		}

		incResults = incorporatedResultMap.IncorporatedResults
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
		*ir.size--
		return nil
	})
	if err != nil {
		// The current implementation never reaches this path, as it only stores
		// IncorporatedResultMap as entities in the mempool. Reaching this error
		// condition implies this code was inconsistently modified.
		panic("unexpected internal error in IncorporatedResults mempool: " + err.Error())
	}

	return removed
}

// Size returns the number of incorporated results in the mempool.
func (ir *IncorporatedResults) Size() uint {
	return *ir.size
}
