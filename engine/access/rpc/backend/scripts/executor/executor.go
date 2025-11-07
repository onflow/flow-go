package executor

import (
	"context"
	"crypto/md5" //nolint:gosec

	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
)

// ScriptExecutor is an interface for executing scripts at a specific block.
// Implementations may run scripts using local storage, execution nodes, or a combination
// of both (local first, then fallback to execution nodes).
type ScriptExecutor interface {
	// Execute executes the provided script at the requested block.
	//
	// Expected error returns during normal operation:
	//   - [access.InvalidRequestError] - if the script execution failed due to invalid arguments or runtime errors.
	//   - [access.ResourceExhausted] - if computation or memory limits were exceeded.
	//   - [access.DataNotFoundError] - if the data was not found.
	//   - [access.OutOfRangeError] - if the requested data is outside the available range.
	//   - [access.PreconditionFailedError] - if the registers storage is still bootstrapping.
	//   - [access.RequestCanceledError] - if the script execution was canceled.
	//   - [access.RequestTimedOutError] - if the script execution timed out.
	//   - [access.ServiceUnavailable] - if no nodes are available or a connection to an execution node could not be established.
	//   - [access.InternalError] - for internal failures or index conversion errors.
	Execute(ctx context.Context, scriptRequest *Request, executionResultInfo *optimistic_sync.ExecutionResultInfo) ([]byte, *accessmodel.ExecutorMetadata, error)
}

// Request encapsulates the data needed to execute a script to make it easier
// to pass around between the various methods involved in script execution
type Request struct {
	blockID            flow.Identifier
	height             uint64
	script             []byte
	arguments          [][]byte
	insecureScriptHash [md5.Size]byte
}

// NewScriptExecutionRequest creates a new Request instance for script execution.
func NewScriptExecutionRequest(
	blockID flow.Identifier,
	height uint64,
	script []byte,
	arguments [][]byte,
) *Request {
	return &Request{
		blockID:   blockID,
		height:    height,
		script:    script,
		arguments: arguments,

		// encode to MD5 as low compute/memory lookup key
		// CAUTION: cryptographically insecure md5 is used here, but only to de-duplicate logs.
		// *DO NOT* use this hash for any protocol-related or cryptographic functions.
		insecureScriptHash: md5.Sum(script), //nolint:gosec
	}
}
