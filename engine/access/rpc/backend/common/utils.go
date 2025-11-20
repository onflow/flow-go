package common

import (
	"github.com/onflow/flow-go/model/flow"
)

// OrderedExecutors creates an ordered list of executors for the same execution result
// - respondingExecutor is the executor who returned an execution result.
// - executorList is the full list of executors who produced the same execution result.
func OrderedExecutors(respondingExecutor flow.Identifier, executorList flow.IdentifierList) flow.IdentifierList {
	ordered := make(flow.IdentifierList, 0, len(executorList))
	ordered = append(ordered, respondingExecutor)

	for _, nodeID := range executorList {
		if nodeID != respondingExecutor {
			ordered = append(ordered, nodeID)
		}
	}

	return ordered
}
