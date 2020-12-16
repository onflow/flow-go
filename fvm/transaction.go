package fvm

import (
	"fmt"
	"strings"

	"github.com/davecgh/go-spew/spew"
	"github.com/onflow/cadence/runtime"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

const TopShotContractAddress = "0b2a3299cc857e29"

func Transaction(tx *flow.TransactionBody, txIndex uint32) *TransactionProcedure {
	return &TransactionProcedure{
		ID:          tx.ID(),
		Transaction: tx,
		TxIndex:     txIndex,
	}
}

type TransactionProcessor interface {
	Process(*VirtualMachine, Context, *TransactionProcedure, *state.State) error
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

func (proc *TransactionProcedure) Run(vm *VirtualMachine, ctx Context, st *state.State) error {
	for _, p := range ctx.TransactionProcessors {
		err := p.Process(vm, ctx, proc, st)
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
	st *state.State,
) error {
	env, err := newEnvironment(ctx, st)
	if err != nil {
		return err
	}
	env.setTransaction(vm, proc.Transaction, proc.TxIndex)

	location := runtime.TransactionLocation(proc.ID[:])

	err = vm.Runtime.ExecuteTransaction(proc.Transaction.Script, proc.Transaction.Arguments, env, location)

	if err != nil {
		i.topshotSafetyErrorCheck(err)
		return err
	}

	i.logger.Info().Msgf("interaction used by a transaction script (%d)", st.InteractionUsed())

	// commit changes
	err = st.Commit()
	if err != nil {
		return err
	}

	proc.Events = env.getEvents()
	proc.Logs = env.getLogs()

	return nil
}

// topshotSafetyErrorCheck is additional check introduced to help chase erroneous execution results
// which caused unexpected network fork. TopShot is first full-fledged game running on Flow, and
// checking failures in this contract indicate the unexpected computation happening.
// This is a temporary measure.
func (i *TransactionInvocator) topshotSafetyErrorCheck(err error) {
	fmt.Println("ERROR", err)
	e := err.Error()
	if strings.Contains(e, TopShotContractAddress) && strings.Contains(e, "checking") {
		re, isRuntime := err.(runtime.Error)
		if !isRuntime {
			i.logger.Err(err).Msg("found checking error for TopShot contract but exception is not RuntimeError")
			return
		}
		ee, is := re.Err.(*runtime.ParsingCheckingError)
		if !is {
			i.logger.Err(err).Msg("found checking error for TopShot contract but exception is not ExtendedParsingCheckingError")
			return
		}

		// serializing such large and complex objects to JSON
		// causes stack overflow, spew works fine
		spew.Config.DisableMethods = true
		dump := spew.Sdump(ee)

		i.logger.Error().Str("extended_error", dump).Msg("TopShot contract checking failed")
	}
}
