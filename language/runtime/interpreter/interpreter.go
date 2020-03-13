package interpreter

import (
	"fmt"
	"math/big"
	goRuntime "runtime"

	"github.com/raviqqe/hamt"

	"github.com/dapperlabs/flow-go/language/runtime/activations"
	"github.com/dapperlabs/flow-go/language/runtime/ast"
	"github.com/dapperlabs/flow-go/language/runtime/common"
	"github.com/dapperlabs/flow-go/language/runtime/errors"
	"github.com/dapperlabs/flow-go/language/runtime/sema"
	// revive:disable
	. "github.com/dapperlabs/flow-go/language/runtime/trampoline"
	// revive:enable
)

type controlReturn interface {
	isControlReturn()
}

type loopBreak struct{}

func (loopBreak) isControlReturn() {}

type loopContinue struct{}

func (loopContinue) isControlReturn() {}

type functionReturn struct {
	Value
}

func (functionReturn) isControlReturn() {}

type ExpressionStatementResult struct {
	Value
}

//

var emptyFunctionType = &sema.FunctionType{
	ReturnTypeAnnotation: &sema.TypeAnnotation{
		Type: &sema.VoidType{},
	},
}

//

type getterSetter struct {
	get func() Value
	set func(Value)
}

// Visit-methods for statement which return a non-nil value
// are treated like they are returning a value.

// OnEventEmittedFunc is a function that is triggered when an event is emitted by the program.
//
type OnEventEmittedFunc func(
	inter *Interpreter,
	event *CompositeValue,
	eventType *sema.CompositeType,
)

// OnStatementFunc is a function that is triggered when a statement is about to be executed.
//
type OnStatementFunc func(
	inter *Interpreter,
	statement *Statement,
)

// StorageReadHandlerFunc is a function that handles storage reads.
//
type StorageReadHandlerFunc func(
	inter *Interpreter,
	storageAddress common.Address,
	key string,
) OptionalValue

// StorageWriteHandlerFunc is a function that handles storage writes.
//
type StorageWriteHandlerFunc func(
	inter *Interpreter,
	storageAddress common.Address,
	key string,
	value OptionalValue,
)

// StorageKeyHandlerFunc is a function that handles storage indexing types.
//
type StorageKeyHandlerFunc func(
	inter *Interpreter,
	storageAddress common.Address,
	indexingType sema.Type,
) string

// InjectedCompositeFieldsHandlerFunc is a function that handles storage reads.
//
type InjectedCompositeFieldsHandlerFunc func(
	inter *Interpreter,
	location ast.Location,
	typeID sema.TypeID,
	compositeKind common.CompositeKind,
) map[string]Value

// ContractValueHandlerFunc is a function that handles contract values.
//
type ContractValueHandlerFunc func(
	inter *Interpreter,
	compositeType *sema.CompositeType,
	constructor FunctionValue,
) *CompositeValue

// ImportProgramHandlerFunc is a function that handles imports of programs.
//
type ImportProgramHandlerFunc func(
	inter *Interpreter,
	location ast.Location,
) *ast.Program

// compositeTypeCode contains the the "prepared" / "callable" "code"
// for the functions and the destructor of a composite
// (contract, struct, resource, event).
//
// As there is no support for inheritance of concrete types,
// these are the "leaf" nodes in the call chain, and are functions.
//
type compositeTypeCode struct {
	compositeFunctions map[string]FunctionValue
	destructorFunction FunctionValue
}

type FunctionWrapper = func(inner FunctionValue) FunctionValue

// wrapperCode contains the "prepared" / "callable" "code"
// for inherited types (interfaces and type requirements).
//
// These are "branch" nodes in the call chain, and are function wrappers,
// i.e. they wrap the functions / function wrappers that inherit them.
//
type wrapperCode struct {
	initializerFunctionWrapper FunctionWrapper
	destructorFunctionWrapper  FunctionWrapper
	functionWrappers           map[string]FunctionWrapper
}

// typeCodes is the value which stores the "prepared" / "callable" "code"
// of all composite types, interface types, and type requirements.
//
type typeCodes struct {
	compositeCodes       map[sema.TypeID]compositeTypeCode
	interfaceCodes       map[sema.TypeID]wrapperCode
	typeRequirementCodes map[sema.TypeID]wrapperCode
}

type Interpreter struct {
	Checker                        *sema.Checker
	PredefinedValues               map[string]Value
	activations                    *activations.Activations
	Globals                        map[string]*Variable
	allInterpreters                map[ast.LocationID]*Interpreter
	allCheckers                    map[ast.LocationID]*sema.Checker
	typeCodes                      typeCodes
	Transactions                   []*HostFunctionValue
	onEventEmitted                 OnEventEmittedFunc
	onStatement                    OnStatementFunc
	storageReadHandler             StorageReadHandlerFunc
	storageWriteHandler            StorageWriteHandlerFunc
	storageKeyHandler              StorageKeyHandlerFunc
	injectedCompositeFieldsHandler InjectedCompositeFieldsHandlerFunc
	contractValueHandler           ContractValueHandlerFunc
	importProgramHandler           ImportProgramHandlerFunc
}

type Option func(*Interpreter) error

// WithOnEventEmittedHandler returns an interpreter option which sets
// the given function as the event handler.
//
func WithOnEventEmittedHandler(handler OnEventEmittedFunc) Option {
	return func(interpreter *Interpreter) error {
		interpreter.SetOnEventEmittedHandler(handler)
		return nil
	}
}

// WithOnStatementHandler returns an interpreter option which sets
// the given function as the statement handler.
//
func WithOnStatementHandler(handler OnStatementFunc) Option {
	return func(interpreter *Interpreter) error {
		interpreter.SetOnStatementHandler(handler)
		return nil
	}
}

// WithPredefinedValues returns an interpreter option which declares
// the given the predefined values.
//
func WithPredefinedValues(predefinedValues map[string]Value) Option {
	return func(interpreter *Interpreter) error {
		interpreter.PredefinedValues = predefinedValues

		for name, value := range predefinedValues {
			err := interpreter.ImportValue(name, value)
			if err != nil {
				return err
			}
		}

		return nil
	}
}

// WithStorageReadHandler returns an interpreter option which sets the given function
// as the function that is used when a stored value is read.
//
func WithStorageReadHandler(handler StorageReadHandlerFunc) Option {
	return func(interpreter *Interpreter) error {
		interpreter.SetStorageReadHandler(handler)
		return nil
	}
}

// WithStorageWriteHandler returns an interpreter option which sets the given function
// as the function that is used when a stored value is written.
//
func WithStorageWriteHandler(handler StorageWriteHandlerFunc) Option {
	return func(interpreter *Interpreter) error {
		interpreter.SetStorageWriteHandler(handler)
		return nil
	}
}

// WithStorageKeyHandler returns an interpreter option which sets the given function
// as the function that is used when a stored value is written.
//
func WithStorageKeyHandler(handler StorageKeyHandlerFunc) Option {
	return func(interpreter *Interpreter) error {
		interpreter.SetStorageKeyHandler(handler)
		return nil
	}
}

// WithInjectedCompositeFieldsHandler returns an interpreter option which sets the given function
// as the function that is used to initialize new composite values' fields
//
func WithInjectedCompositeFieldsHandler(handler InjectedCompositeFieldsHandlerFunc) Option {
	return func(interpreter *Interpreter) error {
		interpreter.SetInjectedCompositeFieldsHandler(handler)
		return nil
	}
}

// WithContractValueHandler returns an interpreter option which sets the given function
// as the function that is used to handle imports of values.
//
func WithContractValueHandler(handler ContractValueHandlerFunc) Option {
	return func(interpreter *Interpreter) error {
		interpreter.SetContractValueHandler(handler)
		return nil
	}
}

// WithImportProgramHandler returns an interpreter option which sets the given function
// as the function that is used to handle the imports of programs.
//
func WithImportProgramHandler(handler ImportProgramHandlerFunc) Option {
	return func(interpreter *Interpreter) error {
		interpreter.SetImportProgramHandler(handler)
		return nil
	}
}

// WithAllInterpreters returns an interpreter option which sets
// the given map of interpreters as the map of all interpreters.
//
func WithAllInterpreters(allInterpreters map[ast.LocationID]*Interpreter) Option {
	return func(interpreter *Interpreter) error {
		interpreter.SetAllInterpreters(allInterpreters)
		return nil
	}
}

// WithAllCheckers returns an interpreter option which sets
// the given map of checkers as the map of all checkers.
//
func WithAllCheckers(allCheckers map[ast.LocationID]*sema.Checker) Option {
	return func(interpreter *Interpreter) error {
		interpreter.SetAllCheckers(allCheckers)
		return nil
	}
}

// withTypeCodes returns an interpreter option which sets the type codes.
//
func withTypeCodes(typeCodes typeCodes) Option {
	return func(interpreter *Interpreter) error {
		interpreter.setTypeCodes(typeCodes)
		return nil
	}
}

func NewInterpreter(checker *sema.Checker, options ...Option) (*Interpreter, error) {
	interpreter := &Interpreter{
		Checker:     checker,
		activations: &activations.Activations{},
		Globals:     map[string]*Variable{},
	}

	defaultOptions := []Option{
		WithAllInterpreters(map[ast.LocationID]*Interpreter{}),
		WithAllCheckers(map[ast.LocationID]*sema.Checker{}),
		withTypeCodes(typeCodes{
			compositeCodes:       map[sema.TypeID]compositeTypeCode{},
			interfaceCodes:       map[sema.TypeID]wrapperCode{},
			typeRequirementCodes: map[sema.TypeID]wrapperCode{},
		}),
	}

	interpreter.defineBaseFunctions()

	for _, option := range append(defaultOptions, options...) {
		err := option(interpreter)
		if err != nil {
			return nil, err
		}
	}

	return interpreter, nil
}

// SetOnEventEmittedHandler sets the function that is triggered when an event is emitted by the program.
//
func (interpreter *Interpreter) SetOnEventEmittedHandler(function OnEventEmittedFunc) {
	interpreter.onEventEmitted = function
}

// SetOnStatementHandler sets the function that is triggered when a statement is about to be executed.
//
func (interpreter *Interpreter) SetOnStatementHandler(function OnStatementFunc) {
	interpreter.onStatement = function
}

// SetStorageReadHandler sets the function that is used when a stored value is read.
//
func (interpreter *Interpreter) SetStorageReadHandler(function StorageReadHandlerFunc) {
	interpreter.storageReadHandler = function
}

// SetStorageWriteHandler sets the function that is used when a stored value is written.
//
func (interpreter *Interpreter) SetStorageWriteHandler(function StorageWriteHandlerFunc) {
	interpreter.storageWriteHandler = function
}

// SetStorageKeyHandler sets the function that is used when a storage is indexed.
//
func (interpreter *Interpreter) SetStorageKeyHandler(function StorageKeyHandlerFunc) {
	interpreter.storageKeyHandler = function
}

// SetInjectedCompositeFieldsHandler sets the function that is used to initialize
// new composite values' fields
//
func (interpreter *Interpreter) SetInjectedCompositeFieldsHandler(function InjectedCompositeFieldsHandlerFunc) {
	interpreter.injectedCompositeFieldsHandler = function
}

// SetContractValueHandler sets the function that is used to handle imports of values
//
func (interpreter *Interpreter) SetContractValueHandler(function ContractValueHandlerFunc) {
	interpreter.contractValueHandler = function
}

// SetImportProgramHandler sets the function that is used to handle imports of programs.
//
func (interpreter *Interpreter) SetImportProgramHandler(function ImportProgramHandlerFunc) {
	interpreter.importProgramHandler = function
}

// SetAllInterpreters sets the given map of interpreters as the map of all interpreters.
//
func (interpreter *Interpreter) SetAllInterpreters(allInterpreters map[ast.LocationID]*Interpreter) {
	interpreter.allInterpreters = allInterpreters

	// Register self
	interpreter.allInterpreters[interpreter.Checker.Location.ID()] = interpreter
}

// SetAllCheckers sets the given map of checkers as the map of all checkers.
//
func (interpreter *Interpreter) SetAllCheckers(allCheckers map[ast.LocationID]*sema.Checker) {
	interpreter.allCheckers = allCheckers

	// Register self
	checker := interpreter.Checker
	interpreter.allCheckers[checker.Location.ID()] = checker
}

// setTypeCodes sets the type codes.
//
func (interpreter *Interpreter) setTypeCodes(typeCodes typeCodes) {
	interpreter.typeCodes = typeCodes
}

// locationRange returns a new location range for the given positioned element.
//
func (interpreter *Interpreter) locationRange(hasPosition ast.HasPosition) LocationRange {
	return LocationRange{
		Location: interpreter.Checker.Location,
		Range:    ast.NewRangeFromPositioned(hasPosition),
	}
}

