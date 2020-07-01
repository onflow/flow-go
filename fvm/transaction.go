package fvm

import (
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"

	"github.com/dapperlabs/flow-go/model/flow"
)

func Transaction(tx *flow.TransactionBody) *InvokableTransaction {
	return &InvokableTransaction{Transaction: tx}
}

type InvokableTransaction struct {
	Transaction *flow.TransactionBody
	ID          flow.Identifier
	Logs        []string
	Events      []cadence.Event
	Err         Error
}

type TransactionProcessor interface {
	Process(*VirtualMachine, Context, *InvokableTransaction, Ledger) error
}

func (inv *InvokableTransaction) Invoke(vm *VirtualMachine, ctx Context, ledger Ledger) error {
	inv.ID = inv.Transaction.ID()

	for _, p := range ctx.TransactionProcessors {
		err := p.Process(vm, ctx, inv, ledger)
		vmErr, fatalErr := handleError(err)
		if fatalErr != nil {
			return fatalErr
		}

		if vmErr != nil {
			inv.Err = vmErr
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
	inv *InvokableTransaction,
	ledger Ledger,
) error {
	env := newEnvironment(ctx, ledger)
	env.setTransaction(vm, inv.Transaction)

	location := runtime.TransactionLocation(inv.ID[:])

	err := vm.Runtime.ExecuteTransaction(inv.Transaction.Script, inv.Transaction.Arguments, env, location)
	if err != nil {
		return err
	}

	inv.Events = env.getEvents()
	inv.Logs = env.getLogs()

	return nil
}
