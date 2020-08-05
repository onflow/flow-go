package sema

import (
	"github.com/onflow/cadence-initial/runtime/ast"
	"github.com/onflow/cadence-initial/runtime/errors"
)

func (checker *Checker) VisitVariableDeclaration(declaration *ast.VariableDeclaration) ast.Repr {
	checker.visitVariableDeclaration(declaration, false)
	return nil
}

func (checker *Checker) visitVariableDeclaration(declaration *ast.VariableDeclaration, isOptionalBinding bool) {

	checker.checkDeclarationAccessModifier(
		declaration.Access,
		declaration.DeclarationKind(),
		declaration.StartPos,
		declaration.IsConstant,
	)

	// Determine the type of the initial value of the variable declaration
	// and save it in the elaboration

	valueType := declaration.Value.Accept(checker).(Type)

	checker.Elaboration.VariableDeclarationValueTypes[declaration] = valueType

	valueIsInvalid := valueType.IsInvalidType()

	// If the variable declaration is an optional binding, the value must be optional

	var valueIsOptional bool
	var optionalValueType *OptionalType

	if isOptionalBinding && !valueIsInvalid {
		optionalValueType, valueIsOptional = valueType.(*OptionalType)
		if !valueIsOptional {
			checker.report(
				&TypeMismatchError{
					ExpectedType: &OptionalType{},
					ActualType:   valueType,
					Range:        ast.NewRangeFromPositioned(declaration.Value),
				},
			)
		}
	}

	// Determine the declaration type based on the value type and the optional type annotation

	declarationType := valueType

	// If the declaration has an explicit type annotation, take it into account:
	// Check it and ensure the value type is *compatible* with the type annotation

	if declaration.TypeAnnotation != nil {

		typeAnnotation := checker.ConvertTypeAnnotation(declaration.TypeAnnotation)
		checker.checkTypeAnnotation(typeAnnotation, declaration.TypeAnnotation)

		declarationType = typeAnnotation.Type

		// check the value type is a subtype of the declaration type
		if declarationType != nil && valueType != nil && !valueIsInvalid && !declarationType.IsInvalidType() {

			if isOptionalBinding {
				if optionalValueType != nil &&
					(optionalValueType.Equal(declarationType) ||
						!IsSubType(optionalValueType.Type, declarationType)) {

					checker.report(
						&TypeMismatchError{
							ExpectedType: declarationType,
							ActualType:   optionalValueType.Type,
							Range:        ast.NewRangeFromPositioned(declaration.Value),
						},
					)
				}

			}
		}
	} else if isOptionalBinding && optionalValueType != nil {
		declarationType = optionalValueType.Type
	}

	checker.Elaboration.VariableDeclarationTargetTypes[declaration] = declarationType

	if declarationType != nil &&
		valueType != nil &&
		!valueIsInvalid &&
		!declarationType.IsInvalidType() &&
		!isOptionalBinding &&
		!checker.checkTypeCompatibility(declaration.Value, valueType, declarationType) {

		checker.report(
			&TypeMismatchError{
				ExpectedType: declarationType,
				ActualType:   valueType,
				Range:        ast.NewRangeFromPositioned(declaration.Value),
			},
		)
	}

	checker.checkTransfer(declaration.Transfer, declarationType)

	// The variable declaration might have a second transfer and second expression.
	//
	// In that case the declaration transfers not only the value of the first expression
	// to the identifier (new variable), but the declaration also transfers the value
	// of the second expression to the first expression (which must be a target expression).
	//
	// This is only valid for resources, i.e. the declaration type, first value type,
	// and the second value type must be resource types, and all transfers must be moves.

	if declaration.SecondTransfer == nil {
		if declaration.SecondValue != nil {
			panic(errors.NewUnreachableError())
		}

		checker.checkVariableMove(declaration.Value)

		// If only one value expression is provided, it is invalidated (if it has a resource type)

		checker.recordResourceInvalidation(
			declaration.Value,
			declarationType,
			ResourceInvalidationKindMove,
		)
	} else {

		// The first expression must be a target expression (e.g. identifier expression,
		// indexing expression, or member access expression)

		if _, firstIsTarget := declaration.Value.(ast.TargetExpression); !firstIsTarget {
			checker.report(
				&InvalidAssignmentTargetError{
					Range: ast.NewRangeFromPositioned(declaration.Value),
				},
			)
		} else {
			// The assignment is valid (i.e. to a target expression)

			// NOTE: Check that the *first* value type is a resource type –
			// The assignment check below will also ensure that the second value type
			// is a resource.
			//
			// The first value is checked instead of the second value,
			// so that second value types that are standalone not considered resource typed
			// are still admitted if they are type compatible (e.g. `nil`).

			valueIsResource := valueType != nil && valueType.IsResourceType()

			if valueType != nil &&
				!valueType.IsInvalidType() &&
				!valueIsResource {

				checker.report(
					&NonResourceTypeError{
						ActualType: valueType,
						Range:      ast.NewRangeFromPositioned(declaration.Value),
					},
				)
			}

			// Check the assignment of the second value to the first expression

			// The check of the assignment of the second value to the first also:
			// - Invalidates the second resource
			// - Checks the second transfer
			// - Checks the second value type is a subtype of value type
			// etc.

			// NOTE: already performs resource invalidation

			_, secondValueType := checker.checkAssignment(
				declaration.Value,
				declaration.SecondValue,
				declaration.SecondTransfer,
				true,
			)

			checker.Elaboration.VariableDeclarationSecondValueTypes[declaration] = secondValueType

			if valueIsResource {
				checker.elaboratePotentialResourceStorageMove(declaration.Value)
			}
		}
	}

	// Finally, declare the variable in the current value activation

	identifier := declaration.Identifier.Identifier

	variable, err := checker.valueActivations.Declare(variableDeclaration{
		identifier:               identifier,
		ty:                       declarationType,
		access:                   declaration.Access,
		kind:                     declaration.DeclarationKind(),
		pos:                      declaration.Identifier.Pos,
		isConstant:               declaration.IsConstant,
		argumentLabels:           nil,
		allowOuterScopeShadowing: true,
	})
	checker.report(err)
	checker.recordVariableDeclarationOccurrence(identifier, variable)
}

func (checker *Checker) elaboratePotentialResourceStorageMove(expression ast.Expression) {
	indexExpression, ok := expression.(*ast.IndexExpression)
	if !ok {
		return
	}

	if !checker.Elaboration.IsTypeIndexExpression[indexExpression] {
		return
	}

	checker.Elaboration.IsResourceMovingStorageIndexExpression[indexExpression] = true
}
