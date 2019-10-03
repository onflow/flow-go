package sema

import "github.com/dapperlabs/flow-go/pkg/language/runtime/ast"

func (checker *Checker) VisitReturnStatement(statement *ast.ReturnStatement) ast.Repr {
	defer checker.checkResourceLossForFunction()

	// check value type matches enclosing function's return type

	if statement.Expression == nil {
		return nil
	}

	valueType := statement.Expression.Accept(checker).(Type)
	valueIsInvalid := IsInvalidType(valueType)

	returnType := checker.functionActivations.Current().ReturnType

	checker.Elaboration.ReturnStatementValueTypes[statement] = valueType
	checker.Elaboration.ReturnStatementReturnTypes[statement] = returnType

	if valueType == nil {
		return nil
	} else if valueIsInvalid {
		// return statement has expression, but function has Void return type?
		if _, ok := returnType.(*VoidType); ok {
			checker.report(
				&InvalidReturnValueError{
					StartPos: statement.Expression.StartPosition(),
					EndPos:   statement.Expression.EndPosition(),
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
					StartPos:     statement.Expression.StartPosition(),
					EndPos:       statement.Expression.EndPosition(),
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
