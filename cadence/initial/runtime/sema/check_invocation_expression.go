/*
 * Cadence - The resource-oriented smart contract programming language
 *
 * Copyright 2019-2020 Dapper Labs, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package sema

import (
	"example.com/cadence-initial/runtime/ast"
	"example.com/cadence-initial/runtime/common"
)

func (checker *Checker) VisitInvocationExpression(invocationExpression *ast.InvocationExpression) ast.Repr {
	typ := checker.checkInvocationExpression(invocationExpression)

	// Events cannot be invoked without an emit statement

	compositeType, isCompositeType := typ.(*CompositeType)
	if isCompositeType && compositeType.Kind == common.CompositeKindEvent {
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

	inInvocation := checker.inInvocation
	checker.inInvocation = true
	defer func() {
		checker.inInvocation = inInvocation
	}()

	// check the invoked expression can be invoked

	invokedExpression := invocationExpression.InvokedExpression
	expressionType := invokedExpression.Accept(checker).(Type)

	isOptionalChainingResult := false
	if memberExpression, ok := invokedExpression.(*ast.MemberExpression); ok {
		var member *Member
		member, isOptionalChainingResult = checker.visitMember(memberExpression)
		if member != nil {
			expressionType = member.TypeAnnotation.Type
		}
	}

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

	// The invoked expression has a function type,
	// check the invocation including all arguments.
	//
	// If the invocation is on a member expression which is optional chaining,'
	// then `isOptionalChainingResult` is true, which means the invocation
	// is only potential, i.e. the invocation will not always

	var argumentTypes []Type
	var returnType Type

	checkInvocation := func() {
		argumentTypes, returnType =
			checker.checkInvocation(invocationExpression, invokableType)
	}

	if isOptionalChainingResult {
		_ = checker.checkPotentiallyUnevaluated(func() Type {
			checkInvocation()
			// ignored
			return nil
		})
	} else {
		checkInvocation()
	}

	checker.Elaboration.InvocationExpressionArgumentTypes[invocationExpression] = argumentTypes

	// If the invocation refers directly to the name of the function as stated in the declaration,
	// or the invocation refers to a function of a composite (member),
	// check that the correct argument labels are supplied in the invocation

	switch typedInvokedExpression := invokedExpression.(type) {
	case *ast.IdentifierExpression:
		checker.checkIdentifierInvocationArgumentLabels(
			invocationExpression,
			typedInvokedExpression,
		)

	case *ast.MemberExpression:
		checker.checkMemberInvocationArgumentLabels(
			invocationExpression,
			typedInvokedExpression,
		)
	}

	checker.checkConstructorInvocationWithResourceResult(
		invocationExpression,
		invokableType,
		returnType,
		inCreate,
	)

	// Update the return info for invocations that do not return (i.e. have a `Never` return type)

	if _, ok = returnType.(*NeverType); ok {
		functionActivation := checker.functionActivations.Current()
		functionActivation.ReturnInfo.DefinitelyHalted = true
	}

	if isOptionalChainingResult {
		return &OptionalType{Type: returnType}
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
	member, _ := checker.visitMember(memberExpression)

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

func (checker *Checker) checkInvocation(
	invocationExpression *ast.InvocationExpression,
	invokableType InvokableType,
) (
	argumentTypes []Type,
	returnType Type,
) {
	functionType := invokableType.InvocationFunctionType()

	parameterCount := len(functionType.Parameters)
	requiredArgumentCount := functionType.RequiredArgumentCount
	typeParameterCount := len(functionType.TypeParameters)

	// Check the type arguments and bind them to type parameters

	typeArgumentCount := len(invocationExpression.TypeArguments)

	typeParameters := make(map[*TypeParameter]Type, typeParameterCount)

	// If the function type is generic, the invocation might provide
	// explicit type arguments for the type parameters.

	// Check that the number of type arguments does not exceed
	// the number of type parameters

	validTypeArgumentCount := typeArgumentCount

	if typeArgumentCount > typeParameterCount {

		validTypeArgumentCount = typeParameterCount

		checker.reportInvalidTypeArgumentCount(
			typeArgumentCount,
			typeParameterCount,
			invocationExpression.TypeArguments,
		)
	}

	// Check all non-superfluous type arguments
	// and bind them to the type parameters

	validTypeArguments := invocationExpression.TypeArguments[:validTypeArgumentCount]

	checker.checkAndBindGenericTypeParameterTypeArguments(
		validTypeArguments,
		functionType.TypeParameters,
		typeParameters,
	)

	// Check that the invocation's argument count matches the function's parameter count

	argumentCount := len(invocationExpression.Arguments)

	// TODO: only pass position of arguments, not whole invocation
	checker.checkInvocationArgumentCount(
		argumentCount,
		parameterCount,
		requiredArgumentCount,
		invocationExpression,
	)

	minCount := argumentCount
	if parameterCount < argumentCount {
		minCount = parameterCount
	}

	argumentTypes = make([]Type, argumentCount)
	parameterTypes := make([]Type, argumentCount)

	// Check all the required arguments

	for argumentIndex := 0; argumentIndex < minCount; argumentIndex++ {

		parameterTypes[argumentIndex] =
			checker.checkInvocationRequiredArgument(
				invocationExpression.Arguments,
				argumentIndex,
				functionType,
				argumentTypes,
				typeParameters,
			)
	}

	// Add extra argument types

	for i := minCount; i < argumentCount; i++ {
		argument := invocationExpression.Arguments[i]

		argumentTypes[i] = argument.Expression.Accept(checker).(Type)
	}

	// The invokable type might have special checks for the arguments

	argumentExpressions := make([]ast.Expression, argumentCount)
	for i, argument := range invocationExpression.Arguments {
		argumentExpressions[i] = argument.Expression
	}

	invokableType.CheckArgumentExpressions(checker, argumentExpressions)

	returnType = functionType.ReturnTypeAnnotation.Type.Resolve(typeParameters)
	if returnType == nil {
		// TODO: report error? does `checkTypeParameterInference` below already do that?
		returnType = &InvalidType{}
	}

	// Check all type parameters have been bound to a type.

	checker.checkTypeParameterInference(
		functionType,
		typeParameters,
		invocationExpression,
	)

	// Save types in the elaboration

	checker.Elaboration.InvocationExpressionTypeParameterTypes[invocationExpression] = typeParameters
	checker.Elaboration.InvocationExpressionParameterTypes[invocationExpression] = parameterTypes
	checker.Elaboration.InvocationExpressionReturnTypes[invocationExpression] = returnType

	return argumentTypes, returnType
}

// checkTypeParameterInference checks that all type parameters
// of the given generic function type have been assigned a type.
//
func (checker *Checker) checkTypeParameterInference(
	functionType *FunctionType,
	typeParameters map[*TypeParameter]Type,
	invocationExpression *ast.InvocationExpression,
) {
	for _, typeParameter := range functionType.TypeParameters {
		if typeParameters[typeParameter] != nil {
			continue
		}

		checker.report(
			&TypeParameterTypeInferenceError{
				Name:  typeParameter.Name,
				Range: ast.NewRangeFromPositioned(invocationExpression),
			},
		)
	}
}

func (checker *Checker) checkInvocationRequiredArgument(
	arguments ast.Arguments,
	argumentIndex int,
	functionType *FunctionType,
	argumentTypes []Type,
	typeParameters map[*TypeParameter]Type,
) (
	parameterType Type,
) {
	argument := arguments[argumentIndex]
	argumentType := argument.Expression.Accept(checker).(Type)
	argumentTypes[argumentIndex] = argumentType

	checker.checkInvocationArgumentMove(argument.Expression, argumentType)

	parameter := functionType.Parameters[argumentIndex]

	// Try to unify the parameter type with the argument type.
	// If unification fails, fall back to the parameter type for now.

	argumentRange := ast.NewRangeFromPositioned(argument.Expression)

	parameterType = parameter.TypeAnnotation.Type
	if parameterType.Unify(argumentType, typeParameters, checker.report, argumentRange) {
		parameterType = parameterType.Resolve(typeParameters)
		if parameterType == nil {
			parameterType = &InvalidType{}
		}
	}

	// Check that the type of the argument matches the type of the parameter.

	checker.checkInvocationArgumentParameterTypeCompatibility(
		argument.Expression,
		argumentType,
		parameterType,
	)

	return parameterType
}

func (checker *Checker) checkInvocationArgumentCount(
	argumentCount int,
	parameterCount int,
	requiredArgumentCount *int,
	pos ast.HasPosition,
) {

	if argumentCount == parameterCount {
		return
	}

	// TODO: improve
	if requiredArgumentCount == nil ||
		argumentCount < *requiredArgumentCount {

		checker.report(
			&ArgumentCountError{
				ParameterCount: parameterCount,
				ArgumentCount:  argumentCount,
				Range:          ast.NewRangeFromPositioned(pos),
			},
		)
	}
}

func (checker *Checker) reportInvalidTypeArgumentCount(
	typeArgumentCount int,
	typeParameterCount int,
	allTypeArguments []*ast.TypeAnnotation,
) {
	exceedingTypeArgumentIndexStart := typeArgumentCount - typeParameterCount - 1

	firstSuperfluousTypeArgument :=
		allTypeArguments[exceedingTypeArgumentIndexStart]

	lastSuperfluousTypeArgument :=
		allTypeArguments[typeArgumentCount-1]

	checker.report(
		&InvalidTypeArgumentCountError{
			TypeParameterCount: typeParameterCount,
			TypeArgumentCount:  typeArgumentCount,
			Range: ast.Range{
				StartPos: firstSuperfluousTypeArgument.StartPosition(),
				EndPos:   lastSuperfluousTypeArgument.EndPosition(),
			},
		},
	)
}

func (checker *Checker) checkAndBindGenericTypeParameterTypeArguments(
	typeArguments []*ast.TypeAnnotation,
	typeParameters []*TypeParameter,
	typeParameterTypes map[*TypeParameter]Type,
) {
	for i := 0; i < len(typeArguments); i++ {
		rawTypeArgument := typeArguments[i]

		typeArgument := checker.ConvertTypeAnnotation(rawTypeArgument)
		checker.checkTypeAnnotation(typeArgument, rawTypeArgument)

		ty := typeArgument.Type

		// Don't check or bind invalid type arguments

		if ty.IsInvalidType() {
			continue
		}

		typeParameter := typeParameters[i]

		// If the type parameter corresponding to the type argument has a type bound,
		// then check that the argument is a subtype of the type bound.

		err := typeParameter.checkTypeBound(ty, ast.NewRangeFromPositioned(rawTypeArgument))
		checker.report(err)

		// Bind the type argument to the type parameter

		typeParameterTypes[typeParameter] = ty
	}
}

func (checker *Checker) checkInvocationArgumentParameterTypeCompatibility(
	argument ast.Expression,
	argumentType, parameterType Type,
) {

	if argumentType.IsInvalidType() ||
		parameterType.IsInvalidType() {

		return
	}

	if !checker.checkTypeCompatibility(argument, argumentType, parameterType) {

		checker.report(
			&TypeMismatchError{
				ExpectedType: parameterType,
				ActualType:   argumentType,
				Range:        ast.NewRangeFromPositioned(argument),
			},
		)
	}
}

func (checker *Checker) checkInvocationArgumentMove(argument ast.Expression, argumentType Type) Type {

	checker.checkVariableMove(argument)
	checker.checkResourceMoveOperation(argument, argumentType)

	return argumentType
}
