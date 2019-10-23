package sema

import (
	"github.com/dapperlabs/flow-go/language/runtime/ast"
	"github.com/dapperlabs/flow-go/language/runtime/common"
)

func (checker *Checker) VisitIdentifierExpression(expression *ast.IdentifierExpression) ast.Repr {
	identifier := expression.Identifier
	variable := checker.findAndCheckVariable(identifier, true)
	if variable == nil {
		return &InvalidType{}
	}

	if variable.Type.IsResourceType() {
		checker.checkResourceVariableCapturingInFunction(variable, expression.Identifier)
		checker.checkResourceUseAfterInvalidation(variable, expression.Identifier)
		checker.resources.AddUse(variable, expression.Pos)
	}

	checker.checkSelfVariableUseInInitializer(variable, expression.Pos)

	return variable.Type
}

// checkSelfVariableUseInInitializer checks uses of `self` in the initializer
// and ensures it is properly initialized
//
func (checker *Checker) checkSelfVariableUseInInitializer(variable *Variable, position ast.Position) {

	// Is this a use of `self`?

	if variable.DeclarationKind != common.DeclarationKindSelf {
		return
	}

	// Is this use of `self` in an initializer?

	initializationInfo := checker.functionActivations.Current().InitializationInfo
	if initializationInfo == nil {
		return
	}

	// The use of `self` is inside the initializer

	checkInitializationComplete := func() {
		if initializationInfo.InitializationComplete() {
			return
		}

		checker.report(
			&UninitializedUseError{
				Name: variable.Identifier,
				Pos:  position,
			},
		)
	}

	if checker.currentMemberExpression != nil {

		// The use of `self` is inside a member access

		// If the member expression refers to a field that must be initialized,
		// it must be initialized. This check is handled in `VisitMemberExpression`

		// Otherwise, the member access is to a non-field, e.g. a function,
		// in which case *all* fields must have been initialized

		selfFieldMember := checker.selfFieldAccessMember(checker.currentMemberExpression)
		field := initializationInfo.FieldMembers[selfFieldMember]

		if field == nil {
			checkInitializationComplete()
		}

	} else {
		// The use of `self` is *not* inside a member access, i.e. `self` is used
		// as a standalone expression, e.g. to pass it as an argument to a function.
		// Ensure that *all* fields were initialized

		checkInitializationComplete()
	}
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

func (checker *Checker) VisitExpressionStatement(statement *ast.ExpressionStatement) ast.Repr {
	result := statement.Expression.Accept(checker)

	if ty, ok := result.(Type); ok &&
		ty.IsResourceType() {

		checker.report(
			&ResourceLossError{
				Range: ast.NewRangeFromPositioned(statement.Expression),
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
	elementType, _ := checker.visitIndexExpression(expression, false)
	return elementType
}

// visitIndexExpression checks if the indexed expression is indexable,
// checks if the indexing expression can be used to index into the indexed expression,
// and returns the expected element type
//
func (checker *Checker) visitIndexExpression(
	indexExpression *ast.IndexExpression,
	isAssignment bool,
) (elementType Type, targetType Type) {

	targetExpression := indexExpression.TargetExpression
	targetType = targetExpression.Accept(checker).(Type)

	// NOTE: check indexed type first for UX reasons

	// check indexed expression's type is indexable
	// by getting the expected element

	if targetType.IsInvalidType() {
		elementType = &InvalidType{}
		return
	}

	defer func() {
		checker.checkAccessResourceLoss(elementType, targetExpression)
	}()

	reportNotIndexableType := func() {
		checker.report(
			&NotIndexableTypeError{
				Type:  targetType,
				Range: ast.NewRangeFromPositioned(targetExpression),
			},
		)

		// set the return value properly
		elementType = &InvalidType{}
	}

	switch indexedType := targetType.(type) {
	case TypeIndexableType:

		indexingType := indexExpression.IndexingType

		// indexing into type-indexable using expression?
		if indexExpression.IndexingExpression != nil {

			// The parser may have parsed a type as an expression,
			// because some type forms are also valid expression forms,
			// and the parser can't disambiguate them.
			// Attempt to convert the expression to a type

			indexingType = ast.ExpressionAsType(indexExpression.IndexingExpression)
			if indexingType == nil {
				checker.report(
					&InvalidTypeIndexingError{
						Range: ast.NewRangeFromPositioned(indexExpression.IndexingExpression),
					},
				)

				elementType = &InvalidType{}
				return
			}
		}

		elementType = checker.visitTypeIndexingExpression(
			indexedType,
			indexExpression,
			indexingType,
			isAssignment,
		)
		return

	case ValueIndexableType:

		// Check if the type instance is actually indexable. For most types (e.g. arrays and dictionaries)
		// this is known statically (in the sense of this host language (Go), not the implemented language),
		// i.e. a Go type switch would be sufficient.
		// However, for some types (e.g. reference types) this depends on what type is referenced

		if !indexedType.isValueIndexableType() {
			reportNotIndexableType()
			return
		}

		// indexing into value-indexable value using type?
		if indexExpression.IndexingType != nil {
			checker.report(
				&InvalidIndexingError{
					Range: ast.NewRangeFromPositioned(indexExpression.IndexingType),
				},
			)

			elementType = &InvalidType{}
			return
		}

		elementType = checker.visitValueIndexingExpression(
			targetExpression,
			indexedType,
			indexExpression.IndexingExpression,
			isAssignment,
		)
		return

	default:
		reportNotIndexableType()
		return
	}
}

func (checker *Checker) visitValueIndexingExpression(
	indexedExpression ast.Expression,
	indexedType ValueIndexableType,
	indexingExpression ast.Expression,
	isAssignment bool,
) Type {
	indexingType := indexingExpression.Accept(checker).(Type)

	elementType := indexedType.ElementType(isAssignment)

	// check indexing expression's type can be used to index
	// into indexed expression's type

	if !indexingType.IsInvalidType() &&
		!IsSubType(indexingType, indexedType.IndexingType()) {

		checker.report(
			&NotIndexingTypeError{
				Type:  indexingType,
				Range: ast.NewRangeFromPositioned(indexingExpression),
			},
		)
	}

	return elementType
}

func (checker *Checker) visitTypeIndexingExpression(
	indexedType TypeIndexableType,
	indexExpression *ast.IndexExpression,
	indexingType ast.Type,
	isAssignment bool,
) Type {

	keyType := checker.ConvertType(indexingType)
	if keyType.IsInvalidType() {
		return &InvalidType{}
	}

	checker.Elaboration.IndexExpressionIndexingTypes[indexExpression] = keyType

	return indexedType.ElementType(keyType, isAssignment)
}
