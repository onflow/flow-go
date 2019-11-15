package sema

import (
	"github.com/dapperlabs/flow-go/language/runtime/ast"
	"github.com/dapperlabs/flow-go/language/runtime/errors"
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
		declarationType = typeAnnotation.Type

		checker.checkTypeAnnotation(typeAnnotation, declaration.TypeAnnotation.StartPos)

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

			} else {
				if !checker.IsTypeCompatible(declaration.Value, valueType, declarationType) {
					checker.report(
						&TypeMismatchError{
							ExpectedType: declarationType,
							ActualType:   valueType,
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

			// Check the assignment of the second value to the first expression

			// The check of the assignment of the second value to the first also:
			// - Invalidates the second resource
			// - Checks the second transfer
			// - Checks the second value type is a subtype of value type
			// etc.

			_, secondValueType := checker.checkAssignment(
				declaration.Value,
				declaration.SecondValue,
				declaration.SecondTransfer,
				true,
			)

			// Check that the second value type is a resource type

			if !secondValueType.IsInvalidType() &&
				!secondValueType.IsResourceType() {

				checker.report(
					&NonResourceTypeError{
						ActualType: secondValueType,
						Range:      ast.NewRangeFromPositioned(declaration.SecondValue),
					},
				)
			}

			checker.Elaboration.VariableDeclarationSecondValueTypes[declaration] = secondValueType
		}
	}

	// Finally, declare the variable in the current value activation

	variable, err := checker.valueActivations.Declare(
		declaration.Identifier.Identifier,
		declarationType,
		declaration.DeclarationKind(),
		declaration.Identifier.Pos,
		declaration.IsConstant,
		nil,
	)
	checker.report(err)
	checker.recordVariableDeclarationOccurrence(declaration.Identifier.Identifier, variable)
}
