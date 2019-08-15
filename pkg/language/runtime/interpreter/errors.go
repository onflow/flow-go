package interpreter

import (
	"fmt"

	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/ast"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/common"
)

// unsupportedOperation

type unsupportedOperation struct {
	kind      common.OperationKind
	operation ast.Operation
	startPos  ast.Position
	endPos    ast.Position
}

func (e *unsupportedOperation) Error() string {
	return fmt.Sprintf(
		"cannot evaluate unsupported %s operation: %s",
		e.kind.Name(),
		e.operation.Symbol(),
	)
}

func (e *unsupportedOperation) StartPosition() ast.Position {
	return e.startPos
}

func (e *unsupportedOperation) EndPosition() ast.Position {
	return e.endPos
}

// NotDeclaredError

type NotDeclaredError struct {
	ExpectedKind common.DeclarationKind
	Name         string
	StartPos     ast.Position
	EndPos       ast.Position
}

func (e *NotDeclaredError) Error() string {
	return fmt.Sprintf("cannot find %s `%s` in this scope", e.ExpectedKind.Name(), e.Name)
}

func (e *NotDeclaredError) SecondaryError() string {
	return "not found in this scope"
}

func (e *NotDeclaredError) StartPosition() ast.Position {
	return e.StartPos
}

func (e *NotDeclaredError) EndPosition() ast.Position {
	return e.EndPos
}

// NotCallableError

type NotCallableError struct {
	Value    Value
	StartPos ast.Position
	EndPos   ast.Position
}

func (e *NotCallableError) Error() string {
	return fmt.Sprintf("cannot call value: %#+v", e.Value)
}

func (e *NotCallableError) StartPosition() ast.Position {
	return e.StartPos
}

func (e *NotCallableError) EndPosition() ast.Position {
	return e.EndPos
}

// ArgumentCountError

type ArgumentCountError struct {
	ParameterCount int
	ArgumentCount  int
	StartPos       ast.Position
	EndPos         ast.Position
}

func (e *ArgumentCountError) Error() string {
	return fmt.Sprintf(
		"incorrect number of arguments: got %d, need %d",
		e.ArgumentCount,
		e.ParameterCount,
	)
}

func (e *ArgumentCountError) StartPosition() ast.Position {
	return e.StartPos
}

func (e *ArgumentCountError) EndPosition() ast.Position {
	return e.EndPos
}

// ConditionError

type ConditionError struct {
	ConditionKind ast.ConditionKind
	StartPos      ast.Position
	EndPos        ast.Position
}

func (e *ConditionError) Error() string {
	return fmt.Sprintf("%s failed", e.ConditionKind.Name())
}

func (e *ConditionError) StartPosition() ast.Position {
	return e.StartPos
}

func (e *ConditionError) EndPosition() ast.Position {
	return e.EndPos
}
