package virtualmachine

import (
	"errors"
	"fmt"

	jsoncdc "github.com/onflow/cadence/encoding/json"

	"github.com/onflow/cadence/runtime"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/hash"
)

// A BlockContext is used to execute transactions in the context of a block.
type BlockContext interface {
	// ExecuteTransaction computes the result of a transaction.
	ExecuteTransaction(
		ledger Ledger,
		tx *flow.TransactionBody,
		options ...TransactionContextOption,
	) (*TransactionResult, error)

	// ExecuteScript computes the result of a read-only script.
	ExecuteScript(ledger Ledger, script []byte) (*ScriptResult, error)

	// GetAccount reads an account from this block context.
	GetAccount(ledger Ledger, addr flow.Address) (*flow.Account, error)
}

type Blocks interface {
	// ByHeight returns the block at the given height. It is only available
	// for finalized blocks.
	ByHeight(height uint64) (*flow.Block, error)
}

type blockContext struct {
	vm     *virtualMachine
	header *flow.Header
	blocks Blocks
}

func (bc *blockContext) newTransactionContext(
	ledger Ledger,
	tx *flow.TransactionBody,
	options ...TransactionContextOption,
) *TransactionContext {

	signingAccounts := make([]runtime.Address, len(tx.Authorizers))
	for i, addr := range tx.Authorizers {
		signingAccounts[i] = runtime.Address(addr)
	}

	ctx := &TransactionContext{
		bc:              bc,
		astCache:        bc.vm.cache,
		ledger:          NewLedgerDAL(ledger),
		signingAccounts: signingAccounts,
		tx:              tx,
		header:          bc.header,
		blocks:          bc.blocks,
		gasLimit:        tx.GasLimit,
	}

	for _, option := range options {
		option(ctx)
	}

	return ctx
}

func (bc *blockContext) newScriptContext(ledger Ledger) *TransactionContext {
	return &TransactionContext{
		bc:       bc,
		astCache: bc.vm.cache,
		ledger:   NewLedgerDAL(ledger),
		header:   bc.header,
		blocks:   bc.blocks,
		gasLimit: scriptGasLimit,
	}
}

// ExecuteTransaction computes the result of a transaction.
//
// Register updates are recorded in the provided ledger view. An error is returned
// if an unexpected error occurs during execution. If the transaction reverts due to
// a normal runtime error, the error is recorded in the transaction result.
func (bc *blockContext) ExecuteTransaction(
	ledger Ledger,
	tx *flow.TransactionBody,
	options ...TransactionContextOption,
) (*TransactionResult, error) {

	txID := tx.ID()
	location := runtime.TransactionLocation(txID[:])

	ctx := bc.newTransactionContext(ledger, tx, options...)

	if !ctx.signatureVerificationDisabled {
		flowErr := ctx.verifySignatures()
		if flowErr != nil {
			return &TransactionResult{
				TransactionID: txID,
				Error:         flowErr,
			}, nil
		}

		flowErr, err := ctx.checkAndIncrementSequenceNumber()
		if err != nil {
			return nil, err
		}
		if flowErr != nil {
			return &TransactionResult{
				TransactionID: txID,
				Error:         flowErr,
			}, nil
		}

		flowErr, err = ctx.deductTransactionFee(tx.Payer)
		if err != nil {
			return nil, err
		}

		if flowErr != nil {
			return &TransactionResult{
				TransactionID: txID,
				Error:         flowErr,
			}, nil
		}
	}

	err := bc.vm.executeTransaction(tx.Script, tx.Arguments, ctx, location)
	if err != nil {
		possibleRuntimeError := runtime.Error{}
		if errors.As(err, &possibleRuntimeError) {
			// runtime errors occur when the execution reverts
			return &TransactionResult{
				TransactionID: txID,
				Error: &CodeExecutionError{
					RuntimeError: possibleRuntimeError,
				},
				Logs: ctx.Logs(),
			}, nil
		}

		// other errors are unexpected and should be treated as fatal
		return nil, fmt.Errorf("failed to execute transaction: %w", err)
	}

	return &TransactionResult{
		TransactionID: txID,
		Error:         nil,
		Events:        ctx.Events(),
		Logs:          ctx.Logs(),
		GasUsed:       0, // TODO: record gas usage
	}, nil
}

func (bc *blockContext) ExecuteScript(ledger Ledger, script []byte) (*ScriptResult, error) {
	scriptHash := hash.DefaultHasher.ComputeHash(script)

	location := runtime.ScriptLocation(scriptHash)

	ctx := bc.newScriptContext(ledger)
	value, err := bc.vm.executeScript(script, ctx, location)
	if err != nil {
		possibleRuntimeError := runtime.Error{}
		if errors.As(err, &possibleRuntimeError) {
			// runtime errors occur when the execution reverts
			return &ScriptResult{
				ScriptID: flow.HashToID(scriptHash),
				Error: &CodeExecutionError{
					RuntimeError: possibleRuntimeError,
				},
				Logs: ctx.Logs(),
			}, nil
		}

		return nil, fmt.Errorf("failed to execute script: %w", err)
	}

	return &ScriptResult{
		ScriptID: flow.HashToID(scriptHash),
		Value:    value,
		Logs:     ctx.Logs(),
		Events:   ctx.events,
	}, nil
}

func (bc *blockContext) GetAccount(ledger Ledger, addr flow.Address) (*flow.Account, error) {
	ledgerAccess := NewLedgerDAL(ledger)
	acct := ledgerAccess.GetAccount(addr)
	if acct == nil {
		return nil, nil
	}

	result, err := bc.ExecuteScript(ledger, DefaultTokenBalanceScript(addr))
	if err != nil {
		return nil, err
	}

	if result.Error == nil {
		acct.Balance = result.Value.ToGoValue().(uint64)
	}

	return acct, nil
}

// ConvertEvents creates flow.Events from runtime.events
func ConvertEvents(txIndex uint32, tr *TransactionResult) ([]flow.Event, error) {

	flowEvents := make([]flow.Event, len(tr.Events))

	for i, event := range tr.Events {
		payload, err := jsoncdc.Encode(event)
		if err != nil {
			return nil, fmt.Errorf("failed to encode event: %w", err)
		}

		flowEvents[i] = flow.Event{
			Type:             flow.EventType(event.EventType.ID()),
			TransactionID:    tr.TransactionID,
			TransactionIndex: txIndex,
			EventIndex:       uint32(i),
			Payload:          payload,
		}
	}

	return flowEvents, nil
}
