package sema

import (
	"github.com/onflow/cadence-initial/runtime/ast"
	"github.com/onflow/cadence-initial/runtime/common"
	"github.com/onflow/cadence-initial/runtime/errors"
)

func (checker *Checker) VisitFunctionDeclaration(declaration *ast.FunctionDeclaration) ast.Repr {
	return checker.visitFunctionDeclaration(
		declaration,
		functionDeclarationOptions{
			mustExit:          true,
			declareFunction:   true,
			checkResourceLoss: true,
		},
	)
}

type functionDeclarationOptions struct {
	// mustExit specifies if the function declaration's function block
	// should be checked for containing proper return statements.
	// This check may be omitted in e.g. function declarations of interfaces
	mustExit bool
	// declareFunction specifies if the function should also be declared in
	// the current scope. This might be e.g. true for global function
	// declarations, but false for function declarations of composites
	declareFunction bool
	// checkResourceLoss if the function should be checked for resource loss.
	// For example, function declarations in interfaces should not be checked.
	checkResourceLoss bool
}

func (checker *Checker) visitFunctionDeclaration(
	declaration *ast.FunctionDeclaration,
	options functionDeclarationOptions,
) ast.Repr {

	checker.checkDeclarationAccessModifier(
		declaration.Access,
		declaration.DeclarationKind(),
		declaration.StartPos,
		true,
	)

	// global functions were previously declared, see `declareFunctionDeclaration`

	functionType := checker.Elaboration.FunctionDeclarationFunctionTypes[declaration]
	if functionType == nil {
		functionType = checker.functionType(declaration.ParameterList, declaration.ReturnTypeAnnotation)

		if options.declareFunction {
			checker.declareFunctionDeclaration(declaration, functionType)
		}
	}

	checker.Elaboration.FunctionDeclarationFunctionTypes[declaration] = functionType

	checker.checkFunction(
		declaration.ParameterList,
		declaration.ReturnTypeAnnotation,
		functionType,
		declaration.FunctionBlock,
		options.mustExit,
		nil,
		options.checkResourceLoss,
	)

	return nil
}

func (checker *Checker) declareFunctionDeclaration(
	declaration *ast.FunctionDeclaration,
	functionType *FunctionType,
) {
	argumentLabels := declaration.ParameterList.EffectiveArgumentLabels()

	variable, err := checker.valueActivations.Declare(variableDeclaration{
		identifier:               declaration.Identifier.Identifier,
		ty:                       functionType,
		access:                   declaration.Access,
		kind:                     common.DeclarationKindFunction,
		pos:                      declaration.Identifier.Pos,
		isConstant:               true,
		argumentLabels:           argumentLabels,
		allowOuterScopeShadowing: false,
	})
	checker.report(err)

	checker.recordVariableDeclarationOccurrence(declaration.Identifier.Identifier, variable)
}

func (checker *Checker) checkFunction(
	parameterList *ast.ParameterList,
	returnTypeAnnotation *ast.TypeAnnotation,
	functionType *FunctionType,
	functionBlock *ast.FunctionBlock,
	mustExit bool,
	initializationInfo *InitializationInfo,
	checkResourceLoss bool,
) {
	// check argument labels
	checker.checkArgumentLabels(parameterList)

	checker.checkParameters(parameterList, functionType.Parameters)

	if functionType.ReturnTypeAnnotation != nil {
		checker.checkTypeAnnotation(functionType.ReturnTypeAnnotation, returnTypeAnnotation)
	}

	// Reset the returning state and restore it when leaving

	returned := checker.resources.Returns
	checker.resources.Returns = false
	defer func() {
		checker.resources.Returns = returned
	}()

	// NOTE: Always declare the function parameters, even if the function body is empty.
	// For example, event declarations have an initializer with an empty body,
	// but their parameters (e.g. duplication) needs to still be checked.

	checker.functionActivations.WithFunction(
		functionType,
		checker.valueActivations.Depth(),
		func() {
			// NOTE: important to begin scope in function activation, so that
			//   variable declarations will have proper function activation
			//   associated to it, and declare parameters in this new scope

			checker.enterValueScope()
			defer checker.leaveValueScope(checkResourceLoss)

			checker.declareParameters(parameterList, functionType.Parameters)

			functionActivation := checker.functionActivations.Current()
			functionActivation.InitializationInfo = initializationInfo

			if functionBlock != nil {
				checker.visitFunctionBlock(
					functionBlock,
					functionType.ReturnTypeAnnotation,
					checkResourceLoss,
				)

				if mustExit {
					returnType := functionType.ReturnTypeAnnotation.Type
					checker.checkFunctionExits(functionBlock, returnType)
				}
			}

			if initializationInfo != nil {
				checker.checkFieldMembersInitialized(initializationInfo)
			}
		},
	)
}

