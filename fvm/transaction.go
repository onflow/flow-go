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

	var err error
	var env *hostEnv

	// TODO move me outside
	maxNumberOfRetries := 2
	for j := 0; j < maxNumberOfRetries; j++ {
		env, err = newEnvironment(ctx, vm, st)
		// env construction error is fatal
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

		// break the loop
		if !i.requiresRetry(err) {
			break
		}

		// reset error part of proc
		proc.Err = nil
	}

	if err != nil {
		return err
	}

	// collect events and logs
	proc.Events = env.getEvents()
	proc.ServiceEvents = env.getServiceEvents()
	proc.Logs = env.getLogs()

	i.logger.Info().
		Str("txHash", proc.ID.String()).
		Uint64("ledger_interaction_used", st.InteractionUsed()).
		Msg("transaction executed with no error")
	return nil
}

// requiresRetry returns true for transactions that has to be rerun
// this is an additional check which was introduced
// to help chase erroneous execution results which caused an unexpected network fork.
// Parsing and checking of deployed contracts should normally succeed.
// This is a temporary measure.
func (i *TransactionInvocator) requiresRetry(err error) bool {
	// if no error no retry
	if err == nil {
		return false
	}
	// Only consider runtime errors,
	// in particular only consider parsing/checking errors
	var runtimeErr runtime.Error
	if !errors.As(err, &runtimeErr) {
		return false
	}

	var parsingCheckingError *runtime.ParsingCheckingError
	if !errors.As(err, &parsingCheckingError) {
		return false
	}

	// Only consider errors in deployed contracts.
	checkerError, ok := parsingCheckingError.Err.(*sema.CheckerError)
	if !ok {
		return false
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
		return false
	}

	i.dumpRuntimeError(runtimeErr)
	return true
}

// logRuntimeError logs run time errors into a file
// This is a temporary measure.
func (i *TransactionInvocator) dumpRuntimeError(runtimeErr runtime.Error) {
	// TODO add to file
	codesJSON, _ := json.Marshal(runtimeErr.Codes)
	programsJSON, _ := json.Marshal(runtimeErr.Programs)

	i.logger.Error().
		Str("codes", string(codesJSON)).
		Str("programs", string(programsJSON)).
		Msg("checking failed")
}
