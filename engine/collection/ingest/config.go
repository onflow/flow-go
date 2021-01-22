package ingest

import (
	"github.com/onflow/flow-go/model/flow"
)

// Config defines configuration for the transaction ingest engine.
type Config struct {
	// how much buffer time there is between a transaction being ingested by a
	// collection node and being included in a collection and block
	ExpiryBuffer uint
	// the maximum transaction gas limit
	MaxGasLimit uint64
	// whether or not we check that transaction scripts are parse-able
	CheckScriptsParse bool
	// the maximum address index we accept
	MaxAddressIndex uint64
	// how many extra nodes in the responsible cluster we propagate transactions to
	// (we always send to at least one)
	PropagationRedundancy uint
}

func DefaultConfig() Config {
	return Config{
		ExpiryBuffer:          flow.DefaultTransactionExpiryBuffer,
		MaxGasLimit:           flow.DefaultMaxGasLimit,
		CheckScriptsParse:     true,
		MaxAddressIndex:       1_000_000,
		PropagationRedundancy: 2,
	}
}
