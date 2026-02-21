package access

import "github.com/onflow/flow-go/model/flow"

type ScheduledTxStatus int

const (
	ScheduledTxStatusScheduled ScheduledTxStatus = iota
	ScheduledTxStatusExecuted
	ScheduledTxStatusCancelled
	ScheduledTxStatusFailed
)

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

	CreatedTransactionID   flow.Identifier
	ExecutedTransactionID  flow.Identifier
	CancelledTransactionID flow.Identifier

	FeesReturned uint64
	FeesDeducted uint64
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
