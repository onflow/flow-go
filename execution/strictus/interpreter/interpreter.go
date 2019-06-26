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

func NewInterpreter(program ast.Program) *Interpreter {
	return &Interpreter{
		Program:     program,
		activations: &Activations{},
		Globals:     map[string]*Variable{},
	}
}

func (interpreter *Interpreter) Interpret() {
	for _, declaration := range interpreter.Program.AllDeclarations {
		declaration.Accept(interpreter)
		name := declaration.DeclarationName()
		interpreter.Globals[name] = interpreter.activations.Find(name)
	}
}

func (interpreter *Interpreter) Invoke(functionName string, inputs ...interface{}) ast.Repr {
	variable, ok := interpreter.Globals[functionName]
	if !ok {
		panic(fmt.Sprintf("undefined function: %s", functionName))
	}

	function, ok := variable.Value.(*FunctionValue)
	if !ok {
		panic(fmt.Sprintf("declaration is not a function: %#+v", variable.Value))
	}

	arguments := ToValues(inputs)

	return interpreter.invokeFunction(function, arguments)
}

func (interpreter *Interpreter) VisitProgram(program ast.Program) ast.Repr {
	return nil
}

func (interpreter *Interpreter) VisitFunctionDeclaration(declaration ast.FunctionDeclaration) ast.Repr {
	expression := ast.FunctionExpression{
		Parameters:    declaration.Parameters,
		ReturnType:    declaration.ReturnType,
		Block:         declaration.Block,
		StartPosition: declaration.StartPosition,
		EndPosition:   declaration.EndPosition,
	}

	// lexical scope: variables in functions are bound to what is visible at declaration time
	function := newFunction(expression, interpreter.activations.CurrentOrNew())

	var parameterTypes []ast.Type
	for _, parameter := range declaration.Parameters {
		parameterTypes = append(parameterTypes, parameter.Type)
	}

	variableDeclaration := ast.VariableDeclaration{
		Value:      expression,
		Identifier: declaration.Identifier,
		IsConst:    true,
		Type: ast.FunctionType{
			ParameterTypes: parameterTypes,
			ReturnType:     declaration.ReturnType,
		},
	}

	// make the function itself available inside the function
	depth := interpreter.activations.Depth()
	variable := newVariable(variableDeclaration, depth, function)
	function.Activation = function.Activation.
		Insert(ActivationKey(declaration.Identifier), variable)

	// function declarations are de-sugared to constant variables
	interpreter.declareVariable(variableDeclaration, function)

	return nil
}

