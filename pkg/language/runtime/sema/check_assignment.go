package sema

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/common"
)

func (checker *Checker) VisitAssignmentStatement(assignment *ast.AssignmentStatement) ast.Repr {
	valueType := assignment.Value.Accept(checker).(Type)
	checker.Elaboration.AssignmentStatementValueTypes[assignment] = valueType

	targetType := checker.visitAssignmentValueType(assignment.Target, assignment.Value, valueType)
	checker.Elaboration.AssignmentStatementTargetTypes[assignment] = targetType

	checker.checkTransfer(assignment.Transfer, valueType)

	// Assignment of a resource or assignment to a resource is not valid,
	// as it would result in a resource loss

	if valueType.IsResourceType() || targetType.IsResourceType() {
		// Assignment to self-field is allowed. This is necessary to initialize
		// fields in the initializer

		// TODO: improve exception: this is only allowed once, in the initializer
		//    https://github.com/dapperlabs/flow-go/issues/947

		if checker.selfFieldAccessMember(assignment.Target) != nil {
			checker.recordResourceInvalidation(
				assignment.Value,
				valueType,
				ResourceInvalidationKindMove,
			)
		} else {
			checker.report(
				&InvalidResourceAssignmentError{
					Range: ast.Range{
						StartPos: assignment.StartPosition(),
						EndPos:   assignment.EndPosition(),
					},
				},
			)
		}
	}

	return nil
}

func (checker *Checker) selfFieldAccessMember(expression ast.Expression) *Member {
	memberExpression, isMemberExpression := expression.(*ast.MemberExpression)
	if !isMemberExpression {
		return nil
	}

	identifierExpression, isIdentifierExpression := memberExpression.Expression.(*ast.IdentifierExpression)
	if !isIdentifierExpression {
		return nil
	}

	variable := checker.valueActivations.Find(identifierExpression.Identifier.Identifier)
	if variable == nil ||
		variable.Kind != common.DeclarationKindSelf {

		return nil
	}

	fieldName := memberExpression.Identifier.Identifier
	return variable.Type.(*CompositeType).Members[fieldName]
}

func (checker *Checker) visitAssignmentValueType(
	targetExpression ast.Expression,
	valueExpression ast.Expression,
	valueType Type,
) (targetType Type) {
	switch target := targetExpression.(type) {
	case *ast.IdentifierExpression:
		return checker.visitIdentifierExpressionAssignment(valueExpression, target, valueType)

	case *ast.IndexExpression:
		return checker.visitIndexExpressionAssignment(valueExpression, target, valueType)

	case *ast.MemberExpression:
		return checker.visitMemberExpressionAssignment(valueExpression, target, valueType)

	default:
		panic(&unsupportedAssignmentTargetExpression{
			target: target,
		})
	}
}

func (checker *Checker) visitIdentifierExpressionAssignment(
	valueExpression ast.Expression,
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
				Name: identifier,
				Range: ast.Range{
					StartPos: target.StartPosition(),
					EndPos:   target.EndPosition(),
				},
			},
		)
	}

	// check value type is subtype of variable type
	if !IsInvalidType(valueType) &&
		!checker.IsTypeCompatible(valueExpression, valueType, variable.Type) {

		checker.report(
			&TypeMismatchError{
				ExpectedType: variable.Type,
				ActualType:   valueType,
				Range: ast.Range{
					StartPos: valueExpression.StartPosition(),
					EndPos:   valueExpression.EndPosition(),
				},
			},
		)
	}

	return variable.Type
}

func (checker *Checker) visitIndexExpressionAssignment(
	valueExpression ast.Expression,
	target *ast.IndexExpression,
	valueType Type,
) (elementType Type) {

	elementType = checker.visitIndexingExpression(target, true)

	if elementType == nil {
		return &InvalidType{}
	}

	if !IsInvalidType(elementType) &&
		!checker.IsTypeCompatible(valueExpression, valueType, elementType) {

		checker.report(
			&TypeMismatchError{
				ExpectedType: elementType,
				ActualType:   valueType,
				Range: ast.Range{
					StartPos: valueExpression.StartPosition(),
					EndPos:   valueExpression.EndPosition(),
				},
			},
		)
	}

	return elementType
}

func (checker *Checker) visitMemberExpressionAssignment(
	valueExpression ast.Expression,
	target *ast.MemberExpression,
	valueType Type,
) (memberType Type) {

	member := checker.visitMember(target)

	if member == nil {
		return &InvalidType{}
	}

	// check member is not constant

	if member.VariableKind == ast.VariableKindConstant {
		if member.IsInitialized {
			checker.report(
				&AssignmentToConstantMemberError{
					Name: target.Identifier.Identifier,
					Range: ast.Range{
						StartPos: valueExpression.StartPosition(),
						EndPos:   valueExpression.EndPosition(),
					},
				},
			)
		}
	}

	member.IsInitialized = true

	// if value type is valid, check value can be assigned to member
	if !IsInvalidType(valueType) &&
		!checker.IsTypeCompatible(valueExpression, valueType, member.Type) {

		checker.report(
			&TypeMismatchError{
				ExpectedType: member.Type,
				ActualType:   valueType,
				Range: ast.Range{
					StartPos: valueExpression.StartPosition(),
					EndPos:   valueExpression.EndPosition(),
				},
			},
		)
	}

	return member.Type
}
