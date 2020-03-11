package sema

import (
	"github.com/dapperlabs/flow-go/language/runtime/ast"
)

// checkEventParameters checks that the event initializer's parameters are valid,
// as determined by `isValidEventParameterType`.
//
func (checker *Checker) checkEventParameters(
	parameterList *ast.ParameterList,
	parameters []*Parameter,
) {

	for i, parameter := range parameterList.Parameters {
		parameterType := parameters[i].TypeAnnotation.Type

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
	switch t := t.(type) {
	case *BoolType, *StringType, *AddressType:
		return true

	case *VariableSizedType:
		return isValidEventParameterType(t.ElementType(false))

	case *ConstantSizedType:
		return isValidEventParameterType(t.ElementType(false))

	default:
		return IsSubType(t, &NumberType{})
	}
}
