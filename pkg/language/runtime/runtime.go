package runtime

import (
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/ast"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/common"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/sema"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/trampoline"

	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/interpreter"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/parser"
)

type RuntimeInterface interface {
	// GetValue gets a value for the given key in the storage, controlled and owned by the given accounts.
	GetValue(owner, controller, key []byte) (value []byte, err error)
	// SetValue sets a value for the given key in the storage, controlled and owned by the given accounts.
	SetValue(owner, controller, key, value []byte) (err error)
	// CreateAccount creates a new account with the given public key and code.
	CreateAccount(publicKey []byte, code []byte) (accountID []byte, err error)
}

type RuntimeError struct {
	Errors []error
}

func (e RuntimeError) Error() string {
	var sb strings.Builder
	sb.WriteString("Execution failed:\n")
	for _, err := range e.Errors {
		sb.WriteString(err.Error())
		sb.WriteString("\n")
	}
	return sb.String()
}

// Runtime is a runtime capable of executing the Bamboo programming language.
type Runtime interface {
	// ExecuteScript executes the given script.
	// It returns errors if the program has errors (e.g syntax errors, type errors),
	// and if the execution fails.
	ExecuteScript(script []byte, runtimeInterface RuntimeInterface) (interface{}, error)
}

// mockRuntime is a mocked version of the Bamboo runtime
type mockRuntime struct{}

// NewMockRuntime returns a mocked version of the Bamboo runtime.
func NewMockRuntime() Runtime {
	return &mockRuntime{}
}

func (r *mockRuntime) ExecuteScript(script []byte, runtimeInterface RuntimeInterface) (interface{}, error) {
	return nil, nil
}

// interpreterRuntime is a interpreter-based version of the Bamboo runtime.
type interpreterRuntime struct {
}

// NewInterpreterRuntime returns a interpreter-based version of the Bamboo runtime.
func NewInterpreterRuntime() Runtime {
	return &interpreterRuntime{}
}

func (r *interpreterRuntime) ExecuteScript(script []byte, runtimeInterface RuntimeInterface) (interface{}, error) {
	code := string(script)

	program, errs := parser.ParseProgram(code)
	if len(errs) > 0 {
		return nil, RuntimeError{errs}
	}

	checker := sema.NewChecker(program)

	if err := checker.DeclareValue(
		"getValue",
		&getValueFunctionType,
		common.DeclarationKindFunction,
		ast.Position{},
		true,
		nil,
	); err != nil {
		return nil, RuntimeError{[]error{err}}
	}

	if err := checker.DeclareValue(
		"setValue",
		&setValueFunctionType,
		common.DeclarationKindFunction,
		ast.Position{},
		true,
		nil,
	); err != nil {
		return nil, RuntimeError{[]error{err}}
	}

	if err := checker.DeclareValue(
		"createAccount",
		&createAccountFunctionType,
		common.DeclarationKindFunction,
		ast.Position{},
		true,
		nil,
	); err != nil {
		return nil, RuntimeError{[]error{err}}
	}

	if err := checker.Check(); err != nil {
		return nil, RuntimeError{[]error{err}}
	}

	inter := interpreter.NewInterpreter(program)
	inter.ImportFunction("getValue", r.newGetValueFunction(runtimeInterface))
	inter.ImportFunction("setValue", r.newSetValueFunction(runtimeInterface))
	inter.ImportFunction("createAccount", r.newCreateAccountFunction(runtimeInterface))

	if err := inter.Interpret(); err != nil {
		return nil, RuntimeError{[]error{err}}
	}

	if _, hasMain := inter.Globals["main"]; !hasMain {
		return nil, nil
	}

	value, err := inter.InvokeExportable("main")
	if err != nil {
		return nil, RuntimeError{[]error{err}}
	}

	return value.ToGoValue(), nil
}

// TODO: improve types
var setValueFunctionType = sema.FunctionType{
	ParameterTypes: []sema.Type{
		// owner
		&sema.VariableSizedType{
			Type: &sema.IntType{},
		},
		// controller
		&sema.VariableSizedType{
			Type: &sema.IntType{},
		},
		// key
		&sema.VariableSizedType{
			Type: &sema.IntType{},
		},
		// value
		// TODO: add proper type
		&sema.IntType{},
	},
	// nothing
	ReturnType: &sema.VoidType{},
}

// TODO: improve types
var getValueFunctionType = sema.FunctionType{
	ParameterTypes: []sema.Type{
		// owner
		&sema.VariableSizedType{
			Type: &sema.IntType{},
		},
		// controller
		&sema.VariableSizedType{
			Type: &sema.IntType{},
		},
		// key
		&sema.VariableSizedType{
			Type: &sema.IntType{},
		},
	},
	// value
	// TODO: add proper type
	ReturnType: &sema.IntType{},
}

