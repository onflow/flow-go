package access

import (
	"github.com/onflow/flow-go/model/flow"
)

// AccountTransaction represents a transaction entry from the account transaction index.
// It contains the essential fields needed to identify and locate a transaction,
// plus metadata about the account's participation.
type AccountTransaction struct {
	Address          flow.Address    // Account address
	BlockHeight      uint64          // Block height where transaction was included
	TransactionID    flow.Identifier // Transaction identifier
	TransactionIndex uint32          // Index of transaction within the block
	IsAuthorizer     bool            // True if the account was an authorizer for this transaction
}
