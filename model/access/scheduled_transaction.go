package access

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

type ScheduledTxStatus int8

const (
	ScheduledTxStatusScheduled ScheduledTxStatus = iota
	ScheduledTxStatusExecuted
	ScheduledTxStatusCancelled
	ScheduledTxStatusFailed
)

func (s ScheduledTxStatus) String() string {
	switch s {
	case ScheduledTxStatusScheduled:
		return "scheduled"
	case ScheduledTxStatusExecuted:
		return "executed"
	case ScheduledTxStatusCancelled:
		return "cancelled"
	case ScheduledTxStatusFailed:
		return "failed"
	default:
		panic(fmt.Sprintf("unknown scheduled transaction status: %d", s))
	}
}

type ScheduledTransaction struct {
	ID              uint64
	Priority        uint8
	Timestamp       uint64 // TODO: this is stored as a UFix64 in the contract, how to convert to a timestamp?
	ExecutionEffort uint64
	Fees            uint64

	TransactionHandlerOwner          flow.Address
	TransactionHandlerTypeIdentifier string
	TransactionHandlerUUID           uint64
	TransactionHandlerPublicPath     string

	Status ScheduledTxStatus

	ScheduledTransactionID flow.Identifier
	ExecutedTransactionID  flow.Identifier
	CancelledTransactionID flow.Identifier
	FailedTransactionID    flow.Identifier

	FeesReturned uint64
	FeesDeducted uint64

	// Expansion fields populated when expandResults is true. Never persisted.
	Transaction     *flow.TransactionBody `msgpack:"-"` // Transaction body (nil unless expanded)
	Result          *TransactionResult    `msgpack:"-"` // Transaction result (nil unless expanded)
	HandlerContract *Contract             `msgpack:"-"` // Handler contract (nil unless expanded)
}

// query by id ([code][id] -> ScheduledTransaction)
// query by transaction id (existing lookups used by api)
// query by user ([code][user][id]) -> ids are increasing. could use one's complement to get reverse order search
// list by scheduled ([code][id], then filter by not executed)
// list by executed ([code][id], then filter by executed)

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
