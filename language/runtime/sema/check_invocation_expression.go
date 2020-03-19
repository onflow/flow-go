package sema

import (
	"github.com/dapperlabs/flow-go/language/runtime/ast"
	"github.com/dapperlabs/flow-go/language/runtime/common"
	"github.com/dapperlabs/flow-go/language/runtime/errors"
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

	isOptionalResult := false
	if memberExpression, ok := invokedExpression.(*ast.MemberExpression); ok {
		var member *Member
		member, isOptionalResult = checker.visitMember(memberExpression)
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

	// invoked expression has function type

	var returnType Type = &InvalidType{}

	argumentTypes, functionType := checker.checkInvocation(invocationExpression, invokableType)
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

	returnType = functionType.ReturnTypeAnnotation.Type
	checker.Elaboration.InvocationExpressionReturnTypes[invocationExpression] = returnType

	parameters := functionType.Parameters
	parameterTypes := make([]Type, len(parameters))
	for i, parameter := range parameters {
		parameterTypes[i] = parameter.TypeAnnotation.Type
	}
	checker.Elaboration.InvocationExpressionParameterTypes[invocationExpression] = parameterTypes

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

	if isOptionalResult {
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

func (checker *Checker) checkTypeBound(typeArgument Type, typeParameter *TypeParameter, pos ast.HasPosition) {
	if typeParameter.Type == nil ||
		typeParameter.Type.IsInvalidType() {

		return
	}

	if !IsSubType(typeArgument, typeParameter.Type) {

		checker.report(
			&TypeMismatchError{
				ExpectedType: typeParameter.Type,
				ActualType:   typeArgument,
				Range:        ast.NewRangeFromPositioned(pos),
			},
		)
	}
}

func (checker *Checker) checkInvocation(
	invocationExpression *ast.InvocationExpression,
	invokableType InvokableType,
) (
	argumentTypes []Type,
	functionType *FunctionType,
) {
	// The function type is either concrete (monomorphic, `*FunctionType`)
	// or it is generic (polymorphic, `*GenericFunctionType`, a "type scheme").

	genericFunctionType := invokableType.InvocationGenericFunctionType()
	functionType = invokableType.InvocationFunctionType()

	// In each case, get the parameter count and required argument count.

	var parameterCount int
	var requiredArgumentCount *int
	var typeParameterCount int

	if genericFunctionType != nil {
		parameterCount = len(genericFunctionType.Parameters)
		requiredArgumentCount = genericFunctionType.RequiredArgumentCount
		typeParameterCount = len(genericFunctionType.TypeParameters)
	} else {
		parameterCount = len(functionType.Parameters)
		requiredArgumentCount = functionType.RequiredArgumentCount
	}

	// Check the type arguments and bind them to type parameters

	typeArgumentCount := len(invocationExpression.TypeArguments)

	typeParameters := make(map[*TypeParameter]Type, typeParameterCount)

	if genericFunctionType != nil {

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
			genericFunctionType.TypeParameters,
			typeParameters,
		)

	} else {
		// If the function type is monomorphic, no argument types are allowed,
		// and no binding is necessary.

		if typeArgumentCount > 0 {
			checker.reportInvalidTypeArguments(
				invocationExpression.TypeArguments,
				typeArgumentCount,
			)
		}
	}

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

	// Check all the required arguments

	for argumentIndex := 0; argumentIndex < minCount; argumentIndex++ {

		checker.checkInvocationRequiredArgument(
			invocationExpression.Arguments,
			argumentIndex,
			genericFunctionType,
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

	// If the function type is generic, prepare the concrete function type

	if genericFunctionType != nil {
		functionType = checker.instantiateGenericFunction(
			genericFunctionType,
			typeParameters,
		)
	}

	// Check all type parameters have been bound to a type.

	if genericFunctionType != nil {
		checker.checkTypeParameterInference(
			genericFunctionType,
			typeParameters,
			invocationExpression,
		)
	}

	// Save the type parameter's types in the elaboration

	checker.Elaboration.InvocationExpressionTypeParameterTypes[invocationExpression] = typeParameters

	return argumentTypes, functionType
}

// checkTypeParameterInference checks that all type parameters
// of the given generic function type have been assigned a type.
//
func (checker *Checker) checkTypeParameterInference(
	genericFunctionType *GenericFunctionType,
	typeParameters map[*TypeParameter]Type,
	invocationExpression *ast.InvocationExpression,
) {
	for _, typeParameter := range genericFunctionType.TypeParameters {
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

// instantiateGenericFunction returns a concrete function type
// for the given generic function type based on the given type parameter bindings.
//
func (checker *Checker) instantiateGenericFunction(
	genericFunctionType *GenericFunctionType,
	typeParameters map[*TypeParameter]Type,
) *FunctionType {

	parameters := make([]*Parameter, len(genericFunctionType.Parameters))

	// Prepare the concrete parameters from the generic function's parameter,
	// which potentially has a generic type parameter annotation.

	for i, parameter := range genericFunctionType.Parameters {

		var parameterType Type

		genericTypeAnnotation := parameter.TypeAnnotation
		switch {
		case genericTypeAnnotation.TypeAnnotation != nil:
			parameterType = genericTypeAnnotation.TypeAnnotation.Type

		case genericTypeAnnotation.TypeParameter != nil:
			typeParameter := genericTypeAnnotation.TypeParameter
			parameterType = typeParameters[typeParameter]

			if parameterType == nil {
				parameterType = &InvalidType{}

				// NOTE: will be reported at end of function
			}

		default:
			panic(errors.NewUnreachableError())
		}

		parameters[i] = &Parameter{
			Label:          parameter.Label,
			Identifier:     parameter.Identifier,
			TypeAnnotation: NewTypeAnnotation(parameterType),
		}
	}

	// Prepare the concrete return type annotation
	// from the generic function's return type annotation,
	// which potentially has a generic type parameter annotation.

	var returnType Type

	genericTypeAnnotation := genericFunctionType.ReturnTypeAnnotation
	switch {
	case genericTypeAnnotation.TypeAnnotation != nil:
		returnType = genericTypeAnnotation.TypeAnnotation.Type

	case genericTypeAnnotation.TypeParameter != nil:
		typeParameter := genericTypeAnnotation.TypeParameter
		returnType = typeParameters[typeParameter]

		if returnType == nil {
			returnType = &InvalidType{}

			// NOTE: will be reported at end of function
		}

	default:
		panic(errors.NewUnreachableError())
	}

	return &FunctionType{
		Parameters:            parameters,
		ReturnTypeAnnotation:  NewTypeAnnotation(returnType),
		RequiredArgumentCount: genericFunctionType.RequiredArgumentCount,
	}
}

func (checker *Checker) checkInvocationRequiredArgument(
	arguments ast.Arguments,
	argumentIndex int,
	genericFunctionType *GenericFunctionType,
	functionType *FunctionType,
	argumentTypes []Type,
	typeParameters map[*TypeParameter]Type,
) {
	argument := arguments[argumentIndex]

	argumentType := argument.Expression.Accept(checker).(Type)

	var parameterType Type
	var typeParameter *TypeParameter

	if genericFunctionType != nil {
		parameter := genericFunctionType.Parameters[argumentIndex]

		// If the function is generic, the parameter might be either a concrete type annotation,
		// or the parameter refers to a type parameter.

		genericTypeAnnotation := parameter.TypeAnnotation
		switch {
		case genericTypeAnnotation.TypeAnnotation != nil:
			parameterType = genericTypeAnnotation.TypeAnnotation.Type

		case genericTypeAnnotation.TypeParameter != nil:
			typeParameter = genericTypeAnnotation.TypeParameter

		default:
			panic(errors.NewUnreachableError())
		}
	} else {
		parameter := functionType.Parameters[argumentIndex]
		parameterType = parameter.TypeAnnotation.Type
	}

	switch {
	case parameterType != nil:
		// Check that the type of the argument matches the type of the parameter
		argumentTypes[argumentIndex] = checker.checkInvocationArgumentType(argument.Expression, argumentType, parameterType)

	case typeParameter != nil:
		if unifiedType, ok := typeParameters[typeParameter]; ok {

			// If the type parameter is already bound to a type argument
			// (either explicit by a type argument, or implicit through an argument's type),
			// check that this argument's type matches the

			if !argumentType.Equal(unifiedType) {
				checker.report(
					&TypeMismatchError{
						ExpectedType: unifiedType,
						ActualType:   argumentType,
						Range:        ast.NewRangeFromPositioned(argument.Expression),
					},
				)
			}
		} else {
			// If the type parameter is not uet bound to a type argument, bind it.

			typeParameters[typeParameter] = argumentType

			// If the type parameter corresponding to the type argument has a type bound,
			// then check that the argument's type is a subtype of the type bound.

			checker.checkTypeBound(argumentType, typeParameter, argument.Expression)
		}

	default:
		panic(errors.NewUnreachableError())
	}
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

func (checker *Checker) reportInvalidTypeArguments(typeArguments []ast.Type, typeArgumentCount int) {
	firstSuperfluousTypeArgument := typeArguments[0]

	lastSuperfluousTypeArgument := typeArguments[typeArgumentCount-1]

	checker.report(
		&InvalidTypeArgumentsError{
			Range: ast.Range{
				StartPos: firstSuperfluousTypeArgument.StartPosition(),
				EndPos:   lastSuperfluousTypeArgument.EndPosition(),
			},
		},
	)
}

func (checker *Checker) reportInvalidTypeArgumentCount(
	typeArgumentCount int,
	typeParameterCount int,
	allTypeArguments []ast.Type,
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
	typeArguments []ast.Type,
	typeParameters []*TypeParameter,
	typeParameterTypes map[*TypeParameter]Type,
) {
	for i := 0; i < len(typeArguments); i++ {
		rawTypeArgument := typeArguments[i]

		typeArgument := checker.ConvertType(rawTypeArgument)

		// Don't check or bind invalid type arguments

		if typeArgument.IsInvalidType() {
			continue
		}

		typeParameter := typeParameters[i]

		// If the type parameter corresponding to the type argument has a type bound,
		// then check that the argument is a subtype of the type bound.

		checker.checkTypeBound(typeArgument, typeParameter, rawTypeArgument)

		// Bind the type argument to the type parameter

		typeParameterTypes[typeParameter] = typeArgument
	}
}

func (checker *Checker) checkInvocationArgumentType(argument ast.Expression, argumentType, parameterType Type) Type {

	if !argumentType.IsInvalidType() &&
		!parameterType.IsInvalidType() &&
		!checker.checkTypeCompatibility(argument, argumentType, parameterType) {

		checker.report(
			&TypeMismatchError{
				ExpectedType: parameterType,
				ActualType:   argumentType,
				Range:        ast.NewRangeFromPositioned(argument),
			},
		)
	}

	checker.checkVariableMove(argument)
	checker.checkResourceMoveOperation(argument, argumentType)

	return argumentType
}
