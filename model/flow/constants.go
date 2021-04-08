package flow

import (
	"fmt"
	"time"
)

// GenesisTime defines the timestamp of the genesis block.
var GenesisTime = time.Date(2018, time.December, 19, 22, 32, 30, 42, time.UTC)

// DefaultTransactionExpiry is the default expiry for transactions, measured
// in blocks. Equivalent to 10 minutes for a 1-second block time.
const DefaultTransactionExpiry = 10 * 60

// DefaultTransactionExpiryBuffer is the default buffer time between a transaction being ingested by a
// collection node and being included in a collection and block.
const DefaultTransactionExpiryBuffer = 30

// DefaultMaxTransactionGasLimit is the default maximum value for the transaction gas limit.
const DefaultMaxTransactionGasLimit = 9999

// DefaultMaxTransactionByteSize is the default maximum transaction byte size. (~1.5MB)
const DefaultMaxTransactionByteSize = 1_500_000

// DefaultMaxCollectionByteSize is the default maximum value for a collection byte size.
const DefaultMaxCollectionByteSize = 3_000_000 // ~3MB. This is should always be higher than the limit on single tx size.

// DefaultMaxCollectionTotalGas is the default maximum value for total gas allowed to be included in a collection.
const DefaultMaxCollectionTotalGas = 10_000_000 // 10M

// DefaultMaxCollectionSize is the default maximum number of transactions allowed inside a collection.
const DefaultMaxCollectionSize = 100

// DefaultAuctionWindow defines the length of the auction window at the beginning of
// an epoch, during which nodes can bid for seats in the committee. Valid epoch events
// such as setup and commit can only be submitted after this window has passed.
const DefaultAuctionWindow = 50_000

// DefaultGracePeriod defines the minimum number of views before the final view of
// an epoch where we need to have an epoch setup and an epoch commit event. This is
// in order to give all nodes the chance to have the information before entering
// the next epoch.
const DefaultGracePeriod = 25_000

// DefaultValueLogGCFrequency is the default frequency in blocks that we call the
// badger value log GC. Equivalent to 10 mins for a 1 second block time
const DefaultValueLogGCFrequency = 10 * 60

const DomainTagLength = 32

// TransactionDomainTag is the prefix of all signed transaction payloads.
//
// A domain tag is encoded as UTF-8 bytes, right padded to a total length of 32 bytes.
var TransactionDomainTag = paddedDomainTag("FLOW-V0.0-transaction")

// UserDomainTag is the prefix of all signed user space payloads.
//
// A domain tag is encoded as UTF-8 bytes, right padded to a total length of 32 bytes.
var UserDomainTag = paddedDomainTag("FLOW-V0.0-user")

func paddedDomainTag(s string) [DomainTagLength]byte {
	var tag [DomainTagLength]byte

	if len(s) > DomainTagLength {
		panic(fmt.Sprintf("domain tag %s cannot be longer than %d characters", s, DomainTagLength))
	}

	copy(tag[:], s)

	return tag
}
