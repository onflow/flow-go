package interpreter

import (
	"bamboo-emulator/execution/strictus/ast"
	"fmt"
)

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

func (interpreter *Interpreter) Invoke(functionName string, arguments ...interface{}) ast.Repr {
	var argumentExpressions []ast.Expression

	for _, argument := range arguments {
		argumentExpressions = append(
			argumentExpressions,
			ast.ToExpression(argument),
		)
	}

	invocation := ast.InvocationExpression{
		Expression: ast.IdentifierExpression{
			Identifier: functionName,
		},
		Arguments: argumentExpressions,
	}

	return invocation.Accept(interpreter)
}

func (interpreter *Interpreter) VisitProgram(program ast.Program) ast.Repr {
	return nil
}

func (interpreter *Interpreter) VisitFunctionDeclaration(declaration ast.FunctionDeclaration) ast.Repr {
	expression := ast.FunctionExpression{
		Parameters: declaration.Parameters,
		ReturnType: declaration.ReturnType,
		Block:      declaration.Block,
	}

	// lexical scope: variables in functions are bound to what is visible at declaration time
	function := newFunction(expression, interpreter.activations.CurrentOrNew())

	// function declarations are de-sugared to constant variables
	interpreter.declareVariable(
		ast.VariableDeclaration{
			Value:      expression,
			Identifier: declaration.Identifier,
			IsConst:    true,
			// TODO: specify parameter types and return type
			Type: ast.FunctionType{},
		},
		function,
	)

	return nil
}

func (interpreter *Interpreter) VisitBlock(block ast.Block) ast.Repr {
	// block scope: each block gets an activation record
	interpreter.activations.PushCurrent()

	for _, statement := range block.Statements {
		result := statement.Accept(interpreter)
		if result != nil {
			interpreter.activations.Pop()
			return result
		}
	}

	interpreter.activations.Pop()
	return nil
}

func (interpreter *Interpreter) VisitReturnStatement(statement ast.ReturnStatement) ast.Repr {
	// NOTE: returning result
	return statement.Expression.Accept(interpreter)
}

func (interpreter *Interpreter) VisitIfStatement(statement ast.IfStatement) ast.Repr {
	if statement.Test.Accept(interpreter).(bool) {
		return statement.Then.Accept(interpreter)
	} else {
		return statement.Else.Accept(interpreter)
	}
}

func (interpreter *Interpreter) VisitWhileStatement(statement ast.WhileStatement) ast.Repr {
	for statement.Test.Accept(interpreter).(bool) {
		result := statement.Block.Accept(interpreter)
		if result != nil {
			return result
		}
	}
	return nil
}

func (interpreter *Interpreter) VisitVariableDeclaration(declaration ast.VariableDeclaration) ast.Repr {
	value := declaration.Value.Accept(interpreter)
	interpreter.declareVariable(declaration, value)
	return nil
}

