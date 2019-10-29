package interpreter

import (
	"fmt"
	goRuntime "runtime"

	"github.com/raviqqe/hamt"

	"github.com/dapperlabs/flow-go/language/runtime/activations"
	"github.com/dapperlabs/flow-go/language/runtime/ast"
	"github.com/dapperlabs/flow-go/language/runtime/common"
	"github.com/dapperlabs/flow-go/language/runtime/errors"
	"github.com/dapperlabs/flow-go/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/language/runtime/trampoline"
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

// StatementTrampoline

type StatementTrampoline struct {
	F    func() Trampoline
	Line int
}

func (m StatementTrampoline) Resume() interface{} {
	return m.F
}

func (m StatementTrampoline) FlatMap(f func(interface{}) Trampoline) Trampoline {
	return FlatMap{Subroutine: m, Continuation: f}
}

func (m StatementTrampoline) Map(f func(interface{}) interface{}) Trampoline {
	return MapTrampoline(m, f)
}

func (m StatementTrampoline) Then(f func(interface{})) Trampoline {
	return ThenTrampoline(m, f)
}

func (m StatementTrampoline) Continue() Trampoline {
	return m.F()
}

// Visit-methods for statement which return a non-nil value
// are treated like they are returning a value.

type Interpreter struct {
	Checker             *sema.Checker
	PredefinedValues    map[string]Value
	activations         *activations.Activations
	Globals             map[string]*Variable
	interfaces          map[string]*ast.InterfaceDeclaration
	CompositeFunctions  map[string]map[string]FunctionValue
	DestructorFunctions map[string]*InterpretedFunctionValue
	SubInterpreters     map[ast.LocationID]*Interpreter
	onEventEmitted      func(EventValue)
}

type InterpreterOpt func(*Interpreter)

func WithOnEventEmittedHandler(handler func(EventValue)) InterpreterOpt {
	return func(inter *Interpreter) {
		inter.onEventEmitted = handler
	}
}

func NewInterpreter(checker *sema.Checker, predefinedValues map[string]Value, opts ...InterpreterOpt) (*Interpreter, error) {
	interpreter := &Interpreter{
		Checker:             checker,
		PredefinedValues:    predefinedValues,
		activations:         &activations.Activations{},
		Globals:             map[string]*Variable{},
		interfaces:          map[string]*ast.InterfaceDeclaration{},
		CompositeFunctions:  map[string]map[string]FunctionValue{},
		DestructorFunctions: map[string]*InterpretedFunctionValue{},
		SubInterpreters:     map[ast.LocationID]*Interpreter{},
		onEventEmitted:      func(EventValue) {},
	}

	for name, value := range predefinedValues {
		err := interpreter.ImportValue(name, value)
		if err != nil {
			return nil, err
		}
	}

	for _, opt := range opts {
		opt(interpreter)
	}

	return interpreter, nil
}

// SetOnEventEmitted registers a callback that is triggered when an event is emitted by the program.
//
func (interpreter *Interpreter) SetOnEventEmitted(callback func(EventValue)) {
	interpreter.onEventEmitted = callback
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
	defer func() {
		if r := recover(); r != nil {
			var ok bool
			// don't recover Go errors
			err, ok = r.(goRuntime.Error)
			if ok {
				panic(err)
			}
			err, ok = r.(error)
			if !ok {
				err = fmt.Errorf("%v", r)
			}
		}
	}()

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

	// pre-declare empty variables for all structures, interfaces, and function declarations
	for _, declaration := range program.InterfaceDeclarations() {
		interpreter.declareVariable(declaration.Identifier.Identifier, nil)
	}
	for _, declaration := range program.CompositeDeclarations() {
		interpreter.declareVariable(declaration.Identifier.Identifier, nil)
	}
	for _, declaration := range program.FunctionDeclarations() {
		interpreter.declareVariable(declaration.Identifier.Identifier, nil)
	}
	for _, declaration := range program.InterfaceDeclarations() {
		interpreter.declareInterface(declaration)
	}
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
	name := declaration.DeclarationName()
	// NOTE: semantic analysis already checked possible invalid redeclaration
	interpreter.Globals[name] = interpreter.findVariable(name)
}

func (interpreter *Interpreter) prepareInvoke(functionName string, arguments []interface{}) (trampoline Trampoline, err error) {
	variable, ok := interpreter.Globals[functionName]
	if !ok {
		return nil, &NotDeclaredError{
			ExpectedKind: common.DeclarationKindFunction,
			Name:         functionName,
		}
	}

	variableValue := variable.Value

	function, ok := variableValue.(FunctionValue)
	if !ok {
		return nil, &NotInvokableError{
			Value: variableValue,
		}
	}

	var argumentValues []Value
	argumentValues, err = ToValues(arguments)
	if err != nil {
		return nil, err
	}

	// ensures the invocation's argument count matches the function's parameter count

	ty := interpreter.Checker.GlobalValues[functionName].Type

	invokableType, ok := ty.(sema.InvokableType)

	if !ok {
		return nil, &NotInvokableError{
			Value: variableValue,
		}
	}

	functionType := invokableType.InvocationFunctionType()

	parameterTypeAnnotations := functionType.ParameterTypeAnnotations
	parameterCount := len(parameterTypeAnnotations)
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

	boxedArguments := make([]Value, len(arguments))
	for i, argument := range argumentValues {
		parameterType := parameterTypeAnnotations[i].Type
		// TODO: value type is not known – only used for Any boxing right now, so reject for now
		if parameterType.Equal(&sema.AnyType{}) {
			return nil, &NotInvokableError{
				Value: variableValue,
			}
		}
		boxedArguments[i] = interpreter.box(argument, nil, parameterType)
	}

	trampoline = function.invoke(boxedArguments, LocationPosition{})
	return trampoline, nil
}

func (interpreter *Interpreter) Invoke(functionName string, arguments ...interface{}) (value Value, err error) {
	// recover internal panics and return them as an error
	defer func() {
		if r := recover(); r != nil {
			var ok bool
			// don't recover Go errors
			err, ok = r.(goRuntime.Error)
			if ok {
				panic(err)
			}
			err, ok = r.(error)
			if !ok {
				err = fmt.Errorf("%v", r)
			}
		}
	}()

	trampoline, err := interpreter.prepareInvoke(functionName, arguments)
	if err != nil {
		return nil, err
	}
	result := interpreter.runAllStatements(trampoline)
	if result == nil {
		return nil, nil
	}
	return result.(Value), nil
}

func (interpreter *Interpreter) InvokeExportable(
	functionName string,
	arguments ...interface{},
) (
	value ExportableValue,
	err error,
) {
	result, err := interpreter.Invoke(functionName, arguments...)
	if err != nil {
		return nil, err
	}

	if result == nil {
		return nil, nil
	}

	return result.(ExportableValue), nil
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

	functionExpression := declaration.ToExpression()
	variable.Value = newInterpretedFunction(
		interpreter,
		functionExpression,
		functionType,
		lexicalScope,
	)

	// NOTE: no result, so it does *not* act like a return-statement
	return Done{}
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

func (interpreter *Interpreter) VisitFunctionBlock(functionBlock *ast.FunctionBlock) ast.Repr {
	// NOTE: see visitFunctionBlock
	panic(&errors.UnreachableError{})
}

func (interpreter *Interpreter) visitFunctionBlock(functionBlock *ast.FunctionBlock, returnType sema.Type) Trampoline {

	// block scope: each function block gets an activation record
	interpreter.activations.PushCurrent()

	beforeStatements, rewrittenPostConditions :=
		interpreter.rewritePostConditions(functionBlock)

	return interpreter.visitStatements(beforeStatements).
		FlatMap(func(_ interface{}) Trampoline {
			return interpreter.visitConditions(functionBlock.PreConditions)
		}).
		FlatMap(func(_ interface{}) Trampoline {
			// NOTE: not interpreting block as it enters a new scope
			// and post-conditions need to be able to refer to block's declarations
			return interpreter.visitStatements(functionBlock.Block.Statements).
				FlatMap(func(blockResult interface{}) Trampoline {

					var resultValue Value
					if _, ok := blockResult.(functionReturn); ok {
						resultValue = blockResult.(functionReturn).Value
					} else {
						resultValue = VoidValue{}
					}

					// if there is a return type, declare the constant `result`
					// which has the return value

					if _, isVoid := returnType.(*sema.VoidType); !isVoid {
						interpreter.declareVariable(sema.ResultIdentifier, resultValue)
					}

					return interpreter.visitConditions(rewrittenPostConditions).
						Map(func(_ interface{}) interface{} {
							return resultValue
						})
				})
		}).
		Then(func(_ interface{}) {
			interpreter.activations.Pop()
		})
}

func (interpreter *Interpreter) rewritePostConditions(functionBlock *ast.FunctionBlock) (
	beforeStatements []ast.Statement,
	rewrittenPostConditions []*ast.Condition,
) {
	beforeExtractor := NewBeforeExtractor()

	rewrittenPostConditions = make([]*ast.Condition, len(functionBlock.PostConditions))

	for i, postCondition := range functionBlock.PostConditions {

		// copy condition and set expression to rewritten one
		newPostCondition := *postCondition

		testExtraction := beforeExtractor.ExtractBefore(postCondition.Test)

		extractedExpressions := testExtraction.ExtractedExpressions

		newPostCondition.Test = testExtraction.RewrittenExpression

		if postCondition.Message != nil {
			messageExtraction := beforeExtractor.ExtractBefore(postCondition.Message)

			newPostCondition.Message = messageExtraction.RewrittenExpression

			extractedExpressions = append(
				extractedExpressions,
				messageExtraction.ExtractedExpressions...,
			)
		}

		for _, extractedExpression := range extractedExpressions {

			// TODO: update interpreter.Checker.Elaboration
			//    VariableDeclarationValueTypes / VariableDeclarationTargetTypes

			beforeStatements = append(beforeStatements,
				&ast.VariableDeclaration{
					Identifier: extractedExpression.Identifier,
					Value:      extractedExpression.Expression,
				},
			)
		}

		rewrittenPostConditions[i] = &newPostCondition
	}

	return beforeStatements, rewrittenPostConditions
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
						message := result.(StringValue).StrValue()

						panic(&ConditionError{
							ConditionKind: condition.Kind,
							Message:       message,
							LocationRange: LocationRange{
								Location: interpreter.Checker.Location,
								Range: ast.Range{
									StartPos: condition.Test.StartPosition(),
									EndPos:   condition.Test.EndPosition(),
								},
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

			value = interpreter.box(value, valueType, returnType)

			return functionReturn{value}
		})
}

func (interpreter *Interpreter) VisitBreakStatement(statement *ast.BreakStatement) ast.Repr {
	return Done{Result: loopBreak{}}
}

func (interpreter *Interpreter) VisitContinueStatement(statement *ast.ContinueStatement) ast.Repr {
	return Done{Result: loopContinue{}}
}

func (interpreter *Interpreter) VisitIfStatement(statement *ast.IfStatement) ast.Repr {
	switch test := statement.Test.(type) {
	case ast.Expression:
		return interpreter.visitIfStatementWithTestExpression(test, statement.Then, statement.Else)
	case *ast.VariableDeclaration:
		return interpreter.visitIfStatementWithVariableDeclaration(test, statement.Then, statement.Else)
	default:
		panic(&errors.UnreachableError{})
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

			if someValue, ok := result.(SomeValue); ok {
				unwrappedValueCopy := someValue.Value.Copy()
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
					} else if _, ok := value.(loopContinue); ok {
						// NO-OP
					} else if functionReturn, ok := value.(functionReturn); ok {
						return Done{Result: functionReturn}
					}

					// recurse
					return statement.Accept(interpreter).(Trampoline)
				})
		})
}

