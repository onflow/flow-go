package fvm

import (
	"github.com/onflow/cadence/runtime"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

type VirtualMachine interface {
	RunProcedure(Context, Procedure, state.Ledger) error
}

// A Procedure is an operation (or set of operations) that reads or writes ledger state. It consists of
// one or several steps run by processors
type Procedure interface {
	Run(vm VirtualMachine, ctx Context, st *state.State) error
}

type TransactionProcedure interface {
	ID() flow.Identifier
	Transaction() *flow.TransactionBody
	Procedure
}

type AccountProcedure interface {
	Address() flow.Address
	Chain() flow.Chain
	Procedure
}

// Processor handles a type of procedure step
type Processor interface {
	Process(VirtualMachine, Procedure, *hostEnv) error
}

// A FlowVirtualMachine augments the Cadence runtime with Flow host functionality.
type FlowVirtualMachine struct {
	Runtime runtime.Runtime
}

// NewVirtualMachine creates a new virtual machine instance with the provided runtime.
func NewVirtualMachine(rt runtime.Runtime) VirtualMachine {
	return &FlowVirtualMachine{
		Runtime: rt,
	}
}

// RunProcedure runs a procedure against a ledger in the given context.
func (vm *FlowVirtualMachine) RunProcedure(ctx Context, proc Procedure, ledger state.Ledger) error {
	// TODO RAMTIN use the ctx and proc to create a env
	st := state.NewState(ledger,
		state.WithMaxKeySizeAllowed(ctx.MaxStateKeySize),
		state.WithMaxValueSizeAllowed(ctx.MaxStateValueSize),
		state.WithMaxInteractionSizeAllowed(ctx.MaxStateInteractionSize))

	return proc.Run(vm, ctx, st)
}

// // GetAccount returns an account by address or an error if none exists.
// func (vm *FlowVirtualMachine) GetAccount(ctx Context, address flow.Address, ledger state.Ledger) (*flow.Account, error) {
// 	st := state.NewState(ledger,
// 		state.WithMaxKeySizeAllowed(ctx.MaxStateKeySize),
// 		state.WithMaxValueSizeAllowed(ctx.MaxStateValueSize),
// 		state.WithMaxInteractionSizeAllowed(ctx.MaxStateInteractionSize))

// 	account, err := getAccount(vm, ctx, st, ctx.Chain, address)
// 	if err != nil {
// 		// TODO: wrap error
// 		return nil, err
// 	}

// 	return account, nil
// }

// // TODO RAMTIN, return both VM error, Tx Error, panic on vm errors, error on tx errors

// invokeMetaTransaction invokes a meta transaction inside the context of an outer transaction.

// Errors that occur in a meta transaction are propagated as a single error that can be
// captured by the Cadence runtime and eventually disambiguated by the parent context.
func (vm *FlowVirtualMachine) invokeMetaTransaction(tx TransactionProcedure, env *hostEnv) error {
	invocator := NewTransactionInvocator(zerolog.Nop())
	err := invocator.Process(vm, ctx, tx, st)
	if err != nil {
		// TODO Panic here
		return err
	}

	if tx.Err != nil {
		return tx.Err
	}

	return nil
}
