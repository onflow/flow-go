package virtualmachine

import (
	"errors"
	"fmt"

	"github.com/dapperlabs/flow-go/language/runtime"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/hash"
)

// A BlockContext is used to execute transactions in the context of a block.
type BlockContext interface {
	// ExecuteTransaction computes the result of a transaction.
	ExecuteTransaction(ledger Ledger, tx *flow.TransactionBody) (*TransactionResult, error)
	// ExecuteScript computes the result of a ready-only script.
	ExecuteScript(ledger Ledger, script []byte) (*ScriptResult, error)
}

// NewBlockContext creates a new block context given a runtime and block.
func NewBlockContext(rt runtime.Runtime, block *flow.Block) BlockContext {
	vm := &virtualMachine{
		rt:    rt,
		block: block,
	}

	return &blockContext{
		vm:    vm,
		block: block,
	}
}

type blockContext struct {
	vm    *virtualMachine
	block *flow.Block
}

func (bc *blockContext) newTransactionContext(ledger Ledger, tx *flow.TransactionBody) *transactionContext {
	signingAccounts := make([]runtime.Address, len(tx.ScriptAccounts))
	for i, addr := range tx.ScriptAccounts {
		signingAccounts[i] = runtime.Address(addr)
	}

	return &transactionContext{
		ledger:          ledger,
		signingAccounts: signingAccounts,
	}
}

func (bc *blockContext) newScriptContext(ledger Ledger) *transactionContext {
	return &transactionContext{
		ledger: ledger,
	}
}

// ExecuteTransaction computes the result of a transaction.
//
// Register updates are recorded in the provided ledger view. An error is returned
// if an unexpected error occurs during execution. If the transaction reverts due to
// a normal runtime error, the error is recorded in the transaction result.
func (bc *blockContext) ExecuteTransaction(ledger Ledger, tx *flow.TransactionBody) (*TransactionResult, error) {
	txID := tx.ID()
	location := runtime.TransactionLocation(txID[:])

	ctx := bc.newTransactionContext(ledger, tx)

	err := bc.vm.executeTransaction(tx.Script, ctx, location)
	if err != nil {
		if errors.As(err, &runtime.Error{}) {
			// runtime errors occur when the execution reverts
			return &TransactionResult{
				TxID:  txID,
				Error: err,
			}, nil
		}

		// other errors are unexpected and should be treated as fatal
		return nil, fmt.Errorf("failed to execute transaction: %w", err)
	}

	return &TransactionResult{
		TxID:  txID,
		Error: nil,
	}, nil
}

func (bc *blockContext) ExecuteScript(ledger Ledger, script []byte) (*ScriptResult, error) {
	scriptHash := hash.DefaultHasher.ComputeHash(script)

	location := runtime.ScriptLocation(scriptHash)

	ctx := bc.newScriptContext(ledger)

	value, err := bc.vm.executeScript(script, ctx, location)
	if err != nil {
		if errors.As(err, &runtime.Error{}) {
			// runtime errors occur when the execution reverts
			return &ScriptResult{
				ScriptID: flow.HashToID(scriptHash),
				Error:    err,
			}, nil
		}

		return nil, fmt.Errorf("failed to execute script: %w", err)
	}

	return &ScriptResult{
		ScriptID: flow.HashToID(scriptHash),
		Value:    value,
	}, nil
}
