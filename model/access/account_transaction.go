package access

import (
	"github.com/onflow/flow-go/model/flow"
)

type TransactionRole int

const (
	// CAUTION: these values are stored in the database. Do not change the values.
	TransactionRoleAuthorizer  TransactionRole = 0 // Account is an authorizer for the transaction
	TransactionRolePayer       TransactionRole = 1 // Account is the payer for the transaction
	TransactionRoleProposer    TransactionRole = 2 // Account is the proposer for the transaction
	TransactionRoleInteraction TransactionRole = 3 // Account is referenced by an event in the transaction
)

// AccountTransaction represents a transaction entry from the account transaction index.
// It contains the essential fields needed to identify and locate a transaction,
// plus metadata about the account's participation.
type AccountTransaction struct {
	Address          flow.Address      // Account address
	BlockHeight      uint64            // Block height where transaction was included
	TransactionID    flow.Identifier   // Transaction identifier
	TransactionIndex uint32            // Index of transaction within the block
	Roles            []TransactionRole // Roles of the account in the transaction
}