// TODO: improve types
var createAccountFunctionType = sema.FunctionType{
	ParameterTypes: []sema.Type{
		// key
		&sema.VariableSizedType{
			Type: &sema.IntType{},
		},
		// code
		&sema.VariableSizedType{
			Type: &sema.IntType{},
		},
	},
	// value
	// TODO: add proper type
	ReturnType: &sema.IntType{},
}

func (r *interpreterRuntime) newSetValueFunction(runtimeInterface RuntimeInterface) interpreter.HostFunctionValue {
	return interpreter.NewHostFunctionValue(
		&setValueFunctionType,
		func(_ *interpreter.Interpreter, arguments []interpreter.Value, _ ast.Position) trampoline.Trampoline {
			if len(arguments) != 4 {
				panic(fmt.Sprintf("setValue requires 4 parameters"))
			}

			owner, controller, key := r.getOwnerControllerKey(arguments)

			// TODO: only integer values supported for now. written in internal byte representation
			intValue, ok := arguments[3].(interpreter.IntValue)
			if !ok {
				panic(fmt.Sprintf("setValue requires fourth parameter to be an Int"))
			}
			value := intValue.Bytes()

			if err := runtimeInterface.SetValue(owner, controller, key, value); err != nil {
				panic(err)
			}

			result := &interpreter.VoidValue{}
			return trampoline.Done{Result: result}
		},
	)
}

func (r *interpreterRuntime) newGetValueFunction(runtimeInterface RuntimeInterface) interpreter.HostFunctionValue {
	return interpreter.NewHostFunctionValue(
		&getValueFunctionType,
		func(_ *interpreter.Interpreter, arguments []interpreter.Value, _ ast.Position) trampoline.Trampoline {
			if len(arguments) != 3 {
				panic(fmt.Sprintf("getValue requires 3 parameters"))
			}

			owner, controller, key := r.getOwnerControllerKey(arguments)

			value, err := runtimeInterface.GetValue(owner, controller, key)
			if err != nil {
				panic(err)
			}

			result := interpreter.IntValue{Int: big.NewInt(0).SetBytes(value)}
			return trampoline.Done{Result: result}
		},
	)
}

func (r *interpreterRuntime) newCreateAccountFunction(runtimeInterface RuntimeInterface) interpreter.HostFunctionValue {
	return interpreter.NewHostFunctionValue(
		&createAccountFunctionType,
		func(_ *interpreter.Interpreter, arguments []interpreter.Value, _ ast.Position) trampoline.Trampoline {
			if len(arguments) != 2 {
				panic(fmt.Sprintf("createAccount requires 2 parameters"))
			}

			publicKey, err := toByteArray(arguments[0])
			if err != nil {
				panic(fmt.Sprintf("createAccount requires the first parameter to be an array"))
			}

			code, err := toByteArray(arguments[1])
			if err != nil {
				panic(fmt.Sprintf("createAccount requires the second parameter to be an array"))
			}

			value, err := runtimeInterface.CreateAccount(publicKey, code)
			if err != nil {
				panic(err)
			}

			result := interpreter.IntValue{Int: big.NewInt(0).SetBytes(value)}
			return trampoline.Done{Result: result}
		},
	)
}

func (r *interpreterRuntime) getOwnerControllerKey(
	arguments []interpreter.Value,
) (
	controller []byte, owner []byte, key []byte,
) {
	var err error
	owner, err = toByteArray(arguments[0])
	if err != nil {
		panic(fmt.Sprintf("setValue requires the first parameter to be an array"))
	}
	controller, err = toByteArray(arguments[1])
	if err != nil {
		panic(fmt.Sprintf("setValue requires the second parameter to be an array"))
	}
	key, err = toByteArray(arguments[2])
	if err != nil {
		panic(fmt.Sprintf("setValue requires the third parameter to be an array"))
	}
	return
}

func toByteArray(value interpreter.Value) ([]byte, error) {
	array, ok := value.(interpreter.ArrayValue)
	if !ok {
		return nil, errors.New("value is not an array")
	}

	result := make([]byte, len(array))
	for i, arrayValue := range array {
		intValue, ok := arrayValue.(interpreter.IntValue)
		if !ok {
			return nil, errors.New("array value is not an Int")
		}
		// check 0 <= value < 256
		if intValue.Cmp(big.NewInt(-1)) != 1 || intValue.Cmp(big.NewInt(256)) != -1 {
			return nil, errors.New("array value is not in byte range (0-255)")
		}

		result[i] = byte(intValue.IntValue())
	}

	return result, nil
}
