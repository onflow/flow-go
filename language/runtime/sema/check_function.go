package sema

import (
	"github.com/dapperlabs/flow-go/language/runtime/ast"
	"github.com/dapperlabs/flow-go/language/runtime/common"
	"github.com/dapperlabs/flow-go/language/runtime/errors"
)

func (checker *Checker) VisitFunctionDeclaration(declaration *ast.FunctionDeclaration) ast.Repr {
	return checker.visitFunctionDeclaration(
		declaration,
		functionDeclarationOptions{
			mustExit:        true,
			declareFunction: true,
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
}

func (checker *Checker) visitFunctionDeclaration(
	declaration *ast.FunctionDeclaration,
	options functionDeclarationOptions,
) ast.Repr {
	checker.checkFunctionAccessModifier(declaration)

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
		declaration.ReturnTypeAnnotation.StartPos,
		functionType,
		declaration.FunctionBlock,
		options.mustExit,
		nil,
	)

	return nil
}

func (checker *Checker) declareFunctionDeclaration(
	declaration *ast.FunctionDeclaration,
	functionType *FunctionType,
) {
	argumentLabels := declaration.ParameterList.ArgumentLabels()

	variable, err := checker.valueActivations.DeclareFunction(
		declaration.Identifier,
		functionType,
		argumentLabels,
	)
	checker.report(err)

	checker.recordVariableDeclarationOccurrence(declaration.Identifier.Identifier, variable)
}

func (checker *Checker) checkFunctionAccessModifier(declaration *ast.FunctionDeclaration) {
	switch declaration.Access {
	case ast.AccessNotSpecified, ast.AccessPublic:
		return
	default:
		checker.report(
			&InvalidAccessModifierError{
				DeclarationKind: common.DeclarationKindFunction,
				Access:          declaration.Access,
				Pos:             declaration.StartPosition(),
			},
		)
	}
}

func (checker *Checker) checkFunction(
	parameterList *ast.ParameterList,
	returnTypePosition ast.Position,
	functionType *FunctionType,
	functionBlock *ast.FunctionBlock,
	mustExit bool,
	initializationInfo *InitializationInfo,
) {
	// check argument labels
	checker.checkArgumentLabels(parameterList)

	checker.checkParameters(parameterList, functionType.ParameterTypeAnnotations)
	if functionType.ReturnTypeAnnotation != nil {
		checker.checkTypeAnnotation(functionType.ReturnTypeAnnotation, returnTypePosition)
	}

	if functionBlock != nil {
		checker.functionActivations.WithFunction(
			functionType,
			checker.valueActivations.Depth(),
			func() {
				// NOTE: important to begin scope in function activation, so that
				//   variable declarations will have proper function activation
				//   associated to it, and declare parameters in this new scope
				checker.enterValueScope()
				defer checker.leaveValueScope()

				checker.declareParameters(parameterList, functionType.ParameterTypeAnnotations)

				functionActivation := checker.functionActivations.Current()
				functionActivation.InitializationInfo = initializationInfo

				checker.visitFunctionBlock(
					functionBlock,
					functionType.ReturnTypeAnnotation,
				)

				if mustExit {
					returnType := functionType.ReturnTypeAnnotation.Type
					checker.checkFunctionExits(functionBlock, returnType)
				}

				if initializationInfo != nil {
					checker.checkFieldMembersInitialized(initializationInfo)
				}
			},
		)
	}
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
	if functionActivation.ReturnInfo.DefinitelyReturned {
		return
	}

	checker.report(
		&MissingReturnStatementError{
			Range: ast.NewRangeFromPositioned(functionBlock),
		},
	)
}

func (checker *Checker) checkParameters(parameterList *ast.ParameterList, parameterTypeAnnotations []*TypeAnnotation) {
	for i, parameter := range parameterList.Parameters {
		parameterTypeAnnotation := parameterTypeAnnotations[i]
		checker.checkTypeAnnotation(parameterTypeAnnotation, parameter.TypeAnnotation.StartPos)
	}
}

func (checker *Checker) checkTypeAnnotation(typeAnnotation *TypeAnnotation, pos ast.Position) {
	checker.checkMoveAnnotation(
		typeAnnotation.Type,
		typeAnnotation.Move,
		pos,
	)
}

func (checker *Checker) checkMoveAnnotation(ty Type, move bool, pos ast.Position) {
	if ty.IsResourceType() {
		if !move {
			checker.report(
				&MissingMoveAnnotationError{
					Pos: pos,
				},
			)
		}
	} else {
		if move {
			checker.report(
				&InvalidMoveAnnotationError{
					Pos: pos,
				},
			)
		}
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
	parameterTypeAnnotations []*TypeAnnotation,
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

		parameterTypeAnnotation := parameterTypeAnnotations[i]
		parameterType := parameterTypeAnnotation.Type

		variable := &Variable{
			Identifier:      identifier.Identifier,
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
	panic(&errors.UnreachableError{})
}

func (checker *Checker) visitFunctionBlock(functionBlock *ast.FunctionBlock, returnTypeAnnotation *TypeAnnotation) {

	checker.enterValueScope()
	defer checker.leaveValueScope()

	checker.visitConditions(functionBlock.PreConditions)

	// NOTE: not checking block as it enters a new scope
	// and post-conditions need to be able to refer to block's declarations

	checker.visitStatements(functionBlock.Block.Statements)

	// if there is a post-condition, declare the function `before`

	// TODO: improve: only declare when a condition actually refers to `before`?

	if len(functionBlock.PostConditions) > 0 {
		checker.declareBefore()
	}

	// if there is a return type, declare the constant `result`
	// which has the return type

	if _, ok := returnTypeAnnotation.Type.(*VoidType); !ok {
		checker.declareResult(returnTypeAnnotation.Type)
	}

	checker.visitConditions(functionBlock.PostConditions)
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
		expression.ReturnTypeAnnotation.StartPos,
		functionType,
		expression.FunctionBlock,
		true,
		nil,
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
