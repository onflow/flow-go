package fvm

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"path"
	"time"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/sema"
	"github.com/opentracing/opentracing-go"
	traceLog "github.com/opentracing/opentracing-go/log"
	"github.com/rs/zerolog"

	fvmErrors "github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/extralog"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
)

func Transaction(tx *flow.TransactionBody, txIndex uint32) *TransactionProcedure {
	return &TransactionProcedure{
		ID:          tx.ID(),
		Transaction: tx,
		TxIndex:     txIndex,
	}
}

type TransactionProcessor interface {
	Process(*VirtualMachine, *Context, *TransactionProcedure, *state.StateHolder, *programs.Programs) error
}

type TransactionProcedure struct {
	ID            flow.Identifier
	Transaction   *flow.TransactionBody
	TxIndex       uint32
	Logs          []string
	Events        []flow.Event
	ServiceEvents []flow.Event
	// TODO: report gas consumption: https://github.com/dapperlabs/flow-go/issues/4139
	GasUsed   uint64
	Err       fvmErrors.Error
	Retried   int
	TraceSpan opentracing.Span
}

func (proc *TransactionProcedure) SetTraceSpan(traceSpan opentracing.Span) {
	proc.TraceSpan = traceSpan
}

func (proc *TransactionProcedure) Run(vm *VirtualMachine, ctx Context, st *state.StateHolder, programs *programs.Programs) error {

	for _, p := range ctx.TransactionProcessors {
		err := p.Process(vm, &ctx, proc, st, programs)
		vmErr, fatalErr := handleError(err)
		if fatalErr != nil {
			return fatalErr
		}

		if vmErr != nil {
			proc.Err = vmErr
			// TODO we should not break here we should continue for fee deductions
			break
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
	ctx *Context,
	proc *TransactionProcedure,
	sth *state.StateHolder,
	programs *programs.Programs,
) error {

	var span opentracing.Span

	if ctx.Tracer != nil && proc.TraceSpan != nil {
		span = ctx.Tracer.StartSpanFromParent(proc.TraceSpan, trace.FVMExecuteTransaction)
		span.LogFields(
			traceLog.String("transaction.ID", proc.ID.String()),
		)
		defer span.Finish()
	}

	var err error
	var env *hostEnv
	var blockHeight uint64
	if ctx.BlockHeader != nil {
		blockHeight = ctx.BlockHeader.Height
	}
	var predeclaredValues []runtime.ValueDeclaration

	if ctx.AccountFreezeAvailable {

		setAccountFrozen := runtime.ValueDeclaration{
			Name: "setAccountFrozen",
			Type: &sema.FunctionType{
				Parameters: []*sema.Parameter{
					{
						Label:          sema.ArgumentLabelNotRequired,
						Identifier:     "account",
						TypeAnnotation: sema.NewTypeAnnotation(&sema.AddressType{}),
					},
					{
						Label:          sema.ArgumentLabelNotRequired,
						Identifier:     "frozen",
						TypeAnnotation: sema.NewTypeAnnotation(sema.BoolType),
					},
				},
				ReturnTypeAnnotation: &sema.TypeAnnotation{
					Type: sema.VoidType,
				},
			},
			Kind:           common.DeclarationKindFunction,
			IsConstant:     true,
			ArgumentLabels: nil,
			Value: interpreter.NewHostFunctionValue(
				func(invocation interpreter.Invocation) interpreter.Value {
					address, ok := invocation.Arguments[0].(interpreter.AddressValue)
					if !ok {
						panic(errors.New("first argument must be an address"))
					}

					frozen, ok := invocation.Arguments[1].(interpreter.BoolValue)
					if !ok {
						panic(errors.New("second argument must be a boolean"))
					}

					err := env.SetAccountFrozen(common.Address(address), bool(frozen))
					if err != nil {
						panic(fmt.Errorf("cannot set account frozen: %w", err))
					}

					return interpreter.VoidValue{}
				},
			),
		}

		predeclaredValues = append(predeclaredValues, setAccountFrozen)
	}

	retry := false
	numberOfRetries := 0
	parentState := sth.State()
	childState := sth.NewChild()
	defer func() {
		// an extra check for state holder health, this should never happen
		if childState != sth.State() {
			// error transaction
			msg := "child state doesn't match the active state on the state holder"
			i.logger.Error().
				Str("txHash", proc.ID.String()).
				Uint64("blockHeight", blockHeight).
				Msg(msg)

			// drop delta
			childState.View().DropDelta()
			proc.Err = &fvmErrors.FVMInternalError{msg}
			proc.Logs = make([]string, 0)
			proc.Events = make([]flow.Event, 0)
			proc.ServiceEvents = make([]flow.Event, 0)
		}

		if mergeError := parentState.MergeState(childState); mergeError != nil {
			panic(mergeError)
		}
		sth.SetActiveState(parentState)
	}()

	for numberOfRetries = 0; numberOfRetries < int(ctx.MaxNumOfTxRetries); numberOfRetries++ {
		if retry {
			// rest state
			sth.SetActiveState(parentState)
			childState = sth.NewChild()
			// force cleanup if retries
			programs.ForceCleanup()

			i.logger.Warn().
				Str("txHash", proc.ID.String()).
				Uint64("blockHeight", blockHeight).
				Int("retries_count", numberOfRetries).
				Uint64("ledger_interaction_used", sth.State().InteractionUsed()).
				Msg("retrying transaction execution")

			// reset error part of proc
			// Warning right now the tx requires retry logic doesn't change
			// anything on state but we might want to revert the state changes (or not commiting)
			// if we decided to expand it furthur.
			proc.Err = nil
			proc.Logs = make([]string, 0)
			proc.Events = make([]flow.Event, 0)
			proc.ServiceEvents = make([]flow.Event, 0)
		}
		env, err = newEnvironment(*ctx, vm, sth, programs)
		// env construction error is fatal
		if err != nil {
			return err
		}
		env.setTransaction(proc.Transaction, proc.TxIndex)
		env.setTraceSpan(span)

		location := common.TransactionLocation(proc.ID[:])

		err = vm.Runtime.ExecuteTransaction(
			runtime.Script{
				Source:    proc.Transaction.Script,
				Arguments: proc.Transaction.Arguments,
			},
			runtime.Context{
				Interface:         env,
				Location:          location,
				PredeclaredValues: predeclaredValues,
			},
		)

		// break the loop
		if !i.requiresRetry(err, proc) {
			break
		}

		retry = true
		proc.Retried++
	}

	// (for future) panic if we tried several times and still failing
	// if numberOfTries == maxNumberOfRetries {
	// 	panic(err)
	// }

	var txError error

	// failed transaction path
	if err != nil {
		txError = err
	}

	// applying contract changes
	// this writes back the contract contents to accounts
	// if any error occurs we fail the tx
	updatedKeys, err := env.Commit()
	if err != nil && txError == nil {
		txError = err
	}

	// check the storage limits
	if ctx.LimitAccountStorage && txError == nil {
		txError = NewTransactionStorageLimiter().Process(vm, ctx, proc, sth, programs)
	}

	if txError != nil {
		// drop delta
		childState.View().DropDelta()
		// if tx fails just do clean up
		programs.Cleanup(nil)
		i.logger.Info().
			Str("txHash", proc.ID.String()).
			Uint64("blockHeight", blockHeight).
			Uint64("ledgerInteractionUsed", sth.State().InteractionUsed()).
			Msg("transaction executed with error")
		return txError
	}

	// based on the contract updates we decide how to clean up the programs
	// for failed transactions we also do the same as
	// transaction without any deployed contracts
	programs.Cleanup(updatedKeys)

	proc.Events = env.getEvents()
	proc.ServiceEvents = env.getServiceEvents()
	proc.Logs = env.getLogs()

	i.logger.Info().
		Str("txHash", proc.ID.String()).
		Uint64("blockHeight", blockHeight).
		Uint64("ledgerInteractionUsed", sth.State().InteractionUsed()).
		Int("retried", proc.Retried).
		Msg("transaction executed successfully")

	return nil
}

// requiresRetry returns true for transactions that has to be rerun
// this is an additional check which was introduced
func (i *TransactionInvocator) requiresRetry(err error, proc *TransactionProcedure) bool {
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

	i.dumpRuntimeError(runtimeErr, proc)
	return true
}

// logRuntimeError logs run time errors into a file
// This is a temporary measure.
func (i *TransactionInvocator) dumpRuntimeError(runtimeErr runtime.Error, procedure *TransactionProcedure) {

	codesJSON, err := json.Marshal(runtimeErr.Codes)
	if err != nil {
		i.logger.Error().Err(err).Msg("cannot marshal codes JSON")
	}
	programsJSON, err := json.Marshal(runtimeErr.Programs)
	if err != nil {
		i.logger.Error().Err(err).Msg("cannot marshal programs JSON")
	}

	t := time.Now().UnixNano()

	codesPath := path.Join(extralog.ExtraLogDumpPath, fmt.Sprintf("%s-codes-%d", procedure.ID.String(), t))
	programsPath := path.Join(extralog.ExtraLogDumpPath, fmt.Sprintf("%s-programs-%d", procedure.ID.String(), t))

	err = ioutil.WriteFile(codesPath, codesJSON, 0700)
	if err != nil {
		i.logger.Error().Err(err).Msg("cannot write codes json")
	}

	err = ioutil.WriteFile(programsPath, programsJSON, 0700)
	if err != nil {
		i.logger.Error().Err(err).Msg("cannot write programs json")
	}

	i.logger.Error().
		Str("txHash", procedure.ID.String()).
		Str("codes", string(codesJSON)).
		Str("programs", string(programsJSON)).
		Msg("checking failed")
}
