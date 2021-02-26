package fvm

import (
	"encoding/json"
	"errors"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/sema"
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

type TransactionProcessor interface {
	Process(*VirtualMachine, Context, *TransactionProcedure, *state.State) error
}

type TransactionProcedure struct {
	ID            flow.Identifier
	Transaction   *flow.TransactionBody
	TxIndex       uint32
	Logs          []string
	Events        []flow.Event
	ServiceEvents []flow.Event
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
	env, err := newEnvironment(ctx, vm, st)
	if err != nil {
		return err
	}
	env.setTransaction(proc.Transaction, proc.TxIndex)

	location := common.TransactionLocation(proc.ID[:])

	err = vm.Runtime.ExecuteTransaction(
		runtime.Script{
			Source:    proc.Transaction.Script,
			Arguments: proc.Transaction.Arguments,
		},
		runtime.Context{
			Interface: env,
			Location:  location,
		},
	)

	if err != nil {
		i.safetyErrorCheck(err)
		return err
	}

	i.logger.Info().
		Str("txHash", proc.ID.String()).
		Msgf("(%d) ledger interactions used by transaction", st.InteractionUsed())

	// commit the env if no error
	// this writes back the contract contents to accounts
	// if any error fail as a tx
	err = env.Commit()
	if err != nil {
		return err
	}

	proc.Events = env.getEvents()
	proc.ServiceEvents = env.getServiceEvents()
	proc.Logs = env.getLogs()

	return nil
}

// safetyErrorCheck is an additional check which was introduced
// to help chase erroneous execution results which caused an unexpected network fork.
// Parsing and checking of deployed contracts should normally succeed.
// This is a temporary measure.
func (i *TransactionInvocator) safetyErrorCheck(err error) {

	// Only consider runtime errors,
	// in particular only consider parsing/checking errors

	var runtimeErr runtime.Error
	if !errors.As(err, &runtimeErr) {
		return
	}

	var parsingCheckingError *runtime.ParsingCheckingError
	if !errors.As(err, &parsingCheckingError) {
		return
	}

	// Only consider errors in deployed contracts.

	checkerError, ok := parsingCheckingError.Err.(*sema.CheckerError)
	if !ok {
		return
	}

	var foundImportedProgramError bool

	for _, checkingErr := range checkerError.Errors {
		importedProgramError, ok := checkingErr.(*sema.ImportedProgramError)
		if !ok {
			continue
		}

		_, ok = importedProgramError.Location.(common.AddressLocation)
		if !ok {
			continue
		}

		foundImportedProgramError = true
		break
	}

	if !foundImportedProgramError {
		return
	}

	codesJSON, _ := json.Marshal(runtimeErr.Codes)
	programsJSON, _ := json.Marshal(runtimeErr.Programs)

	i.logger.Error().
		Str("codes", string(codesJSON)).
		Str("programs", string(programsJSON)).
		Msg("checking failed")
}
