package fvm

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path"
	"runtime/debug"
	"strings"
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

type TransactionInvoker struct {
	logger zerolog.Logger
}

func NewTransactionInvoker(logger zerolog.Logger) *TransactionInvoker {
	return &TransactionInvoker{
		logger: logger,
	}
}

func (i *TransactionInvoker) Process(
	vm *VirtualMachine,
	ctx *Context,
	proc *TransactionProcedure,
	sth *state.StateHolder,
	programs *programs.Programs,
) (processErr error) {

	txIDStr := proc.ID.String()
	var span opentracing.Span
	if ctx.Tracer != nil && proc.TraceSpan != nil {
		span = ctx.Tracer.StartSpanFromParent(proc.TraceSpan, trace.FVMExecuteTransaction)
		span.LogFields(
			traceLog.String("transaction_id", txIDStr),
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
				Str("txHash", txIDStr).
				Uint64("blockHeight", blockHeight).
				Msg(msg)

			// drop delta
			childState.View().DropDelta()
			proc.Err = errors.NewFVMInternalErrorf(msg)
			proc.Logs = make([]string, 0)
			proc.Events = make([]flow.Event, 0)
			proc.ServiceEvents = make([]flow.Event, 0)
		}
		if mergeError := parentState.MergeState(childState, sth.EnforceInteractionLimits()); mergeError != nil {
			processErr = fmt.Errorf("transaction invocation failed: %w", mergeError)
		}
		sth.SetActiveState(parentState)
		sth.EnableLimitEnforcement()
	}()

	for numberOfRetries = 0; numberOfRetries < int(ctx.MaxNumOfTxRetries); numberOfRetries++ {
		if retry {
			// rest state
			sth.SetActiveState(parentState)
			childState = sth.NewChild()
			// force cleanup if retries
			programs.ForceCleanup()

			i.logger.Warn().
				Str("txHash", txIDStr).
				Uint64("blockHeight", blockHeight).
				Int("retries_count", numberOfRetries).
				Uint64("ledger_interaction_used", sth.State().InteractionUsed()).
				Msg("retrying transaction execution")

			// reset error part of proc
			// Warning right now the tx requires retry logic doesn't change
			// anything on state but we might want to revert the state changes (or not committing)
			// if we decided to expand it further.
			proc.Err = nil
			proc.Logs = make([]string, 0)
			proc.Events = make([]flow.Event, 0)
			proc.ServiceEvents = make([]flow.Event, 0)

			// reset env
			env = NewTransactionEnvironment(*ctx, vm, sth, programs, proc.Transaction, proc.TxIndex, span)
		}

		location := common.TransactionLocation(proc.ID[:])

		err := recoverLedgerInteractionLimitExceeded(i.logger, func() error {
			return vm.Runtime.ExecuteTransaction(
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
		})

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

	// disable the limit checks on states
	sth.DisableLimitEnforcement()

	// try to deduct fees even if there is an error.
	// disable the limit checks on states
	feesError := i.deductTransactionFees(env, proc)
	if feesError != nil {
		txError = feesError
	}

	sth.EnableLimitEnforcement()

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
		txError = recoverLedgerInteractionLimitExceeded(i.logger, func() error {
			return NewTransactionStorageLimiter().CheckLimits(env, sth.State().UpdatedAddresses())
		})
	}

	// it there was any transaction error clear changes and try to deduct fees again
	if txError != nil {
		sth.DisableLimitEnforcement()
		// drop delta since transaction failed
		childState.View().DropDelta()
		// if tx fails just do clean up
		programs.Cleanup(nil)
		// log transaction as failed
		i.logger.Info().
			Str("txHash", txIDStr).
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
				Str("txHash", txIDStr).
				Uint64("blockHeight", blockHeight).
				Uint64("ledgerInteractionUsed", sth.State().InteractionUsed()).
				Msg("transaction fee deduction executed with error")

			return feesError
		}
	} else {
		// transaction is ok, log as successful
		i.logger.Info().
			Str("txHash", txIDStr).
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

// Recover specific panic coming from ExecuteTransaction and return it as an error.
// This panic is documented in https://www.notion.so/dapperlabs/Testnet-31-Execution-restart-11-15-2021-2446a26f55994f73bf1e4c275d640bd2#47a0ad9c4f4443f39b109ba4c1934e3c.
// An attempt to catch it was done in the following PR: https://github.com/onflow/flow-go/pull/1625
// That PR caught the panic to late, and the state was potentially not roll-backed correctly
// This panic is in an un-unwrappable error so errors.As or errors.Is will not work.
// The proper fix is that this panic does not reach this point at all https://github.com/onflow/cadence/issues/1255.
func recoverLedgerInteractionLimitExceeded(logger zerolog.Logger, f func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			if strings.Contains(fmt.Sprintf("%v", r), "[Error Code: 1106]") {
				logger.Error().
					Interface("error", r).
					Str("trace", string(debug.Stack())). // there was a problem with the stack being cut off
					Msg("VM LedgerIntractionLimitExceeded panic")
				err = errors.NewLedgerIntractionLimitExceededError(state.DefaultMaxInteractionSize, state.DefaultMaxInteractionSize)
				return
			}
			panic(r)
		}
	}()

	return f()
}

func (i *TransactionInvoker) deductTransactionFees(env *TransactionEnv, proc *TransactionProcedure) (err error) {
	if !env.ctx.TransactionFeesEnabled {
		return nil
	}

	// start a new computation meter for deducting transaction fees.
	subMeter := env.computationHandler.StartSubMeter(DefaultGasLimit)
	defer func() {
		merr := subMeter.Discard()
		if merr == nil {
			return
		}
		if err != nil {
			// The error merr (from discarding the subMeter) will be hidden by err (transaction fee deduction error)
			// as it has priority. So log merr.
			i.logger.Error().Err(merr).
				Msg("error discarding computation meter in deductTransactionFees (while also handling a deductTransactionFees error)")
			return
		}
		err = merr
	}()

	deductTxFees := DeductTransactionFeesInvocation(env, proc.TraceSpan)
	err = recoverLedgerInteractionLimitExceeded(i.logger, func() error {
		_, err := deductTxFees(proc.Transaction.Payer)
		return err
	})

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

var setAccountFrozenFunctionType = &sema.FunctionType{
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
}

func valueDeclarations(ctx *Context, env Environment) []runtime.ValueDeclaration {
	var predeclaredValues []runtime.ValueDeclaration

	if ctx.AccountFreezeAvailable {
		// TODO return the errors instead of panicing

		setAccountFrozen := runtime.ValueDeclaration{
			Name:           "setAccountFrozen",
			Type:           setAccountFrozenFunctionType,
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

					var err error
					if env, isTXEnv := env.(*TransactionEnv); isTXEnv {
						err = env.SetAccountFrozen(common.Address(address), bool(frozen))
					} else {
						err = errors.NewOperationNotSupportedError("SetAccountFrozen")
					}
					if err != nil {
						panic(err)
					}

					return interpreter.VoidValue{}
				},
				setAccountFrozenFunctionType,
			),
		}

		predeclaredValues = append(predeclaredValues, setAccountFrozen)
	}
	return predeclaredValues
}

// requiresRetry returns true for transactions that has to be rerun
// this is an additional check which was introduced
func (i *TransactionInvoker) requiresRetry(err error, proc *TransactionProcedure) bool {
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
func (i *TransactionInvoker) dumpRuntimeError(runtimeErr *runtime.Error, procedure *TransactionProcedure) {

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
