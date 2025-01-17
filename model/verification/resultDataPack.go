package verification

import (
	"github.com/onflow/flow-go/model/flow"
)

// ResultDataPack represents an execution result with some metadata.
// This is an internal entity for verification node.
type ResultDataPack struct {
	ExecutorID      flow.Identifier
	ExecutionResult *flow.ExecutionResult
}
