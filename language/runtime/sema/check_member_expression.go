package sema

import (
	"github.com/dapperlabs/flow-go/language/runtime/ast"
	"github.com/dapperlabs/flow-go/language/runtime/common"
)

// NOTE: only called if the member expression is *not* an assignment
//
func (checker *Checker) VisitMemberExpression(expression *ast.MemberExpression) ast.Repr {
	member := checker.visitMember(expression)

	if member == nil {
		return &InvalidType{}
	}

	accessedSelfMember := checker.accessedSelfMember(expression)
	if accessedSelfMember != nil {

		functionActivation := checker.functionActivations.Current()

		// Prevent an access to a field before it was initialized.
		//
		// If this is not an assignment to a `self` member, and the member is a field
		// which must be initialized, ensure the field has been initialized.
		//
		// An access of a member which is not a field / which must not be initialized, is safe
		// (e.g. a composite function call)

		info := functionActivation.InitializationInfo
		isInInitializer := info != nil

		if isInInitializer {
			fieldInitialized := info.InitializedFieldMembers.Contains(accessedSelfMember)

			field := info.FieldMembers[accessedSelfMember]
			if field != nil && !fieldInitialized {

				checker.report(
					&UninitializedFieldAccessError{
						Name: field.Identifier.Identifier,
						Pos:  field.Identifier.Pos,
					},
				)
			}
		}
	}

	return member.Type
}

func (checker *Checker) visitMember(expression *ast.MemberExpression) *Member {
	member, ok := checker.Elaboration.MemberExpressionMembers[expression]
	if ok {
		return member
	}

	accessedExpression := expression.Expression

	var expressionType Type

	func() {
		previousMemberExpression := checker.currentMemberExpression
		checker.currentMemberExpression = expression
		defer func() {
			checker.currentMemberExpression = previousMemberExpression
		}()

		expressionType = accessedExpression.Accept(checker).(Type)
	}()

	checker.checkAccessResourceLoss(expressionType, accessedExpression)

	// If the the access is to a member of `self` and a resource,
	// its use must be recorded/checked, so that it isn't used after it was invalidated

	accessedSelfMember := checker.accessedSelfMember(expression)
	if accessedSelfMember != nil &&
		accessedSelfMember.Type.IsResourceType() {

		// NOTE: Preventing the capturing of the resource field is already implicitly handled:
		// By definition, the resource field can only be nested in a resource,
		// so `self` is a resource, and the capture of it is checked separately

		checker.checkResourceUseAfterInvalidation(accessedSelfMember, expression.Identifier)
		checker.resources.AddUse(accessedSelfMember, expression.Identifier.Pos)
	}

	origins := checker.memberOrigins[expressionType]

	identifier := expression.Identifier.Identifier
	identifierStartPosition := expression.Identifier.StartPosition()
	identifierEndPosition := expression.Identifier.EndPosition()

	// Check if the type instance actually has members. For most types (e.g. composite types)
	// this is known statically (in the sense of this host language (Go), not the implemented language),
	// i.e. a Go type switch would be sufficient.
	// However, for some types (e.g. reference types) this depends on what type is referenced

	if ty, ok := expressionType.(MemberAccessibleType); ok && ty.HasMembers() {
		targetRange := ast.NewRangeFromPositioned(expression.Expression)
		member = ty.GetMember(identifier, targetRange, checker.report)
	}

	if member == nil {
		if !expressionType.IsInvalidType() {
			checker.report(
				&NotDeclaredMemberError{
					Type: expressionType,
					Name: identifier,
					Range: ast.Range{
						StartPos: identifierStartPosition,
						EndPos:   identifierEndPosition,
					},
				},
			)
		}
	} else {
		origin := origins[identifier]
		checker.Occurrences.Put(
			identifierStartPosition,
			identifierEndPosition,
			origin,
		)

		// Check access and report if inaccessible

		// TODO: add option to checker to specify behaviour
		//   for not-specified access modifier

		if !checker.IsAccessibleMember(member) {
			checker.report(
				&InvalidAccessError{
					Name:              member.Identifier.Identifier,
					RestrictingAccess: member.Access,
					DeclarationKind:   member.DeclarationKind,
					Range:             ast.NewRangeFromPositioned(expression),
				},
			)
		}

		// Check that the member access is not to a function of resource type
		// outside of an invocation of it.
		//
		// This would result in a bound method for a resource, which is invalid.

		if !checker.inAssignment &&
			!checker.inInvocation &&
			member.DeclarationKind == common.DeclarationKindFunction &&
			!expressionType.IsInvalidType() &&
			expressionType.IsResourceType() {

			checker.report(
				&ResourceMethodBindingError{
					Range: ast.NewRangeFromPositioned(expression),
				},
			)
		}
	}

	checker.Elaboration.MemberExpressionMembers[expression] = member

	return member
}

func (checker *Checker) IsAccessibleMember(member *Member) bool {
	return member.Access != ast.AccessPrivate ||
		checker.containerTypes[member.ContainerType]
}
