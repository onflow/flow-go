package access

import (
	"github.com/onflow/flow-go/model/flow"
)

// ExecutorMetadata contains information about the execution result used to handle
// the API query. It records the ID of the execution result and the node IDs of the
// execution nodes that produced it.
type ExecutorMetadata struct {
	ExecutionResultID flow.Identifier
	ExecutorIDs       flow.IdentifierList
}
