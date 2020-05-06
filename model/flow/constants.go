package flow

import (
	"time"
)

const (
	// TestingChainID is the chain ID used strictly for testing.
	TestingChainID = "flow-testing"
	// DefaultChainID is the chain ID for the main consensus node chain.
	DefaultChainID = "flow"
	// DefaultTransactionExpiry is the default expiry for transactions, measured
	// in blocks. Equivalent to 10 minutes for a 1-second block time.
	DefaultTransactionExpiry = 10 * 60
)

func GenesisTime() time.Time {
	return time.Date(2018, time.December, 19, 22, 32, 30, 42, time.UTC)
}
