package executor

import (
	"context"
	"crypto/md5" //nolint:gosec

	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
)

// TODO(Uliana): add godoc
type ScriptExecutor interface {
	// TODO(Uliana): add godoc
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
