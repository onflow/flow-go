package flow

import (
	"fmt"
	"time"
)

// GenesisTime defines the timestamp of the genesis block.
var GenesisTime = time.Date(2018, time.December, 19, 22, 32, 30, 42, time.UTC)

// DefaultProtocolVersion is the default protocol version, indicating none was
// explicitly set during bootstrapping.
const DefaultProtocolVersion uint = 0

// DefaultTransactionExpiry is the default expiry for transactions, measured in blocks.
// The default value is equivalent to 10 minutes for a 1-second block time.
//
// Let E by the transaction expiry. If a transaction T specifies a reference
// block R with height H, then T may be included in any block B where:
// * R<-*B - meaning B has R as an ancestor, and
// * R.height < B.height <= R.height+E
const DefaultTransactionExpiry = 10 * 60

// DefaultTransactionExpiryBuffer is the default buffer time between a transaction being ingested by a
// collection node and being included in a collection and block.
const DefaultTransactionExpiryBuffer = 30

// DefaultMaxTransactionGasLimit is the default maximum value for the transaction gas limit.
const DefaultMaxTransactionGasLimit = 9999

// EstimatedComputationPerMillisecond is the approximate number of computation units that can be performed in a millisecond.
// this was calibrated during the Variable Transaction Fees: Execution Effort FLIP https://github.com/onflow/flow/pull/753
const EstimatedComputationPerMillisecond = 9999.0 / 200.0

// DefaultMaxTransactionByteSize is the default maximum transaction byte size. (~1.5MB)
const DefaultMaxTransactionByteSize = 1_500_000

// DefaultMaxCollectionByteSize is the default maximum value for a collection byte size.
const DefaultMaxCollectionByteSize = 3_000_000 // ~3MB. This is should always be higher than the limit on single tx size.

// DefaultMaxCollectionTotalGas is the default maximum value for total gas allowed to be included in a collection.
const DefaultMaxCollectionTotalGas = 10_000_000 // 10M

// DefaultMaxCollectionSize is the default maximum number of transactions allowed inside a collection.
const DefaultMaxCollectionSize = 100

// DefaultValueLogGCWaitDuration is the default wait duration before we repeatedly call the badger value log GC.
const DefaultValueLogGCWaitDuration time.Duration = 10 * time.Minute

// DefaultRequiredApprovalsForSealConstruction is the default number of approvals required to construct a candidate seal
// for subsequent inclusion in block.
// when set to 1, it requires at least 1 approval to build a seal
// when set to 0, it can build seal without any approval
const DefaultRequiredApprovalsForSealConstruction = uint(1)

// DefaultRequiredApprovalsForSealValidation is the default number of approvals that should be
// present and valid for each chunk. Setting this to 0 will disable counting of chunk approvals
// this can be used temporarily to ease the migration to new chunk based sealing.
// TODO:
//   - This value will result in consensus not depending on verification at all for sealing (no approvals required)
//   - Full protocol should be +2/3 of all currently authorized verifiers.
const DefaultRequiredApprovalsForSealValidation = 0

// DefaultRequiredReceiptsCommittingToExecutionResult is the default number of receipts that should be committing to an execution
// result which is being incorporated into a candidate block.
// ATTENTION:
// For each result in the candidate block, there must be k receipts included in the candidate block, with k strictly larger than 0.
// This value has to be strictly larger than 0, otherwise leader can incorporate a result which was not executed.
const DefaultRequiredReceiptsCommittingToExecutionResult = 1

// DefaultChunkAssignmentAlpha is the default number of verifiers that should be
// assigned to each chunk.
const DefaultChunkAssignmentAlpha = 3

// DefaultEmergencySealingActive is a flag which indicates when emergency sealing is active, this is a temporary measure
// to make fire fighting easier while seal & verification is under development.
const DefaultEmergencySealingActive = false

// threshold for re-requesting approvals: min height difference between the latest finalized block
// and the block incorporating a result
const DefaultApprovalRequestsThreshold = uint64(10)

// DomainTagLength is set to 32 bytes.
//
// Signatures on Flow that needs to be scoped to a certain domain need to
// have the same length in order to avoid tag collision issues, when prefixing the
// message to sign.
const DomainTagLength = 32

const TransactionTagString = "FLOW-V0.0-transaction"

// TransactionDomainTag is the prefix of all signed transaction payloads.
//
// The tag is the string `TransactionTagString` encoded as UTF-8 bytes,
// right padded to a total length of 32 bytes.
var TransactionDomainTag = paddedDomainTag(TransactionTagString)

// paddedDomainTag padds string tags to form the actuatl domain separation tag used for signing
// and verifiying.
//
// A domain tag is encoded as UTF-8 bytes, right padded to a total length of 32 bytes.
func paddedDomainTag(s string) [DomainTagLength]byte {
	var tag [DomainTagLength]byte

	if len(s) > DomainTagLength {
		panic(fmt.Sprintf("domain tag %s cannot be longer than %d characters", s, DomainTagLength))
	}

	copy(tag[:], s)

	return tag
}
