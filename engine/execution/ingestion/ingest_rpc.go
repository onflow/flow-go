package ingestion

import (
	"context"

	"github.com/onflow/flow-go/model/flow"
)

// IngestRPC represents the RPC calls that the execution ingest engine exposes to support the Access Node API calls
type IngestRPC interface {

	// ExecuteScriptAtBlockID executes a script at the given Block id
	ExecuteScriptAtBlockID(ctx context.Context, script []byte, arguments [][]byte, blockID flow.Identifier) ([]byte, error)

	// GetAccount returns the Account details at the given Block id
	GetAccount(ctx context.Context, address flow.Address, blockID flow.Identifier) (*flow.Account, error)

	// GetRegisterAtBlockID returns the value of a register at the given Block id (if available)
	GetRegisterAtBlockID(ctx context.Context, owner, controller, key []byte, blockID flow.Identifier) ([]byte, error)
}
