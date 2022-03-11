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
	// the maximum transaction byte size limit
	MaxTransactionByteSize uint64
	// maximum collection byte size, it acts as hard limit max for the tx size.
	MaxCollectionByteSize uint64
	// maximum number of un-processed transaction messages to hold in the queue.
	MaxMessageQueueSize uint
}

func DefaultConfig() Config {
	return Config{
		ExpiryBuffer:           flow.DefaultTransactionExpiryBuffer,
		MaxGasLimit:            flow.DefaultMaxTransactionGasLimit,
		MaxTransactionByteSize: flow.DefaultMaxTransactionByteSize,
		MaxCollectionByteSize:  flow.DefaultMaxCollectionByteSize,
		CheckScriptsParse:      true,
		MaxAddressIndex:        flow.DefaultMaxAddressIndex,
		PropagationRedundancy:  2,
		MaxMessageQueueSize:    10_000,
	}
}