func (interpreter *Interpreter) findVariable(name string) *Variable {
	result := interpreter.activations.Find(name)
	if result == nil {
		return nil
	}
	return result.(*Variable)
}

func (interpreter *Interpreter) findOrDeclareVariable(name string) *Variable {
	variable := interpreter.findVariable(name)
	if variable == nil {
		variable = &Variable{}
		interpreter.setVariable(name, variable)
	}
	return variable
}

func (interpreter *Interpreter) setVariable(name string, variable *Variable) {
	interpreter.activations.Set(name, variable)
}

func (interpreter *Interpreter) Interpret() (err error) {
	// recover internal panics and return them as an error
	defer recoverErrors(func(internalErr error) {
		err = internalErr
	})

	interpreter.runAllStatements(interpreter.interpret())

	return nil
}

type Statement struct {
	Trampoline Trampoline
	Line       int
}

func (interpreter *Interpreter) runUntilNextStatement(t Trampoline) (result interface{}, statement *Statement) {
	for {
		statement := getStatement(t)

		if statement != nil {
			return nil, &Statement{
				// NOTE: resumption using outer trampoline,
				// not just inner statement trampoline
				Trampoline: t,
				Line:       statement.Line,
			}
		}

		result := t.Resume()

		if continuation, ok := result.(func() Trampoline); ok {

			t = continuation()
			continue
		}

		return result, nil
	}
}

func (interpreter *Interpreter) runAllStatements(t Trampoline) interface{} {
	for {
		result, statement := interpreter.runUntilNextStatement(t)
		if statement == nil {
			return result
		}

		if interpreter.onStatement != nil {
			interpreter.onStatement(interpreter, statement)
		}

		result = statement.Trampoline.Resume()
		if continuation, ok := result.(func() Trampoline); ok {
			t = continuation()
			continue
		}

		return result
	}
}

func getStatement(t Trampoline) *StatementTrampoline {
	switch t := t.(type) {
	case FlatMap:
		return getStatement(t.Subroutine)
	case StatementTrampoline:
		return &t
	default:
		return nil
	}
}

func (interpreter *Interpreter) interpret() Trampoline {
	return interpreter.Checker.Program.Accept(interpreter).(Trampoline)
}

func (interpreter *Interpreter) prepareInterpretation() {
	program := interpreter.Checker.Program

	// Pre-declare empty variables for all interfaces, composites, and function declarations
	for _, declaration := range program.InterfaceDeclarations() {
		interpreter.declareVariable(declaration.Identifier.Identifier, nil)
	}
	for _, declaration := range program.CompositeDeclarations() {
		interpreter.declareVariable(declaration.Identifier.Identifier, nil)
	}
	for _, declaration := range program.FunctionDeclarations() {
		interpreter.declareVariable(declaration.Identifier.Identifier, nil)
	}

	// TODO:
	// Register top-level interface declarations, as their functions' conditions
	// need to be included in conforming composites' functions
}

func (interpreter *Interpreter) visitGlobalDeclarations(declarations []ast.Declaration) Trampoline {
	count := len(declarations)

	// no declarations? stop
	if count == 0 {
		// NOTE: no result, so it does *not* act like a return-statement
		return Done{}
	}

	// interpret the first declaration, then the remaining ones
	return interpreter.visitGlobalDeclaration(declarations[0]).
		FlatMap(func(_ interface{}) Trampoline {
			return interpreter.visitGlobalDeclarations(declarations[1:])
		})
}

// visitGlobalDeclaration firsts interprets the global declaration,
// then finds the declaration and adds it to the globals
func (interpreter *Interpreter) visitGlobalDeclaration(declaration ast.Declaration) Trampoline {
	return declaration.Accept(interpreter).(Trampoline).
		Then(func(_ interface{}) {
			interpreter.declareGlobal(declaration)
		})
}

func (interpreter *Interpreter) declareGlobal(declaration ast.Declaration) {
	identifier := declaration.DeclarationIdentifier()
	if identifier == nil {
		return
	}
	name := identifier.Identifier
	// NOTE: semantic analysis already checked possible invalid redeclaration
	interpreter.Globals[name] = interpreter.findVariable(name)
}

func (interpreter *Interpreter) prepareInvokeVariable(functionName string, arguments []interface{}) (trampoline Trampoline, err error) {
	variable, ok := interpreter.Globals[functionName]
	if !ok {
		return nil, &NotDeclaredError{
			ExpectedKind: common.DeclarationKindFunction,
			Name:         functionName,
		}
	}

	variableValue := variable.Value

	functionValue, ok := variableValue.(FunctionValue)
	if !ok {
		return nil, &NotInvokableError{
			Value: variableValue,
		}
	}

	ty := interpreter.Checker.GlobalValues[functionName].Type

	invokableType, ok := ty.(sema.InvokableType)

	if !ok {
		return nil, &NotInvokableError{
			Value: variableValue,
		}
	}

	functionType := invokableType.InvocationFunctionType()

	return interpreter.prepareInvoke(functionValue, functionType, arguments)
}

func (interpreter *Interpreter) prepareInvokeTransaction(
	index int,
	arguments []interface{},
) (trampoline Trampoline, err error) {
	if index >= len(interpreter.Transactions) {
		return nil, &TransactionNotDeclaredError{Index: index}
	}

	functionValue := interpreter.Transactions[index]

	transactionType := interpreter.Checker.TransactionTypes[index]
	functionType := transactionType.EntryPointFunctionType()

	return interpreter.prepareInvoke(functionValue, functionType, arguments)
}

func (interpreter *Interpreter) prepareInvoke(
	functionValue FunctionValue,
	functionType *sema.FunctionType,
	arguments []interface{},
) (trampoline Trampoline, err error) {

	var argumentValues []Value
	argumentValues, err = ToValues(arguments)
	if err != nil {
		return nil, err
	}

	// ensures the invocation's argument count matches the function's parameter count

	parameters := functionType.Parameters
	parameterCount := len(parameters)
	argumentCount := len(argumentValues)

	if argumentCount != parameterCount {

		if functionType.RequiredArgumentCount == nil ||
			argumentCount < *functionType.RequiredArgumentCount {

			return nil, &ArgumentCountError{
				ParameterCount: parameterCount,
				ArgumentCount:  argumentCount,
			}
		}
	}

	preparedArguments := make([]Value, len(arguments))
	for i, argument := range argumentValues {
		parameterType := parameters[i].TypeAnnotation.Type
		// TODO: value type is not known, reject for now
		switch parameterType.(type) {
		case *sema.AnyStructType, *sema.AnyResourceType:
			return nil, &NotInvokableError{
				Value: functionValue,
			}
		}

		preparedArguments[i] = interpreter.convertAndBox(argument, nil, parameterType)
	}

	// NOTE: can't fill argument types, as they are unknown
	trampoline = functionValue.Invoke(Invocation{
		Arguments:   preparedArguments,
		Interpreter: interpreter,
	})

	return trampoline, nil
}

func (interpreter *Interpreter) Invoke(functionName string, arguments ...interface{}) (value Value, err error) {
	// recover internal panics and return them as an error
	defer recoverErrors(func(internalErr error) {
		err = internalErr
	})

	trampoline, err := interpreter.prepareInvokeVariable(functionName, arguments)
	if err != nil {
		return nil, err
	}
	result := interpreter.runAllStatements(trampoline)
	if result == nil {
		return nil, nil
	}
	return result.(Value), nil
}

func (interpreter *Interpreter) InvokeTransaction(index int, arguments ...interface{}) (err error) {
	// recover internal panics and return them as an error
	defer recoverErrors(func(internalErr error) {
		err = internalErr
	})

	trampoline, err := interpreter.prepareInvokeTransaction(index, arguments)
	if err != nil {
		return err
	}

	_ = interpreter.runAllStatements(trampoline)

	return nil
}

func recoverErrors(onError func(error)) {
	if r := recover(); r != nil {
		var ok bool
		// don't recover Go errors
		goErr, ok := r.(goRuntime.Error)
		if ok {
			panic(goErr)
		}

		err, ok := r.(error)
		if !ok {
			err = fmt.Errorf("%v", r)
		}

		onError(err)
	}
}

func (interpreter *Interpreter) VisitProgram(program *ast.Program) ast.Repr {
	interpreter.prepareInterpretation()

	return interpreter.visitGlobalDeclarations(program.Declarations)
}

func (interpreter *Interpreter) VisitFunctionDeclaration(declaration *ast.FunctionDeclaration) ast.Repr {

	identifier := declaration.Identifier.Identifier

	functionType := interpreter.Checker.Elaboration.FunctionDeclarationFunctionTypes[declaration]

	variable := interpreter.findOrDeclareVariable(identifier)

	// lexical scope: variables in functions are bound to what is visible at declaration time
	lexicalScope := interpreter.activations.CurrentOrNew()

	// make the function itself available inside the function
	lexicalScope = lexicalScope.Insert(common.StringEntry(identifier), variable)

	variable.Value = interpreter.functionDeclarationValue(declaration, functionType, lexicalScope)

	// NOTE: no result, so it does *not* act like a return-statement
	return Done{}
}

func (interpreter *Interpreter) functionDeclarationValue(
	declaration *ast.FunctionDeclaration,
	functionType *sema.FunctionType,
	lexicalScope hamt.Map,
) InterpretedFunctionValue {

	var preConditions ast.Conditions
	if declaration.FunctionBlock.PreConditions != nil {
		preConditions = *declaration.FunctionBlock.PreConditions
	}

	var beforeStatements []ast.Statement
	var rewrittenPostConditions ast.Conditions

	if declaration.FunctionBlock.PostConditions != nil {
		postConditionsRewrite :=
			interpreter.Checker.Elaboration.PostConditionsRewrite[declaration.FunctionBlock.PostConditions]

		rewrittenPostConditions = postConditionsRewrite.RewrittenPostConditions
		beforeStatements = postConditionsRewrite.BeforeStatements
	}

	return InterpretedFunctionValue{
		Interpreter:      interpreter,
		ParameterList:    declaration.ParameterList,
		Type:             functionType,
		Activation:       lexicalScope,
		BeforeStatements: beforeStatements,
		PreConditions:    preConditions,
		Statements:       declaration.FunctionBlock.Statements,
		PostConditions:   rewrittenPostConditions,
	}
}

// NOTE: consider using NewInterpreter if the value should be predefined in all programs
func (interpreter *Interpreter) ImportValue(name string, value Value) error {
	if _, ok := interpreter.Globals[name]; ok {
		return &RedeclarationError{
			Name: name,
		}
	}

	variable := interpreter.declareVariable(name, value)
	interpreter.Globals[name] = variable
	return nil
}

func (interpreter *Interpreter) VisitBlock(block *ast.Block) ast.Repr {
	// block scope: each block gets an activation record
	interpreter.activations.PushCurrent()

	return interpreter.visitStatements(block.Statements).
		Then(func(_ interface{}) {
			interpreter.activations.Pop()
		})
}

func (interpreter *Interpreter) visitStatements(statements []ast.Statement) Trampoline {
	count := len(statements)

	// no statements? stop
	if count == 0 {
		// NOTE: no result, so it does *not* act like a return-statement
		return Done{}
	}

	statement := statements[0]
	line := statement.StartPosition().Line

	// interpret the first statement, then the remaining ones
	return StatementTrampoline{
		F: func() Trampoline {
			return statement.Accept(interpreter).(Trampoline)
		},
		Line: line,
	}.FlatMap(func(returnValue interface{}) Trampoline {
		if _, isReturn := returnValue.(controlReturn); isReturn {
			return Done{Result: returnValue}
		}
		return interpreter.visitStatements(statements[1:])
	})
}

func (interpreter *Interpreter) VisitFunctionBlock(_ *ast.FunctionBlock) ast.Repr {
	// NOTE: see visitBlock
	panic(errors.NewUnreachableError())
}

func (interpreter *Interpreter) visitFunctionBody(
	beforeStatements []ast.Statement,
	preConditions ast.Conditions,
	body Trampoline,
	postConditions ast.Conditions,
	returnType sema.Type,
) Trampoline {

	// block scope: each function block gets an activation record
	interpreter.activations.PushCurrent()

	return interpreter.visitStatements(beforeStatements).
		FlatMap(func(_ interface{}) Trampoline {
			return interpreter.visitConditions(preConditions)
		}).
		FlatMap(func(_ interface{}) Trampoline {
			return body
		}).
		FlatMap(func(blockResult interface{}) Trampoline {
			var resultValue Value
			if _, ok := blockResult.(functionReturn); ok {
				resultValue = blockResult.(functionReturn).Value
			} else {
				resultValue = VoidValue{}
			}

			// If there is a return type, declare the constant `result`
			// which has the return value

			if _, isVoid := returnType.(*sema.VoidType); !isVoid {
				interpreter.declareVariable(sema.ResultIdentifier, resultValue)
			}

			return interpreter.visitConditions(postConditions).
				Map(func(_ interface{}) interface{} {
					return resultValue
				})
		}).
		Then(func(_ interface{}) {
			interpreter.activations.Pop()
		})
}

