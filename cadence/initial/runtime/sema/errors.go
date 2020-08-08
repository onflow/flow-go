/*
 * Cadence - The resource-oriented smart contract programming language
 *
 * Copyright 2019-2020 Dapper Labs, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package sema

import (
	"fmt"
	"math/big"

	"example.com/cadence-initial/runtime/ast"
	"example.com/cadence-initial/runtime/common"
	"example.com/cadence-initial/runtime/errors"
)

// astTypeConversionError

type astTypeConversionError struct {
	invalidASTType ast.Type
}

func (e *astTypeConversionError) Error() string {
	return fmt.Sprintf("cannot convert unsupported AST type: %#+v", e.invalidASTType)
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

// MissingLocationError

type MissingLocationError struct{}

func (e *MissingLocationError) Error() string {
	return "missing location"
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

type RedeclarationError struct {
	Kind        common.DeclarationKind
	Name        string
	Pos         ast.Position
	PreviousPos *ast.Position
}

func (e *RedeclarationError) Error() string {
	return fmt.Sprintf(
		"cannot redeclare %s: `%s` is already declared",
		e.Kind.Name(),
		e.Name,
	)
}

func (*RedeclarationError) isSemanticError() {}

func (e *RedeclarationError) StartPosition() ast.Position {
	return e.Pos
}

func (e *RedeclarationError) EndPosition() ast.Position {
	length := len(e.Name)
	return e.Pos.Shifted(length - 1)
}

func (e *RedeclarationError) ErrorNotes() []errors.ErrorNote {
	if e.PreviousPos == nil {
		return nil
	}

	previousStartPos := *e.PreviousPos
	length := len(e.Name)
	previousEndPos := previousStartPos.Shifted(length - 1)

	return []errors.ErrorNote{
		&RedeclarationNote{
			Range: ast.Range{
				StartPos: previousStartPos,
				EndPos:   previousEndPos,
			},
		},
	}
}

// RedeclarationNote

type RedeclarationNote struct {
	ast.Range
}

func (n RedeclarationNote) Message() string {
	return "previously declared here"
}

// NotDeclaredError

type NotDeclaredError struct {
	ExpectedKind common.DeclarationKind
	Name         string
	Pos          ast.Position
}

func (e *NotDeclaredError) Error() string {
	return fmt.Sprintf(
		"cannot find %s in this scope: `%s`",
		e.ExpectedKind.Name(),
		e.Name,
	)
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
		e.ExpectedType.QualifiedString(),
		e.ActualType.QualifiedString(),
	)
}

// TypeMismatchWithDescriptionError

type TypeMismatchWithDescriptionError struct {
	ExpectedTypeDescription string
	ActualType              Type
	ast.Range
}

func (e *TypeMismatchWithDescriptionError) Error() string {
	return "mismatched types"
}

func (*TypeMismatchWithDescriptionError) isSemanticError() {}

func (e *TypeMismatchWithDescriptionError) SecondaryError() string {
	return fmt.Sprintf(
		"expected %s, got `%s`",
		e.ExpectedTypeDescription,
		e.ActualType.QualifiedString(),
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
		e.Type.QualifiedString(),
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
		e.Type.QualifiedString(),
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
		e.Type.QualifiedString(),
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
		e.Type.QualifiedString(),
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
	return "incorrect number of arguments"
}

func (e *ArgumentCountError) SecondaryError() string {
	return fmt.Sprintf(
		"expected %d, got %d",
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
	return "incorrect argument label"
}

func (e *IncorrectArgumentLabelError) SecondaryError() string {
	expected := "none"
	if e.ExpectedArgumentLabel != "" {
		expected = fmt.Sprintf("`%s`", e.ExpectedArgumentLabel)
	}
	return fmt.Sprintf(
		"expected %s, got `%s`",
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
		"cannot apply unary operation %s to type",
		e.Operation.Symbol(),
	)
}

func (e *InvalidUnaryOperandError) SecondaryError() string {
	return fmt.Sprintf(
		"expected `%s`, got `%s`",
		e.ExpectedType.QualifiedString(),
		e.ActualType.QualifiedString(),
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
		"cannot apply binary operation %s to %s-hand type",
		e.Operation.Symbol(),
		e.Side.Name(),
	)
}

func (e *InvalidBinaryOperandError) SecondaryError() string {
	return fmt.Sprintf(
		"expected `%s`, got `%s`",
		e.ExpectedType.QualifiedString(),
		e.ActualType.QualifiedString(),
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
		"cannot apply binary operation %s to types: `%s`, `%s`",
		e.Operation.Symbol(),
		e.LeftType.QualifiedString(),
		e.RightType.QualifiedString(),
	)
}

func (*InvalidBinaryOperandsError) isSemanticError() {}

// InvalidNilCoalescingRightResourceOperandError

type InvalidNilCoalescingRightResourceOperandError struct {
	ast.Range
}

func (e *InvalidNilCoalescingRightResourceOperandError) Error() string {
	return "nil-coalescing with right-hand resource is not supported at the moment"
}

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
	Explanation     string
	Access          ast.Access
	Pos             ast.Position
}

func (e *InvalidAccessModifierError) Error() string {
	var explanation string
	if e.Explanation != "" {
		explanation = fmt.Sprintf(". %s", e.Explanation)
	}

	if e.Access == ast.AccessNotSpecified {
		return fmt.Sprintf(
			"invalid effective access modifier for %s%s",
			e.DeclarationKind.Name(),
			explanation,
		)
	} else {
		return fmt.Sprintf(
			"invalid access modifier for %s: `%s`%s",
			e.DeclarationKind.Name(),
			e.Access.Keyword(),
			explanation,
		)
	}
}

func (*InvalidAccessModifierError) isSemanticError() {}

func (e *InvalidAccessModifierError) StartPosition() ast.Position {
	return e.Pos
}

func (e *InvalidAccessModifierError) EndPosition() ast.Position {
	length := len(e.Access.Keyword())
	return e.Pos.Shifted(length - 1)
}

// MissingAccessModifierError

type MissingAccessModifierError struct {
	DeclarationKind common.DeclarationKind
	Explanation     string
	Pos             ast.Position
}

func (e *MissingAccessModifierError) Error() string {
	var explanation string
	if e.Explanation != "" {
		explanation = fmt.Sprintf(". %s", e.Explanation)
	}

	return fmt.Sprintf(
		"missing access modifier for %s%s",
		e.DeclarationKind.Name(),
		explanation,
	)
}

func (*MissingAccessModifierError) isSemanticError() {}

func (e *MissingAccessModifierError) StartPosition() ast.Position {
	return e.Pos
}

func (e *MissingAccessModifierError) EndPosition() ast.Position {
	return e.Pos
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
	Identifier string
	Kind       common.DeclarationKind
	ast.Range
}

func (e *InvalidDeclarationError) Error() string {
	if e.Identifier != "" {
		return fmt.Sprintf(
			"cannot declare %s here: `%s`",
			e.Kind.Name(),
			e.Identifier,
		)
	}

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
		e.ContainerType.QualifiedString(),
	)
}

func (*MissingInitializerError) isSemanticError() {}

func (e *MissingInitializerError) StartPosition() ast.Position {
	return e.FirstFieldPos
}

func (e *MissingInitializerError) EndPosition() ast.Position {
	length := len(e.FirstFieldName)
	return e.FirstFieldPos.Shifted(length - 1)
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
		e.Type.QualifiedString(),
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
		e.ContainerType.QualifiedString(),
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
		"invalid return with value from function with `%s` return type",
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
		e.Type.QualifiedString(),
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
	CompositeParameters []*Parameter
	InterfaceParameters []*Parameter
}

// TODO: improve error message:
//  use `InitializerMismatch`, `MissingMembers`, `MemberMismatches`, etc

type ConformanceError struct {
	CompositeType                  *CompositeType
	InterfaceType                  *InterfaceType
	InitializerMismatch            *InitializerMismatch
	MissingMembers                 []*Member
	MemberMismatches               []MemberMismatch
	MissingNestedCompositeTypes    []*CompositeType
	Pos                            ast.Position
	InterfaceTypeIsTypeRequirement bool
}

func (e *ConformanceError) Error() string {
	var interfaceDescription string
	if e.InterfaceTypeIsTypeRequirement {
		interfaceDescription = "type requirement"
	} else {
		interfaceDescription = "interface"
	}

	return fmt.Sprintf(
		"%s `%s` does not conform to %s %s `%s`",
		e.CompositeType.Kind.Name(),
		e.CompositeType.QualifiedString(),
		e.InterfaceType.CompositeKind.Name(),
		interfaceDescription,
		e.InterfaceType.QualifiedString(),
	)
}

func (*ConformanceError) isSemanticError() {}

func (e *ConformanceError) StartPosition() ast.Position {
	return e.Pos
}

func (e *ConformanceError) EndPosition() ast.Position {
	return e.Pos
}

func (e *ConformanceError) ErrorNotes() (notes []errors.ErrorNote) {

	for _, memberMismatch := range e.MemberMismatches {
		compositeMemberIdentifierRange :=
			ast.NewRangeFromPositioned(memberMismatch.CompositeMember.Identifier)

		notes = append(notes, &MemberMismatchNote{
			Range: compositeMemberIdentifierRange,
		})
	}

	return
}

// MemberMismatchNote

type MemberMismatchNote struct {
	ast.Range
}

func (n MemberMismatchNote) Message() string {
	return "mismatch here"
}

// DuplicateConformanceError

// TODO: just make this a warning?

type DuplicateConformanceError struct {
	CompositeType *CompositeType
	InterfaceType *InterfaceType
	ast.Range
}

func (e *DuplicateConformanceError) Error() string {
	return fmt.Sprintf(
		"%s `%s` repeats conformance to %s `%s`",
		e.CompositeType.Kind.Name(),
		e.CompositeType.QualifiedString(),
		e.InterfaceType.CompositeKind.DeclarationKind(true).Name(),
		e.InterfaceType.QualifiedString(),
	)
}

func (*DuplicateConformanceError) isSemanticError() {}

// MissingConformanceError

type MissingConformanceError struct {
	CompositeType *CompositeType
	InterfaceType *InterfaceType
	ast.Range
}

func (e *MissingConformanceError) Error() string {
	return fmt.Sprintf(
		"%s `%s` is missing a declaration to required conformance to %s `%s`",
		e.CompositeType.Kind.Name(),
		e.CompositeType.QualifiedString(),
		e.InterfaceType.CompositeKind.DeclarationKind(true).Name(),
		e.InterfaceType.QualifiedString(),
	)
}

func (*MissingConformanceError) isSemanticError() {}

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
	return fmt.Sprintf(
		"cannot find declaration `%s` in `%s`",
		e.Name,
		e.ImportLocation,
	)
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
	return fmt.Sprintf(
		"checking of imported program `%s` failed",
		e.ImportLocation,
	)
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

// AlwaysFailingNonResourceCastingTypeError

type AlwaysFailingNonResourceCastingTypeError struct {
	ValueType  Type
	TargetType Type
	ast.Range
}

func (e *AlwaysFailingNonResourceCastingTypeError) Error() string {
	return fmt.Sprintf(
		"cast of value of resource-type `%s` to non-resource type `%s` will always fail",
		e.ValueType.QualifiedString(),
		e.TargetType.QualifiedString(),
	)
}

func (*AlwaysFailingNonResourceCastingTypeError) isSemanticError() {}

// AlwaysFailingResourceCastingTypeError

type AlwaysFailingResourceCastingTypeError struct {
	ValueType  Type
	TargetType Type
	ast.Range
}

func (e *AlwaysFailingResourceCastingTypeError) Error() string {
	return fmt.Sprintf(
		"cast of value of non-resource-type `%s` to resource type `%s` will always fail",
		e.ValueType.QualifiedString(),
		e.TargetType.QualifiedString(),
	)
}

func (*AlwaysFailingResourceCastingTypeError) isSemanticError() {}

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
	ExpectedType   Type
	ExpectedMinInt *big.Int
	ExpectedMaxInt *big.Int
	ast.Range
}

func (e *InvalidIntegerLiteralRangeError) Error() string {
	return "integer literal out of range"
}

func (e *InvalidIntegerLiteralRangeError) SecondaryError() string {
	return fmt.Sprintf(
		"expected `%s`, in range [%s, %s]",
		e.ExpectedType.QualifiedString(),
		e.ExpectedMinInt,
		e.ExpectedMaxInt,
	)
}

func (*InvalidIntegerLiteralRangeError) isSemanticError() {}

// InvalidAddressLiteralError

type InvalidAddressLiteralError struct {
	ast.Range
}

func (e *InvalidAddressLiteralError) Error() string {
	return "invalid address"
}

func (*InvalidAddressLiteralError) isSemanticError() {}

// InvalidFixedPointLiteralRangeError

type InvalidFixedPointLiteralRangeError struct {
	ExpectedType          Type
	ExpectedMinInt        *big.Int
	ExpectedMinFractional *big.Int
	ExpectedMaxInt        *big.Int
	ExpectedMaxFractional *big.Int
	ast.Range
}

func (e *InvalidFixedPointLiteralRangeError) Error() string {
	return "fixed-point literal out of range"
}

func (e *InvalidFixedPointLiteralRangeError) SecondaryError() string {
	return fmt.Sprintf(
		"expected `%s`, in range [%s.%s, %s.%s]",
		e.ExpectedType.QualifiedString(),
		e.ExpectedMinInt,
		e.ExpectedMinFractional,
		e.ExpectedMaxInt,
		e.ExpectedMaxFractional,
	)
}

func (*InvalidFixedPointLiteralRangeError) isSemanticError() {}

// InvalidFixedPointLiteralScaleError

type InvalidFixedPointLiteralScaleError struct {
	ExpectedType  Type
	ExpectedScale uint
	ast.Range
}

func (e *InvalidFixedPointLiteralScaleError) Error() string {
	return "fixed-point literal scale out of range"
}

func (e *InvalidFixedPointLiteralScaleError) SecondaryError() string {
	return fmt.Sprintf(
		"expected `%s`, with maximum scale %d",
		e.ExpectedType.QualifiedString(),
		e.ExpectedScale,
	)
}

func (*InvalidFixedPointLiteralScaleError) isSemanticError() {}

// MissingReturnStatementError

type MissingReturnStatementError struct {
	ast.Range
}

func (e *MissingReturnStatementError) Error() string {
	return "missing return statement"
}

func (*MissingReturnStatementError) isSemanticError() {}

// UnsupportedOptionalChainingAssignmentError

type UnsupportedOptionalChainingAssignmentError struct {
	ast.Range
}

func (e *UnsupportedOptionalChainingAssignmentError) Error() string {
	return "cannot assign to optional chaining expression"
}

func (*UnsupportedOptionalChainingAssignmentError) isSemanticError() {}

// MissingResourceAnnotationError

type MissingResourceAnnotationError struct {
	ast.Range
}

func (e *MissingResourceAnnotationError) Error() string {
	return fmt.Sprintf(
		"missing resource annotation: `%s`",
		common.CompositeKindResource.Annotation(),
	)
}

func (*MissingResourceAnnotationError) isSemanticError() {}

// InvalidNestedResourceMoveError

type InvalidNestedResourceMoveError struct {
	StartPos ast.Position
	EndPos   ast.Position
}

func (e *InvalidNestedResourceMoveError) Error() string {
	return "cannot move nested resource"
}

func (*InvalidNestedResourceMoveError) isSemanticError() {}

func (e *InvalidNestedResourceMoveError) StartPosition() ast.Position {
	return e.StartPos
}

func (e *InvalidNestedResourceMoveError) EndPosition() ast.Position {
	return e.EndPos
}

// InvalidResourceAnnotationError

type InvalidResourceAnnotationError struct {
	ast.Range
}

func (e *InvalidResourceAnnotationError) Error() string {
	return fmt.Sprintf(
		"invalid resource annotation: `%s`",
		common.CompositeKindResource.Annotation(),
	)
}

func (*InvalidResourceAnnotationError) isSemanticError() {}

// InvalidInterfaceTypeError

type InvalidInterfaceTypeError struct {
	Type Type
	ast.Range
}

func (e *InvalidInterfaceTypeError) Error() string {
	return "invalid interface type"
}

func (e *InvalidInterfaceTypeError) SecondaryError() string {
	var restrictedAny Type = &AnyStructType{}
	if e.Type.IsResourceType() {
		restrictedAny = &AnyResourceType{}
	}

	return fmt.Sprintf(
		"got `%[1]s`; consider using `%[2]s{%[1]s}`",
		e.Type.QualifiedString(),
		restrictedAny,
	)
}

func (*InvalidInterfaceTypeError) isSemanticError() {}

// IncorrectTransferOperationError

type IncorrectTransferOperationError struct {
	ActualOperation   ast.TransferOperation
	ExpectedOperation ast.TransferOperation
	ast.Range
}

func (e *IncorrectTransferOperationError) Error() string {
	return "incorrect transfer operation"
}

func (e *IncorrectTransferOperationError) SecondaryError() string {
	return fmt.Sprintf(
		"expected `%s`",
		e.ExpectedOperation.Operator(),
	)
}

func (*IncorrectTransferOperationError) isSemanticError() {}

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
		default:
			panic(errors.NewUnreachableError())
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
		panic(errors.NewUnreachableError())
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
		panic(errors.NewUnreachableError())
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
		notes = append(notes, &ResourceInvalidationNote{
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
	default:
		panic(errors.NewUnreachableError())
	}
	return fmt.Sprintf("resource %s here", action)
}

// MissingCreateError

type MissingCreateError struct {
	ast.Range
}

func (e *MissingCreateError) Error() string {
	return "cannot create resource"
}

func (e *MissingCreateError) SecondaryError() string {
	return "expected `create`"
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
	return "invalid move operation for non-resource"
}

func (e *InvalidMoveOperationError) SecondaryError() string {
	return "unexpected `<-`"
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
	Name          string
	CompositeKind common.CompositeKind
	Pos           ast.Position
}

func (e *InvalidResourceFieldError) Error() string {
	return fmt.Sprintf(
		"invalid resource field in %s: `%s`",
		e.CompositeKind.Name(),
		e.Name,
	)
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
	return "invalid index"
}

func (e *InvalidTypeIndexingError) SecondaryError() string {
	return "expected type"
}

func (*InvalidTypeIndexingError) isSemanticError() {}

// InvalidIndexingError

type InvalidIndexingError struct {
	ast.Range
}

func (e *InvalidIndexingError) Error() string {
	return "invalid index"
}

func (e *InvalidIndexingError) SecondaryError() string {
	return "expected expression"
}

func (*InvalidIndexingError) isSemanticError() {}

// InvalidSwapExpressionError

type InvalidSwapExpressionError struct {
	Side common.OperandSide
	ast.Range
}

func (e *InvalidSwapExpressionError) Error() string {
	return fmt.Sprintf(
		"invalid %s-hand side of swap",
		e.Side.Name(),
	)
}

func (e *InvalidSwapExpressionError) SecondaryError() string {
	return "expected target expression"
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
		e.Type.QualifiedString(),
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
		e.Type.QualifiedString(),
	)
}

func (*EmitNonEventError) isSemanticError() {}

// EmitImportedEventError

type EmitImportedEventError struct {
	Type Type
	ast.Range
}

func (e *EmitImportedEventError) Error() string {
	return fmt.Sprintf(
		"cannot emit imported event type: `%s`",
		e.Type.QualifiedString(),
	)
}

func (*EmitImportedEventError) isSemanticError() {}

// InvalidResourceAssignmentError

type InvalidResourceAssignmentError struct {
	ast.Range
}

func (e *InvalidResourceAssignmentError) Error() string {
	return "cannot assign to resource-typed target. consider force assigning (<-!) or swapping (<->)"
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
		e.ContainerType.QualifiedString(),
	)
}

func (*MissingDestructorError) isSemanticError() {}

func (e *MissingDestructorError) StartPosition() ast.Position {
	return e.FirstFieldPos
}

func (e *MissingDestructorError) EndPosition() ast.Position {
	return e.FirstFieldPos.Shifted(len(e.FirstFieldName) - 1)
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
	Type      Type
	Pos       ast.Position
}

func (e *ResourceFieldNotInvalidatedError) Error() string {
	return fmt.Sprintf(
		"field `%s` of type `%s` is not invalidated (moved or destroyed)",
		e.FieldName,
		e.Type.QualifiedString(),
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
	Name            string
	DeclarationKind common.DeclarationKind
	ast.Range
}

func (e *InvalidResourceArrayMemberError) Error() string {
	return fmt.Sprintf(
		"array %s `%s` is not available for resource arrays",
		e.DeclarationKind.Name(),
		e.Name,
	)
}

func (*InvalidResourceArrayMemberError) isSemanticError() {}

// InvalidResourceDictionaryMemberError

type InvalidResourceDictionaryMemberError struct {
	Name            string
	DeclarationKind common.DeclarationKind
	ast.Range
}

func (e *InvalidResourceDictionaryMemberError) Error() string {
	return fmt.Sprintf(
		"dictionary %s `%s` is not available for resource dictionaries",
		e.DeclarationKind.Name(),
		e.Name,
	)
}

func (*InvalidResourceDictionaryMemberError) isSemanticError() {}

// NonReferenceTypeReferenceError

type NonReferenceTypeReferenceError struct {
	ActualType Type
	ast.Range
}

func (e *NonReferenceTypeReferenceError) Error() string {
	return "cannot create reference"
}

func (e *NonReferenceTypeReferenceError) SecondaryError() string {
	return fmt.Sprintf(
		"expected reference type, got `%s`",
		e.ActualType.QualifiedString(),
	)
}

func (*NonReferenceTypeReferenceError) isSemanticError() {}

// OptionalTypeReferenceError

type OptionalTypeReferenceError struct {
	ActualType Type
	ast.Range
}

func (e *OptionalTypeReferenceError) Error() string {
	return fmt.Sprintf(
		"cannot create reference to optional type, got `%s`",
		e.ActualType.QualifiedString(),
	)
}

func (*OptionalTypeReferenceError) isSemanticError() {}

// InvalidResourceCreationError

type InvalidResourceCreationError struct {
	Type Type
	ast.Range
}

func (e *InvalidResourceCreationError) Error() string {
	return fmt.Sprintf(
		"cannot create resource type outside of containing contract: `%s`",
		e.Type.QualifiedString(),
	)
}

func (*InvalidResourceCreationError) isSemanticError() {}

// NonResourceTypeError

type NonResourceTypeError struct {
	ActualType Type
	ast.Range
}

func (e *NonResourceTypeError) Error() string {
	return "invalid type"
}

func (e *NonResourceTypeError) SecondaryError() string {
	return fmt.Sprintf(
		"expected resource type, got `%s`",
		e.ActualType.QualifiedString(),
	)
}

func (*NonResourceTypeError) isSemanticError() {}

// InvalidAssignmentTargetError

type InvalidAssignmentTargetError struct {
	ast.Range
}

func (e *InvalidAssignmentTargetError) Error() string {
	return "cannot assign to expression"
}

func (*InvalidAssignmentTargetError) isSemanticError() {}

// ResourceMethodBindingError

type ResourceMethodBindingError struct {
	ast.Range
}

func (e *ResourceMethodBindingError) Error() string {
	return "cannot create bound method for resource"
}

func (*ResourceMethodBindingError) isSemanticError() {}

// InvalidDictionaryKeyTypeError

type InvalidDictionaryKeyTypeError struct {
	Type Type
	ast.Range
}

func (e *InvalidDictionaryKeyTypeError) Error() string {
	return fmt.Sprintf(
		"cannot use type as dictionary key type: `%s`",
		e.Type.QualifiedString(),
	)
}

func (*InvalidDictionaryKeyTypeError) isSemanticError() {}

// MissingFunctionBodyError

type MissingFunctionBodyError struct {
	Pos ast.Position
}

func (e *MissingFunctionBodyError) Error() string {
	return "missing function implementation"
}

func (*MissingFunctionBodyError) isSemanticError() {}

func (e *MissingFunctionBodyError) StartPosition() ast.Position {
	return e.Pos
}

func (e *MissingFunctionBodyError) EndPosition() ast.Position {
	return e.Pos
}

// InvalidOptionalChainingError

type InvalidOptionalChainingError struct {
	Type Type
	ast.Range
}

func (e *InvalidOptionalChainingError) Error() string {
	return fmt.Sprintf(
		"cannot use optional chaining: type '%s' is not optional",
		e.Type.QualifiedString(),
	)
}

func (*InvalidOptionalChainingError) isSemanticError() {}

// InvalidAccessError

type InvalidAccessError struct {
	Name              string
	RestrictingAccess ast.Access
	DeclarationKind   common.DeclarationKind
	ast.Range
}

func (e *InvalidAccessError) Error() string {
	return fmt.Sprintf(
		"cannot access `%s`: %s has %s access",
		e.Name,
		e.DeclarationKind.Name(),
		e.RestrictingAccess.Description(),
	)
}

func (*InvalidAccessError) isSemanticError() {}

// InvalidAssignmentAccessError

type InvalidAssignmentAccessError struct {
	Name              string
	RestrictingAccess ast.Access
	DeclarationKind   common.DeclarationKind
	ast.Range
}

func (e *InvalidAssignmentAccessError) Error() string {
	return fmt.Sprintf(
		"cannot assign to `%s`: %s has %s access",
		e.Name,
		e.DeclarationKind.Name(),
		e.RestrictingAccess.Description(),
	)
}

func (e *InvalidAssignmentAccessError) SecondaryError() string {
	return fmt.Sprintf(
		"has %s access. Consider making it publicly settable",
		e.RestrictingAccess.Description(),
	)
}

func (*InvalidAssignmentAccessError) isSemanticError() {}

// InvalidCharacterLiteralError

type InvalidCharacterLiteralError struct {
	Length int
	ast.Range
}

func (e *InvalidCharacterLiteralError) Error() string {
	return "character literal has invalid length"
}

func (e *InvalidCharacterLiteralError) SecondaryError() string {
	return fmt.Sprintf("expected 1, got %d",
		e.Length,
	)
}

func (*InvalidCharacterLiteralError) isSemanticError() {}

// InvalidFailableResourceDowncastOutsideOptionalBindingError

type InvalidFailableResourceDowncastOutsideOptionalBindingError struct {
	ast.Range
}

func (e *InvalidFailableResourceDowncastOutsideOptionalBindingError) Error() string {
	return "cannot failably downcast resource type outside of optional binding"
}

func (*InvalidFailableResourceDowncastOutsideOptionalBindingError) isSemanticError() {}

// ReadOnlyTargetAssignmentError

type ReadOnlyTargetAssignmentError struct {
	ast.Range
}

func (e *ReadOnlyTargetAssignmentError) Error() string {
	return "cannot assign to read-only target"
}

func (*ReadOnlyTargetAssignmentError) isSemanticError() {}

// InvalidTransactionBlockError

type InvalidTransactionBlockError struct {
	Name string
	Pos  ast.Position
}

func (e *InvalidTransactionBlockError) Error() string {
	return "invalid transaction block"
}

func (e *InvalidTransactionBlockError) SecondaryError() string {
	return fmt.Sprintf(
		"expected `prepare` or `execute`, got `%s`",
		e.Name,
	)
}

func (*InvalidTransactionBlockError) isSemanticError() {}

func (e *InvalidTransactionBlockError) StartPosition() ast.Position {
	return e.Pos
}

func (e *InvalidTransactionBlockError) EndPosition() ast.Position {
	length := len(e.Name)
	return e.Pos.Shifted(length - 1)
}

// TransactionMissingPrepareError

type TransactionMissingPrepareError struct {
	FirstFieldName string
	FirstFieldPos  ast.Position
}

func (e *TransactionMissingPrepareError) Error() string {
	return fmt.Sprintf(
		"transaction missing prepare function for field `%s`",
		e.FirstFieldName,
	)
}

func (*TransactionMissingPrepareError) isSemanticError() {}

func (e *TransactionMissingPrepareError) StartPosition() ast.Position {
	return e.FirstFieldPos
}

func (e *TransactionMissingPrepareError) EndPosition() ast.Position {
	length := len(e.FirstFieldName)
	return e.FirstFieldPos.Shifted(length - 1)
}

// InvalidResourceTransactionParameterError

type InvalidResourceTransactionParameterError struct {
	Type Type
	ast.Range
}

func (e *InvalidResourceTransactionParameterError) Error() string {
	return fmt.Sprintf(
		"transaction parameter must not be resource type: `%s`",
		e.Type.QualifiedString(),
	)
}

func (*InvalidResourceTransactionParameterError) isSemanticError() {}

// InvalidTransactionFieldAccessModifierError

type InvalidTransactionFieldAccessModifierError struct {
	Name   string
	Access ast.Access
	Pos    ast.Position
}

func (e *InvalidTransactionFieldAccessModifierError) Error() string {
	return fmt.Sprintf(
		"access modifier not allowed for transaction field `%s`: `%s`",
		e.Name,
		e.Access.Keyword(),
	)
}

func (*InvalidTransactionFieldAccessModifierError) isSemanticError() {}

func (e *InvalidTransactionFieldAccessModifierError) StartPosition() ast.Position {
	return e.Pos
}

func (e *InvalidTransactionFieldAccessModifierError) EndPosition() ast.Position {
	length := len(e.Access.Keyword())
	return e.Pos.Shifted(length - 1)
}

// InvalidTransactionPrepareParameterTypeError

type InvalidTransactionPrepareParameterTypeError struct {
	Type Type
	ast.Range
}

func (e *InvalidTransactionPrepareParameterTypeError) Error() string {
	return fmt.Sprintf(
		"prepare parameter must be of type `%s`, not `%s`",
		&AuthAccountType{},
		e.Type.QualifiedString(),
	)
}

func (*InvalidTransactionPrepareParameterTypeError) isSemanticError() {}

// InvalidNestedDeclarationError

type InvalidNestedDeclarationError struct {
	NestedDeclarationKind    common.DeclarationKind
	ContainerDeclarationKind common.DeclarationKind
	ast.Range
}

func (e *InvalidNestedDeclarationError) Error() string {
	return fmt.Sprintf(
		"%s declarations cannot be nested inside %s declarations",
		e.NestedDeclarationKind.Name(),
		e.ContainerDeclarationKind.Name(),
	)
}

func (*InvalidNestedDeclarationError) isSemanticError() {}

// InvalidNestedTypeError

type InvalidNestedTypeError struct {
	Type *ast.NominalType
}

func (e *InvalidNestedTypeError) Error() string {
	return fmt.Sprintf(
		"type does not support nested types: `%s`",
		e.Type,
	)
}

func (*InvalidNestedTypeError) isSemanticError() {}

func (e *InvalidNestedTypeError) StartPosition() ast.Position {
	return e.Type.StartPosition()
}

func (e *InvalidNestedTypeError) EndPosition() ast.Position {
	return e.Type.EndPosition()
}

// DeclarationKindMismatchError

type DeclarationKindMismatchError struct {
	ExpectedDeclarationKind common.DeclarationKind
	ActualDeclarationKind   common.DeclarationKind
	ast.Range
}

func (e *DeclarationKindMismatchError) Error() string {
	return "mismatched declarations"
}

func (*DeclarationKindMismatchError) isSemanticError() {}

func (e *DeclarationKindMismatchError) SecondaryError() string {
	return fmt.Sprintf(
		"expected `%s`, got `%s`",
		e.ExpectedDeclarationKind.Name(),
		e.ActualDeclarationKind.Name(),
	)
}

// InvalidTopLevelDeclarationError

type InvalidTopLevelDeclarationError struct {
	DeclarationKind common.DeclarationKind
	ast.Range
}

func (e *InvalidTopLevelDeclarationError) Error() string {
	return fmt.Sprintf(
		"%s declarations are not valid at the top-level",
		e.DeclarationKind.Name(),
	)
}

func (*InvalidTopLevelDeclarationError) isSemanticError() {}

// InvalidSelfInvalidationError

type InvalidSelfInvalidationError struct {
	InvalidationKind ResourceInvalidationKind
	StartPos         ast.Position
	EndPos           ast.Position
}

func (e *InvalidSelfInvalidationError) Error() string {
	var action string
	switch e.InvalidationKind {
	case ResourceInvalidationKindMove:
		action = "move"
	case ResourceInvalidationKindDestroy:
		action = "destroy"
	default:
		panic(errors.NewUnreachableError())
	}
	return fmt.Sprintf("cannot %s `self`", action)
}

func (*InvalidSelfInvalidationError) isSemanticError() {}

func (e *InvalidSelfInvalidationError) StartPosition() ast.Position {
	return e.StartPos
}

func (e *InvalidSelfInvalidationError) EndPosition() ast.Position {
	return e.EndPos
}

// InvalidMoveError

type InvalidMoveError struct {
	Name            string
	DeclarationKind common.DeclarationKind
	Pos             ast.Position
}

func (e *InvalidMoveError) Error() string {
	return fmt.Sprintf(
		"cannot move %s: `%s`",
		e.DeclarationKind.Name(),
		e.Name,
	)
}

func (*InvalidMoveError) isSemanticError() {}

func (e *InvalidMoveError) StartPosition() ast.Position {
	return e.Pos
}

func (e *InvalidMoveError) EndPosition() ast.Position {
	length := len(e.Name)
	return e.Pos.Shifted(length - 1)
}

// ConstantSizedArrayLiteralSizeError

type ConstantSizedArrayLiteralSizeError struct {
	ActualSize   int64
	ExpectedSize int64
	ast.Range
}

func (e *ConstantSizedArrayLiteralSizeError) Error() string {
	return "incorrect number of array literal elements"
}

func (e *ConstantSizedArrayLiteralSizeError) SecondaryError() string {
	return fmt.Sprintf(
		"expected %d, got %d",
		e.ExpectedSize,
		e.ActualSize,
	)
}

func (*ConstantSizedArrayLiteralSizeError) isSemanticError() {}

// InvalidRestrictedTypeError

type InvalidRestrictedTypeError struct {
	Type Type
	ast.Range
}

func (e *InvalidRestrictedTypeError) Error() string {
	return fmt.Sprintf(
		"cannot restrict type: %s",
		e.Type.QualifiedString(),
	)
}

func (*InvalidRestrictedTypeError) isSemanticError() {}

// InvalidRestrictionTypeError

type InvalidRestrictionTypeError struct {
	Type Type
	ast.Range
}

func (e *InvalidRestrictionTypeError) Error() string {
	return fmt.Sprintf(
		"cannot restrict using non-resource/structure interface type: %s",
		e.Type.QualifiedString(),
	)
}

func (*InvalidRestrictionTypeError) isSemanticError() {}

// RestrictionCompositeKindMismatchError

type RestrictionCompositeKindMismatchError struct {
	CompositeKind         common.CompositeKind
	PreviousCompositeKind common.CompositeKind
	ast.Range
}

func (e *RestrictionCompositeKindMismatchError) Error() string {
	return fmt.Sprintf(
		"interface kind %s does not match previous interface kind %s",
		e.CompositeKind,
		e.PreviousCompositeKind,
	)
}

func (*RestrictionCompositeKindMismatchError) isSemanticError() {}

// InvalidRestrictionTypeDuplicateError

type InvalidRestrictionTypeDuplicateError struct {
	Type *InterfaceType
	ast.Range
}

func (e *InvalidRestrictionTypeDuplicateError) Error() string {
	return fmt.Sprintf(
		"duplicate restriction: %s",
		e.Type.QualifiedString(),
	)
}

func (*InvalidRestrictionTypeDuplicateError) isSemanticError() {}

// InvalidNonConformanceRestrictionError

type InvalidNonConformanceRestrictionError struct {
	Type *InterfaceType
	ast.Range
}

func (e *InvalidNonConformanceRestrictionError) Error() string {
	return fmt.Sprintf(
		"restricted type does not conform to restricting type: %s",
		e.Type.QualifiedString(),
	)
}

func (*InvalidNonConformanceRestrictionError) isSemanticError() {}

// InvalidRestrictedTypeMemberAccessError

type InvalidRestrictedTypeMemberAccessError struct {
	Name string
	ast.Range
}

func (e *InvalidRestrictedTypeMemberAccessError) Error() string {
	return fmt.Sprintf("member of restricted type is not accessible: %s", e.Name)
}

func (*InvalidRestrictedTypeMemberAccessError) isSemanticError() {}

// RestrictionMemberClashError

type RestrictionMemberClashError struct {
	Name                  string
	RedeclaringType       *InterfaceType
	OriginalDeclaringType *InterfaceType
	ast.Range
}

func (e *RestrictionMemberClashError) Error() string {
	return fmt.Sprintf(
		"restriction has member clash with previous restriction `%s`: %s",
		e.OriginalDeclaringType.QualifiedString(),
		e.Name,
	)
}

func (*RestrictionMemberClashError) isSemanticError() {}

// AmbiguousRestrictedTypeError

type AmbiguousRestrictedTypeError struct {
	ast.Range
}

func (e *AmbiguousRestrictedTypeError) Error() string {
	return "ambiguous restricted type"
}

func (*AmbiguousRestrictedTypeError) isSemanticError() {}

// NonOptionalForceError

type NonOptionalForceError struct {
	Type Type
	ast.Range
}

func (e *NonOptionalForceError) Error() string {
	return fmt.Sprintf(
		"cannot force non-optional type: `%s`",
		e.Type.QualifiedString(),
	)
}

func (*NonOptionalForceError) isSemanticError() {}

// InvalidPathDomainError

type InvalidPathDomainError struct {
	ActualDomain string
	ast.Range
}

func (e *InvalidPathDomainError) Error() string {
	return "invalid path domain"
}

func (*InvalidPathDomainError) isSemanticError() {}

var validPathDomainDescription = func() string {
	words := make([]string, 0, len(common.AllPathDomainsByIdentifier))

	for domain := range common.AllPathDomainsByIdentifier {
		words = append(words, fmt.Sprintf("`%s`", domain))
	}

	return common.EnumerateWords(words, "or")
}()

func (e *InvalidPathDomainError) SecondaryError() string {
	return fmt.Sprintf(
		"expected one of %s; got `%s`",
		validPathDomainDescription,
		e.ActualDomain,
	)
}

// InvalidTypeArgumentCountError

type InvalidTypeArgumentCountError struct {
	TypeParameterCount int
	TypeArgumentCount  int
	ast.Range
}

func (e *InvalidTypeArgumentCountError) Error() string {
	return "incorrect number of type arguments"
}

func (e *InvalidTypeArgumentCountError) SecondaryError() string {
	return fmt.Sprintf(
		"expected up to %d, got %d",
		e.TypeParameterCount,
		e.TypeArgumentCount,
	)
}

func (e *InvalidTypeArgumentCountError) isSemanticError() {}

// TypeParameterTypeInferenceError

type TypeParameterTypeInferenceError struct {
	Name string
	ast.Range
}

func (e *TypeParameterTypeInferenceError) Error() string {
	return fmt.Sprintf(
		"type parameter could not be inferred: `%s`",
		e.Name,
	)
}

func (e *TypeParameterTypeInferenceError) isSemanticError() {}

// InvalidConstantSizedTypeBaseError

type InvalidConstantSizedTypeBaseError struct {
	ActualBase   int
	ExpectedBase int
	ast.Range
}

func (e *InvalidConstantSizedTypeBaseError) Error() string {
	return "invalid base for constant sized type size"
}

func (e *InvalidConstantSizedTypeBaseError) SecondaryError() string {
	return fmt.Sprintf(
		"expected %d, got %d",
		e.ActualBase,
		e.ExpectedBase,
	)
}

func (e *InvalidConstantSizedTypeBaseError) isSemanticError() {}

// InvalidConstantSizedTypeSizeError

type InvalidConstantSizedTypeSizeError struct {
	ActualSize     *big.Int
	ExpectedMinInt *big.Int
	ExpectedMaxInt *big.Int
	ast.Range
}

func (e *InvalidConstantSizedTypeSizeError) Error() string {
	return "invalid size for constant sized type"
}

func (e *InvalidConstantSizedTypeSizeError) SecondaryError() string {
	return fmt.Sprintf(
		"expected value in range [%s, %s], got %s",
		e.ExpectedMinInt,
		e.ExpectedMaxInt,
		e.ActualSize,
	)
}

func (e *InvalidConstantSizedTypeSizeError) isSemanticError() {}

// UnsupportedResourceForLoopError

type UnsupportedResourceForLoopError struct {
	ast.Range
}

func (e *UnsupportedResourceForLoopError) Error() string {
	return "cannot loop over resources"
}

func (e *UnsupportedResourceForLoopError) isSemanticError() {}

// TypeParameterTypeMismatchError

type TypeParameterTypeMismatchError struct {
	TypeParameter *TypeParameter
	ExpectedType  Type
	ActualType    Type
	ast.Range
}

func (e *TypeParameterTypeMismatchError) Error() string {
	return "mismatched types for type parameter"
}

func (*TypeParameterTypeMismatchError) isSemanticError() {}

func (e *TypeParameterTypeMismatchError) SecondaryError() string {
	return fmt.Sprintf(
		"type parameter %s is bound to `%s`, but got `%s` here",
		e.TypeParameter.Name,
		e.ExpectedType.QualifiedString(),
		e.ActualType.QualifiedString(),
	)
}