// VisitVariableDeclaration first visits the declaration's value,
// then declares the variable with the name bound to the value
func (interpreter *Interpreter) VisitVariableDeclaration(declaration *ast.VariableDeclaration) ast.Repr {
	return declaration.Value.Accept(interpreter).(Trampoline).
		FlatMap(func(result interface{}) Trampoline {

			valueType := interpreter.Checker.Elaboration.VariableDeclarationValueTypes[declaration]
			targetType := interpreter.Checker.Elaboration.VariableDeclarationTargetTypes[declaration]

			valueCopy := interpreter.copyAndBox(result.(Value), valueType, targetType)

			interpreter.declareVariable(
				declaration.Identifier.Identifier,
				valueCopy,
			)

			// NOTE: ignore result, so it does *not* act like a return-statement
			return Done{}
		})
}

func (interpreter *Interpreter) declareVariable(identifier string, value Value) *Variable {
	// NOTE: semantic analysis already checked possible invalid redeclaration
	variable := &Variable{Value: value}
	interpreter.setVariable(identifier, variable)
	return variable
}

func (interpreter *Interpreter) VisitAssignmentStatement(assignment *ast.AssignmentStatement) ast.Repr {
	return assignment.Value.Accept(interpreter).(Trampoline).
		FlatMap(func(result interface{}) Trampoline {

			valueType := interpreter.Checker.Elaboration.AssignmentStatementValueTypes[assignment]
			targetType := interpreter.Checker.Elaboration.AssignmentStatementTargetTypes[assignment]

			valueCopy := interpreter.copyAndBox(result.(Value), valueType, targetType)

			return interpreter.visitAssignmentValue(assignment.Target, valueCopy)
		})
}

