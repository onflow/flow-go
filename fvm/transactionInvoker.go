package fvm

import (
	"fmt"
	"strconv"

	"github.com/onflow/flow-go/fvm/systemcontracts"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/attribute"
	otelTrace "go.opentelemetry.io/otel/trace"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/evm"
	reusableRuntime "github.com/onflow/flow-go/fvm/runtime"
	"github.com/onflow/flow-go/fvm/storage"
	"github.com/onflow/flow-go/fvm/storage/derived"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/fvm/storage/state"
	"github.com/onflow/flow-go/module/trace"
)

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

	ctx      Context
	proc     *TransactionProcedure
	txnState storage.TransactionPreparer

	span otelTrace.Span
	env  environment.Environment

	errs *errors.ErrorsCollector

	startedTransactionBodyExecution bool
	nestedTxnId                     state.NestedTransactionId

	cadenceRuntime  *reusableRuntime.ReusableCadenceRuntime
	txnBodyExecutor runtime.Executor

	output ProcedureOutput
}

func newTransactionExecutor(
	ctx Context,
	proc *TransactionProcedure,
	txnState storage.TransactionPreparer,
) *transactionExecutor {
	span := ctx.StartChildSpan(trace.FVMExecuteTransaction)
	span.SetAttributes(attribute.String("transaction_id", proc.ID.String()))

	ctx.TxIndex = proc.TxIndex
	ctx.TxId = proc.ID
	ctx.TxBody = proc.Transaction

	env := environment.NewTransactionEnvironment(
		span,
		ctx.EnvironmentParams,
		txnState)

	return &transactionExecutor{
		TransactionExecutorParams: ctx.TransactionExecutorParams,
		TransactionVerifier: TransactionVerifier{
			VerificationConcurrency: 4,
		},
		ctx:                             ctx,
		proc:                            proc,
		txnState:                        txnState,
		span:                            span,
		env:                             env,
		errs:                            errors.NewErrorsCollector(),
		startedTransactionBodyExecution: false,
		cadenceRuntime:                  env.BorrowCadenceRuntime(),
	}
}

func (executor *transactionExecutor) Cleanup() {
	executor.env.ReturnCadenceRuntime(executor.cadenceRuntime)
	executor.span.End()
}

func (executor *transactionExecutor) Output() ProcedureOutput {
	return executor.output
}

func (executor *transactionExecutor) handleError(
	err error,
	step string,
) error {
	txErr, failure := errors.SplitErrorTypes(err)
	if failure != nil {
		// log the full error path
		executor.ctx.Logger.Err(err).
			Str("step", step).
			Msg("fatal error when handling a transaction")
		return failure
	}

	if txErr != nil {
		executor.output.Err = txErr
	}

	return nil
}

func (executor *transactionExecutor) Preprocess() error {
	return executor.handleError(executor.preprocess(), "preprocess")
}

func (executor *transactionExecutor) Execute() error {
	return executor.handleError(executor.execute(), "executing")
}

func (executor *transactionExecutor) preprocess() error {
	if executor.AuthorizationChecksEnabled {
		err := executor.CheckAuthorization(
			executor.ctx.TracerSpan,
			executor.proc,
			executor.txnState,
			executor.AccountKeyWeightThreshold)
		if err != nil {
			executor.errs.Collect(err)
			return executor.errs.ErrorOrNil()
		}
	}

	if executor.SequenceNumberCheckAndIncrementEnabled {
		err := executor.CheckAndIncrementSequenceNumber(
			executor.ctx.TracerSpan,
			executor.proc,
			executor.txnState)
		if err != nil {
			executor.errs.Collect(err)
			return executor.errs.ErrorOrNil()
		}
	}

	if !executor.TransactionBodyExecutionEnabled {
		return nil
	}

	executor.errs.Collect(executor.preprocessTransactionBody())
	if executor.errs.CollectedFailure() {
		return executor.errs.ErrorOrNil()
	}

	return nil
}

// preprocessTransactionBody preprocess parts of a transaction body that are
// infrequently modified and are expensive to compute.  For now this includes
// reading meter parameter overrides and parsing programs.
func (executor *transactionExecutor) preprocessTransactionBody() error {
	// setup evm
	if executor.ctx.EVMEnabled {
		chain := executor.ctx.Chain
		sc := systemcontracts.SystemContractsForChain(chain.ChainID())
		err := evm.SetupEnvironment(
			chain.ChainID(),
			executor.env,
			executor.cadenceRuntime.TxRuntimeEnv,
			chain.ServiceAddress(),
			sc.FlowToken.Address,
		)
		if err != nil {
			return err
		}
	}

	meterParams, err := getBodyMeterParameters(
		executor.ctx,
		executor.proc,
		executor.txnState)
	if err != nil {
		return fmt.Errorf("error gettng meter parameters: %w", err)
	}

	txnId, err := executor.txnState.BeginNestedTransactionWithMeterParams(
		meterParams)
	if err != nil {
		return err
	}
	executor.startedTransactionBodyExecution = true
	executor.nestedTxnId = txnId

	executor.txnBodyExecutor = executor.cadenceRuntime.NewTransactionExecutor(
		runtime.Script{
			Source:    executor.proc.Transaction.Script,
			Arguments: executor.proc.Transaction.Arguments,
		},
		common.TransactionLocation(executor.proc.ID))

	// This initializes various cadence variables and parses the programs used
	// by the transaction body.
	err = executor.txnBodyExecutor.Preprocess()
	if err != nil {
		return fmt.Errorf(
			"transaction preprocess failed: %w",
			err)
	}

	return nil
}

