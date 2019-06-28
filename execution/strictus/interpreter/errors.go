package interpreter

import (
	"bamboo-runtime/execution/strictus/ast"
	"fmt"
)

// astTypeConversionError

type astTypeConversionError struct {
	invalidASTType ast.Type
}

func (e *astTypeConversionError) Error() string {
	return fmt.Sprintf("can't convert unsupported AST type: %#+v", e.invalidASTType)
}

// unsupportedAssignmentTargetExpression

type unsupportedAssignmentTargetExpression struct {
	target ast.Expression
}

func (e *unsupportedAssignmentTargetExpression) Error() string {
	return fmt.Sprintf("can't assign to unsupported target expression: %#+v", e.target)
}

// unsupportedOperation

type unsupportedOperation struct {
	kind      OperationKind
	operation ast.Operation
	position  ast.Position
}

func (e *unsupportedOperation) Error() string {
	return fmt.Sprintf("can't evaluate unsupported %s operation: %s", e.kind.Name(), e.operation.String())
}

// NotDeclaredError

type NotDeclaredError struct {
	ExpectedKind DeclarationKind
	Name         string
	Position     ast.Position
}

func (e *NotDeclaredError) Error() string {
	kind := "identifier"
	if e.ExpectedKind != DeclarationKindAny {
		kind = e.ExpectedKind.Name()
	}
	return fmt.Sprintf("can't refer to undeclared %s: %s", kind, e.Name)
}

// NotCallableError

type NotCallableError struct {
	Value         Value
	StartPosition ast.Position
	EndPosition   ast.Position
}

func (e *NotCallableError) Error() string {
	return fmt.Sprintf("can't call value: %#+v", e.Value)
}

// NotIndexableError

type NotIndexableError struct {
	Value         Value
	StartPosition ast.Position
	EndPosition   ast.Position
}

func (e *NotIndexableError) Error() string {
	return fmt.Sprintf("can't index into value: %#+v", e.Value)
}

// InvalidUnaryOperandError

type InvalidUnaryOperandError struct {
	Operation     ast.Operation
	ExpectedType  Type
	Value         Value
	StartPosition ast.Position
	EndPosition   ast.Position
}

func (e *InvalidUnaryOperandError) Error() string {
	return fmt.Sprintf(
		"can't apply unary operation %s to value: %s. Expected type %s",
		e.Operation.Symbol(),
		e.Value,
		e.ExpectedType.String(),
	)
}

// InvalidBinaryOperandError

type InvalidBinaryOperandError struct {
	Operation     ast.Operation
	Side          OperandSide
	ExpectedType  Type
	Value         Value
	StartPosition ast.Position
	EndPosition   ast.Position
}

func (e *InvalidBinaryOperandError) Error() string {
	return fmt.Sprintf(
		"can't apply binary operation %s to %s-hand value: %s. Expected type %s",
		e.Operation.Symbol(),
		e.Side.Name(),
		e.Value,
		e.ExpectedType.String(),
	)
}

// ArgumentCountError

type ArgumentCountError struct {
	ParameterCount int
	ArgumentCount  int
	StartPosition  ast.Position
	EndPosition    ast.Position
}

func (e *ArgumentCountError) Error() string {
	return fmt.Sprintf(
		"incorrect number of arguments: got %d, need %d",
		e.ArgumentCount,
		e.ParameterCount,
	)
}

// RedeclarationError

type RedeclarationError struct {
	Name     string
	Position ast.Position
}

func (e *RedeclarationError) Error() string {
	return fmt.Sprintf("can't redeclare already declared identifier: %s", e.Name)
}

// AssignmentToConstantError

type AssignmentToConstantError struct {
	Name     string
	Position ast.Position
}

func (e *AssignmentToConstantError) Error() string {
	return fmt.Sprintf("can't assign to constant: %s", e.Name)
}

// InvalidIndexValueError

type InvalidIndexValueError struct {
	Value         Value
	StartPosition ast.Position
	EndPosition   ast.Position
}

func (e *InvalidIndexValueError) Error() string {
	return fmt.Sprintf("can't index with value: %#+v", e.Value)
}