func (interpreter *Interpreter) visitConditions(conditions []*ast.Condition) Trampoline {
	count := len(conditions)

	// no conditions? stop
	if count == 0 {
		return Done{}
	}

	// interpret the first condition, then the remaining ones
	condition := conditions[0]
	return condition.Accept(interpreter).(Trampoline).
		FlatMap(func(value interface{}) Trampoline {
			result := value.(BoolValue)

			if !result {

				var messageTrampoline Trampoline

				if condition.Message == nil {
					messageTrampoline = Done{Result: NewStringValue("")}
				} else {
					messageTrampoline = condition.Message.Accept(interpreter).(Trampoline)
				}

				return messageTrampoline.
					Then(func(result interface{}) {
						message := result.(*StringValue).Str

						panic(&ConditionError{
							ConditionKind: condition.Kind,
							Message:       message,
							LocationRange: LocationRange{
								Location: interpreter.Checker.Location,
								Range:    ast.NewRangeFromPositioned(condition.Test),
							},
						})
					})
			}

			return interpreter.visitConditions(conditions[1:])
		})
}

func (interpreter *Interpreter) VisitCondition(condition *ast.Condition) ast.Repr {
	return condition.Test.Accept(interpreter)
}

func (interpreter *Interpreter) VisitReturnStatement(statement *ast.ReturnStatement) ast.Repr {
	// NOTE: returning result

	if statement.Expression == nil {
		return Done{Result: functionReturn{VoidValue{}}}
	}

	return statement.Expression.Accept(interpreter).(Trampoline).
		Map(func(result interface{}) interface{} {
			value := result.(Value)

			valueType := interpreter.Checker.Elaboration.ReturnStatementValueTypes[statement]
			returnType := interpreter.Checker.Elaboration.ReturnStatementReturnTypes[statement]

			value = interpreter.copyAndConvert(value, valueType, returnType)

			return functionReturn{value}
		})
}

func (interpreter *Interpreter) VisitBreakStatement(_ *ast.BreakStatement) ast.Repr {
	return Done{Result: loopBreak{}}
}

func (interpreter *Interpreter) VisitContinueStatement(_ *ast.ContinueStatement) ast.Repr {
	return Done{Result: loopContinue{}}
}

func (interpreter *Interpreter) VisitIfStatement(statement *ast.IfStatement) ast.Repr {
	switch test := statement.Test.(type) {
	case ast.Expression:
		return interpreter.visitIfStatementWithTestExpression(test, statement.Then, statement.Else)
	case *ast.VariableDeclaration:
		return interpreter.visitIfStatementWithVariableDeclaration(test, statement.Then, statement.Else)
	default:
		panic(errors.NewUnreachableError())
	}
}

func (interpreter *Interpreter) visitIfStatementWithTestExpression(
	test ast.Expression,
	thenBlock, elseBlock *ast.Block,
) Trampoline {

	return test.Accept(interpreter).(Trampoline).
		FlatMap(func(result interface{}) Trampoline {
			value := result.(BoolValue)
			if value {
				return thenBlock.Accept(interpreter).(Trampoline)
			} else if elseBlock != nil {
				return elseBlock.Accept(interpreter).(Trampoline)
			}

			// NOTE: no result, so it does *not* act like a return-statement
			return Done{}
		})
}

func (interpreter *Interpreter) visitIfStatementWithVariableDeclaration(
	declaration *ast.VariableDeclaration,
	thenBlock, elseBlock *ast.Block,
) Trampoline {

	return declaration.Value.Accept(interpreter).(Trampoline).
		FlatMap(func(result interface{}) Trampoline {

			if someValue, ok := result.(*SomeValue); ok {

				targetType := interpreter.Checker.Elaboration.VariableDeclarationTargetTypes[declaration]
				valueType := interpreter.Checker.Elaboration.VariableDeclarationValueTypes[declaration]
				unwrappedValueCopy := interpreter.copyAndConvert(someValue.Value, valueType, targetType)

				interpreter.activations.PushCurrent()
				interpreter.declareVariable(
					declaration.Identifier.Identifier,
					unwrappedValueCopy,
				)

				return thenBlock.Accept(interpreter).(Trampoline).
					Then(func(_ interface{}) {
						interpreter.activations.Pop()
					})
			} else if elseBlock != nil {
				return elseBlock.Accept(interpreter).(Trampoline)
			}

			// NOTE: ignore result, so it does *not* act like a return-statement
			return Done{}
		})
}

func (interpreter *Interpreter) VisitWhileStatement(statement *ast.WhileStatement) ast.Repr {
	return statement.Test.Accept(interpreter).(Trampoline).
		FlatMap(func(result interface{}) Trampoline {
			value := result.(BoolValue)
			if !value {
				return Done{}
			}

			return statement.Block.Accept(interpreter).(Trampoline).
				FlatMap(func(value interface{}) Trampoline {
					if _, ok := value.(loopBreak); ok {
						return Done{}
						// revive:disable:empty-block
					} else if _, ok := value.(loopContinue); ok {
						// revive:enable
						// NO-OP
					} else if functionReturn, ok := value.(functionReturn); ok {
						return Done{Result: functionReturn}
					}

					// recurse
					return statement.Accept(interpreter).(Trampoline)
				})
		})
}

func (interpreter *Interpreter) visitPotentialStorageRemoval(expression ast.Expression) Trampoline {
	movingStorageIndexExpression := interpreter.movingStorageIndexExpression(expression)
	if movingStorageIndexExpression == nil {
		return expression.Accept(interpreter).(Trampoline)
	}

	return interpreter.indexExpressionGetterSetter(movingStorageIndexExpression).
		Map(func(result interface{}) interface{} {
			getterSetter := result.(getterSetter)
			value := getterSetter.get()
			getterSetter.set(NilValue{})
			return value
		})
}

// VisitVariableDeclaration first visits the declaration's value,
// then declares the variable with the name bound to the value
func (interpreter *Interpreter) VisitVariableDeclaration(declaration *ast.VariableDeclaration) ast.Repr {

	targetType := interpreter.Checker.Elaboration.VariableDeclarationTargetTypes[declaration]
	valueType := interpreter.Checker.Elaboration.VariableDeclarationValueTypes[declaration]
	secondValueType := interpreter.Checker.Elaboration.VariableDeclarationSecondValueTypes[declaration]

	return interpreter.visitPotentialStorageRemoval(declaration.Value).
		FlatMap(func(result interface{}) Trampoline {

			valueCopy := interpreter.copyAndConvert(result.(Value), valueType, targetType)

			interpreter.declareVariable(
				declaration.Identifier.Identifier,
				valueCopy,
			)

			if declaration.SecondValue == nil {
				// NOTE: ignore result, so it does *not* act like a return-statement
				return Done{}
			}

			return interpreter.visitAssignment(
				declaration.Transfer.Operation,
				declaration.Value,
				valueType,
				declaration.SecondValue,
				secondValueType,
				declaration,
			)
		})
}

func (interpreter *Interpreter) movingStorageIndexExpression(expression ast.Expression) *ast.IndexExpression {
	indexExpression, ok := expression.(*ast.IndexExpression)
	if !ok || !interpreter.Checker.Elaboration.IsResourceMovingStorageIndexExpression[indexExpression] {
		return nil
	}

	return indexExpression
}

func (interpreter *Interpreter) declareVariable(identifier string, value Value) *Variable {
	// NOTE: semantic analysis already checked possible invalid redeclaration
	variable := &Variable{Value: value}
	interpreter.setVariable(identifier, variable)
	return variable
}

func (interpreter *Interpreter) VisitAssignmentStatement(assignment *ast.AssignmentStatement) ast.Repr {
	targetType := interpreter.Checker.Elaboration.AssignmentStatementTargetTypes[assignment]
	valueType := interpreter.Checker.Elaboration.AssignmentStatementValueTypes[assignment]

	target := assignment.Target
	value := assignment.Value

	return interpreter.visitAssignment(
		assignment.Transfer.Operation,
		target, targetType,
		value, valueType,
		assignment,
	)
}

func (interpreter *Interpreter) visitAssignment(
	transferOperation ast.TransferOperation,
	target ast.Expression, targetType sema.Type,
	value ast.Expression, valueType sema.Type,
	position ast.HasPosition,
) Trampoline {

	// First evaluate the target, which results in a getter/setter function pair
	return interpreter.assignmentGetterSetter(target).
		FlatMap(func(result interface{}) Trampoline {
			getterSetter := result.(getterSetter)

			// If the assignment is a forced move,
			// ensure that the target is nil,
			// otherwise panic

			if transferOperation == ast.TransferOperationMoveForced {
				target := getterSetter.get()
				if _, ok := target.(NilValue); !ok {
					locationRange := interpreter.locationRange(position)

					panic(&ForceAssignmentToNonNilResourceError{
						LocationRange: locationRange,
					})
				}
			}

			// Finally, evaluate the value, and assign it using the setter function
			return value.Accept(interpreter).(Trampoline).
				FlatMap(func(result interface{}) Trampoline {

					valueCopy := interpreter.copyAndConvert(result.(Value), valueType, targetType)
					getterSetter.set(valueCopy)

					// NOTE: no result, so it does *not* act like a return-statement
					return Done{}
				})
		})
}

func (interpreter *Interpreter) VisitSwapStatement(swap *ast.SwapStatement) ast.Repr {

	leftType := interpreter.Checker.Elaboration.SwapStatementLeftTypes[swap]
	rightType := interpreter.Checker.Elaboration.SwapStatementRightTypes[swap]

	// Evaluate the left expression
	return interpreter.assignmentGetterSetter(swap.Left).
		FlatMap(func(result interface{}) Trampoline {
			leftGetterSetter := result.(getterSetter)
			leftValue := leftGetterSetter.get()
			if interpreter.movingStorageIndexExpression(swap.Left) != nil {
				leftGetterSetter.set(NilValue{})
			}

			// Evaluate the right expression
			return interpreter.assignmentGetterSetter(swap.Right).
				Then(func(result interface{}) {
					rightGetterSetter := result.(getterSetter)
					rightValue := rightGetterSetter.get()
					if interpreter.movingStorageIndexExpression(swap.Right) != nil {
						rightGetterSetter.set(NilValue{})
					}

					// Add right value to left target
					// and left value to right target

					rightValueCopy := interpreter.copyAndConvert(rightValue.(Value), rightType, leftType)
					leftValueCopy := interpreter.copyAndConvert(leftValue.(Value), leftType, rightType)

					leftGetterSetter.set(rightValueCopy)
					rightGetterSetter.set(leftValueCopy)
				})
		})
}

// assignmentGetterSetter returns a getter/setter function pair
// for the target expression, wrapped in a trampoline
//
func (interpreter *Interpreter) assignmentGetterSetter(target ast.Expression) Trampoline {
	switch target := target.(type) {
	case *ast.IdentifierExpression:
		return interpreter.identifierExpressionGetterSetter(target)

	case *ast.IndexExpression:
		return interpreter.indexExpressionGetterSetter(target)

	case *ast.MemberExpression:
		return interpreter.memberExpressionGetterSetter(target)
	}

	panic(errors.NewUnreachableError())
}

// identifierExpressionGetterSetter returns a getter/setter function pair
// for the target identifier expression, wrapped in a trampoline
//
func (interpreter *Interpreter) identifierExpressionGetterSetter(identifierExpression *ast.IdentifierExpression) Trampoline {
	variable := interpreter.findVariable(identifierExpression.Identifier.Identifier)
	return Done{
		Result: getterSetter{
			get: func() Value {
				return variable.Value
			},
			set: func(value Value) {
				variable.Value = value
			},
		},
	}
}

// indexExpressionGetterSetter returns a getter/setter function pair
// for the target index expression, wrapped in a trampoline
//
func (interpreter *Interpreter) indexExpressionGetterSetter(indexExpression *ast.IndexExpression) Trampoline {
	return indexExpression.TargetExpression.Accept(interpreter).(Trampoline).
		FlatMap(func(result interface{}) Trampoline {
			switch typedResult := result.(type) {
			case ValueIndexableValue:
				return indexExpression.IndexingExpression.Accept(interpreter).(Trampoline).
					FlatMap(func(result interface{}) Trampoline {
						indexingValue := result.(Value)
						locationRange := interpreter.locationRange(indexExpression)
						return Done{
							Result: getterSetter{
								get: func() Value {
									return typedResult.Get(interpreter, locationRange, indexingValue)
								},
								set: func(value Value) {
									typedResult.Set(interpreter, locationRange, indexingValue, value)
								},
							},
						}
					})

			case StorageValue:
				return interpreter.visitStorageIndexExpression(indexExpression, typedResult.Address, AccessLevelPrivate)

			case PublishedValue:
				return interpreter.visitStorageIndexExpression(indexExpression, typedResult.Address, AccessLevelPublic)

			default:
				panic(errors.NewUnreachableError())
			}
		})
}

