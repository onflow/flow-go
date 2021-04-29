package processors

import (
	"fmt"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/sema"
	"github.com/opentracing/opentracing-go"
	traceLog "github.com/opentracing/opentracing-go/log"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/fvm/context"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/module/trace"
)

type TransactionInvocator struct{}

func NewTransactionInvocator(logger zerolog.Logger) *TransactionInvocator {
	return &TransactionInvocator{}
}

func (i *TransactionInvocator) Invocate(
	vm context.VirtualMachine,
	ctx *context.Context,
	proc context.Runnable,
	sth *state.StateHolder,
	programs *programs.Programs,
	env *envs.TransactionEnvironment,
	parentSpan opentracing.Span,
	logger zerolog.Logger,
) (processErr error) {

	var span opentracing.Span
	if parentSpan != nil {
		span = ctx.Tracer.StartSpanFromParent(parentSpan, trace.FVMExecuteTransaction)
		span.LogFields(
			traceLog.String("transaction.ID", proc.ID().String()),
		)
		defer span.Finish()
	}

	var blockHeight uint64
	if ctx.BlockHeader != nil {
		blockHeight = ctx.BlockHeader.Height
	}

	var txError error
	retry := false
	numberOfRetries := 0

	parentState := sth.State()
	childState := sth.NewChild()
	predeclaredValues := i.valueDeclarations(ctx, env)

	defer func() {
		// an extra check for state holder health, this should never happen
		if childState != sth.State() {
			// error transaction
			msg := "child state doesn't match the active state on the state holder"
			logger.Error().
				Str("txHash", proc.ID().String()).
				Uint64("blockHeight", blockHeight).
				Msg(msg)

			// drop delta
			childState.View().DropDelta()
			env.Reset()
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

			logger.Warn().
				Str("txHash", proc.ID().String()).
				Uint64("blockHeight", blockHeight).
				Int("retries_count", numberOfRetries).
				Uint64("ledger_interaction_used", sth.State().InteractionUsed()).
				Msg("retrying transaction execution")

			// reset error part of proc
			// Warning right now the tx requires retry logic doesn't change
			// anything on state but we might want to revert the state changes (or not commiting)
			// if we decided to expand it furthur.
			processErr = nil
			env.Reset()
		}

		env.setTraceSpan(span)

		txId := proc.ID()
		location := common.TransactionLocation(txId[:])

		err := vm.Runtime().ExecuteTransaction(
			runtime.Script{
				Source:    proc.Script(),
				Arguments: proc.Arguments(),
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
		if !i.requiresRetry(err, proc, logger) {
			break
		}

		retry = true
	}

	// (for future use) panic if we tried several times and still failing because of checking issue
	// if numberOfTries == maxNumberOfRetries {
	// 	panic(err)
	// }

	// applying contract changes
	// this writes back the contract contents to accounts
	// if any error occurs we fail the tx
	updatedKeys, err := env.Commit()
	if err != nil && txError == nil {
		txError = fmt.Errorf("transaction invocation failed: %w", err)
	}

	// check the storage limits
	if ctx.LimitAccountStorage && txError == nil {
		txError = StorageLimiter{}.Process(vm, ctx, proc, sth, programs)
	}

	if txError != nil {
		// drop delta
		childState.View().DropDelta()
		env.Reset()
		// if tx fails just do clean up
		programs.Cleanup(nil)
		logger.Info().
			Str("txHash", proc.ID().String()).
			Uint64("blockHeight", blockHeight).
			Uint64("ledgerInteractionUsed", sth.State().InteractionUsed()).
			Msg("transaction executed with error")
		return txError
	}

	// based on the contract updates we decide how to clean up the programs
	// for failed transactions we also do the same as
	// transaction without any deployed contracts
	programs.Cleanup(updatedKeys)

	env.Commit()

	logger.Info().
		Str("txHash", proc.ID().String()).
		Uint64("blockHeight", blockHeight).
		Uint64("ledgerInteractionUsed", sth.State().InteractionUsed()).
		Msg("transaction executed successfully")

	return nil
}

func (TransactionInvocator) valueDeclarations(ctx *context.Context, txEnv *env.TransactionEnvironment) []runtime.ValueDeclaration {
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
					err := txEnv.SetAccountFrozen(common.Address(address), bool(frozen))
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
func (i *TransactionInvocator) requiresRetry(err error, proc context.Procedure, logger zerolog.Logger) bool {
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

	logger.Error().
		Str("txHash", proc.ID().String()).
		Msg("checking failed")

	return true
}
