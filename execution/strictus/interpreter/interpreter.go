package interpreter

import (
	"bamboo-emulator/execution/strictus/ast"
	"fmt"
)

type variable struct {
	isConst bool
	value   interface{}
}

type Interpreter struct {
	Program     ast.Program
	activations *Activations
}

func NewInterpreter(program ast.Program) *Interpreter {
	return &Interpreter{
		Program:     program,
		activations: &Activations{},
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
		Identifier: functionName,
		Arguments:  argumentExpressions,
	}

	return interpreter.VisitInvocationExpression(invocation)
}

func (interpreter *Interpreter) VisitProgram(ast.Program) ast.Repr {
	return nil
}

func (interpreter *Interpreter) VisitFunction(ast.Function) ast.Repr {
	return nil
}

func (interpreter *Interpreter) VisitBlock(block ast.Block) ast.Repr {
	// block scope: each block gets an activation record
	interpreter.activations.Push()

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
	if _, exists := interpreter.activations.Find(declaration.Identifier).(variable); exists {
		panic(fmt.Sprintf("invalid redefinition of identifier: %s", declaration.Identifier))
	}

	value := declaration.Value.Accept(interpreter)
	interpreter.activations.Set(
		declaration.Identifier,
		variable{
			isConst: declaration.IsConst,
			value:   value,
		})

	return nil
}

func (interpreter *Interpreter) VisitAssignment(assignment ast.Assignment) ast.Repr {
	variable, ok := interpreter.activations.Find(assignment.Identifier).(variable)
	if !ok {
		panic(fmt.Sprintf("reference to unbound identifier: %s", assignment.Identifier))
	}
	if variable.isConst {
		panic(fmt.Sprintf("invalid assignment to constant: %s", assignment.Identifier))
	}
	variable.value = assignment.Value.Accept(interpreter)
	interpreter.activations.Set(assignment.Identifier, variable)
	return nil
}

func (interpreter *Interpreter) VisitIdentifierExpression(expression ast.IdentifierExpression) ast.Repr {
	variable, ok := interpreter.activations.Find(expression.Identifier).(variable)
	if !ok {
		panic(fmt.Sprintf("reference to unbound identifier: %s", expression.Identifier))
	}
	return variable.value
}

func (interpreter *Interpreter) VisitBinaryExpression(expression ast.BinaryExpression) ast.Repr {
	left := expression.Left.Accept(interpreter)
	right := expression.Right.Accept(interpreter)

	leftInt, leftIsInt := left.(int64)
	rightInt, rightIsInt := right.(int64)
	if leftIsInt && rightIsInt {
		switch expression.Operation {
		case ast.OperationPlus:
			return leftInt + rightInt
		case ast.OperationMinus:
			return leftInt - rightInt
		case ast.OperationMod:
			return leftInt % rightInt
		case ast.OperationMul:
			return leftInt * rightInt
		case ast.OperationDiv:
			return leftInt / rightInt
		case ast.OperationLess:
			return leftInt < rightInt
		case ast.OperationLessEqual:
			return leftInt <= rightInt
		case ast.OperationGreater:
			return leftInt > rightInt
		case ast.OperationGreaterEqual:
			return leftInt >= rightInt
		case ast.OperationEqual:
			return leftInt == rightInt
		case ast.OperationUnequal:
			return leftInt != rightInt
		}
	}

	leftBool, leftIsBool := left.(bool)
	rightBool, rightIsBool := right.(bool)
	if leftIsBool && rightIsBool {
		switch expression.Operation {
		case ast.OperationEqual:
			return leftBool == rightBool
		case ast.OperationUnequal:
			return leftBool != rightBool
		}
	}

	return nil
}

func (interpreter *Interpreter) VisitExpressionStatement(statement ast.ExpressionStatement) ast.Repr {
	statement.Expression.Accept(interpreter)
	return nil
}

func (interpreter *Interpreter) VisitBoolExpression(expression ast.BoolExpression) ast.Repr {
	return expression.Value
}

func (interpreter *Interpreter) VisitIntExpression(expression ast.IntExpression) ast.Repr {
	return expression.Value
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
	value, ok := expression.Expression.Accept(interpreter).([]interface{})
	if !ok {
		return nil
	}
	index, ok := expression.Index.Accept(interpreter).(int64)
	if !ok {
		return nil
	}
	return value[index]
}

func (interpreter *Interpreter) VisitConditionalExpression(expression ast.ConditionalExpression) ast.Repr {
	if expression.Test.Accept(interpreter).(bool) {
		return expression.Then.Accept(interpreter)
	} else {
		return expression.Else.Accept(interpreter)
	}
}

func (interpreter *Interpreter) VisitInvocationExpression(expression ast.InvocationExpression) ast.Repr {
	var arguments []ast.Repr

	for _, argument := range expression.Arguments {
		arguments = append(
			arguments,
			argument.Accept(interpreter),
		)
	}

	functionName := expression.Identifier
	function, ok := interpreter.Program.Functions[functionName]
	if !ok {
		panic(fmt.Sprintf("unknown function: %s", functionName))
	}

	argumentCount := len(arguments)
	parameterCount := len(function.Parameters)
	if argumentCount != parameterCount {
		panic(fmt.Sprintf("invalid number of arguments: got %d, need %d", argumentCount, parameterCount))
	}

	interpreter.activations.Push()

	for parameterIndex, parameter := range function.Parameters {
		argument := arguments[parameterIndex]
		interpreter.activations.Set(
			parameter.Identifier,
			variable{isConst: true, value: argument},
		)
	}

	result := interpreter.VisitBlock(function.Block)

	interpreter.activations.Pop()

	return result
}
