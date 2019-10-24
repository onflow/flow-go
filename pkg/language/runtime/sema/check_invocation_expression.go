package sema

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/common"
)

func (checker *Checker) VisitInvocationExpression(invocationExpression *ast.InvocationExpression) ast.Repr {
	typ := checker.checkInvocationExpression(invocationExpression)

	// events cannot be invoked without an emit statement
	if _, isEventType := typ.(*EventType); isEventType {
		checker.report(
			&InvalidEventUsageError{
				Range: ast.NewRangeFromPositioned(invocationExpression),
			},
		)
		return &InvalidType{}
	}

	return typ
}

func (checker *Checker) checkInvocationExpression(invocationExpression *ast.InvocationExpression) Type {
	inCreate := checker.inCreate
	checker.inCreate = false
	defer func() {
		checker.inCreate = inCreate
	}()

	// check the invoked expression can be invoked

	invokedExpression := invocationExpression.InvokedExpression
	expressionType := invokedExpression.Accept(checker).(Type)

	invokableType, ok := expressionType.(InvokableType)
	if !ok {
		if !expressionType.IsInvalidType() {
			checker.report(
				&NotCallableError{
					Type:  expressionType,
					Range: ast.NewRangeFromPositioned(invokedExpression),
				},
			)
		}
		return &InvalidType{}
	}

	// invoked expression has function type

	functionType := invokableType.InvocationFunctionType()

	var returnType Type = &InvalidType{}

	argumentTypes := checker.checkInvocationArguments(invocationExpression, functionType)

	// If the invocation refers directly to the name of the function as stated in the declaration,
	// or the invocation refers to a function of a composite (member),
	// check that the correct argument labels are supplied in the invocation

	if identifierExpression, ok := invokedExpression.(*ast.IdentifierExpression); ok {
		checker.checkIdentifierInvocationArgumentLabels(
			invocationExpression,
			identifierExpression,
		)
	} else if memberExpression, ok := invokedExpression.(*ast.MemberExpression); ok {
		checker.checkMemberInvocationArgumentLabels(
			invocationExpression,
			memberExpression,
		)
	}

	parameterTypeAnnotations := functionType.ParameterTypeAnnotations
	if len(argumentTypes) == len(parameterTypeAnnotations) &&
		functionType.GetReturnType != nil {

		returnType = functionType.GetReturnType(argumentTypes)
	} else {
		returnType = functionType.ReturnTypeAnnotation.Type
	}

	checker.Elaboration.InvocationExpressionArgumentTypes[invocationExpression] = argumentTypes

	parameterTypes := make([]Type, len(parameterTypeAnnotations))
	for i, parameterTypeAnnotation := range parameterTypeAnnotations {
		parameterTypes[i] = parameterTypeAnnotation.Type
	}
	checker.Elaboration.InvocationExpressionParameterTypes[invocationExpression] = parameterTypes

	checker.checkConstructorInvocationWithResourceResult(
		invocationExpression,
		invokableType,
		returnType,
		inCreate,
	)

	// Update the return info for invocations that do not return (i.e. have a `Never` return type)

	if returnType.Equal(&NeverType{}) {
		functionActivation := checker.functionActivations.Current()
		functionActivation.ReturnInfo.MaybeReturned = true
		functionActivation.ReturnInfo.DefinitelyReturned = true
	}

	return returnType
}

func (checker *Checker) checkConstructorInvocationWithResourceResult(
	invocationExpression *ast.InvocationExpression,
	invokableType InvokableType,
	returnType Type,
	inCreate bool,
) {
	if _, ok := invokableType.(*SpecialFunctionType); !ok {
		return
	}

	// NOTE: not using `isResourceType`,
	// as only direct resource types can be constructed

	if compositeReturnType, ok := returnType.(*CompositeType); !ok ||
		compositeReturnType.Kind != common.CompositeKindResource {

		return
	}

	if inCreate {
		return
	}

	checker.report(
		&MissingCreateError{
			Range: ast.NewRangeFromPositioned(invocationExpression),
		},
	)
}