func (interpreter *Interpreter) visitStorageIndexExpression(
	indexExpression *ast.IndexExpression,
	storageAddress common.Address,
	accessLevel AccessLevel,
) Trampoline {
	indexingType := interpreter.Checker.Elaboration.IndexExpressionIndexingTypes[indexExpression]
	rawKey := interpreter.storageKeyHandler(interpreter, storageAddress, indexingType)
	key := PrefixedStorageKey(rawKey, accessLevel)
	return Done{
		Result: getterSetter{
			get: func() Value {
				return interpreter.readStored(storageAddress, key)
			},
			set: func(value Value) {
				interpreter.writeStored(storageAddress, key, value.(OptionalValue))
			},
		},
	}
}

func (interpreter *Interpreter) visitReadStorageIndexExpression(
	expression *ast.IndexExpression,
	storageAddress common.Address,
	accessLevel AccessLevel,
) Trampoline {
	return interpreter.visitStorageIndexExpression(expression, storageAddress, accessLevel).
		Map(func(result interface{}) interface{} {
			getterSetter := result.(getterSetter)
			return getterSetter.get()
		})
}

// memberExpressionGetterSetter returns a getter/setter function pair
// for the target member expression, wrapped in a trampoline
//
func (interpreter *Interpreter) memberExpressionGetterSetter(memberExpression *ast.MemberExpression) Trampoline {
	return memberExpression.Expression.Accept(interpreter).(Trampoline).
		FlatMap(func(result interface{}) Trampoline {
			structure := result.(MemberAccessibleValue)
			locationRange := interpreter.locationRange(memberExpression)
			identifier := memberExpression.Identifier.Identifier
			return Done{
				Result: getterSetter{
					get: func() Value {
						return structure.GetMember(interpreter, locationRange, identifier)
					},
					set: func(value Value) {
						structure.SetMember(interpreter, locationRange, identifier, value)
					},
				},
			}
		})
}

func (interpreter *Interpreter) VisitIdentifierExpression(expression *ast.IdentifierExpression) ast.Repr {
	name := expression.Identifier.Identifier
	variable := interpreter.findVariable(name)
	return Done{Result: variable.Value}
}

// valueTuple

type valueTuple struct {
	left, right Value
}

// visitBinaryOperation interprets the left-hand side and the right-hand side and returns
// the result in a valueTuple
func (interpreter *Interpreter) visitBinaryOperation(expr *ast.BinaryExpression) Trampoline {
	// interpret the left-hand side
	return expr.Left.Accept(interpreter).(Trampoline).
		FlatMap(func(left interface{}) Trampoline {
			// after interpreting the left-hand side,
			// interpret the right-hand side
			return expr.Right.Accept(interpreter).(Trampoline).
				FlatMap(func(right interface{}) Trampoline {
					tuple := valueTuple{
						left.(Value),
						right.(Value),
					}
					return Done{Result: tuple}
				})
		})
}

func (interpreter *Interpreter) VisitBinaryExpression(expression *ast.BinaryExpression) ast.Repr {
	switch expression.Operation {
	case ast.OperationPlus:
		return interpreter.visitBinaryOperation(expression).
			Map(func(result interface{}) interface{} {
				tuple := result.(valueTuple)
				left := tuple.left.(NumberValue)
				right := tuple.right.(NumberValue)
				return left.Plus(right)
			})

	case ast.OperationMinus:
		return interpreter.visitBinaryOperation(expression).
			Map(func(result interface{}) interface{} {
				tuple := result.(valueTuple)
				left := tuple.left.(NumberValue)
				right := tuple.right.(NumberValue)
				return left.Minus(right)
			})

	case ast.OperationMod:
		return interpreter.visitBinaryOperation(expression).
			Map(func(result interface{}) interface{} {
				tuple := result.(valueTuple)
				left := tuple.left.(NumberValue)
				right := tuple.right.(NumberValue)
				return left.Mod(right)
			})

	case ast.OperationMul:
		return interpreter.visitBinaryOperation(expression).
			Map(func(result interface{}) interface{} {
				tuple := result.(valueTuple)
				left := tuple.left.(NumberValue)
				right := tuple.right.(NumberValue)
				return left.Mul(right)
			})

	case ast.OperationDiv:
		return interpreter.visitBinaryOperation(expression).
			Map(func(result interface{}) interface{} {
				tuple := result.(valueTuple)
				left := tuple.left.(NumberValue)
				right := tuple.right.(NumberValue)
				return left.Div(right)
			})

	case ast.OperationLess:
		return interpreter.visitBinaryOperation(expression).
			Map(func(result interface{}) interface{} {
				tuple := result.(valueTuple)
				left := tuple.left.(NumberValue)
				right := tuple.right.(NumberValue)
				return left.Less(right)
			})

	case ast.OperationLessEqual:
		return interpreter.visitBinaryOperation(expression).
			Map(func(result interface{}) interface{} {
				tuple := result.(valueTuple)
				left := tuple.left.(NumberValue)
				right := tuple.right.(NumberValue)
				return left.LessEqual(right)
			})

	case ast.OperationGreater:
		return interpreter.visitBinaryOperation(expression).
			Map(func(result interface{}) interface{} {
				tuple := result.(valueTuple)
				left := tuple.left.(NumberValue)
				right := tuple.right.(NumberValue)
				return left.Greater(right)
			})

	case ast.OperationGreaterEqual:
		return interpreter.visitBinaryOperation(expression).
			Map(func(result interface{}) interface{} {
				tuple := result.(valueTuple)
				left := tuple.left.(NumberValue)
				right := tuple.right.(NumberValue)
				return left.GreaterEqual(right)
			})

	case ast.OperationEqual:
		return interpreter.visitBinaryOperation(expression).
			Map(func(result interface{}) interface{} {
				tuple := result.(valueTuple)
				return interpreter.testEqual(tuple.left, tuple.right)
			})

	case ast.OperationUnequal:
		return interpreter.visitBinaryOperation(expression).
			Map(func(result interface{}) interface{} {
				tuple := result.(valueTuple)
				return BoolValue(!interpreter.testEqual(tuple.left, tuple.right))
			})

	case ast.OperationOr:
		// interpret the left-hand side
		return expression.Left.Accept(interpreter).(Trampoline).
			FlatMap(func(left interface{}) Trampoline {
				// only interpret right-hand side if left-hand side is false
				leftBool := left.(BoolValue)
				if leftBool {
					return Done{Result: leftBool}
				}

				// after interpreting the left-hand side,
				// interpret the right-hand side
				return expression.Right.Accept(interpreter).(Trampoline).
					FlatMap(func(right interface{}) Trampoline {
						return Done{Result: right.(BoolValue)}
					})
			})

	case ast.OperationAnd:
		// interpret the left-hand side
		return expression.Left.Accept(interpreter).(Trampoline).
			FlatMap(func(left interface{}) Trampoline {
				// only interpret right-hand side if left-hand side is true
				leftBool := left.(BoolValue)
				if !leftBool {
					return Done{Result: leftBool}
				}

				// after interpreting the left-hand side,
				// interpret the right-hand side
				return expression.Right.Accept(interpreter).(Trampoline).
					FlatMap(func(right interface{}) Trampoline {
						return Done{Result: right.(BoolValue)}
					})
			})

	case ast.OperationNilCoalesce:
		// interpret the left-hand side
		return expression.Left.Accept(interpreter).(Trampoline).
			FlatMap(func(left interface{}) Trampoline {
				// only evaluate right-hand side if left-hand side is nil
				if _, ok := left.(NilValue); ok {
					return expression.Right.Accept(interpreter).(Trampoline).
						Map(func(result interface{}) interface{} {
							value := result.(Value)

							rightType := interpreter.Checker.Elaboration.BinaryExpressionRightTypes[expression]
							resultType := interpreter.Checker.Elaboration.BinaryExpressionResultTypes[expression]

							// NOTE: important to convert both any and optional
							return interpreter.convertAndBox(value, rightType, resultType)
						})
				}

				value := left.(*SomeValue).Value
				return Done{Result: value}
			})

	case ast.OperationConcat:
		return interpreter.visitBinaryOperation(expression).
			Map(func(result interface{}) interface{} {
				tuple := result.(valueTuple)
				left := tuple.left.(ConcatenatableValue)
				right := tuple.right.(ConcatenatableValue)
				return left.Concat(right)
			})
	}

	panic(&unsupportedOperation{
		kind:      common.OperationKindBinary,
		operation: expression.Operation,
		Range:     ast.NewRangeFromPositioned(expression),
	})
}

func (interpreter *Interpreter) testEqual(left, right Value) BoolValue {
	left = interpreter.unbox(left)
	right = interpreter.unbox(right)

	// TODO: add support for arrays and dictionaries

	switch left := left.(type) {
	case EquatableValue:
		// NOTE: might be NilValue
		right, ok := right.(EquatableValue)
		if !ok {
			return false
		}
		return left.Equal(right)

	case NilValue:
		_, ok := right.(NilValue)
		return BoolValue(ok)

	case *CompositeValue:
		// TODO: call `equals` if RHS is composite
		return false

	case *ArrayValue,
		*DictionaryValue:
		// TODO:
		return false
	}

	panic(errors.NewUnreachableError())
}

func (interpreter *Interpreter) VisitUnaryExpression(expression *ast.UnaryExpression) ast.Repr {
	return expression.Expression.Accept(interpreter).(Trampoline).
		Map(func(result interface{}) interface{} {
			value := result.(Value)

			switch expression.Operation {
			case ast.OperationNegate:
				boolValue := value.(BoolValue)
				return boolValue.Negate()

			case ast.OperationMinus:
				integerValue := value.(NumberValue)
				return integerValue.Negate()

			case ast.OperationMove:
				return value
			}

			panic(&unsupportedOperation{
				kind:      common.OperationKindUnary,
				operation: expression.Operation,
				Range: ast.Range{
					StartPos: expression.StartPos,
					EndPos:   expression.EndPos,
				},
			})
		})
}

func (interpreter *Interpreter) VisitExpressionStatement(statement *ast.ExpressionStatement) ast.Repr {
	return statement.Expression.Accept(interpreter).(Trampoline).
		Map(func(result interface{}) interface{} {
			var value Value
			var ok bool
			value, ok = result.(Value)
			if !ok {
				value = nil
			}
			return ExpressionStatementResult{value}
		})
}

func (interpreter *Interpreter) VisitBoolExpression(expression *ast.BoolExpression) ast.Repr {
	value := BoolValue(expression.Value)

	return Done{Result: value}
}

func (interpreter *Interpreter) VisitNilExpression(_ *ast.NilExpression) ast.Repr {
	value := NilValue{}
	return Done{Result: value}
}

func (interpreter *Interpreter) VisitIntegerExpression(expression *ast.IntegerExpression) ast.Repr {
	value := IntValue{expression.Value}

	return Done{Result: value}
}

func (interpreter *Interpreter) VisitFixedPointExpression(expression *ast.FixedPointExpression) ast.Repr {
	// TODO: adjust once/if we support more fixed point types
	value := interpreter.convertToFixedPointBigInt(expression, sema.Fix64Scale)
	return Done{Result: Fix64Value(value.Int64())}
}

func (interpreter *Interpreter) convertToFixedPointBigInt(expression *ast.FixedPointExpression, scale uint) *big.Int {
	ten := big.NewInt(10)

	// integer = expression.UnsignedInteger * 10 ^ scale

	targetScale := big.NewInt(0).SetUint64(uint64(scale))

	integer := big.NewInt(0).Mul(
		expression.UnsignedInteger,
		big.NewInt(0).Exp(ten, targetScale, nil),
	)

	// fractional = expression.Fractional * 10 ^ (scale - expression.Scale)

	var fractional *big.Int
	if expression.Scale == scale {
		fractional = expression.Fractional
	} else if expression.Scale < scale {
		scaleDiff := big.NewInt(0).SetUint64(uint64(scale - expression.Scale))
		fractional = big.NewInt(0).Mul(
			expression.Fractional,
			big.NewInt(0).Exp(ten, scaleDiff, nil),
		)
	} else {
		scaleDiff := big.NewInt(0).SetUint64(uint64(expression.Scale - scale))
		fractional = big.NewInt(0).Div(expression.Fractional,
			big.NewInt(0).Exp(ten, scaleDiff, nil),
		)
	}

	// value = integer + fractional

	if expression.Negative {
		integer.Neg(integer)
		fractional.Neg(fractional)
	}

	return integer.Add(integer, fractional)
}

