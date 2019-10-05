package sema

import "github.com/dapperlabs/flow-go/pkg/language/runtime/ast"

func (checker *Checker) VisitMemberExpression(expression *ast.MemberExpression) ast.Repr {
	member := checker.visitMember(expression)

	var memberType Type = &InvalidType{}
	if member != nil {
		memberType = member.Type
	}

	return memberType
}

func (checker *Checker) visitMember(expression *ast.MemberExpression) *Member {
	member, ok := checker.Elaboration.MemberExpressionMembers[expression]
	if ok {
		return member
	}

	accessedExpression := expression.Expression

	expressionType := accessedExpression.Accept(checker).(Type)

	checker.checkNonIdentifierResourceLoss(expressionType, accessedExpression)

	origins := checker.memberOrigins[expressionType]

	identifier := expression.Identifier.Identifier
	identifierStartPosition := expression.Identifier.StartPosition()
	identifierEndPosition := expression.Identifier.EndPosition()

	if ty, ok := expressionType.(HasMembers); ok {
		member = ty.GetMember(identifier)
	}

	if _, isArrayType := expressionType.(ArrayType); isArrayType && member != nil {
		// TODO: implement Equatable interface: https://github.com/dapperlabs/bamboo-node/issues/78
		if identifier == "contains" {
			functionType := member.Type.(*FunctionType)

			if !IsEquatableType(functionType.ParameterTypeAnnotations[0].Type) {
				checker.report(
					&NotEquatableTypeError{
						Type:     expressionType,
						StartPos: identifierStartPosition,
						EndPos:   identifierEndPosition,
					},
				)

				return nil
			}
		}
	}

	if member == nil {
		if !IsInvalidType(expressionType) {
			checker.report(
				&NotDeclaredMemberError{
					Type:     expressionType,
					Name:     identifier,
					StartPos: identifierStartPosition,
					EndPos:   identifierEndPosition,
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
