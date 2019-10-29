package sema

import "github.com/dapperlabs/flow-go/language/runtime/ast"

// NOTE: only called if the member expression is *not* an assignment
//
func (checker *Checker) VisitMemberExpression(expression *ast.MemberExpression) ast.Repr {
	member := checker.visitMember(expression)

	if member == nil {
		return &InvalidType{}
	}

	selfFieldMember := checker.selfFieldAccessMember(expression)
	if selfFieldMember != nil {

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
			fieldInitialized := info.InitializedFieldMembers.Contains(selfFieldMember)

			field := info.FieldMembers[selfFieldMember]
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

	selfFieldMember := checker.selfFieldAccessMember(expression)
	if selfFieldMember != nil &&
		selfFieldMember.Type.IsResourceType() {

		// NOTE: Preventing the capturing of the resource field is already implicitly handled:
		// By definition, the resource field can only be nested in a resource,
		// so `self` is a resource, and the capture of it is checked separately

		checker.checkResourceUseAfterInvalidation(selfFieldMember, expression.Identifier)
		checker.resources.AddUse(selfFieldMember, expression.Identifier.Pos)
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
	}

	checker.Elaboration.MemberExpressionMembers[expression] = member

	return member
}