func (interpreter *Interpreter) VisitStringExpression(expression *ast.StringExpression) ast.Repr {
	value := NewStringValue(expression.Value)

	return Done{Result: value}
}

func (interpreter *Interpreter) VisitArrayExpression(expression *ast.ArrayExpression) ast.Repr {
	return interpreter.visitExpressionsNonCopying(expression.Values).
		FlatMap(func(result interface{}) Trampoline {
			values := result.(*ArrayValue)

			argumentTypes := interpreter.Checker.Elaboration.ArrayExpressionArgumentTypes[expression]
			elementType := interpreter.Checker.Elaboration.ArrayExpressionElementType[expression]

			copies := make([]Value, len(values.Values))
			for i, argument := range values.Values {
				argumentType := argumentTypes[i]
				copies[i] = interpreter.copyAndConvert(argument, argumentType, elementType)
			}

			return Done{Result: NewArrayValueUnownedNonCopying(copies...)}
		})
}

func (interpreter *Interpreter) VisitDictionaryExpression(expression *ast.DictionaryExpression) ast.Repr {
	return interpreter.visitEntries(expression.Entries).
		FlatMap(func(result interface{}) Trampoline {

			entryTypes := interpreter.Checker.Elaboration.DictionaryExpressionEntryTypes[expression]
			dictionaryType := interpreter.Checker.Elaboration.DictionaryExpressionType[expression]

			newDictionary := NewDictionaryValueUnownedNonCopying()
			for i, dictionaryEntryValues := range result.([]DictionaryEntryValues) {
				entryType := entryTypes[i]

				key := interpreter.copyAndConvert(
					dictionaryEntryValues.Key,
					entryType.KeyType,
					dictionaryType.KeyType,
				)

				value := interpreter.copyAndConvert(
					dictionaryEntryValues.Value,
					entryType.ValueType,
					dictionaryType.ValueType,
				)

				// TODO: panic for duplicate keys?

				// NOTE: important to convert in optional, as assignment to dictionary
				// is always considered as an optional

				newDictionary.Insert(key, value)
			}

			return Done{Result: newDictionary}
		})
}

func (interpreter *Interpreter) VisitMemberExpression(expression *ast.MemberExpression) ast.Repr {
	return expression.Expression.Accept(interpreter).(Trampoline).
		Map(func(result interface{}) interface{} {
			if expression.Optional {
				switch typedResult := result.(type) {
				case NilValue:
					return typedResult

				case *SomeValue:
					result = typedResult.Value

				default:
					panic(errors.NewUnreachableError())
				}
			}

			value := result.(MemberAccessibleValue)
			locationRange := interpreter.locationRange(expression)
			resultValue := value.GetMember(interpreter, locationRange, expression.Identifier.Identifier)

			// If the member access is optional chaining, only wrap the result value
			// in an optional, if it is not already an optional value

			if expression.Optional {
				if _, ok := resultValue.(OptionalValue); !ok {
					return NewSomeValueOwningNonCopying(resultValue)
				}
			}
			return resultValue
		})
}

// PrefixedStorageKey returns the storage identifier with the proper prefix
// based on the given access level.
//
// \x1F = Information Separator One
//
func PrefixedStorageKey(key string, accessLevel AccessLevel) string {
	return fmt.Sprintf("%s\x1F%s", accessLevel.Prefix(), key)
}

func (interpreter *Interpreter) VisitIndexExpression(expression *ast.IndexExpression) ast.Repr {
	return expression.TargetExpression.Accept(interpreter).(Trampoline).
		FlatMap(func(result interface{}) Trampoline {
			switch typedResult := result.(type) {
			case ValueIndexableValue:
				return expression.IndexingExpression.Accept(interpreter).(Trampoline).
					FlatMap(func(result interface{}) Trampoline {
						indexingValue := result.(Value)
						locationRange := interpreter.locationRange(expression)
						value := typedResult.Get(interpreter, locationRange, indexingValue)
						return Done{Result: value}
					})

			case StorageValue:
				return interpreter.visitReadStorageIndexExpression(
					expression,
					typedResult.Address,
					AccessLevelPrivate,
				)

			case PublishedValue:
				return interpreter.visitReadStorageIndexExpression(
					expression,
					typedResult.Address,
					AccessLevelPublic,
				)

			default:
				panic(errors.NewUnreachableError())
			}
		})
}

func (interpreter *Interpreter) VisitConditionalExpression(expression *ast.ConditionalExpression) ast.Repr {
	return expression.Test.Accept(interpreter).(Trampoline).
		FlatMap(func(result interface{}) Trampoline {
			value := result.(BoolValue)

			if value {
				return expression.Then.Accept(interpreter).(Trampoline)
			}
			return expression.Else.Accept(interpreter).(Trampoline)
		})
}

func (interpreter *Interpreter) VisitInvocationExpression(invocationExpression *ast.InvocationExpression) ast.Repr {
	// interpret the invoked expression
	return invocationExpression.InvokedExpression.Accept(interpreter).(Trampoline).
		FlatMap(func(result interface{}) Trampoline {

			// Handle optional chaining on member expression, if any:
			// - If the member expression is nil, finish execution
			// - If the member expression is some value, the wrapped value
			//   is the function value that should be invoked

			isOptionalChaining := false

			if invokedMemberExpression, ok :=
				invocationExpression.InvokedExpression.(*ast.MemberExpression); ok && invokedMemberExpression.Optional {

				isOptionalChaining = true

				switch typedResult := result.(type) {
				case NilValue:
					return Done{Result: typedResult}

				case *SomeValue:
					result = typedResult.Value

				default:
					panic(errors.NewUnreachableError())
				}
			}

			function := result.(FunctionValue)

			// NOTE: evaluate all argument expressions in call-site scope, not in function body
			argumentExpressions := make([]ast.Expression, len(invocationExpression.Arguments))
			for i, argument := range invocationExpression.Arguments {
				argumentExpressions[i] = argument.Expression
			}

			return interpreter.visitExpressionsNonCopying(argumentExpressions).
				FlatMap(func(result interface{}) Trampoline {
					arguments := result.(*ArrayValue).Values

					argumentTypes :=
						interpreter.Checker.Elaboration.InvocationExpressionArgumentTypes[invocationExpression]
					parameterTypes :=
						interpreter.Checker.Elaboration.InvocationExpressionParameterTypes[invocationExpression]

					invocation := interpreter.functionValueInvocationTrampoline(
						function,
						arguments,
						argumentTypes,
						parameterTypes,
						invocationExpression.StartPosition(),
					)

					// If this is invocation is optional chaining, wrap the result
					// as an optional, as the result is expected to be an optional

					if !isOptionalChaining {
						return invocation
					}

					return invocation.Map(func(result interface{}) interface{} {
						return &SomeValue{Value: result.(Value)}
					})
				})
		})
}

func (interpreter *Interpreter) InvokeFunctionValue(
	function FunctionValue,
	arguments []Value,
	argumentTypes []sema.Type,
	parameterTypes []sema.Type,
	pos ast.Position,
) (value Value, err error) {
	// recover internal panics and return them as an error
	defer recoverErrors(func(internalErr error) {
		err = internalErr
	})

	trampoline := interpreter.functionValueInvocationTrampoline(
		function,
		arguments,
		argumentTypes,
		parameterTypes,
		pos,
	)

	result := interpreter.runAllStatements(trampoline)
	if result == nil {
		return nil, nil
	}
	return result.(Value), nil
}

func (interpreter *Interpreter) functionValueInvocationTrampoline(
	function FunctionValue,
	arguments []Value,
	argumentTypes []sema.Type,
	parameterTypes []sema.Type,
	pos ast.Position,
) Trampoline {

	parameterTypeCount := len(parameterTypes)
	argumentCopies := make([]Value, len(arguments))

	for i, argument := range arguments {
		argumentType := argumentTypes[i]
		if i < parameterTypeCount {
			parameterType := parameterTypes[i]
			argumentCopies[i] = interpreter.copyAndConvert(argument, argumentType, parameterType)
		} else {
			argumentCopies[i] = argument.Copy()
		}
	}

	// TODO: optimize: only potentially used by host-functions

	location := LocationPosition{
		Position: pos,
		Location: interpreter.Checker.Location,
	}

	return function.Invoke(Invocation{
		Arguments:     argumentCopies,
		ArgumentTypes: argumentTypes,
		Location:      location,
		Interpreter:   interpreter,
	})
}

func (interpreter *Interpreter) invokeInterpretedFunction(
	function InterpretedFunctionValue,
	invocation Invocation,
) Trampoline {

	// Start a new activation record.
	// Lexical scope: use the function declaration's activation record,
	// not the current one (which would be dynamic scope)
	interpreter.activations.Push(function.Activation)

	// Make `self` available, if any
	if invocation.Self != nil {
		interpreter.declareVariable(sema.SelfIdentifier, invocation.Self)
	}

	return interpreter.invokeInterpretedFunctionActivated(function, invocation.Arguments)
}

// NOTE: assumes the function's activation (or an extension of it) is pushed!
//
func (interpreter *Interpreter) invokeInterpretedFunctionActivated(
	function InterpretedFunctionValue,
	arguments []Value,
) Trampoline {

	if function.ParameterList != nil {
		interpreter.bindParameterArguments(function.ParameterList, arguments)
	}

	functionBlockTrampoline := interpreter.visitFunctionBody(
		function.BeforeStatements,
		function.PreConditions,
		interpreter.visitStatements(function.Statements),
		function.PostConditions,
		function.Type.ReturnTypeAnnotation.Type,
	)

	return functionBlockTrampoline.
		Then(func(_ interface{}) {
			interpreter.activations.Pop()
		})
}

// bindParameterArguments binds the argument values to the given parameters
//
func (interpreter *Interpreter) bindParameterArguments(
	parameterList *ast.ParameterList,
	arguments []Value,
) {
	for parameterIndex, parameter := range parameterList.Parameters {
		argument := arguments[parameterIndex]
		interpreter.declareVariable(parameter.Identifier.Identifier, argument)
	}
}

func (interpreter *Interpreter) visitExpressionsNonCopying(expressions []ast.Expression) Trampoline {
	var trampoline Trampoline = Done{Result: NewArrayValueUnownedNonCopying()}

	for _, expression := range expressions {
		// NOTE: important: rebind expression, because it is captured in the closure below
		expression := expression

		// append the evaluation of this expression
		trampoline = trampoline.FlatMap(func(result interface{}) Trampoline {
			array := result.(*ArrayValue)

			// evaluate the expression
			return expression.Accept(interpreter).(Trampoline).
				FlatMap(func(result interface{}) Trampoline {
					value := result.(Value)

					newValues := append(array.Values, value)
					return Done{Result: NewArrayValueUnownedNonCopying(newValues...)}
				})
		})
	}

	return trampoline
}

func (interpreter *Interpreter) visitEntries(entries []ast.Entry) Trampoline {
	var trampoline Trampoline = Done{Result: []DictionaryEntryValues{}}

	for _, entry := range entries {
		// NOTE: important: rebind entry, because it is captured in the closure below
		func(entry ast.Entry) {
			// append the evaluation of this entry
			trampoline = trampoline.FlatMap(func(result interface{}) Trampoline {
				resultEntries := result.([]DictionaryEntryValues)

				// evaluate the key expression
				return entry.Key.Accept(interpreter).(Trampoline).
					FlatMap(func(result interface{}) Trampoline {
						key := result.(Value)

						// evaluate the value expression
						return entry.Value.Accept(interpreter).(Trampoline).
							FlatMap(func(result interface{}) Trampoline {
								value := result.(Value)

								newResultEntries := append(
									resultEntries,
									DictionaryEntryValues{
										Key:   key,
										Value: value,
									},
								)
								return Done{Result: newResultEntries}
							})
					})
			})
		}(entry)
	}

	return trampoline
}

func (interpreter *Interpreter) VisitFunctionExpression(expression *ast.FunctionExpression) ast.Repr {

	// lexical scope: variables in functions are bound to what is visible at declaration time
	lexicalScope := interpreter.activations.CurrentOrNew()

	functionType := interpreter.Checker.Elaboration.FunctionExpressionFunctionType[expression]

	var preConditions ast.Conditions
	if expression.FunctionBlock.PreConditions != nil {
		preConditions = *expression.FunctionBlock.PreConditions
	}

	var beforeStatements []ast.Statement
	var rewrittenPostConditions ast.Conditions

	if expression.FunctionBlock.PostConditions != nil {
		postConditionsRewrite :=
			interpreter.Checker.Elaboration.PostConditionsRewrite[expression.FunctionBlock.PostConditions]

		rewrittenPostConditions = postConditionsRewrite.RewrittenPostConditions
		beforeStatements = postConditionsRewrite.BeforeStatements
	}

	function := InterpretedFunctionValue{
		Interpreter:      interpreter,
		ParameterList:    expression.ParameterList,
		Type:             functionType,
		Activation:       lexicalScope,
		BeforeStatements: beforeStatements,
		PreConditions:    preConditions,
		Statements:       expression.FunctionBlock.Statements,
		PostConditions:   rewrittenPostConditions,
	}

	return Done{Result: function}
}

