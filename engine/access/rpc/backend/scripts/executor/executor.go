package executor

import (
	"context"
	"time"

	"github.com/onflow/flow-go/model/flow"
)

// TODO(Uliana): add godoc
type ScriptExecutor interface {
	// TODO(Uliana): add godoc
	Execute(ctx context.Context, scriptRequest *Request) ([]byte, time.Duration, error)
}

// Request encapsulates the data needed to execute a script to make it easier
// to pass around between the various methods involved in script execution
type Request struct {
	blockID           flow.Identifier
	height            uint64
	script            []byte
	arguments         [][]byte
	executionResultID flow.Identifier
}

// TODO(Uliana): add godoc
func NewScriptExecutionRequest(
	blockID flow.Identifier,
	height uint64,
	script []byte,
	arguments [][]byte,
	executionResultID flow.Identifier,
) *Request {
	return &Request{
		blockID:           blockID,
		height:            height,
		script:            script,
		arguments:         arguments,
		executionResultID: executionResultID,
	}
}
