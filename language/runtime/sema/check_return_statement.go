package sema

import "github.com/dapperlabs/flow-go/language/runtime/ast"

func (checker *Checker) VisitReturnStatement(statement *ast.ReturnStatement) ast.Repr {
	functionActivation := checker.functionActivations.Current()

	defer func() {
		checker.checkResourceLossForFunction()
		checker.resources.Returns = true
		functionActivation.ReturnInfo.MaybeReturned = true
		functionActivation.ReturnInfo.DefinitelyReturned = true
	}()

	// check value type matches enclosing function's return type

	if statement.Expression == nil {
		return nil
	}

	valueType := statement.Expression.Accept(checker).(Type)
	returnType := functionActivation.ReturnType

	checker.Elaboration.ReturnStatementValueTypes[statement] = valueType
	checker.Elaboration.ReturnStatementReturnTypes[statement] = returnType

	if valueType == nil {
		return nil
	}

	// return statement has expression, but function has Void return type?
	if _, ok := returnType.(*VoidType); ok {
		checker.report(
			&InvalidReturnValueError{
				Range: ast.NewRangeFromPositioned(statement.Expression),
			},
		)
	} else {

		if !valueType.IsInvalidType() &&
			!returnType.IsInvalidType() &&
			!checker.checkTypeCompatibility(statement.Expression, valueType, returnType) {

			checker.report(
				&TypeMismatchError{
					ExpectedType: returnType,
					ActualType:   valueType,
					Range:        ast.NewRangeFromPositioned(statement.Expression),
				},
			)
		}

		checker.checkVariableMove(statement.Expression)
		checker.checkResourceMoveOperation(statement.Expression, valueType)
	}

	return nil
}

func (checker *Checker) checkResourceLossForFunction() {
	functionValueActivationDepth :=
		checker.functionActivations.Current().ValueActivationDepth
	checker.checkResourceLoss(functionValueActivationDepth)
}
