package sema

import (
	"fmt"
	"math/big"

	"github.com/dapperlabs/flow-go/language/runtime/ast"
	"github.com/dapperlabs/flow-go/language/runtime/common"
	"github.com/dapperlabs/flow-go/language/runtime/errors"
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
	ast.Range
}

func (e *unsupportedOperation) Error() string {
	return fmt.Sprintf(
		"cannot check unsupported %s operation: `%s`",
		e.kind.Name(),
		e.operation.Symbol(),
	)
}

// CheckerError

type CheckerError struct {
	Errors []error
}

func (e CheckerError) Error() string {
	return "Checking failed"
}

func (e CheckerError) ChildErrors() []error {
	return e.Errors
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
	Name string
	ast.Range
}

func (e *AssignmentToConstantError) Error() string {
	return fmt.Sprintf("cannot assign to constant: `%s`", e.Name)
}

func (*AssignmentToConstantError) isSemanticError() {}

// TypeMismatchError

type TypeMismatchError struct {
	ExpectedType Type
	ActualType   Type
	ast.Range
}

func (e *TypeMismatchError) Error() string {
	return "mismatched types"
}

func (*TypeMismatchError) isSemanticError() {}

func (e *TypeMismatchError) SecondaryError() string {
	return fmt.Sprintf(
		"expected `%s`, got `%s`",
		e.ExpectedType,
		e.ActualType,
	)
}

// NotIndexableTypeError

type NotIndexableTypeError struct {
	Type Type
	ast.Range
}

func (e *NotIndexableTypeError) Error() string {
	return fmt.Sprintf(
		"cannot index into value which has type: `%s`",
		e.Type,
	)
}

func (*NotIndexableTypeError) isSemanticError() {}

// NotIndexingTypeError

type NotIndexingTypeError struct {
	Type Type
	ast.Range
}

func (e *NotIndexingTypeError) Error() string {
	return fmt.Sprintf(
		"cannot index with value which has type: `%s`",
		e.Type,
	)
}

func (*NotIndexingTypeError) isSemanticError() {}

// NotEquatableTypeError

type NotEquatableTypeError struct {
	Type Type
	ast.Range
}

func (e *NotEquatableTypeError) Error() string {
	return fmt.Sprintf(
		"cannot compare value which has type: `%s`",
		e.Type,
	)
}

func (*NotEquatableTypeError) isSemanticError() {}

// NotCallableError

type NotCallableError struct {
	Type Type
	ast.Range
}

func (e *NotCallableError) Error() string {
	return fmt.Sprintf("cannot call type: `%s`",
		e.Type,
	)
}

func (*NotCallableError) isSemanticError() {}

// ArgumentCountError

type ArgumentCountError struct {
	ParameterCount int
	ArgumentCount  int
	ast.Range
}

func (e *ArgumentCountError) Error() string {
	return fmt.Sprintf(
		"incorrect number of arguments: expected %d, got %d",
		e.ParameterCount,
		e.ArgumentCount,
	)
}

func (*ArgumentCountError) isSemanticError() {}

// MissingArgumentLabelError

// TODO: suggest adding argument label

type MissingArgumentLabelError struct {
	ExpectedArgumentLabel string
	ast.Range
}

func (e *MissingArgumentLabelError) Error() string {
	return fmt.Sprintf(
		"missing argument label: `%s`",
		e.ExpectedArgumentLabel,
	)
}

func (*MissingArgumentLabelError) isSemanticError() {}

// IncorrectArgumentLabelError

