package flow

import (
	"fmt"
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

const domainTagLength = 32

// TransactionDomainTag is the prefix of all signed transaction payloads.
//
// A domain tag is encoded as UTF-8 bytes, right padded to a total length of 32 bytes.
var TransactionDomainTag = paddedDomainTag("FLOW-V0.0-transaction")

// UserDomainTag is the prefix of all signed user space payloads.
//
// A domain tag is encoded as UTF-8 bytes, right padded to a total length of 32 bytes.
var UserDomainTag = paddedDomainTag("FLOW-V0.0-user")

func paddedDomainTag(s string) [domainTagLength]byte {
	var tag [domainTagLength]byte

	if len(s) > domainTagLength {
		panic(fmt.Sprintf("domain tag %s cannot be longer than %d characters", s, domainTagLength))
	}

	copy(tag[:], s)

	return tag
}
