package sema

import (
	"github.com/dapperlabs/flow-go/language/runtime/ast"
)

// checkEventParameters checks that the event initializer's parameters are valid,
// as determined by `isValidEventParameterType`.
//
func (checker *Checker) checkEventParameters(
	parameterList *ast.ParameterList,
	parameterTypeAnnotations []*TypeAnnotation,
) {

	for i, parameter := range parameterList.Parameters {
		parameterTypeAnnotation := parameterTypeAnnotations[i]

		parameterType := parameterTypeAnnotation.Type

		if !parameterType.IsInvalidType() &&
			!isValidEventParameterType(parameterType) {

			checker.report(
				&InvalidEventParameterTypeError{
					Type: parameterType,
					Range: ast.Range{
						StartPos: parameter.StartPos,
						EndPos:   parameter.TypeAnnotation.EndPosition(),
					},
				},
			)
		}
	}
}

// isValidEventParameterType returns true if the given type is a valid event parameter type.
//
// Events currently only support simple primitive Cadence types.
//
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
