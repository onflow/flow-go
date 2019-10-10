package runtime

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
	runtimeErrors "github.com/dapperlabs/flow-go/pkg/language/runtime/errors"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/interpreter"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/parser"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/stdlib"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/trampoline"
	"github.com/dapperlabs/flow-go/pkg/types"
)

type ImportLocation interface {
	isImportLocation()
}

type StringImportLocation ast.StringImportLocation

func (StringImportLocation) isImportLocation() {}

type AddressImportLocation ast.AddressImportLocation

func (AddressImportLocation) isImportLocation() {}

type Interface interface {
	// ResolveImport resolves an import of a program.
	ResolveImport(ImportLocation) ([]byte, error)
	// GetValue gets a value for the given key in the storage, controlled and owned by the given accounts.
	GetValue(owner, controller, key []byte) (value []byte, err error)
	// SetValue sets a value for the given key in the storage, controlled and owned by the given accounts.
	SetValue(owner, controller, key, value []byte) (err error)
	// CreateAccount creates a new account with the given public key and code.
	CreateAccount(publicKey []byte, code []byte) (accountID []byte, err error)
	// UpdateAccountCode updates the code associated with an account.
	UpdateAccountCode(accountID, code []byte) (err error)
	// GetSigningAccounts returns the signing accounts.
	GetSigningAccounts() []types.Address
	// Log logs a string.
	Log(string)
}

type Error struct {
	Errors []error
}

func (e Error) Error() string {
	var sb strings.Builder
	sb.WriteString("Execution failed:\n")
	for _, err := range e.Errors {
		sb.WriteString(runtimeErrors.UnrollChildErrors(err))
		sb.WriteString("\n")
	}
	return sb.String()
}

// Runtime is a runtime capable of executing the Flow programming language.
type Runtime interface {
	// ExecuteScript executes the given script.
	// It returns errors if the program has errors (e.g syntax errors, type errors),
	// and if the execution fails.
	ExecuteScript(script []byte, runtimeInterface Interface) (interface{}, error)
}

// mockRuntime is a mocked version of the Flow runtime
type mockRuntime struct{}

// NewMockRuntime returns a mocked version of the Flow runtime.
func NewMockRuntime() Runtime {
	return &mockRuntime{}
}

func (r *mockRuntime) ExecuteScript(script []byte, runtimeInterface Interface) (interface{}, error) {
	return nil, nil
}

// interpreterRuntime is a interpreter-based version of the Flow runtime.
type interpreterRuntime struct {
}

// NewInterpreterRuntime returns a interpreter-based version of the Flow runtime.
func NewInterpreterRuntime() Runtime {
	return &interpreterRuntime{}
}

// TODO: improve types
var setValueFunctionType = sema.FunctionType{
	ParameterTypeAnnotations: sema.NewTypeAnnotations(
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
	),
	// nothing
	ReturnTypeAnnotation: sema.NewTypeAnnotation(
		&sema.VoidType{},
	),
}

// TODO: improve types
var getValueFunctionType = sema.FunctionType{
	ParameterTypeAnnotations: sema.NewTypeAnnotations(
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
	),
	// value
	// TODO: add proper type
	ReturnTypeAnnotation: sema.NewTypeAnnotation(
		&sema.IntType{},
	),
}

// TODO: improve types
var createAccountFunctionType = sema.FunctionType{
	ParameterTypeAnnotations: sema.NewTypeAnnotations(
		// key
		&sema.OptionalType{
			Type: &sema.VariableSizedType{
				Type: &sema.IntType{},
			},
		},
		// code
		&sema.OptionalType{
			Type: &sema.VariableSizedType{
				Type: &sema.IntType{},
			},
		},
	),
	// value
	// TODO: add proper type
	ReturnTypeAnnotation: sema.NewTypeAnnotation(
		&sema.IntType{},
	),
}