func (interpreter *Interpreter) VisitSwapStatement(swap *ast.SwapStatement) ast.Repr {
	// Evaluate the left expression
	return swap.Left.Accept(interpreter).(Trampoline).
		FlatMap(func(result interface{}) Trampoline {
			leftValue := result.(Value)

			// Evaluate the right expression
			return swap.Right.Accept(interpreter).(Trampoline).
				FlatMap(func(result interface{}) Trampoline {
					rightValue := result.(Value)

					// Assign the right-hand side value to the left-hand side
					return interpreter.visitAssignmentValue(swap.Left, rightValue).
						FlatMap(func(_ interface{}) Trampoline {

							// Assign the left-hand side value to the right-hand side
							return interpreter.visitAssignmentValue(swap.Right, leftValue)
						})
				})
		})
}

func (interpreter *Interpreter) visitAssignmentValue(target ast.Expression, value Value) Trampoline {
	switch target := target.(type) {
	case *ast.IdentifierExpression:
		return interpreter.visitIdentifierExpressionAssignment(target, value)

	case *ast.IndexExpression:
		return interpreter.visitIndexExpressionAssignment(target, value)

	case *ast.MemberExpression:
		return interpreter.visitMemberExpressionAssignment(target, value)
	}

	panic(&errors.UnreachableError{})
}

func (interpreter *Interpreter) visitIdentifierExpressionAssignment(target *ast.IdentifierExpression, value Value) Trampoline {
	variable := interpreter.findVariable(target.Identifier.Identifier)
	variable.Value = value
	// NOTE: no result, so it does *not* act like a return-statement
	return Done{}
}

func (interpreter *Interpreter) visitIndexExpressionAssignment(target *ast.IndexExpression, value Value) Trampoline {
	return target.TargetExpression.Accept(interpreter).(Trampoline).
		FlatMap(func(result interface{}) Trampoline {
			switch typedResult := result.(type) {
			case ValueIndexableValue:
				return target.IndexingExpression.Accept(interpreter).(Trampoline).
					FlatMap(func(result interface{}) Trampoline {
						indexingValue := result.(Value)
						locationRange := interpreter.locationRange(target)
						typedResult.Set(locationRange, indexingValue, value)

						// NOTE: no result, so it does *not* act like a return-statement
						return Done{}
					})

			case TypeIndexableValue:
				indexingType := interpreter.Checker.Elaboration.IndexExpressionIndexingTypes[target]
				typedResult.Set(indexingType, value)
				return Done{}

			default:
				panic(&errors.UnreachableError{})
			}
		})
}

func (interpreter *Interpreter) visitMemberExpressionAssignment(target *ast.MemberExpression, value Value) Trampoline {
	return target.Expression.Accept(interpreter).(Trampoline).
		FlatMap(func(result interface{}) Trampoline {
			structure := result.(MemberAccessibleValue)
			locationRange := interpreter.locationRange(target)
			structure.SetMember(interpreter, locationRange, target.Identifier.Identifier, value)

			// NOTE: no result, so it does *not* act like a return-statement
			return Done{}
		})
}

