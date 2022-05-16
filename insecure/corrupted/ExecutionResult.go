package corrupted

import (
	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/model/flow"
)

type ExecutionResult struct {
	FlowResult     flow.ExecutionResult
	SpockSnapshots []*delta.SpockSnapshot
}