// TODO: improve types
var updateAccountCodeFunctionType = sema.FunctionType{
	ParameterTypeAnnotations: sema.NewTypeAnnotations(
		// accountID
		&sema.VariableSizedType{
			Type: &sema.IntType{},
		},
		// code
		&sema.VariableSizedType{
			Type: &sema.IntType{},
		},
	),
	// nothing
	ReturnTypeAnnotation: sema.NewTypeAnnotation(
		&sema.VoidType{},
	),
}

var accountType = stdlib.AccountType.Type

var getAccountFunctionType = sema.FunctionType{
	ParameterTypeAnnotations: sema.NewTypeAnnotations(
		// TODO:
		// address
		&sema.StringType{},
	),
	ReturnTypeAnnotation: sema.NewTypeAnnotation(
		accountType,
	),
}

var logFunctionType = sema.FunctionType{
	ParameterTypeAnnotations: sema.NewTypeAnnotations(
		&sema.AnyType{},
	),
	ReturnTypeAnnotation: sema.NewTypeAnnotation(
		&sema.VoidType{},
	),
}

var typeDeclarations = stdlib.BuiltinTypes.ToTypeDeclarations()

func (r *interpreterRuntime) parse(script []byte, runtimeInterface Interface) (program *ast.Program, err error) {
	program, _, err = parser.ParseProgram(string(script))
	return
}

type Resolver = func(astLocation ast.ImportLocation) (program *ast.Program, e error)

func (r *interpreterRuntime) importResolver(runtimeInterface Interface) Resolver {
	return func(astLocation ast.ImportLocation) (program *ast.Program, e error) {
		var location ImportLocation
		switch astLocation := astLocation.(type) {
		case ast.StringImportLocation:
			location = StringImportLocation(astLocation)
		case ast.AddressImportLocation:
			location = AddressImportLocation(astLocation)
		default:
			panic(runtimeErrors.UnreachableError{})
		}
		script, err := runtimeInterface.ResolveImport(location)
		if err != nil {
			return nil, err
		}
		return r.parse(script, runtimeInterface)
	}
}

func (r *interpreterRuntime) ExecuteScript(script []byte, runtimeInterface Interface) (interface{}, error) {
	program, err := r.parse(script, runtimeInterface)
	if err != nil {
		return nil, err
	}

	importResolver := r.importResolver(runtimeInterface)
	err = program.ResolveImports(importResolver)
	if err != nil {
		return nil, err
	}

	functions := r.standardLibraryFunctions(runtimeInterface)

	valueDeclarations := functions.ToValueDeclarations()

	checker, err := sema.NewChecker(program, valueDeclarations, typeDeclarations)
	if err != nil {
		return nil, Error{[]error{err}}
	}

	if err := checker.Check(); err != nil {
		return nil, Error{[]error{err}}
	}

	main, ok := checker.GlobalValues["main"]
	if !ok {
		// TODO: error because no main?
		return nil, nil
	}

	mainFunctionType, ok := main.Type.(*sema.FunctionType)
	if !ok {
		err := errors.New("`main` is not a function")
		return nil, Error{[]error{err}}
	}

	signingAccountAddresses := runtimeInterface.GetSigningAccounts()

	// check parameter count

	signingAccountsCount := len(signingAccountAddresses)
	mainFunctionParameterCount := len(mainFunctionType.ParameterTypeAnnotations)
	if signingAccountsCount != mainFunctionParameterCount {
		err := fmt.Errorf(
			"parameter count mismatch for `main` function: expected %d, got %d",
			signingAccountsCount,
			mainFunctionParameterCount,
		)
		return nil, Error{[]error{err}}
	}

	// check parameter types

	for _, parameterTypeAnnotation := range mainFunctionType.ParameterTypeAnnotations {
		parameterType := parameterTypeAnnotation.Type

		if !parameterType.Equal(accountType) {
			err := fmt.Errorf(
				"parameter type mismatch for `main` function: expected `%s`, got `%s`",
				accountType,
				parameterType,
			)
			return nil, Error{[]error{err}}
		}
	}

	inter, err := interpreter.NewInterpreter(checker, functions.ToValues())
	if err != nil {
		return nil, Error{[]error{err}}
	}

	if err := inter.Interpret(); err != nil {
		return nil, Error{[]error{err}}
	}

	signingAccounts := make([]interface{}, signingAccountsCount)
	storedValues := make([]map[string]interpreter.Value, signingAccountsCount)

	for i, address := range signingAccountAddresses {
		signingAccount, storedValue, err := loadAccount(runtimeInterface, address)
		if err != nil {
			return nil, Error{[]error{err}}
		}

		signingAccounts[i] = signingAccount
		storedValues[i] = storedValue
	}

	value, err := inter.InvokeExportable("main", signingAccounts...)
	if err != nil {
		return nil, Error{[]error{err}}
	}

	for i, storedValue := range storedValues {
		address := signingAccountAddresses[i]

		var newStoredData bytes.Buffer
		encoder := gob.NewEncoder(&newStoredData)
		err = encoder.Encode(&storedValue)
		if err != nil {
			return nil, Error{[]error{err}}
		}

		// TODO: fix controller and key
		err := runtimeInterface.SetValue(address.Bytes(), []byte{}, []byte("storage"), newStoredData.Bytes())
		if err != nil {
			return nil, Error{[]error{err}}
		}
	}

	return value.ToGoValue(), nil
}

