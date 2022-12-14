package fvm

import (
	"fmt"
	"strconv"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/attribute"
	otelTrace "go.opentelemetry.io/otel/trace"

	"github.com/onflow/flow-go/fvm/derived"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/errors"
	reusableRuntime "github.com/onflow/flow-go/fvm/runtime"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/module/trace"
)

// TODO(patrick): rm once emulator is updated.
type TransactionInvoker struct {
}

func NewTransactionInvoker() *TransactionInvoker {
	return &TransactionInvoker{}
}

type TransactionExecutorParams struct {
	AuthorizationChecksEnabled bool

	SequenceNumberCheckAndIncrementEnabled bool

	// If AccountKeyWeightThreshold is set to a negative number, signature
	// verification is skipped during authorization checks.
	//
	// Note: This is set only by tests
	AccountKeyWeightThreshold int

	// Note: This is disabled only by tests
	TransactionBodyExecutionEnabled bool
}

func DefaultTransactionExecutorParams() TransactionExecutorParams {
	return TransactionExecutorParams{
		AuthorizationChecksEnabled:             true,
		SequenceNumberCheckAndIncrementEnabled: true,
		AccountKeyWeightThreshold:              AccountKeyWeightThreshold,
		TransactionBodyExecutionEnabled:        true,
	}
}

type transactionExecutor struct {
	TransactionExecutorParams

	TransactionVerifier
	TransactionSequenceNumberChecker
	TransactionStorageLimiter
	TransactionPayerBalanceChecker

	ctx            Context
	proc           *TransactionProcedure
	txnState       *state.TransactionState
	derivedTxnData *derived.DerivedTransactionData

	span otelTrace.Span
	env  environment.Environment

	errs *errors.ErrorsCollector

	nestedTxnId state.NestedTransactionId

	cadenceRuntime  *reusableRuntime.ReusableCadenceRuntime
	txnBodyExecutor runtime.Executor
}

func newTransactionExecutor(
	ctx Context,
	proc *TransactionProcedure,
	txnState *state.TransactionState,
	derivedTxnData *derived.DerivedTransactionData,
) *transactionExecutor {
	span := proc.StartSpanFromProcTraceSpan(
		ctx.Tracer,
		trace.FVMExecuteTransaction)
	span.SetAttributes(attribute.String("transaction_id", proc.ID.String()))

	ctx.RootSpan = span
	ctx.TxIndex = proc.TxIndex
	ctx.TxId = proc.Transaction.ID()
	ctx.TxBody = proc.Transaction

	env := environment.NewTransactionEnvironment(
		ctx.EnvironmentParams,
		txnState,
		derivedTxnData)

	return &transactionExecutor{
		TransactionExecutorParams: ctx.TransactionExecutorParams,
		ctx:                       ctx,
		proc:                      proc,
		txnState:                  txnState,
		derivedTxnData:            derivedTxnData,
		span:                      span,
		env:                       env,
		errs:                      errors.NewErrorsCollector(),
		cadenceRuntime:            env.BorrowCadenceRuntime(),
	}
}

func (executor *transactionExecutor) Cleanup() {
	executor.env.ReturnCadenceRuntime(executor.cadenceRuntime)
	executor.span.End()
}

func (executor *transactionExecutor) Preprocess() error {
	// TODO(patrick): split ExecuteTransactionBody preprocessing.
	return nil
}

func (executor *transactionExecutor) Execute() error {
	err := executor.execute()
	txErr, failure := errors.SplitErrorTypes(err)
	if failure != nil {
		// log the full error path
		executor.ctx.Logger.Err(err).
			Msg("fatal error when executing a transaction")
		return failure
	}

	if txErr != nil {
		executor.proc.Err = txErr
	}

	return nil
}

func (executor *transactionExecutor) execute() error {
	if executor.AuthorizationChecksEnabled {
		err := executor.CheckAuthorization(
			executor.ctx.Tracer,
			executor.proc,
			executor.txnState,
			executor.AccountKeyWeightThreshold)
		if err != nil {
			return err
		}
	}

	if executor.SequenceNumberCheckAndIncrementEnabled {
		err := executor.CheckAndIncrementSequenceNumber(
			executor.ctx.Tracer,
			executor.proc,
			executor.txnState)
		if err != nil {
			return err
		}
	}

	if executor.TransactionBodyExecutionEnabled {
		err := executor.ExecuteTransactionBody()
		if err != nil {
			return err
		}
	}

	return nil
}

