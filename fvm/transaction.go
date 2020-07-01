package fvm

import (
	"github.com/onflow/cadence/runtime"

	"github.com/dapperlabs/flow-go/model/flow"
)

func Transaction(tx *flow.TransactionBody) InvokableTransaction {
	return InvokableTransaction{tx: tx}
}

type InvokableTransaction struct {
	tx *flow.TransactionBody
}

func (i InvokableTransaction) Transaction() *flow.TransactionBody {
	return i.tx
}

func (i InvokableTransaction) Parse(vm *VirtualMachine, ctx Context, ledger Ledger) (Invokable, error) {
	panic("implement me")
}

func (i InvokableTransaction) Invoke(vm *VirtualMachine, ctx Context, ledger Ledger) (*InvocationResult, error) {
	if ctx.TransactionSignatureVerifier != nil {
		err := ctx.TransactionSignatureVerifier.Verify(i.tx, ledger)
		if err != nil {
			return createInvocationResult(i.tx.ID(), nil, nil, nil, err)
		}
	}

	if ctx.TransactionSequenceNumberChecker != nil {
		err := ctx.TransactionSequenceNumberChecker.Check(i.tx, ledger)
		if err != nil {
			return createInvocationResult(i.tx.ID(), nil, nil, nil, err)
		}
	}

	if ctx.TransactionFeeDeductor != nil {
		err := ctx.TransactionFeeDeductor.DeductFees(vm, ctx, i.tx, ledger)
		if err != nil {
			return createInvocationResult(i.tx.ID(), nil, nil, nil, err)
		}
	}

	return i.invoke(vm, ctx, ledger)
}

func (i InvokableTransaction) invoke(vm *VirtualMachine, ctx Context, ledger Ledger) (*InvocationResult, error) {
	txID := i.tx.ID()

	env := newEnvironment(vm, ctx, ledger)
	env.setTransaction(i.tx)

	location := runtime.TransactionLocation(txID[:])

	err := vm.runtime.ExecuteTransaction(i.tx.Script, i.tx.Arguments, env, location)

	return createInvocationResult(txID, nil, env.getEvents(), env.getLogs(), err)
}