func (r *interpreterRuntime) standardLibraryFunctions(runtimeInterface Interface) stdlib.StandardLibraryFunctions {
	return append(
		stdlib.BuiltinFunctions,
		stdlib.NewStandardLibraryFunction(
			"getValue",
			&getValueFunctionType,
			r.newGetValueFunction(runtimeInterface),
			nil,
		),
		stdlib.NewStandardLibraryFunction(
			"setValue",
			&setValueFunctionType,
			r.newSetValueFunction(runtimeInterface),
			nil,
		),
		stdlib.NewStandardLibraryFunction(
			"createAccount",
			&createAccountFunctionType,
			r.newCreateAccountFunction(runtimeInterface),
			nil,
		),
		stdlib.NewStandardLibraryFunction(
			"updateAccountCode",
			&updateAccountCodeFunctionType,
			r.newUpdateAccountCodeFunction(runtimeInterface),
			nil,
		),
		stdlib.NewStandardLibraryFunction(
			"getAccount",
			&getAccountFunctionType,
			r.newGetAccountFunction(runtimeInterface),
			nil,
		),
		stdlib.NewStandardLibraryFunction(
			"log",
			&logFunctionType,
			r.newLogFunction(runtimeInterface),
			nil,
		),
	)
}

func loadAccount(runtimeInterface Interface, address types.Address) (
	interface{},
	map[string]interpreter.Value,
	error,
) {
	// TODO: fix controller and key
	storedData, err := runtimeInterface.GetValue(address.Bytes(), []byte{}, []byte("storage"))
	if err != nil {
		return nil, nil, Error{[]error{err}}
	}

	storedValue := map[string]interpreter.Value{}
	if len(storedData) > 0 {
		decoder := gob.NewDecoder(bytes.NewReader(storedData))
		err = decoder.Decode(&storedValue)
		if err != nil {
			return nil, nil, Error{[]error{err}}
		}
	}

	account := interpreter.CompositeValue{
		Identifier: stdlib.AccountType.Name,
		Fields: &map[string]interpreter.Value{
			"address": interpreter.NewStringValue(address.String()),
			"storage": storageValue(storedValue),
		},
	}

	return account, storedValue, nil
}

func storageValue(storedValue map[string]interpreter.Value) interpreter.StorageValue {
	return interpreter.StorageValue{
		Getter: func(key sema.Type) interpreter.Value {
			return storedValue[key.String()]
		},
		Setter: func(key sema.Type, value interpreter.Value) {
			storedValue[key.String()] = value
		},
	}
}

