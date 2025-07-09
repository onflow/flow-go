package executor

import (
	"context"
	"crypto/md5"
	"time"

	"github.com/onflow/flow-go/model/flow"
)

type ScriptExecutor interface {
	Execute(ctx context.Context, request *ScriptExecutionRequest) ([]byte, time.Duration, error)
}

// ScriptExecutionRequest encapsulates the data needed to execute a script to make it easier
// to pass around between the various methods involved in script execution
type ScriptExecutionRequest struct {
	blockID            flow.Identifier
	height             uint64
	script             []byte
	arguments          [][]byte
	insecureScriptHash [md5.Size]byte
}

func NewScriptExecutionRequest(
	blockID flow.Identifier,
	height uint64,
	script []byte,
	arguments [][]byte,
) *ScriptExecutionRequest {
	return &ScriptExecutionRequest{
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
