package flow

import (
	"bytes"
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

const domainTagLength = 40
const domainTagPad = "\x1F" // https://en.wikipedia.org/wiki/C0_and_C1_control_codes#Field_separators

// TransactionDomainTag is the prefix of all signed transaction payloads.
//
// A domain tag is encoded as UTF-8 bytes, right padded to a total length of 40 bytes.
var TransactionDomainTag = domainTag("FLOW-V0.0-transaction")

// UserDomainTag is the prefix of all signed user space payloads.
//
// A domain tag is encoded as UTF-8 bytes, right padded to a total length of 40 bytes.
var UserDomainTag = domainTag("FLOW-V0.0-user-domain")

func domainTag(s string) []byte {
	tag := []byte(s)
	tag = append(tag, bytes.Repeat([]byte(domainTagPad), domainTagLength-len(tag))...)

	if len(tag) != domainTagLength {
		panic("domain tag length is invalid")
	}

	return tag
}
