package sema

import "github.com/dapperlabs/flow-go/pkg/language/runtime/ast"

func (checker *Checker) VisitArrayExpression(expression *ast.ArrayExpression) ast.Repr {

	// visit all elements, ensure they are all the same type

	var elementType Type

	for _, value := range expression.Values {
		valueType := value.Accept(checker).(Type)

		checker.checkResourceMoveOperation(value, valueType)

		// infer element type from first element
		// TODO: find common super type?
		if elementType == nil {
			elementType = valueType
		} else if !IsSubType(valueType, elementType) {
			checker.report(
				&TypeMismatchError{
					ExpectedType: elementType,
					ActualType:   valueType,
					StartPos:     value.StartPosition(),
					EndPos:       value.EndPosition(),
				},
			)
		}
	}

	if elementType == nil {
		elementType = &NeverType{}
	}

	return &VariableSizedType{
		Type: elementType,
	}
}
