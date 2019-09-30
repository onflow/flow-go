package sema

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/common"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/errors"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema/exit_detector"
)

func (checker *Checker) VisitFunctionDeclaration(declaration *ast.FunctionDeclaration) ast.Repr {
	return checker.visitFunctionDeclaration(declaration, true)
}

func (checker *Checker) visitFunctionDeclaration(declaration *ast.FunctionDeclaration, mustExit bool) ast.Repr {
	checker.checkFunctionAccessModifier(declaration)

	// global functions were previously declared, see `declareFunctionDeclaration`

	functionType := checker.Elaboration.FunctionDeclarationFunctionTypes[declaration]
	if functionType == nil {
		functionType = checker.declareFunctionDeclaration(declaration)
	}

	checker.checkFunction(
		declaration.Parameters,
		declaration.ReturnTypeAnnotation.StartPos,
		functionType,
		declaration.FunctionBlock,
		mustExit,
	)

	return nil
}

func (checker *Checker) declareFunctionDeclaration(declaration *ast.FunctionDeclaration) *FunctionType {

	functionType := checker.functionType(declaration.Parameters, declaration.ReturnTypeAnnotation)
	argumentLabels := declaration.Parameters.ArgumentLabels()

	checker.Elaboration.FunctionDeclarationFunctionTypes[declaration] = functionType

	variable, err := checker.valueActivations.DeclareFunction(
		declaration.Identifier,
		functionType,
		argumentLabels,
	)
	checker.report(err)

	checker.recordVariableDeclarationOccurrence(declaration.Identifier.Identifier, variable)

	return functionType
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
	parameters ast.Parameters,
	returnTypePosition ast.Position,
	functionType *FunctionType,
	functionBlock *ast.FunctionBlock,
	mustExit bool,
) {
	checker.valueActivations.Enter()
	defer checker.valueActivations.Leave()

	// check argument labels
	checker.checkArgumentLabels(parameters)

	checker.declareParameters(parameters, functionType.ParameterTypeAnnotations)

	checker.checkParameters(parameters, functionType.ParameterTypeAnnotations)
	if functionType.ReturnTypeAnnotation != nil {
		checker.checkTypeAnnotation(functionType.ReturnTypeAnnotation, returnTypePosition)
	}

	if functionBlock != nil {
		checker.functionActivations.WithFunction(functionType, func() {
			checker.visitFunctionBlock(
				functionBlock,
				functionType.ReturnTypeAnnotation,
			)
		})

		if _, returnTypeIsVoid := functionType.ReturnTypeAnnotation.Type.(*VoidType); !returnTypeIsVoid {
			if mustExit && !exit_detector.FunctionBlockExits(functionBlock) {
				checker.report(
					&MissingReturnStatementError{
						StartPos: functionBlock.StartPosition(),
						EndPos:   functionBlock.EndPosition(),
					},
				)
			}
		}
	}
}

func (checker *Checker) checkParameters(parameters ast.Parameters, parameterTypeAnnotations []*TypeAnnotation) {
	for i, parameter := range parameters {
		parameterTypeAnnotation := parameterTypeAnnotations[i]
		checker.checkTypeAnnotation(parameterTypeAnnotation, parameter.StartPos)
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
func (checker *Checker) checkArgumentLabels(parameters ast.Parameters) {

	argumentLabelPositions := map[string]ast.Position{}

	for _, parameter := range parameters {
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
func (checker *Checker) declareParameters(parameters ast.Parameters, parameterTypeAnnotations []*TypeAnnotation) {

	depth := checker.valueActivations.Depth()

	for i, parameter := range parameters {
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
			Kind:       common.DeclarationKindParameter,
			IsConstant: true,
			Type:       parameterType,
			Depth:      depth,
			Pos:        &identifier.Pos,
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

	checker.valueActivations.Enter()
	defer checker.valueActivations.Leave()

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
		common.DeclarationKindConstant,
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
	functionType := checker.functionType(expression.Parameters, expression.ReturnTypeAnnotation)

	checker.Elaboration.FunctionExpressionFunctionType[expression] = functionType

	checker.checkFunction(
		expression.Parameters,
		expression.ReturnTypeAnnotation.StartPos,
		functionType,
		expression.FunctionBlock,
		true,
	)

	// function expressions are not allowed in conditions

	if checker.inCondition {
		checker.report(
			&FunctionExpressionInConditionError{
				StartPos: expression.StartPosition(),
				EndPos:   expression.EndPosition(),
			},
		)
	}

	return functionType
}