// NOTE: only called for top-level composite declarations
func (interpreter *Interpreter) VisitCompositeDeclaration(declaration *ast.CompositeDeclaration) ast.Repr {

	// lexical scope: variables in functions are bound to what is visible at declaration time
	lexicalScope := interpreter.activations.CurrentOrNew()

	_, _ = interpreter.declareCompositeValue(declaration, lexicalScope)

	// NOTE: no result, so it does *not* act like a return-statement
	return Done{}
}

// declareCompositeValue creates and declares the value for
// the composite declaration.
//
// For all composite kinds a constructor function is created.
//
// The constructor is a host function which creates a new composite,
// calls the initializer (interpreted function), if any,
// and then returns the composite.
//
// Inside the initializer and all functions, `self` is bound to
// the new composite value, and the constructor itself is bound
//
// For contracts, `contractValueHandler` is used to declare
// a contract value / instance (singleton).
//
// For all other composite kinds the constructor function is declared.
//
func (interpreter *Interpreter) declareCompositeValue(
	declaration *ast.CompositeDeclaration,
	lexicalScope hamt.Map,
) (
	scope hamt.Map,
	value Value,
) {
	identifier := declaration.Identifier.Identifier
	variable := interpreter.findOrDeclareVariable(identifier)

	// Make the value available in the initializer
	lexicalScope = lexicalScope.
		Insert(common.StringEntry(identifier), variable)

	// Evaluate nested declarations in a new scope, so values
	// of nested declarations won't be visible after the containing declaration

	members := map[string]Value{}

	(func() {
		interpreter.activations.PushCurrent()
		defer interpreter.activations.Pop()

		for _, nestedInterfaceDeclaration := range declaration.InterfaceDeclarations {
			interpreter.declareInterface(nestedInterfaceDeclaration, lexicalScope)
		}

		for _, nestedCompositeDeclaration := range declaration.CompositeDeclarations {

			// Pass the lexical scope, which has the containing composite's value declared,
			// to the nested declarations so they can refer to it, and update the lexical scope
			// so the container's functions can refer to the nested composite's value

			var nestedValue Value
			lexicalScope, nestedValue =
				interpreter.declareCompositeValue(nestedCompositeDeclaration, lexicalScope)

			memberIdentifier := nestedCompositeDeclaration.Identifier.Identifier
			members[memberIdentifier] = nestedValue
		}
	})()

	compositeType := interpreter.Checker.Elaboration.CompositeDeclarationTypes[declaration]
	typeID := compositeType.ID()

	var initializerFunction FunctionValue
	if declaration.CompositeKind == common.CompositeKindEvent {
		initializerFunction = NewHostFunctionValue(
			func(invocation Invocation) Trampoline {
				for i, argument := range invocation.Arguments {
					parameter := compositeType.ConstructorParameters[i]
					invocation.Self.Fields[parameter.Identifier] = argument
				}
				return Done{}
			},
		)
	} else {
		compositeInitializerFunction := interpreter.compositeInitializerFunction(declaration, lexicalScope)
		if compositeInitializerFunction != nil {
			initializerFunction = *compositeInitializerFunction
		}
	}

	var destructorFunction FunctionValue
	compositeDestructorFunction := interpreter.compositeDestructorFunction(declaration, lexicalScope)
	if compositeDestructorFunction != nil {
		destructorFunction = *compositeDestructorFunction
	}

	functions := interpreter.compositeFunctions(declaration, lexicalScope)

	wrapFunctions := func(code wrapperCode) {

		// Wrap initializer

		initializerFunctionWrapper :=
			code.initializerFunctionWrapper

		if initializerFunctionWrapper != nil {
			initializerFunction = initializerFunctionWrapper(initializerFunction)
		}

		// Wrap destructor

		destructorFunctionWrapper :=
			code.destructorFunctionWrapper

		if destructorFunctionWrapper != nil {
			destructorFunction = destructorFunctionWrapper(destructorFunction)
		}

		// Wrap functions

		for name, functionWrapper := range code.functionWrappers {
			functions[name] = functionWrapper(functions[name])
		}
	}

	// NOTE: First the conditions of the type requirements are evaluated,
	//  then the conditions of this composite's conformances
	//
	// Because the conditions are wrappers, they have to be applied
	// in reverse order: first the conformances, then the type requirements;
	// each conformances and type requirements in reverse order as well.

	for i := len(compositeType.Conformances) - 1; i >= 0; i-- {
		conformance := compositeType.Conformances[i]

		wrapFunctions(interpreter.typeCodes.interfaceCodes[conformance.ID()])
	}

	typeRequirements := compositeType.TypeRequirements()

	for i := len(typeRequirements) - 1; i >= 0; i-- {
		typeRequirement := typeRequirements[i]

		wrapFunctions(interpreter.typeCodes.typeRequirementCodes[typeRequirement.ID()])
	}

	interpreter.typeCodes.compositeCodes[compositeType.ID()] = compositeTypeCode{
		destructorFunction: destructorFunction,
		compositeFunctions: functions,
	}

	location := interpreter.Checker.Location

	constructor := NewHostFunctionValue(
		func(invocation Invocation) Trampoline {

			// Load injected fields
			var injectedFields map[string]Value
			if interpreter.injectedCompositeFieldsHandler != nil {
				injectedFields = interpreter.injectedCompositeFieldsHandler(
					interpreter,
					location,
					typeID,
					declaration.CompositeKind,
				)
			}

			value := &CompositeValue{
				Location:       location,
				TypeID:         typeID,
				Kind:           declaration.CompositeKind,
				Fields:         map[string]Value{},
				InjectedFields: injectedFields,
				Functions:      functions,
				Destructor:     destructorFunction,
				// NOTE: new value has no owner
				Owner: nil,
			}

			invocation.Self = value

			if declaration.CompositeKind == common.CompositeKindContract {
				// NOTE: set the variable value immediately, as the contract value
				// needs to be available for nested declarations

				variable.Value = value
			}

			var initializationTrampoline Trampoline = Done{}

			if initializerFunction != nil {
				// NOTE: arguments are already properly boxed by invocation expression

				initializationTrampoline = initializerFunction.Invoke(invocation)
			}

			return initializationTrampoline.
				Map(func(_ interface{}) interface{} {
					return value
				})
		},
	)

	// Contract declarations declare a value / instance (singleton),
	// for all other composite kinds, the constructor is declared

	if declaration.CompositeKind == common.CompositeKindContract {
		contract := interpreter.contractValueHandler(interpreter, compositeType, constructor)
		contract.NestedValues = members
		value = contract
		// NOTE: variable value is also set in the constructor function: it needs to be available
		// for nested declarations, which might be invoked when the constructor is invoked
		variable.Value = value
	} else {
		constructor.Members = members
		value = constructor
		variable.Value = value
	}

	return lexicalScope, value
}

func (interpreter *Interpreter) compositeInitializerFunction(
	compositeDeclaration *ast.CompositeDeclaration,
	lexicalScope hamt.Map,
) *InterpretedFunctionValue {

	// TODO: support multiple overloaded initializers

	initializers := compositeDeclaration.Members.Initializers()
	var initializer *ast.SpecialFunctionDeclaration
	if len(initializers) == 0 {
		return nil
	}

	initializer = initializers[0]
	functionType := interpreter.Checker.Elaboration.SpecialFunctionTypes[initializer].FunctionType

	parameterList := initializer.ParameterList

	var preConditions ast.Conditions
	if initializer.FunctionBlock.PreConditions != nil {
		preConditions = *initializer.FunctionBlock.PreConditions
	}

	statements := initializer.FunctionBlock.Statements

	var beforeStatements []ast.Statement
	var rewrittenPostConditions ast.Conditions

	if initializer.FunctionBlock.PostConditions != nil {
		postConditionsRewrite :=
			interpreter.Checker.Elaboration.PostConditionsRewrite[initializer.FunctionBlock.PostConditions]

		beforeStatements = postConditionsRewrite.BeforeStatements
		rewrittenPostConditions = postConditionsRewrite.RewrittenPostConditions
	}

	return &InterpretedFunctionValue{
		Interpreter:      interpreter,
		ParameterList:    parameterList,
		Type:             functionType,
		Activation:       lexicalScope,
		BeforeStatements: beforeStatements,
		PreConditions:    preConditions,
		Statements:       statements,
		PostConditions:   rewrittenPostConditions,
	}
}

func (interpreter *Interpreter) compositeDestructorFunction(
	compositeDeclaration *ast.CompositeDeclaration,
	lexicalScope hamt.Map,
) *InterpretedFunctionValue {

	destructor := compositeDeclaration.Members.Destructor()
	if destructor == nil {
		return nil
	}

	statements := destructor.FunctionBlock.Statements

	var preConditions ast.Conditions

	if destructor.FunctionBlock.PreConditions != nil {
		preConditions = *destructor.FunctionBlock.PreConditions
	}

	var beforeStatements []ast.Statement
	var rewrittenPostConditions ast.Conditions

	if destructor.FunctionBlock.PostConditions != nil {
		postConditionsRewrite :=
			interpreter.Checker.Elaboration.PostConditionsRewrite[destructor.FunctionBlock.PostConditions]

		beforeStatements = postConditionsRewrite.BeforeStatements
		rewrittenPostConditions = postConditionsRewrite.RewrittenPostConditions
	}

	return &InterpretedFunctionValue{
		Interpreter:      interpreter,
		Type:             emptyFunctionType,
		Activation:       lexicalScope,
		BeforeStatements: beforeStatements,
		PreConditions:    preConditions,
		Statements:       statements,
		PostConditions:   rewrittenPostConditions,
	}
}

func (interpreter *Interpreter) compositeFunctions(
	compositeDeclaration *ast.CompositeDeclaration,
	lexicalScope hamt.Map,
) map[string]FunctionValue {

	functions := map[string]FunctionValue{}

	for _, functionDeclaration := range compositeDeclaration.Members.Functions {
		name := functionDeclaration.Identifier.Identifier
		functions[name] =
			interpreter.compositeFunction(
				functionDeclaration,
				lexicalScope,
			)
	}

	return functions
}

func (interpreter *Interpreter) functionWrappers(
	members *ast.Members,
	lexicalScope hamt.Map,
) map[string]FunctionWrapper {

	functionWrappers := map[string]FunctionWrapper{}

	for _, functionDeclaration := range members.Functions {

		functionType := interpreter.Checker.Elaboration.FunctionDeclarationFunctionTypes[functionDeclaration]

		name := functionDeclaration.Identifier.Identifier
		functionWrapper := interpreter.functionConditionsWrapper(
			functionDeclaration,
			functionType.ReturnTypeAnnotation.Type,
			lexicalScope,
		)
		if functionWrapper == nil {
			continue
		}
		functionWrappers[name] = functionWrapper
	}

	return functionWrappers
}

func (interpreter *Interpreter) compositeFunction(
	functionDeclaration *ast.FunctionDeclaration,
	lexicalScope hamt.Map,
) InterpretedFunctionValue {

	functionType := interpreter.Checker.Elaboration.FunctionDeclarationFunctionTypes[functionDeclaration]

	var preConditions ast.Conditions

	if functionDeclaration.FunctionBlock.PreConditions != nil {
		preConditions = *functionDeclaration.FunctionBlock.PreConditions
	}

	var beforeStatements []ast.Statement
	var postConditions ast.Conditions

	if functionDeclaration.FunctionBlock.PostConditions != nil {

		postConditionsRewrite :=
			interpreter.Checker.Elaboration.PostConditionsRewrite[functionDeclaration.FunctionBlock.PostConditions]

		beforeStatements = postConditionsRewrite.BeforeStatements
		postConditions = postConditionsRewrite.RewrittenPostConditions
	}

	parameterList := functionDeclaration.ParameterList
	statements := functionDeclaration.FunctionBlock.Statements

	return InterpretedFunctionValue{
		Interpreter:      interpreter,
		ParameterList:    parameterList,
		Type:             functionType,
		Activation:       lexicalScope,
		BeforeStatements: beforeStatements,
		PreConditions:    preConditions,
		Statements:       statements,
		PostConditions:   postConditions,
	}
}