func (checker *Checker) checkIdentifierInvocationArgumentLabels(
	invocationExpression *ast.InvocationExpression,
	identifierExpression *ast.IdentifierExpression,
) {
	variable := checker.findAndCheckVariable(identifierExpression.Identifier, false)

	if variable == nil || len(variable.ArgumentLabels) == 0 {
		return
	}

	checker.checkInvocationArgumentLabels(
		invocationExpression.Arguments,
		variable.ArgumentLabels,
	)
}

func (checker *Checker) checkMemberInvocationArgumentLabels(
	invocationExpression *ast.InvocationExpression,
	memberExpression *ast.MemberExpression,
) {
	member := checker.visitMember(memberExpression)

	if member == nil || len(member.ArgumentLabels) == 0 {
		return
	}

	checker.checkInvocationArgumentLabels(
		invocationExpression.Arguments,
		member.ArgumentLabels,
	)
}

func (checker *Checker) checkInvocationArgumentLabels(
	arguments []*ast.Argument,
	argumentLabels []string,
) {
	argumentCount := len(arguments)

	for i, argumentLabel := range argumentLabels {
		if i >= argumentCount {
			break
		}

		argument := arguments[i]
		providedLabel := argument.Label
		if argumentLabel == ArgumentLabelNotRequired {
			// argument label is not required,
			// check it is not provided

			if providedLabel != "" {
				checker.report(
					&IncorrectArgumentLabelError{
						ActualArgumentLabel:   providedLabel,
						ExpectedArgumentLabel: "",
						Range: ast.Range{
							StartPos: *argument.LabelStartPos,
							EndPos:   *argument.LabelEndPos,
						},
					},
				)
			}
		} else {
			// argument label is required,
			// check it is provided and correct
			if providedLabel == "" {
				checker.report(
					&MissingArgumentLabelError{
						ExpectedArgumentLabel: argumentLabel,
						Range:                 ast.NewRangeFromPositioned(argument.Expression),
					},
				)
			} else if providedLabel != argumentLabel {
				checker.report(
					&IncorrectArgumentLabelError{
						ActualArgumentLabel:   providedLabel,
						ExpectedArgumentLabel: argumentLabel,
						Range: ast.Range{
							StartPos: *argument.LabelStartPos,
							EndPos:   *argument.LabelEndPos,
						},
					},
				)
			}
		}
	}
}

func (checker *Checker) checkInvocationArguments(
	invocationExpression *ast.InvocationExpression,
	functionType *FunctionType,
) (
	argumentTypes []Type,
) {
	argumentCount := len(invocationExpression.Arguments)

	// check the invocation's argument count matches the function's parameter count
	parameterCount := len(functionType.ParameterTypeAnnotations)
	if argumentCount != parameterCount {

		// TODO: improve
		if functionType.RequiredArgumentCount == nil ||
			argumentCount < *functionType.RequiredArgumentCount {

			checker.report(
				&ArgumentCountError{
					ParameterCount: parameterCount,
					ArgumentCount:  argumentCount,
					Range:          ast.NewRangeFromPositioned(invocationExpression),
				},
			)
		}
	}

	minCount := argumentCount
	if parameterCount < argumentCount {
		minCount = parameterCount
	}

	argumentTypes = make([]Type, minCount)

	for i := 0; i < minCount; i++ {
		// ensure the type of the argument matches the type of the parameter

		parameterType := functionType.ParameterTypeAnnotations[i].Type
		argument := invocationExpression.Arguments[i]

		argumentType := argument.Expression.Accept(checker).(Type)

		argumentTypes[i] = argumentType

		if !parameterType.IsInvalidType() &&
			!checker.IsTypeCompatible(argument.Expression, argumentType, parameterType) {

			checker.report(
				&TypeMismatchError{
					ExpectedType: parameterType,
					ActualType:   argumentType,
					Range:        ast.NewRangeFromPositioned(argument.Expression),
				},
			)
		}

		checker.checkResourceMoveOperation(argument.Expression, argumentType)
	}

	return argumentTypes
}
