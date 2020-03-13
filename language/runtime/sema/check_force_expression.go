package sema

import (
	"github.com/dapperlabs/flow-go/language/runtime/ast"
)

func (checker *Checker) VisitForceExpression(expression *ast.ForceExpression) ast.Repr {

	valueType := expression.Expression.Accept(checker).(Type)

	if valueType.IsInvalidType() {
		return valueType
	}

	optionalType, ok := valueType.(*OptionalType)
	if !ok {
		checker.report(
			&NonOptionalForceError{
				Type:  valueType,
				Range: ast.NewRangeFromPositioned(expression.Expression),
			},
		)

		return valueType
	}

	return optionalType.Type
}