func (interpreter *Interpreter) VisitFieldDeclaration(_ *ast.FieldDeclaration) ast.Repr {
	// fields can't be interpreted
	panic(errors.NewUnreachableError())
}

func (interpreter *Interpreter) copyAndConvert(value Value, valueType, targetType sema.Type) Value {
	return interpreter.convertAndBox(value.Copy(), valueType, targetType)
}

// convertAndBox converts a value to a target type, and boxes in optionals and any value, if necessary
func (interpreter *Interpreter) convertAndBox(value Value, valueType, targetType sema.Type) Value {
	value = interpreter.convert(value, valueType, targetType)
	return interpreter.boxOptional(value, valueType, targetType)
}

func (interpreter *Interpreter) convert(value Value, valueType, targetType sema.Type) Value {
	if valueType == nil {
		return value
	}

	if _, valueIsOptional := valueType.(*sema.OptionalType); valueIsOptional {
		return value
	}

	unwrappedTargetType := sema.UnwrapOptionalType(targetType)

	if valueType.Equal(unwrappedTargetType) {
		return value
	}

	switch unwrappedTargetType.(type) {
	case *sema.IntType:
		return ConvertInt(value, interpreter)

	case *sema.UIntType:
		return ConvertUInt(value, interpreter)

	case *sema.AddressType:
		return ConvertAddress(value, interpreter)

	// Int*
	case *sema.Int8Type:
		return ConvertInt8(value, interpreter)

	case *sema.Int16Type:
		return ConvertInt16(value, interpreter)

	case *sema.Int32Type:
		return ConvertInt32(value, interpreter)

	case *sema.Int64Type:
		return ConvertInt64(value, interpreter)

	case *sema.Int128Type:
		return ConvertInt128(value, interpreter)

	case *sema.Int256Type:
		return ConvertInt256(value, interpreter)

	// UInt*
	case *sema.UInt8Type:
		return ConvertUInt8(value, interpreter)

	case *sema.UInt16Type:
		return ConvertUInt16(value, interpreter)

	case *sema.UInt32Type:
		return ConvertUInt32(value, interpreter)

	case *sema.UInt64Type:
		return ConvertUInt64(value, interpreter)

	case *sema.UInt128Type:
		return ConvertUInt128(value, interpreter)

	case *sema.UInt256Type:
		return ConvertUInt256(value, interpreter)

	// Word*
	case *sema.Word8Type:
		return ConvertWord8(value, interpreter)

	case *sema.Word16Type:
		return ConvertWord16(value, interpreter)

	case *sema.Word32Type:
		return ConvertWord32(value, interpreter)

	case *sema.Word64Type:
		return ConvertWord64(value, interpreter)

	// Fix*

	case *sema.Fix64Type:
		return ConvertFix64(value, interpreter)

	case *sema.UFix64Type:
		return ConvertUFix64(value, interpreter)

	default:
		return value
	}
}

// boxOptional boxes a value in optionals, if necessary
func (interpreter *Interpreter) boxOptional(value Value, valueType, targetType sema.Type) Value {
	inner := value
	for {
		optionalType, ok := targetType.(*sema.OptionalType)
		if !ok {
			break
		}

		if some, ok := inner.(*SomeValue); ok {
			inner = some.Value
		} else if _, ok := inner.(NilValue); ok {
			// NOTE: nested nil will be unboxed!
			return inner
		} else {
			value = NewSomeValueOwningNonCopying(value)
			valueType = &sema.OptionalType{
				Type: valueType,
			}
		}

		targetType = optionalType.Type
	}
	return value
}

func (interpreter *Interpreter) unbox(value Value) Value {
	for {
		some, ok := value.(*SomeValue)
		if !ok {
			return value
		}

		value = some.Value
	}
}

// NOTE: only called for top-level interface declarations
func (interpreter *Interpreter) VisitInterfaceDeclaration(declaration *ast.InterfaceDeclaration) ast.Repr {

	// lexical scope: variables in functions are bound to what is visible at declaration time
	lexicalScope := interpreter.activations.CurrentOrNew()

	interpreter.declareInterface(declaration, lexicalScope)

	// NOTE: no result, so it does *not* act like a return-statement
	return Done{}
}

func (interpreter *Interpreter) declareInterface(
	declaration *ast.InterfaceDeclaration,
	lexicalScope hamt.Map,
) {
	// Evaluate nested declarations in a new scope, so values
	// of nested declarations won't be visible after the containing declaration

	(func() {
		interpreter.activations.PushCurrent()
		defer interpreter.activations.Pop()

		for _, nestedInterfaceDeclaration := range declaration.InterfaceDeclarations {
			interpreter.declareInterface(nestedInterfaceDeclaration, lexicalScope)
		}

		for _, nestedCompositeDeclaration := range declaration.CompositeDeclarations {
			interpreter.declareTypeRequirement(nestedCompositeDeclaration, lexicalScope)
		}
	})()

	interfaceType := interpreter.Checker.Elaboration.InterfaceDeclarationTypes[declaration]
	typeID := interfaceType.ID()

	initializerFunctionWrapper := interpreter.initializerFunctionWrapper(declaration.Members, lexicalScope)
	destructorFunctionWrapper := interpreter.destructorFunctionWrapper(declaration.Members, lexicalScope)
	functionWrappers := interpreter.functionWrappers(declaration.Members, lexicalScope)

	interpreter.typeCodes.interfaceCodes[typeID] = wrapperCode{
		initializerFunctionWrapper: initializerFunctionWrapper,
		destructorFunctionWrapper:  destructorFunctionWrapper,
		functionWrappers:           functionWrappers,
	}
}

func (interpreter *Interpreter) declareTypeRequirement(
	declaration *ast.CompositeDeclaration,
	lexicalScope hamt.Map,
) {
	// Evaluate nested declarations in a new scope, so values
	// of nested declarations won't be visible after the containing declaration

	(func() {
		interpreter.activations.PushCurrent()
		defer interpreter.activations.Pop()

		for _, nestedInterfaceDeclaration := range declaration.InterfaceDeclarations {
			interpreter.declareInterface(nestedInterfaceDeclaration, lexicalScope)
		}

		for _, nestedCompositeDeclaration := range declaration.CompositeDeclarations {
			interpreter.declareTypeRequirement(nestedCompositeDeclaration, lexicalScope)
		}
	})()

	compositeType := interpreter.Checker.Elaboration.CompositeDeclarationTypes[declaration]
	typeID := compositeType.ID()

	initializerFunctionWrapper := interpreter.initializerFunctionWrapper(declaration.Members, lexicalScope)
	destructorFunctionWrapper := interpreter.destructorFunctionWrapper(declaration.Members, lexicalScope)
	functionWrappers := interpreter.functionWrappers(declaration.Members, lexicalScope)

	interpreter.typeCodes.typeRequirementCodes[typeID] = wrapperCode{
		initializerFunctionWrapper: initializerFunctionWrapper,
		destructorFunctionWrapper:  destructorFunctionWrapper,
		functionWrappers:           functionWrappers,
	}
}

func (interpreter *Interpreter) initializerFunctionWrapper(
	members *ast.Members,
	lexicalScope hamt.Map,
) FunctionWrapper {

	// TODO: support multiple overloaded initializers

	initializers := members.Initializers()
	if len(initializers) == 0 {
		return nil
	}

	firstInitializer := initializers[0]
	if firstInitializer.FunctionBlock == nil {
		return nil
	}

	return interpreter.functionConditionsWrapper(
		firstInitializer.FunctionDeclaration,
		&sema.VoidType{},
		lexicalScope,
	)
}

func (interpreter *Interpreter) destructorFunctionWrapper(
	members *ast.Members,
	lexicalScope hamt.Map,
) FunctionWrapper {

	destructor := members.Destructor()
	if destructor == nil {
		return nil
	}

	return interpreter.functionConditionsWrapper(
		destructor.FunctionDeclaration,
		&sema.VoidType{},
		lexicalScope,
	)
}

func (interpreter *Interpreter) functionConditionsWrapper(
	declaration *ast.FunctionDeclaration,
	returnType sema.Type,
	lexicalScope hamt.Map,
) FunctionWrapper {

	if declaration.FunctionBlock == nil {
		return nil
	}

	var preConditions ast.Conditions
	if declaration.FunctionBlock.PreConditions != nil {
		preConditions = *declaration.FunctionBlock.PreConditions
	}

	var beforeStatements []ast.Statement
	var rewrittenPostConditions ast.Conditions

	if declaration.FunctionBlock.PostConditions != nil {

		postConditionsRewrite :=
			interpreter.Checker.Elaboration.PostConditionsRewrite[declaration.FunctionBlock.PostConditions]

		beforeStatements = postConditionsRewrite.BeforeStatements
		rewrittenPostConditions = postConditionsRewrite.RewrittenPostConditions
	}

	return func(inner FunctionValue) FunctionValue {
		return NewHostFunctionValue(func(invocation Invocation) Trampoline {
			// Start a new activation record.
			// Lexical scope: use the function declaration's activation record,
			// not the current one (which would be dynamic scope)
			interpreter.activations.Push(lexicalScope)

			if declaration.ParameterList != nil {
				interpreter.bindParameterArguments(
					declaration.ParameterList,
					invocation.Arguments,
				)
			}

			if invocation.Self != nil {
				interpreter.declareVariable(sema.SelfIdentifier, invocation.Self)
			}

			// NOTE: The `inner` function might be nil.
			//   This is the case if the conforming type did not declare a function.

			var body Trampoline = Done{}
			if inner != nil {
				// NOTE: It is important to wrap the invocation in a trampoline,
				//  so the inner function isn't invoked here

				body = More(func() Trampoline {

					// NOTE: It is important to actually return the value returned
					//   from the inner function, otherwise it is lost

					return inner.Invoke(invocation).
						Map(func(returnValue interface{}) interface{} {
							return functionReturn{returnValue.(Value)}
						})
				})
			}

			functionBlockTrampoline := interpreter.visitFunctionBody(
				beforeStatements,
				preConditions,
				body,
				rewrittenPostConditions,
				returnType,
			)

			return functionBlockTrampoline.
				Then(func(_ interface{}) {
					interpreter.activations.Pop()
				})
		})
	}
}

func (interpreter *Interpreter) ensureLoaded(location ast.Location, loadProgram func() *ast.Program) (subInterpreter *Interpreter) {
	locationID := location.ID()

	// If sub-interpreter already exists, return it
	subInterpreter = interpreter.allInterpreters[locationID]
	if subInterpreter != nil {
		return subInterpreter
	}

	// Create a new sub-interpreter and interpret the top-level declarations
	var checkerErr *sema.CheckerError
	var importedChecker *sema.Checker
	importedChecker, checkerErr = interpreter.Checker.EnsureLoaded(location, loadProgram)
	if importedChecker == nil {
		panic("missing checker")
	}
	if checkerErr != nil {
		panic(checkerErr)
	}

	var err error
	subInterpreter, err = NewInterpreter(
		importedChecker,
		WithPredefinedValues(interpreter.PredefinedValues),
		WithOnEventEmittedHandler(interpreter.onEventEmitted),
		WithOnStatementHandler(interpreter.onStatement),
		WithStorageReadHandler(interpreter.storageReadHandler),
		WithStorageWriteHandler(interpreter.storageWriteHandler),
		WithStorageKeyHandler(interpreter.storageKeyHandler),
		WithInjectedCompositeFieldsHandler(interpreter.injectedCompositeFieldsHandler),
		WithContractValueHandler(interpreter.contractValueHandler),
		WithImportProgramHandler(interpreter.importProgramHandler),
		WithAllInterpreters(interpreter.allInterpreters),
		WithAllCheckers(interpreter.allCheckers),
		withTypeCodes(interpreter.typeCodes),
	)
	if err != nil {
		panic(err)
	}

	subInterpreter.runAllStatements(subInterpreter.interpret())

	return subInterpreter
}

func (interpreter *Interpreter) VisitImportDeclaration(declaration *ast.ImportDeclaration) ast.Repr {

	location := declaration.Location

	subInterpreter := interpreter.ensureLoaded(
		location,
		func() *ast.Program {
			return interpreter.importProgramHandler(interpreter, location)
		},
	)

	// determine which identifiers are imported /
	// which variables need to be declared

	var variables map[string]*Variable
	identifierLength := len(declaration.Identifiers)
	if identifierLength > 0 {
		variables = make(map[string]*Variable, identifierLength)
		for _, identifier := range declaration.Identifiers {
			variables[identifier.Identifier] =
				subInterpreter.Globals[identifier.Identifier]
		}
	} else {
		variables = subInterpreter.Globals
	}

	// set variables for all imported values
	for name, variable := range variables {

		// don't import predeclared values
		if _, ok := subInterpreter.Checker.PredeclaredValues[name]; ok {
			continue
		}

		// don't import base values
		if _, ok := sema.BaseValues[name]; ok {
			continue
		}

		interpreter.setVariable(name, variable)
	}

	return Done{}
}

