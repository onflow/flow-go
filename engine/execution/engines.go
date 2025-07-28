package execution

import (
	"context"

	"github.com/onflow/flow-go/model/flow"
)

// ScriptExecutor represents the RPC calls that the execution script engine exposes to support the Access Node API calls
type ScriptExecutor interface {

	// ExecuteScriptAtBlockID executes a script at the given UnsignedBlock id
	// it returns the value, the computation used and the error (if any)
	ExecuteScriptAtBlockID(ctx context.Context, script []byte, arguments [][]byte, blockID flow.Identifier) ([]byte, uint64, error)

	// GetAccount returns the Account details at the given UnsignedBlock id
	GetAccount(ctx context.Context, address flow.Address, blockID flow.Identifier) (*flow.Account, error)

	// GetRegisterAtBlockID returns the value of a register at the given UnsignedBlock id (if available)
	GetRegisterAtBlockID(ctx context.Context, owner, key []byte, blockID flow.Identifier) ([]byte, error)
}