func (interpreter *Interpreter) VisitBlock(block ast.Block) ast.Repr {
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

func (interpreter *Interpreter) VisitReturnStatement(statement ast.ReturnStatement) ast.Repr {
	// NOTE: returning result

	if statement.Expression == nil {
		return VoidValue{}
	}

	return statement.Expression.Accept(interpreter)
}

func (interpreter *Interpreter) VisitIfStatement(statement ast.IfStatement) ast.Repr {
	if statement.Test.Accept(interpreter).(BoolValue) {
		return statement.Then.Accept(interpreter)
	} else {
		return statement.Else.Accept(interpreter)
	}
}

func (interpreter *Interpreter) VisitWhileStatement(statement ast.WhileStatement) ast.Repr {
	for statement.Test.Accept(interpreter).(BoolValue) {
		result := statement.Block.Accept(interpreter)
		if result != nil {
			return result
		}
	}
	return nil
}

func (interpreter *Interpreter) VisitVariableDeclaration(declaration ast.VariableDeclaration) ast.Repr {
	value := declaration.Value.Accept(interpreter).(Value)
	interpreter.declareVariable(declaration, value)
	return nil
}

func (interpreter *Interpreter) declareVariable(declaration ast.VariableDeclaration, value Value) ast.Repr {
	variable := interpreter.activations.Find(declaration.Identifier)
	depth := interpreter.activations.Depth()
	if variable != nil && variable.Depth == depth {
		panic(fmt.Sprintf("invalid redefinition of identifier: %s", declaration.Identifier))
	}

	variable = newVariable(declaration, depth, value)

	interpreter.activations.Set(declaration.Identifier, variable)

	return nil
}

func (interpreter *Interpreter) VisitAssignment(assignment ast.Assignment) ast.Repr {
	value := assignment.Value.Accept(interpreter).(Value)

	switch target := assignment.Target.(type) {
	case ast.IdentifierExpression:
		identifier := target.Identifier
		variable := interpreter.activations.Find(identifier)
		if variable == nil {
			panic(fmt.Sprintf("reference to unbound identifier: %s", identifier))
		}

		variable.Set(value)
		interpreter.activations.Set(identifier, variable)

	case ast.IndexExpression:
		indexedValue := target.Expression.Accept(interpreter)
		array, ok := indexedValue.(ArrayValue)
		if !ok {
			panic(fmt.Sprintf("can't index into non-array value: %#+v", indexedValue))
		}

		indexValue := target.Index.Accept(interpreter)
		index, ok := indexValue.(IntegerValue)
		if !ok {
			panic(fmt.Sprintf("can't index with value: %#+v", indexValue))
		}
		array[index.IntValue()] = value

	case ast.MemberExpression:
		// TODO:

	default:
		panic(fmt.Sprintf("assignment to unknown target expression: %#+v", target))
	}
	return nil
}

func (interpreter *Interpreter) VisitIdentifierExpression(expression ast.IdentifierExpression) ast.Repr {
	variable := interpreter.activations.Find(expression.Identifier)
	if variable == nil {
		panic(fmt.Sprintf("reference to unbound identifier: %s", expression.Identifier))
	}
	return variable.Value
}

func (interpreter *Interpreter) VisitBinaryExpression(expression ast.BinaryExpression) ast.Repr {
	left := expression.Left.Accept(interpreter)
	right := expression.Right.Accept(interpreter)

	leftInt, leftIsInt := left.(IntegerValue)
	rightInt, rightIsInt := right.(IntegerValue)
	if leftIsInt && rightIsInt {
		switch expression.Operation {
		case ast.OperationPlus:
			return leftInt.Plus(rightInt)
		case ast.OperationMinus:
			return leftInt.Minus(rightInt)
		case ast.OperationMod:
			return leftInt.Mod(rightInt)
		case ast.OperationMul:
			return leftInt.Mul(rightInt)
		case ast.OperationDiv:
			return leftInt.Div(rightInt)
		case ast.OperationLess:
			return leftInt.Less(rightInt)
		case ast.OperationLessEqual:
			return leftInt.LessEqual(rightInt)
		case ast.OperationGreater:
			return leftInt.Greater(rightInt)
		case ast.OperationGreaterEqual:
			return leftInt.GreaterEqual(rightInt)
		case ast.OperationEqual:
			return BoolValue(leftInt.Equal(rightInt))
		case ast.OperationUnequal:
			return BoolValue(!leftInt.Equal(rightInt))
		default:
			panic(fmt.Sprintf(
				"unsupported operation in integer binary expression: %s",
				expression.Operation.String(),
			))
		}
	}

	leftBool, leftIsBool := left.(BoolValue)
	rightBool, rightIsBool := right.(BoolValue)
	if leftIsBool && rightIsBool {
		switch expression.Operation {
		case ast.OperationEqual:
			return BoolValue(leftBool == rightBool)
		case ast.OperationUnequal:
			return BoolValue(leftBool != rightBool)
		case ast.OperationOr:
			return BoolValue(leftBool || rightBool)
		case ast.OperationAnd:
			return BoolValue(leftBool && rightBool)
		default:
			panic(fmt.Sprintf(
				"unsupported operation in boolean binary expression: %s",
				expression.Operation.String(),
			))
		}
	}

	panic(fmt.Sprintf(
		"invalid operands for binary expression: %s: %v, %v",
		expression.Operation.String(),
		left,
		right,
	))

	return nil
}

func (interpreter *Interpreter) VisitUnaryExpression(expression ast.UnaryExpression) ast.Repr {
	value := expression.Expression.Accept(interpreter)

	switch expression.Operation {
	case ast.OperationNegate:
		boolValue, ok := value.(BoolValue)
		if !ok {
			panic(fmt.Sprintf(
				"non-boolean value for unary negate: %s: %v",
				expression.Operation.String(),
				value,
			))
		}

		return boolValue.Negate()

	case ast.OperationMinus:
		intValue, ok := value.(IntegerValue)
		if !ok {
			panic(fmt.Sprintf(
				"non-integer value for unary minus: %s: %v",
				expression.Operation.String(),
				value,
			))
		}
		return intValue.Negate()

	default:
		panic(fmt.Sprintf(
			"unsupported operation in unary expression: %s",
			expression.Operation.String(),
		))
	}

	return nil
}

func (interpreter *Interpreter) VisitExpressionStatement(statement ast.ExpressionStatement) ast.Repr {
	statement.Expression.Accept(interpreter)
	return nil
}

func (interpreter *Interpreter) VisitBoolExpression(expression ast.BoolExpression) ast.Repr {
	return BoolValue(expression.Value)
}

func (interpreter *Interpreter) VisitIntExpression(expression ast.IntExpression) ast.Repr {
	return IntValue{expression.Value}
}

func (interpreter *Interpreter) VisitArrayExpression(expression ast.ArrayExpression) ast.Repr {
	var values []interface{}

	for _, value := range expression.Values {
		values = append(values, value.Accept(interpreter))
	}

	return ArrayValue(values)
}

func (interpreter *Interpreter) VisitMemberExpression(ast.MemberExpression) ast.Repr {
	// TODO: no dictionaries yet
	return nil
}

func (interpreter *Interpreter) VisitIndexExpression(expression ast.IndexExpression) ast.Repr {
	indexedValue := expression.Expression.Accept(interpreter)
	array, ok := indexedValue.(ArrayValue)
	if !ok {
		panic(fmt.Sprintf("can't index into non-array value: %#+v", indexedValue))
	}

	indexValue := expression.Index.Accept(interpreter)
	index, ok := indexValue.(IntegerValue)
	if !ok {
		panic(fmt.Sprintf("can't index with value: %#+v", indexValue))
	}
	return array[index.IntValue()]
}

func (interpreter *Interpreter) VisitConditionalExpression(expression ast.ConditionalExpression) ast.Repr {
	if expression.Test.Accept(interpreter).(BoolValue) {
		return expression.Then.Accept(interpreter)
	} else {
		return expression.Else.Accept(interpreter)
	}
}

func (interpreter *Interpreter) VisitInvocationExpression(invocationExpression ast.InvocationExpression) ast.Repr {

	// evaluate the invoked expression
	value := invocationExpression.Expression.Accept(interpreter)
	function, ok := value.(*FunctionValue)
	if !ok {
		panic(fmt.Sprintf("can't invoke value: %#+v", value))
	}

	// NOTE: evaluate all argument expressions in call-site scope, not in function body
	arguments := interpreter.evaluateFunctionInvocationArguments(invocationExpression, function)

	return interpreter.invokeFunction(function, arguments)
}

func (interpreter *Interpreter) invokeFunction(function *FunctionValue, arguments []Value) ast.Repr {
	interpreter.checkInvocationArgumentCount(len(arguments), function)

	// start a new activation record
	// lexical scope: use the function declaration's activation record,
	// not the current one (which would be dynamic scope)
	interpreter.activations.Push(function.Activation)
	defer interpreter.activations.Pop()

	interpreter.bindFunctionInvocationParameters(function, arguments)

	return function.Expression.Block.Accept(interpreter)
}

// bindFunctionInvocationParameters binds the argument values to the parameters in the function
func (interpreter *Interpreter) bindFunctionInvocationParameters(
	function *FunctionValue,
	arguments []Value,
) {
	for parameterIndex, parameter := range function.Expression.Parameters {
		argument := arguments[parameterIndex]

		interpreter.activations.Set(
			parameter.Identifier,
			&Variable{
				Declaration: ast.VariableDeclaration{
					IsConst:    true,
					Identifier: parameter.Identifier,
					Type:       parameter.Type,
				},
				Value: argument,
			},
		)
	}
}

// evaluateFunctionInvocationArguments evaluates all function invocation argument expressions
func (interpreter *Interpreter) evaluateFunctionInvocationArguments(
	invocationExpression ast.InvocationExpression,
	function *FunctionValue,
) []Value {
	var arguments []Value
	for _, argumentExpression := range invocationExpression.Arguments {
		argument := argumentExpression.Accept(interpreter).(Value)
		arguments = append(arguments, argument)
	}
	return arguments
}

// checkInvocationArgumentCount ensures the invocation's argument count
// matches the function's parameter count
func (interpreter *Interpreter) checkInvocationArgumentCount(
	argumentCount int,
	function *FunctionValue,
) {
	parameterCount := len(function.Expression.Parameters)
	if argumentCount != parameterCount {
		panic(fmt.Sprintf(
			"invalid number of arguments: got %d, need %d",
			argumentCount,
			parameterCount,
		))
	}
}

func (interpreter *Interpreter) VisitFunctionExpression(expression ast.FunctionExpression) ast.Repr {
	// lexical scope: variables in functions are bound to what is visible at declaration time
	return newFunction(expression, interpreter.activations.CurrentOrNew())
}
