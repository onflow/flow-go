package runtime

import (
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/sema"

	"github.com/onflow/flow-go/fvm/errors"
)

// WrappedCadenceRuntime wraps cadence runtime to handle errors.
// Errors from cadence runtime should be handled with `errors.HandleRuntimeError` before the FVM can understand them.
// Handling all possible locations, where the error could be coming from, here,
// makes it impossible to forget to handle the error.
type WrappedCadenceRuntime struct {
	runtime.Runtime
}

func (wr WrappedCadenceRuntime) NewScriptExecutor(s runtime.Script, c runtime.Context) runtime.Executor {
	return WrappedCadenceExecutor{wr.Runtime.NewScriptExecutor(s, c)}
}

func (wr WrappedCadenceRuntime) ExecuteScript(s runtime.Script, c runtime.Context) (cadence.Value, error) {
	v, err := wr.Runtime.ExecuteScript(s, c)
	return v, errors.HandleRuntimeError(err)
}

func (wr WrappedCadenceRuntime) NewTransactionExecutor(s runtime.Script, c runtime.Context) runtime.Executor {
	return WrappedCadenceExecutor{wr.Runtime.NewTransactionExecutor(s, c)}
}

func (wr WrappedCadenceRuntime) ExecuteTransaction(s runtime.Script, c runtime.Context) error {
	err := wr.Runtime.ExecuteTransaction(s, c)
	return errors.HandleRuntimeError(err)
}

func (wr WrappedCadenceRuntime) NewContractFunctionExecutor(
	contractLocation common.AddressLocation,
	functionName string,
	arguments []cadence.Value,
	argumentTypes []sema.Type,
	context runtime.Context,
) runtime.Executor {
	return WrappedCadenceExecutor{wr.Runtime.NewContractFunctionExecutor(contractLocation, functionName, arguments, argumentTypes, context)}
}

func (wr WrappedCadenceRuntime) InvokeContractFunction(
	contractLocation common.AddressLocation,
	functionName string,
	arguments []cadence.Value,
	argumentTypes []sema.Type,
	context runtime.Context,
) (cadence.Value, error) {
	v, err := wr.Runtime.InvokeContractFunction(contractLocation, functionName, arguments, argumentTypes, context)
	return v, errors.HandleRuntimeError(err)
}

func (wr WrappedCadenceRuntime) ParseAndCheckProgram(source []byte, context runtime.Context) (*interpreter.Program, error) {
	p, err := wr.Runtime.ParseAndCheckProgram(source, context)
	return p, errors.HandleRuntimeError(err)
}

func (wr WrappedCadenceRuntime) ReadStored(address common.Address, path cadence.Path, context runtime.Context) (cadence.Value, error) {
	v, err := wr.Runtime.ReadStored(address, path, context)
	return v, errors.HandleRuntimeError(err)
}

func (wr WrappedCadenceRuntime) Storage(context runtime.Context) (*runtime.Storage, *interpreter.Interpreter, error) {
	s, i, err := wr.Runtime.Storage(context)
	return s, i, errors.HandleRuntimeError(err)
}

type WrappedCadenceExecutor struct {
	runtime.Executor
}

func (we WrappedCadenceExecutor) Preprocess() error {
	return errors.HandleRuntimeError(we.Executor.Preprocess())
}

func (we WrappedCadenceExecutor) Execute() error {
	return errors.HandleRuntimeError(we.Executor.Execute())
}

func (we WrappedCadenceExecutor) Result() (cadence.Value, error) {
	v, err := we.Executor.Result()
	return v, errors.HandleRuntimeError(err)
}
