package stdmap

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Results implements the execution results memory pool of the consensus node,
// used to store execution results and to generate block seals.
type Results struct {
	*Backend
}

// NewResults creates a new memory pool for execution results.
func NewResults(limit uint) (*Results, error) {

	// create the results memory pool with the lookup maps
	r := &Results{
		Backend: NewBackend(WithLimit(limit)),
	}

	return r, nil
}

// Add adds an execution result to the mempool.
func (r *Results) Add(result *flow.ExecutionResult) bool {
	added := r.Backend.Add(result)
	return added
}

// Rem will remove a result by ID.
func (r *Results) Rem(resultID flow.Identifier) bool {
	removed := r.Backend.Rem(resultID)
	return removed
}

// ByID will retrieve an approval by ID.
func (r *Results) ByID(resultID flow.Identifier) (*flow.ExecutionResult, bool) {
	entity, exists := r.Backend.ByID(resultID)
	if !exists {
		return nil, false
	}
	result := entity.(*flow.ExecutionResult)
	return result, true
}

// All will return all execution results in the memory pool.
func (r *Results) All() []*flow.ExecutionResult {
	entities := r.Backend.All()
	results := make([]*flow.ExecutionResult, 0, len(entities))
	for _, entity := range entities {
		results = append(results, entity.(*flow.ExecutionResult))
	}
	return results
}
