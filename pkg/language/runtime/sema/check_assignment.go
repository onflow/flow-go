package sema

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
)

// TODO: handle potential loss of target's current value if it is a resource

func (checker *Checker) VisitAssignment(assignment *ast.AssignmentStatement) ast.Repr {
	valueType := assignment.Value.Accept(checker).(Type)
	checker.Elaboration.AssignmentStatementValueTypes[assignment] = valueType

	targetType := checker.visitAssignmentValueType(assignment, valueType)
	checker.Elaboration.AssignmentStatementTargetTypes[assignment] = targetType

	checker.checkTransfer(assignment.Transfer, valueType)
	checker.recordResourceInvalidation(
		assignment.Value,
		valueType,
		ResourceInvalidationKindMove,
	)

	return nil
}

func (checker *Checker) visitAssignmentValueType(assignment *ast.AssignmentStatement, valueType Type) (targetType Type) {
	switch target := assignment.Target.(type) {
	case *ast.IdentifierExpression:
		return checker.visitIdentifierExpressionAssignment(assignment, target, valueType)

	case *ast.IndexExpression:
		return checker.visitIndexExpressionAssignment(assignment, target, valueType)

	case *ast.MemberExpression:
		return checker.visitMemberExpressionAssignment(assignment, target, valueType)

	default:
		panic(&unsupportedAssignmentTargetExpression{
			target: target,
		})
	}
}

func (checker *Checker) visitIdentifierExpressionAssignment(
	assignment *ast.AssignmentStatement,
	target *ast.IdentifierExpression,
	valueType Type,
) (targetType Type) {
	identifier := target.Identifier.Identifier

	// check identifier was declared before
	variable := checker.findAndCheckVariable(target.Identifier, true)
	if variable == nil {
		return &InvalidType{}
	}

	// check identifier is not a constant
	if variable.IsConstant {
		checker.report(
			&AssignmentToConstantError{
				Name:     identifier,
				StartPos: target.StartPosition(),
				EndPos:   target.EndPosition(),
			},
		)
	}

	// check value type is subtype of variable type
	if !IsInvalidType(valueType) &&
		!checker.IsTypeCompatible(assignment.Value, valueType, variable.Type) {

		checker.report(
			&TypeMismatchError{
				ExpectedType: variable.Type,
				ActualType:   valueType,
				StartPos:     assignment.Value.StartPosition(),
				EndPos:       assignment.Value.EndPosition(),
			},
		)
	}

	return variable.Type
}

func (checker *Checker) visitIndexExpressionAssignment(
	assignment *ast.AssignmentStatement,
	target *ast.IndexExpression,
	valueType Type,
) (elementType Type) {

	elementType = checker.visitIndexingExpression(target.Expression, target.Index, true)

	if elementType == nil {
		return &InvalidType{}
	}

	if !IsInvalidType(elementType) &&
		!checker.IsTypeCompatible(assignment.Value, valueType, elementType) {

		checker.report(
			&TypeMismatchError{
				ExpectedType: elementType,
				ActualType:   valueType,
				StartPos:     assignment.Value.StartPosition(),
				EndPos:       assignment.Value.EndPosition(),
			},
		)
	}

	return elementType
}

func (checker *Checker) visitMemberExpressionAssignment(
	assignment *ast.AssignmentStatement,
	target *ast.MemberExpression,
	valueType Type,
) (memberType Type) {

	member := checker.visitMember(target)

	if member == nil {
		return
	}

	// check member is not constant

	if member.VariableKind == ast.VariableKindConstant {
		if member.IsInitialized {
			checker.report(
				&AssignmentToConstantMemberError{
					Name:     target.Identifier.Identifier,
					StartPos: assignment.Value.StartPosition(),
					EndPos:   assignment.Value.EndPosition(),
				},
			)
		}
	}

	member.IsInitialized = true

	// if value type is valid, check value can be assigned to member
	if !IsInvalidType(valueType) &&
		!checker.IsTypeCompatible(assignment.Value, valueType, member.Type) {

		checker.report(
			&TypeMismatchError{
				ExpectedType: member.Type,
				ActualType:   valueType,
				StartPos:     assignment.Value.StartPosition(),
				EndPos:       assignment.Value.EndPosition(),
			},
		)
	}

	return member.Type
}
