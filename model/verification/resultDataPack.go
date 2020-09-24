package verification

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// ResultDataPack represents an execution result with some metadata.
// This is an internal entity for verification node.
type ResultDataPack struct {
	ExecutorID      flow.Identifier
	ExecutionResult *flow.ExecutionResult
}

// ID returns the unique identifier for the ResultDataPack which is the
// id of its execution result.
func (r *ResultDataPack) ID() flow.Identifier {
	return r.ExecutionResult.ID()
}

// Checksum returns the checksum of the ResultDataPack.
func (r *ResultDataPack) Checksum() flow.Identifier {
	return flow.MakeID(r)
}