type IncorrectArgumentLabelError struct {
	ExpectedArgumentLabel string
	ActualArgumentLabel   string
	ast.Range
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

// InvalidUnaryOperandError

type InvalidUnaryOperandError struct {
	Operation    ast.Operation
	ExpectedType Type
	ActualType   Type
	ast.Range
}

func (e *InvalidUnaryOperandError) Error() string {
	return fmt.Sprintf(
		"cannot apply unary operation %s to type: expected `%s`, got `%s`",
		e.Operation.Symbol(),
		e.ExpectedType,
		e.ActualType,
	)
}

func (*InvalidUnaryOperandError) isSemanticError() {}

// InvalidBinaryOperandError

type InvalidBinaryOperandError struct {
	Operation    ast.Operation
	Side         common.OperandSide
	ExpectedType Type
	ActualType   Type
	ast.Range
}

func (e *InvalidBinaryOperandError) Error() string {
	return fmt.Sprintf(
		"cannot apply binary operation %s to %s-hand type: expected `%s`, got `%s`",
		e.Operation.Symbol(),
		e.Side.Name(),
		e.ExpectedType,
		e.ActualType,
	)
}

func (*InvalidBinaryOperandError) isSemanticError() {}

// InvalidBinaryOperandsError

type InvalidBinaryOperandsError struct {
	Operation ast.Operation
	LeftType  Type
	RightType Type
	ast.Range
}

func (e *InvalidBinaryOperandsError) Error() string {
	return fmt.Sprintf(
		"cannot apply binary operation %s to different types: `%s`, `%s`",
		e.Operation.Symbol(),
		e.LeftType,
		e.RightType,
	)
}

func (*InvalidBinaryOperandsError) isSemanticError() {}

// ControlStatementError

type ControlStatementError struct {
	ControlStatement common.ControlStatement
	ast.Range
}

func (e *ControlStatementError) Error() string {
	return fmt.Sprintf(
		"control statement outside of loop: `%s`",
		e.ControlStatement.Symbol(),
	)
}

func (*ControlStatementError) isSemanticError() {}

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

// UnknownSpecialFunctionError

type UnknownSpecialFunctionError struct {
	Pos ast.Position
}

func (e *UnknownSpecialFunctionError) Error() string {
	return "unknown special function. did you mean `init`, `destroy`, or forgot the `fun` keyword?"
}

func (*UnknownSpecialFunctionError) isSemanticError() {}

func (e *UnknownSpecialFunctionError) StartPosition() ast.Position {
	return e.Pos
}

func (e *UnknownSpecialFunctionError) EndPosition() ast.Position {
	return e.Pos
}

// InvalidVariableKindError

type InvalidVariableKindError struct {
	Kind ast.VariableKind
	ast.Range
}

func (e *InvalidVariableKindError) Error() string {
	if e.Kind == ast.VariableKindNotSpecified {
		return fmt.Sprintf("missing variable kind")
	}
	return fmt.Sprintf("invalid variable kind: `%s`", e.Kind.Name())
}

func (*InvalidVariableKindError) isSemanticError() {}

// InvalidDeclarationError

type InvalidDeclarationError struct {
	Kind common.DeclarationKind
	ast.Range
}

func (e *InvalidDeclarationError) Error() string {
	return fmt.Sprintf("cannot declare %s here", e.Kind.Name())
}

func (*InvalidDeclarationError) isSemanticError() {}

// MissingInitializerError

type MissingInitializerError struct {
	ContainerType  Type
	FirstFieldName string
	FirstFieldPos  ast.Position
}

func (e *MissingInitializerError) Error() string {
	return fmt.Sprintf(
		"missing initializer for field `%s` in type `%s`",
		e.FirstFieldName,
		e.ContainerType,
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
	Name string
	Type Type
	ast.Range
}

func (e *NotDeclaredMemberError) Error() string {
	return fmt.Sprintf(
		"value of type `%s` has no member `%s`",
		e.Type,
		e.Name,
	)
}

func (e *NotDeclaredMemberError) SecondaryError() string {
	return "unknown member"
}

func (*NotDeclaredMemberError) isSemanticError() {}

// AssignmentToConstantMemberError

// TODO: maybe split up into two errors:
//  - assignment to constant field
//  - assignment to function

type AssignmentToConstantMemberError struct {
	Name string
	ast.Range
}

func (e *AssignmentToConstantMemberError) Error() string {
	return fmt.Sprintf("cannot assign to constant member: `%s`", e.Name)
}

func (*AssignmentToConstantMemberError) isSemanticError() {}

// FieldUninitializedError

type FieldUninitializedError struct {
	Name          string
	ContainerType Type
	Pos           ast.Position
}

func (e *FieldUninitializedError) Error() string {
	return fmt.Sprintf(
		"missing initialization of field `%s` in type `%s`",
		e.Name,
		e.ContainerType,
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
	ast.Range
}

func (e *FunctionExpressionInConditionError) Error() string {
	return "condition contains function"
}

func (*FunctionExpressionInConditionError) isSemanticError() {}

// UnexpectedReturnValueError

type InvalidReturnValueError struct {
	ast.Range
}

func (e *InvalidReturnValueError) Error() string {
	return fmt.Sprintf(
		"invalid return with value from function without %s return type",
		&VoidType{},
	)
}

func (*InvalidReturnValueError) isSemanticError() {}

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
		e.Type,
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
	CompositeMember *Member
	InterfaceMember *Member
}

type InitializerMismatch struct {
	CompositeParameterTypes []*TypeAnnotation
	InterfaceParameterTypes []*TypeAnnotation
}

// TODO: improve error message:
//  use `InitializerMismatch`, `MissingMembers`, `MemberMismatches`, etc

type ConformanceError struct {
	CompositeType       *CompositeType
	InterfaceType       *InterfaceType
	InitializerMismatch *InitializerMismatch
	MissingMembers      []*Member
	MemberMismatches    []MemberMismatch
	Pos                 ast.Position
}

func (e *ConformanceError) Error() string {
	return fmt.Sprintf(
		"structure `%s` does not conform to interface `%s`",
		e.CompositeType.Identifier,
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
	CompositeIdentifier string
	Conformance         *ast.NominalType
}

func (e *DuplicateConformanceError) Error() string {
	return fmt.Sprintf(
		"structure `%s` repeats conformance for interface `%s`",
		e.CompositeIdentifier,
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
	ImportLocation ast.Location
	ast.Range
}

func (e *UnresolvedImportError) Error() string {
	return fmt.Sprintf(
		"import of location `%s` could not be resolved",
		e.ImportLocation,
	)
}

func (*UnresolvedImportError) isSemanticError() {}

// RepeatedImportError

// TODO: make warning?

type RepeatedImportError struct {
	ImportLocation ast.Location
	ast.Range
}

func (e *RepeatedImportError) Error() string {
	return fmt.Sprintf(
		"repeated import of location `%s`",
		e.ImportLocation,
	)
}

func (*RepeatedImportError) isSemanticError() {}

// NotExportedError

type NotExportedError struct {
	Name           string
	ImportLocation ast.Location
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
	ImportLocation ast.Location
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
	Type Type
	ast.Range
}

func (e *UnsupportedTypeError) Error() string {
	return fmt.Sprintf(
		"unsupported type: `%s`",
		e.Type,
	)
}

func (*UnsupportedTypeError) isSemanticError() {}

// UnsupportedDeclarationError

type UnsupportedDeclarationError struct {
	DeclarationKind common.DeclarationKind
	ast.Range
}

func (e *UnsupportedDeclarationError) Error() string {
	return fmt.Sprintf(
		"%s declarations are not supported yet",
		e.DeclarationKind.Name(),
	)
}

func (*UnsupportedDeclarationError) isSemanticError() {}

// UnsupportedOverloadingError

type UnsupportedOverloadingError struct {
	DeclarationKind common.DeclarationKind
	ast.Range
}

func (e *UnsupportedOverloadingError) Error() string {
	return fmt.Sprintf(
		"%s overloading is not supported yet",
		e.DeclarationKind.Name(),
	)
}

func (*UnsupportedOverloadingError) isSemanticError() {}

// CompositeKindMismatchError

type CompositeKindMismatchError struct {
	ExpectedKind common.CompositeKind
	ActualKind   common.CompositeKind
	ast.Range
}

func (e *CompositeKindMismatchError) Error() string {
	return "mismatched composite kinds"
}

func (*CompositeKindMismatchError) isSemanticError() {}

func (e *CompositeKindMismatchError) SecondaryError() string {
	return fmt.Sprintf(
		"expected `%s`, got `%s`",
		e.ExpectedKind.Name(),
		e.ActualKind.Name(),
	)
}

// InvalidIntegerLiteralRangeError

type InvalidIntegerLiteralRangeError struct {
	ExpectedType     Type
	ExpectedRangeMin *big.Int
	ExpectedRangeMax *big.Int
	ast.Range
}

func (e *InvalidIntegerLiteralRangeError) Error() string {
	return fmt.Sprintf(
		"integer literal out of range: expected `%s`, in range [%s, %s]",
		e.ExpectedType,
		e.ExpectedRangeMin,
		e.ExpectedRangeMax,
	)
}

func (*InvalidIntegerLiteralRangeError) isSemanticError() {}

// MissingReturnStatementError

type MissingReturnStatementError struct {
	ast.Range
}

func (e *MissingReturnStatementError) Error() string {
	return "missing return statement"
}

func (*MissingReturnStatementError) isSemanticError() {}

// UnsupportedExpressionError

type UnsupportedExpressionError struct {
	ExpressionKind common.ExpressionKind
	ast.Range
}

func (e *UnsupportedExpressionError) Error() string {
	return fmt.Sprintf(
		"%s expressions are not supported yet",
		e.ExpressionKind.Name(),
	)
}

func (*UnsupportedExpressionError) isSemanticError() {}

// MissingMoveAnnotationError

type MissingMoveAnnotationError struct {
	Pos ast.Position
}

func (e *MissingMoveAnnotationError) Error() string {
	return "missing move annotation: `<-`"
}

func (*MissingMoveAnnotationError) isSemanticError() {}

func (e *MissingMoveAnnotationError) StartPosition() ast.Position {
	return e.Pos
}

func (e *MissingMoveAnnotationError) EndPosition() ast.Position {
	return e.Pos
}

// InvalidNestedMoveError

type InvalidNestedMoveError struct {
	StartPos ast.Position
	EndPos   ast.Position
}

func (e *InvalidNestedMoveError) Error() string {
	return "cannot move nested resource"
}

func (*InvalidNestedMoveError) isSemanticError() {}

func (e *InvalidNestedMoveError) StartPosition() ast.Position {
	return e.StartPos
}

func (e *InvalidNestedMoveError) EndPosition() ast.Position {
	return e.EndPos
}

// InvalidMoveAnnotationError

type InvalidMoveAnnotationError struct {
	Pos ast.Position
}

func (e *InvalidMoveAnnotationError) Error() string {
	return "invalid move annotation: `<-`"
}

func (*InvalidMoveAnnotationError) isSemanticError() {}

func (e *InvalidMoveAnnotationError) StartPosition() ast.Position {
	return e.Pos
}

func (e *InvalidMoveAnnotationError) EndPosition() ast.Position {
	return e.Pos.Shifted(len(common.CompositeKindResource.Annotation()) - 1)
}

// IncorrectTransferOperationError

type IncorrectTransferOperationError struct {
	ActualOperation   ast.TransferOperation
	ExpectedOperation ast.TransferOperation
	Pos               ast.Position
}

func (e *IncorrectTransferOperationError) Error() string {
	return fmt.Sprintf(
		"incorrect transfer operation: expected `%s`",
		e.ExpectedOperation.Operator(),
	)
}

func (*IncorrectTransferOperationError) isSemanticError() {}

func (e *IncorrectTransferOperationError) StartPosition() ast.Position {
	return e.Pos
}

func (e *IncorrectTransferOperationError) EndPosition() ast.Position {
	return e.Pos
}

// InvalidConstructionError

type InvalidConstructionError struct {
	ast.Range
}

func (e *InvalidConstructionError) Error() string {
	return "cannot create value: not a resource"
}

func (*InvalidConstructionError) isSemanticError() {}

// InvalidDestructionError

type InvalidDestructionError struct {
	ast.Range
}

func (e *InvalidDestructionError) Error() string {
	return "cannot destroy value: not a resource"
}

func (*InvalidDestructionError) isSemanticError() {}

// ResourceLossError

type ResourceLossError struct {
	ast.Range
}

func (e *ResourceLossError) Error() string {
	return "loss of resource"
}

func (*ResourceLossError) isSemanticError() {}

// ResourceUseAfterInvalidationError

type ResourceUseAfterInvalidationError struct {
	StartPos      ast.Position
	EndPos        ast.Position
	Invalidations []ResourceInvalidation
	InLoop        bool
	// NOTE: cached values, use `Cause()`
	_wasMoved     bool
	_wasDestroyed bool
	// NOTE: cached value, use `HasInvalidationInPreviousLoopIteration()`
	_hasInvalidationInPreviousLoop *bool
}

func (e *ResourceUseAfterInvalidationError) Cause() (wasMoved, wasDestroyed bool) {
	// check cache
	if e._wasMoved || e._wasDestroyed {
		return e._wasMoved, e._wasDestroyed
	}

	// update cache
	for _, invalidation := range e.Invalidations {
		switch invalidation.Kind {
		case ResourceInvalidationKindMove:
			wasMoved = true
		case ResourceInvalidationKindDestroy:
			wasDestroyed = true
		}
	}

	e._wasMoved = wasMoved
	e._wasDestroyed = wasDestroyed

	return
}

func (e *ResourceUseAfterInvalidationError) Error() string {
	wasMoved, wasDestroyed := e.Cause()
	switch {
	case wasMoved && wasDestroyed:
		return "use of moved or destroyed resource"
	case wasMoved:
		return "use of moved resource"
	case wasDestroyed:
		return "use of destroyed resource"
	default:
		panic(&errors.UnreachableError{})
	}
}

func (e *ResourceUseAfterInvalidationError) SecondaryError() string {
	message := ""
	wasMoved, wasDestroyed := e.Cause()
	switch {
	case wasMoved && wasDestroyed:
		message = "resource used here after being moved or destroyed"
	case wasMoved:
		message = "resource used here after being moved"
	case wasDestroyed:
		message = "resource used here after being destroyed"
	default:
		panic(&errors.UnreachableError{})
	}

	if e.InLoop {
		site := "later"
		if e.HasInvalidationInPreviousLoopIteration() {
			site = "previous"
		}
		message += fmt.Sprintf(", in %s iteration of loop", site)
	}

	return message
}

func (e *ResourceUseAfterInvalidationError) HasInvalidationInPreviousLoopIteration() (result bool) {
	if e._hasInvalidationInPreviousLoop != nil {
		return *e._hasInvalidationInPreviousLoop
	}

	defer func() {
		e._hasInvalidationInPreviousLoop = &result
	}()

	// invalidation occurred in previous loop
	// if all invalidations occur after the use

	for _, invalidation := range e.Invalidations {
		if invalidation.StartPos.Compare(e.StartPos) < 0 {
			return false
		}
	}

	return true
}

func (e *ResourceUseAfterInvalidationError) ErrorNotes() (notes []errors.ErrorNote) {
	for _, invalidation := range e.Invalidations {
		notes = append(notes, ResourceInvalidationNote{
			ResourceInvalidation: invalidation,
			Range: ast.Range{
				StartPos: invalidation.StartPos,
				EndPos:   invalidation.EndPos,
			},
		})
	}
	return
}

func (*ResourceUseAfterInvalidationError) isSemanticError() {}

func (e *ResourceUseAfterInvalidationError) StartPosition() ast.Position {
	return e.StartPos
}

func (e *ResourceUseAfterInvalidationError) EndPosition() ast.Position {
	return e.EndPos
}

// ResourceInvalidationNote

type ResourceInvalidationNote struct {
	ResourceInvalidation
	ast.Range
}

func (n ResourceInvalidationNote) Message() string {
	var action string
	switch n.Kind {
	case ResourceInvalidationKindMove:
		action = "moved"
	case ResourceInvalidationKindDestroy:
		action = "destroyed"
	}
	return fmt.Sprintf("resource %s here", action)
}

// MissingCreateError

type MissingCreateError struct {
	ast.Range
}

func (e *MissingCreateError) Error() string {
	return "cannot create resource: expected `create`"
}

func (*MissingCreateError) isSemanticError() {}

// MissingMoveOperationError

type MissingMoveOperationError struct {
	Pos ast.Position
}

func (e *MissingMoveOperationError) Error() string {
	return "missing move operation: `<-`"
}

func (*MissingMoveOperationError) isSemanticError() {}

func (e *MissingMoveOperationError) StartPosition() ast.Position {
	return e.Pos
}

func (e *MissingMoveOperationError) EndPosition() ast.Position {
	return e.Pos
}

// InvalidMoveOperationError

type InvalidMoveOperationError struct {
	ast.Range
}

func (e *InvalidMoveOperationError) Error() string {
	return "invalid move operation for non-resource: unexpected `<-`"
}

func (*InvalidMoveOperationError) isSemanticError() {}

// ResourceCapturingError

type ResourceCapturingError struct {
	Name string
	Pos  ast.Position
}

func (e *ResourceCapturingError) Error() string {
	return fmt.Sprintf("cannot capture resource in closure: `%s`", e.Name)
}

func (*ResourceCapturingError) isSemanticError() {}

func (e *ResourceCapturingError) StartPosition() ast.Position {
	return e.Pos
}

func (e *ResourceCapturingError) EndPosition() ast.Position {
	length := len(e.Name)
	return e.Pos.Shifted(length - 1)
}

// InvalidResourceFieldError

type InvalidResourceFieldError struct {
	Name string
	Pos  ast.Position
}

func (e *InvalidResourceFieldError) Error() string {
	return fmt.Sprintf("invalid resource field in non-resource: `%s`", e.Name)
}

func (*InvalidResourceFieldError) isSemanticError() {}

func (e *InvalidResourceFieldError) StartPosition() ast.Position {
	return e.Pos
}

func (e *InvalidResourceFieldError) EndPosition() ast.Position {
	length := len(e.Name)
	return e.Pos.Shifted(length - 1)
}

// InvalidTypeIndexingError

type InvalidTypeIndexingError struct {
	ast.Range
}

func (e *InvalidTypeIndexingError) Error() string {
	return "invalid index: expected type"
}

func (*InvalidTypeIndexingError) isSemanticError() {}

// InvalidIndexingError

type InvalidIndexingError struct {
	ast.Range
}

func (e *InvalidIndexingError) Error() string {
	return "invalid index: expected expression"
}

func (*InvalidIndexingError) isSemanticError() {}

// InvalidSwapExpressionError

type InvalidSwapExpressionError struct {
	Side common.OperandSide
	ast.Range
}

func (e *InvalidSwapExpressionError) Error() string {
	return fmt.Sprintf(
		"invalid %s-hand side of swap: expected target expression",
		e.Side.Name(),
	)
}

func (*InvalidSwapExpressionError) isSemanticError() {}

// InvalidEventParameterTypeError

type InvalidEventParameterTypeError struct {
	Type Type
	ast.Range
}

func (e *InvalidEventParameterTypeError) Error() string {
	return fmt.Sprintf(
		"unsupported event parameter type: `%s`",
		e.Type,
	)
}

func (*InvalidEventParameterTypeError) isSemanticError() {}

// InvalidEventUsageError

type InvalidEventUsageError struct {
	ast.Range
}

func (e *InvalidEventUsageError) Error() string {
	return "events can only be invoked in an `emit` statement"
}

func (*InvalidEventUsageError) isSemanticError() {}

// EmitNonEventError

type EmitNonEventError struct {
	Type Type
	ast.Range
}

func (e *EmitNonEventError) Error() string {
	return fmt.Sprintf(
		"cannot emit non-event type: `%s`",
		e.Type,
	)
}

func (*EmitNonEventError) isSemanticError() {}

// EmitImportedEventError

type EmitImportedEventError struct {
	Type Type
	ast.Range
}

func (e *EmitImportedEventError) Error() string {
	return fmt.Sprintf("cannot emit imported event type: `%s`", e.Type)
}

func (*EmitImportedEventError) isSemanticError() {}

// InvalidResourceAssignmentError

type InvalidResourceAssignmentError struct {
	ast.Range
}

func (e *InvalidResourceAssignmentError) Error() string {
	return "cannot assign to resource-typed target. consider swapping (<->)"
}

func (*InvalidResourceAssignmentError) isSemanticError() {}

// InvalidDestructorError

type InvalidDestructorError struct {
	ast.Range
}

func (e *InvalidDestructorError) Error() string {
	return "cannot declare destructor for non-resource"
}

func (*InvalidDestructorError) isSemanticError() {}

// MissingDestructorError

type MissingDestructorError struct {
	ContainerType  Type
	FirstFieldName string
	FirstFieldPos  ast.Position
}

func (e *MissingDestructorError) Error() string {
	return fmt.Sprintf(
		"missing destructor for resource field `%s` in type `%s`",
		e.FirstFieldName,
		e.ContainerType,
	)
}

func (*MissingDestructorError) isSemanticError() {}

func (e *MissingDestructorError) StartPosition() ast.Position {
	return e.FirstFieldPos
}

func (e *MissingDestructorError) EndPosition() ast.Position {
	return e.FirstFieldPos
}

// InvalidDestructorParametersError

type InvalidDestructorParametersError struct {
	ast.Range
}

func (e *InvalidDestructorParametersError) Error() string {
	return "invalid parameters for destructor"
}

func (e *InvalidDestructorParametersError) SecondaryError() string {
	return "consider removing these parameters"
}

func (*InvalidDestructorParametersError) isSemanticError() {}

// ResourceFieldNotInvalidatedError

type ResourceFieldNotInvalidatedError struct {
	FieldName string
	TypeName  string
	Pos       ast.Position
}

func (e *ResourceFieldNotInvalidatedError) Error() string {
	return fmt.Sprintf(
		"field `%s` of resource type `%s` is not invalidated (moved or destroyed)",
		e.FieldName,
		e.TypeName,
	)
}

func (e *ResourceFieldNotInvalidatedError) SecondaryError() string {
	return "not invalidated"
}

func (*ResourceFieldNotInvalidatedError) isSemanticError() {}

func (e *ResourceFieldNotInvalidatedError) StartPosition() ast.Position {
	return e.Pos
}

func (e *ResourceFieldNotInvalidatedError) EndPosition() ast.Position {
	length := len(e.FieldName)
	return e.Pos.Shifted(length - 1)
}

// UninitializedFieldAccessError

type UninitializedFieldAccessError struct {
	Name string
	Pos  ast.Position
}

func (e *UninitializedFieldAccessError) Error() string {
	return fmt.Sprintf(
		"cannot access uninitialized field: `%s`",
		e.Name,
	)
}

func (*UninitializedFieldAccessError) isSemanticError() {}

func (e *UninitializedFieldAccessError) StartPosition() ast.Position {
	return e.Pos
}

func (e *UninitializedFieldAccessError) EndPosition() ast.Position {
	length := len(e.Name)
	return e.Pos.Shifted(length - 1)
}

// UnreachableStatementError

type UnreachableStatementError struct {
	ast.Range
}

func (e *UnreachableStatementError) Error() string {
	return "unreachable statement"
}

func (*UnreachableStatementError) isSemanticError() {}

// UninitializedUseError

type UninitializedUseError struct {
	Name string
	Pos  ast.Position
}

func (e *UninitializedUseError) Error() string {
	return fmt.Sprintf(
		"cannot use incompletely initialized value: `%s`",
		e.Name,
	)
}

func (*UninitializedUseError) isSemanticError() {}

func (e *UninitializedUseError) StartPosition() ast.Position {
	return e.Pos
}

func (e *UninitializedUseError) EndPosition() ast.Position {
	length := len(e.Name)
	return e.Pos.Shifted(length - 1)
}

// InvalidResourceArrayMemberError

type InvalidResourceArrayMemberError struct {
	Name string
	ast.Range
}

func (e *InvalidResourceArrayMemberError) Error() string {
	return fmt.Sprintf(
		"array member `%s` is not available for resource arrays",
		e.Name,
	)
}

func (*InvalidResourceArrayMemberError) isSemanticError() {}

// NonResourceReferenceError

type NonResourceReferenceError struct {
	ActualType Type
	ast.Range
}

func (e *NonResourceReferenceError) Error() string {
	return fmt.Sprintf(
		"cannot create reference: expected resource or resource interface, got `%s`",
		e.ActualType,
	)
}

func (*NonResourceReferenceError) isSemanticError() {}

// NonStorageReferenceError

type NonStorageReferenceError struct {
	ast.Range
}

func (e *NonStorageReferenceError) Error() string {
	return "cannot create reference which is not into storage"
}

func (*NonStorageReferenceError) isSemanticError() {}