func (interpreter *Interpreter) VisitTransactionDeclaration(declaration *ast.TransactionDeclaration) ast.Repr {
	interpreter.declareTransactionEntryPoint(declaration)

	// NOTE: no result, so it does *not* act like a return-statement
	return Done{}
}

func (interpreter *Interpreter) declareTransactionEntryPoint(declaration *ast.TransactionDeclaration) {
	transactionType := interpreter.Checker.Elaboration.TransactionDeclarationTypes[declaration]

	lexicalScope := interpreter.activations.CurrentOrNew()

	var prepareFunction *ast.FunctionDeclaration
	var prepareFunctionType *sema.FunctionType
	if declaration.Prepare != nil {
		prepareFunction = declaration.Prepare.FunctionDeclaration
		prepareFunctionType = transactionType.PrepareFunctionType().InvocationFunctionType()
	}

	var executeFunction *ast.FunctionDeclaration
	var executeFunctionType *sema.FunctionType
	if declaration.Execute != nil {
		executeFunction = declaration.Execute.FunctionDeclaration
		executeFunctionType = transactionType.ExecuteFunctionType().InvocationFunctionType()
	}

	postConditionsRewrite :=
		interpreter.Checker.Elaboration.PostConditionsRewrite[declaration.PostConditions]

	self := &CompositeValue{
		Location: interpreter.Checker.Location,
		Fields:   map[string]Value{},
	}

	transactionFunction := NewHostFunctionValue(
		func(invocation Invocation) Trampoline {
			interpreter.activations.Push(lexicalScope)

			invocation.Self = self
			interpreter.declareVariable(sema.SelfIdentifier, self)

			if declaration.ParameterList != nil {
				// If the transaction has a parameter list of N parameters,
				// bind the first N arguments of the invocation to the transaction parameters,
				// then leave the remaining arguments for the prepare function

				transactionParameterCount := len(declaration.ParameterList.Parameters)

				transactionArguments := invocation.Arguments[:transactionParameterCount]
				prepareArguments := invocation.Arguments[transactionParameterCount:]

				interpreter.bindParameterArguments(declaration.ParameterList, transactionArguments)
				invocation.Arguments = prepareArguments
			}

			// NOTE: get current scope instead of using `lexicalScope`,
			// because current scope has `self` declared
			transactionScope := interpreter.activations.CurrentOrNew()

			var prepareTrampoline Trampoline = Done{}
			var executeTrampoline Trampoline = Done{}

			if prepareFunction != nil {
				prepare := interpreter.functionDeclarationValue(
					prepareFunction,
					prepareFunctionType,
					transactionScope,
				)

				prepareTrampoline = More(func() Trampoline {
					return prepare.Invoke(invocation)
				})
			}

			if executeFunction != nil {
				execute := interpreter.functionDeclarationValue(
					executeFunction,
					executeFunctionType,
					transactionScope,
				)

				executeTrampoline = More(func() Trampoline {
					invocationWithoutArguments := invocation
					invocationWithoutArguments.Arguments = nil
					return execute.Invoke(invocationWithoutArguments)
				})
			}

			var preConditions ast.Conditions
			if declaration.PreConditions != nil {
				preConditions = *declaration.PreConditions
			}

			return prepareTrampoline.
				FlatMap(func(_ interface{}) Trampoline {
					return interpreter.visitFunctionBody(
						postConditionsRewrite.BeforeStatements,
						preConditions,
						executeTrampoline,
						postConditionsRewrite.RewrittenPostConditions,
						&sema.VoidType{},
					)
				})
		})

	interpreter.Transactions = append(interpreter.Transactions, &transactionFunction)
}

func (interpreter *Interpreter) VisitEmitStatement(statement *ast.EmitStatement) ast.Repr {
	return statement.InvocationExpression.Accept(interpreter).(Trampoline).
		FlatMap(func(result interface{}) Trampoline {
			event := result.(*CompositeValue)

			eventType := interpreter.Checker.Elaboration.EmitStatementEventTypes[statement]

			interpreter.onEventEmitted(interpreter, event, eventType)

			// NOTE: no result, so it does *not* act like a return-statement
			return Done{}
		})
}

func (interpreter *Interpreter) VisitCastingExpression(expression *ast.CastingExpression) ast.Repr {
	return expression.Expression.Accept(interpreter).(Trampoline).
		Map(func(result interface{}) interface{} {
			value := result.(Value)

			expectedType := interpreter.Checker.Elaboration.CastingTargetTypes[expression]

			switch expression.Operation {
			case ast.OperationFailableCast:
				dynamicType := value.DynamicType(interpreter)
				if interpreter.IsSubType(dynamicType, expectedType) {
					return NewSomeValueOwningNonCopying(value)
				}
				return NilValue{}

			case ast.OperationCast:
				staticValueType := interpreter.Checker.Elaboration.CastingStaticValueTypes[expression]
				return interpreter.convertAndBox(value, staticValueType, expectedType)

			default:
				panic(errors.NewUnreachableError())
			}
		})
}

func (interpreter *Interpreter) VisitCreateExpression(expression *ast.CreateExpression) ast.Repr {
	return expression.InvocationExpression.Accept(interpreter)
}

func (interpreter *Interpreter) VisitDestroyExpression(expression *ast.DestroyExpression) ast.Repr {
	return expression.Expression.Accept(interpreter).(Trampoline).
		FlatMap(func(result interface{}) Trampoline {
			value := result.(Value)

			// TODO: optimize: only potentially used by host-functions
			location := LocationPosition{
				Position: expression.StartPosition(),
				Location: interpreter.Checker.Location,
			}

			return value.(DestroyableValue).Destroy(interpreter, location)
		})
}

func (interpreter *Interpreter) VisitReferenceExpression(referenceExpression *ast.ReferenceExpression) ast.Repr {

	authorized := referenceExpression.Type.(*ast.ReferenceType).Authorized

	if interpreter.Checker.Elaboration.IsReferenceIntoStorage[referenceExpression] {
		indexExpression := referenceExpression.Expression.(*ast.IndexExpression)
		return indexExpression.TargetExpression.Accept(interpreter).(Trampoline).
			FlatMap(func(result interface{}) Trampoline {
				storage := result.(StorageValue)

				indexingType := interpreter.Checker.Elaboration.IndexExpressionIndexingTypes[indexExpression]
				key := interpreter.storageKeyHandler(interpreter, storage.Address, indexingType)

				referenceValue := &StorageReferenceValue{
					Authorized:           authorized,
					TargetStorageAddress: storage.Address,
					TargetKey:            key,
					// NOTE: new value has no owner
					Owner: nil,
				}

				return Done{Result: referenceValue}
			})
	} else {
		return referenceExpression.Expression.Accept(interpreter).(Trampoline).
			Map(func(result interface{}) interface{} {
				return &EphemeralReferenceValue{
					Authorized: authorized,
					Value:      result.(Value),
				}
			})
	}
}

func (interpreter *Interpreter) VisitForceExpression(expression *ast.ForceExpression) ast.Repr {
	return expression.Expression.Accept(interpreter).(Trampoline).
		Map(func(result interface{}) interface{} {
			switch result := result.(type) {
			case *SomeValue:
				return result.Value

			case NilValue:
				panic(
					&ForceNilError{
						LocationRange: interpreter.locationRange(expression.Expression),
					},
				)

			default:
				panic(errors.NewUnreachableError())
			}
		})
}

func (interpreter *Interpreter) readStored(storageAddress common.Address, key string) OptionalValue {
	return interpreter.storageReadHandler(interpreter, storageAddress, key)
}

func (interpreter *Interpreter) writeStored(storageAddress common.Address, key string, value OptionalValue) {
	value.SetOwner(&storageAddress)

	interpreter.storageWriteHandler(interpreter, storageAddress, key, value)
}

type ValueConverter func(Value, *Interpreter) Value

var converters = map[string]ValueConverter{
	"Int":     ConvertInt,
	"UInt":    ConvertUInt,
	"Int8":    ConvertInt8,
	"Int16":   ConvertInt16,
	"Int32":   ConvertInt32,
	"Int64":   ConvertInt64,
	"Int128":  ConvertInt128,
	"Int256":  ConvertInt256,
	"UInt8":   ConvertUInt8,
	"UInt16":  ConvertUInt16,
	"UInt32":  ConvertUInt32,
	"UInt64":  ConvertUInt64,
	"UInt128": ConvertUInt128,
	"UInt256": ConvertUInt256,
	"Word8":   ConvertWord8,
	"Word16":  ConvertWord16,
	"Word32":  ConvertWord32,
	"Word64":  ConvertWord64,
	"Fix64":   ConvertFix64,
	"UFix64":  ConvertUFix64,
	"Address": ConvertAddress,
}

func init() {
	for _, numberType := range sema.AllNumberTypes {
		if _, ok := converters[numberType.String()]; !ok {
			panic(fmt.Sprintf("missing converter for number type: %s", numberType))
		}
	}
}

func (interpreter *Interpreter) defineBaseFunctions() {
	for name, converter := range converters {
		err := interpreter.ImportValue(
			name,
			interpreter.newConverterFunction(converter),
		)
		if err != nil {
			panic(errors.NewUnreachableError())
		}
	}
}

func (interpreter *Interpreter) newConverterFunction(converter ValueConverter) FunctionValue {
	return NewHostFunctionValue(
		func(invocation Invocation) Trampoline {
			value := invocation.Arguments[0]
			return Done{Result: converter(value, interpreter)}
		},
	)
}

// TODO:
// - FunctionType
// - PublishedType
//
// - Character
// - Account
// - PublicAccount
// - Block
// - Storage
// - References

func (interpreter *Interpreter) IsSubType(subType DynamicType, superType sema.Type) bool {
	switch typedSubType := subType.(type) {
	case VoidType:
		switch superType.(type) {
		case *sema.VoidType, *sema.AnyStructType:
			return true

		default:
			return false
		}

	case StringType:
		switch superType.(type) {
		case *sema.StringType, *sema.AnyStructType:
			return true

		default:
			return false
		}

	case BoolType:
		switch superType.(type) {
		case *sema.BoolType, *sema.AnyStructType:
			return true

		default:
			return false
		}

	case AddressType:
		switch superType.(type) {
		case *sema.AddressType, *sema.AnyStructType:
			return true

		default:
			return false
		}

	case NumberType:
		return sema.IsSubType(typedSubType.StaticType, superType)

	case CompositeType:
		return sema.IsSubType(typedSubType.StaticType, superType)

	case ArrayType:
		var superTypeElementType sema.Type

		switch typedSuperType := superType.(type) {
		case *sema.VariableSizedType:
			superTypeElementType = typedSuperType.Type

		case *sema.ConstantSizedType:
			superTypeElementType = typedSuperType.Type

		case *sema.AnyStructType, *sema.AnyResourceType:
			return true

		default:
			return false
		}

		for _, elementType := range typedSubType.ElementTypes {
			if !interpreter.IsSubType(elementType, superTypeElementType) {
				return false
			}
		}

		return true

	case DictionaryType:

		switch typedSuperType := superType.(type) {
		case *sema.DictionaryType:
			for _, entryTypes := range typedSubType.EntryTypes {
				if !interpreter.IsSubType(entryTypes.KeyType, typedSuperType.KeyType) ||
					!interpreter.IsSubType(entryTypes.ValueType, typedSuperType.ValueType) {

					return false
				}
			}

			return true

		case *sema.AnyStructType, *sema.AnyResourceType:
			return true

		default:
			return false
		}

	case NilType:
		switch superType.(type) {
		case *sema.OptionalType, *sema.AnyStructType, *sema.AnyResourceType:
			return true

		default:
			return false
		}

	case SomeType:
		switch typedSuperType := superType.(type) {
		case *sema.OptionalType:
			return interpreter.IsSubType(typedSubType.InnerType, typedSuperType.Type)

		case *sema.AnyStructType, *sema.AnyResourceType:
			return true

		default:
			return false
		}

	case StorageType:
		switch superType.(type) {
		case *sema.StorageType, *sema.AnyStructType:
			return true

		default:
			return false
		}

	case ReferenceType:
		switch typedSuperType := superType.(type) {
		case *sema.AnyStructType:
			return true

		case *sema.ReferenceType:
			if typedSubType.Authorized() {
				return interpreter.IsSubType(typedSubType.InnerType(), typedSuperType.Type)
			} else {
				// NOTE: Allowing all casts for casting unauthorized references is intentional:
				// all invalid cases have already been rejected statically
				return true
			}

		default:
			return false
		}
	}

	return false
}
