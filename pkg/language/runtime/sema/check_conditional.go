package sema

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/errors"
)

func (checker *Checker) VisitIfStatement(statement *ast.IfStatement) ast.Repr {

	thenElement := statement.Then

	var elseElement ast.Element = ast.NotAnElement{}
	if statement.Else != nil {
		elseElement = statement.Else
	}

	switch test := statement.Test.(type) {
	case ast.Expression:
		checker.visitConditional(test, thenElement, elseElement)

	case *ast.VariableDeclaration:
		checker.checkConditionalBranches(
			func() Type {
				checker.withValueScope(func() {
					checker.visitVariableDeclaration(test, true)
					thenElement.Accept(checker)
				})
				return nil
			},
			func() Type {
				elseElement.Accept(checker)
				return nil
			},
		)
	default:
		panic(&errors.UnreachableError{})
	}

	return nil
}

func (checker *Checker) VisitConditionalExpression(expression *ast.ConditionalExpression) ast.Repr {

	thenType, elseType := checker.visitConditional(expression.Test, expression.Then, expression.Else)

	if thenType == nil || elseType == nil {
		panic(&errors.UnreachableError{})
	}

	// TODO: improve
	resultType := thenType

	if !IsSubType(elseType, resultType) {
		checker.report(
			&TypeMismatchError{
				ExpectedType: resultType,
				ActualType:   elseType,
				Range:        ast.NewRangeFromPositioned(expression.Else),
			},
		)
	}

	return resultType
}

// visitConditional checks a conditional.
// The test expression must be a boolean.
// The "then" and "else" elements may be expressions, in which case their types are returned.
//
func (checker *Checker) visitConditional(
	test ast.Expression,
	thenElement ast.Element,
	elseElement ast.Element,
) (
	thenType, elseType Type,
) {
	testType := test.Accept(checker).(Type)

	if !IsSubType(testType, &BoolType{}) {
		checker.report(
			&TypeMismatchError{
				ExpectedType: &BoolType{},
				ActualType:   testType,
				Range:        ast.NewRangeFromPositioned(test),
			},
		)
	}

	return checker.checkConditionalBranches(
		func() Type {
			thenResult := thenElement.Accept(checker)
			if thenResult == nil {
				return nil
			}
			return thenResult.(Type)
		},
		func() Type {
			elseResult := elseElement.Accept(checker)
			if elseResult == nil {
				return nil
			}
			return elseResult.(Type)
		},
	)
}

// checkConditionalBranches checks two conditional branches.
// It is assumed that either one of the branches is taken, so function returns,
// resource uses and invalidations, as well as field initializations,
// are only potential in each branch, but definite if they occur in both branches.
//
func (checker *Checker) checkConditionalBranches(
	checkThen TypeCheckFunc,
	checkElse TypeCheckFunc,
) (
	thenType, elseType Type,
) {
	functionActivation := checker.functionActivations.Current()

	initialReturnInfo := functionActivation.ReturnInfo
	thenReturnInfo := initialReturnInfo.Clone()
	elseReturnInfo := initialReturnInfo.Clone()

	var thenInitializedMembers *MemberSet
	var elseInitializedMembers *MemberSet
	if functionActivation.InitializationInfo != nil {
		initialInitializedMembers := functionActivation.InitializationInfo.InitializedFieldMembers
		thenInitializedMembers = initialInitializedMembers.Clone()
		elseInitializedMembers = initialInitializedMembers.Clone()
	}

	initialResources := checker.resources
	thenResources := initialResources.Clone()
	elseResources := initialResources.Clone()

	thenType = checker.checkBranch(
		checkThen,
		thenReturnInfo,
		thenInitializedMembers,
		thenResources,
	)

	elseType = checker.checkBranch(
		checkElse,
		elseReturnInfo,
		elseInitializedMembers,
		elseResources,
	)

	functionActivation.ReturnInfo.MergeBranches(thenReturnInfo, elseReturnInfo)

	if functionActivation.InitializationInfo != nil {
		functionActivation.InitializationInfo.InitializedFieldMembers =
			thenInitializedMembers.Intersection(elseInitializedMembers)
	}

	checker.resources.MergeBranches(thenResources, elseResources)

	return
}

// checkBranch checks a conditional branch.
// It is assumed that function returns, resource uses and invalidations,
// as well as field initializations, are only potential / temporary.
//
func (checker *Checker) checkBranch(
	check TypeCheckFunc,
	temporaryReturnInfo *ReturnInfo,
	temporaryInitializedMembers *MemberSet,
	temporaryResources *Resources,
) Type {
	return wrapTypeCheck(check,
		func(f TypeCheckFunc) Type {
			return checker.checkWithResources(f, temporaryResources)
		},
		func(f TypeCheckFunc) Type {
			return checker.checkWithInitializedMembers(f, temporaryInitializedMembers)
		},
		func(f TypeCheckFunc) Type {
			return checker.checkWithReturnInfo(f, temporaryReturnInfo)
		},
	)()
}
