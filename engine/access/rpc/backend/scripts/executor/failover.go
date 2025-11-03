package executor

import (
	"context"

	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
)

// FailoverScriptExecutor is a script executor that executes scripts and gets accounts using local storage first,
// then falls back to execution nodes if data is not available for the height or if request
// failed due to a non-user error.
type FailoverScriptExecutor struct {
	localExecutor         ScriptExecutor
	executionNodeExecutor ScriptExecutor
}

var _ ScriptExecutor = (*FailoverScriptExecutor)(nil)

// NewFailoverScriptExecutor creates a new [FailoverScriptExecutor].
func NewFailoverScriptExecutor(localExecutor ScriptExecutor, execNodeExecutor ScriptExecutor) *FailoverScriptExecutor {
	return &FailoverScriptExecutor{
		localExecutor:         localExecutor,
		executionNodeExecutor: execNodeExecutor,
	}
}

// Execute executes the provided script at the requested block.
//
// Expected error returns during normal operation:
//   - [InvalidArgumentError] - if the script execution failed due to invalid arguments or runtime errors.
//   - [ScriptExecutionCanceledError] - if the script execution was canceled.
//   - [DataNotFoundError] - if data not found.
//   - [common.FailedToQueryExternalNodeError] - when the request to execution node failed.
//   - [ServiceUnavailable] - if no nodes are available or a connection to an execution node could not be established.
//   - [InternalError] - for internal failures or index conversion errors.
func (f *FailoverScriptExecutor) Execute(ctx context.Context, request *Request, executionResultInfo *optimistic_sync.ExecutionResultInfo,
) ([]byte, *accessmodel.ExecutorMetadata, error) {
	localResult, localMetadata, localErr := f.localExecutor.Execute(ctx, request, executionResultInfo)
	if localErr == nil || IsInvalidArgumentError(localErr) || IsScriptExecutionCanceledError(localErr) {
		return localResult, localMetadata, localErr
	}

	// Note: scripts that timeout are retried on the execution nodes since ANs may have performance
	// issues for some scripts.
	return f.executionNodeExecutor.Execute(ctx, request, executionResultInfo)
}