func (interpreter *Interpreter) VisitIdentifierExpression(expression *ast.IdentifierExpression) ast.Repr {
	variable := interpreter.findVariable(expression.Identifier.Identifier)
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
				left := tuple.left.(IntegerValue)
				right := tuple.right.(IntegerValue)
				return left.Plus(right)
			})

	case ast.OperationMinus:
		return interpreter.visitBinaryOperation(expression).
			Map(func(result interface{}) interface{} {
				tuple := result.(valueTuple)
				left := tuple.left.(IntegerValue)
				right := tuple.right.(IntegerValue)
				return left.Minus(right)
			})

	case ast.OperationMod:
		return interpreter.visitBinaryOperation(expression).
			Map(func(result interface{}) interface{} {
				tuple := result.(valueTuple)
				left := tuple.left.(IntegerValue)
				right := tuple.right.(IntegerValue)
				return left.Mod(right)
			})

	case ast.OperationMul:
		return interpreter.visitBinaryOperation(expression).
			Map(func(result interface{}) interface{} {
				tuple := result.(valueTuple)
				left := tuple.left.(IntegerValue)
				right := tuple.right.(IntegerValue)
				return left.Mul(right)
			})

	case ast.OperationDiv:
		return interpreter.visitBinaryOperation(expression).
			Map(func(result interface{}) interface{} {
				tuple := result.(valueTuple)
				left := tuple.left.(IntegerValue)
				right := tuple.right.(IntegerValue)
				return left.Div(right)
			})

	case ast.OperationLess:
		return interpreter.visitBinaryOperation(expression).
			Map(func(result interface{}) interface{} {
				tuple := result.(valueTuple)
				left := tuple.left.(IntegerValue)
				right := tuple.right.(IntegerValue)
				return left.Less(right)
			})

	case ast.OperationLessEqual:
		return interpreter.visitBinaryOperation(expression).
			Map(func(result interface{}) interface{} {
				tuple := result.(valueTuple)
				left := tuple.left.(IntegerValue)
				right := tuple.right.(IntegerValue)
				return left.LessEqual(right)
			})

	case ast.OperationGreater:
		return interpreter.visitBinaryOperation(expression).
			Map(func(result interface{}) interface{} {
				tuple := result.(valueTuple)
				left := tuple.left.(IntegerValue)
				right := tuple.right.(IntegerValue)
				return left.Greater(right)
			})

	case ast.OperationGreaterEqual:
		return interpreter.visitBinaryOperation(expression).
			Map(func(result interface{}) interface{} {
				tuple := result.(valueTuple)
				left := tuple.left.(IntegerValue)
				right := tuple.right.(IntegerValue)
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

							// NOTE: important to box both any and optional
							return interpreter.box(value, rightType, resultType)
						})
				}

				value := left.(SomeValue).Value
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
		Range: ast.Range{
			StartPos: expression.StartPosition(),
			EndPos:   expression.EndPosition(),
		},
	})
}

func (interpreter *Interpreter) testEqual(left, right Value) BoolValue {
	left = interpreter.unbox(left)
	right = interpreter.unbox(right)

	switch left := left.(type) {
	case IntegerValue:
		// NOTE: might be NilValue
		right, ok := right.(IntegerValue)
		if !ok {
			return false
		}
		return left.Equal(right)

	case BoolValue:
		return BoolValue(left == right)

	case NilValue:
		_, ok := right.(NilValue)
		return BoolValue(ok)

	case StringValue:
		// NOTE: might be NilValue
		right, ok := right.(StringValue)
		if !ok {
			return false
		}
		return left.Equal(right)
	}

	panic(&errors.UnreachableError{})
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
				integerValue := value.(IntegerValue)
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

func (interpreter *Interpreter) VisitNilExpression(expression *ast.NilExpression) ast.Repr {
	value := NilValue{}
	return Done{Result: value}
}

func (interpreter *Interpreter) VisitIntExpression(expression *ast.IntExpression) ast.Repr {
	value := IntValue{expression.Value}

	return Done{Result: value}
}

func (interpreter *Interpreter) VisitStringExpression(expression *ast.StringExpression) ast.Repr {
	value := NewStringValue(expression.Value)

	return Done{Result: value}
}

func (interpreter *Interpreter) VisitArrayExpression(expression *ast.ArrayExpression) ast.Repr {
	return interpreter.visitExpressions(expression.Values).
		FlatMap(func(result interface{}) Trampoline {
			values := result.(ArrayValue)

			argumentTypes := interpreter.Checker.Elaboration.ArrayExpressionArgumentTypes[expression]
			elementType := interpreter.Checker.Elaboration.ArrayExpressionElementType[expression]

			copies := make([]Value, len(*values.Values))
			for i, argument := range *values.Values {
				argumentType := argumentTypes[i]
				copies[i] = interpreter.copyAndBox(argument, argumentType, elementType)
			}

			return Done{Result: NewArrayValue(copies...)}
		})
}

func (interpreter *Interpreter) VisitDictionaryExpression(expression *ast.DictionaryExpression) ast.Repr {
	return interpreter.visitEntries(expression.Entries).
		FlatMap(func(result interface{}) Trampoline {

			entryTypes := interpreter.Checker.Elaboration.DictionaryExpressionEntryTypes[expression]
			dictionaryType := interpreter.Checker.Elaboration.DictionaryExpressionType[expression]

			newDictionary := DictionaryValue{}
			for i, dictionaryEntryValues := range result.([]DictionaryEntryValues) {
				entryType := entryTypes[i]

				key := interpreter.copyAndBox(
					dictionaryEntryValues.Key,
					entryType.KeyType,
					dictionaryType.KeyType,
				)

				value := interpreter.copyAndBox(
					dictionaryEntryValues.Value,
					entryType.ValueType,
					dictionaryType.ValueType,
				)

				// TODO: improve: should be just for current entry
				locationRange := interpreter.locationRange(expression)

				// TODO: panic for duplicate keys?

				// NOTE: important to box in optional, as assignment to dictionary
				// is always considered as an optional

				newDictionary.Set(locationRange, key, SomeValue{value})
			}

			return Done{Result: newDictionary}
		})
}

func (interpreter *Interpreter) VisitMemberExpression(expression *ast.MemberExpression) ast.Repr {
	return expression.Expression.Accept(interpreter).(Trampoline).
		Map(func(result interface{}) interface{} {
			value := result.(MemberAccessibleValue)
			locationRange := interpreter.locationRange(expression)
			return value.GetMember(interpreter, locationRange, expression.Identifier.Identifier)
		})
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
						value := typedResult.Get(locationRange, indexingValue)
						return Done{Result: value}
					})

			case TypeIndexableValue:
				indexingType := interpreter.Checker.Elaboration.IndexExpressionIndexingTypes[expression]
				result := typedResult.Get(indexingType)
				return Done{Result: result}

			default:
				panic(&errors.UnreachableError{})
			}
		})
}

