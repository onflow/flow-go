package sema

import "github.com/dapperlabs/flow-go/pkg/language/runtime/ast"

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
	valueIsInvalid := IsInvalidType(valueType)

	returnType := functionActivation.ReturnType

	checker.Elaboration.ReturnStatementValueTypes[statement] = valueType
	checker.Elaboration.ReturnStatementReturnTypes[statement] = returnType

	if valueType == nil {
		return nil
	} else if valueIsInvalid {
		// return statement has expression, but function has Void return type?
		if _, ok := returnType.(*VoidType); ok {
			checker.report(
				&InvalidReturnValueError{
					Range: ast.Range{
						StartPos: statement.Expression.StartPosition(),
						EndPos:   statement.Expression.EndPosition(),
					},
				},
			)
		}
	} else {

		if !IsInvalidType(returnType) &&
			!checker.IsTypeCompatible(statement.Expression, valueType, returnType) {

			checker.report(
				&TypeMismatchError{
					ExpectedType: returnType,
					ActualType:   valueType,
					Range: ast.Range{
						StartPos: statement.Expression.StartPosition(),
						EndPos:   statement.Expression.EndPosition(),
					},
				},
			)
		}

		checker.checkResourceMoveOperation(statement.Expression, valueType)
	}

	return nil
}

func (checker *Checker) checkResourceLossForFunction() {
	functionValueActivationDepth :=
		checker.functionActivations.Current().ValueActivationDepth
	checker.checkResourceLoss(functionValueActivationDepth)
}
