package sema

import "github.com/dapperlabs/flow-go/pkg/language/runtime/ast"

func (checker *Checker) VisitIdentifierExpression(expression *ast.IdentifierExpression) ast.Repr {
	identifier := expression.Identifier
	variable := checker.findAndCheckVariable(identifier, true)
	if variable == nil {
		return &InvalidType{}
	}

	if variable.Type.IsResourceType() {
		resourceInfo := checker.resources.Get(variable)

		if resourceInfo.Invalidations.Size() > 0 {
			checker.report(
				&ResourceUseAfterInvalidationError{
					Name:          expression.Identifier.Identifier,
					Pos:           expression.Identifier.Pos,
					Invalidations: resourceInfo.Invalidations.All(),
				},
			)
		}

		checker.resources.AddUse(variable, expression.Pos)
	}

	return variable.Type
}

func (checker *Checker) VisitExpressionStatement(statement *ast.ExpressionStatement) ast.Repr {
	result := statement.Expression.Accept(checker)

	if ty, ok := result.(Type); ok &&
		ty.IsResourceType() {

		checker.report(
			&ResourceLossError{
				StartPos: statement.Expression.StartPosition(),
				EndPos:   statement.Expression.EndPosition(),
			},
		)
	}

	return nil
}

func (checker *Checker) VisitBoolExpression(expression *ast.BoolExpression) ast.Repr {
	return &BoolType{}
}

func (checker *Checker) VisitNilExpression(expression *ast.NilExpression) ast.Repr {
	// TODO: verify
	return &OptionalType{
		Type: &NeverType{},
	}
}

func (checker *Checker) VisitIntExpression(expression *ast.IntExpression) ast.Repr {
	return &IntType{}
}

func (checker *Checker) VisitStringExpression(expression *ast.StringExpression) ast.Repr {
	return &StringType{}
}

func (checker *Checker) VisitIndexExpression(expression *ast.IndexExpression) ast.Repr {
	return checker.visitIndexingExpression(expression.Expression, expression.Index, false)
}

// visitIndexingExpression checks if the indexed expression is indexable,
// checks if the indexing expression can be used to index into the indexed expression,
// and returns the expected element type
//
func (checker *Checker) visitIndexingExpression(
	indexedExpression ast.Expression,
	indexingExpression ast.Expression,
	isAssignment bool,
) Type {

	indexedType := indexedExpression.Accept(checker).(Type)
	indexingType := indexingExpression.Accept(checker).(Type)

	// NOTE: check indexed type first for UX reasons

	// check indexed expression's type is indexable
	// by getting the expected element

	if IsInvalidType(indexedType) {
		return &InvalidType{}
	}

	elementType := IndexableElementType(indexedType, isAssignment)
	if elementType == nil {
		elementType = &InvalidType{}

		checker.report(
			&NotIndexableTypeError{
				Type:     indexedType,
				StartPos: indexedExpression.StartPosition(),
				EndPos:   indexedExpression.EndPosition(),
			},
		)

		return elementType
	}

	// check indexing expression's type can be used to index
	// into indexed expression's type

	if !IsInvalidType(indexingType) &&
		!IsIndexingType(indexingType, indexedType) {

		checker.report(
			&NotIndexingTypeError{
				Type:     indexingType,
				StartPos: indexingExpression.StartPosition(),
				EndPos:   indexingExpression.EndPosition(),
			},
		)
	}

	checker.checkNonIdentifierResourceLoss(indexedType, indexedExpression)

	return elementType
}
