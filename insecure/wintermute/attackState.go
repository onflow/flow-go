package wintermute

import (
	"github.com/onflow/flow-go/model/flow"
)

// attackState keeps data structures related to a specific wintermute attack instance.
type attackState struct {
	originalResult    *flow.ExecutionResult // original valid execution result
	corruptedResult   *flow.ExecutionResult // corrupted execution result by orchestrator
	originalChunkIds  flow.IdentifierList   // list of chunk ids of original result
	corruptedChunkIds flow.IdentifierFilter // list of chunk ids of corrupted result
}
