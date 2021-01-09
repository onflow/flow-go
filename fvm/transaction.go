package fvm

import (
	"errors"

	"github.com/davecgh/go-spew/spew"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/rs/zerolog"

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

type TransactionInvocator struct {
	logger zerolog.Logger
}

func NewTransactionInvocator(logger zerolog.Logger) *TransactionInvocator {
	return &TransactionInvocator{
		logger: logger,
	}
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
	env.setTransaction(vm, proc.Transaction, proc.TxIndex)

	location := common.TransactionLocation(proc.ID[:])

	err = vm.Runtime.ExecuteTransaction(
		runtime.Script{
			Source:    proc.Transaction.Script,
			Arguments: proc.Transaction.Arguments,
		},
		runtime.Context{
			Interface: env,
			Location: location,
		},
	)

	if err != nil {
		i.safetyErrorCheck(err)
		return err
	}

	proc.Events = env.getEvents()
	proc.Logs = env.getLogs()

	return nil
}

// safetyErrorCheck is an additional check which was introduced
// to help chase erroneous execution results which caused an unexpected network fork.
// Parsing and checking of deployed contracts should normally succeed.
// This is a temporary measure.
func (i *TransactionInvocator) safetyErrorCheck(err error) {
	var parsingCheckingError *runtime.ParsingCheckingError
	if !errors.As(err, &parsingCheckingError) {
		return
	}

	// serializing such large and complex objects to JSON
	// causes stack overflow, spew works fine
	spew.Config.DisableMethods = true
	dump := spew.Sdump(parsingCheckingError)

	i.logger.Error().Str("extended_error", dump).Msg("checking failed")
}
