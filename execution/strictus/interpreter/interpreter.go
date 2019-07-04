package interpreter

import (
	"bamboo-runtime/execution/strictus/ast"
	"fmt"
)

// Visit-methods for statement which return a non-nil value
// are treated like they are returning a value.

type Interpreter struct {
	Program     ast.Program
	activations *Activations
	Globals     map[string]*Variable
}

func NewInterpreter(program *ast.Program) *Interpreter {
	return &Interpreter{
		// TODO: store pointer
		Program:     *program,
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

	for _, declaration := range interpreter.Program.Declarations {
		declaration.Accept(interpreter)
		name := declaration.DeclarationName()

		if _, exists := interpreter.Globals[name]; exists {
			return &RedeclarationError{
				Name: name,
				Pos:  declaration.GetIdentifierPosition(),
			}
		}

		interpreter.Globals[name] = interpreter.activations.Find(name)
	}

	return nil
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

	return interpreter.invokeFunction(function, arguments, nil, nil), nil
}

func (interpreter *Interpreter) invokeFunction(
	function FunctionValue,
	arguments []Value,
	startPosition *ast.Position,
	endPosition *ast.Position,
) Value {

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
	return nil
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

	return nil
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
	defer interpreter.activations.Pop()

	for _, statement := range block.Statements {
		result := statement.Accept(interpreter)
		if result != nil {
			return result
		}
	}

	return nil
}

func (interpreter *Interpreter) VisitReturnStatement(statement *ast.ReturnStatement) ast.Repr {
	// NOTE: returning result

	if statement.Expression == nil {
		return VoidValue{}
	}

	return statement.Expression.Accept(interpreter)
}

func (interpreter *Interpreter) VisitIfStatement(statement *ast.IfStatement) ast.Repr {
	if statement.Test.Accept(interpreter).(BoolValue) {
		return statement.Then.Accept(interpreter)
	} else if statement.Else != nil {
		return statement.Else.Accept(interpreter)
	}

	return nil
}

func (interpreter *Interpreter) VisitWhileStatement(statement *ast.WhileStatement) ast.Repr {
	for statement.Test.Accept(interpreter).(BoolValue) {
		result := statement.Block.Accept(interpreter)
		if result != nil {
			return result
		}
	}
	return nil
}

func (interpreter *Interpreter) VisitVariableDeclaration(declaration *ast.VariableDeclaration) ast.Repr {
	value := declaration.Value.Accept(interpreter).(Value)
	interpreter.declareVariable(declaration, value)
	return nil
}

func (interpreter *Interpreter) declareVariable(declaration *ast.VariableDeclaration, value Value) ast.Repr {
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

	return nil
}

func (interpreter *Interpreter) VisitAssignment(assignment *ast.AssignmentStatement) ast.Repr {
	value := assignment.Value.Accept(interpreter).(Value)

	switch target := assignment.Target.(type) {
	case *ast.IdentifierExpression:
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

	case *ast.IndexExpression:
		indexedValue := target.Expression.Accept(interpreter).(Value)
		array, ok := indexedValue.(ArrayValue)
		if !ok {
			panic(&NotIndexableError{
				Value:    indexedValue,
				StartPos: target.Expression.StartPosition(),
				EndPos:   target.Expression.EndPosition(),
			})
		}

		indexValue := target.Index.Accept(interpreter).(Value)
		index, ok := indexValue.(IntegerValue)
		if !ok {
			panic(&InvalidIndexValueError{
				Value:    indexValue,
				StartPos: target.Index.StartPosition(),
				EndPos:   target.Index.EndPosition(),
			})
		}
		array[index.IntValue()] = value

	case *ast.MemberExpression:
		// TODO:

	default:
		panic(&unsupportedAssignmentTargetExpression{
			target: target,
		})
	}
	return nil
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
	return variable.Value
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

func (interpreter *Interpreter) visitBinaryIntegerOperation(expr *ast.BinaryExpression) (left, right IntegerValue) {
	leftValue := expr.Left.Accept(interpreter).(Value)
	left = interpreter.visitBinaryIntegerOperand(
		leftValue,
		expr.Operation,
		OperandSideLeft,
		expr.Left.StartPosition(),
		expr.Left.EndPosition(),
	)

	rightValue := expr.Right.Accept(interpreter).(Value)
	right = interpreter.visitBinaryIntegerOperand(
		rightValue,
		expr.Operation,
		OperandSideRight,
		expr.Right.StartPosition(),
		expr.Right.EndPosition(),
	)

	return left, right
}

func (interpreter *Interpreter) visitBinaryBoolOperation(expr *ast.BinaryExpression) (left, right BoolValue) {
	leftValue := expr.Left.Accept(interpreter).(Value)
	left = interpreter.visitBinaryBoolOperand(
		leftValue,
		expr.Operation,
		OperandSideLeft,
		expr.Left.StartPosition(),
		expr.Left.EndPosition(),
	)

	rightValue := expr.Right.Accept(interpreter).(Value)
	right = interpreter.visitBinaryBoolOperand(
		rightValue,
		expr.Operation,
		OperandSideRight,
		expr.Right.StartPosition(),
		expr.Right.EndPosition(),
	)

	return left, right
}

func (interpreter *Interpreter) VisitBinaryExpression(expression *ast.BinaryExpression) ast.Repr {
	switch expression.Operation {
	case ast.OperationPlus:
		left, right := interpreter.visitBinaryIntegerOperation(expression)
		return left.Plus(right)

	case ast.OperationMinus:
		left, right := interpreter.visitBinaryIntegerOperation(expression)
		return left.Minus(right)

	case ast.OperationMod:
		left, right := interpreter.visitBinaryIntegerOperation(expression)
		return left.Mod(right)

	case ast.OperationMul:
		left, right := interpreter.visitBinaryIntegerOperation(expression)
		return left.Mul(right)

	case ast.OperationDiv:
		left, right := interpreter.visitBinaryIntegerOperation(expression)
		return left.Div(right)

	case ast.OperationLess:
		left, right := interpreter.visitBinaryIntegerOperation(expression)
		return left.Less(right)

	case ast.OperationLessEqual:
		left, right := interpreter.visitBinaryIntegerOperation(expression)
		return left.LessEqual(right)

	case ast.OperationGreater:
		left, right := interpreter.visitBinaryIntegerOperation(expression)
		return left.Greater(right)

	case ast.OperationGreaterEqual:
		left, right := interpreter.visitBinaryIntegerOperation(expression)
		return left.GreaterEqual(right)

	case ast.OperationEqual:
		leftValue := expression.Left.Accept(interpreter).(Value)
		rightValue := expression.Right.Accept(interpreter).(Value)

		switch leftValue.(type) {
		case IntegerValue:
			left := interpreter.visitBinaryIntegerOperand(
				leftValue,
				expression.Operation,
				OperandSideLeft,
				expression.Left.StartPosition(),
				expression.Left.EndPosition(),
			)
			right := interpreter.visitBinaryIntegerOperand(
				rightValue,
				expression.Operation,
				OperandSideRight,
				expression.Right.StartPosition(),
				expression.Right.EndPosition(),
			)
			return BoolValue(left.Equal(right))

		case BoolValue:
			left := interpreter.visitBinaryBoolOperand(
				leftValue,
				expression.Operation,
				OperandSideLeft,
				expression.Left.StartPosition(),
				expression.Left.EndPosition(),
			)
			right := interpreter.visitBinaryBoolOperand(
				rightValue,
				expression.Operation,
				OperandSideRight,
				expression.Right.StartPosition(),
				expression.Right.EndPosition(),
			)
			return BoolValue(left == right)
		}

	case ast.OperationUnequal:
		leftValue := expression.Left.Accept(interpreter).(Value)
		rightValue := expression.Right.Accept(interpreter).(Value)

		switch leftValue.(type) {
		case IntegerValue:
			left := interpreter.visitBinaryIntegerOperand(
				leftValue,
				expression.Operation,
				OperandSideLeft,
				expression.Left.StartPosition(),
				expression.Left.EndPosition(),
			)
			right := interpreter.visitBinaryIntegerOperand(
				rightValue,
				expression.Operation,
				OperandSideRight,
				expression.Right.StartPosition(),
				expression.Right.EndPosition(),
			)
			return BoolValue(!left.Equal(right))

		case BoolValue:
			left := interpreter.visitBinaryBoolOperand(
				leftValue,
				expression.Operation,
				OperandSideLeft,
				expression.Left.StartPosition(),
				expression.Left.EndPosition(),
			)
			right := interpreter.visitBinaryBoolOperand(
				rightValue,
				expression.Operation,
				OperandSideRight,
				expression.Right.StartPosition(),
				expression.Right.EndPosition(),
			)
			return BoolValue(left != right)
		}

	case ast.OperationOr:
		left, right := interpreter.visitBinaryBoolOperation(expression)
		return BoolValue(left || right)

	case ast.OperationAnd:
		left, right := interpreter.visitBinaryBoolOperation(expression)
		return BoolValue(left && right)
	}

	panic(&unsupportedOperation{
		kind:      OperationKindBinary,
		operation: expression.Operation,
	})

	return nil
}

func (interpreter *Interpreter) VisitUnaryExpression(expression *ast.UnaryExpression) ast.Repr {
	value := expression.Expression.Accept(interpreter).(Value)

	switch expression.Operation {
	case ast.OperationNegate:
		boolValue := interpreter.visitUnaryBoolOperand(
			value,
			expression.Operation,
			expression.StartPos,
			expression.EndPos,
		)
		return boolValue.Negate()

	case ast.OperationMinus:
		integerValue := interpreter.visitUnaryIntegerOperand(
			value,
			expression.Operation,
			expression.StartPos,
			expression.EndPos,
		)
		return integerValue.Negate()
	}

	panic(&unsupportedOperation{
		kind:      OperationKindUnary,
		operation: expression.Operation,
	})

	return nil
}

func (interpreter *Interpreter) VisitExpressionStatement(statement *ast.ExpressionStatement) ast.Repr {
	statement.Expression.Accept(interpreter)
	return nil
}

func (interpreter *Interpreter) VisitBoolExpression(expression *ast.BoolExpression) ast.Repr {
	return BoolValue(expression.Value)
}

func (interpreter *Interpreter) VisitIntExpression(expression *ast.IntExpression) ast.Repr {
	return IntValue{expression.Value}
}

func (interpreter *Interpreter) VisitArrayExpression(expression *ast.ArrayExpression) ast.Repr {
	var values []interface{}

	for _, value := range expression.Values {
		values = append(values, value.Accept(interpreter))
	}

	return ArrayValue(values)
}

func (interpreter *Interpreter) VisitMemberExpression(*ast.MemberExpression) ast.Repr {
	// TODO: no dictionaries yet
	return nil
}

func (interpreter *Interpreter) VisitIndexExpression(expression *ast.IndexExpression) ast.Repr {
	indexedValue := expression.Expression.Accept(interpreter).(Value)
	array, ok := indexedValue.(ArrayValue)
	if !ok {
		panic(&NotIndexableError{
			Value:    indexedValue,
			StartPos: expression.Expression.StartPosition(),
			EndPos:   expression.Expression.EndPosition(),
		})
	}

	indexValue := expression.Index.Accept(interpreter).(Value)
	index, ok := indexValue.(IntegerValue)
	if !ok {
		panic(&InvalidIndexValueError{
			Value:    indexValue,
			StartPos: expression.Index.StartPosition(),
			EndPos:   expression.Index.EndPosition(),
		})
	}
	return array[index.IntValue()]
}

func (interpreter *Interpreter) VisitConditionalExpression(expression *ast.ConditionalExpression) ast.Repr {
	if expression.Test.Accept(interpreter).(BoolValue) {
		return expression.Then.Accept(interpreter)
	} else {
		return expression.Else.Accept(interpreter)
	}
}

func (interpreter *Interpreter) VisitInvocationExpression(invocationExpression *ast.InvocationExpression) ast.Repr {

	// evaluate the invoked expression
	value := invocationExpression.Expression.Accept(interpreter).(Value)
	function, ok := value.(FunctionValue)
	if !ok {
		panic(&NotCallableError{
			Value:    value,
			StartPos: invocationExpression.Expression.StartPosition(),
			EndPos:   invocationExpression.Expression.EndPosition(),
		})
	}

	// NOTE: evaluate all argument expressions in call-site scope, not in function body
	arguments := interpreter.evaluateExpressions(invocationExpression.Arguments)

	return interpreter.invokeFunction(
		function,
		arguments,
		invocationExpression.StartPos,
		invocationExpression.EndPos,
	)
}

func (interpreter *Interpreter) invokeInterpretedFunction(function *InterpretedFunctionValue, arguments []Value) Value {
	// start a new activation record
	// lexical scope: use the function declaration's activation record,
	// not the current one (which would be dynamic scope)
	interpreter.activations.Push(function.Activation)
	defer interpreter.activations.Pop()

	interpreter.bindFunctionInvocationParameters(function, arguments)

	blockResult := function.Expression.Block.Accept(interpreter)
	if blockResult == nil {
		return VoidValue{}
	}
	return blockResult.(Value)
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

func (interpreter *Interpreter) evaluateExpressions(expressions []ast.Expression) []Value {
	var values []Value
	for _, expression := range expressions {
		argument := expression.Accept(interpreter).(Value)
		values = append(values, argument)
	}
	return values
}

func (interpreter *Interpreter) VisitFunctionExpression(expression *ast.FunctionExpression) ast.Repr {
	// lexical scope: variables in functions are bound to what is visible at declaration time
	return newInterpretedFunction(expression, interpreter.activations.CurrentOrNew())
}
