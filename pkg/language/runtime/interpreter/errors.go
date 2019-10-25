package interpreter

import (
	"fmt"

	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/common"
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
		"cannot find %s `%s` in this scope",
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
		"incorrect number of arguments: got %d, need %d",
		e.ArgumentCount,
		e.ParameterCount,
	)
}

// AnyParameterTypeInInvocationError

type AnyParameterTypeInInvocationError struct{}

func (e *AnyParameterTypeInInvocationError) Error() string {
	return "cannot invoke functions with `Any` parameter type"
}

// ConditionError

type ConditionError struct {
	ConditionKind ast.ConditionKind
	Message       string
	LocationRange LocationRange
}

func (e *ConditionError) ImportLocation() ast.Location {
	return e.LocationRange.ImportLocation
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
