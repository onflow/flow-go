package interpreter

import (
	"github.com/dapperlabs/bamboo-node/language/runtime/ast"
	"github.com/dapperlabs/bamboo-node/language/runtime/errors"
	. "github.com/dapperlabs/bamboo-node/language/runtime/trampoline"
	"fmt"
)

// Visit-methods for statement which return a non-nil value
// are treated like they are returning a value.

type Interpreter struct {
	Program     *ast.Program
	activations *Activations
	Globals     map[string]*Variable
}

func NewInterpreter(program *ast.Program) *Interpreter {
	return &Interpreter{
		Program:     program,
		activations: &Activations{},
		Globals:     map[string]*Variable{},
	}
}

func (interpreter *Interpreter) Interpret() (err error) {
	// recover internal panics and return them as an error
	defer func() {
		if r := recover(); r != nil {
			var ok bool
			err, ok = r.(error)
			if !ok {
				err = fmt.Errorf("%v", r)
			}
		}
	}()

	Run(More(func() Trampoline {
		return interpreter.visitProgramDeclarations()
	}))

	return nil
}

func (interpreter *Interpreter) visitProgramDeclarations() Trampoline {
	return interpreter.visitDeclarations(interpreter.Program.Declarations)
}

func (interpreter *Interpreter) visitDeclarations(declarations []ast.Declaration) Trampoline {
	count := len(declarations)

	// no declarations? stop
	if count == 0 {
		// NOTE: no result, so it does *not* act like a return-statement
		return Done{}
	}

	// interpret the first declaration, then the remaining ones
	return interpreter.visitDeclaration(declarations[0]).
		FlatMap(func(_ interface{}) Trampoline {
			return interpreter.visitDeclarations(declarations[1:])
		})
}

// visitDeclaration firsts interprets the declaration,
// then finds the declaration and adds it to the globals
func (interpreter *Interpreter) visitDeclaration(declaration ast.Declaration) Trampoline {
	return declaration.Accept(interpreter).(Trampoline).
		Then(func(_ interface{}) {
			interpreter.defineGlobal(declaration)
		})
}

func (interpreter *Interpreter) defineGlobal(declaration ast.Declaration) {
	name := declaration.DeclarationName()
	if _, exists := interpreter.Globals[name]; exists {
		panic(&RedeclarationError{
			Name: name,
			Pos:  declaration.GetIdentifierPosition(),
		})
	}
	interpreter.Globals[name] = interpreter.activations.Find(name)
}

