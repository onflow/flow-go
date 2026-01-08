package optimistic_sync

import (
	"github.com/onflow/flow-go/model/flow"
)

// Criteria defines the filtering criteria for execution result queries.
// It specifies requirements for execution result selection, including the number
// of agreeing executors and requires executor nodes.
// TODO: encapsulate all the fields and use constructor!
type Criteria struct {
	// AgreeingExecutorsCount is the number of receipts including the same ExecutionResult
	AgreeingExecutorsCount uint

	// RequiredExecutors is the list of EN node IDs, one of which must have produced the result
	RequiredExecutors flow.IdentifierList

	// ParentExecutionResultID is the ID of the parent execution result.
	// If set, the result's PreviousResultID field must exactly match.
	ParentExecutionResultID flow.Identifier
}

// DefaultCriteria is the operator's default criteria for execution result queries.
var DefaultCriteria = Criteria{
	AgreeingExecutorsCount: 2,
}

// OverrideWith overrides the original criteria with the incoming criteria, returning a new Criteria object.
// Fields from `override` criteria take precedence when set.
func (c *Criteria) OverrideWith(override Criteria) Criteria {
	newCriteria := *c

	if override.AgreeingExecutorsCount > 0 {
		newCriteria.AgreeingExecutorsCount = override.AgreeingExecutorsCount
	}

	if len(override.RequiredExecutors) > 0 {
		newCriteria.RequiredExecutors = override.RequiredExecutors
	}

	if override.ParentExecutionResultID != flow.ZeroID {
		newCriteria.ParentExecutionResultID = override.ParentExecutionResultID
	}

	return newCriteria
}

// Validate verifies that the criteria can be satisfied by the currently available execution nodes.
//
// The validation ensures that the requested AgreeingExecutorsCount is feasible
// and that every required executor ID is present in the available set.
//
// Expected errors during normal operations:
//   - [optimistic_sync.AgreeingExecutorsCountExceededError]: Agreeing executors count exceeds available executors.
//   - [optimistic_sync.UnknownRequiredExecutorError]: A required executor ID is not in the available set.
func (c *Criteria) Validate(availableExecutors flow.IdentityList) error {
	if uint(len(availableExecutors)) < c.AgreeingExecutorsCount {
		return NewAgreeingExecutorsCountExceededError(c.AgreeingExecutorsCount, len(availableExecutors))
	}

	executors := availableExecutors.Lookup()
	for _, executorID := range c.RequiredExecutors {
		if _, ok := executors[executorID]; !ok {
			return NewUnknownRequiredExecutorError(executorID)
		}
	}

	return nil
}