// checkFunctionExits checks that the given function block exits
// with a return-type appropriate return statement.
// The return is not needed if the function has a `Void` return type.
//
func (checker *Checker) checkFunctionExits(functionBlock *ast.FunctionBlock, returnType Type) {
	if _, returnTypeIsVoid := returnType.(*VoidType); returnTypeIsVoid {
		return
	}

	functionActivation := checker.functionActivations.Current()

	definitelyReturnedOrHalted :=
		functionActivation.ReturnInfo.DefinitelyReturned ||
			functionActivation.ReturnInfo.DefinitelyHalted

	if definitelyReturnedOrHalted {
		return
	}

	checker.report(
		&MissingReturnStatementError{
			Range: ast.NewRangeFromPositioned(functionBlock),
		},
	)
}

func (checker *Checker) checkParameters(parameterList *ast.ParameterList, parameters []*Parameter) {
	for i, parameter := range parameterList.Parameters {
		parameterTypeAnnotation := parameters[i].TypeAnnotation

		checker.checkTypeAnnotation(
			parameterTypeAnnotation,
			parameter.TypeAnnotation,
		)
	}
}

// checkArgumentLabels checks that all argument labels (if any) are unique
//
func (checker *Checker) checkArgumentLabels(parameterList *ast.ParameterList) {

	argumentLabelPositions := map[string]ast.Position{}

	for _, parameter := range parameterList.Parameters {
		label := parameter.Label
		if label == "" || label == ArgumentLabelNotRequired {
			continue
		}

		labelPos := parameter.StartPos

		if previousPos, ok := argumentLabelPositions[label]; ok {
			checker.report(
				&RedeclarationError{
					Kind:        common.DeclarationKindArgumentLabel,
					Name:        label,
					Pos:         labelPos,
					PreviousPos: &previousPos,
				},
			)
		}

		argumentLabelPositions[label] = labelPos
	}
}

// declareParameters declares a constant for each parameter,
// ensuring names are unique and constants don't already exist
//
func (checker *Checker) declareParameters(
	parameterList *ast.ParameterList,
	parameters []*Parameter,
) {
	depth := checker.valueActivations.Depth()

	for i, parameter := range parameterList.Parameters {
		identifier := parameter.Identifier

		// check if variable with this identifier is already declared in the current scope
		existingVariable := checker.valueActivations.Find(identifier.Identifier)
		if existingVariable != nil && existingVariable.Depth == depth {
			checker.report(
				&RedeclarationError{
					Kind:        common.DeclarationKindParameter,
					Name:        identifier.Identifier,
					Pos:         identifier.Pos,
					PreviousPos: existingVariable.Pos,
				},
			)

			continue
		}

		parameterType := parameters[i].TypeAnnotation.Type

		variable := &Variable{
			Identifier:      identifier.Identifier,
			Access:          ast.AccessPublic,
			DeclarationKind: common.DeclarationKindParameter,
			IsConstant:      true,
			Type:            parameterType,
			Depth:           depth,
			Pos:             &identifier.Pos,
		}
		checker.valueActivations.Set(identifier.Identifier, variable)
		checker.recordVariableDeclarationOccurrence(identifier.Identifier, variable)
	}
}