func (executor *transactionExecutor) execute() error {
	if !executor.startedTransactionBodyExecution {
		return executor.errs.ErrorOrNil()
	}

	return executor.ExecuteTransactionBody()
}

func (executor *transactionExecutor) ExecuteTransactionBody() error {
	var invalidator derived.TransactionInvalidator
	if !executor.errs.CollectedError() {

		var txError error
		invalidator, txError = executor.normalExecution()
		if executor.errs.Collect(txError).CollectedFailure() {
			return executor.errs.ErrorOrNil()
		}
	}

	if executor.errs.CollectedError() {
		invalidator = nil
		executor.txnState.RunWithAllLimitsDisabled(executor.errorExecution)
		if executor.errs.CollectedFailure() {
			return executor.errs.ErrorOrNil()
		}
	}

	// log the execution intensities here, so that they do not contain data
	// from transaction fee deduction, because the payer is not charged for that.
	executor.logExecutionIntensities()

	executor.errs.Collect(executor.commit(invalidator))

	return executor.errs.ErrorOrNil()
}

func (executor *transactionExecutor) deductTransactionFees() (err error) {
	if !executor.env.TransactionFeesEnabled() {
		return nil
	}

	computationLimit := uint64(executor.txnState.TotalComputationLimit())

	computationUsed, err := executor.env.ComputationUsed()
	if err != nil {
		return errors.NewTransactionFeeDeductionFailedError(
			executor.proc.Transaction.Payer,
			computationLimit,
			err)
	}

	if computationUsed > computationLimit {
		computationUsed = computationLimit
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
	log := executor.env.Logger()
	if !log.Debug().Enabled() {
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
	log.Debug().
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
	var maxTxFees uint64
	// run with limits disabled since this is a static cost check
	// and should be accounted for in the inclusion cost.
	executor.txnState.RunWithAllLimitsDisabled(func() {
		maxTxFees, err = executor.CheckPayerBalanceAndReturnMaxFees(
			executor.proc,
			executor.txnState,
			executor.env)
	})

	if err != nil {
		return
	}

	var bodyTxnId state.NestedTransactionId
	bodyTxnId, err = executor.txnState.BeginNestedTransaction()
	if err != nil {
		return
	}

	err = executor.txnBodyExecutor.Execute()
	if err != nil {
		err = fmt.Errorf("transaction execute failed: %w", err)
		return
	}

	// Before checking storage limits, we must apply all pending changes
	// that may modify storage usage.
	var contractUpdates environment.ContractUpdates
	contractUpdates, err = executor.env.FlushPendingUpdates()
	if err != nil {
		err = fmt.Errorf(
			"transaction invocation failed to flush pending changes from "+
				"environment: %w",
			err)
		return
	}

	var bodySnapshot *snapshot.ExecutionSnapshot
	bodySnapshot, err = executor.txnState.CommitNestedTransaction(bodyTxnId)
	if err != nil {
		return
	}

	invalidator = environment.NewDerivedDataInvalidator(
		contractUpdates,
		executor.ctx.Chain.ServiceAddress(),
		bodySnapshot)

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
			bodySnapshot,
			executor.proc.Transaction.Payer,
			maxTxFees)
	})

	if err != nil {
		return
	}

	executor.txnState.RunWithAllLimitsDisabled(func() {
		err = executor.deductTransactionFees()
	})

	return
}

// Clear changes and try to deduct fees again.
func (executor *transactionExecutor) errorExecution() {
	// log transaction as failed
	log := executor.env.Logger()
	log.Info().
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
		log.Info().
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

	err := executor.output.PopulateEnvironmentValues(executor.env)
	if err != nil {
		return err
	}

	// Based on various (e.g., contract) updates, we decide
	// how to clean up the derived data.  For failed transactions we also do
	// the same as a successful transaction without any updates.
	executor.txnState.AddInvalidator(invalidator)

	_, commitErr := executor.txnState.CommitNestedTransaction(
		executor.nestedTxnId)
	if commitErr != nil {
		return fmt.Errorf(
			"transaction invocation failed when merging state: %w",
			commitErr)
	}

	return nil
}
