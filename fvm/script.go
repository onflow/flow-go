package fvm

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/evm"
	"github.com/onflow/flow-go/fvm/storage"
	"github.com/onflow/flow-go/fvm/storage/logical"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/hash"
)

type ScriptProcedure struct {
	ID             flow.Identifier
	Script         []byte
	Arguments      [][]byte
	RequestContext context.Context
}

func Script(code []byte) *ScriptProcedure {
	scriptHash := hash.DefaultComputeHash(code)

	return &ScriptProcedure{
		Script:         code,
		ID:             flow.HashToID(scriptHash),
		RequestContext: context.Background(),
	}
}

func (proc *ScriptProcedure) WithArguments(args ...[]byte) *ScriptProcedure {
	return &ScriptProcedure{
		ID:             proc.ID,
		Script:         proc.Script,
		RequestContext: proc.RequestContext,
		Arguments:      args,
	}
}

func (proc *ScriptProcedure) WithRequestContext(
	reqContext context.Context,
) *ScriptProcedure {
	return &ScriptProcedure{
		ID:             proc.ID,
		Script:         proc.Script,
		RequestContext: reqContext,
		Arguments:      proc.Arguments,
	}
}

func NewScriptWithContextAndArgs(
	code []byte,
	reqContext context.Context,
	args ...[]byte,
) *ScriptProcedure {
	scriptHash := hash.DefaultComputeHash(code)
	return &ScriptProcedure{
		ID:             flow.HashToID(scriptHash),
		Script:         code,
		RequestContext: reqContext,
		Arguments:      args,
	}
}

func (proc *ScriptProcedure) NewExecutor(
	ctx Context,
	txnState storage.TransactionPreparer,
) ProcedureExecutor {
	return newScriptExecutor(ctx, proc, txnState)
}

func (proc *ScriptProcedure) ComputationLimit(ctx Context) uint64 {
	computationLimit := ctx.ComputationLimit
	// if ctx.ComputationLimit is also zero, fallback to the default computation limit
	if computationLimit == 0 {
		computationLimit = DefaultComputationLimit
	}
	return computationLimit
}

func (proc *ScriptProcedure) MemoryLimit(ctx Context) uint64 {
	memoryLimit := ctx.MemoryLimit
	// if ctx.MemoryLimit is also zero, fallback to the default memory limit
	if memoryLimit == 0 {
		memoryLimit = DefaultMemoryLimit
	}
	return memoryLimit
}

func (proc *ScriptProcedure) ShouldDisableMemoryAndInteractionLimits(
	ctx Context,
) bool {
	return ctx.DisableMemoryAndInteractionLimits
}

func (ScriptProcedure) Type() ProcedureType {
	return ScriptProcedureType
}

func (proc *ScriptProcedure) ExecutionTime() logical.Time {
	return logical.EndOfBlockExecutionTime
}

type scriptExecutor struct {
	ctx      Context
	proc     *ScriptProcedure
	txnState storage.TransactionPreparer

	env environment.Environment

	output ProcedureOutput
}

func newScriptExecutor(
	ctx Context,
	proc *ScriptProcedure,
	txnState storage.TransactionPreparer,
) *scriptExecutor {
	// update `ctx.EnvironmentParams` with the script info before
	// creating the executor
	scriptInfo := environment.NewScriptInfoParams(proc.Script, proc.Arguments)
	ctx.EnvironmentParams.SetScriptInfoParams(scriptInfo)
	return &scriptExecutor{
		ctx:      ctx,
		proc:     proc,
		txnState: txnState,
		env: environment.NewScriptEnv(
			proc.RequestContext,
			ctx.TracerSpan,
			ctx.EnvironmentParams,
			txnState),
	}
}

func (executor *scriptExecutor) Cleanup() {
	// Do nothing.
}

func (executor *scriptExecutor) Output() ProcedureOutput {
	return executor.output
}

func (executor *scriptExecutor) Preprocess() error {
	// Do nothing.
	return nil
}

func (executor *scriptExecutor) Execute() error {
	err := executor.execute()
	txError, failure := errors.SplitErrorTypes(err)
	if failure != nil {
		if errors.IsLedgerFailure(failure) {
			return fmt.Errorf(
				"cannot execute the script, this error usually happens if "+
					"the reference block for this script is not set to a "+
					"recent block: %w",
				failure)
		}
		return failure
	}
	if txError != nil {
		executor.output.Err = txError
	}

	return nil
}

func (executor *scriptExecutor) execute() error {
	meterParams, err := getBodyMeterParameters(
		executor.ctx,
		executor.proc,
		executor.txnState)
	if err != nil {
		return fmt.Errorf("error getting meter parameters: %w", err)
	}

	txnId, err := executor.txnState.BeginNestedTransactionWithMeterParams(
		meterParams)
	if err != nil {
		return err
	}

	errs := errors.NewErrorsCollector()
	errs.Collect(executor.executeScript())

	_, err = executor.txnState.CommitNestedTransaction(txnId)
	errs.Collect(err)

	return errs.ErrorOrNil()
}

func (executor *scriptExecutor) executeScript() error {
	rt := executor.env.BorrowCadenceRuntime()
	defer executor.env.ReturnCadenceRuntime(rt)

	if executor.ctx.EVMEnabled {
		chain := executor.ctx.Chain
		err := evm.SetupEnvironment(
			chain.ChainID(),
			executor.env,
			rt.ScriptRuntimeEnv,
			chain.ServiceAddress(),
			FlowTokenAddress(chain),
		)
		if err != nil {
			return err
		}
	}

	value, err := rt.ExecuteScript(
		runtime.Script{
			Source:    executor.proc.Script,
			Arguments: executor.proc.Arguments,
		},
		common.ScriptLocation(executor.proc.ID),
	)
	populateErr := executor.output.PopulateEnvironmentValues(executor.env)
	if err != nil {
		return multierror.Append(err, populateErr)
	}

	executor.output.Value = value
	return populateErr
}