func (r *interpreterRuntime) newSetValueFunction(runtimeInterface Interface) interpreter.HostFunction {
	return func(arguments []interpreter.Value, _ interpreter.Location) trampoline.Trampoline {
		owner, controller, key := r.getOwnerControllerKey(arguments)

		// TODO: only integer values supported for now. written in internal byte representation
		intValue, ok := arguments[3].(interpreter.IntValue)
		if !ok {
			panic(fmt.Sprintf("setValue requires fourth parameter to be an Int"))
		}
		value := intValue.Int.Bytes()

		if err := runtimeInterface.SetValue(owner, controller, key, value); err != nil {
			panic(err)
		}

		result := &interpreter.VoidValue{}
		return trampoline.Done{Result: result}
	}
}

func (r *interpreterRuntime) newGetValueFunction(runtimeInterface Interface) interpreter.HostFunction {
	return func(arguments []interpreter.Value, _ interpreter.Location) trampoline.Trampoline {

		owner, controller, key := r.getOwnerControllerKey(arguments)

		value, err := runtimeInterface.GetValue(owner, controller, key)
		if err != nil {
			panic(err)
		}

		result := interpreter.IntValue{Int: big.NewInt(0).SetBytes(value)}
		return trampoline.Done{Result: result}
	}
}

func (r *interpreterRuntime) newCreateAccountFunction(runtimeInterface Interface) interpreter.HostFunction {
	return func(arguments []interpreter.Value, _ interpreter.Location) trampoline.Trampoline {
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
	}
}

func (r *interpreterRuntime) newUpdateAccountCodeFunction(runtimeInterface Interface) interpreter.HostFunction {
	return func(arguments []interpreter.Value, _ interpreter.Location) trampoline.Trampoline {
		if len(arguments) != 2 {
			panic(fmt.Sprintf("updateAccountCode requires 2 parameters"))
		}

		accountID, err := toByteArray(arguments[0])
		if err != nil {
			panic(fmt.Sprintf("updateAccountCode requires the first parameter to be an array"))
		}

		code, err := toByteArray(arguments[1])
		if err != nil {
			panic(fmt.Sprintf("updateAccountCode requires the second parameter to be an array"))
		}

		err = runtimeInterface.UpdateAccountCode(accountID, code)
		if err != nil {
			panic(err)
		}

		result := &interpreter.VoidValue{}
		return trampoline.Done{Result: result}
	}
}

func (r *interpreterRuntime) newGetAccountFunction(runtimeInterface Interface) interpreter.HostFunction {
	return func(arguments []interpreter.Value, _ interpreter.Location) trampoline.Trampoline {
		if len(arguments) != 1 {
			panic(fmt.Sprintf("getAccount requires 1 parameter"))
		}

		stringValue, ok := arguments[0].(interpreter.StringValue)
		if !ok {
			panic(fmt.Sprintf("getAccount requires the first parameter to be an array"))
		}

		address := types.HexToAddress(stringValue.StrValue())

		account, _, err := loadAccount(runtimeInterface, address)
		if err != nil {
			panic(err)
		}

		return trampoline.Done{Result: account}
	}
}

func (r *interpreterRuntime) newLogFunction(runtimeInterface Interface) interpreter.HostFunction {
	return func(arguments []interpreter.Value, _ interpreter.Location) trampoline.Trampoline {
		runtimeInterface.Log(fmt.Sprint(arguments[0]))
		return trampoline.Done{Result: &interpreter.VoidValue{}}
	}
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
	_, isNil := value.(interpreter.NilValue)
	if isNil {
		return nil, nil
	}

	someValue, ok := value.(interpreter.SomeValue)
	if ok {
		value = someValue.Value
	}

	array, ok := value.(interpreter.ArrayValue)
	if !ok {
		return nil, errors.New("value is not an array")
	}

	result := make([]byte, len(*array.Values))
	for i, arrayValue := range *array.Values {
		intValue, ok := arrayValue.(interpreter.IntValue)
		if !ok {
			return nil, errors.New("array value is not an Int")
		}
		// check 0 <= value < 256
		if intValue.Int.Cmp(big.NewInt(-1)) != 1 || intValue.Int.Cmp(big.NewInt(256)) != -1 {
			return nil, errors.New("array value is not in byte range (0-255)")
		}

		result[i] = byte(intValue.IntValue())
	}

	return result, nil
}
