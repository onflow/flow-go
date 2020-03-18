package storage

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// TransactionInfos represents persistent storage for TransactionInfos.
type TransactionInfos interface {

	// Store inserts the TransactionInfo, keyed by fingerprint.
	Store(tx *flow.TransactionInfo) error

	// ByID returns the TransactionInfo for the given fingerprint.
	ByID(txID flow.Identifier) (*flow.TransactionInfo, error)
}
