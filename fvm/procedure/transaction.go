package fvm

import (
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

func Transaction(tx *flow.TransactionBody, txIndex uint32) *TransactionProcedure {
	return &TransactionProcedure{
		ID: tx.ID(),
		// TODO RAMTIN optimize to reuse processors if possible (change to service if needed)
		// Processors: []fvm.Processor{
		// 	NewTransactionSignatureVerifier(AccountKeyWeightThreshold),
		// 	NewTransactionSequenceNumberChecker(),
		// 	NewTransactionFeeDeductor(),
		// 	NewTransactionInvocator(logger),
		// },
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
	NewAccounts []flow.Address
	GasUsed     uint64
	Err         Error
}

func (proc *TransactionProcedure) Run(vm VirtualMachine, ctx Context, st *state.State) error {

	env, err := fvm.NewEnvironment(ctx, st)
	if err != nil {
		return err
	}
	env.setTransaction(vm, proc.Transaction, proc.TxIndex)

	for _, p := range proc.Processors {
		err := p.Process(vm, proc, env)
		// TODO RAMTIN - maybe error handling should be on process level
		vmErr, fatalErr := HandleError(err)
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
