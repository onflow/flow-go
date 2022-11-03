package fvm

import (
	"fmt"
	"strconv"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/attribute"
	otelTrace "go.opentelemetry.io/otel/trace"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/errors"
	programsCache "github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/module/trace"
)

type TransactionInvoker struct {
}

func NewTransactionInvoker() TransactionInvoker {
	return TransactionInvoker{}
}

func (i TransactionInvoker) Process(
	ctx Context,
	proc *TransactionProcedure,
	txnState *state.TransactionState,
	programs *programsCache.TransactionPrograms,
) error {
	executor := newInvocationExecutor(ctx, proc, txnState, programs)
	defer executor.Cleanup()
	return executor.Process()
}

type invocationExecutor struct {
	proc     *TransactionProcedure
	txnState *state.TransactionState
	programs *programsCache.TransactionPrograms

	span otelTrace.Span
	env  environment.Environment

	errs *errors.ErrorsCollector

	nestedTxnId state.NestedTransactionId
}

func newInvocationExecutor(
	ctx Context,
	proc *TransactionProcedure,
	txnState *state.TransactionState,
	programs *programsCache.TransactionPrograms,
) *invocationExecutor {
	span := proc.StartSpanFromProcTraceSpan(
		ctx.Tracer,
		trace.FVMExecuteTransaction)
	span.SetAttributes(attribute.String("transaction_id", proc.ID.String()))

	env := NewTransactionEnvironment(
		ctx,
		txnState,
		programs,
		proc.Transaction,
		proc.TxIndex,
		span)

	return &invocationExecutor{
		proc:     proc,
		txnState: txnState,
		programs: programs,
		span:     span,
		env:      env,
		errs:     errors.NewErrorsCollector(),
	}
}

func (executor *invocationExecutor) Cleanup() {
	executor.span.End()
}

func (executor *invocationExecutor) Process() error {
	var beginErr error
	executor.nestedTxnId, beginErr = executor.txnState.BeginNestedTransaction()
	if beginErr != nil {
		return beginErr
	}

	var modifiedSets programsCache.ModifiedSetsInvalidator
	var txError error
	modifiedSets, txError = executor.normalExecution()
	if executor.errs.Collect(txError).CollectedFailure() {
		return executor.errs.ErrorOrNil()
	}

	if executor.errs.CollectedError() {
		modifiedSets = programsCache.ModifiedSetsInvalidator{}
		executor.errorExecution()
		if executor.errs.CollectedFailure() {
			return executor.errs.ErrorOrNil()
		}
	}

	executor.errs.Collect(executor.commit(modifiedSets))

	return executor.errs.ErrorOrNil()
}

func (executor *invocationExecutor) deductTransactionFees() (err error) {
	if !executor.env.TransactionFeesEnabled() {
		return nil
	}

	computationUsed := executor.env.ComputationUsed()
	if computationUsed > uint64(executor.txnState.TotalComputationLimit()) {
		computationUsed = uint64(executor.txnState.TotalComputationLimit())
	}

	_, err = executor.env.DeductTransactionFees(
		executor.proc.Transaction.Payer,
		executor.proc.Transaction.InclusionEffort(),
		computationUsed)

	if err != nil {
		return errors.NewTransactionFeeDeductionFailedError(
			executor.proc.Transaction.Payer,
			computationUsed,
			err)
	}
	return nil
}

// logExecutionIntensities logs execution intensities of the transaction
func (executor *invocationExecutor) logExecutionIntensities() {
	if !executor.env.Logger().Debug().Enabled() {
		return
	}

	computation := zerolog.Dict()
	for s, u := range executor.txnState.ComputationIntensities() {
		computation.Uint(strconv.FormatUint(uint64(s), 10), u)
	}
	memory := zerolog.Dict()
	for s, u := range executor.txnState.MemoryIntensities() {
		memory.Uint(strconv.FormatUint(uint64(s), 10), u)
	}
	executor.env.Logger().Info().
		Uint64("ledgerInteractionUsed", executor.txnState.InteractionUsed()).
		Uint64("computationUsed", executor.txnState.TotalComputationUsed()).
		Uint64("memoryEstimate", executor.txnState.TotalMemoryEstimate()).
		Dict("computationIntensities", computation).
		Dict("memoryIntensities", memory).
		Msg("transaction execution data")
}

