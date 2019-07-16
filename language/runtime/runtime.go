package runtime

import (
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/dapperlabs/bamboo-node/language/runtime/interpreter"
	"github.com/dapperlabs/bamboo-node/language/runtime/parser"
)

type RuntimeInterface interface {
	// GetValue gets a value for the given key in the storage, controlled and owned by the given accounts
	GetValue(controller []byte, owner []byte, key []byte) (value []byte, err error)
	// SetValue sets a value for the given key in the storage, controlled and owned by the given accounts
	SetValue(controller []byte, owner []byte, key []byte, value []byte) (err error)
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

	program, errs := parser.Parse(code)
	if len(errs) > 0 {
		return nil, RuntimeError{errs}
	}

	inter := interpreter.NewInterpreter(program)
	inter.ImportFunction("getValue", r.newGetValueFunction(runtimeInterface))
	inter.ImportFunction("setValue", r.newSetValueFunction(runtimeInterface))

	err := inter.Interpret()
	if err != nil {
		return nil, RuntimeError{[]error{err}}
	}

	if _, hasMain := inter.Globals["main"]; !hasMain {
		return nil, nil
	}

	value, err := inter.Invoke("main")
	if err != nil {
		return nil, RuntimeError{[]error{err}}
	}

	return value.ToGoValue(), nil
}

// TODO: improve types
var setValueFunctionType = interpreter.FunctionType{
	ParameterTypes: []interpreter.Type{
		// controller
		&interpreter.VariableSizedType{
			Type: &interpreter.UInt8Type{},
		},
		// owner
		&interpreter.VariableSizedType{
			Type: &interpreter.UInt8Type{},
		},
		// key
		&interpreter.VariableSizedType{
			Type: &interpreter.UInt8Type{},
		},
		// value
		// TODO: add proper type
		&interpreter.IntType{},
	},
	// nothing
	ReturnType: &interpreter.VoidType{},
}

// TODO: improve types
var getValueFunctionType = interpreter.FunctionType{
	ParameterTypes: []interpreter.Type{
		// controller
		&interpreter.VariableSizedType{
			Type: &interpreter.UInt8Type{},
		},
		// owner
		&interpreter.VariableSizedType{
			Type: &interpreter.UInt8Type{},
		},
		// key
		&interpreter.VariableSizedType{
			Type: &interpreter.UInt8Type{},
		},
	},
	// value
	// TODO: add proper type
	ReturnType: &interpreter.IntType{},
}

func (r *interpreterRuntime) newSetValueFunction(runtimeInterface RuntimeInterface) *interpreter.HostFunctionValue {
	return interpreter.NewHostFunction(
		&setValueFunctionType,
		func(_ *interpreter.Interpreter, arguments []interpreter.Value) interpreter.Value {
			if len(arguments) != 4 {
				panic(fmt.Sprintf("setValue requires 4 parameters"))
			}

			controller, owner, key := r.getControllerOwnerKey(arguments)

			// TODO: only integer values supported for now. written in internal byte representation
			intValue, ok := arguments[3].(interpreter.IntValue)
			if !ok {
				panic(fmt.Sprintf("setValue requires fourth parameter to be an Int"))
			}
			value := intValue.Bytes()

			if err := runtimeInterface.SetValue(controller, owner, key, value); err != nil {
				panic(err)
			}

			return &interpreter.VoidValue{}
		},
	)
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

func (r *interpreterRuntime) newGetValueFunction(runtimeInterface RuntimeInterface) *interpreter.HostFunctionValue {
	return interpreter.NewHostFunction(
		&getValueFunctionType,
		func(_ *interpreter.Interpreter, arguments []interpreter.Value) interpreter.Value {
			if len(arguments) != 3 {
				panic(fmt.Sprintf("getValue requires 3 parameters"))
			}

			controller, owner, key := r.getControllerOwnerKey(arguments)

			value, err := runtimeInterface.GetValue(controller, owner, key)
			if err != nil {
				panic(err)
			}

			return interpreter.IntValue{Int: big.NewInt(0).SetBytes(value)}
		},
	)
}

func (r *interpreterRuntime) getControllerOwnerKey(
	arguments []interpreter.Value,
) (
	controller []byte, owner []byte, key []byte,
) {
	var err error
	controller, err = toByteArray(arguments[0])
	if err != nil {
		panic(fmt.Sprintf("setValue requires the first parameter to be an array"))
	}
	owner, err = toByteArray(arguments[1])
	if err != nil {
		panic(fmt.Sprintf("setValue requires the second parameter to be an array"))
	}
	key, err = toByteArray(arguments[2])
	if err != nil {
		panic(fmt.Sprintf("setValue requires the third parameter to be an array"))
	}
	return
}
