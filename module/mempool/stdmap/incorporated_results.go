package stdmap

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool/model"
)

// IncorporatedResults implements the incorporated results memory pool of the
// consensus nodes, used to store results that need to be sealed.
type IncorporatedResults struct {
	*Backend
	size uint
}

// NewIncorporatedResults creates a mempool for the incorporated results.
func NewIncorporatedResults(limit uint) *IncorporatedResults {
	return &IncorporatedResults{
		Backend: NewBackend(WithLimit(limit)),
	}
}

// Add adds an IncorporatedResult to the mempool.
func (ir *IncorporatedResults) Add(incorporatedResult *flow.IncorporatedResult) (bool, error) {

	key := incorporatedResult.Result.ID()

	appended := false
	err := ir.Backend.Run(func(backdata map[flow.Identifier]flow.Entity) error {

		var incResults map[flow.Identifier]*flow.IncorporatedResult

		entity, ok := backdata[key]
		if !ok {
			// no record with key is available in the mempool, initialise
			// incResults.
			incResults = make(map[flow.Identifier]*flow.IncorporatedResult)
		} else {
			incorporatedResultMap, ok := entity.(model.IncorporatedResultMap)
			if !ok {
				return fmt.Errorf("could not assert entity to IncorporatedResultMap")
			}

			incResults = incorporatedResultMap.IncorporatedResults

			if _, ok := incResults[incorporatedResult.IncorporatedBlockID]; ok {
				// incorporated result is already associated with result and
				// incorporated block.
				return nil
			}

			// removes map entry associated with key for update
			delete(backdata, key)
		}

		// appends incorporated result to the map
		incResults[incorporatedResult.IncorporatedBlockID] = incorporatedResult

		// adds the new incorporated results map associated with key to mempool
		incorporatedResultMap := model.IncorporatedResultMap{
			ExecutionResult:     incorporatedResult.Result,
			IncorporatedResults: incResults,
		}

		backdata[key] = incorporatedResultMap
		appended = true
		ir.size++
		return nil
	})

	return appended, err
}

// All returns all the items in the mempool.
func (ir *IncorporatedResults) All() []*flow.IncorporatedResult {
	res := make([]*flow.IncorporatedResult, 0)

	entities := ir.Backend.All()
	for _, entity := range entities {
		irMap, _ := entity.(model.IncorporatedResultMap)

		for _, ir := range irMap.IncorporatedResults {
			res = append(res, ir)
		}
	}

	return res
}

// ByResultID returns all the IncorporatedResults that contain a specific
// ExecutionResult, indexed by IncorporatedBlockID.
func (ir *IncorporatedResults) ByResultID(resultID flow.Identifier) (*flow.ExecutionResult, map[flow.Identifier]*flow.IncorporatedResult) {

	entity, exists := ir.Backend.ByID(resultID)
	if !exists {
		return nil, nil
	}

	irMap, ok := entity.(model.IncorporatedResultMap)
	if !ok {
		return nil, nil
	}

	return irMap.ExecutionResult, irMap.IncorporatedResults
}

// Rem removes an IncorporatedResult from the mempool.
func (ir *IncorporatedResults) Rem(incorporatedResult *flow.IncorporatedResult) bool {
	key := incorporatedResult.Result.ID()

	removed := false
	_ = ir.Backend.Run(func(backdata map[flow.Identifier]flow.Entity) error {
		var incResults map[flow.Identifier]*flow.IncorporatedResult

		entity, ok := backdata[key]
		if !ok {
			// there are no items for this result
			return nil
		} else {
			incorporatedResultMap, ok := entity.(model.IncorporatedResultMap)
			if !ok {
				return fmt.Errorf("could not assert entity to IncorporatedResultMap")
			}

			incResults = incorporatedResultMap.IncorporatedResults

			if _, ok := incResults[incorporatedResult.IncorporatedBlockID]; !ok {
				// there are no items for this IncorporatedBlockID
				return nil
			}

			// removes map entry associated with key for update
			delete(backdata, key)
		}

		// remove item from map
		delete(incResults, incorporatedResult.IncorporatedBlockID)

		if len(incResults) > 0 {
			// adds the new incorporated results map associated with key to mempool
			incorporatedResultMap := model.IncorporatedResultMap{
				ExecutionResult:     incorporatedResult.Result,
				IncorporatedResults: incResults,
			}

			backdata[key] = incorporatedResultMap
		}

		removed = true
		ir.size--
		return nil
	})

	return removed
}

// Size returns the number of incorporated results in the mempool.
func (ir *IncorporatedResults) Size() uint {
	return ir.size
}
