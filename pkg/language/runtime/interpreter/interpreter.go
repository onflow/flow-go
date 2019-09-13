package interpreter

import (
	"fmt"
	goRuntime "runtime"

	"github.com/raviqqe/hamt"

	"github.com/dapperlabs/flow-go/pkg/language/runtime/activations"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/common"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/errors"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/pkg/language/runtime/trampoline"
)

type loopBreak struct{}
type loopContinue struct{}
type functionReturn struct {
	Value
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
	Checker            *sema.Checker
	PredefinedValues   map[string]Value
	activations        *activations.Activations
	Globals            map[string]*Variable
	interfaces         map[string]*ast.InterfaceDeclaration
	ImportLocation     ast.ImportLocation
	CompositeFunctions map[string]map[string]FunctionValue
}

func NewInterpreter(checker *sema.Checker, predefinedValues map[string]Value) (*Interpreter, error) {
	interpreter := &Interpreter{
		Checker:            checker,
		PredefinedValues:   predefinedValues,
		activations:        &activations.Activations{},
		Globals:            map[string]*Variable{},
		interfaces:         map[string]*ast.InterfaceDeclaration{},
		CompositeFunctions: map[string]map[string]FunctionValue{},
	}

	for name, value := range predefinedValues {
		err := interpreter.ImportValue(name, value)
		if err != nil {
			return nil, err
		}
	}

	return interpreter, nil
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
	return More(func() Trampoline {
		interpreter.prepareInterpretation()

		return interpreter.visitProgramDeclarations()
	})
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

func (interpreter *Interpreter) visitProgramDeclarations() Trampoline {
	return interpreter.visitGlobalDeclarations(interpreter.Checker.Program.Declarations)
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
		return nil, &NotCallableError{
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

	functionType, ok := ty.(*sema.FunctionType)
	if !ok {
		return nil, &NotCallableError{
			Value: variableValue,
		}
	}

	parameterTypes := functionType.ParameterTypes
	parameterCount := len(parameterTypes)
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
		// TODO: value type is not known – only used for Any boxing right now, so reject for now
		if parameterTypes[i].Equal(&sema.AnyType{}) {
			return nil, &NotCallableError{
				Value: variableValue,
			}
		}
		boxedArguments[i] = interpreter.box(argument, nil, parameterTypes[i])
	}

	trampoline = function.invoke(boxedArguments, Location{})
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
	panic(&errors.UnreachableError{})
}

func (interpreter *Interpreter) VisitFunctionDeclaration(declaration *ast.FunctionDeclaration) ast.Repr {

	identifier := declaration.Identifier.Identifier

	functionType := interpreter.Checker.FunctionDeclarationFunctionTypes[declaration]

	variable := interpreter.findOrDeclareVariable(identifier)

	// lexical scope: variables in functions are bound to what is visible at declaration time
	lexicalScope := interpreter.activations.CurrentOrNew()

	// make the function itself available inside the function
	lexicalScope = lexicalScope.Insert(common.StringKey(identifier), variable)

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
		if returnValue != nil {
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
					if blockResult == nil {
						resultValue = VoidValue{}
					} else {
						resultValue = blockResult.(functionReturn).Value
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
					messageTrampoline = Done{Result: StringValue("")}
				} else {
					messageTrampoline = condition.Message.Accept(interpreter).(Trampoline)
				}

				return messageTrampoline.
					Then(func(result interface{}) {
						message := string(result.(StringValue))

						panic(&ConditionError{
							ConditionKind: condition.Kind,
							Message:       message,
							LocationRange: LocationRange{
								ImportLocation: interpreter.ImportLocation,
								StartPos:       condition.Test.StartPosition(),
								EndPos:         condition.Test.EndPosition(),
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

			valueType := interpreter.Checker.ReturnStatementValueTypes[statement]
			returnType := interpreter.Checker.ReturnStatementReturnTypes[statement]

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

			valueType := interpreter.Checker.VariableDeclarationValueTypes[declaration]
			targetType := interpreter.Checker.VariableDeclarationTargetTypes[declaration]

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

func (interpreter *Interpreter) VisitAssignment(assignment *ast.AssignmentStatement) ast.Repr {
	return assignment.Value.Accept(interpreter).(Trampoline).
		FlatMap(func(result interface{}) Trampoline {

			valueType := interpreter.Checker.AssignmentStatementValueTypes[assignment]
			targetType := interpreter.Checker.AssignmentStatementTargetTypes[assignment]

			valueCopy := interpreter.copyAndBox(result.(Value), valueType, targetType)

			return interpreter.visitAssignmentValue(assignment, valueCopy)
		})
}

func (interpreter *Interpreter) visitAssignmentValue(assignment *ast.AssignmentStatement, value Value) Trampoline {
	switch target := assignment.Target.(type) {
	case *ast.IdentifierExpression:
		interpreter.visitIdentifierExpressionAssignment(target, value)
		// NOTE: no result, so it does *not* act like a return-statement
		return Done{}

	case *ast.IndexExpression:
		return interpreter.visitIndexExpressionAssignment(target, value)

	case *ast.MemberExpression:
		return interpreter.visitMemberExpressionAssignment(target, value)
	}

	panic(&errors.UnreachableError{})
}

func (interpreter *Interpreter) visitIdentifierExpressionAssignment(target *ast.IdentifierExpression, value Value) {
	variable := interpreter.findVariable(target.Identifier.Identifier)
	variable.Value = value
}

func (interpreter *Interpreter) visitIndexExpressionAssignment(target *ast.IndexExpression, value Value) Trampoline {
	return target.Expression.Accept(interpreter).(Trampoline).
		FlatMap(func(result interface{}) Trampoline {
			indexedValue := result.(IndexableValue)

			return target.Index.Accept(interpreter).(Trampoline).
				FlatMap(func(result interface{}) Trampoline {
					indexingValue := result.(Value)
					indexedValue.Set(indexingValue, value)

					// NOTE: no result, so it does *not* act like a return-statement
					return Done{}
				})
		})
}

func (interpreter *Interpreter) visitMemberExpressionAssignment(target *ast.MemberExpression, value Value) Trampoline {
	return target.Expression.Accept(interpreter).(Trampoline).
		FlatMap(func(result interface{}) Trampoline {
			structure := result.(CompositeValue)

			structure.SetMember(interpreter, target.Identifier.Identifier, value)

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

							rightType := interpreter.Checker.BinaryExpressionRightTypes[expression]
							resultType := interpreter.Checker.BinaryExpressionResultTypes[expression]

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
		startPos:  expression.StartPosition(),
		endPos:    expression.EndPosition(),
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
				startPos:  expression.StartPos,
				endPos:    expression.EndPos,
			})
		})
}

func (interpreter *Interpreter) VisitExpressionStatement(statement *ast.ExpressionStatement) ast.Repr {
	return statement.Expression.Accept(interpreter).(Trampoline).
		Map(func(_ interface{}) interface{} {
			// NOTE: ignore result, so it does *not* act like a return-statement
			return nil
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
	value := StringValue(expression.Value)

	return Done{Result: value}
}

func (interpreter *Interpreter) VisitArrayExpression(expression *ast.ArrayExpression) ast.Repr {
	return interpreter.visitExpressions(expression.Values, nil)
}

func (interpreter *Interpreter) VisitDictionaryExpression(expression *ast.DictionaryExpression) ast.Repr {
	return interpreter.visitEntries(expression.Entries, DictionaryValue{})
}

func (interpreter *Interpreter) VisitMemberExpression(expression *ast.MemberExpression) ast.Repr {
	return expression.Expression.Accept(interpreter).(Trampoline).
		Map(func(result interface{}) interface{} {
			value := result.(ValueWithMembers)
			return value.GetMember(interpreter, expression.Identifier.Identifier)
		})
}

func (interpreter *Interpreter) VisitIndexExpression(expression *ast.IndexExpression) ast.Repr {
	return expression.Expression.Accept(interpreter).(Trampoline).
		FlatMap(func(result interface{}) Trampoline {
			indexedValue := result.(IndexableValue)

			return expression.Index.Accept(interpreter).(Trampoline).
				FlatMap(func(result interface{}) Trampoline {
					indexingValue := result.(Value)
					value := indexedValue.Get(indexingValue)
					return Done{Result: value}
				})
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

			return interpreter.visitExpressions(argumentExpressions, nil).
				FlatMap(func(result interface{}) Trampoline {
					arguments := result.(ArrayValue)

					argumentTypes := interpreter.Checker.InvocationExpressionArgumentTypes[invocationExpression]
					parameterTypes := interpreter.Checker.InvocationExpressionParameterTypes[invocationExpression]

					argumentCopies := make([]Value, len(*arguments.Values))
					for i, argument := range *arguments.Values {
						argumentType := argumentTypes[i]
						parameterType := parameterTypes[i]
						argumentCopies[i] = interpreter.copyAndBox(argument, argumentType, parameterType)
					}

					// TODO: optimize: only potentially used by host-functions
					location := Location{
						Position:       invocationExpression.StartPosition(),
						ImportLocation: interpreter.ImportLocation,
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
		function.Type.ReturnType,
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
	for parameterIndex, parameter := range function.Expression.Parameters {
		argument := arguments[parameterIndex]
		interpreter.declareVariable(parameter.Identifier.Identifier, argument)
	}
}

func (interpreter *Interpreter) visitExpressions(expressions []ast.Expression, values []Value) Trampoline {
	count := len(expressions)

	// no expressions? stop
	if count == 0 {
		return Done{Result: ArrayValue{Values: &values}}
	}

	// interpret the first expression
	return expressions[0].Accept(interpreter).(Trampoline).
		FlatMap(func(result interface{}) Trampoline {
			value := result.(Value)

			// interpret the remaining expressions
			return interpreter.visitExpressions(expressions[1:], append(values, value))
		})
}

func (interpreter *Interpreter) visitEntries(entries []ast.Entry, result DictionaryValue) Trampoline {
	count := len(entries)

	// no entries? stop
	if count == 0 {
		return Done{Result: result}
	}

	entry := entries[0]

	// interpret the key expression
	return entry.Key.Accept(interpreter).(Trampoline).
		FlatMap(func(keyResult interface{}) Trampoline {
			key := keyResult.(Value)

			// interpret the value expression
			return entry.Value.Accept(interpreter).(Trampoline).
				FlatMap(func(valueResult interface{}) Trampoline {
					value := valueResult.(Value)

					// NOTE: not setting using indexing,
					// as key might need special handling
					result.Set(key, value)

					// interpret the remaining entries
					return interpreter.visitEntries(entries[1:], result)
				})
		})
}

func (interpreter *Interpreter) VisitFunctionExpression(expression *ast.FunctionExpression) ast.Repr {

	// lexical scope: variables in functions are bound to what is visible at declaration time
	lexicalScope := interpreter.activations.CurrentOrNew()

	functionType := interpreter.Checker.FunctionExpressionFunctionType[expression]

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
		Insert(common.StringKey(identifier), variable)

	// TODO: support multiple overloaded initializers

	var initializerFunction *InterpretedFunctionValue
	if len(declaration.Initializers) > 0 {
		firstInitializer := declaration.Initializers[0]

		functionType := interpreter.Checker.InitializerFunctionTypes[firstInitializer]

		f := interpreter.initializerFunction(
			declaration,
			firstInitializer,
			functionType,
			lexicalScope,
		)
		initializerFunction = &f
	}

	functions := interpreter.compositeFunctions(declaration, lexicalScope)

	interpreter.CompositeFunctions[identifier] = functions

	variable.Value = NewHostFunctionValue(
		func(arguments []Value, location Location) Trampoline {

			value := CompositeValue{
				Identifier: identifier,
				Fields:     &map[string]Value{},
				Functions:  &functions,
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
	return NewHostFunctionValue(func(arguments []Value, location Location) Trampoline {
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
	initializer *ast.InitializerDeclaration,
	functionType *sema.FunctionType,
	lexicalScope hamt.Map,
) InterpretedFunctionValue {

	function := initializer.ToFunctionExpression()

	// copy function block, append interfaces' pre-conditions and post-condition
	functionBlockCopy := *function.FunctionBlock
	function.FunctionBlock = &functionBlockCopy

	for _, conformance := range compositeDeclaration.Conformances {
		interfaceDeclaration := interpreter.interfaces[conformance.Identifier.Identifier]

		// TODO: support multiple overloaded initializers

		if len(interfaceDeclaration.Initializers) == 0 {
			continue
		}

		firstInitializer := interfaceDeclaration.Initializers[0]
		if firstInitializer == nil || firstInitializer.FunctionBlock == nil {
			continue
		}

		functionBlockCopy.PreConditions = append(
			functionBlockCopy.PreConditions,
			firstInitializer.FunctionBlock.PreConditions...,
		)

		functionBlockCopy.PostConditions = append(
			functionBlockCopy.PostConditions,
			firstInitializer.FunctionBlock.PostConditions...,
		)
	}

	return newInterpretedFunction(
		interpreter,
		function,
		functionType,
		lexicalScope,
	)
}

func (interpreter *Interpreter) compositeFunctions(
	compositeDeclaration *ast.CompositeDeclaration,
	lexicalScope hamt.Map,
) map[string]FunctionValue {

	functions := map[string]FunctionValue{}

	for _, functionDeclaration := range compositeDeclaration.Functions {
		functionType := interpreter.Checker.FunctionDeclarationFunctionTypes[functionDeclaration]

		function := interpreter.compositeFunction(compositeDeclaration, functionDeclaration)

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
	compositeDeclaration *ast.CompositeDeclaration,
	functionDeclaration *ast.FunctionDeclaration,
) *ast.FunctionExpression {

	functionIdentifier := functionDeclaration.Identifier.Identifier

	function := functionDeclaration.ToExpression()

	// copy function block, append interfaces' pre-conditions and post-condition
	functionBlockCopy := *function.FunctionBlock
	function.FunctionBlock = &functionBlockCopy

	for _, conformance := range compositeDeclaration.Conformances {
		conformanceIdentifier := conformance.Identifier.Identifier
		interfaceDeclaration := interpreter.interfaces[conformanceIdentifier]
		interfaceFunction, ok := interfaceDeclaration.FunctionsByIdentifier()[functionIdentifier]
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
	panic(&errors.UnreachableError{})
}

func (interpreter *Interpreter) VisitInitializerDeclaration(initializer *ast.InitializerDeclaration) ast.Repr {
	panic(&errors.UnreachableError{})
}

func (interpreter *Interpreter) copyAndBox(value Value, valueType, targetType sema.Type) Value {
	result := value.Copy()
	return interpreter.box(result, valueType, targetType)
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

	interpreter.declareInterfaceMetaType(declaration)

	return Done{}
}

func (interpreter *Interpreter) declareInterface(declaration *ast.InterfaceDeclaration) {
	interpreter.interfaces[declaration.Identifier.Identifier] = declaration
}

func (interpreter *Interpreter) declareInterfaceMetaType(declaration *ast.InterfaceDeclaration) {

	interfaceType := interpreter.Checker.InterfaceDeclarationTypes[declaration]

	variable := interpreter.findOrDeclareVariable(declaration.Identifier.Identifier)
	variable.Value = MetaTypeValue{Type: interfaceType}
}

func (interpreter *Interpreter) VisitImportDeclaration(declaration *ast.ImportDeclaration) ast.Repr {
	importedChecker := interpreter.Checker.ImportCheckers[declaration.Location]

	subInterpreter, err := NewInterpreter(importedChecker, interpreter.PredefinedValues)
	if err != nil {
		panic(err)
	}

	subInterpreter.ImportLocation = declaration.Location

	return subInterpreter.interpret().
		Then(func(_ interface{}) {
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

				// if the imported name refers to a structure,
				// also take the structure functions from the sub-interpreter
				if structureFunctions, ok := subInterpreter.CompositeFunctions[name]; ok {
					interpreter.CompositeFunctions[name] = structureFunctions
				}
			}
		})
}

func (interpreter *Interpreter) VisitFailableDowncastExpression(expression *ast.FailableDowncastExpression) ast.Repr {
	return expression.Expression.Accept(interpreter).(Trampoline).
		Map(func(result interface{}) interface{} {
			value := result.(Value)

			anyValue := value.(AnyValue)
			expectedType := interpreter.Checker.FailableDowncastingTypes[expression]

			if !interpreter.Checker.IsSubType(anyValue.Type, expectedType) {
				return NilValue{}
			}

			return SomeValue{Value: anyValue.Value}
		})
}
