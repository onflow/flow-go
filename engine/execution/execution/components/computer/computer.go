package computer

import (
	"errors"
	"fmt"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/engine/execution/execution/modules/context"
	"github.com/dapperlabs/flow-go/engine/execution/execution/modules/ledger"
	"github.com/dapperlabs/flow-go/language/runtime"
	"github.com/dapperlabs/flow-go/model/flow"
)

// A TransactionResult is the result of executing a transaction.
type TransactionResult struct {
	TxHash  crypto.Hash
	Error   error
	GasUsed uint64
}

// A Computer uses the Cadence runtime to compute transaction results.
type Computer interface {
	// ExecuteTransaction computes the result of a transaction.
	ExecuteTransaction(ledger *ledger.View, tx flow.TransactionBody) (*TransactionResult, error)
}

type computer struct {
	runtime         runtime.Runtime
	contextProvider context.Provider
}

// New initializes a new computer with a runtime and context provider.
func New(runtime runtime.Runtime, contextProvider context.Provider) Computer {
	return &computer{
		runtime:         runtime,
		contextProvider: contextProvider,
	}
}

// ExecuteTransaction computes the result of a transaction.
//
// Register updates are recorded in the provided ledger view. An error is returned
// if an unexpected error occurs during execution. If the transaction reverts due to
// a normal runtime error, the error is recorded in the transaction result.
func (c *computer) ExecuteTransaction(ledger *ledger.View, tx flow.TransactionBody) (*TransactionResult, error) {
	ctx := c.contextProvider.NewTransactionContext(tx, ledger)

	location := runtime.TransactionLocation(tx.Fingerprint())

	err := c.runtime.ExecuteTransaction(tx.Script, ctx, location)
	if err != nil {
		if errors.As(err, &runtime.Error{}) {
			// runtime errors occur when the execution reverts
			return &TransactionResult{
				TxHash: crypto.Hash(tx.Fingerprint()),
				Error:  err,
			}, nil
		}

		// other errors are unexpected and should be treated as fatal
		return nil, fmt.Errorf("failed to execute transaction: %w", err)
	}

	return &TransactionResult{
		TxHash: crypto.Hash(tx.Fingerprint()),
		Error:  nil,
	}, nil
}
