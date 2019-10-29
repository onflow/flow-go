package sema

import "github.com/dapperlabs/flow-go/language/runtime/ast"

func (checker *Checker) visitConditions(conditions []*ast.Condition) {

	// flag the checker to be inside a condition.
	// this flag is used to detect illegal expressions,
	// see e.g. VisitFunctionExpression

	wasInCondition := checker.inCondition
	checker.inCondition = true
	defer func() {
		checker.inCondition = wasInCondition
	}()

	// check all conditions: check the expression
	// and ensure the result is boolean

	for _, condition := range conditions {
		condition.Accept(checker)
	}
}

func (checker *Checker) VisitCondition(condition *ast.Condition) ast.Repr {

	// check test expression is boolean

	testType := condition.Test.Accept(checker).(Type)

	if !testType.IsInvalidType() &&
		!IsSubType(testType, &BoolType{}) {

		checker.report(
			&TypeMismatchError{
				ExpectedType: &BoolType{},
				ActualType:   testType,
				Range:        ast.NewRangeFromPositioned(condition.Test),
			},
		)
	}

	// check message expression results in a string

	if condition.Message != nil {

		messageType := condition.Message.Accept(checker).(Type)

		if !messageType.IsInvalidType() &&
			!IsSubType(messageType, &StringType{}) {

			checker.report(
				&TypeMismatchError{
					ExpectedType: &StringType{},
					ActualType:   testType,
					Range:        ast.NewRangeFromPositioned(condition.Message),
				},
			)
		}
	}

	return nil
}
