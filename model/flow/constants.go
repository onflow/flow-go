package flow

import (
	"time"
)

// DefaultTransactionExpiry is the default expiry for transactions, measured
// in blocks. Equivalent to 10 minutes for a 1-second block time.
const DefaultTransactionExpiry = 10 * 60

// DefaultMaxGasLimit is the default maximum value for the transaction gas limit.
const DefaultMaxGasLimit = 9999

func GenesisTime() time.Time {
	return time.Date(2018, time.December, 19, 22, 32, 30, 42, time.UTC)
}

// DefaultValueLogGCFrequency is the default frequency in blocks that we call the
// badger value log GC. Equivalent to 10 mins for a 1 second block time
const DefaultValueLogGCFrequency = 10 * 60
