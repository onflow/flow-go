package sema

import (
	"fmt"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/ast"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/common"
)

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
	kind      common.OperationKind
	operation ast.Operation
	startPos  *ast.Position
	endPos    *ast.Position
}

func (e *unsupportedOperation) Error() string {
	return fmt.Sprintf(
		"cannot check unsupported %s operation: %s",
		e.kind.Name(),
		e.operation.Symbol(),
	)
}

func (e *unsupportedOperation) StartPosition() *ast.Position {
	return e.startPos
}

func (e *unsupportedOperation) EndPosition() *ast.Position {
	return e.endPos
}

// SemanticError

type SemanticError interface {
	error
	ast.HasPosition
	isSemanticError()
}

// RedeclarationError

// TODO: show previous declaration

type RedeclarationError struct {
	Kind        common.DeclarationKind
	Name        string
	Pos         *ast.Position
	PreviousPos *ast.Position
}

func (e *RedeclarationError) Error() string {
	return fmt.Sprintf("cannot redeclare already declared %s: %s", e.Kind.Name(), e.Name)
}

func (*RedeclarationError) isSemanticError() {}

func (e *RedeclarationError) StartPosition() *ast.Position {
	return e.Pos
}

func (e *RedeclarationError) EndPosition() *ast.Position {
	return e.Pos
}

// NotDeclaredError

type NotDeclaredError struct {
	ExpectedKind common.DeclarationKind
	Name         string
	StartPos     *ast.Position
	EndPos       *ast.Position
}

func (e *NotDeclaredError) Error() string {
	return fmt.Sprintf("cannot find %s in this scope: %s", e.ExpectedKind.Name(), e.Name)
}

func (*NotDeclaredError) isSemanticError() {}

func (e *NotDeclaredError) SecondaryError() string {
	return "not found in this scope"
}

func (e *NotDeclaredError) StartPosition() *ast.Position {
	return e.StartPos
}

func (e *NotDeclaredError) EndPosition() *ast.Position {
	return e.EndPos
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

func (*AssignmentToConstantError) isSemanticError() {}

func (e *AssignmentToConstantError) StartPosition() *ast.Position {
	return e.StartPos
}

func (e *AssignmentToConstantError) EndPosition() *ast.Position {
	return e.EndPos
}

// TypeMismatchError

type TypeMismatchError struct {
	ExpectedType Type
	ActualType   Type
	StartPos     *ast.Position
	EndPos       *ast.Position
}

func (e *TypeMismatchError) Error() string {
	return "mismatched types"
}

func (*TypeMismatchError) isSemanticError() {}

func (e *TypeMismatchError) SecondaryError() string {
	return fmt.Sprintf(
		"expected `%s`, got `%s`",
		e.ExpectedType.String(),
		e.ActualType.String(),
	)
}

func (e *TypeMismatchError) StartPosition() *ast.Position {
	return e.StartPos
}

func (e *TypeMismatchError) EndPosition() *ast.Position {
	return e.EndPos
}

// NotIndexableTypeError

type NotIndexableTypeError struct {
	Type     Type
	StartPos *ast.Position
	EndPos   *ast.Position
}

func (e *NotIndexableTypeError) Error() string {
	return fmt.Sprintf("cannot index into value which has type: %s", e.Type.String())
}

func (*NotIndexableTypeError) isSemanticError() {}

func (e *NotIndexableTypeError) StartPosition() *ast.Position {
	return e.StartPos
}

func (e *NotIndexableTypeError) EndPosition() *ast.Position {
	return e.EndPos
}

// NotIndexingTypeError

type NotIndexingTypeError struct {
	Type     Type
	StartPos *ast.Position
	EndPos   *ast.Position
}

func (e *NotIndexingTypeError) Error() string {
	return fmt.Sprintf("cannot index with value which has type: %s", e.Type.String())
}

func (*NotIndexingTypeError) isSemanticError() {}

func (e *NotIndexingTypeError) StartPosition() *ast.Position {
	return e.StartPos
}

func (e *NotIndexingTypeError) EndPosition() *ast.Position {
	return e.EndPos
}

// NotCallableError

type NotCallableError struct {
	Type     Type
	StartPos *ast.Position
	EndPos   *ast.Position
}

func (e *NotCallableError) Error() string {
	return fmt.Sprintf("cannot call type: %s", e.Type.String())
}

func (*NotCallableError) isSemanticError() {}

func (e *NotCallableError) StartPosition() *ast.Position {
	return e.StartPos
}

func (e *NotCallableError) EndPosition() *ast.Position {
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
		"incorrect number of arguments: expected %d, got %d",
		e.ParameterCount,
		e.ArgumentCount,
	)
}

