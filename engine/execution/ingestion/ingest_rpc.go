package ingestion

import "github.com/dapperlabs/flow-go/model/flow"

// IngestRPC represents the RPC calls that the execution ingest engine exposes to support the Access Node API calls
type IngestRPC interface {

	// ExecuteScriptAtBlockID executes a script at the given Block id
	ExecuteScriptAtBlockID(script []byte, blockID flow.Identifier) ([]byte, error)

	// GetAccount returns the Account details at the given Block id
	GetAccount(address flow.Address, blockID flow.Identifier) (*flow.Account, error)
}
