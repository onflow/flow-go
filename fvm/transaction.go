package fvm

import (
	"github.com/onflow/cadence/runtime"

	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

func Transaction(tx *flow.TransactionBody, txIndex uint32) *TransactionProcedure {
	return &TransactionProcedure{
		ID:          tx.ID(),
		Transaction: tx,
		TxIndex:     txIndex,
	}
}

type TransactionProcedure struct {
	ID          flow.Identifier
	Transaction *flow.TransactionBody
	TxIndex     uint32
	Logs        []string
	Events      []flow.Event
	// TODO: report gas consumption: https://github.com/dapperlabs/flow-go/issues/4139
	GasUsed uint64
	Err     Error
}

type TransactionProcessor interface {
	Process(*VirtualMachine, Context, *TransactionProcedure, state.Ledger) error
}

func (proc *TransactionProcedure) Run(vm *VirtualMachine, ctx Context, ledger state.Ledger) error {
	for _, p := range ctx.TransactionProcessors {
		err := p.Process(vm, ctx, proc, ledger)
		vmErr, fatalErr := handleError(err)
		if fatalErr != nil {
			return fatalErr
		}

		if vmErr != nil {
			proc.Err = vmErr
			return nil
		}
	}

	return nil
}

type TransactionInvocator struct{}

func NewTransactionInvocator() *TransactionInvocator {
	return &TransactionInvocator{}
}

func (i *TransactionInvocator) Process(
	vm *VirtualMachine,
	ctx Context,
	proc *TransactionProcedure,
	ledger state.Ledger,
) error {
	env, err := newEnvironment(ctx, ledger)
	if err != nil {
		return err
	}
	env.setTransaction(vm, proc.Transaction)

	location := runtime.TransactionLocation(proc.ID[:])

	err = vm.Runtime.ExecuteTransaction(proc.Transaction.Script, proc.Transaction.Arguments, env, location)
	if err != nil {
		return err
	}

	proc.Events = env.getEvents()
	proc.Logs = env.getLogs()

	return nil
}
