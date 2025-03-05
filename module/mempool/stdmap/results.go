package stdmap

import (
	"github.com/onflow/flow-go/model/flow"
)

// Results implements the execution results memory pool of the consensus node,
// used to store execution results and to generate block seals.
type Results struct {
	*Backend[flow.Identifier, *flow.ExecutionResult]
}

// NewResults creates a new memory pool for execution results.
func NewResults(limit uint) (*Results, error) {

	// create the results memory pool with the lookup maps
	r := &Results{
		Backend: NewBackend(WithLimit[flow.Identifier, *flow.ExecutionResult](limit)),
	}

	return r, nil
}

// Add adds an execution result to the mempool.
func (r *Results) Add(result *flow.ExecutionResult) bool {
	added := r.Backend.Add(result.ID(), result)
	return added
}

// Remove will remove a result by ID.
func (r *Results) Remove(resultID flow.Identifier) bool {
	removed := r.Backend.Remove(resultID)
	return removed
}

// ByID will retrieve an approval by ID.
func (r *Results) ByID(resultID flow.Identifier) (*flow.ExecutionResult, bool) {
	result, exists := r.Backend.Get(resultID)
	if !exists {
		return nil, false
	}
	return result, true
}

// All will return all execution results in the memory pool.
func (r *Results) All() []*flow.ExecutionResult {
	all := r.Backend.All()
	results := make([]*flow.ExecutionResult, 0, len(all))
	for _, result := range all {
		results = append(results, result)
	}
	return results
}