func (executor *transactionExecutor) ExecuteTransactionBody() error {
	meterParams, err := getBodyMeterParameters(
		executor.ctx,
		executor.proc,
		executor.txnState,
		executor.derivedTxnData)
	if err != nil {
		return fmt.Errorf("error gettng meter parameters: %w", err)
	}

	var beginErr error
	executor.nestedTxnId, beginErr = executor.txnState.BeginNestedTransactionWithMeterParams(
		meterParams)
	if beginErr != nil {
		return beginErr
	}

	var invalidator derived.TransactionInvalidator
	var txError error
	invalidator, txError = executor.normalExecution()
	if executor.errs.Collect(txError).CollectedFailure() {
		return executor.errs.ErrorOrNil()
	}

	if executor.errs.CollectedError() {
		invalidator = nil
		executor.errorExecution()
		if executor.errs.CollectedFailure() {
			return executor.errs.ErrorOrNil()
		}
	}

	executor.errs.Collect(executor.commit(invalidator))

	return executor.errs.ErrorOrNil()
}

func (executor *transactionExecutor) deductTransactionFees() (err error) {
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
func (executor *transactionExecutor) logExecutionIntensities() {
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

func (executor *transactionExecutor) normalExecution() (
	invalidator derived.TransactionInvalidator,
	err error,
) {
	maxTxFees, err := executor.CheckPayerBalanceAndReturnMaxFees(
		executor.proc,
		executor.txnState,
		executor.env)

	if err != nil {
		return
	}

	executor.txnBodyExecutor = executor.cadenceRuntime.NewTransactionExecutor(
		runtime.Script{
			Source:    executor.proc.Transaction.Script,
			Arguments: executor.proc.Transaction.Arguments,
		},
		common.TransactionLocation(executor.proc.ID))

	err = executor.txnBodyExecutor.Preprocess()
	if err != nil {
		err = fmt.Errorf(
			"transaction invocation failed when executing transaction: %w",
			err)
		return
	}

	err = executor.txnBodyExecutor.Execute()
	if err != nil {
		err = fmt.Errorf(
			"transaction invocation failed when executing transaction: %w",
			err)
		return
	}

	// Before checking storage limits, we must apply all pending changes
	// that may modify storage usage.
	invalidator, err = executor.env.FlushPendingUpdates()
	if err != nil {
		err = fmt.Errorf(
			"transaction invocation failed to flush pending changes from "+
				"environment: %w",
			err)
		return
	}

	// Check if all account storage limits are ok
	//
	// disable the computation/memory limit checks on storage checks,
	// so we don't error from computation/memory limits on this part.
	//
	// The storage limit check is performed for all accounts that were touched during the transaction.
	// The storage capacity of an account depends on its balance and should be higher than the accounts storage used.
	// The payer account is special cased in this check and its balance is considered max_fees lower than its
	// actual balance, for the purpose of calculating storage capacity, because the payer will have to pay for this tx.
	executor.txnState.RunWithAllLimitsDisabled(func() {
		err = executor.CheckStorageLimits(
			executor.env,
			executor.txnState.UpdatedAddresses(),
			executor.proc.Transaction.Payer,
			maxTxFees)
	})

	if err != nil {
		return
	}

	// log the execution intensities here, so that they do not contain data
	// from transaction fee deduction, because the payer is not charged for that.
	executor.logExecutionIntensities()

	executor.txnState.RunWithAllLimitsDisabled(func() {
		err = executor.deductTransactionFees()
	})

	return
}

// Clear changes and try to deduct fees again.
func (executor *transactionExecutor) errorExecution() {
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

func (executor *transactionExecutor) commit(
	invalidator derived.TransactionInvalidator,
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

	// Based on various (e.g., contract and frozen account) updates, we decide
	// how to clean up the derived data.  For failed transactions we also do
	// the same as a successful transaction without any updates.
	executor.derivedTxnData.AddInvalidator(invalidator)

	_, commitErr := executor.txnState.Commit(executor.nestedTxnId)
	if commitErr != nil {
		return fmt.Errorf(
			"transaction invocation failed when merging state: %w",
			commitErr)
	}

	return nil
}
