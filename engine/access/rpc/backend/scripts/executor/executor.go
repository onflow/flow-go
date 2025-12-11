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
	//   - [access.InvalidRequestError]: If the script execution failed due to invalid arguments or runtime errors.
	//   - [access.ResourceExhausted]: If computation or memory limits were exceeded.
	//   - [access.DataNotFoundError]: If the data was not found.
	//   - [access.OutOfRangeError]: If the requested data is outside the available range.
	//   - [access.PreconditionFailedError]: If the registers storage is still bootstrapping.
	//   - [access.RequestCanceledError]: If the script execution was canceled.
	//   - [access.RequestTimedOutError]: If the script execution timed out.
	//   - [access.ServiceUnavailable]: If no nodes are available or a connection to an execution node could not be established.
	//   - [access.InternalError]: For internal failures or index conversion errors.
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
