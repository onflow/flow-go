package interpreter

import (
	"fmt"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/ast"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/common"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/sema"
)

// SecondaryError

// SecondaryError is an interface for errors that provide a secondary error message
//
type SecondaryError interface {
	SecondaryError() string
}

// unsupportedOperation

type unsupportedOperation struct {
	kind      common.OperationKind
	operation ast.Operation
	pos       *ast.Position
}

func (e *unsupportedOperation) Error() string {
	return fmt.Sprintf("cannot evaluate unsupported %s operation: %s", e.kind.Name(), e.operation.Symbol())
}

// NotDeclaredError

type NotDeclaredError struct {
	ExpectedKind common.DeclarationKind
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

// InvalidBinaryOperandError

type InvalidBinaryOperandError struct {
	Operation    ast.Operation
	Side         common.OperandSide
	ExpectedType sema.Type
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
	ExpectedType sema.Type
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