func (interpreter *Interpreter) Invoke(functionName string, inputs ...interface{}) (value Value, err error) {
	variable, ok := interpreter.Globals[functionName]
	if !ok {
		return nil, &NotDeclaredError{
			ExpectedKind: DeclarationKindFunction,
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

	arguments, err := ToValues(inputs)
	if err != nil {
		return nil, err
	}

	// recover internal panics and return them as an error
	defer func() {
		if r := recover(); r != nil {
			var ok bool
			err, ok = r.(error)
			if !ok {
				err = fmt.Errorf("%v", r)
			}
		}
	}()

	result := Run(interpreter.invokeFunction(function, arguments, nil, nil))
	if result == nil {
		return nil, nil
	}
	return result.(Value), nil
}

func (interpreter *Interpreter) invokeFunction(
	function FunctionValue,
	arguments []Value,
	startPosition *ast.Position,
	endPosition *ast.Position,
) Trampoline {

	// ensures the invocation's argument count matches the function's parameter count

	parameterCount := function.parameterCount()
	argumentCount := len(arguments)

	if argumentCount != parameterCount {
		panic(&ArgumentCountError{
			ParameterCount: parameterCount,
			ArgumentCount:  argumentCount,
			StartPos:       startPosition,
			EndPos:         endPosition,
		})
	}

	return function.invoke(interpreter, arguments)
}

func (interpreter *Interpreter) VisitProgram(program *ast.Program) ast.Repr {
	panic(errors.UnreachableError{})
}

func (interpreter *Interpreter) VisitFunctionDeclaration(declaration *ast.FunctionDeclaration) ast.Repr {
	expression := &ast.FunctionExpression{
		Parameters: declaration.Parameters,
		ReturnType: declaration.ReturnType,
		Block:      declaration.Block,
		StartPos:   declaration.StartPos,
		EndPos:     declaration.EndPos,
	}

	// lexical scope: variables in functions are bound to what is visible at declaration time
	function := newInterpretedFunction(expression, interpreter.activations.CurrentOrNew())

	var parameterTypes []ast.Type
	for _, parameter := range declaration.Parameters {
		parameterTypes = append(parameterTypes, parameter.Type)
	}

	functionType := &ast.FunctionType{
		ParameterTypes: parameterTypes,
		ReturnType:     declaration.ReturnType,
	}
	variableDeclaration := &ast.VariableDeclaration{
		Value:         expression,
		Identifier:    declaration.Identifier,
		IsConst:       true,
		Type:          functionType,
		StartPos:      declaration.StartPos,
		EndPos:        declaration.EndPos,
		IdentifierPos: declaration.IdentifierPos,
	}

	// make the function itself available inside the function
	depth := interpreter.activations.Depth()
	variable := newVariable(variableDeclaration, depth, function)
	function.Activation = function.Activation.
		Insert(ActivationKey(declaration.Identifier), variable)

	// function declarations are de-sugared to constants
	interpreter.declareVariable(variableDeclaration, function)

	// NOTE: no result, so it does *not* act like a return-statement
	return Done{}
}

func (interpreter *Interpreter) ImportFunction(name string, function *HostFunctionValue) {
	variableDeclaration := &ast.VariableDeclaration{
		Identifier: name,
		IsConst:    true,
		// TODO: Type
	}

	interpreter.declareVariable(variableDeclaration, function)
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

	// interpret the first statement, then the remaining ones
	return interpreter.visitStatement(statements[0]).
		FlatMap(func(returnValue interface{}) Trampoline {
			if returnValue != nil {
				return Done{Result: returnValue}
			}
			return interpreter.visitStatements(statements[1:])
		})
}

func (interpreter *Interpreter) visitStatement(statement ast.Statement) Trampoline {
	// the enclosing block pushed an activation, see VisitBlock.
	// ensure it is popped properly even when a panic occurs
	defer func() {
		if e := recover(); e != nil {
			interpreter.activations.Pop()
			panic(e)
		}
	}()

	return statement.Accept(interpreter).(Trampoline)
}

func (interpreter *Interpreter) VisitReturnStatement(statement *ast.ReturnStatement) ast.Repr {
	// NOTE: returning result

	if statement.Expression == nil {
		return Done{Result: VoidValue{}}
	}

	return statement.Expression.Accept(interpreter).(Trampoline)
}

func (interpreter *Interpreter) VisitIfStatement(statement *ast.IfStatement) ast.Repr {
	return statement.Test.Accept(interpreter).(Trampoline).
		FlatMap(func(result interface{}) Trampoline {
			value := result.(BoolValue)
			if value {
				return statement.Then.Accept(interpreter).(Trampoline)
			} else if statement.Else != nil {
				return statement.Else.Accept(interpreter).(Trampoline)
			}

			// NOTE: no result, so it does *not* act like a return-statement
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
				FlatMap(func(returnValue interface{}) Trampoline {
					if returnValue != nil {
						return Done{Result: returnValue}
					}

					// recurse
					return interpreter.VisitWhileStatement(statement).(Trampoline)
				})
		})
}

// VisitVariableDeclaration first visits the declaration's value,
// then declares the variable with the name bound to the value
func (interpreter *Interpreter) VisitVariableDeclaration(declaration *ast.VariableDeclaration) ast.Repr {
	return declaration.Value.Accept(interpreter).(Trampoline).
		FlatMap(func(result interface{}) Trampoline {
			value := result.(Value)

			interpreter.declareVariable(declaration, value)

			// NOTE: ignore result, so it does *not* act like a return-statement
			return Done{}
		})
}

func (interpreter *Interpreter) declareVariable(declaration *ast.VariableDeclaration, value Value) {
	variable := interpreter.activations.Find(declaration.Identifier)
	depth := interpreter.activations.Depth()
	if variable != nil && variable.Depth == depth {
		panic(&RedeclarationError{
			Name: declaration.Identifier,
			Pos:  declaration.GetIdentifierPosition(),
		})
	}

	variable = newVariable(declaration, depth, value)

	interpreter.activations.Set(declaration.Identifier, variable)
}

func (interpreter *Interpreter) VisitAssignment(assignment *ast.AssignmentStatement) ast.Repr {
	return assignment.Value.Accept(interpreter).(Trampoline).
		FlatMap(func(result interface{}) Trampoline {
			value := result.(Value)
			return interpreter.visitAssignmentValue(assignment, value)
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
	// TODO:

	default:
		panic(&unsupportedAssignmentTargetExpression{
			target: target,
		})
	}

	panic(errors.UnreachableError{})
}

func (interpreter *Interpreter) visitIndexExpressionAssignment(target *ast.IndexExpression, value Value) Trampoline {
	return target.Expression.Accept(interpreter).(Trampoline).
		FlatMap(func(result interface{}) Trampoline {
			indexedValue := result.(Value)

			array, ok := indexedValue.(ArrayValue)
			if !ok {
				panic(&NotIndexableError{
					Value:    indexedValue,
					StartPos: target.Expression.StartPosition(),
					EndPos:   target.Expression.EndPosition(),
				})
			}

			return target.Index.Accept(interpreter).(Trampoline).
				FlatMap(func(result interface{}) Trampoline {
					indexValue := result.(Value)
					index, ok := indexValue.(IntegerValue)
					if !ok {
						panic(&InvalidIndexValueError{
							Value:    indexValue,
							StartPos: target.Index.StartPosition(),
							EndPos:   target.Index.EndPosition(),
						})
					}
					array[index.IntValue()] = value

					// NOTE: no result, so it does *not* act like a return-statement
					return Done{}
				})
		})
}

func (interpreter *Interpreter) visitIdentifierExpressionAssignment(target *ast.IdentifierExpression, value Value) {
	identifier := target.Identifier
	variable := interpreter.activations.Find(identifier)
	if variable == nil {
		panic(&NotDeclaredError{
			ExpectedKind: DeclarationKindVariable,
			Name:         identifier,
			StartPos:     target.StartPosition(),
			EndPos:       target.EndPosition(),
		})
	}
	if !variable.Set(value) {
		panic(&AssignmentToConstantError{
			Name:     identifier,
			StartPos: target.StartPosition(),
			EndPos:   target.EndPosition(),
		})
	}
	interpreter.activations.Set(identifier, variable)
}

func (interpreter *Interpreter) VisitIdentifierExpression(expression *ast.IdentifierExpression) ast.Repr {
	variable := interpreter.activations.Find(expression.Identifier)
	if variable == nil {
		panic(&NotDeclaredError{
			ExpectedKind: DeclarationKindValue,
			Name:         expression.Identifier,
			StartPos:     expression.StartPosition(),
			EndPos:       expression.EndPosition(),
		})
	}
	return Done{Result: variable.Value}
}

func (interpreter *Interpreter) visitBinaryIntegerOperand(
	value Value,
	operation ast.Operation,
	side OperandSide,
	startPos *ast.Position,
	endPos *ast.Position,
) IntegerValue {
	integerValue, isInteger := value.(IntegerValue)
	if !isInteger {
		panic(&InvalidBinaryOperandError{
			Operation:    operation,
			Side:         side,
			ExpectedType: &IntegerType{},
			Value:        value,
			StartPos:     startPos,
			EndPos:       endPos,
		})
	}
	return integerValue
}

func (interpreter *Interpreter) visitBinaryBoolOperand(
	value Value,
	operation ast.Operation,
	side OperandSide,
	startPos *ast.Position,
	endPos *ast.Position,
) BoolValue {
	boolValue, isBool := value.(BoolValue)
	if !isBool {
		panic(&InvalidBinaryOperandError{
			Operation:    operation,
			Side:         side,
			ExpectedType: &BoolType{},
			Value:        value,
			StartPos:     startPos,
			EndPos:       endPos,
		})
	}
	return boolValue
}

func (interpreter *Interpreter) visitUnaryBoolOperand(
	value Value,
	operation ast.Operation,
	startPos *ast.Position,
	endPos *ast.Position,
) BoolValue {
	boolValue, isBool := value.(BoolValue)
	if !isBool {
		panic(&InvalidUnaryOperandError{
			Operation:    operation,
			ExpectedType: &BoolType{},
			Value:        value,
			StartPos:     startPos,
			EndPos:       endPos,
		})
	}
	return boolValue
}

func (interpreter *Interpreter) visitUnaryIntegerOperand(
	value Value,
	operation ast.Operation,
	startPos *ast.Position,
	endPos *ast.Position,
) IntegerValue {
	integerValue, isInteger := value.(IntegerValue)
	if !isInteger {
		panic(&InvalidUnaryOperandError{
			Operation:    operation,
			ExpectedType: &IntegerType{},
			Value:        value,
			StartPos:     startPos,
			EndPos:       endPos,
		})
	}
	return integerValue
}

// visitBinaryOperation interprets the left-hand side and the right-hand side and returns
// the result in a integerTuple or booleanTuple
func (interpreter *Interpreter) visitBinaryOperation(expr *ast.BinaryExpression) Trampoline {
	// interpret the left-hand side
	return expr.Left.Accept(interpreter).(Trampoline).
		FlatMap(func(left interface{}) Trampoline {
			// after interpreting the left-hand side,
			// interpret the right-hand side
			return expr.Right.Accept(interpreter).(Trampoline).
				FlatMap(func(right interface{}) Trampoline {

					leftValue := left.(Value)
					rightValue := right.(Value)

					switch leftValue.(type) {
					case IntegerValue:
						left := interpreter.visitBinaryIntegerOperand(
							leftValue,
							expr.Operation,
							OperandSideLeft,
							expr.Left.StartPosition(),
							expr.Left.EndPosition(),
						)
						right := interpreter.visitBinaryIntegerOperand(
							rightValue,
							expr.Operation,
							OperandSideRight,
							expr.Right.StartPosition(),
							expr.Right.EndPosition(),
						)
						return Done{Result: integerTuple{left, right}}

					case BoolValue:
						left := interpreter.visitBinaryBoolOperand(
							leftValue,
							expr.Operation,
							OperandSideLeft,
							expr.Left.StartPosition(),
							expr.Left.EndPosition(),
						)
						right := interpreter.visitBinaryBoolOperand(
							rightValue,
							expr.Operation,
							OperandSideRight,
							expr.Right.StartPosition(),
							expr.Right.EndPosition(),
						)

						return Done{Result: boolTuple{left, right}}
					}

					panic(&errors.UnreachableError{})
				})
		})
}

// visitBinaryIntegerOperation interprets the left-hand side and right-hand side
// of the binary expression, and applies both integer values to the given binary function
func (interpreter *Interpreter) visitBinaryIntegerOperation(
	expression *ast.BinaryExpression,
	binaryFunction func(IntegerValue, IntegerValue) Value,
) Trampoline {
	return interpreter.visitBinaryOperation(expression).
		Map(func(result interface{}) interface{} {
			tuple, ok := result.(integerTuple)
			if !ok {
				leftValue, rightValue := result.(valueTuple).values()
				panic(&InvalidBinaryOperandTypesError{
					Operation:    expression.Operation,
					ExpectedType: &IntegerType{},
					LeftValue:    leftValue,
					RightValue:   rightValue,
					StartPos:     expression.StartPosition(),
					EndPos:       expression.EndPosition(),
				})
			}

			left, right := tuple.destructure()
			return binaryFunction(left, right)
		})
}

// visitBinaryBoolOperation interprets the left-hand side and right-hand side
// of the binary expression, and applies both boolean values to the given binary function
func (interpreter *Interpreter) visitBinaryBoolOperation(
	expression *ast.BinaryExpression,
	binaryFunction func(BoolValue, BoolValue) Value,
) Trampoline {
	return interpreter.visitBinaryOperation(expression).
		Map(func(result interface{}) interface{} {
			tuple, ok := result.(boolTuple)
			if !ok {
				leftValue, rightValue := result.(valueTuple).values()
				panic(&InvalidBinaryOperandTypesError{
					Operation:    expression.Operation,
					ExpectedType: &BoolType{},
					LeftValue:    leftValue,
					RightValue:   rightValue,
					StartPos:     expression.StartPosition(),
					EndPos:       expression.EndPosition(),
				})
			}

			left, right := tuple.destructure()
			return binaryFunction(left, right)
		})
}

func (interpreter *Interpreter) VisitBinaryExpression(expression *ast.BinaryExpression) ast.Repr {
	switch expression.Operation {
	case ast.OperationPlus:
		return interpreter.visitBinaryIntegerOperation(
			expression,
			func(left IntegerValue, right IntegerValue) Value {
				return left.Plus(right)
			})

	case ast.OperationMinus:
		return interpreter.visitBinaryIntegerOperation(
			expression,
			func(left IntegerValue, right IntegerValue) Value {
				return left.Minus(right)
			})

	case ast.OperationMod:
		return interpreter.visitBinaryIntegerOperation(
			expression,
			func(left IntegerValue, right IntegerValue) Value {
				return left.Mod(right)
			})

	case ast.OperationMul:
		return interpreter.visitBinaryIntegerOperation(
			expression,
			func(left IntegerValue, right IntegerValue) Value {
				return left.Mul(right)
			})

	case ast.OperationDiv:
		return interpreter.visitBinaryIntegerOperation(
			expression,
			func(left IntegerValue, right IntegerValue) Value {
				return left.Div(right)
			})

	case ast.OperationLess:
		return interpreter.visitBinaryIntegerOperation(
			expression,
			func(left IntegerValue, right IntegerValue) Value {
				return left.Less(right)
			})

	case ast.OperationLessEqual:
		return interpreter.visitBinaryIntegerOperation(
			expression,
			func(left IntegerValue, right IntegerValue) Value {
				return left.LessEqual(right)
			})

	case ast.OperationGreater:
		return interpreter.visitBinaryIntegerOperation(
			expression,
			func(left IntegerValue, right IntegerValue) Value {
				return left.Greater(right)
			})

	case ast.OperationGreaterEqual:
		return interpreter.visitBinaryIntegerOperation(
			expression,
			func(left IntegerValue, right IntegerValue) Value {
				return left.GreaterEqual(right)
			})

	case ast.OperationEqual:
		return interpreter.visitBinaryOperation(expression).
			Map(func(result interface{}) interface{} {
				switch tuple := result.(type) {
				case integerTuple:
					left, right := tuple.destructure()
					return BoolValue(left.Equal(right))

				case boolTuple:
					left, right := tuple.destructure()
					return BoolValue(left == right)
				}

				panic(&errors.UnreachableError{})
			})

	case ast.OperationUnequal:
		return interpreter.visitBinaryOperation(expression).
			Map(func(tuple interface{}) interface{} {
				switch typedTuple := tuple.(type) {
				case integerTuple:
					left, right := typedTuple.destructure()
					return BoolValue(!left.Equal(right))

				case boolTuple:
					left, right := typedTuple.destructure()
					return BoolValue(left != right)
				}

				panic(&errors.UnreachableError{})
			})

	case ast.OperationOr:
		return interpreter.visitBinaryBoolOperation(
			expression,
			func(left BoolValue, right BoolValue) Value {
				return BoolValue(left || right)
			})

	case ast.OperationAnd:
		return interpreter.visitBinaryBoolOperation(
			expression,
			func(left BoolValue, right BoolValue) Value {
				return BoolValue(left && right)
			})
	}

	panic(&unsupportedOperation{
		kind:      OperationKindBinary,
		operation: expression.Operation,
	})
}

func (interpreter *Interpreter) VisitUnaryExpression(expression *ast.UnaryExpression) ast.Repr {
	return expression.Expression.Accept(interpreter).(Trampoline).
		Map(func(result interface{}) interface{} {
			value := result.(Value)

			switch expression.Operation {
			case ast.OperationNegate:
				boolValue := interpreter.visitUnaryBoolOperand(
					value,
					expression.Operation,
					expression.StartPosition(),
					expression.EndPosition(),
				)
				return boolValue.Negate()

			case ast.OperationMinus:
				integerValue := interpreter.visitUnaryIntegerOperand(
					value,
					expression.Operation,
					expression.StartPosition(),
					expression.EndPosition(),
				)
				return integerValue.Negate()
			}

			panic(&unsupportedOperation{
				kind:      OperationKindUnary,
				operation: expression.Operation,
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

func (interpreter *Interpreter) VisitIntExpression(expression *ast.IntExpression) ast.Repr {
	value := IntValue{expression.Value}

	return Done{Result: value}
}

func (interpreter *Interpreter) VisitArrayExpression(expression *ast.ArrayExpression) ast.Repr {
	return interpreter.visitExpressions(expression.Values, nil)
}

func (interpreter *Interpreter) VisitMemberExpression(*ast.MemberExpression) ast.Repr {
	// TODO: no dictionaries yet
	panic(&errors.UnreachableError{})
}

func (interpreter *Interpreter) VisitIndexExpression(expression *ast.IndexExpression) ast.Repr {
	return expression.Expression.Accept(interpreter).(Trampoline).
		FlatMap(func(result interface{}) Trampoline {
			indexedValue := result.(Value)
			array, ok := indexedValue.(ArrayValue)
			if !ok {
				panic(&NotIndexableError{
					Value:    indexedValue,
					StartPos: expression.Expression.StartPosition(),
					EndPos:   expression.Expression.EndPosition(),
				})
			}

			return expression.Index.Accept(interpreter).(Trampoline).
				FlatMap(func(result interface{}) Trampoline {
					indexValue := result.(Value)
					index, ok := indexValue.(IntegerValue)
					if !ok {
						panic(&InvalidIndexValueError{
							Value:    indexValue,
							StartPos: expression.Index.StartPosition(),
							EndPos:   expression.Index.EndPosition(),
						})
					}

					value := array[index.IntValue()]

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
	return invocationExpression.Expression.Accept(interpreter).(Trampoline).
		FlatMap(func(result interface{}) Trampoline {
			value := result.(Value)
			function, ok := value.(FunctionValue)
			if !ok {
				panic(&NotCallableError{
					Value:    value,
					StartPos: invocationExpression.Expression.StartPosition(),
					EndPos:   invocationExpression.Expression.EndPosition(),
				})
			}

			// NOTE: evaluate all argument expressions in call-site scope, not in function body
			return interpreter.visitExpressions(invocationExpression.Arguments, nil).
				FlatMap(func(result interface{}) Trampoline {

					arguments := result.(ArrayValue)

					return interpreter.invokeFunction(
						function,
						arguments,
						invocationExpression.StartPosition(),
						invocationExpression.EndPosition(),
					)
				})
		})
}

func (interpreter *Interpreter) invokeInterpretedFunction(
	function *InterpretedFunctionValue,
	arguments []Value,
) Trampoline {

	// start a new activation record
	// lexical scope: use the function declaration's activation record,
	// not the current one (which would be dynamic scope)
	interpreter.activations.Push(function.Activation)
	defer interpreter.activations.Pop()

	interpreter.bindFunctionInvocationParameters(function, arguments)

	return function.Expression.Block.Accept(interpreter).(Trampoline).
		Map(func(blockResult interface{}) interface{} {
			if blockResult == nil {
				return VoidValue{}
			}
			return blockResult.(Value)
		})
}

// bindFunctionInvocationParameters binds the argument values to the parameters in the function
func (interpreter *Interpreter) bindFunctionInvocationParameters(
	function *InterpretedFunctionValue,
	arguments []Value,
) {
	for parameterIndex, parameter := range function.Expression.Parameters {
		argument := arguments[parameterIndex]

		interpreter.activations.Set(
			parameter.Identifier,
			&Variable{
				Declaration: &ast.VariableDeclaration{
					IsConst:    true,
					Identifier: parameter.Identifier,
					Type:       parameter.Type,
					StartPos:   parameter.StartPos,
					EndPos:     parameter.EndPos,
				},
				Value: argument,
			},
		)
	}
}

func (interpreter *Interpreter) visitExpressions(expressions []ast.Expression, values []Value) Trampoline {
	count := len(expressions)

	// no expressions? stop
	if count == 0 {
		return Done{Result: ArrayValue(values)}
	}

	// interpret the first expression
	return expressions[0].Accept(interpreter).(Trampoline).
		FlatMap(func(result interface{}) Trampoline {
			value := result.(Value)

			// interpret the remaining expressions
			return interpreter.visitExpressions(expressions[1:], append(values, value))
		})
}

func (interpreter *Interpreter) VisitFunctionExpression(expression *ast.FunctionExpression) ast.Repr {
	// lexical scope: variables in functions are bound to what is visible at declaration time
	function := newInterpretedFunction(expression, interpreter.activations.CurrentOrNew())

	return Done{Result: function}
}
