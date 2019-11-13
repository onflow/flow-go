package runtime

import (
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/dapperlabs/flow-go/language/runtime/ast"
	runtimeErrors "github.com/dapperlabs/flow-go/language/runtime/errors"
	"github.com/dapperlabs/flow-go/language/runtime/interpreter"
	"github.com/dapperlabs/flow-go/language/runtime/parser"
	"github.com/dapperlabs/flow-go/language/runtime/sema"
	"github.com/dapperlabs/flow-go/language/runtime/stdlib"
	"github.com/dapperlabs/flow-go/language/runtime/trampoline"
	"github.com/dapperlabs/flow-go/model/flow"
)

type Interface interface {
	// ResolveImport resolves an import of a program.
	ResolveImport(Location) ([]byte, error)
	// GetValue gets a value for the given key in the storage, controlled and owned by the given accounts.
	GetValue(owner, controller, key []byte) (value []byte, err error)
	// SetValue sets a value for the given key in the storage, controlled and owned by the given accounts.
	SetValue(owner, controller, key, value []byte) (err error)
	// CreateAccount creates a new account with the given public keys and code.
	CreateAccount(publicKeys [][]byte, code []byte) (address flow.Address, err error)
	// AddAccountKey appends a key to an account.
	AddAccountKey(address flow.Address, publicKey []byte) error
	// RemoveAccountKey removes a key from an account by index.
	RemoveAccountKey(address flow.Address, index int) (publicKey []byte, err error)
	// UpdateAccountCode updates the code associated with an account.
	UpdateAccountCode(address flow.Address, code []byte) (err error)
	// GetSigningAccounts returns the signing accounts.
	GetSigningAccounts() []flow.Address
	// Log logs a string.
	Log(string)
	// EmitEvent is called when an event is emitted by the runtime.
	EmitEvent(flow.Event)
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
	//
	// This function returns an error if the program has errors (e.g syntax errors, type errors),
	// or if the execution fails.
	ExecuteScript(script []byte, runtimeInterface Interface, location Location) (interface{}, error)

	// ParseAndCheckProgram parses and checks the given code without executing the program.
	//
	// This function returns an error if the program contains any syntax or semantic errors.
	ParseAndCheckProgram(code []byte, runtimeInterface Interface, location Location) error
}

// mockRuntime is a mocked version of the Flow runtime
type mockRuntime struct{}

// NewMockRuntime returns a mocked version of the Flow runtime.
func NewMockRuntime() Runtime {
	return &mockRuntime{}
}

func (r *mockRuntime) ExecuteScript(script []byte, runtimeInterface Interface, location Location) (interface{}, error) {
	return nil, nil
}

