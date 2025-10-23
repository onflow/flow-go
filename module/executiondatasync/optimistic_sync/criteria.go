package optimistic_sync

import (
	"github.com/onflow/flow-go/model/flow"
)

// Criteria defines the filtering criteria for execution result queries.
// It specifies requirements for execution result selection, including the number
// of agreeing executors and requires executor nodes.
type Criteria struct {
	// AgreeingExecutorsCount is the number of receipts including the same ExecutionResult
	AgreeingExecutorsCount uint
	// RequiredExecutors is the list of EN node IDs, one of which must have produced the result
	RequiredExecutors flow.IdentifierList
	// ParentExecutionResultID is the ID of the parent execution result.
	// If set, then the result must be a child result.
	ParentExecutionResultID flow.Identifier
}

// DefaultCriteria is the operator's default criteria for execution result queries.
var DefaultCriteria = Criteria{
	AgreeingExecutorsCount: 2,
}

// OverrideWith overrides the original criteria with the incoming criteria, returning a new Criteria object.
// Fields from `override` criteria take precedence when set.
func (c *Criteria) OverrideWith(other Criteria) Criteria {
	newCriteria := *c

	if other.AgreeingExecutorsCount > 0 {
		newCriteria.AgreeingExecutorsCount = other.AgreeingExecutorsCount
	}

	if len(other.RequiredExecutors) > 0 {
		newCriteria.RequiredExecutors = other.RequiredExecutors
	}

	if other.ParentExecutionResultID != flow.ZeroID {
		newCriteria.ParentExecutionResultID = other.ParentExecutionResultID
	}

	return newCriteria
}
