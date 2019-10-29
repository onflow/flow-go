package sema

import (
	"github.com/dapperlabs/flow-go/language/runtime/ast"
	"github.com/dapperlabs/flow-go/language/runtime/common"
)

func (checker *Checker) VisitAssignmentStatement(assignment *ast.AssignmentStatement) ast.Repr {
	valueType := assignment.Value.Accept(checker).(Type)
	checker.Elaboration.AssignmentStatementValueTypes[assignment] = valueType

	targetType := checker.visitAssignmentValueType(assignment.Target, assignment.Value, valueType)
	checker.Elaboration.AssignmentStatementTargetTypes[assignment] = targetType

	checker.checkTransfer(assignment.Transfer, valueType)

	// An assignment of a resource or an assignment to a resource is not valid,
	// as it would result in a resource loss

	if valueType.IsResourceType() || targetType.IsResourceType() {

		// However, an assignment to a `self` field in the initializer is allowed.
		// In that case the value that is assigned must be invalidated.
		//
		// The check for a repeated assignment of a constant field after initialization
		// is not part of this logic here, see `visitMemberExpressionAssignment`

		selfFieldMember := checker.selfFieldAccessMember(assignment.Target)

		if selfFieldMember != nil &&
			checker.functionActivations.Current().InitializationInfo != nil {

			checker.recordResourceInvalidation(
				assignment.Value,
				valueType,
				ResourceInvalidationKindMove,
			)
		} else {
			checker.report(
				&InvalidResourceAssignmentError{
					Range: ast.NewRangeFromPositioned(assignment),
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
		variable.DeclarationKind != common.DeclarationKindSelf {

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
				Name:  identifier,
				Range: ast.NewRangeFromPositioned(target),
			},
		)
	}

	// check value type is subtype of variable type
	if !valueType.IsInvalidType() &&
		!checker.IsTypeCompatible(valueExpression, valueType, variable.Type) {

		checker.report(
			&TypeMismatchError{
				ExpectedType: variable.Type,
				ActualType:   valueType,
				Range:        ast.NewRangeFromPositioned(valueExpression),
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

	elementType, _ = checker.visitIndexExpression(target, true)

	if elementType == nil {
		return &InvalidType{}
	}

	if !elementType.IsInvalidType() &&
		!checker.IsTypeCompatible(valueExpression, valueType, elementType) {

		checker.report(
			&TypeMismatchError{
				ExpectedType: elementType,
				ActualType:   valueType,
				Range:        ast.NewRangeFromPositioned(valueExpression),
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

	// If the value type is valid, check that the value can be assigned to the member type

	if !valueType.IsInvalidType() &&
		!checker.IsTypeCompatible(valueExpression, valueType, member.Type) {

		checker.report(
			&TypeMismatchError{
				ExpectedType: member.Type,
				ActualType:   valueType,
				Range:        ast.NewRangeFromPositioned(valueExpression),
			},
		)
	}

	reportAssignmentToConstant := func() {
		checker.report(
			&AssignmentToConstantMemberError{
				Name:  target.Identifier.Identifier,
				Range: ast.NewRangeFromPositioned(valueExpression),
			},
		)
	}

	// If this is an assignment to a `self` field, it needs special handling
	// depending on if the assignment is in an initializer or not

	selfFieldMember := checker.selfFieldAccessMember(target)
	if selfFieldMember != nil {

		functionActivation := checker.functionActivations.Current()

		// If this is an assignment to a `self` field in the initializer,
		// ensure it is only assigned once, to initialize it

		if functionActivation.InitializationInfo != nil {

			// If the function has already returned, the initialization
			// is not definitive, and it must be ignored

			if !functionActivation.ReturnInfo.MaybeReturned {

				// If the field is constant and it has already previously been
				// initialized, report an error for the repeated assignment

				initializedFieldMembers := functionActivation.InitializationInfo.InitializedFieldMembers

				if selfFieldMember.VariableKind == ast.VariableKindConstant &&
					initializedFieldMembers.Contains(selfFieldMember) {

					// TODO: dedicated error: assignment to constant after initialization

					reportAssignmentToConstant()
				} else {
					// This is the initial assignment to the field, record it

					initializedFieldMembers.Add(selfFieldMember)
				}
			}

		} else if selfFieldMember.VariableKind == ast.VariableKindConstant {

			// If this is an assignment outside the initializer,
			// an assignment to a constant field is invalid

			reportAssignmentToConstant()
		}

	} else {

		// The assignment is not to a `self` field. Report if there is an attempt
		// to assign to a constant field, which is always invalid,
		// independent of the location of the assignment (initializer or not)

		if member.VariableKind == ast.VariableKindConstant {

			reportAssignmentToConstant()
		}
	}

	return member.Type
}
