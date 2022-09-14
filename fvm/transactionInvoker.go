package fvm

import (
	"fmt"
	"strconv"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/attribute"
	otelTrace "go.opentelemetry.io/otel/trace"

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
	sth *state.StateHolder,
	programs *programsCache.Programs,
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

	nestedTxnId, err := sth.BeginNestedTransaction()
	if err != nil {
		return err
	}

	var modifiedSets programsCache.ModifiedSets
	defer func() {
		// based on the contract and frozen account updates we decide how to
		// clean up the programs for failed transactions we also do the same as
		// transaction without any deployed contracts
		programs.Cleanup(modifiedSets)

		if sth.NumNestedTransactions() > 1 {
			err := sth.RestartNestedTransaction(nestedTxnId)
			if err != nil {
				processErr = fmt.Errorf(
					"cannot restart nested transaction: %w",
					err,
				)
			}

			msg := "transaction has unexpected nested transactions"
			ctx.Logger.Error().
				Msg(msg)

			proc.Err = errors.NewFVMInternalErrorf(msg)
			proc.Logs = make([]string, 0)
			proc.Events = make([]flow.Event, 0)
			proc.ServiceEvents = make([]flow.Event, 0)

		}

		err := sth.Commit(nestedTxnId)
		if err != nil {
			processErr = fmt.Errorf("transaction invocation failed when merging state: %w", err)
		}
	}()

	env := NewTransactionEnv(ctx, sth, programs, proc.Transaction, proc.TxIndex, span)

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
	i.logExecutionIntensities(ctx, sth, txIDStr)

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
	modifiedSets, err = env.Commit()
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

		modifiedSets = programsCache.ModifiedSets{}
		env.Reset()

		// drop delta since transaction failed
		err := sth.RestartNestedTransaction(nestedTxnId)
		if err != nil {
			return fmt.Errorf(
				"cannot restart nested transaction: %w",
				err,
			)
		}

		// log transaction as failed
		ctx.Logger.Info().
			Msg("transaction executed with error")

		// try to deduct fees again, to get the fee deduction events
		feesError = i.deductTransactionFees(env, proc, sth, computationUsed)

		// if fee deduction fails just do clean up and exit
		if feesError != nil {
			ctx.Logger.Info().
				Msg("transaction fee deduction executed with error")

			txError = feesError

			// drop delta
			_ = sth.RestartNestedTransaction(nestedTxnId)
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
func (i TransactionInvoker) logExecutionIntensities(ctx Context, sth *state.StateHolder, txHash string) {
	if ctx.Logger.Debug().Enabled() {
		computation := zerolog.Dict()
		for s, u := range sth.ComputationIntensities() {
			computation.Uint(strconv.FormatUint(uint64(s), 10), u)
		}
		memory := zerolog.Dict()
		for s, u := range sth.MemoryIntensities() {
			memory.Uint(strconv.FormatUint(uint64(s), 10), u)
		}
		ctx.Logger.Info().
			Uint64("ledgerInteractionUsed", sth.InteractionUsed()).
			Uint("computationUsed", sth.TotalComputationUsed()).
			Uint64("memoryEstimate", sth.TotalMemoryEstimate()).
			Dict("computationIntensities", computation).
			Dict("memoryIntensities", memory).
			Msg("transaction execution data")
	}
}
