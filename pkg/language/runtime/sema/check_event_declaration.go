package sema

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
)

func (checker *Checker) VisitEventDeclaration(declaration *ast.EventDeclaration) ast.Repr {
	eventType := checker.Elaboration.EventDeclarationTypes[declaration]

	constructorFunctionType := eventType.ConstructorFunctionType()

	checker.checkFunction(
		declaration.ParameterList,
		ast.Position{},
		constructorFunctionType,
		nil,
		false,
		nil,
	)

	// check that parameters are primitive types
	checker.checkEventParameters(declaration.ParameterList, constructorFunctionType.ParameterTypeAnnotations)

	return nil
}

func (checker *Checker) declareEventDeclaration(declaration *ast.EventDeclaration) {
	identifier := declaration.Identifier

	convertedParameterTypeAnnotations := checker.parameterTypeAnnotations(declaration.ParameterList)

	fields := make([]EventFieldType, len(declaration.ParameterList.Parameters))
	for i, parameter := range declaration.ParameterList.Parameters {
		parameterTypeAnnotation := convertedParameterTypeAnnotations[i]

		fields[i] = EventFieldType{
			Identifier: parameter.Identifier.Identifier,
			Type:       parameterTypeAnnotation.Type,
		}
	}

	eventType := &EventType{
		Identifier:                          identifier.Identifier,
		ImportLocation:                      checker.Location,
		Fields:                              fields,
		ConstructorParameterTypeAnnotations: convertedParameterTypeAnnotations,
	}

	typeDeclarationErr := checker.typeActivations.Declare(identifier, eventType)
	checker.report(typeDeclarationErr)

	constructorDeclarationErr := checker.declareEventConstructor(declaration, eventType)

	// only report declaration error for constructor if declaration error for type does not occur
	if typeDeclarationErr == nil {
		checker.report(constructorDeclarationErr)
	}

	checker.recordVariableDeclarationOccurrence(
		identifier.Identifier,
		&Variable{
			Identifier:      identifier.Identifier,
			DeclarationKind: declaration.DeclarationKind(),
			IsConstant:      true,
			Type:            eventType,
			Pos:             &identifier.Pos,
		},
	)

	checker.Elaboration.EventDeclarationTypes[declaration] = eventType
}

func (checker *Checker) declareEventConstructor(declaration *ast.EventDeclaration, eventType *EventType) error {
	_, err := checker.valueActivations.DeclareFunction(
		declaration.Identifier,
		eventType.ConstructorFunctionType(),
		declaration.ParameterList.ArgumentLabels(),
	)

	return err
}

func (checker *Checker) checkEventParameters(parameterList *ast.ParameterList, parameterTypeAnnotations []*TypeAnnotation) {
	for i, parameter := range parameterList.Parameters {
		parameterTypeAnnotation := parameterTypeAnnotations[i]

		// only allow primitive parameters
		if !isValidEventParameterType(parameterTypeAnnotation.Type) {
			checker.report(&InvalidEventParameterTypeError{
				Type: parameterTypeAnnotation.Type,
				Range: ast.Range{
					StartPos: parameter.StartPos,
					EndPos:   parameter.TypeAnnotation.EndPosition(),
				},
			})
		}
	}

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