func (interpreter *Interpreter) VisitConditionalExpression(expression *ast.ConditionalExpression) ast.Repr {
	return expression.Test.Accept(interpreter).(Trampoline).
		FlatMap(func(result interface{}) Trampoline {
			value := result.(BoolValue)

			if value {
				return expression.Then.Accept(interpreter).(Trampoline)
			} else {
				return expression.Else.Accept(interpreter).(Trampoline)
			}
		})
}

func (interpreter *Interpreter) VisitInvocationExpression(invocationExpression *ast.InvocationExpression) ast.Repr {
	// interpret the invoked expression
	return invocationExpression.InvokedExpression.Accept(interpreter).(Trampoline).
		FlatMap(func(result interface{}) Trampoline {
			function := result.(FunctionValue)

			// NOTE: evaluate all argument expressions in call-site scope, not in function body
			argumentExpressions := make([]ast.Expression, len(invocationExpression.Arguments))
			for i, argument := range invocationExpression.Arguments {
				argumentExpressions[i] = argument.Expression
			}

			return interpreter.visitExpressions(argumentExpressions).
				FlatMap(func(result interface{}) Trampoline {
					arguments := result.(ArrayValue)

					argumentTypes :=
						interpreter.Checker.Elaboration.InvocationExpressionArgumentTypes[invocationExpression]
					parameterTypes :=
						interpreter.Checker.Elaboration.InvocationExpressionParameterTypes[invocationExpression]

					argumentCopies := make([]Value, len(*arguments.Values))
					for i, argument := range *arguments.Values {
						argumentType := argumentTypes[i]
						parameterType := parameterTypes[i]
						argumentCopies[i] = interpreter.copyAndBox(argument, argumentType, parameterType)
					}

					// TODO: optimize: only potentially used by host-functions
					location := LocationPosition{
						Position: invocationExpression.StartPosition(),
						Location: interpreter.Checker.Location,
					}
					return function.invoke(argumentCopies, location)
				})
		})
}

func (interpreter *Interpreter) invokeInterpretedFunction(
	function InterpretedFunctionValue,
	arguments []Value,
) Trampoline {

	// start a new activation record
	// lexical scope: use the function declaration's activation record,
	// not the current one (which would be dynamic scope)
	interpreter.activations.Push(function.Activation)

	return interpreter.invokeInterpretedFunctionActivated(function, arguments)
}

// NOTE: assumes the function's activation (or an extension of it) is pushed!
//
func (interpreter *Interpreter) invokeInterpretedFunctionActivated(
	function InterpretedFunctionValue,
	arguments []Value,
) Trampoline {

	interpreter.bindFunctionInvocationParameters(function, arguments)

	functionBlockTrampoline := interpreter.visitFunctionBlock(
		function.Expression.FunctionBlock,
		function.Type.ReturnTypeAnnotation.Type,
	)

	return functionBlockTrampoline.
		Then(func(_ interface{}) {
			interpreter.activations.Pop()
		})
}

// bindFunctionInvocationParameters binds the argument values to the parameters in the function
func (interpreter *Interpreter) bindFunctionInvocationParameters(
	function InterpretedFunctionValue,
	arguments []Value,
) {
	if function.Expression.ParameterList == nil {
		return
	}

	for parameterIndex, parameter := range function.Expression.ParameterList.Parameters {
		argument := arguments[parameterIndex]
		interpreter.declareVariable(parameter.Identifier.Identifier, argument)
	}
}

