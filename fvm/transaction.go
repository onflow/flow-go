package fvm

import (
	"fmt"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
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
	// TODO: report gas consumption: https://github.com/dapperlabs/flow-go/issues/4139
	GasUsed uint64
	Err     Error
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

func (inv InvokableTransaction) ConvertEvents(txIndex uint32) ([]flow.Event, error) {
	flowEvents := make([]flow.Event, len(inv.Events))

	for i, event := range inv.Events {
		payload, err := jsoncdc.Encode(event)
		if err != nil {
			return nil, fmt.Errorf("failed to encode event: %w", err)
		}

		flowEvents[i] = flow.Event{
			Type:             flow.EventType(event.EventType.ID()),
			TransactionID:    inv.ID,
			TransactionIndex: txIndex,
			EventIndex:       uint32(i),
			Payload:          payload,
		}
	}

	return flowEvents, nil
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
