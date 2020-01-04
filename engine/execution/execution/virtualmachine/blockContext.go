package virtualmachine

import (
	"errors"
	"fmt"

	"github.com/dapperlabs/flow-go/language/runtime"
	"github.com/dapperlabs/flow-go/model/flow"
)

// A BlockContext is used to execute transactions in the context of a block.
type BlockContext interface {
	// ExecuteTransaction computes the result of a transaction.
	ExecuteTransaction(ledger Ledger, tx *flow.Transaction) (*TransactionResult, error)
}

type blockContext struct {
	vm    *virtualMachine
	block *flow.Block
}

func (bc *blockContext) newTransactionContext(ledger Ledger, tx *flow.Transaction) *transactionContext {
	signingAccounts := make([]runtime.Address, len(tx.ScriptAccounts))
	for i, addr := range tx.ScriptAccounts {
		signingAccounts[i] = runtime.Address(addr)
	}

	return &transactionContext{
		ledger:          ledger,
		signingAccounts: signingAccounts,
	}
}

// ExecuteTransaction computes the result of a transaction.
//
// Register updates are recorded in the provided ledger view. An error is returned
// if an unexpected error occurs during execution. If the transaction reverts due to
// a normal runtime error, the error is recorded in the transaction result.
func (bc *blockContext) ExecuteTransaction(ledger Ledger, tx *flow.Transaction) (*TransactionResult, error) {
	location := runtime.TransactionLocation(tx.Hash())

	ctx := bc.newTransactionContext(ledger, tx)

	err := bc.vm.rt.ExecuteTransaction(tx.Script, ctx, location)
	if err != nil {
		if errors.As(err, &runtime.Error{}) {
			// runtime errors occur when the execution reverts
			return &TransactionResult{
				TxHash: tx.Hash(),
				Error:  err,
			}, nil
		}

		// other errors are unexpected and should be treated as fatal
		return nil, fmt.Errorf("failed to execute transaction: %w", err)
	}

	return &TransactionResult{
		TxHash: tx.Hash(),
		Error:  nil,
	}, nil
}
