package sema

import (
	"github.com/dapperlabs/flow-go/language/runtime/ast"
	"github.com/dapperlabs/flow-go/language/runtime/common"
)

// NOTE: only called if the member expression is *not* an assignment
//
func (checker *Checker) VisitMemberExpression(expression *ast.MemberExpression) ast.Repr {
	member, isOptional := checker.visitMember(expression)

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

	if isOptional {
		return &OptionalType{Type: member.Type}
	}
	return member.Type
}

func (checker *Checker) visitMember(expression *ast.MemberExpression) (member *Member, isOptional bool) {
	memberInfo, ok := checker.Elaboration.MemberExpressionMemberInfos[expression]
	if ok {
		return memberInfo.Member, memberInfo.IsOptional
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

	getMemberForType := func(expressionType Type) {
		if ty, ok := expressionType.(MemberAccessibleType); ok && ty.HasMembers() {
			targetRange := ast.NewRangeFromPositioned(expression.Expression)
			member = ty.GetMember(identifier, targetRange, checker.report)
		}
	}

	// Get the member from the accessed value based
	// on the use of optional chaining syntax

	if expression.Optional {

		// If the member expression is using optional chaining,
		// check if the accessed type is optional

		if optionalExpressionType, ok := expressionType.(*OptionalType); ok {
			// The accessed type is optional, get the member from the wrapped type

			getMemberForType(optionalExpressionType.Type)
			isOptional = true
		} else {
			// Optional chaining was used on a non-optional type, report an error

			checker.report(
				&InvalidOptionalChainingError{
					Type:  expressionType,
					Range: ast.NewRangeFromPositioned(expression),
				},
			)

			// NOTE: still try to get member for non-optional expression
			// to avoid spurious error that member does not exist,
			// even if the non-optional accessed type has the member

			getMemberForType(expressionType)
		}
	} else {
		// The member is accessed directly without optional chaining.
		// Get the member directly from the accessed type

		getMemberForType(expressionType)
		isOptional = false
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

		if !checker.isReadableMember(member) {
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

	checker.Elaboration.MemberExpressionMemberInfos[expression] =
		MemberInfo{
			Member:     member,
			IsOptional: isOptional,
		}

	return member, isOptional
}

func (checker *Checker) isReadableMember(member *Member) bool {
	return checker.isReadableAccess(member.Access) ||
		checker.containerTypes[member.ContainerType]
}

func (checker *Checker) isWriteableMember(member *Member) bool {
	return checker.isWriteableAccess(member.Access) ||
		checker.containerTypes[member.ContainerType]
}
