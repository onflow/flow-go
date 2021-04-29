package fvm

import (
	"fmt"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/fvm/context"
	errors "github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/procedures"
	"github.com/onflow/flow-go/fvm/processors"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/hash"
)

func NewInterpreterRuntime() runtime.Runtime {
	return runtime.NewInterpreterRuntime(
		runtime.WithContractUpdateValidationEnabled(true),
	)
}

// A VirtualMachine augments the Cadence runtime with Flow host functionality.
type FlowVirtualMachine struct {
	runtime runtime.Runtime
}

// NewVirtualMachine creates a new virtual machine instance with the provided runtime.
func NewVirtualMachine(rt runtime.Runtime) *FlowVirtualMachine {
	return &FlowVirtualMachine{
		runtime: rt,
	}
}

func (vm *FlowVirtualMachine) Runtime() runtime.Runtime {
	return vm.runtime
}

// Run runs a procedure against a ledger in the given context.
func (vm *FlowVirtualMachine) Run(ctx context.Context, proc context.Procedure, v state.View, programs *programs.Programs) (err error) {

	st := state.NewState(v,
		state.WithMaxKeySizeAllowed(ctx.MaxStateKeySize),
		state.WithMaxValueSizeAllowed(ctx.MaxStateValueSize),
		state.WithMaxInteractionSizeAllowed(ctx.MaxStateInteractionSize))
	sth := state.NewStateHolder(st)

	defer func() {
		if r := recover(); r != nil {

			// Cadence may fail to encode certain values.
			// Return an error for now, which will cause transactions to revert.
			//
			if encodingErr, ok := r.(interpreter.EncodingUnsupportedValueError); ok {
				err = errors.NewEncodingUnsupportedValueError(encodingErr.Value, encodingErr.Path)
				return
			}

			panic(r)
		}
	}()

	err = proc.Run(vm, ctx, sth, programs)
	if err != nil {
		return err
	}

	return nil
}

// GetAccount returns an account by address or an error if none exists.
func (vm *FlowVirtualMachine) GetAccount(ctx context.Context, address flow.Address, v state.View, programs *programs.Programs) (*flow.Account, error) {
	st := state.NewState(v,
		state.WithMaxKeySizeAllowed(ctx.MaxStateKeySize),
		state.WithMaxValueSizeAllowed(ctx.MaxStateValueSize),
		state.WithMaxInteractionSizeAllowed(ctx.MaxStateInteractionSize))

	sth := state.NewStateHolder(st)
	account, err := procedures.GetAccount(vm, ctx, sth, programs, address)
	if err != nil {
		return nil, fmt.Errorf("cannot get account: %w", err)
	}
	return account, nil
}

// InvokeMetaTransaction invokes a meta transaction inside the context of an outer transaction.
//
// Errors that occur in a meta transaction are propagated as a single error that can be
// captured by the Cadence runtime and eventually disambiguated by the parent context.
func (vm *FlowVirtualMachine) InvokeMetaTransaction(ctx context.Context, tx *flow.TransactionBody, sth *state.StateHolder, programs *programs.Programs) (errors.Error, error) {
	invocator := processors.NewTransactionInvocator(zerolog.Nop())
	err := invocator.Process(vm, &ctx, tx, sth, programs)
	txErr, fatalErr := errors.SplitErrorTypes(err)
	return txErr, fatalErr
}

func Transaction(tx *flow.TransactionBody, txIndex uint32) *procedures.TransactionProcedure {
	return &procedures.TransactionProcedure{
		ID:          tx.ID(),
		Transaction: tx,
		TxIndex:     txIndex,
	}
}

func Script(code []byte) *procedures.ScriptProcedure {
	scriptHash := hash.DefaultHasher.ComputeHash(code)

	return &procedures.ScriptProcedure{
		Script: code,
		ID:     flow.HashToID(scriptHash),
	}
}