func (executor *invocationExecutor) normalExecution() (
	modifiedSets programsCache.ModifiedSetsInvalidator,
	err error,
) {
	rt := executor.env.BorrowCadenceRuntime()
	defer executor.env.ReturnCadenceRuntime(rt)

	err = rt.ExecuteTransaction(
		runtime.Script{
			Source:    executor.proc.Transaction.Script,
			Arguments: executor.proc.Transaction.Arguments,
		},
		common.TransactionLocation(executor.proc.ID))

	if err != nil {
		err = fmt.Errorf(
			"transaction invocation failed when executing transaction: %w",
			err)
		return
	}

	// Before checking storage limits, we must applying all pending changes
	// that may modify storage usage.
	modifiedSets, err = executor.env.FlushPendingUpdates()
	if err != nil {
		err = fmt.Errorf(
			"transaction invocation failed to flush pending changes from "+
				"environment: %w",
			err)
		return
	}

	// log the execution intensities here, so that they do not contain data
	// from storage limit checks and transaction deduction, because the payer
	// is not charged for those.
	executor.logExecutionIntensities()

	executor.txnState.RunWithAllLimitsDisabled(func() {
		err = executor.deductTransactionFees()
	})
	if err != nil {
		return
	}

	// Check if all account storage limits are ok
	//
	// disable the computation/memory limit checks on storage checks,
	// so we don't error from computation/memory limits on this part.
	// We cannot charge the user for this part, since fee deduction already happened.
	executor.txnState.RunWithAllLimitsDisabled(func() {
		err = NewTransactionStorageLimiter().CheckLimits(
			executor.env,
			executor.txnState.UpdatedAddresses())
	})

	return
}

// Clear changes and try to deduct fees again.
func (executor *invocationExecutor) errorExecution() {
	executor.txnState.DisableAllLimitEnforcements()
	defer executor.txnState.EnableAllLimitEnforcements()

	// log transaction as failed
	executor.env.Logger().Info().
		Err(executor.errs.ErrorOrNil()).
		Msg("transaction executed with error")

	executor.env.Reset()

	// drop delta since transaction failed
	restartErr := executor.txnState.RestartNestedTransaction(executor.nestedTxnId)
	if executor.errs.Collect(restartErr).CollectedFailure() {
		return
	}

	// try to deduct fees again, to get the fee deduction events
	feesError := executor.deductTransactionFees()

	// if fee deduction fails just do clean up and exit
	if feesError != nil {
		executor.env.Logger().Info().
			Err(feesError).
			Msg("transaction fee deduction executed with error")

		if executor.errs.Collect(feesError).CollectedFailure() {
			return
		}

		// drop delta
		executor.errs.Collect(
			executor.txnState.RestartNestedTransaction(executor.nestedTxnId))
	}
}

func (executor *invocationExecutor) commit(
	modifiedSets programsCache.ModifiedSetsInvalidator,
) error {
	if executor.txnState.NumNestedTransactions() > 1 {
		// This is a fvm internal programming error.  We forgot to call Commit
		// somewhere in the control flow.  We should halt.
		return fmt.Errorf(
			"successfully executed transaction has unexpected " +
				"nested transactions.")
	}

	// if tx failed this will only contain fee deduction logs
	executor.proc.Logs = append(executor.proc.Logs, executor.env.Logs()...)
	executor.proc.ComputationUsed += executor.env.ComputationUsed()
	executor.proc.MemoryEstimate += executor.env.MemoryEstimate()
	if executor.proc.IsSampled() {
		executor.proc.ComputationIntensities = executor.env.ComputationIntensities()
	}

	// if tx failed this will only contain fee deduction events
	executor.proc.Events = append(executor.proc.Events, executor.env.Events()...)
	executor.proc.ServiceEvents = append(
		executor.proc.ServiceEvents,
		executor.env.ServiceEvents()...)

	// based on the contract and frozen account updates we decide how to
	// clean up the programs for failed transactions we also do the same as
	// transaction without any deployed contracts
	executor.programs.AddInvalidator(modifiedSets)

	_, commitErr := executor.txnState.Commit(executor.nestedTxnId)
	if commitErr != nil {
		return fmt.Errorf(
			"transaction invocation failed when merging state: %w",
			commitErr)
	}

	return nil
}
