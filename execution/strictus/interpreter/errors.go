package interpreter

import (
	"bamboo-runtime/execution/strictus/ast"
	"fmt"
)

// SecondaryError

// SecondaryError is an interface for errors that provide a secondary error message
//
type SecondaryError interface {
	SecondaryError() string
}

// astTypeConversionError

type astTypeConversionError struct {
	invalidASTType ast.Type
}

func (e *astTypeConversionError) Error() string {
	return fmt.Sprintf("cannot convert unsupported AST type: %#+v", e.invalidASTType)
}

// unsupportedAssignmentTargetExpression

type unsupportedAssignmentTargetExpression struct {
	target ast.Expression
}

func (e *unsupportedAssignmentTargetExpression) Error() string {
	return fmt.Sprintf("cannot assign to unsupported target expression: %#+v", e.target)
}

// unsupportedOperation

type unsupportedOperation struct {
	kind      OperationKind
	operation ast.Operation
	pos       *ast.Position
}

func (e *unsupportedOperation) Error() string {
	return fmt.Sprintf("cannot evaluate unsupported %s operation: %s", e.kind.Name(), e.operation.Symbol())
}

// ProgramError

type ProgramError interface {
	error
	ast.HasPosition
	isProgramError()
}

// NotDeclaredError

type NotDeclaredError struct {
	ExpectedKind DeclarationKind
	Name         string
	StartPos     *ast.Position
	EndPos       *ast.Position
}

func (e *NotDeclaredError) Error() string {
	return fmt.Sprintf("cannot find %s `%s` in this scope", e.ExpectedKind.Name(), e.Name)
}

func (e *NotDeclaredError) SecondaryError() string {
	return "not found in this scope"
}

func (e *NotDeclaredError) StartPosition() *ast.Position {
	return e.StartPos
}

func (e *NotDeclaredError) EndPosition() *ast.Position {
	return e.EndPos
}

// NotCallableError

type NotCallableError struct {
	Value    Value
	StartPos *ast.Position
	EndPos   *ast.Position
}

func (e *NotCallableError) Error() string {
	return fmt.Sprintf("cannot call value: %#+v", e.Value)
}

func (e *NotCallableError) StartPosition() *ast.Position {
	return e.StartPos
}

func (e *NotCallableError) EndPosition() *ast.Position {
	return e.EndPos
}

// NotIndexableError

type NotIndexableError struct {
	Value    Value
	StartPos *ast.Position
	EndPos   *ast.Position
}

func (e *NotIndexableError) Error() string {
	return fmt.Sprintf("cannot index into value: %#+v", e.Value)
}

func (e *NotIndexableError) StartPosition() *ast.Position {
	return e.StartPos
}

func (e *NotIndexableError) EndPosition() *ast.Position {
	return e.EndPos
}

// InvalidUnaryOperandError

type InvalidUnaryOperandError struct {
	Operation    ast.Operation
	ExpectedType Type
	Value        Value
	StartPos     *ast.Position
	EndPos       *ast.Position
}

func (e *InvalidUnaryOperandError) Error() string {
	return fmt.Sprintf(
		"cannot apply unary operation %s to value: %#+v. Expected type %s",
		e.Operation.Symbol(),
		e.Value,
		e.ExpectedType.String(),
	)
}

func (e *InvalidUnaryOperandError) StartPosition() *ast.Position {
	return e.StartPos
}

func (e *InvalidUnaryOperandError) EndPosition() *ast.Position {
	return e.EndPos
}

// InvalidBinaryOperandError

type InvalidBinaryOperandError struct {
	Operation    ast.Operation
	Side         OperandSide
	ExpectedType Type
	Value        Value
	StartPos     *ast.Position
	EndPos       *ast.Position
}

func (e *InvalidBinaryOperandError) Error() string {
	return fmt.Sprintf(
		"cannot apply binary operation %s to %s-hand value: %s. Expected type %s",
		e.Operation.Symbol(),
		e.Side.Name(),
		e.Value,
		e.ExpectedType.String(),
	)
}

func (e *InvalidBinaryOperandError) StartPosition() *ast.Position {
	return e.StartPos
}

func (e *InvalidBinaryOperandError) EndPosition() *ast.Position {
	return e.EndPos
}

// InvalidBinaryOperandTypesError

type InvalidBinaryOperandTypesError struct {
	Operation    ast.Operation
	ExpectedType Type
	LeftValue    Value
	RightValue   Value
	StartPos     *ast.Position
	EndPos       *ast.Position
}

func (e *InvalidBinaryOperandTypesError) Error() string {
	return fmt.Sprintf(
		"can't apply binary operation %s to values: %s, %s. Expected type %s",
		e.Operation.Symbol(),
		e.LeftValue, e.RightValue,
		e.ExpectedType.String(),
	)
}

func (e *InvalidBinaryOperandTypesError) StartPosition() *ast.Position {
	return e.StartPos
}

func (e *InvalidBinaryOperandTypesError) EndPosition() *ast.Position {
	return e.EndPos
}

// ArgumentCountError

type ArgumentCountError struct {
	ParameterCount int
	ArgumentCount  int
	StartPos       *ast.Position
	EndPos         *ast.Position
}

func (e *ArgumentCountError) Error() string {
	return fmt.Sprintf(
		"incorrect number of arguments: got %d, need %d",
		e.ArgumentCount,
		e.ParameterCount,
	)
}

func (e *ArgumentCountError) StartPosition() *ast.Position {
	return e.StartPos
}

func (e *ArgumentCountError) EndPosition() *ast.Position {
	return e.EndPos
}

// RedeclarationError

type RedeclarationError struct {
	Name string
	Pos  *ast.Position
}

func (e *RedeclarationError) Error() string {
	return fmt.Sprintf("cannot redeclare already declared identifier: %s", e.Name)
}

func (e *RedeclarationError) StartPosition() *ast.Position {
	return e.Pos
}

func (e *RedeclarationError) EndPosition() *ast.Position {
	return e.Pos
}

// AssignmentToConstantError

type AssignmentToConstantError struct {
	Name     string
	StartPos *ast.Position
	EndPos   *ast.Position
}

func (e *AssignmentToConstantError) Error() string {
	return fmt.Sprintf("cannot assign to constant: %s", e.Name)
}

func (e *AssignmentToConstantError) StartPosition() *ast.Position {
	return e.StartPos
}

func (e *AssignmentToConstantError) EndPosition() *ast.Position {
	return e.EndPos
}

// InvalidIndexValueError

type InvalidIndexValueError struct {
	Value    Value
	StartPos *ast.Position
	EndPos   *ast.Position
}

func (e *InvalidIndexValueError) Error() string {
	return fmt.Sprintf("cannot index with value: %#+v", e.Value)
}

func (e *InvalidIndexValueError) StartPosition() *ast.Position {
	return e.StartPos
}

func (e *InvalidIndexValueError) EndPosition() *ast.Position {
	return e.EndPos
}
