package ingest

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

type Config struct {
	// how much buffer time there is between a transaction being ingested by a
	// collection node and being included in a collection and block
	ExpiryBuffer uint
	// the maximum transaction gas limit
	MaxGasLimit uint64
	// whether or not we allow transactions that reference a block we have not
	// validated or seen yet
	AllowUnknownReference bool
	// whether or not we check that transaction scripts are parse-able
	CheckScriptsParse bool
	// how many extra nodes in the responsible cluster we propagate transactions to
	// (we always send to at least one)
	PropagationRedundancy uint
}

func DefaultConfig() Config {
	return Config{
		ExpiryBuffer:          30,
		MaxGasLimit:           flow.DefaultMaxGasLimit,
		AllowUnknownReference: false,
		CheckScriptsParse:     true,
		PropagationRedundancy: 2,
	}
}