func (checker *Checker) VisitFunctionBlock(functionBlock *ast.FunctionBlock) ast.Repr {
	// NOTE: see visitFunctionBlock
	panic(errors.NewUnreachableError())
}

func (checker *Checker) visitWithPostConditions(postConditions *ast.Conditions, returnType Type, body func()) {

	var rewrittenPostConditions *PostConditionsRewrite

	// If there are post-conditions, rewrite them, extracting `before` expressions.
	// The result are variable declarations which need to be evaluated before
	// the function body

	if postConditions != nil {
		rewriteResult := checker.rewritePostConditions(*postConditions)
		rewrittenPostConditions = &rewriteResult

		checker.Elaboration.PostConditionsRewrite[postConditions] = rewriteResult

		checker.visitStatements(rewriteResult.BeforeStatements)
	}

	body()

	// If there is a post-conditions, declare the function `before`

	// TODO: improve: only declare when a condition actually refers to `before`?

	if postConditions != nil &&
		len(*postConditions) > 0 {

		checker.declareBefore()
	}

	// If there is a return type, declare the constant `result` which has the return type

	if _, ok := returnType.(*VoidType); !ok {
		checker.declareResult(returnType)
	}

	if rewrittenPostConditions != nil {
		checker.visitConditions(rewrittenPostConditions.RewrittenPostConditions)
	}
}

func (checker *Checker) visitFunctionBlock(
	functionBlock *ast.FunctionBlock,
	returnTypeAnnotation *TypeAnnotation,
	checkResourceLoss bool,
) {
	checker.enterValueScope()
	defer checker.leaveValueScope(checkResourceLoss)

	if functionBlock.PreConditions != nil {
		checker.visitConditions(*functionBlock.PreConditions)
	}

	checker.visitWithPostConditions(
		functionBlock.PostConditions,
		returnTypeAnnotation.Type,
		func() {
			// NOTE: not checking block as it enters a new scope
			// and post-conditions need to be able to refer to block's declarations

			checker.visitStatements(functionBlock.Block.Statements)
		},
	)
}

func (checker *Checker) declareResult(ty Type) {
	_, err := checker.valueActivations.DeclareImplicitConstant(
		ResultIdentifier,
		ty,
		common.DeclarationKindResult,
	)
	checker.report(err)
	// TODO: record occurrence - but what position?
}

func (checker *Checker) declareBefore() {
	_, err := checker.valueActivations.DeclareImplicitConstant(
		BeforeIdentifier,
		beforeType,
		common.DeclarationKindFunction,
	)
	checker.report(err)
	// TODO: record occurrence – but what position?
}

func (checker *Checker) VisitFunctionExpression(expression *ast.FunctionExpression) ast.Repr {

	// TODO: infer
	functionType := checker.functionType(expression.ParameterList, expression.ReturnTypeAnnotation)

	checker.Elaboration.FunctionExpressionFunctionType[expression] = functionType

	checker.checkFunction(
		expression.ParameterList,
		expression.ReturnTypeAnnotation,
		functionType,
		expression.FunctionBlock,
		true,
		nil,
		true,
	)

	// function expressions are not allowed in conditions

	if checker.inCondition {
		checker.report(
			&FunctionExpressionInConditionError{
				Range: ast.NewRangeFromPositioned(expression),
			},
		)
	}

	return functionType
}

// checkFieldMembersInitialized checks that all fields that were required
// to be initialized (as stated in the initialization info) have been initialized.
//
func (checker *Checker) checkFieldMembersInitialized(info *InitializationInfo) {
	for member, field := range info.FieldMembers {
		isInitialized := info.InitializedFieldMembers.Contains(member)
		if isInitialized {
			continue
		}

		checker.report(
			&FieldUninitializedError{
				Name:          field.Identifier.Identifier,
				Pos:           field.Identifier.Pos,
				ContainerType: info.ContainerType,
			},
		)
	}
}
