package fvm

import (
	"fmt"
	"strconv"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/sema"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/attribute"
	otelTrace "go.opentelemetry.io/otel/trace"

	"github.com/onflow/flow-go/fvm/errors"
	programsCache "github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
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
	programs *programsCache.Programs,
) error {
	txIDStr := proc.ID.String()
	var blockHeight uint64
	if ctx.BlockHeader != nil {
		blockHeight = ctx.BlockHeader.Height
	}

	logger := i.logger.With().
		Str("txHash", txIDStr).
		Uint64("blockHeight", blockHeight).
		Logger()

	var span otelTrace.Span
	if ctx.Tracer != nil && proc.TraceSpan != nil {
		span = ctx.Tracer.StartSpanFromParent(proc.TraceSpan, trace.FVMExecuteTransaction)
		span.SetAttributes(
			attribute.String("transaction_id", txIDStr),
		)
		defer span.End()
	}

	parentState := sth.State()
	// always set the state back to the parent state after processing the transaction
	defer sth.SetActiveState(parentState)

	childState := sth.NewChild()

	// ==== Do the actual processing ====
	err := i.process(vm, ctx, proc, sth, programs, span, logger)

	// if the process error is a fatal error, we are about to crash the FVM. No need to do any other checking.
	var fatal errors.Failure
	if errors.As(err, &fatal) {
		return err
	}

	if childState != sth.State() {
		// error transaction
		msg := "child state doesn't match the active state on the state holder"
		logger.Error().
			Msg(msg)

		// This is a failure scenario. This should not happen and indicates a FVM bug.
		return errors.NewFVMInternalFailuref(msg)
	}

	// merge even if there is an error
	mergeError := parentState.MergeState(childState, sth.EnforceInteractionLimits())

	if mergeError != nil {
		// if merge error is fatal just return it
		if errors.As(mergeError, &fatal) {
			return mergeError
		}

		// if there was already an error, it takes precedence
		if err != nil {
			// however still log the merge error
			logger.
				Error().
				Err(mergeError).
				Msg("Hiding merging state error because an error occurred during transaction processing")

			return err
		}
		return fmt.Errorf("transaction invocation failed when merging state: %w", mergeError)
	}

	return err
}

func (i *TransactionInvoker) process(
	vm *VirtualMachine,
	ctx *Context,
	proc *TransactionProcedure,
	sth *state.StateHolder,
	programs *programsCache.Programs,
	span otelTrace.Span,
	logger zerolog.Logger,
) (processErr error) {
	state := sth.State()

	env := NewTransactionEnvironment(*ctx, vm, sth, programs, proc.Transaction, proc.TxIndex, span)

	predeclaredValues := valueDeclarations(env)

	location := common.TransactionLocation(proc.ID)

	var txError error
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
		var interactionLimiExceededErr *errors.LedgerInteractionLimitExceededError
		if errors.As(err, &interactionLimiExceededErr) {
			// If it is this special interaction limit error, just set it directly as the tx error
			txError = err
		} else {
			// Otherwise, do what we use to do
			txError = fmt.Errorf("transaction invocation failed when executing transaction: %w", errors.HandleRuntimeError(err))
		}
	}

	// read computationUsed from the environment. This will be used to charge fees.
	computationUsed := env.ComputationUsed()
	memoryEstimate := env.MemoryEstimate()

	// log te execution intensities here, so tha they do not contain data from storage limit checks and
	// transaction deduction, because the payer is not charged for those.
	i.logExecutionIntensities(sth, logger)

	// disable the limit checks on states
	sth.DisableAllLimitEnforcements()
	// try to deduct fees even if there is an error.
	// disable the limit checks on states
	feesError := i.deductTransactionFees(env, proc, sth, computationUsed)
	if feesError != nil {
		txError = feesError
	}

	sth.EnableAllLimitEnforcements()

	// applying contract changes
	// this writes back the contract contents to accounts
	// if any error occurs we fail the tx
	// this needs to happen before checking limits, so that contract changes are committed to the state
	modifiedSets, err := env.Commit()
	if err != nil && txError == nil {
		txError = fmt.Errorf("transaction invocation failed when committing Environment: %w", err)
	}

	// if there is still no error check if all account storage limits are ok
	if txError == nil {
		// disable the computation/memory limit checks on storage checks,
		// so we don't error from computation/memory limits on this part.
		// We cannot charge the user for this part, since fee deduction already happened.
		sth.DisableAllLimitEnforcements()
		txError = NewTransactionStorageLimiter().CheckLimits(env, sth.UpdatedAddresses())
		sth.EnableAllLimitEnforcements()
	}

	// it there was any transaction error clear changes and try to deduct fees again
	if txError != nil {
		sth.DisableAllLimitEnforcements()
		defer sth.EnableAllLimitEnforcements()

		// drop delta since transaction failed
		state.View().DropDelta()
		// if tx fails just do clean up
		programs.Cleanup(programsCache.ModifiedSets{})
		// log transaction as failed
		logger.Info().
			Msg("transaction executed with error")

		// TODO(patrick): make env reusable on error
		// reset env
		env = NewTransactionEnvironment(*ctx, vm, sth, programs, proc.Transaction, proc.TxIndex, span)

		// try to deduct fees again, to get the fee deduction events
		feesError = i.deductTransactionFees(env, proc, sth, computationUsed)

		modifiedSets, err = env.Commit()
		if err != nil && feesError == nil {
			feesError = fmt.Errorf("transaction invocation failed after deducting fees: %w", err)
		}

		// if fee deduction fails just do clean up and exit
		if feesError != nil {
			// drop delta
			state.View().DropDelta()
			programs.Cleanup(programsCache.ModifiedSets{})
			logger.Info().
				Msg("transaction fee deduction executed with error")

			txError = feesError
		}
	}

	// if tx failed this will only contain fee deduction logs
	proc.Logs = append(proc.Logs, env.Logs()...)
	proc.ComputationUsed = proc.ComputationUsed + computationUsed
	proc.MemoryEstimate = proc.MemoryEstimate + memoryEstimate

	// based on the contract and frozen account updates we decide how to clean
	// up the programs for failed transactions we also do the same as
	// transaction without any deployed contracts
	programs.Cleanup(modifiedSets)

	// if tx failed this will only contain fee deduction events
	proc.Events = append(proc.Events, env.Events()...)
	proc.ServiceEvents = append(proc.ServiceEvents, env.ServiceEvents()...)

	return txError
}

