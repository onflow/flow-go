package sema

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
)

func (checker *Checker) VisitEventDeclaration(declaration *ast.EventDeclaration) ast.Repr {
	eventType := checker.Elaboration.EventDeclarationTypes[declaration]

	// check argument labels
	checker.checkArgumentLabels(declaration.Parameters)

	// check parameters
	for i, parameter := range declaration.Parameters {
		parameterTypeAnnotation := eventType.ParameterTypeAnnotations[i]
		checker.checkTypeAnnotation(parameterTypeAnnotation, parameter.TypeAnnotation.StartPos)

		// only allow primitive parameters
		if !isValidEventParameterType(parameterTypeAnnotation.Type) {
			checker.report(&InvalidEventParameterTypeError{
				Type:     parameterTypeAnnotation.Type,
				StartPos: parameter.StartPos,
				EndPos:   parameter.TypeAnnotation.EndPosition(),
			})
		}
	}

	return nil
}

func (checker *Checker) declareEventDeclaration(declaration *ast.EventDeclaration) {
	identifier := declaration.Identifier

	convertedParameterTypeAnnotations := checker.parameterTypeAnnotations(declaration.Parameters)

	eventType := &EventType{
		Identifier:               identifier.Identifier,
		ParameterTypeAnnotations: convertedParameterTypeAnnotations,
		ArgumentLabels:           declaration.Parameters.ArgumentLabels(),
	}

	typeDeclaredErr := checker.typeActivations.Declare(identifier, eventType)
	checker.report(typeDeclaredErr)

	_, valueDeclaredErr := checker.valueActivations.DeclareFunction(
		identifier,
		&FunctionType{
			ParameterTypeAnnotations: eventType.ParameterTypeAnnotations,
			ReturnTypeAnnotation:     NewTypeAnnotation(eventType),
		},
		declaration.Parameters.ArgumentLabels(),
	)

	if typeDeclaredErr == nil {
		checker.report(valueDeclaredErr)
	}

	checker.recordVariableDeclarationOccurrence(
		identifier.Identifier,
		&Variable{
			Identifier: identifier.Identifier,
			Kind:       declaration.DeclarationKind(),
			IsConstant: true,
			Type:       eventType,
			Pos:        &identifier.Pos,
		},
	)

	checker.Elaboration.EventDeclarationTypes[declaration] = eventType
}

// isValidEventParameterType returns true if the given type is a valid event parameters.
//
// Events currently only support simple primitive Cadence types.
func isValidEventParameterType(t Type) bool {
	if IsSubType(t, &BoolType{}) {
		return true
	}

	if IsSubType(t, &StringType{}) {
		return true
	}

	if IsSubType(t, &IntegerType{}) {
		return true
	}

	switch arrayType := t.(type) {
	case *VariableSizedType:
		return isValidEventParameterType(arrayType.ElementType(false))
	case *ConstantSizedType:
		return isValidEventParameterType(arrayType.ElementType(false))
	default:
		return false
	}
}
