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
	//   - [version.ErrOutOfRange] - if block height is higher that last handled block height.
	//   - [execution.ErrIncompatibleNodeVersion] - if the block height is not compatible with the node version.
	//   - [storage.ErrNotFound] - if data was not found.
	//   - [indexer.ErrIndexNotInitialized] - if the storage is still bootstrapping.
	//   - [storage.ErrHeightNotIndexed] - if the requested height is below the first indexed height or above the latest indexed height.
	//   - [codes.InvalidArgument] - if the script execution failed due to invalid arguments or runtime errors.
	//   - [codes.Canceled] - if the script execution was canceled.
	//   - [codes.DeadlineExceeded] - if the script execution timed out.
	//   - [codes.ResourceExhausted] - if computation or memory limits were exceeded.
	//   - [codes.FailedPrecondition] - if data for block is not available.
	//   - [codes.OutOfRange] - if data for block height is not available.
	//   - [codes.NotFound] - if data not found.
	//   - [codes.Internal] - for internal failures or index conversion errors.
	//   - [codes.Unavailable] - if no nodes are available or a connection to an execution node could not be established.
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
