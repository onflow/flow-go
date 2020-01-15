package verification

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

type CompleteExecutionResult struct {
	Result      *flow.ExecutionResult
	Block       *flow.Block
	Collections flow.CollectionList
}
