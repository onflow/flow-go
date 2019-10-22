package sema

import "github.com/dapperlabs/flow-go/pkg/language/runtime/ast"

func (checker *Checker) VisitVariableDeclaration(declaration *ast.VariableDeclaration) ast.Repr {
	checker.visitVariableDeclaration(declaration, false)
	return nil
}

func (checker *Checker) visitVariableDeclaration(declaration *ast.VariableDeclaration, isOptionalBinding bool) {
	valueType := declaration.Value.Accept(checker).(Type)

	checker.Elaboration.VariableDeclarationValueTypes[declaration] = valueType

	valueIsInvalid := IsInvalidType(valueType)

	// if the variable declaration is a optional binding, the value must be optional

	var valueIsOptional bool
	var optionalValueType *OptionalType

	if isOptionalBinding && !valueIsInvalid {
		optionalValueType, valueIsOptional = valueType.(*OptionalType)
		if !valueIsOptional {
			checker.report(
				&TypeMismatchError{
					ExpectedType: &OptionalType{},
					ActualType:   valueType,
					Range: ast.Range{
						StartPos: declaration.Value.StartPosition(),
						EndPos:   declaration.Value.EndPosition(),
					},
				},
			)
		}
	}

	declarationType := valueType

	// does the declaration have an explicit type annotation?
	if declaration.TypeAnnotation != nil {
		typeAnnotation := checker.ConvertTypeAnnotation(declaration.TypeAnnotation)
		declarationType = typeAnnotation.Type

		checker.checkTypeAnnotation(typeAnnotation, declaration.TypeAnnotation.StartPos)

		// check the value type is a subtype of the declaration type
		if declarationType != nil && valueType != nil && !valueIsInvalid && !IsInvalidType(declarationType) {

			if isOptionalBinding {
				if optionalValueType != nil &&
					(optionalValueType.Equal(declarationType) ||
						!IsSubType(optionalValueType.Type, declarationType)) {

					checker.report(
						&TypeMismatchError{
							ExpectedType: declarationType,
							ActualType:   optionalValueType.Type,
							Range: ast.Range{
								StartPos: declaration.Value.StartPosition(),
								EndPos:   declaration.Value.EndPosition(),
							},
						},
					)
				}

			} else {
				if !checker.IsTypeCompatible(declaration.Value, valueType, declarationType) {
					checker.report(
						&TypeMismatchError{
							ExpectedType: declarationType,
							ActualType:   valueType,
							Range: ast.Range{
								StartPos: declaration.Value.StartPosition(),
								EndPos:   declaration.Value.EndPosition(),
							},
						},
					)
				}
			}
		}
	} else if isOptionalBinding && optionalValueType != nil {
		declarationType = optionalValueType.Type
	}

	checker.checkTransfer(declaration.Transfer, declarationType)
	checker.recordResourceInvalidation(
		declaration.Value,
		declarationType,
		ResourceInvalidationKindMove,
	)

	checker.Elaboration.VariableDeclarationTargetTypes[declaration] = declarationType

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
