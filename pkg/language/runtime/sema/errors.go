package sema

import (
	"fmt"
	"strings"

	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/ast"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/common"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/errors"
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
	startPos  ast.Position
	endPos    ast.Position
}

func (e *unsupportedOperation) Error() string {
	return fmt.Sprintf(
		"cannot check unsupported %s operation: `%s`",
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

// CheckerError

type CheckerError struct {
	Errors []error
}

func (e CheckerError) Error() string {
	var sb strings.Builder

	sb.WriteString("Checking failed:\n")

	for _, err := range e.Errors {
		sb.WriteString(errors.UnrollChildErrors(err))
		sb.WriteString("\n")
	}

	return sb.String()
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
	Pos         ast.Position
	PreviousPos *ast.Position
}

func (e *RedeclarationError) Error() string {
	return fmt.Sprintf("cannot redeclare %s: `%s` is already declared", e.Kind.Name(), e.Name)
}

func (*RedeclarationError) isSemanticError() {}

func (e *RedeclarationError) StartPosition() ast.Position {
	return e.Pos
}

func (e *RedeclarationError) EndPosition() ast.Position {
	length := len(e.Name)
	return e.Pos.Shifted(length - 1)
}

// NotDeclaredError

type NotDeclaredError struct {
	ExpectedKind common.DeclarationKind
	Name         string
	Pos          ast.Position
}

func (e *NotDeclaredError) Error() string {
	return fmt.Sprintf("cannot find %s in this scope: `%s`", e.ExpectedKind.Name(), e.Name)
}

func (*NotDeclaredError) isSemanticError() {}

func (e *NotDeclaredError) SecondaryError() string {
	return "not found in this scope"
}

func (e *NotDeclaredError) StartPosition() ast.Position {
	return e.Pos
}

func (e *NotDeclaredError) EndPosition() ast.Position {
	length := len(e.Name)
	return e.Pos.Shifted(length - 1)
}

// AssignmentToConstantError

type AssignmentToConstantError struct {
	Name     string
	StartPos ast.Position
	EndPos   ast.Position
}

func (e *AssignmentToConstantError) Error() string {
	return fmt.Sprintf("cannot assign to constant: `%s`", e.Name)
}

func (*AssignmentToConstantError) isSemanticError() {}

func (e *AssignmentToConstantError) StartPosition() ast.Position {
	return e.StartPos
}

func (e *AssignmentToConstantError) EndPosition() ast.Position {
	return e.EndPos
}

// TypeMismatchError

type TypeMismatchError struct {
	ExpectedType Type
	ActualType   Type
	StartPos     ast.Position
	EndPos       ast.Position
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

func (e *TypeMismatchError) StartPosition() ast.Position {
	return e.StartPos
}

func (e *TypeMismatchError) EndPosition() ast.Position {
	return e.EndPos
}

// NotIndexableTypeError

type NotIndexableTypeError struct {
	Type     Type
	StartPos ast.Position
	EndPos   ast.Position
}

func (e *NotIndexableTypeError) Error() string {
	return fmt.Sprintf("cannot index into value which has type: `%s`", e.Type.String())
}

func (*NotIndexableTypeError) isSemanticError() {}

func (e *NotIndexableTypeError) StartPosition() ast.Position {
	return e.StartPos
}

func (e *NotIndexableTypeError) EndPosition() ast.Position {
	return e.EndPos
}

// NotIndexingTypeError

type NotIndexingTypeError struct {
	Type     Type
	StartPos ast.Position
	EndPos   ast.Position
}

func (e *NotIndexingTypeError) Error() string {
	return fmt.Sprintf(
		"cannot index with value which has type: `%s`",
		e.Type.String(),
	)
}

func (*NotIndexingTypeError) isSemanticError() {}

func (e *NotIndexingTypeError) StartPosition() ast.Position {
	return e.StartPos
}

func (e *NotIndexingTypeError) EndPosition() ast.Position {
	return e.EndPos
}

// NotCallableError

type NotCallableError struct {
	Type     Type
	StartPos ast.Position
	EndPos   ast.Position
}

func (e *NotCallableError) Error() string {
	return fmt.Sprintf("cannot call type: `%s`", e.Type.String())
}

func (*NotCallableError) isSemanticError() {}

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
		"incorrect number of arguments: expected %d, got %d",
		e.ParameterCount,
		e.ArgumentCount,
	)
}

func (*ArgumentCountError) isSemanticError() {}

func (e *ArgumentCountError) StartPosition() ast.Position {
	return e.StartPos
}

func (e *ArgumentCountError) EndPosition() ast.Position {
	return e.EndPos
}

// MissingArgumentLabelError

// TODO: suggest adding argument label

type MissingArgumentLabelError struct {
	ExpectedArgumentLabel string
	StartPos              ast.Position
	EndPos                ast.Position
}

func (e *MissingArgumentLabelError) Error() string {
	return fmt.Sprintf(
		"missing argument label: `%s`",
		e.ExpectedArgumentLabel,
	)
}

func (*MissingArgumentLabelError) isSemanticError() {}

func (e *MissingArgumentLabelError) StartPosition() ast.Position {
	return e.StartPos
}

func (e *MissingArgumentLabelError) EndPosition() ast.Position {
	return e.EndPos
}

// IncorrectArgumentLabelError

type IncorrectArgumentLabelError struct {
	ExpectedArgumentLabel string
	ActualArgumentLabel   string
	StartPos              ast.Position
	EndPos                ast.Position
}

func (e *IncorrectArgumentLabelError) Error() string {
	expected := "none"
	if e.ExpectedArgumentLabel != "" {
		expected = fmt.Sprintf(`%s`, e.ExpectedArgumentLabel)
	}
	return fmt.Sprintf(
		"incorrect argument label: expected `%s`, got `%s`",
		expected,
		e.ActualArgumentLabel,
	)
}

func (*IncorrectArgumentLabelError) isSemanticError() {}

func (e *IncorrectArgumentLabelError) StartPosition() ast.Position {
	return e.StartPos
}

func (e *IncorrectArgumentLabelError) EndPosition() ast.Position {
	return e.EndPos
}

// InvalidUnaryOperandError

type InvalidUnaryOperandError struct {
	Operation    ast.Operation
	ExpectedType Type
	ActualType   Type
	StartPos     ast.Position
	EndPos       ast.Position
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

func (e *InvalidUnaryOperandError) StartPosition() ast.Position {
	return e.StartPos
}

func (e *InvalidUnaryOperandError) EndPosition() ast.Position {
	return e.EndPos
}

// InvalidBinaryOperandError

type InvalidBinaryOperandError struct {
	Operation    ast.Operation
	Side         common.OperandSide
	ExpectedType Type
	ActualType   Type
	StartPos     ast.Position
	EndPos       ast.Position
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

func (e *InvalidBinaryOperandError) StartPosition() ast.Position {
	return e.StartPos
}

func (e *InvalidBinaryOperandError) EndPosition() ast.Position {
	return e.EndPos
}

// InvalidBinaryOperandsError

type InvalidBinaryOperandsError struct {
	Operation ast.Operation
	LeftType  Type
	RightType Type
	StartPos  ast.Position
	EndPos    ast.Position
}

func (e *InvalidBinaryOperandsError) Error() string {
	return fmt.Sprintf(
		"cannot apply binary operation %s to different types: `%s`, `%s`",
		e.Operation.Symbol(),
		e.LeftType.String(),
		e.RightType.String(),
	)
}

func (*InvalidBinaryOperandsError) isSemanticError() {}

func (e *InvalidBinaryOperandsError) StartPosition() ast.Position {
	return e.StartPos
}

func (e *InvalidBinaryOperandsError) EndPosition() ast.Position {
	return e.EndPos
}

// ControlStatementError

type ControlStatementError struct {
	ControlStatement common.ControlStatement
	StartPos         ast.Position
	EndPos           ast.Position
}

func (e *ControlStatementError) Error() string {
	return fmt.Sprintf(
		"control statement outside of loop: `%s`",
		e.ControlStatement.Symbol(),
	)
}

func (*ControlStatementError) isSemanticError() {}

func (e *ControlStatementError) StartPosition() ast.Position {
	return e.StartPos
}

func (e *ControlStatementError) EndPosition() ast.Position {
	return e.EndPos
}

// InvalidAccessModifierError

type InvalidAccessModifierError struct {
	DeclarationKind common.DeclarationKind
	Access          ast.Access
	Pos             ast.Position
}

func (e *InvalidAccessModifierError) Error() string {
	return fmt.Sprintf(
		"invalid access modifier for %s: `%s`",
		e.DeclarationKind.Name(),
		e.Access.Keyword(),
	)
}

func (*InvalidAccessModifierError) isSemanticError() {}

func (e *InvalidAccessModifierError) StartPosition() ast.Position {
	return e.Pos
}

func (e *InvalidAccessModifierError) EndPosition() ast.Position {
	length := len(e.Access.Keyword())
	return e.Pos.Shifted(length - 1)
}

// InvalidNameError

type InvalidNameError struct {
	Name string
	Pos  ast.Position
}

func (e *InvalidNameError) Error() string {
	return fmt.Sprintf("invalid name: `%s`", e.Name)
}

func (*InvalidNameError) isSemanticError() {}

func (e *InvalidNameError) StartPosition() ast.Position {
	return e.Pos
}

func (e *InvalidNameError) EndPosition() ast.Position {
	length := len(e.Name)
	return e.Pos.Shifted(length - 1)
}

// InvalidInitializerNameError

type InvalidInitializerNameError struct {
	Name string
	Pos  ast.Position
}

func (e *InvalidInitializerNameError) Error() string {
	return fmt.Sprintf("invalid initializer name: `%s`", e.Name)
}

func (*InvalidInitializerNameError) isSemanticError() {}

func (e *InvalidInitializerNameError) SecondaryError() string {
	return fmt.Sprintf("initializer must be named `%s`", InitializerIdentifier)
}

func (e *InvalidInitializerNameError) StartPosition() ast.Position {
	return e.Pos
}

func (e *InvalidInitializerNameError) EndPosition() ast.Position {
	length := len(e.Name)
	return e.Pos.Shifted(length - 1)
}

// InvalidVariableKindError

type InvalidVariableKindError struct {
	Kind     ast.VariableKind
	StartPos ast.Position
	EndPos   ast.Position
}

func (e *InvalidVariableKindError) Error() string {
	if e.Kind == ast.VariableKindNotSpecified {
		return fmt.Sprintf("missing variable kind")
	}
	return fmt.Sprintf("invalid variable kind: `%s`", e.Kind.Name())
}

func (*InvalidVariableKindError) isSemanticError() {}

func (e *InvalidVariableKindError) StartPosition() ast.Position {
	return e.StartPos
}

func (e *InvalidVariableKindError) EndPosition() ast.Position {
	return e.EndPos
}

// InvalidDeclarationError

type InvalidDeclarationError struct {
	Kind     common.DeclarationKind
	StartPos ast.Position
	EndPos   ast.Position
}

func (e *InvalidDeclarationError) Error() string {
	return fmt.Sprintf("cannot declare %s here", e.Kind.Name())
}

func (*InvalidDeclarationError) isSemanticError() {}

func (e *InvalidDeclarationError) StartPosition() ast.Position {
	return e.StartPos
}

func (e *InvalidDeclarationError) EndPosition() ast.Position {
	return e.EndPos
}

// MissingInitializerError

type MissingInitializerError struct {
	TypeIdentifier string
	FirstFieldName string
	FirstFieldPos  ast.Position
}

func (e *MissingInitializerError) Error() string {
	return fmt.Sprintf(
		"missing initializer for field `%s` of type `%s`",
		e.FirstFieldName,
		e.TypeIdentifier,
	)
}

func (*MissingInitializerError) isSemanticError() {}

func (e *MissingInitializerError) StartPosition() ast.Position {
	return e.FirstFieldPos
}

func (e *MissingInitializerError) EndPosition() ast.Position {
	return e.FirstFieldPos
}

// NotDeclaredMemberError

type NotDeclaredMemberError struct {
	Name     string
	Type     Type
	StartPos ast.Position
	EndPos   ast.Position
}

func (e *NotDeclaredMemberError) Error() string {
	return fmt.Sprintf(
		"value of type `%s` has no member `%s`",
		e.Type.String(),
		e.Name,
	)
}

func (e *NotDeclaredMemberError) SecondaryError() string {
	return "unknown member"
}

func (*NotDeclaredMemberError) isSemanticError() {}

func (e *NotDeclaredMemberError) StartPosition() ast.Position {
	return e.StartPos
}

func (e *NotDeclaredMemberError) EndPosition() ast.Position {
	return e.EndPos
}

// AssignmentToConstantMemberError

// TODO: maybe split up into two errors:
//  - assignment to constant field
//  - assignment to function

type AssignmentToConstantMemberError struct {
	Name     string
	StartPos ast.Position
	EndPos   ast.Position
}

func (e *AssignmentToConstantMemberError) Error() string {
	return fmt.Sprintf("cannot assign to constant member: `%s`", e.Name)
}

func (*AssignmentToConstantMemberError) isSemanticError() {}

func (e *AssignmentToConstantMemberError) StartPosition() ast.Position {
	return e.StartPos
}

func (e *AssignmentToConstantMemberError) EndPosition() ast.Position {
	return e.EndPos
}

// FieldUninitializedError

type FieldUninitializedError struct {
	Name          string
	StructureType *StructureType
	Pos           ast.Position
}

func (e *FieldUninitializedError) Error() string {
	return fmt.Sprintf(
		"field `%s` of type `%s` is not initialized",
		e.Name,
		e.StructureType.Identifier,
	)
}

func (e *FieldUninitializedError) SecondaryError() string {
	return "not initialized"
}

func (*FieldUninitializedError) isSemanticError() {}

func (e *FieldUninitializedError) StartPosition() ast.Position {
	return e.Pos
}

func (e *FieldUninitializedError) EndPosition() ast.Position {
	length := len(e.Name)
	return e.Pos.Shifted(length - 1)
}

// FunctionExpressionInConditionError

type FunctionExpressionInConditionError struct {
	StartPos ast.Position
	EndPos   ast.Position
}

func (e *FunctionExpressionInConditionError) Error() string {
	return "condition contains function"
}

func (*FunctionExpressionInConditionError) isSemanticError() {}

func (e *FunctionExpressionInConditionError) StartPosition() ast.Position {
	return e.StartPos
}

func (e *FunctionExpressionInConditionError) EndPosition() ast.Position {
	return e.EndPos
}

// UnexpectedReturnValueError

type InvalidReturnValueError struct {
	StartPos ast.Position
	EndPos   ast.Position
}

func (e *InvalidReturnValueError) Error() string {
	return fmt.Sprintf(
		"invalid return with value from function without %s return type",
		(&VoidType{}).String(),
	)
}

func (*InvalidReturnValueError) isSemanticError() {}

func (e *InvalidReturnValueError) StartPosition() ast.Position {
	return e.StartPos
}

func (e *InvalidReturnValueError) EndPosition() ast.Position {
	return e.EndPos
}

// InvalidImplementationError

type InvalidImplementationError struct {
	ImplementedKind common.DeclarationKind
	ContainerKind   common.DeclarationKind
	Pos             ast.Position
}

func (e *InvalidImplementationError) Error() string {
	return fmt.Sprintf(
		"cannot implement %s in %s",
		e.ImplementedKind.Name(),
		e.ContainerKind.Name(),
	)
}

func (*InvalidImplementationError) isSemanticError() {}

func (e *InvalidImplementationError) StartPosition() ast.Position {
	return e.Pos
}

func (e *InvalidImplementationError) EndPosition() ast.Position {
	return e.Pos
}

// InvalidConformanceError

type InvalidConformanceError struct {
	Type Type
	Pos  ast.Position
}

func (e *InvalidConformanceError) Error() string {
	return fmt.Sprintf(
		"cannot conform to non-interface type: `%s`",
		e.Type.String(),
	)
}

func (*InvalidConformanceError) isSemanticError() {}

func (e *InvalidConformanceError) StartPosition() ast.Position {
	return e.Pos
}

func (e *InvalidConformanceError) EndPosition() ast.Position {
	return e.Pos
}

// ConformanceError

// TODO: report each missing member and mismatch as note

type MemberMismatch struct {
	StructureMember *Member
	InterfaceMember *Member
}

type InitializerMismatch struct {
	StructureParameterTypes []Type
	InterfaceParameterTypes []Type
}

type ConformanceError struct {
	StructureType       *StructureType
	InterfaceType       *InterfaceType
	InitializerMismatch *InitializerMismatch
	MissingMembers      []*Member
	MemberMismatches    []MemberMismatch
	Pos                 ast.Position
}

func (e *ConformanceError) Error() string {
	return fmt.Sprintf(
		"structure `%s` does not conform to interface `%s`",
		e.StructureType.Identifier,
		e.InterfaceType.Identifier,
	)
}

func (*ConformanceError) isSemanticError() {}

func (e *ConformanceError) StartPosition() ast.Position {
	return e.Pos
}

func (e *ConformanceError) EndPosition() ast.Position {
	return e.Pos
}

// DuplicateConformanceError

// TODO: just make this a warning?

type DuplicateConformanceError struct {
	StructureIdentifier string
	Conformance         *ast.NominalType
}

func (e *DuplicateConformanceError) Error() string {
	return fmt.Sprintf(
		"structure `%s` repeats conformance for interface `%s`",
		e.StructureIdentifier,
		e.Conformance.Identifier.Identifier,
	)
}

func (*DuplicateConformanceError) isSemanticError() {}

func (e *DuplicateConformanceError) StartPosition() ast.Position {
	return e.Conformance.StartPosition()
}

func (e *DuplicateConformanceError) EndPosition() ast.Position {
	return e.Conformance.EndPosition()
}

// UnresolvedImportError

type UnresolvedImportError struct {
	ImportLocation ast.ImportLocation
	StartPos       ast.Position
	EndPos         ast.Position
}

func (e *UnresolvedImportError) Error() string {
	return fmt.Sprintf(
		"import of location `%s` could not be resolved",
		e.ImportLocation,
	)
}

func (*UnresolvedImportError) isSemanticError() {}

func (e *UnresolvedImportError) StartPosition() ast.Position {
	return e.StartPos
}

func (e *UnresolvedImportError) EndPosition() ast.Position {
	return e.EndPos
}

// RepeatedImportError

// TODO: make warning?

type RepeatedImportError struct {
	ImportLocation ast.ImportLocation
	StartPos       ast.Position
	EndPos         ast.Position
}

func (e *RepeatedImportError) Error() string {
	return fmt.Sprintf(
		"repeated import of location `%s`",
		e.ImportLocation,
	)
}

func (*RepeatedImportError) isSemanticError() {}

func (e *RepeatedImportError) StartPosition() ast.Position {
	return e.StartPos
}

func (e *RepeatedImportError) EndPosition() ast.Position {
	return e.EndPos
}

// NotExportedError

type NotExportedError struct {
	Name           string
	ImportLocation ast.ImportLocation
	Pos            ast.Position
}

func (e *NotExportedError) Error() string {
	return fmt.Sprintf("cannot find declaration `%s` in `%s`", e.Name, e.ImportLocation)
}

func (*NotExportedError) isSemanticError() {}

func (e *NotExportedError) StartPosition() ast.Position {
	return e.Pos
}

func (e *NotExportedError) EndPosition() ast.Position {
	length := len(e.Name)
	return e.Pos.Shifted(length - 1)
}

// ImportedProgramError

type ImportedProgramError struct {
	CheckerError   *CheckerError
	ImportLocation ast.ImportLocation
	Pos            ast.Position
}

func (e *ImportedProgramError) Error() string {
	return fmt.Sprintf("checking of imported program `%s` failed", e.ImportLocation)
}

func (e *ImportedProgramError) ChildErrors() []error {
	return e.CheckerError.Errors
}

func (*ImportedProgramError) isSemanticError() {}

func (e *ImportedProgramError) StartPosition() ast.Position {
	return e.Pos
}

func (e *ImportedProgramError) EndPosition() ast.Position {
	return e.Pos
}

// UnsupportedTypeError

type UnsupportedTypeError struct {
	Type     Type
	StartPos ast.Position
	EndPos   ast.Position
}

func (e *UnsupportedTypeError) Error() string {
	return fmt.Sprintf(
		"unsupported type: `%s`",
		e.Type.String(),
	)
}

func (*UnsupportedTypeError) isSemanticError() {}

func (e *UnsupportedTypeError) StartPosition() ast.Position {
	return e.StartPos
}

func (e *UnsupportedTypeError) EndPosition() ast.Position {
	return e.EndPos
}
