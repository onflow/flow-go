package interpreter

import (
	"fmt"

	"github.com/dapperlabs/flow-go/language/runtime/ast"
	"github.com/dapperlabs/flow-go/language/runtime/common"
	"github.com/dapperlabs/flow-go/language/runtime/sema"
)

// unsupportedOperation

type unsupportedOperation struct {
	kind      common.OperationKind
	operation ast.Operation
	ast.Range
}

func (e *unsupportedOperation) Error() string {
	return fmt.Sprintf(
		"cannot evaluate unsupported %s operation: %s",
		e.kind.Name(),
		e.operation.Symbol(),
	)
}

// NotDeclaredError

type NotDeclaredError struct {
	ExpectedKind common.DeclarationKind
	Name         string
}

func (e *NotDeclaredError) Error() string {
	return fmt.Sprintf(
		"cannot find %s in this scope: `%s`",
		e.ExpectedKind.Name(),
		e.Name,
	)
}

func (e *NotDeclaredError) SecondaryError() string {
	return "not found in this scope"
}

// NotInvokableError

type NotInvokableError struct {
	Value Value
}

func (e *NotInvokableError) Error() string {
	return fmt.Sprintf("cannot call value: %#+v", e.Value)
}

// ArgumentCountError

type ArgumentCountError struct {
	ParameterCount int
	ArgumentCount  int
}

func (e *ArgumentCountError) Error() string {
	return fmt.Sprintf(
		"incorrect number of arguments: expected %d, got %d",
		e.ParameterCount,
		e.ArgumentCount,
	)
}

// InvalidParameterTypeInInvocationError

type InvalidParameterTypeInInvocationError struct {
	InvalidParameterType sema.Type
}

func (e *InvalidParameterTypeInInvocationError) Error() string {
	return fmt.Sprintf("cannot invoke functions with parameter type: `%s`", e.InvalidParameterType)
}

// TransactionNotDeclaredError

type TransactionNotDeclaredError struct {
	Index int
}

func (e *TransactionNotDeclaredError) Error() string {
	return fmt.Sprintf(
		"cannot find transaction with index %d in this scope",
		e.Index,
	)
}

// ConditionError

type ConditionError struct {
	ConditionKind ast.ConditionKind
	Message       string
	LocationRange LocationRange
}

func (e *ConditionError) ImportLocation() ast.Location {
	return e.LocationRange.Location
}

func (e *ConditionError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("%s failed", e.ConditionKind.Name())
	}
	return fmt.Sprintf("%s failed: %s", e.ConditionKind.Name(), e.Message)
}

func (e *ConditionError) StartPosition() ast.Position {
	return e.LocationRange.StartPos
}

func (e *ConditionError) EndPosition() ast.Position {
	return e.LocationRange.EndPos
}

// RedeclarationError

type RedeclarationError struct {
	Name string
}

func (e *RedeclarationError) Error() string {
	return fmt.Sprintf("cannot redeclare: `%s` is already declared", e.Name)
}

// DereferenceError

type DereferenceError struct {
	LocationRange LocationRange
}

func (e *DereferenceError) ImportLocation() ast.Location {
	return e.LocationRange.Location
}

func (e *DereferenceError) Error() string {
	return "dereference failed"
}

func (e *DereferenceError) StartPosition() ast.Position {
	return e.LocationRange.StartPos
}

func (e *DereferenceError) EndPosition() ast.Position {
	return e.LocationRange.EndPos
}

// OverflowError

type OverflowError struct{}

func (e OverflowError) Error() string {
	return "overflow"
}

// UnderflowError

type UnderflowError struct{}

func (e UnderflowError) Error() string {
	return "underflow"
}

// UnderflowError

type DivisionByZeroError struct{}

func (e DivisionByZeroError) Error() string {
	return "division by zero"
}

// DestroyedCompositeError

type DestroyedCompositeError struct {
	CompositeKind common.CompositeKind
	LocationRange LocationRange
}

func (e *DestroyedCompositeError) ImportLocation() ast.Location {
	return e.LocationRange.Location
}

func (e *DestroyedCompositeError) Error() string {
	return fmt.Sprintf("%s is destroyed", e.CompositeKind)
}

func (e *DestroyedCompositeError) StartPosition() ast.Position {
	return e.LocationRange.StartPos
}

func (e *DestroyedCompositeError) EndPosition() ast.Position {
	return e.LocationRange.EndPos
}

// ForceAssignmentToNonNilResourceError

type ForceAssignmentToNonNilResourceError struct {
	LocationRange LocationRange
}

func (e *ForceAssignmentToNonNilResourceError) ImportLocation() ast.Location {
	return e.LocationRange.Location
}

func (e *ForceAssignmentToNonNilResourceError) Error() string {
	return "force assignment to non-nil resource-typed value"
}

func (e *ForceAssignmentToNonNilResourceError) StartPosition() ast.Position {
	return e.LocationRange.StartPos
}

func (e *ForceAssignmentToNonNilResourceError) EndPosition() ast.Position {
	return e.LocationRange.EndPos
}

// ForceNilError

type ForceNilError struct {
	LocationRange LocationRange
}

func (e *ForceNilError) ImportLocation() ast.Location {
	return e.LocationRange.Location
}

func (e *ForceNilError) Error() string {
	return "unexpectedly found nil while forcing an Optional value"
}

func (e *ForceNilError) StartPosition() ast.Position {
	return e.LocationRange.StartPos
}

func (e *ForceNilError) EndPosition() ast.Position {
	return e.LocationRange.EndPos
}
