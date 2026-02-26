package access

import (
	"fmt"
	"strings"

	"github.com/onflow/flow-go/model/flow"
)

// ScheduledTransactionStatus represents the lifecycle state of a scheduled transaction.
type ScheduledTransactionStatus int8

const (
	ScheduledTxStatusScheduled ScheduledTransactionStatus = iota
	ScheduledTxStatusExecuted
	ScheduledTxStatusCancelled
	ScheduledTxStatusFailed
)

var scheduledTransactionStatusStrings = map[ScheduledTransactionStatus]string{
	ScheduledTxStatusScheduled: "scheduled",
	ScheduledTxStatusExecuted:  "executed",
	ScheduledTxStatusCancelled: "cancelled",
	ScheduledTxStatusFailed:    "failed",
}

// String returns the string representation of the status.
func (s ScheduledTransactionStatus) String() string {
	if str, ok := scheduledTransactionStatusStrings[s]; ok {
		return str
	}
	panic(fmt.Sprintf("unknown scheduled transaction status: %d", s))
}

// ParseScheduledTransactionStatus parses a string into a ScheduledTransactionStatus.
//
// Any error indicates the string is not a valid status.
func ParseScheduledTransactionStatus(s string) (ScheduledTransactionStatus, error) {
	switch strings.ToLower(s) {
	case scheduledTransactionStatusStrings[ScheduledTxStatusScheduled]:
		return ScheduledTxStatusScheduled, nil
	case scheduledTransactionStatusStrings[ScheduledTxStatusExecuted]:
		return ScheduledTxStatusExecuted, nil
	case scheduledTransactionStatusStrings[ScheduledTxStatusCancelled]:
		return ScheduledTxStatusCancelled, nil
	case scheduledTransactionStatusStrings[ScheduledTxStatusFailed]:
		return ScheduledTxStatusFailed, nil
	default:
		return 0, fmt.Errorf("unknown scheduled transaction status: %s", s)
	}
}

// ScheduledTransactionPriority represents the execution priority of a scheduled transaction.
type ScheduledTransactionPriority uint8

const (
	ScheduledTxPriorityHigh   ScheduledTransactionPriority = 0
	ScheduledTxPriorityMedium ScheduledTransactionPriority = 1
	ScheduledTxPriorityLow    ScheduledTransactionPriority = 2
)

var scheduledTransactionPriorityStrings = map[ScheduledTransactionPriority]string{
	ScheduledTxPriorityHigh:   "high",
	ScheduledTxPriorityMedium: "medium",
	ScheduledTxPriorityLow:    "low",
}

// String returns the string representation of the priority.
func (p ScheduledTransactionPriority) String() string {
	if str, ok := scheduledTransactionPriorityStrings[p]; ok {
		return str
	}
	panic(fmt.Sprintf("unknown scheduled transaction priority: %d", p))
}

// ParseScheduledTransactionPriority parses a string into a ScheduledTransactionPriority.
//
// Any error indicates the string is not a valid priority.
func ParseScheduledTransactionPriority(s string) (ScheduledTransactionPriority, error) {
	switch strings.ToLower(s) {
	case scheduledTransactionPriorityStrings[ScheduledTxPriorityHigh]:
		return ScheduledTxPriorityHigh, nil
	case scheduledTransactionPriorityStrings[ScheduledTxPriorityMedium]:
		return ScheduledTxPriorityMedium, nil
	case scheduledTransactionPriorityStrings[ScheduledTxPriorityLow]:
		return ScheduledTxPriorityLow, nil
	default:
		return 0, fmt.Errorf("unknown scheduled transaction priority: %s", s)
	}
}

// ScheduledTransaction represents a scheduled transaction as indexed by the access node.
type ScheduledTransaction struct {
	ID              uint64
	Priority        ScheduledTransactionPriority
	Timestamp       uint64 // stored by the contract as a UFix64 with the fractional zeroed out
	ExecutionEffort uint64
	Fees            uint64

	TransactionHandlerOwner          flow.Address
	TransactionHandlerTypeIdentifier string
	TransactionHandlerUUID           uint64
	TransactionHandlerPublicPath     string

	Status ScheduledTransactionStatus

	CreatedTransactionID   flow.Identifier
	ExecutedTransactionID  flow.Identifier
	CancelledTransactionID flow.Identifier

	FeesReturned uint64
	FeesDeducted uint64

	// IsPlaceholder is true if the scheduled transaction was created based on the current chain state,
	// and not based on a protocol event. This happens when the index is bootstrapped after the original
	// transaction where the scheduled transaction was first created.
	// When true, the `CreatedTransactionID`, `TransactionHandlerUUID`, and `TransactionHandlerPublicPath`
	// fields are undefined.
	IsPlaceholder bool

	// Expansion fields populated when expandResults is true. Never persisted.
	Transaction     *flow.TransactionBody `msgpack:"-"` // Transaction body (nil unless expanded)
	Result          *TransactionResult    `msgpack:"-"` // Transaction result (nil unless expanded)
	HandlerContract *Contract             `msgpack:"-"` // Handler contract (nil unless expanded)
}

// ScheduledTransactionCursor identifies a position in the scheduled transaction index for
// cursor-based pagination. It corresponds to the last entry returned in a previous page.
type ScheduledTransactionCursor struct {
	ID uint64 // Scheduled transaction ID of the last returned entry
}

// ScheduledTransactionsPage represents a single page of scheduled transaction results.
type ScheduledTransactionsPage struct {
	Transactions []ScheduledTransaction      // Results in this page (descending order by ID)
	NextCursor   *ScheduledTransactionCursor // Cursor to fetch the next page, nil when no more results
}
