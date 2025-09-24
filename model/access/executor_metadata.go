package access

import (
	"github.com/onflow/flow-go/model/flow"
)

// ExecutorMetadata contains metadata about an execution result produced by
// execution nodes. It records the ID of the execution result and the
// identifiers of the executors that produced it.
type ExecutorMetadata struct {
	ExecutionResultID flow.Identifier
	ExecutorIDs       flow.IdentifierList
}

// IsEmpty reports whether the metadata has no information set.
// It returns true when ExecutionResultID equals flow.ZeroID and ExecutorIDs are empty.
func (m ExecutorMetadata) IsEmpty() bool {
	if m.ExecutionResultID == flow.ZeroID && len(m.ExecutorIDs) == 0 {
		return true
	}

	return false
}
