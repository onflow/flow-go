package executor

import (
	"context"

	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
)

// TODO(Uliana): add godoc
type ScriptExecutor interface {
	// TODO(Uliana): add godoc
	Execute(ctx context.Context, scriptRequest *Request) ([]byte, *accessmodel.ExecutorMetadata, error)
}

// Request encapsulates the data needed to execute a script to make it easier
// to pass around between the various methods involved in script execution
type Request struct {
	blockID        flow.Identifier
	height         uint64
	script         []byte
	arguments      [][]byte
	execResultInfo *optimistic_sync.ExecutionResultInfo
}

// NewScriptExecutionRequest creates a new Request instance for script execution.
func NewScriptExecutionRequest(
	blockID flow.Identifier,
	height uint64,
	script []byte,
	arguments [][]byte,
	execResultInfo *optimistic_sync.ExecutionResultInfo,
) *Request {
	return &Request{
		blockID:        blockID,
		height:         height,
		script:         script,
		arguments:      arguments,
		execResultInfo: execResultInfo,
	}
}