func (r *mockRuntime) ParseAndCheckProgram(code []byte, runtimeInterface Interface, location Location) error {
	return nil
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
		// publicKeys
		&sema.VariableSizedType{
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
var addAccountKeyFunctionType = sema.FunctionType{
	ParameterTypeAnnotations: sema.NewTypeAnnotations(
		// address
		&sema.StringType{},
		// key
		&sema.VariableSizedType{
			Type: &sema.IntType{},
		},
	),
	// nothing
	ReturnTypeAnnotation: sema.NewTypeAnnotation(
		&sema.VoidType{},
	),
}

// TODO: improve types
var removeAccountKeyFunctionType = sema.FunctionType{
	ParameterTypeAnnotations: sema.NewTypeAnnotations(
		// address
		&sema.StringType{},
		// index
		&sema.IntType{},
	),
	// nothing
	ReturnTypeAnnotation: sema.NewTypeAnnotation(
		&sema.VoidType{},
	),
}

// TODO: improve types
var updateAccountCodeFunctionType = sema.FunctionType{
	ParameterTypeAnnotations: sema.NewTypeAnnotations(
		// address
		&sema.StringType{},
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

// built-in event types

var accountCreatedEventType = sema.EventType{
	Identifier: "AccountCreated",
	Fields: []sema.EventFieldType{
		{
			Identifier: "address",
			Type:       &sema.StringType{},
		},
	},
	ConstructorParameterTypeAnnotations: []*sema.TypeAnnotation{
		{
			Move: false,
			Type: &sema.StringType{},
		},
	},
}

var accountKeyAddedEventType = sema.EventType{
	Identifier: "AccountKeyAdded",
	Fields: []sema.EventFieldType{
		{
			Identifier: "address",
			Type:       &sema.StringType{},
		},
		{
			Identifier: "publicKey",
			Type: &sema.VariableSizedType{
				Type: &sema.IntType{},
			},
		},
	},
	ConstructorParameterTypeAnnotations: []*sema.TypeAnnotation{
		{
			Move: false,
			Type: &sema.StringType{},
		},
		{
			Move: false,
			Type: &sema.VariableSizedType{
				Type: &sema.IntType{},
			},
		},
	},
}

var accountKeyRemovedEventType = sema.EventType{
	Identifier: "AccountKeyRemoved",
	Fields: []sema.EventFieldType{
		{
			Identifier: "address",
			Type:       &sema.StringType{},
		},
		{
			Identifier: "publicKey",
			Type: &sema.VariableSizedType{
				Type: &sema.IntType{},
			},
		},
	},
	ConstructorParameterTypeAnnotations: []*sema.TypeAnnotation{
		{
			Move: false,
			Type: &sema.StringType{},
		},
		{
			Move: false,
			Type: &sema.VariableSizedType{
				Type: &sema.IntType{},
			},
		},
	},
}

var accountCodeUpdatedEventType = sema.EventType{
	Identifier: "AccountCodeUpdated",
	Fields: []sema.EventFieldType{
		{
			Identifier: "address",
			Type:       &sema.StringType{},
		},
		{
			Identifier: "codeHash",
			Type: &sema.VariableSizedType{
				Type: &sema.IntType{},
			},
		},
	},
	ConstructorParameterTypeAnnotations: []*sema.TypeAnnotation{
		{
			Move: false,
			Type: &sema.StringType{},
		},
		{
			Move: false,
			Type: &sema.VariableSizedType{
				Type: &sema.IntType{},
			},
		},
	},
}

var typeDeclarations = stdlib.BuiltinTypes.ToTypeDeclarations()

func (r *interpreterRuntime) parse(script []byte) (program *ast.Program, err error) {
	program, _, err = parser.ParseProgram(string(script))
	return
}

type ImportResolver = func(astLocation ast.Location) (program *ast.Program, e error)

func (r *interpreterRuntime) importResolver(runtimeInterface Interface) ImportResolver {
	return func(astLocation ast.Location) (program *ast.Program, e error) {
		var location Location
		switch astLocation := astLocation.(type) {
		case ast.StringLocation:
			location = StringLocation(astLocation)
		case ast.AddressLocation:
			location = AddressLocation(astLocation)
		default:
			panic(runtimeErrors.NewUnreachableError())
		}
		script, err := runtimeInterface.ResolveImport(location)
		if err != nil {
			return nil, err
		}
		return r.parse(script)
	}
}

// emitEvent converts an event value to native Go types and emits it to the runtime interface.
func (r *interpreterRuntime) emitEvent(eventValue interpreter.EventValue, runtimeInterface Interface) {
	values := make(map[string]interface{})

	for _, field := range eventValue.Fields {
		value := field.Value.(interpreter.ExportableValue)
		values[field.Identifier] = value.ToGoValue()
	}

	var eventTypeID string

	switch location := eventValue.Location.(type) {
	case ast.AddressLocation:
		eventTypeID = fmt.Sprintf("account.%s.%s", location, eventValue.ID)
	case TransactionLocation:
		eventTypeID = fmt.Sprintf("tx.%s.%s", location, eventValue.ID)
	case ScriptLocation:
		eventTypeID = fmt.Sprintf("script.%s.%s", location, eventValue.ID)
	default:
		panic(fmt.Sprintf("event definition from unsupported location: %s", location))
	}

	event := flow.Event{
		Type:   eventTypeID,
		Values: values,
	}

	runtimeInterface.EmitEvent(event)
}

func (r *interpreterRuntime) emitAccountEvent(
	eventType sema.EventType,
	runtimeInterface Interface,
	values ...interface{},
) {
	eventTypeID := fmt.Sprintf("flow.%s", eventType.Identifier)

	valueMap := make(map[string]interface{})

	for i, value := range values {
		field := eventType.Fields[i]
		valueMap[field.Identifier] = value
	}

	event := flow.Event{
		Type:   eventTypeID,
		Values: valueMap,
	}

	runtimeInterface.EmitEvent(event)
}

func (r *interpreterRuntime) ExecuteScript(script []byte, runtimeInterface Interface, location Location) (interface{}, error) {
	return r.executeScript(script, runtimeInterface, location)
}

func (r *interpreterRuntime) ParseAndCheckProgram(script []byte, runtimeInterface Interface, location Location) error {
	functions := r.standardLibraryFunctions(runtimeInterface)

	_, err := r.parseAndCheckProgram(script, runtimeInterface, location, functions)
	return err
}

func (r *interpreterRuntime) parseAndCheckProgram(
	script []byte,
	runtimeInterface Interface,
	location Location,
	functions stdlib.StandardLibraryFunctions,
) (*sema.Checker, error) {
	program, err := r.parse(script)
	if err != nil {
		return nil, err
	}

	importResolver := r.importResolver(runtimeInterface)
	err = program.ResolveImports(importResolver)
	if err != nil {
		return nil, err
	}

	valueDeclarations := functions.ToValueDeclarations()

	checker, err := sema.NewChecker(
		program,
		location,
		sema.WithPredeclaredValues(valueDeclarations),
		sema.WithPredeclaredTypes(typeDeclarations),
	)
	if err != nil {
		return nil, Error{[]error{err}}
	}

	if err := checker.Check(); err != nil {
		return nil, Error{[]error{err}}
	}

	return checker, nil
}

func (r *interpreterRuntime) executeScript(
	script []byte,
	runtimeInterface Interface,
	location Location,
) (interface{}, error) {
	functions := r.standardLibraryFunctions(runtimeInterface)

	checker, err := r.parseAndCheckProgram(script, runtimeInterface, location, functions)
	if err != nil {
		return nil, err
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

	interpreterRuntimeStorage := newInterpreterRuntimeStorage(runtimeInterface)

	inter, err := interpreter.NewInterpreter(
		checker,
		interpreter.WithPredefinedValues(functions.ToValues()),
		interpreter.WithOnEventEmittedHandler(func(_ *interpreter.Interpreter, eventValue interpreter.EventValue) {
			r.emitEvent(eventValue, runtimeInterface)
		}),
		interpreter.WithStorageReadHandler(interpreterRuntimeStorage.readValue),
		interpreter.WithStorageWriteHandler(interpreterRuntimeStorage.writeValue),
		interpreter.WithStorageKeyHandlerFunc(func(_ *interpreter.Interpreter, _ string, indexingType sema.Type) string {
			return indexingType.String()
		}),
	)
	if err != nil {
		return nil, Error{[]error{err}}
	}

	if err := inter.Interpret(); err != nil {
		return nil, Error{[]error{err}}
	}

	signingAccounts := make([]interface{}, signingAccountsCount)

	for i, address := range signingAccountAddresses {
		signingAccounts[i] = accountValue(address)
	}

	value, err := inter.InvokeExportable("main", signingAccounts...)
	if err != nil {
		return nil, Error{[]error{err}}
	}

	// Write back all stored values, which were actually just cached, back into storage
	interpreterRuntimeStorage.writeCached()

	return value.ToGoValue(), nil
}

func (r *interpreterRuntime) standardLibraryFunctions(runtimeInterface Interface) stdlib.StandardLibraryFunctions {
	return append(
		stdlib.BuiltinFunctions,
		stdlib.NewStandardLibraryFunction(
			"createAccount",
			&createAccountFunctionType,
			r.newCreateAccountFunction(runtimeInterface),
			nil,
		),
		stdlib.NewStandardLibraryFunction(
			"addAccountKey",
			&addAccountKeyFunctionType,
			r.addAccountKeyFunction(runtimeInterface),
			nil,
		),
		stdlib.NewStandardLibraryFunction(
			"removeAccountKey",
			&removeAccountKeyFunctionType,
			r.removeAccountKeyFunction(runtimeInterface),
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

func accountValue(address flow.Address) interpreter.Value {
	return interpreter.CompositeValue{
		Identifier: stdlib.AccountType.Name,
		Fields: &map[string]interpreter.Value{
			"address": interpreter.NewStringValue(address.String()),
			"storage": interpreter.StorageValue{Identifier: address.String()},
		},
	}
}

func (r *interpreterRuntime) newCreateAccountFunction(runtimeInterface Interface) interpreter.HostFunction {
	return func(arguments []interpreter.Value, _ interpreter.LocationPosition) trampoline.Trampoline {
		pkArray, ok := arguments[0].(interpreter.ArrayValue)
		if !ok {
			panic(fmt.Sprintf("createAccount requires the first parameter to be an array"))
		}

		pkValues := *pkArray.Values
		publicKeys := make([][]byte, len(pkValues))

		for i, pkVal := range pkValues {
			publicKey, err := toByteArray(pkVal)
			if err != nil {
				panic(fmt.Sprintf("createAccount requires the first parameter to be an array of arrays"))
			}
			publicKeys[i] = publicKey
		}

		code, err := toByteArray(arguments[1])
		if err != nil {
			panic(fmt.Sprintf("createAccount requires the third parameter to be an array"))
		}

		accountAddress, err := runtimeInterface.CreateAccount(publicKeys, code)
		if err != nil {
			panic(err)
		}

		r.emitAccountEvent(accountCreatedEventType, runtimeInterface, accountAddress)

		accountID := accountAddress.Bytes()

		result := interpreter.IntValue{Int: big.NewInt(0).SetBytes(accountID)}
		return trampoline.Done{Result: result}
	}
}

func (r *interpreterRuntime) addAccountKeyFunction(runtimeInterface Interface) interpreter.HostFunction {
	return func(arguments []interpreter.Value, _ interpreter.LocationPosition) trampoline.Trampoline {
		if len(arguments) != 2 {
			panic(fmt.Sprintf("addAccountKey requires 2 parameters"))
		}

		accountAddressStr, ok := arguments[0].(interpreter.StringValue)
		if !ok {
			panic(fmt.Sprintf("addAccountKey requires the first parameter to be a string"))
		}

		publicKey, err := toByteArray(arguments[1])
		if err != nil {
			panic(fmt.Sprintf("addAccountKey requires the second parameter to be an array"))
		}

		accountAddress := flow.HexToAddress(accountAddressStr.StrValue())

		err = runtimeInterface.AddAccountKey(accountAddress, publicKey)
		if err != nil {
			panic(err)
		}

		r.emitAccountEvent(accountKeyAddedEventType, runtimeInterface, accountAddress, publicKey)

		result := &interpreter.VoidValue{}
		return trampoline.Done{Result: result}
	}
}

func (r *interpreterRuntime) removeAccountKeyFunction(runtimeInterface Interface) interpreter.HostFunction {
	return func(arguments []interpreter.Value, _ interpreter.LocationPosition) trampoline.Trampoline {
		if len(arguments) != 2 {
			panic(fmt.Sprintf("removeAccountKey requires 2 parameters"))
		}

		accountAddressStr, ok := arguments[0].(interpreter.StringValue)
		if !ok {
			panic(fmt.Sprintf("removeAccountKey requires the first parameter to be a string"))
		}

		index, ok := arguments[1].(interpreter.IntValue)
		if !ok {
			panic(fmt.Sprintf("removeAccountKey requires the second parameter to be an integer"))

		}

		accountAddress := flow.HexToAddress(accountAddressStr.StrValue())

		publicKey, err := runtimeInterface.RemoveAccountKey(accountAddress, index.IntValue())
		if err != nil {
			panic(err)
		}

		r.emitAccountEvent(accountKeyRemovedEventType, runtimeInterface, accountAddress, publicKey)

		result := &interpreter.VoidValue{}
		return trampoline.Done{Result: result}
	}
}

func (r *interpreterRuntime) newUpdateAccountCodeFunction(runtimeInterface Interface) interpreter.HostFunction {
	return func(arguments []interpreter.Value, _ interpreter.LocationPosition) trampoline.Trampoline {
		if len(arguments) != 2 {
			panic(fmt.Sprintf("updateAccountCode requires 2 parameters"))
		}

		accountAddressStr, ok := arguments[0].(interpreter.StringValue)
		if !ok {
			panic(fmt.Sprintf("updateAccountCode requires the first parameter to be a string"))
		}

		code, err := toByteArray(arguments[1])
		if err != nil {
			panic(fmt.Sprintf("updateAccountCode requires the second parameter to be an array"))
		}

		accountAddress := flow.HexToAddress(accountAddressStr.StrValue())

		err = runtimeInterface.UpdateAccountCode(accountAddress, code)
		if err != nil {
			panic(err)
		}

		r.emitAccountEvent(accountCodeUpdatedEventType, runtimeInterface, accountAddress, code)

		result := &interpreter.VoidValue{}
		return trampoline.Done{Result: result}
	}
}

func (r *interpreterRuntime) newGetAccountFunction(runtimeInterface Interface) interpreter.HostFunction {
	return func(arguments []interpreter.Value, _ interpreter.LocationPosition) trampoline.Trampoline {
		if len(arguments) != 1 {
			panic(fmt.Sprintf("getAccount requires 1 parameter"))
		}

		stringValue, ok := arguments[0].(interpreter.StringValue)
		if !ok {
			panic(fmt.Sprintf("getAccount requires the first parameter to be an array"))
		}

		address := flow.HexToAddress(stringValue.StrValue())
		account := accountValue(address)

		return trampoline.Done{Result: account}
	}
}

func (r *interpreterRuntime) newLogFunction(runtimeInterface Interface) interpreter.HostFunction {
	return func(arguments []interpreter.Value, _ interpreter.LocationPosition) trampoline.Trampoline {
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
	intArray, err := toIntArray(value)
	if err != nil {
		return nil, err
	}

	byteArray := make([]byte, len(intArray))

	for i, intValue := range intArray {
		// check 0 <= value < 256
		if !(0 <= intValue && intValue < 256) {
			return nil, errors.New("array value is not in byte range (0-255)")
		}

		byteArray[i] = byte(intValue)
	}

	return byteArray, nil
}

func toIntArray(value interpreter.Value) ([]int, error) {
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

	result := make([]int, len(*array.Values))
	for i, arrayValue := range *array.Values {
		intValue, ok := arrayValue.(interpreter.IntValue)
		if !ok {
			return nil, errors.New("array value is not an Int")
		}

		result[i] = intValue.IntValue()
	}

	return result, nil
}
