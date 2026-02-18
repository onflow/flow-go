package access

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

type TransactionRole int

const (
	// CAUTION: these values are stored in the database. Do not change the values.
	TransactionRoleAuthorizer TransactionRole = 0 // Account is an authorizer for the transaction
	TransactionRolePayer      TransactionRole = 1 // Account is the payer for the transaction
	TransactionRoleProposer   TransactionRole = 2 // Account is the proposer for the transaction
	TransactionRoleInteracted TransactionRole = 3 // Account is referenced by an event in the transaction
)

// String returns the string representation of a [TransactionRole] matching the OpenAPI spec
// enum values: "authorizer", "payer", "proposer", "interacted".
//
// Panics on unknown values since roles are stored in the database and unknown values indicate
// data corruption.
func (r TransactionRole) String() string {
	switch r {
	case TransactionRoleAuthorizer:
		return "authorizer"
	case TransactionRolePayer:
		return "payer"
	case TransactionRoleProposer:
		return "proposer"
	case TransactionRoleInteracted:
		return "interacted"
	default:
		panic(fmt.Sprintf("unknown TransactionRole: %d", int(r)))
	}
}

// ParseTransactionRole parses a string into a [TransactionRole]. Accepted values match the
// OpenAPI spec enum: "authorizer", "payer", "proposer", "interacted".
//
// Returns an error when the input string does not match any known role name.
func ParseTransactionRole(s string) (TransactionRole, error) {
	switch s {
	case TransactionRoleAuthorizer.String():
		return TransactionRoleAuthorizer, nil
	case TransactionRolePayer.String():
		return TransactionRolePayer, nil
	case TransactionRoleProposer.String():
		return TransactionRoleProposer, nil
	case TransactionRoleInteracted.String():
		return TransactionRoleInteracted, nil
	default:
		return 0, fmt.Errorf("unknown transaction role: %q", s)
	}
}

// AccountTransaction represents a transaction entry from the account transaction index.
// It contains the essential fields needed to identify and locate a transaction,
// plus metadata about the account's participation.
type AccountTransaction struct {
	Address          flow.Address          // Account address
	BlockHeight      uint64                // Block height where transaction was included
	BlockTimestamp   uint64                // Block timestamp where transaction was included
	TransactionID    flow.Identifier       // Transaction identifier
	TransactionIndex uint32                // Index of transaction within the block
	Roles            []TransactionRole     // Roles of the account in the transaction
	Transaction      *flow.TransactionBody // Transaction body
	Result           *TransactionResult    // Transaction result
}

// AccountTransactionCursor identifies a position in the account transaction index for
// cursor-based pagination. It corresponds to the last entry returned in a previous page.
type AccountTransactionCursor struct {
	BlockHeight      uint64 // Block height of the last returned entry
	TransactionIndex uint32 // Transaction index within the block of the last returned entry
}

// AccountTransactionsPage represents a single page of account transaction results.
type AccountTransactionsPage struct {
	Transactions []AccountTransaction      // Transactions in this page (descending order by height)
	NextCursor   *AccountTransactionCursor // Cursor to fetch the next page, nil when no more results
}
