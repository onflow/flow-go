package stdmap

import "github.com/onflow/flow-go/model/flow"

// IncorporatedResults implements the incorporated results memory pool of the
// consensus nodes, used to store results that need to be sealed.
type IncorporatedResults struct {
	*Backend
}

// NewIncorporatedResults creates a mempool for the incorporated results.
func NewIncorporatedResults(limit uint) *IncorporatedResults {
	return &IncorporatedResults{
		Backend: NewBackend(WithLimit(limit)),
	}
}

// Add adds an IncorporatedResult to the mempool.
func (ir *IncorporatedResults) Add(incorporatedResult *flow.IncorporatedResult) bool {
	return ir.Backend.Add(incorporatedResult)
}

// All returns all the items in the mempool.
func (ir *IncorporatedResults) All() []*flow.IncorporatedResult {
	entities := ir.Backend.All()
	res := make([]*flow.IncorporatedResult, 0, len(ir.entities))
	for _, entity := range entities {
		res = append(res, entity.(*flow.IncorporatedResult))
	}
	return res
}

// ByResultID returns all the IncorporatedResults that contain a specific
// ExecutionResult.
// TODO: Implement a more efficient lookup.
func (ir *IncorporatedResults) ByResultID(resultID flow.Identifier) []*flow.IncorporatedResult {
	entities := ir.Backend.All()
	var res []*flow.IncorporatedResult
	for _, entity := range entities {
		item := entity.(*flow.IncorporatedResult)
		if item.Result.ID() == resultID {
			res = append(res, item)
		}
	}
	return res
}

// Rem removes an IncorporatedResult from the mempool.
func (ir *IncorporatedResults) Rem(incorporatedResultID flow.Identifier) bool {
	return ir.Backend.Rem(incorporatedResultID)
}