func (i *TransactionInvoker) deductTransactionFees(
	env *TransactionEnv,
	proc *TransactionProcedure,
	sth *state.StateHolder,
	computationUsed uint64) (err error) {
	if !env.ctx.TransactionFeesEnabled {
		return nil
	}

	if computationUsed > uint64(sth.TotalComputationLimit()) {
		computationUsed = uint64(sth.TotalComputationLimit())
	}

	// Hardcoded inclusion effort (of 1.0 UFix). Eventually this will be
	// dynamic.	Execution effort will be connected to computation used.
	inclusionEffort := uint64(100_000_000)
	_, err = InvokeDeductTransactionFeesContract(
		env,
		proc.Transaction.Payer,
		inclusionEffort,
		computationUsed)

	if err != nil {
		return errors.NewTransactionFeeDeductionFailedError(proc.Transaction.Payer, err)
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

func valueDeclarations(env Environment) []runtime.ValueDeclaration {
	// TODO return the errors instead of panicing

	setAccountFrozen := runtime.ValueDeclaration{
		Name:           "setAccountFrozen",
		Type:           setAccountFrozenFunctionType,
		Kind:           common.DeclarationKindFunction,
		IsConstant:     true,
		ArgumentLabels: nil,
		Value: interpreter.NewUnmeteredHostFunctionValue(
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

	return []runtime.ValueDeclaration{setAccountFrozen}
}

// logExecutionIntensities logs execution intensities of the transaction
func (i *TransactionInvoker) logExecutionIntensities(sth *state.StateHolder, logger zerolog.Logger) {
	if logger.Debug().Enabled() {
		computation := zerolog.Dict()
		for s, u := range sth.ComputationIntensities() {
			computation.Uint(strconv.FormatUint(uint64(s), 10), u)
		}
		memory := zerolog.Dict()
		for s, u := range sth.MemoryIntensities() {
			memory.Uint(strconv.FormatUint(uint64(s), 10), u)
		}
		logger.Info().
			Uint64("ledgerInteractionUsed", sth.InteractionUsed()).
			Uint("computationUsed", sth.TotalComputationUsed()).
			Uint64("memoryEstimate", sth.TotalMemoryEstimate()).
			Dict("computationIntensities", computation).
			Dict("memoryIntensities", memory).
			Msg("transaction execution data")
	}
}