func (*ArgumentCountError) isSemanticError() {}

func (e *ArgumentCountError) StartPosition() *ast.Position {
	return e.StartPos
}

func (e *ArgumentCountError) EndPosition() *ast.Position {
	return e.EndPos
}

// MissingArgumentLabelError

// TODO: suggest adding argument label

type MissingArgumentLabelError struct {
	ExpectedArgumentLabel string
	StartPos              *ast.Position
	EndPos                *ast.Position
}

func (e *MissingArgumentLabelError) Error() string {
	return fmt.Sprintf(
		"missing argument label: %s",
		e.ExpectedArgumentLabel,
	)
}

func (*MissingArgumentLabelError) isSemanticError() {}

func (e *MissingArgumentLabelError) StartPosition() *ast.Position {
	return e.StartPos
}

func (e *MissingArgumentLabelError) EndPosition() *ast.Position {
	return e.EndPos
}

// IncorrectArgumentLabelError

type IncorrectArgumentLabelError struct {
	ExpectedArgumentLabel string
	ActualArgumentLabel   string
	StartPos              *ast.Position
	EndPos                *ast.Position
}

func (e *IncorrectArgumentLabelError) Error() string {
	expected := "none"
	if e.ExpectedArgumentLabel != "" {
		expected = fmt.Sprintf(`%s`, e.ExpectedArgumentLabel)
	}
	return fmt.Sprintf(
		"incorrect argument label: expected %s, got `%s`",
		expected,
		e.ActualArgumentLabel,
	)
}

func (*IncorrectArgumentLabelError) isSemanticError() {}

func (e *IncorrectArgumentLabelError) StartPosition() *ast.Position {
	return e.StartPos
}

func (e *IncorrectArgumentLabelError) EndPosition() *ast.Position {
	return e.EndPos
}

// InvalidUnaryOperandError

type InvalidUnaryOperandError struct {
	Operation    ast.Operation
	ExpectedType Type
	ActualType   Type
	StartPos     *ast.Position
	EndPos       *ast.Position
}

func (e *InvalidUnaryOperandError) Error() string {
	return fmt.Sprintf(
		"cannot apply unary operation %s to type: expected `%s`, got `%s`",
		e.Operation.Symbol(),
		e.ExpectedType.String(),
		e.ActualType.String(),
	)
}

func (*InvalidUnaryOperandError) isSemanticError() {}

func (e *InvalidUnaryOperandError) StartPosition() *ast.Position {
	return e.StartPos
}

func (e *InvalidUnaryOperandError) EndPosition() *ast.Position {
	return e.EndPos
}

// InvalidBinaryOperandError

type InvalidBinaryOperandError struct {
	Operation    ast.Operation
	Side         common.OperandSide
	ExpectedType Type
	ActualType   Type
	StartPos     *ast.Position
	EndPos       *ast.Position
}

func (e *InvalidBinaryOperandError) Error() string {
	return fmt.Sprintf(
		"cannot apply binary operation %s to %s-hand type: expected `%s`, got `%s`",
		e.Operation.Symbol(),
		e.Side.Name(),
		e.ExpectedType.String(),
		e.ActualType.String(),
	)
}

func (*InvalidBinaryOperandError) isSemanticError() {}

func (e *InvalidBinaryOperandError) StartPosition() *ast.Position {
	return e.StartPos
}

func (e *InvalidBinaryOperandError) EndPosition() *ast.Position {
	return e.EndPos
}

// InvalidBinaryOperandsError

type InvalidBinaryOperandsError struct {
	Operation ast.Operation
	LeftType  Type
	RightType Type
	StartPos  *ast.Position
	EndPos    *ast.Position
}

func (e *InvalidBinaryOperandsError) Error() string {
	return fmt.Sprintf(
		"can't apply binary operation %s to different types: `%s`, `%s`",
		e.Operation.Symbol(),
		e.LeftType.String(),
		e.RightType.String(),
	)
}

func (*InvalidBinaryOperandsError) isSemanticError() {}

func (e *InvalidBinaryOperandsError) StartPosition() *ast.Position {
	return e.StartPos
}

func (e *InvalidBinaryOperandsError) EndPosition() *ast.Position {
	return e.EndPos
}
