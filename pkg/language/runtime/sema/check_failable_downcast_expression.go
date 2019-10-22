package sema

import "github.com/dapperlabs/flow-go/pkg/language/runtime/ast"

func (checker *Checker) VisitFailableDowncastExpression(expression *ast.FailableDowncastExpression) ast.Repr {

	leftHandExpression := expression.Expression
	leftHandType := leftHandExpression.Accept(checker).(Type)

	rightHandTypeAnnotation := checker.ConvertTypeAnnotation(expression.TypeAnnotation)
	checker.checkTypeAnnotation(rightHandTypeAnnotation, expression.TypeAnnotation.StartPos)

	rightHandType := rightHandTypeAnnotation.Type

	checker.Elaboration.FailableDowncastingTypes[expression] = rightHandType

	// TODO: non-Any types (interfaces, wrapped (e.g Any?, [Any], etc.)) are not supported for now

	if _, ok := leftHandType.(*AnyType); !ok {

		checker.report(
			&UnsupportedTypeError{
				Type: leftHandType,
				Range: ast.Range{
					StartPos: leftHandExpression.StartPosition(),
					EndPos:   leftHandExpression.EndPosition(),
				},
			},
		)
	}

	return &OptionalType{Type: rightHandType}
}
