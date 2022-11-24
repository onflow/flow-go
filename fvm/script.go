package fvm

import (
	"context"
	"fmt"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/programs"
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

// TODO(patrick): add ProcedureExecutor interface (superset of
// TransactionExecutor api).
func (proc *ScriptProcedure) NewExecutor(
	ctx Context,
	txnState *state.TransactionState,
	programs *programs.TransactionPrograms,
) *scriptExecutor {
	return newScriptExecutor(ctx, proc, txnState, programs)
}

func (proc *ScriptProcedure) Run(
	ctx Context,
	txnState *state.TransactionState,
	programs *programs.TransactionPrograms,
) error {
	// TODO(patrick): switch to run(proc.NewExecutor(ctx, txnState, programs)
	return proc.NewExecutor(ctx, txnState, programs).Execute()
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

func (proc *ScriptProcedure) InitialSnapshotTime() programs.LogicalTime {
	return programs.EndOfBlockExecutionTime
}

func (proc *ScriptProcedure) ExecutionTime() programs.LogicalTime {
	return programs.EndOfBlockExecutionTime
}

type scriptExecutor struct {
	proc *ScriptProcedure

	txnState    *state.TransactionState
	txnPrograms *programs.TransactionPrograms

	env environment.Environment
}

func newScriptExecutor(
	ctx Context,
	proc *ScriptProcedure,
	txnState *state.TransactionState,
	txnPrograms *programs.TransactionPrograms,
) *scriptExecutor {
	return &scriptExecutor{
		proc:        proc,
		txnState:    txnState,
		txnPrograms: txnPrograms,
		env:         NewScriptEnv(proc.RequestContext, ctx, txnState, txnPrograms),
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
	return nil
}