func (interpreter *Interpreter) visitExpressions(expressions []ast.Expression) Trampoline {
	var trampoline Trampoline = Done{Result: NewArrayValue()}

	for _, expression := range expressions {
		// NOTE: important: rebind expression, because it is captured in the closure below
		expression := expression

		// append the evaluation of this expression
		trampoline = trampoline.FlatMap(func(result interface{}) Trampoline {
			array := result.(ArrayValue)

			// evaluate the expression
			return expression.Accept(interpreter).(Trampoline).
				FlatMap(func(result interface{}) Trampoline {
					value := result.(Value)

					newValues := append(*array.Values, value)
					return Done{Result: NewArrayValue(newValues...)}
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

	function := newInterpretedFunction(interpreter, expression, functionType, lexicalScope)

	return Done{Result: function}
}

func (interpreter *Interpreter) VisitCompositeDeclaration(declaration *ast.CompositeDeclaration) ast.Repr {

	interpreter.declareCompositeConstructor(declaration)

	// NOTE: no result, so it does *not* act like a return-statement
	return Done{}
}

// declareCompositeConstructor creates a constructor function
// for the given composite, bound in a variable.
//
// The constructor is a host function which creates a new composite,
// calls the initializer (interpreted function), if any,
// and then returns the composite.
//
// Inside the initializer and all functions, `self` is bound to
// the new composite value, and the constructor itself is bound
//
func (interpreter *Interpreter) declareCompositeConstructor(declaration *ast.CompositeDeclaration) {

	// lexical scope: variables in functions are bound to what is visible at declaration time
	lexicalScope := interpreter.activations.CurrentOrNew()

	identifier := declaration.Identifier.Identifier
	variable := interpreter.findOrDeclareVariable(identifier)

	// make the constructor available in the initializer
	lexicalScope = lexicalScope.
		Insert(common.StringEntry(identifier), variable)

	initializerFunction := interpreter.initializerFunction(declaration, lexicalScope)

	destructorFunction := interpreter.destructorFunction(declaration, lexicalScope)
	interpreter.DestructorFunctions[identifier] = destructorFunction

	functions := interpreter.compositeFunctions(declaration, lexicalScope)
	interpreter.CompositeFunctions[identifier] = functions

	variable.Value = NewHostFunctionValue(
		func(arguments []Value, location LocationPosition) Trampoline {

			value := CompositeValue{
				Location:   interpreter.Checker.Location,
				Identifier: identifier,
				Fields:     &map[string]Value{},
				Functions:  &functions,
				Destructor: destructorFunction,
			}

			var initializationTrampoline Trampoline = Done{}

			if initializerFunction != nil {
				// NOTE: arguments are already properly boxed by invocation expression

				initializationTrampoline = interpreter.bindSelf(*initializerFunction, value).
					invoke(arguments, location)
			}

			return initializationTrampoline.
				Map(func(_ interface{}) interface{} {
					return value
				})
		},
	)
}

// bindSelf returns a function which binds `self` to the structure
//
func (interpreter *Interpreter) bindSelf(
	function InterpretedFunctionValue,
	structure CompositeValue,
) FunctionValue {
	return NewHostFunctionValue(func(arguments []Value, location LocationPosition) Trampoline {
		// start a new activation record
		// lexical scope: use the function declaration's activation record,
		// not the current one (which would be dynamic scope)
		interpreter.activations.Push(function.Activation)

		// make `self` available
		interpreter.declareVariable(sema.SelfIdentifier, structure)

		return interpreter.invokeInterpretedFunctionActivated(function, arguments)
	})
}

func (interpreter *Interpreter) initializerFunction(
	compositeDeclaration *ast.CompositeDeclaration,
	lexicalScope hamt.Map,
) *InterpretedFunctionValue {

	// NOTE: gather all conformances' preconditions and postconditions,
	// even if the composite declaration does not have an initializer

	var preConditions []*ast.Condition
	var postConditions []*ast.Condition

	for _, conformance := range compositeDeclaration.Conformances {
		interfaceDeclaration := interpreter.interfaces[conformance.Identifier.Identifier]

		// TODO: support multiple overloaded initializers

		initializers := interfaceDeclaration.Members.Initializers()
		if len(initializers) == 0 {
			continue
		}

		firstInitializer := initializers[0]
		if firstInitializer == nil || firstInitializer.FunctionBlock == nil {
			continue
		}

		preConditions = append(
			preConditions,
			firstInitializer.FunctionBlock.PreConditions...,
		)

		postConditions = append(
			postConditions,
			firstInitializer.FunctionBlock.PostConditions...,
		)
	}

	var function *ast.FunctionExpression
	var functionType *sema.FunctionType

	initializers := compositeDeclaration.Members.Initializers()
	if len(initializers) > 0 {
		// TODO: support multiple overloaded initializers

		firstInitializer := initializers[0]

		function = firstInitializer.ToExpression()

		// copy function block – this makes rewriting the conditions safe
		functionBlockCopy := *function.FunctionBlock
		function.FunctionBlock = &functionBlockCopy

		functionType = interpreter.Checker.Elaboration.SpecialFunctionTypes[firstInitializer].FunctionType
	} else if len(preConditions) > 0 || len(postConditions) > 0 {

		// no initializer, but preconditions or postconditions from conformances,
		// prepare a function expression just for those

		// NOTE: the preconditions and postconditions are added below

		function = &ast.FunctionExpression{
			FunctionBlock: &ast.FunctionBlock{},
		}

		functionType = emptyFunctionType
	}

	// no initializer in the composite declaration and also
	// no preconditions or postconditions in the conformances: no need for initializer

	if function == nil {
		return nil
	}

	// prepend the conformances' preconditions and postconditions, if any

	function.FunctionBlock.PreConditions = append(preConditions, function.FunctionBlock.PreConditions...)
	function.FunctionBlock.PostConditions = append(postConditions, function.FunctionBlock.PostConditions...)

	result := newInterpretedFunction(
		interpreter,
		function,
		functionType,
		lexicalScope,
	)
	return &result
}

func (interpreter *Interpreter) destructorFunction(
	compositeDeclaration *ast.CompositeDeclaration,
	lexicalScope hamt.Map,
) *InterpretedFunctionValue {

	// NOTE: gather all conformances' preconditions and postconditions,
	// even if the composite declaration does not have a destructor

	var preConditions []*ast.Condition
	var postConditions []*ast.Condition

	for _, conformance := range compositeDeclaration.Conformances {
		conformanceIdentifier := conformance.Identifier.Identifier
		interfaceDeclaration := interpreter.interfaces[conformanceIdentifier]
		interfaceDestructor := interfaceDeclaration.Members.Destructor()
		if interfaceDestructor == nil || interfaceDestructor.FunctionBlock == nil {
			continue
		}

		preConditions = append(
			preConditions,
			interfaceDestructor.FunctionBlock.PreConditions...,
		)

		postConditions = append(
			postConditions,
			interfaceDestructor.FunctionBlock.PostConditions...,
		)
	}

	var function *ast.FunctionExpression

	destructor := compositeDeclaration.Members.Destructor()
	if destructor != nil {

		function = destructor.ToExpression()

		// copy function block – this makes rewriting the conditions safe
		functionBlockCopy := *function.FunctionBlock
		function.FunctionBlock = &functionBlockCopy

	} else if len(preConditions) > 0 || len(postConditions) > 0 {

		// no destructor, but preconditions or postconditions from conformances,
		// prepare a function expression just for those

		// NOTE: the preconditions and postconditions are added below

		function = &ast.FunctionExpression{
			FunctionBlock: &ast.FunctionBlock{},
		}
	}

	// no destructor in the resource declaration and also
	// no preconditions or postconditions in the conformances: no need for destructor

	if function == nil {
		return nil
	}

	// prepend the conformances' preconditions and postconditions, if any

	function.FunctionBlock.PreConditions = append(preConditions, function.FunctionBlock.PreConditions...)
	function.FunctionBlock.PostConditions = append(postConditions, function.FunctionBlock.PostConditions...)

	result := newInterpretedFunction(
		interpreter,
		function,
		emptyFunctionType,
		lexicalScope,
	)
	return &result
}

func (interpreter *Interpreter) compositeFunctions(
	compositeDeclaration *ast.CompositeDeclaration,
	lexicalScope hamt.Map,
) map[string]FunctionValue {

	functions := map[string]FunctionValue{}

	for _, functionDeclaration := range compositeDeclaration.Members.Functions {
		functionType := interpreter.Checker.Elaboration.FunctionDeclarationFunctionTypes[functionDeclaration]

		function := interpreter.compositeFunction(functionDeclaration, compositeDeclaration.Conformances)

		functions[functionDeclaration.Identifier.Identifier] =
			newInterpretedFunction(
				interpreter,
				function,
				functionType,
				lexicalScope,
			)
	}

	return functions
}

func (interpreter *Interpreter) compositeFunction(
	functionDeclaration *ast.FunctionDeclaration,
	conformances []*ast.NominalType,
) *ast.FunctionExpression {

	functionIdentifier := functionDeclaration.Identifier.Identifier

	function := functionDeclaration.ToExpression()

	// copy function block, append interfaces' pre-conditions and post-condition
	functionBlockCopy := *function.FunctionBlock
	function.FunctionBlock = &functionBlockCopy

	for _, conformance := range conformances {
		conformanceIdentifier := conformance.Identifier.Identifier
		interfaceDeclaration := interpreter.interfaces[conformanceIdentifier]
		interfaceFunction, ok := interfaceDeclaration.Members.FunctionsByIdentifier()[functionIdentifier]
		if !ok || interfaceFunction.FunctionBlock == nil {
			continue
		}

		functionBlockCopy.PreConditions = append(
			functionBlockCopy.PreConditions,
			interfaceFunction.FunctionBlock.PreConditions...,
		)

		functionBlockCopy.PostConditions = append(
			functionBlockCopy.PostConditions,
			interfaceFunction.FunctionBlock.PostConditions...,
		)
	}

	return function
}

func (interpreter *Interpreter) VisitFieldDeclaration(field *ast.FieldDeclaration) ast.Repr {
	// fields can't be interpreted
	panic(&errors.UnreachableError{})
}

func (interpreter *Interpreter) copyAndBox(value Value, valueType, targetType sema.Type) Value {
	if valueType == nil || !valueType.IsResourceType() {
		value = value.Copy()
	}
	return interpreter.box(value, valueType, targetType)
}

// box boxes a value in optionals and any value, if necessary
func (interpreter *Interpreter) box(value Value, valueType, targetType sema.Type) Value {
	value, valueType = interpreter.boxOptional(value, valueType, targetType)
	return interpreter.boxAny(value, valueType, targetType)
}

// boxOptional boxes a value in optionals, if necessary
func (interpreter *Interpreter) boxOptional(value Value, valueType, targetType sema.Type) (Value, sema.Type) {
	inner := value
	for {
		optionalType, ok := targetType.(*sema.OptionalType)
		if !ok {
			break
		}

		if some, ok := inner.(SomeValue); ok {
			inner = some.Value
		} else if _, ok := inner.(NilValue); ok {
			// NOTE: nested nil will be unboxed!
			return inner, &sema.OptionalType{
				Type: &sema.NeverType{},
			}
		} else {
			value = SomeValue{Value: value}
			valueType = &sema.OptionalType{
				Type: valueType,
			}
		}

		targetType = optionalType.Type
	}
	return value, valueType
}

// boxOptional boxes a value in an Any value, if necessary
func (interpreter *Interpreter) boxAny(value Value, valueType, targetType sema.Type) Value {
	switch targetType := targetType.(type) {
	case *sema.AnyType:
		// no need to box already boxed value
		if _, ok := value.(AnyValue); ok {
			return value
		}
		return AnyValue{
			Value: value,
			Type:  valueType,
		}

	case *sema.OptionalType:
		if _, ok := value.(NilValue); ok {
			return value
		}
		some := value.(SomeValue)
		return SomeValue{
			Value: interpreter.boxAny(
				some.Value,
				valueType.(*sema.OptionalType).Type,
				targetType.Type,
			),
		}

	// TODO: support more types, e.g. arrays, dictionaries
	default:
		return value
	}
}

func (interpreter *Interpreter) unbox(value Value) Value {
	for {
		some, ok := value.(SomeValue)
		if !ok {
			return value
		}

		value = some.Value
	}
}

func (interpreter *Interpreter) VisitInterfaceDeclaration(declaration *ast.InterfaceDeclaration) ast.Repr {
	return Done{}
}

func (interpreter *Interpreter) declareInterface(declaration *ast.InterfaceDeclaration) {
	interpreter.interfaces[declaration.Identifier.Identifier] = declaration
}

func (interpreter *Interpreter) VisitImportDeclaration(declaration *ast.ImportDeclaration) ast.Repr {
	importedChecker := interpreter.Checker.ImportCheckers[declaration.Location.ID()]

	subInterpreter, err := NewInterpreter(
		importedChecker,
		interpreter.PredefinedValues,
		WithOnEventEmittedHandler(interpreter.onEventEmitted),
	)
	if err != nil {
		panic(err)
	}

	if subInterpreter.Checker.Location == nil {
		subInterpreter.Checker.Location = declaration.Location
	}

	interpreter.SubInterpreters[declaration.Location.ID()] = subInterpreter

	return subInterpreter.interpret().
		Then(func(_ interface{}) {

			for subSubImportLocation, subSubInterpreter := range subInterpreter.SubInterpreters {
				interpreter.SubInterpreters[subSubImportLocation] = subSubInterpreter
			}

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

				interpreter.setVariable(name, variable)

				// if the imported name refers to a composite, also take the composite functions
				// and the destructor function from the sub-interpreter

				if compositeFunctions, ok := subInterpreter.CompositeFunctions[name]; ok {
					interpreter.CompositeFunctions[name] = compositeFunctions
				}

				if destructorFunction, ok := subInterpreter.DestructorFunctions[name]; ok {
					interpreter.DestructorFunctions[name] = destructorFunction
				}
			}
		})
}

func (interpreter *Interpreter) VisitEventDeclaration(declaration *ast.EventDeclaration) ast.Repr {
	interpreter.declareEventConstructor(declaration)

	// NOTE: no result, so it does *not* act like a return-statement
	return Done{}
}

// declareEventConstructor declares the constructor function for an event type.
//
// The constructor is assigned to a variable with the same identifier as the event type itself.
// For example, this allows an event instance for event type MyEvent(x: Int) to be created
// by calling MyEvent(x: 2).
func (interpreter *Interpreter) declareEventConstructor(declaration *ast.EventDeclaration) {
	identifier := declaration.Identifier.Identifier

	eventType := interpreter.Checker.Elaboration.EventDeclarationTypes[declaration]

	variable := interpreter.findOrDeclareVariable(identifier)
	variable.Value = NewHostFunctionValue(
		func(arguments []Value, location LocationPosition) Trampoline {
			fields := make([]EventField, len(eventType.Fields))
			for i, field := range eventType.Fields {
				fields[i] = EventField{
					Identifier: field.Identifier,
					Value:      arguments[i],
				}
			}

			value := EventValue{
				ID:       eventType.Identifier,
				Fields:   fields,
				Location: interpreter.Checker.Location,
			}

			return Done{Result: value}
		},
	)
}

func (interpreter *Interpreter) VisitEmitStatement(statement *ast.EmitStatement) ast.Repr {
	return statement.InvocationExpression.Accept(interpreter).(Trampoline).
		FlatMap(func(result interface{}) Trampoline {
			event := result.(EventValue)

			interpreter.onEventEmitted(event)

			// NOTE: no result, so it does *not* act like a return-statement
			return Done{}
		})
}

func (interpreter *Interpreter) VisitFailableDowncastExpression(expression *ast.FailableDowncastExpression) ast.Repr {
	return expression.Expression.Accept(interpreter).(Trampoline).
		Map(func(result interface{}) interface{} {
			value := result.(Value)

			anyValue := value.(AnyValue)
			expectedType := interpreter.Checker.Elaboration.FailableDowncastingTypes[expression]

			if !sema.IsSubType(anyValue.Type, expectedType) {
				return NilValue{}
			}

			return SomeValue{Value: anyValue.Value}
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
	indexExpression := referenceExpression.Expression.(*ast.IndexExpression)
	return indexExpression.TargetExpression.Accept(interpreter).(Trampoline).
		FlatMap(func(result interface{}) Trampoline {
			storage := result.(StorageValue)

			indexingType := interpreter.Checker.Elaboration.IndexExpressionIndexingTypes[indexExpression]

			referenceValue := ReferenceValue{
				Storage:      storage,
				IndexingType: indexingType,
			}
			return Done{Result: referenceValue}
		})
}
