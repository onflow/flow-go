package testutil

import (
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/interpreter"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/sema"
)

var _ runtime.Runtime = &TestRuntime{}

type TestRuntime struct {
	ReadStoredFunc func(
		address common.Address,
		path cadence.Path,
		context runtime.Context,
	) (cadence.Value, error)
	InvokeContractFunc func(
		a common.AddressLocation,
		s string,
		values []cadence.Value,
		types []sema.Type,
		ctx runtime.Context,
	) (cadence.Value, error)
}

func (t *TestRuntime) Config() runtime.Config {
	panic("Config not defined")
}

func (t *TestRuntime) NewScriptExecutor(_ runtime.Script, _ runtime.Context) runtime.Executor {
	panic("NewScriptExecutor not defined")
}

func (t *TestRuntime) NewTransactionExecutor(_ runtime.Script, _ runtime.Context) runtime.Executor {
	panic("NewTransactionExecutor not defined")
}

func (t *TestRuntime) NewContractFunctionExecutor(
	_ common.AddressLocation,
	_ string,
	_ []cadence.Value,
	_ []sema.Type,
	_ runtime.Context,
) runtime.Executor {
	panic("NewContractFunctionExecutor not defined")
}

func (t *TestRuntime) SetDebugger(_ *interpreter.Debugger) {
	panic("SetDebugger not defined")
}

func (t *TestRuntime) ExecuteScript(_ runtime.Script, _ runtime.Context) (cadence.Value, error) {
	panic("ExecuteScript not defined")
}

func (t *TestRuntime) ExecuteTransaction(_ runtime.Script, _ runtime.Context) error {
	panic("ExecuteTransaction not defined")
}

func (t *TestRuntime) InvokeContractFunction(
	a common.AddressLocation,
	s string,
	values []cadence.Value,
	types []sema.Type,
	ctx runtime.Context,
) (cadence.Value, error) {
	if t.InvokeContractFunc == nil {
		panic("InvokeContractFunction not defined")
	}
	return t.InvokeContractFunc(a, s, values, types, ctx)
}

func (t *TestRuntime) ParseAndCheckProgram(_ []byte, _ runtime.Context) (*interpreter.Program, error) {
	panic("ParseAndCheckProgram not defined")
}

func (t *TestRuntime) ReadStored(
	address common.Address,
	path cadence.Path,
	context runtime.Context,
) (cadence.Value, error) {
	if t.ReadStoredFunc == nil {
		panic("ReadStored not defined")
	}
	return t.ReadStoredFunc(address, path, context)
}

func (*TestRuntime) Storage(_ runtime.Context) (*runtime.Storage, *interpreter.Interpreter, error) {
	panic("Storage not defined")
}
