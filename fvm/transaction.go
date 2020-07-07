package fvm

import (
	"fmt"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/cadence/runtime"

	"github.com/dapperlabs/flow-go/fvm/state"
	"github.com/dapperlabs/flow-go/model/flow"
)

func Transaction(tx *flow.TransactionBody) *TransactionProcedure {
	return &TransactionProcedure{
		ID:          tx.ID(),
		Transaction: tx,
	}
}

type TransactionProcedure struct {
	ID          flow.Identifier
	Transaction *flow.TransactionBody
	Logs        []string
	Events      []cadence.Event
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

func (proc *TransactionProcedure) ConvertEvents(txIndex uint32) ([]flow.Event, error) {
	flowEvents := make([]flow.Event, len(proc.Events))

	for i, event := range proc.Events {
		payload, err := jsoncdc.Encode(event)
		if err != nil {
			return nil, fmt.Errorf("failed to encode event: %w", err)
		}

		flowEvents[i] = flow.Event{
			Type:             flow.EventType(event.EventType.ID()),
			TransactionID:    proc.ID,
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
	proc *TransactionProcedure,
	ledger state.Ledger,
) error {
	env := newEnvironment(ctx, ledger)
	env.setTransaction(vm, proc.Transaction)

	location := runtime.TransactionLocation(proc.ID[:])

	vm.Runtime.SetOnWriteValue(func(owner runtime.Address, key string, value cadence.Value) {
		if ctx.OnWriteValue != nil {
			ctx.OnWriteValue(owner, key, value)
		}
	})

	err := vm.Runtime.ExecuteTransaction(
		proc.Transaction.Script,
		proc.Transaction.Arguments,
		env,
		location,
	)
	if err != nil {
		return err
	}

	proc.Events = env.getEvents()
	proc.Logs = env.getLogs()

	return nil
}
