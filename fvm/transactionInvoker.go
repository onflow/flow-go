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
	"github.com/onflow/flow-go/model/flow"
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
) (processErr error) {

	txIDStr := proc.ID.String()
	var span otelTrace.Span
	if ctx.Tracer != nil && proc.TraceSpan != nil {
		span = ctx.Tracer.StartSpanFromParent(proc.TraceSpan, trace.FVMExecuteTransaction)
		span.SetAttributes(
			attribute.String("transaction_id", txIDStr),
		)
		defer span.End()
	}

	nestedTxnId, err := txnState.BeginNestedTransaction()
	if err != nil {
		return err
	}

	var modifiedSets programsCache.ModifiedSetsInvalidator
	defer func() {
		if txnState.NumNestedTransactions() > 1 {
			if processErr == nil {
				// This is a fvm internal programming error.  We forgot to
				// call Commit somewhere in the control flow.  We should halt.
				processErr = fmt.Errorf(
					"successfully executed transaction has unexpected " +
						"nested transactions.")
				return
			} else {
				err := txnState.RestartNestedTransaction(nestedTxnId)
				if err != nil {
					// This should never happen since merging views should
					// never fail.
					processErr = fmt.Errorf(
						"cannot restart nested transaction on error: %w "+
							"(original txError: %v)",
						err,
						processErr)
					return
				}

				ctx.Logger.Warn().Msg(
					"transaction had unexpected nested transactions, " +
						"which have been restarted.")

				// Note: proc.Err is set by TransactionProcedure.
				proc.Logs = make([]string, 0)
				proc.Events = make([]flow.Event, 0)
				proc.ServiceEvents = make([]flow.Event, 0)
			}
		}

		// based on the contract and frozen account updates we decide how to
		// clean up the programs for failed transactions we also do the same as
		// transaction without any deployed contracts
		programs.AddInvalidator(modifiedSets)

		err := txnState.Commit(nestedTxnId)
		if err != nil {
			processErr = fmt.Errorf("transaction invocation failed when merging state: %w", err)
		}
	}()

	env := NewTransactionEnv(ctx, txnState, programs, proc.Transaction, proc.TxIndex, span)

	rt := env.BorrowCadenceRuntime()
	defer env.ReturnCadenceRuntime(rt)

	var txError error
	err = rt.ExecuteTransaction(
		runtime.Script{
			Source:    proc.Transaction.Script,
			Arguments: proc.Transaction.Arguments,
		},
		common.TransactionLocation(proc.ID))

	if err != nil {
		txError = fmt.Errorf(
			"transaction invocation failed when executing transaction: %w",
			err,
		)
	}

	// read computationUsed from the environment. This will be used to charge fees.
	computationUsed := env.ComputationUsed()
	memoryEstimate := env.MemoryEstimate()

	// log te execution intensities here, so tha they do not contain data from storage limit checks and
	// transaction deduction, because the payer is not charged for those.
	i.logExecutionIntensities(ctx, txnState, txIDStr)

	// disable the limit checks on states
	txnState.DisableAllLimitEnforcements()
	// try to deduct fees even if there is an error.
	// disable the limit checks on states
	feesError := i.deductTransactionFees(env, proc, txnState, computationUsed)
	if feesError != nil {
		txError = feesError
	}

	txnState.EnableAllLimitEnforcements()

	// applying contract changes
	// this writes back the contract contents to accounts
	// if any error occurs we fail the tx
	// this needs to happen before checking limits, so that contract changes are committed to the state
	modifiedSets, err = env.FlushPendingUpdates()
	if err != nil && txError == nil {
		txError = fmt.Errorf("transaction invocation failed when committing Environment: %w", err)
	}

	// if there is still no error check if all account storage limits are ok
	if txError == nil {
		// disable the computation/memory limit checks on storage checks,
		// so we don't error from computation/memory limits on this part.
		// We cannot charge the user for this part, since fee deduction already happened.
		txnState.DisableAllLimitEnforcements()
		txError = NewTransactionStorageLimiter().CheckLimits(env, txnState.UpdatedAddresses())
		txnState.EnableAllLimitEnforcements()
	}

	// it there was any transaction error clear changes and try to deduct fees again
	if txError != nil {
		txnState.DisableAllLimitEnforcements()
		defer txnState.EnableAllLimitEnforcements()

		// log transaction as failed
		ctx.Logger.Info().
			Err(txError).
			Msg("transaction executed with error")

		modifiedSets = programsCache.ModifiedSetsInvalidator{}
		env.Reset()

		// drop delta since transaction failed
		err := txnState.RestartNestedTransaction(nestedTxnId)
		if err != nil {
			return fmt.Errorf(
				"cannot restart nested transaction: %w",
				err,
			)
		}

		// try to deduct fees again, to get the fee deduction events
		feesError = i.deductTransactionFees(env, proc, txnState, computationUsed)

		// if fee deduction fails just do clean up and exit
		if feesError != nil {
			ctx.Logger.Info().
				Msg("transaction fee deduction executed with error")

			txError = feesError

			// drop delta
			_ = txnState.RestartNestedTransaction(nestedTxnId)
		}
	}

	// if tx failed this will only contain fee deduction logs
	proc.Logs = append(proc.Logs, env.Logs()...)
	proc.ComputationUsed = proc.ComputationUsed + computationUsed
	proc.MemoryEstimate = proc.MemoryEstimate + memoryEstimate

	// if tx failed this will only contain fee deduction events
	proc.Events = append(proc.Events, env.Events()...)
	proc.ServiceEvents = append(proc.ServiceEvents, env.ServiceEvents()...)

	return txError
}

func (i TransactionInvoker) deductTransactionFees(
	env environment.Environment,
	proc *TransactionProcedure,
	txnState *state.TransactionState,
	computationUsed uint64,
) (err error) {
	if !env.TransactionFeesEnabled() {
		return nil
	}

	if computationUsed > uint64(txnState.TotalComputationLimit()) {
		computationUsed = uint64(txnState.TotalComputationLimit())
	}

	// Hardcoded inclusion effort (of 1.0 UFix). Eventually this will be
	// dynamic.	Execution effort will be connected to computation used.
	inclusionEffort := uint64(100_000_000)
	_, err = env.DeductTransactionFees(
		proc.Transaction.Payer,
		inclusionEffort,
		computationUsed)

	if err != nil {
		return errors.NewTransactionFeeDeductionFailedError(proc.Transaction.Payer, err)
	}
	return nil
}

// logExecutionIntensities logs execution intensities of the transaction
func (i TransactionInvoker) logExecutionIntensities(
	ctx Context,
	txnState *state.TransactionState,
	txHash string,
) {
	if ctx.Logger.Debug().Enabled() {
		computation := zerolog.Dict()
		for s, u := range txnState.ComputationIntensities() {
			computation.Uint(strconv.FormatUint(uint64(s), 10), u)
		}
		memory := zerolog.Dict()
		for s, u := range txnState.MemoryIntensities() {
			memory.Uint(strconv.FormatUint(uint64(s), 10), u)
		}
		ctx.Logger.Info().
			Uint64("ledgerInteractionUsed", txnState.InteractionUsed()).
			Uint("computationUsed", txnState.TotalComputationUsed()).
			Uint64("memoryEstimate", txnState.TotalMemoryEstimate()).
			Dict("computationIntensities", computation).
			Dict("memoryIntensities", memory).
			Msg("transaction execution data")
	}
}