func (interpreter *Interpreter) declareVariable(declaration ast.VariableDeclaration, value ast.Repr) ast.Repr {
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
	value := assignment.Value.Accept(interpreter)

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
		array, ok := indexedValue.([]interface{})
		if !ok {
			panic(fmt.Sprintf("can't index into non-array value: %#+v", indexedValue))
		}

		indexValue := target.Index.Accept(interpreter)
		index, ok := indexValue.(ast.UInt64Expression)
		if !ok {
			panic(fmt.Sprintf("can't index with value: %#+v", indexValue))
		}
		array[index] = value

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

	leftInt, leftIsInt := left.(ast.IntExpression)
	rightInt, rightIsInt := right.(ast.IntExpression)
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
			return leftInt == rightInt
		case ast.OperationUnequal:
			return leftInt != rightInt
		}
	}

	leftBool, leftIsBool := left.(ast.BoolExpression)
	rightBool, rightIsBool := right.(ast.BoolExpression)
	if leftIsBool && rightIsBool {
		switch expression.Operation {
		case ast.OperationEqual:
			return leftBool == rightBool
		case ast.OperationUnequal:
			return leftBool != rightBool
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

func (interpreter *Interpreter) VisitExpressionStatement(statement ast.ExpressionStatement) ast.Repr {
	statement.Expression.Accept(interpreter)
	return nil
}

func (interpreter *Interpreter) VisitBoolExpression(expression ast.BoolExpression) ast.Repr {
	return expression
}

func (interpreter *Interpreter) VisitInt8Expression(expression ast.Int8Expression) ast.Repr {
	return expression
}

func (interpreter *Interpreter) VisitInt16Expression(expression ast.Int16Expression) ast.Repr {
	return expression
}

func (interpreter *Interpreter) VisitInt32Expression(expression ast.Int32Expression) ast.Repr {
	return expression
}

func (interpreter *Interpreter) VisitInt64Expression(expression ast.Int64Expression) ast.Repr {
	return expression
}

func (interpreter *Interpreter) VisitUInt8Expression(expression ast.UInt8Expression) ast.Repr {
	return expression
}

func (interpreter *Interpreter) VisitUInt16Expression(expression ast.UInt16Expression) ast.Repr {
	return expression
}

func (interpreter *Interpreter) VisitUInt32Expression(expression ast.UInt32Expression) ast.Repr {
	return expression
}

func (interpreter *Interpreter) VisitUInt64Expression(expression ast.UInt64Expression) ast.Repr {
	return expression
}

func (interpreter *Interpreter) VisitArrayExpression(expression ast.ArrayExpression) ast.Repr {
	var values []interface{}

	for _, value := range expression.Values {
		values = append(values, value.Accept(interpreter))
	}

	return values
}

func (interpreter *Interpreter) VisitMemberExpression(ast.MemberExpression) ast.Repr {
	// TODO: no dictionaries yet
	return nil
}

func (interpreter *Interpreter) VisitIndexExpression(expression ast.IndexExpression) ast.Repr {
	indexedValue := expression.Expression.Accept(interpreter)
	array, ok := indexedValue.([]interface{})
	if !ok {
		panic(fmt.Sprintf("can't index into non-array value: %#+v", indexedValue))
	}

	indexValue := expression.Index.Accept(interpreter)
	index, ok := indexValue.(ast.UInt64Expression)
	if !ok {
		panic(fmt.Sprintf("can't index with value: %#+v", indexValue))
	}
	return array[index]
}

func (interpreter *Interpreter) VisitConditionalExpression(expression ast.ConditionalExpression) ast.Repr {
	if expression.Test.Accept(interpreter).(bool) {
		return expression.Then.Accept(interpreter)
	} else {
		return expression.Else.Accept(interpreter)
	}
}

func (interpreter *Interpreter) VisitInvocationExpression(invocationExpression ast.InvocationExpression) ast.Repr {

	// evaluate the invoked expression
	value := invocationExpression.Expression.Accept(interpreter)
	function, ok := value.(*Function)
	if !ok {
		panic(fmt.Sprintf("can't invoke value: %#+v", value))
	}

	// ensure invocation's argument count matches function's parameter count
	argumentCount := len(invocationExpression.Arguments)
	parameterCount := len(function.Expression.Parameters)
	if argumentCount != parameterCount {
		panic(fmt.Sprintf("invalid number of arguments: got %d, need %d", argumentCount, parameterCount))
	}

	// start a new activation record
	// lexical scope: use the function declaration's activation record,
	// not the current one (which would be dynamic scope)
	interpreter.activations.Push(function.Activation)

	// evaluate all argument expressions and bind the resulting values to the parameters
	for parameterIndex, parameter := range function.Expression.Parameters {
		argumentExpression := invocationExpression.Arguments[parameterIndex]
		argument := argumentExpression.Accept(interpreter)

		interpreter.activations.Set(
			parameter.Identifier,
			&Variable{
				Declaration: ast.VariableDeclaration{
					IsConst:    true,
					Identifier: parameter.Identifier,
					Type:       parameter.Type,
					Value:      argumentExpression,
				},
				Value: argument,
			},
		)
	}

	result := function.Expression.Block.Accept(interpreter)

	interpreter.activations.Pop()

	return result
}

func (interpreter *Interpreter) VisitFunctionExpression(expression ast.FunctionExpression) ast.Repr {
	// lexical scope: variables in functions are bound to what is visible at declaration time
	return newFunction(expression, interpreter.activations.CurrentOrNew())
}
