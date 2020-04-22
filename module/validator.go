package module

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// TransactionValidator describes the interface for validating transactions.
type TransactionValidator interface {

	// ValidateTransaction checks the given transaction for validity. If the
	// transaction is valid for inclusion in a collection, returns nil, otherwise
	// returns an error.
	ValidateTransaction(tx *flow.TransactionBody) error
}
