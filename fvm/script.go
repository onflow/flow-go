package fvm

import (
	"context"
	"fmt"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/fvm/derived"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/hash"
)

type ScriptProcedure struct {
	ID             flow.Identifier
	Script         []byte
	Arguments      [][]byte
	RequestContext context.Context
	Value          cadence.Value
	Logs           []string
	Events         []flow.Event
	GasUsed        uint64
	MemoryEstimate uint64
	Err            errors.CodedError
}

func Script(code []byte) *ScriptProcedure {
	scriptHash := hash.DefaultHasher.ComputeHash(code)

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
	scriptHash := hash.DefaultHasher.ComputeHash(code)
	return &ScriptProcedure{
		ID:             flow.HashToID(scriptHash),
		Script:         code,
		RequestContext: reqContext,
		Arguments:      args,
	}
}

func (proc *ScriptProcedure) NewExecutor(
	ctx Context,
	txnState *state.TransactionState,
	derivedTxnData *derived.DerivedTransactionData,
) ProcedureExecutor {
	return newScriptExecutor(ctx, proc, txnState, derivedTxnData)
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

func (proc *ScriptProcedure) InitialSnapshotTime() derived.LogicalTime {
	return derived.EndOfBlockExecutionTime
}

func (proc *ScriptProcedure) ExecutionTime() derived.LogicalTime {
	return derived.EndOfBlockExecutionTime
}

type scriptExecutor struct {
	ctx            Context
	proc           *ScriptProcedure
	txnState       *state.TransactionState
	derivedTxnData *derived.DerivedTransactionData

	env environment.Environment
}

func newScriptExecutor(
	ctx Context,
	proc *ScriptProcedure,
	txnState *state.TransactionState,
	derivedTxnData *derived.DerivedTransactionData,
) *scriptExecutor {
	return &scriptExecutor{
		ctx:            ctx,
		proc:           proc,
		txnState:       txnState,
		derivedTxnData: derivedTxnData,
		env: environment.NewScriptEnvironment(
			proc.RequestContext,
			ctx.EnvironmentParams,
			txnState,
			derivedTxnData),
	}
}

func (executor *scriptExecutor) Cleanup() {
	// Do nothing.
}

func (executor *scriptExecutor) Preprocess() error {
	// Do nothing.
	return nil
}

func (executor *scriptExecutor) Execute() error {
	err := executor.execute()
	txError, failure := errors.SplitErrorTypes(err)
	if failure != nil {
		if errors.IsALedgerFailure(failure) {
			return fmt.Errorf(
				"cannot execute the script, this error usually happens if "+
					"the reference block for this script is not set to a "+
					"recent block: %w",
				failure)
		}
		return failure
	}
	if txError != nil {
		executor.proc.Err = txError
	}

	return nil
}

func (executor *scriptExecutor) execute() error {
	meterParams, err := getBodyMeterParameters(
		executor.ctx,
		executor.proc,
		executor.txnState,
		executor.derivedTxnData)
	if err != nil {
		return fmt.Errorf("error gettng meter parameters: %w", err)
	}

	txnId, err := executor.txnState.BeginNestedTransactionWithMeterParams(
		meterParams)
	if err != nil {
		return err
	}

	rt := executor.env.BorrowCadenceRuntime()
	defer executor.env.ReturnCadenceRuntime(rt)

	value, err := rt.ExecuteScript(
		runtime.Script{
			Source:    executor.proc.Script,
			Arguments: executor.proc.Arguments,
		},
		common.ScriptLocation(executor.proc.ID))

	if err != nil {
		return err
	}

	executor.proc.Value = value
	executor.proc.Logs = executor.env.Logs()
	executor.proc.Events = executor.env.Events()
	executor.proc.GasUsed = executor.env.ComputationUsed()
	executor.proc.MemoryEstimate = executor.env.MemoryEstimate()

	_, err = executor.txnState.Commit(txnId)
	return err
}
