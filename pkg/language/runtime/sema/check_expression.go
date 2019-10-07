package sema

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
)

func (checker *Checker) VisitIdentifierExpression(expression *ast.IdentifierExpression) ast.Repr {
	identifier := expression.Identifier
	variable := checker.findAndCheckVariable(identifier, true)
	if variable == nil {
		return &InvalidType{}
	}

	if variable.Type.IsResourceType() {
		checker.checkResourceVariableCapturingInFunction(variable, expression.Identifier)
		checker.checkResourceVariableUseAfterInvalidation(variable, expression.Identifier)
		checker.resources.AddUse(variable, expression.Pos)
	}

	return variable.Type
}

// checkResourceVariableCapturingInFunction checks if a resource variable is captured in a function
//
func (checker *Checker) checkResourceVariableCapturingInFunction(variable *Variable, useIdentifier ast.Identifier) {
	currentFunctionDepth := -1
	currentFunctionActivation := checker.functionActivations.Current()
	if currentFunctionActivation != nil {
		currentFunctionDepth = currentFunctionActivation.ValueActivationDepth
	}

	if currentFunctionDepth == -1 ||
		variable.Depth > currentFunctionDepth {

		return
	}

	checker.report(
		&ResourceCapturingError{
			Name: useIdentifier.Identifier,
			Pos:  useIdentifier.Pos,
		},
	)
}

// checkResourceVariableUseAfterInvalidation checks if a resource variable
// is used after it was previously invalidated (moved or destroyed)
//
func (checker *Checker) checkResourceVariableUseAfterInvalidation(variable *Variable, useIdentifier ast.Identifier) {
	resourceInfo := checker.resources.Get(variable)
	if resourceInfo.Invalidations.Size() == 0 {
		return
	}

	checker.report(
		&ResourceUseAfterInvalidationError{
			Name:          useIdentifier.Identifier,
			Pos:           useIdentifier.Pos,
			Invalidations: resourceInfo.Invalidations.All(),
		},
	)
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
	return checker.visitIndexingExpression(expression, false)
}

// visitIndexingExpression checks if the indexed expression is indexable,
// checks if the indexing expression can be used to index into the indexed expression,
// and returns the expected element type
//
func (checker *Checker) visitIndexingExpression(
	indexExpression *ast.IndexExpression,
	isAssignment bool,
) Type {

	targetExpression := indexExpression.TargetExpression
	indexedType := targetExpression.Accept(checker).(Type)

	// NOTE: check indexed type first for UX reasons

	// check indexed expression's type is indexable
	// by getting the expected element

	if IsInvalidType(indexedType) {
		return &InvalidType{}
	}

	_, isStorage := indexedType.(*StorageType)
	if isStorage {

		indexingType := indexExpression.IndexingType

		// indexing into storage using expression?
		if indexExpression.IndexingExpression != nil {

			// Identifier expressions are valid, as the parser can't differentiate
			// between identifier expressions and nominal types.

			identifierExpression, isIdentifier := indexExpression.IndexingExpression.(*ast.IdentifierExpression)

			if isIdentifier {
				indexingType = &ast.NominalType{
					Identifier: identifierExpression.Identifier,
				}
			} else {
				checker.report(
					&InvalidStorageIndexingError{
						StartPos: indexExpression.IndexingExpression.StartPosition(),
						EndPos:   indexExpression.IndexingExpression.EndPosition(),
					},
				)

				return &InvalidType{}
			}
		}

		return checker.visitStorageIndexingExpression(
			targetExpression,
			indexingType,
			isAssignment,
		)
	} else {
		// indexing into non-storage value using type?
		if indexExpression.IndexingType != nil {
			checker.report(
				&InvalidIndexingError{
					StartPos: indexExpression.IndexingType.StartPosition(),
					EndPos:   indexExpression.IndexingType.EndPosition(),
				},
			)

			return &InvalidType{}
		}

		return checker.visitNormalIndexingExpression(
			targetExpression,
			indexedType,
			indexExpression.IndexingExpression,
			isAssignment,
		)
	}
}

func (checker *Checker) visitNormalIndexingExpression(
	indexedExpression ast.Expression,
	indexedType Type,
	indexingExpression ast.Expression,
	isAssignment bool,
) Type {
	indexingType := indexingExpression.Accept(checker).(Type)

	indexableType, isIndexableType := indexedType.(IndexableType)
	if !isIndexableType {
		checker.report(
			&NotIndexableTypeError{
				Type:     indexedType,
				StartPos: indexedExpression.StartPosition(),
				EndPos:   indexedExpression.EndPosition(),
			},
		)

		return &InvalidType{}
	}

	elementType := indexableType.ElementType(isAssignment)

	// check indexing expression's type can be used to index
	// into indexed expression's type

	if !IsInvalidType(indexingType) &&
		!IsSubType(indexingType, indexableType.IndexingType()) {

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

func (checker *Checker) visitStorageIndexingExpression(
	indexedExpression ast.Expression,
	indexingType ast.Type,
	isAssignment bool,
) Type {

	keyType := checker.ConvertType(indexingType)
	if IsInvalidType(keyType) {
		return &InvalidType{}
	}

	if isAssignment {
		return keyType
	} else {
		return &OptionalType{Type: keyType}
	}
}
