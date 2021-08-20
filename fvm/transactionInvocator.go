package fvm

import (
	"encoding/json"
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

	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/extralog"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
)

const (
	flowServiceAccountContract = "FlowServiceAccount"
	deductFeesContractFunction = "deductTransactionFee"
)

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
) (processErr error) {

	var span opentracing.Span
	if ctx.Tracer != nil && proc.TraceSpan != nil {
		span = ctx.Tracer.StartSpanFromParent(proc.TraceSpan, trace.FVMExecuteTransaction)
		span.LogFields(
			traceLog.String("transaction.ID", proc.ID.String()),
		)
		defer span.Finish()
	}

	var blockHeight uint64
	if ctx.BlockHeader != nil {
		blockHeight = ctx.BlockHeader.Height
	}

	var env *TransactionEnv
	var txError error
	retry := false
	numberOfRetries := 0

	parentState := sth.State()
	childState := sth.NewChild()
	env = NewTransactionEnvironment(*ctx, vm, sth, programs, proc.Transaction, proc.TxIndex, span)
	predeclaredValues := valueDeclarations(ctx, env)

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
			proc.Err = errors.NewFVMInternalErrorf(msg)
			proc.Logs = make([]string, 0)
			proc.Events = make([]flow.Event, 0)
			proc.ServiceEvents = make([]flow.Event, 0)
		}
		if mergeError := parentState.MergeState(childState); mergeError != nil {
			processErr = fmt.Errorf("transaction invocation failed: %w", mergeError)
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

			// reset env
			env = NewTransactionEnvironment(*ctx, vm, sth, programs, proc.Transaction, proc.TxIndex, span)
		}

		location := common.TransactionLocation(proc.ID[:])

		err := vm.Runtime.ExecuteTransaction(
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
		if err != nil {
			txError = fmt.Errorf("transaction invocation failed: %w", errors.HandleRuntimeError(err))
		}

		// break the loop
		if !i.requiresRetry(err, proc) {
			break
		}

		retry = true
		proc.Retried++
	}

	// (for future use) panic if we tried several times and still failing because of checking issue
	// if numberOfTries == maxNumberOfRetries {
	// 	panic(err)
	// }

	// try to deduct fees even if there is an error.
	feesError := i.deductTransactionFees(env, proc)
	if feesError != nil {
		txError = feesError
	}

	// applying contract changes
	// this writes back the contract contents to accounts
	// if any error occurs we fail the tx
	// this needs to happen before checking limits, so that contract changes are committed to the state
	updatedKeys, err := env.Commit()
	if err != nil && txError == nil {
		txError = fmt.Errorf("transaction invocation failed: %w", err)
	}

	// if there is still no error check if all account storage limits are ok
	if txError == nil {
		txError = NewTransactionStorageLimiter().CheckLimits(env, sth.State().UpdatedAddresses())
	}

	// it there was any transaction error clear changes and try to deduct fees again
	if txError != nil {
		// drop delta since transaction failed
		childState.View().DropDelta()
		// if tx fails just do clean up
		programs.Cleanup(nil)
		// log transaction as failed
		i.logger.Info().
			Str("txHash", proc.ID.String()).
			Uint64("blockHeight", blockHeight).
			Uint64("ledgerInteractionUsed", sth.State().InteractionUsed()).
			Msg("transaction executed with error")

		// reset env
		env = NewTransactionEnvironment(*ctx, vm, sth, programs, proc.Transaction, proc.TxIndex, span)

		// try to deduct fees again, to get the fee deduction events
		feesError = i.deductTransactionFees(env, proc)

		updatedKeys, err = env.Commit()
		if err != nil && feesError == nil {
			feesError = fmt.Errorf("transaction invocation failed: %w", err)
		}

		// if fee deduction fails just do clean up and exit
		if feesError != nil {
			// drop delta
			childState.View().DropDelta()
			programs.Cleanup(nil)
			i.logger.Info().
				Str("txHash", proc.ID.String()).
				Uint64("blockHeight", blockHeight).
				Uint64("ledgerInteractionUsed", sth.State().InteractionUsed()).
				Msg("transaction fee deduction executed with error")

			return feesError
		}
	} else {
		// transaction is ok, log as successful
		i.logger.Info().
			Str("txHash", proc.ID.String()).
			Uint64("blockHeight", blockHeight).
			Uint64("ledgerInteractionUsed", sth.State().InteractionUsed()).
			Int("retried", proc.Retried).
			Msg("transaction executed successfully")
	}

	// if tx failed this will only contain fee deduction logs and computation
	proc.Logs = append(proc.Logs, env.Logs()...)
	proc.ComputationUsed = proc.ComputationUsed + env.GetComputationUsed()

	// based on the contract updates we decide how to clean up the programs
	// for failed transactions we also do the same as
	// transaction without any deployed contracts
	programs.Cleanup(updatedKeys)

	// if tx failed this will only contain fee deduction events
	proc.Events = append(proc.Events, env.Events()...)
	proc.ServiceEvents = append(proc.ServiceEvents, env.ServiceEvents()...)

	return txError
}

func (i *TransactionInvocator) deductTransactionFees(env *TransactionEnv, proc *TransactionProcedure) error {
	if !env.ctx.TransactionFeesEnabled {
		return nil
	}

	invocator := NewTransactionContractFunctionInvocator(
		common.AddressLocation{
			Address: common.BytesToAddress(env.ctx.Chain.ServiceAddress().Bytes()),
			Name:    flowServiceAccountContract,
		},
		deductFeesContractFunction,
		[]interpreter.Value{
			interpreter.NewAddressValue(common.BytesToAddress(proc.Transaction.Payer.Bytes())),
		},
		[]sema.Type{
			sema.AuthAccountType,
		},
		env.ctx.Logger,
	)
	_, err := invocator.Invoke(env, proc.TraceSpan)

	if err != nil {
		// TODO: Fee value is currently a constant. this should be changed when it is not
		fees, ok := DefaultTransactionFees.ToGoValue().(uint64)
		if !ok {
			err = fmt.Errorf("could not get transaction fees during formatting of TransactionFeeDeductionFailedError: %w", err)
		}

		return errors.NewTransactionFeeDeductionFailedError(proc.Transaction.Payer, fees, err)
	}
	return nil
}

func valueDeclarations(ctx *Context, env *TransactionEnv) []runtime.ValueDeclaration {
	var predeclaredValues []runtime.ValueDeclaration

	if ctx.AccountFreezeAvailable {
		// TODO return the errors instead of panicing
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
						panic(errors.NewValueErrorf(invocation.Arguments[0].String(),
							"first argument of setAccountFrozen must be an address"))
					}

					frozen, ok := invocation.Arguments[1].(interpreter.BoolValue)
					if !ok {
						panic(errors.NewValueErrorf(invocation.Arguments[0].String(),
							"second argument of setAccountFrozen must be a boolean"))
					}
					err := env.SetAccountFrozen(common.Address(address), bool(frozen))
					if err != nil {
						panic(err)
					}

					return interpreter.VoidValue{}
				},
			),
		}

		predeclaredValues = append(predeclaredValues, setAccountFrozen)
	}
	return predeclaredValues
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

	i.dumpRuntimeError(&runtimeErr, proc)
	return true
}

// logRuntimeError logs run time errors into a file
// This is a temporary measure.
func (i *TransactionInvocator) dumpRuntimeError(runtimeErr *runtime.Error, procedure *TransactionProcedure) {

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
