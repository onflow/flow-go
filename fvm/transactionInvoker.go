package fvm

import (
	"fmt"
	"strconv"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/attribute"

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

	txIDStr := proc.ID.String()
	span := proc.StartSpanFromProcTraceSpan(ctx.Tracer, trace.FVMExecuteTransaction)
	span.SetAttributes(
		attribute.String("transaction_id", txIDStr),
	)
	defer span.End()

	nestedTxnId, beginErr := txnState.BeginNestedTransaction()
	if beginErr != nil {
		return beginErr
	}

	var modifiedSets programsCache.ModifiedSetsInvalidator

	errs := errors.NewErrorsCollector()

	env := NewTransactionEnvironment(ctx, txnState, programs, proc.Transaction, proc.TxIndex, span)

	var txError error
	modifiedSets, txError = i.normalExecution(proc, txnState, env)
	if errs.Collect(txError).CollectedFailure() {
		return errs.ErrorOrNil()
	}

	if errs.CollectedError() {
		modifiedSets = programsCache.ModifiedSetsInvalidator{}

		fatalErr := i.errorExecution(proc, txnState, nestedTxnId, env, errs)
		if fatalErr != nil {
			return fatalErr
		}
	}

	errs.Collect(i.commit(
		proc,
		txnState,
		nestedTxnId,
		env,
		programs,
		modifiedSets))

	return errs.ErrorOrNil()
}

func (i TransactionInvoker) deductTransactionFees(
	proc *TransactionProcedure,
	txnState *state.TransactionState,
	env environment.Environment,
) (err error) {
	if !env.TransactionFeesEnabled() {
		return nil
	}

	computationUsed := env.ComputationUsed()
	if computationUsed > uint64(txnState.TotalComputationLimit()) {
		computationUsed = uint64(txnState.TotalComputationLimit())
	}

	// Hardcoded inclusion effort (of 1.0 UFix). Eventually this will be
	// dynamic. Execution effort will be connected to computation used.
	inclusionEffort := uint64(100_000_000)
	_, err = env.DeductTransactionFees(
		proc.Transaction.Payer,
		inclusionEffort,
		computationUsed)

	if err != nil {
		return errors.NewTransactionFeeDeductionFailedError(
			proc.Transaction.Payer,
			computationUsed,
			err)
	}
	return nil
}

// logExecutionIntensities logs execution intensities of the transaction
func (i TransactionInvoker) logExecutionIntensities(
	txnState *state.TransactionState,
	env environment.Environment,
) {
	if !env.Logger().Debug().Enabled() {
		return
	}

	computation := zerolog.Dict()
	for s, u := range txnState.ComputationIntensities() {
		computation.Uint(strconv.FormatUint(uint64(s), 10), u)
	}
	memory := zerolog.Dict()
	for s, u := range txnState.MemoryIntensities() {
		memory.Uint(strconv.FormatUint(uint64(s), 10), u)
	}
	env.Logger().Info().
		Uint64("ledgerInteractionUsed", txnState.InteractionUsed()).
		Uint64("computationUsed", txnState.TotalComputationUsed()).
		Uint64("memoryEstimate", txnState.TotalMemoryEstimate()).
		Dict("computationIntensities", computation).
		Dict("memoryIntensities", memory).
		Msg("transaction execution data")
}

func (i TransactionInvoker) normalExecution(
	proc *TransactionProcedure,
	txnState *state.TransactionState,
	env environment.Environment,
) (
	modifiedSets programsCache.ModifiedSetsInvalidator,
	err error,
) {
	rt := env.BorrowCadenceRuntime()
	defer env.ReturnCadenceRuntime(rt)

	err = rt.ExecuteTransaction(
		runtime.Script{
			Source:    proc.Transaction.Script,
			Arguments: proc.Transaction.Arguments,
		},
		common.TransactionLocation(proc.ID))

	if err != nil {
		err = fmt.Errorf(
			"transaction invocation failed when executing transaction: %w",
			err)
		return
	}

	// Before checking storage limits, we must applying all pending changes
	// that may modify storage usage.
	modifiedSets, err = env.FlushPendingUpdates()
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
	i.logExecutionIntensities(txnState, env)

	txnState.RunWithAllLimitsDisabled(func() {
		err = i.deductTransactionFees(proc, txnState, env)
	})
	if err != nil {
		return
	}

	// Check if all account storage limits are ok
	//
	// disable the computation/memory limit checks on storage checks,
	// so we don't error from computation/memory limits on this part.
	// We cannot charge the user for this part, since fee deduction already happened.
	txnState.RunWithAllLimitsDisabled(func() {
		err = NewTransactionStorageLimiter().CheckLimits(env, txnState.UpdatedAddresses())
	})

	return
}

// Clear changes and try to deduct fees again.
func (i TransactionInvoker) errorExecution(
	proc *TransactionProcedure,
	txnState *state.TransactionState,
	nestedTxnId state.NestedTransactionId,
	env environment.Environment,
	errs *errors.ErrorsCollector,
) error {
	txnState.DisableAllLimitEnforcements()
	defer txnState.EnableAllLimitEnforcements()

	// log transaction as failed
	env.Logger().Info().
		Err(errs.ErrorOrNil()).
		Msg("transaction executed with error")

	env.Reset()

	// drop delta since transaction failed
	restartErr := txnState.RestartNestedTransaction(nestedTxnId)
	if errs.Collect(restartErr).CollectedFailure() {
		return errs.ErrorOrNil()
	}

	// try to deduct fees again, to get the fee deduction events
	feesError := i.deductTransactionFees(proc, txnState, env)

	// if fee deduction fails just do clean up and exit
	if feesError != nil {
		env.Logger().Info().
			Err(feesError).
			Msg("transaction fee deduction executed with error")

		if errs.Collect(feesError).CollectedFailure() {
			return errs.ErrorOrNil()
		}

		// drop delta
		restartErr = txnState.RestartNestedTransaction(nestedTxnId)
		if errs.Collect(restartErr).CollectedFailure() {
			return errs.ErrorOrNil()
		}
	}

	return nil
}

func (i TransactionInvoker) commit(
	proc *TransactionProcedure,
	txnState *state.TransactionState,
	nestedTxnId state.NestedTransactionId,
	env environment.Environment,
	programs *programsCache.TransactionPrograms,
	modifiedSets programsCache.ModifiedSetsInvalidator,
) error {
	if txnState.NumNestedTransactions() > 1 {
		// This is a fvm internal programming error.  We forgot to call Commit
		// somewhere in the control flow.  We should halt.
		return fmt.Errorf(
			"successfully executed transaction has unexpected " +
				"nested transactions.")
	}

	// if tx failed this will only contain fee deduction logs
	proc.Logs = append(proc.Logs, env.Logs()...)
	proc.ComputationUsed = proc.ComputationUsed + env.ComputationUsed()
	proc.MemoryEstimate = proc.MemoryEstimate + env.MemoryEstimate()
	if proc.IsSampled() {
		proc.ComputationIntensities = env.ComputationIntensities()
	}

	// if tx failed this will only contain fee deduction events
	proc.Events = append(proc.Events, env.Events()...)
	proc.ServiceEvents = append(proc.ServiceEvents, env.ServiceEvents()...)

	// based on the contract and frozen account updates we decide how to
	// clean up the programs for failed transactions we also do the same as
	// transaction without any deployed contracts
	programs.AddInvalidator(modifiedSets)

	_, commitErr := txnState.Commit(nestedTxnId)
	if commitErr != nil {
		return fmt.Errorf(
			"transaction invocation failed when merging state: %w",
			commitErr)
	}

	return nil
}
