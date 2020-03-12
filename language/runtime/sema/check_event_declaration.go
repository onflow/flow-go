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
// Events currently only support a few simple Cadence types.
//
func isValidEventParameterType(t Type) bool {
	switch t := t.(type) {
	case *BoolType, *StringType, *CharacterType, *AddressType:
		return true

	case *OptionalType:
		return isValidEventParameterType(t.Type)

	case *VariableSizedType:
		return isValidEventParameterType(t.ElementType(false))

	case *ConstantSizedType:
		return isValidEventParameterType(t.ElementType(false))

	case *DictionaryType:
		return isValidEventParameterType(t.KeyType) &&
			isValidEventParameterType(t.ValueType)

	default:
		return IsSubType(t, &NumberType{})
	}
}
