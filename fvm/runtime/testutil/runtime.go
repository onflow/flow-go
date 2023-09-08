package testutil

import (
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/sema"
)

var _ runtime.Runtime = &TestInterpreterRuntime{}

type TestInterpreterRuntime struct {
	ReadStoredFunc     func(address common.Address, path cadence.Path, context runtime.Context) (cadence.Value, error)
	InvokeContractFunc func(a common.AddressLocation, s string, values []cadence.Value, types []sema.Type, ctx runtime.Context) (cadence.Value, error)
}

func (t *TestInterpreterRuntime) Config() runtime.Config {
	panic("Config not defined")
}

func (t *TestInterpreterRuntime) NewScriptExecutor(script runtime.Script, context runtime.Context) runtime.Executor {
	panic("NewScriptExecutor not defined")
}

func (t *TestInterpreterRuntime) NewTransactionExecutor(script runtime.Script, context runtime.Context) runtime.Executor {
	panic("NewTransactionExecutor not defined")
}

func (t *TestInterpreterRuntime) NewContractFunctionExecutor(contractLocation common.AddressLocation, functionName string, arguments []cadence.Value, argumentTypes []sema.Type, context runtime.Context) runtime.Executor {
	panic("NewContractFunctionExecutor not defined")
}

func (t *TestInterpreterRuntime) SetDebugger(debugger *interpreter.Debugger) {
	panic("SetDebugger not defined")
}

func (t *TestInterpreterRuntime) ExecuteScript(runtime.Script, runtime.Context) (cadence.Value, error) {
	panic("ExecuteScript not defined")
}

func (t *TestInterpreterRuntime) ExecuteTransaction(runtime.Script, runtime.Context) error {
	panic("ExecuteTransaction not defined")
}

func (t *TestInterpreterRuntime) InvokeContractFunction(a common.AddressLocation, s string, values []cadence.Value, types []sema.Type, ctx runtime.Context) (cadence.Value, error) {
	if t.InvokeContractFunc == nil {
		panic("InvokeContractFunction not defined")
	}
	return t.InvokeContractFunc(a, s, values, types, ctx)
}

func (t *TestInterpreterRuntime) ParseAndCheckProgram([]byte, runtime.Context) (*interpreter.Program, error) {
	panic("ParseAndCheckProgram not defined")
}

func (t *TestInterpreterRuntime) SetCoverageReport(*runtime.CoverageReport) {
	panic("SetCoverageReport not defined")
}

func (t *TestInterpreterRuntime) SetContractUpdateValidationEnabled(bool) {
	panic("SetContractUpdateValidationEnabled not defined")
}

func (t *TestInterpreterRuntime) SetAtreeValidationEnabled(bool) {
	panic("SetAtreeValidationEnabled not defined")
}

func (t *TestInterpreterRuntime) SetTracingEnabled(bool) {
	panic("SetTracingEnabled not defined")
}

func (t *TestInterpreterRuntime) SetInvalidatedResourceValidationEnabled(bool) {
	panic("SetInvalidatedResourceValidationEnabled not defined")
}

func (t *TestInterpreterRuntime) SetResourceOwnerChangeHandlerEnabled(bool) {
	panic("SetResourceOwnerChangeHandlerEnabled not defined")
}

func (t *TestInterpreterRuntime) ReadStored(address common.Address, path cadence.Path, context runtime.Context) (cadence.Value, error) {
	if t.ReadStoredFunc == nil {
		panic("ReadStored not defined")
	}
	return t.ReadStoredFunc(address, path, context)
}

func (*TestInterpreterRuntime) Storage(runtime.Context) (*runtime.Storage, *interpreter.Interpreter, error) {
	panic("not implemented")
}
