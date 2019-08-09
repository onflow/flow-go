package sema

import (
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/activations"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/ast"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/common"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/errors"
	"strings"
)

const ArgumentLabelNotRequired = "_"
const InitializerIdentifier = "init"
const SelfIdentifier = "self"

type functionContext struct {
	returnType Type
	loops      int
}

type checkerResult struct {
	Type   Type
	Errors []error
}

type CheckerError struct {
	Errors []error
}

func (e CheckerError) Error() string {
	var sb strings.Builder
	sb.WriteString("Checking failed:\n")
	for _, err := range e.Errors {
		sb.WriteString(err.Error())
		if err, ok := err.(errors.SecondaryError); ok {
			sb.WriteString(". ")
			sb.WriteString(err.SecondaryError())
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func checkerError(errs []error) *CheckerError {
	if errs != nil {
		return &CheckerError{errs}
	}
	return nil
}

// Checker

type Checker struct {
	Program          *ast.Program
	valueActivations *activations.Activations
	typeActivations  *activations.Activations
	functionContexts []*functionContext
	Globals          map[string]*Variable
}

func NewChecker(program *ast.Program) *Checker {
	typeActivations := &activations.Activations{}
	typeActivations.Push(baseTypes)

	return &Checker{
		Program:          program,
		valueActivations: &activations.Activations{},
		typeActivations:  typeActivations,
		Globals:          map[string]*Variable{},
	}
}

func (checker *Checker) Check() (err error) {
	result := checker.Program.Accept(checker).(checkerResult)
	return checkerError(result.Errors)
}

func (checker *Checker) IsSubType(subType Type, superType Type) bool {
	if subType.Equal(superType) {
		return true
	}

	if superType.Equal(&AnyType{}) {
		return true
	}

	if _, ok := superType.(*IntegerType); ok {
		switch subType.(type) {
		case *IntType,
			*Int8Type, *Int16Type, *Int32Type, *Int64Type,
			*UInt8Type, *UInt16Type, *UInt32Type, *UInt64Type:

			return true

		default:
			return false
		}
	}

	// TODO: functions

	return false
}

func (checker *Checker) IndexableElementType(ty Type) Type {
	switch ty := ty.(type) {
	case ArrayType:
		return ty.elementType()
	}

	return nil
}

func (checker *Checker) IsIndexingType(indexingType Type, indexedType Type) bool {
	switch indexedType.(type) {
	// arrays can be index with integers
	case ArrayType:
		return checker.IsSubType(indexingType, &IntegerType{})
	}

	return false
}

func (checker *Checker) setVariable(name string, variable *Variable) {
	checker.valueActivations.Set(name, variable)
}

func (checker *Checker) setType(name string, ty Type) {
	checker.typeActivations.Set(name, ty)
}

func (checker *Checker) findVariable(name string) *Variable {
	value := checker.valueActivations.Find(name)
	if value == nil {
		return nil
	}
	variable, ok := value.(*Variable)
	if !ok {
		return nil
	}
	return variable
}

func (checker *Checker) findType(name string) Type {
	value := checker.typeActivations.Find(name)
	if value == nil {
		return nil
	}
	ty, ok := value.(Type)
	if !ok {
		return nil
	}
	return ty
}

func (checker *Checker) pushActivations() {
	checker.valueActivations.PushCurrent()
	checker.typeActivations.PushCurrent()
}

func (checker *Checker) popActivations() {
	checker.valueActivations.Pop()
	checker.typeActivations.Pop()
}

func (checker *Checker) VisitProgram(program *ast.Program) ast.Repr {
	var errs []error

	for _, declaration := range program.Declarations {

		result := declaration.Accept(checker).(checkerResult)
		errs = append(errs, result.Errors...)

		if err := checker.declareGlobal(declaration); err != nil {
			// NOTE: append, don't return
			errs = append(errs, err.Errors...)
		}
	}

	return checkerResult{
		Type:   nil,
		Errors: errs,
	}
}

func (checker *Checker) VisitFunctionDeclaration(declaration *ast.FunctionDeclaration) ast.Repr {
	var errs []error

	switch declaration.Access {
	case ast.AccessNotSpecified, ast.AccessPublic:
		break
	default:
		errs = append(errs,
			&InvalidAccessModifierError{
				DeclarationKind: common.DeclarationKindFunction,
				Access:          declaration.Access,
				Pos:             declaration.StartPos,
			},
		)
	}

	functionType, err := checker.functionType(declaration.Parameters, declaration.ReturnType)
	if err != nil {
		// NOTE: append, don't return
		errs = append(errs, err.Errors...)
	}

	argumentLabels := checker.argumentLabels(declaration.Parameters)

	// declare the function before checking it,
	// so it can be referred to inside the function

	if err := checker.declareFunction(
		declaration.Identifier,
		declaration.IdentifierPos,
		functionType,
		argumentLabels,
	); err != nil {
		// NOTE: append, don't return
		errs = append(errs, err.Errors...)
	}

	// check the function after declaring,
	// so it can be referred to inside the function

	if err := checker.checkFunction(
		declaration.Parameters,
		functionType,
		declaration.Block,
	); err != nil {
		// NOTE: append, don't return
		errs = append(errs, err.Errors...)
	}

	return checkerResult{
		Type:   nil,
		Errors: errs,
	}
}

func (checker *Checker) argumentLabels(parameters []*ast.Parameter) []string {
	argumentLabels := make([]string, len(parameters))

	for i, parameter := range parameters {
		argumentLabel := parameter.Label
		// if no argument label is given, the parameter name
		// is used as the argument labels and is required
		if argumentLabel == "" {
			argumentLabel = parameter.Identifier
		}
		argumentLabels[i] = argumentLabel
	}

	return argumentLabels
}

func (checker *Checker) checkFunction(
	parameters []*ast.Parameter,
	functionType *FunctionType,
	block *ast.Block,
) *CheckerError {
	var errs []error

	checker.pushActivations()
	defer checker.popActivations()

	// check argument labels
	if err := checker.checkArgumentLabels(parameters); err != nil {
		// NOTE: append, don't return
		errs = append(errs, err.Errors...)
	}

	if err := checker.declareParameters(parameters, functionType.ParameterTypes); err != nil {
		// NOTE: append, don't return
		errs = append(errs, err.Errors...)
	}

	func() {
		// check the function's block
		checker.enterFunction(functionType)
		defer checker.leaveFunction()

		result := block.Accept(checker).(checkerResult)
		errs = append(errs, result.Errors...)
	}()

	return checkerError(errs)
}

// checkArgumentLabels checks that all argument labels (if any) are unique
//
func (checker *Checker) checkArgumentLabels(parameters []*ast.Parameter) *CheckerError {
	var errs []error
	argumentLabelPositions := map[string]*ast.Position{}

	for _, parameter := range parameters {
		label := parameter.Label
		if label == "" || label == ArgumentLabelNotRequired {
			continue
		}

		if previousPos, ok := argumentLabelPositions[label]; ok {
			errs = append(errs,
				&RedeclarationError{
					Kind:        common.DeclarationKindArgumentLabel,
					Name:        label,
					Pos:         parameter.LabelPos,
					PreviousPos: previousPos,
				},
			)
		}

		argumentLabelPositions[label] = parameter.LabelPos
	}

	return checkerError(errs)
}

// declareParameters declares a constant for each parameter,
// ensuring names are unique and constants don't already exist
//
func (checker *Checker) declareParameters(parameters []*ast.Parameter, parameterTypes []Type) *CheckerError {
	var errs []error

	depth := checker.valueActivations.Depth()

	for i, parameter := range parameters {
		identifier := parameter.Identifier

		// check if variable with this identifier is already declared in the current scope
		existingVariable := checker.findVariable(identifier)
		if existingVariable != nil && existingVariable.Depth == depth {
			errs = append(errs,
				&RedeclarationError{
					Kind:        common.DeclarationKindParameter,
					Name:        identifier,
					Pos:         parameter.IdentifierPos,
					PreviousPos: existingVariable.Pos,
				},
			)

			continue
		}

		parameterType := parameterTypes[i]

		checker.setVariable(
			identifier,
			&Variable{
				IsConstant: true,
				Type:       parameterType,
				Depth:      depth,
				Pos:        parameter.IdentifierPos,
			},
		)
	}

	return checkerError(errs)
}

func (checker *Checker) VisitVariableDeclaration(declaration *ast.VariableDeclaration) ast.Repr {
	valueResult := declaration.Value.Accept(checker).(checkerResult)
	valueType := valueResult.Type
	errs := valueResult.Errors

	declarationType := valueType
	// does the declaration have an explicit type annotation?
	if declaration.Type != nil {
		var err *CheckerError
		declarationType, err = checker.ConvertType(declaration.Type)
		if err != nil {
			// NOTE: append, don't return
			errs = append(errs, err.Errors...)
		}

		// check the value type is a subtype of the declaration type
		if declarationType != nil &&
			valueType != nil &&
			!checker.IsSubType(valueType, declarationType) {

			errs = append(errs,
				&TypeMismatchError{
					ExpectedType: declarationType,
					ActualType:   valueType,
					StartPos:     declaration.Value.StartPosition(),
					EndPos:       declaration.Value.EndPosition(),
				},
			)
		}
	}

	if err := checker.declareVariable(declaration, declarationType); err != nil {
		// NOTE: append, don't return
		errs = append(errs, err.Errors...)
	}

	return checkerResult{
		Type:   nil,
		Errors: errs,
	}
}

func (checker *Checker) declareVariable(declaration *ast.VariableDeclaration, ty Type) *CheckerError {
	var errs []error

	identifier := declaration.Identifier

	// check if variable with this name is already declared in the current scope
	existingVariable := checker.findVariable(identifier)
	depth := checker.valueActivations.Depth()
	if existingVariable != nil && existingVariable.Depth == depth {
		errs = append(errs,
			&RedeclarationError{
				Kind:        declaration.DeclarationKind(),
				Name:        identifier,
				Pos:         declaration.IdentifierPosition(),
				PreviousPos: existingVariable.Pos,
			},
		)
	}

	// variable with this name is not declared in current scope, declare it
	checker.setVariable(
		identifier,
		&Variable{
			IsConstant: declaration.IsConstant,
			Depth:      depth,
			Type:       ty,
			Pos:        declaration.IdentifierPos,
		},
	)

	return checkerError(errs)
}

func (checker *Checker) declareGlobal(declaration ast.Declaration) *CheckerError {
	var errs []error

	name := declaration.DeclarationName()
	checker.Globals[name] = checker.findVariable(name)

	return checkerError(errs)
}

func (checker *Checker) VisitBlock(block *ast.Block) ast.Repr {
	var errs []error

	checker.pushActivations()
	defer checker.popActivations()

	// check all statements

	for _, statement := range block.Statements {

		// check statement is not a local structure declaration

		if _, ok := statement.(*ast.StructureDeclaration); ok {
			errs = append(errs, &InvalidDeclarationError{
				Kind:     common.DeclarationKindStructure,
				StartPos: statement.StartPosition(),
				EndPos:   statement.EndPosition(),
			})

			continue
		}

		// check statement

		result := statement.Accept(checker).(checkerResult)
		errs = append(errs, result.Errors...)
	}

	return checkerResult{
		Type:   nil,
		Errors: errs,
	}
}

func (checker *Checker) VisitReturnStatement(statement *ast.ReturnStatement) ast.Repr {
	var errs []error

	// check value type matches enclosing function's return type

	if statement.Expression == nil {
		return checkerResult{
			Type:   nil,
			Errors: nil,
		}
	}

	valueResult := statement.Expression.Accept(checker).(checkerResult)
	errs = append(errs, valueResult.Errors...)

	valueType := valueResult.Type
	returnType := checker.currentFunction().returnType

	if valueType != nil && !checker.IsSubType(valueType, returnType) {
		errs = append(errs,
			&TypeMismatchError{
				ExpectedType: returnType,
				ActualType:   valueType,
				StartPos:     statement.Expression.StartPosition(),
				EndPos:       statement.Expression.EndPosition(),
			},
		)
	}

	return checkerResult{
		Type:   nil,
		Errors: errs,
	}
}

func (checker *Checker) VisitBreakStatement(statement *ast.BreakStatement) ast.Repr {
	var errs []error

	// check statement is inside loop
	if checker.currentFunction().loops == 0 {
		errs = append(errs,
			&ControlStatementError{
				ControlStatement: common.ControlStatementBreak,
				StartPos:         statement.StartPos,
				EndPos:           statement.EndPos,
			},
		)
	}

	return checkerResult{
		Type:   nil,
		Errors: errs,
	}
}

func (checker *Checker) VisitContinueStatement(statement *ast.ContinueStatement) ast.Repr {
	var errs []error

	// check statement is inside loop
	if checker.currentFunction().loops == 0 {
		errs = append(errs,
			&ControlStatementError{
				ControlStatement: common.ControlStatementContinue,
				StartPos:         statement.StartPos,
				EndPos:           statement.EndPos,
			},
		)
	}

	return checkerResult{
		Type:   nil,
		Errors: errs,
	}
}

func (checker *Checker) VisitIfStatement(statement *ast.IfStatement) ast.Repr {
	var errs []error

	var elseElement ast.Element = ast.NotAnElement{}
	if statement.Else != nil {
		elseElement = statement.Else
	}

	if _, _, err := checker.visitConditional(statement.Test, statement.Then, elseElement); err != nil {
		// NOTE: append, don't return
		errs = append(errs, err.Errors...)
	}

	return checkerResult{
		Type:   nil,
		Errors: errs,
	}
}

func (checker *Checker) VisitWhileStatement(statement *ast.WhileStatement) ast.Repr {
	var errs []error

	testExpression := statement.Test
	testResult := testExpression.Accept(checker).(checkerResult)
	errs = append(errs, testResult.Errors...)

	testType := testResult.Type

	if !checker.IsSubType(testType, &BoolType{}) {
		errs = append(errs,
			&TypeMismatchError{
				ExpectedType: &BoolType{},
				ActualType:   testType,
				StartPos:     testExpression.StartPosition(),
				EndPos:       testExpression.EndPosition(),
			},
		)
	}

	checker.currentFunction().loops += 1
	defer func() {
		checker.currentFunction().loops -= 1
	}()

	blockResult := statement.Block.Accept(checker).(checkerResult)
	errs = append(errs, blockResult.Errors...)

	return checkerResult{
		Type:   nil,
		Errors: errs,
	}
}

func (checker *Checker) VisitAssignment(assignment *ast.AssignmentStatement) ast.Repr {
	var errs []error

	valueResult := assignment.Value.Accept(checker).(checkerResult)
	errs = append(errs, valueResult.Errors...)
	valueType := valueResult.Type

	if err := checker.visitAssignmentValueType(assignment, valueType); err != nil {
		// NOTE: append, don't return
		errs = append(errs, err.Errors...)
	}

	return checkerResult{
		Type:   nil,
		Errors: errs,
	}
}

func (checker *Checker) visitAssignmentValueType(assignment *ast.AssignmentStatement, valueType Type) *CheckerError {
	switch target := assignment.Target.(type) {
	case *ast.IdentifierExpression:
		return checker.visitIdentifierExpressionAssignment(assignment, target, valueType)

	case *ast.IndexExpression:
		return checker.visitIndexExpressionAssignment(assignment, target, valueType)

	case *ast.MemberExpression:
		return checker.visitMemberExpressionAssignment(assignment, target, valueType)

	default:
		panic(&unsupportedAssignmentTargetExpression{
			target: target,
		})
	}

	panic(&errors.UnreachableError{})
}

func (checker *Checker) visitIdentifierExpressionAssignment(
	assignment *ast.AssignmentStatement,
	target *ast.IdentifierExpression,
	valueType Type,
) *CheckerError {
	var errs []error

	identifier := target.Identifier

	// check identifier was declared before
	variable := checker.findVariable(identifier)
	if variable == nil {
		errs = append(errs,
			&NotDeclaredError{
				ExpectedKind: common.DeclarationKindVariable,
				Name:         identifier,
				Pos:          target.StartPosition(),
			},
		)
	} else {
		// check identifier is not a constant
		if variable.IsConstant {
			errs = append(errs,
				&AssignmentToConstantError{
					Name:     identifier,
					StartPos: target.StartPosition(),
					EndPos:   target.EndPosition(),
				},
			)
		}

		// check value type is subtype of variable type
		if !checker.IsSubType(valueType, variable.Type) {
			errs = append(errs,
				&TypeMismatchError{
					ExpectedType: variable.Type,
					ActualType:   valueType,
					StartPos:     assignment.Value.StartPosition(),
					EndPos:       assignment.Value.EndPosition(),
				},
			)
		}
	}

	return checkerError(errs)
}

func (checker *Checker) visitIndexExpressionAssignment(
	assignment *ast.AssignmentStatement,
	target *ast.IndexExpression,
	valueType Type,
) *CheckerError {
	var errs []error

	elementResult := checker.visitIndexingExpression(target.Expression, target.Index)
	errs = append(errs, elementResult.Errors...)

	elementType := elementResult.Type

	if elementType != nil && !checker.IsSubType(valueType, elementType) {
		errs = append(errs,
			&TypeMismatchError{
				ExpectedType: elementType,
				ActualType:   valueType,
				StartPos:     assignment.Value.StartPosition(),
				EndPos:       assignment.Value.EndPosition(),
			},
		)
	}

	return checkerError(errs)
}

func (checker *Checker) visitMemberExpressionAssignment(
	assignment *ast.AssignmentStatement,
	target *ast.MemberExpression,
	valueType Type,
) *CheckerError {
	var errs []error

	member, err := checker.visitMember(target)
	if err != nil {
		errs = append(errs, err.Errors...)
	}

	if member != nil {
		// check member is not constant

		if member.IsConstant {
			if member.IsInitialized {
				errs = append(errs,
					&AssignmentToConstantMemberError{
						Name:     target.Identifier,
						StartPos: assignment.Value.StartPosition(),
						EndPos:   assignment.Value.EndPosition(),
					},
				)
			}
		}

		member.IsInitialized = true

		// check value can be assigned to member
		if !checker.IsSubType(valueType, member.Type) {
			errs = append(errs,
				&TypeMismatchError{
					ExpectedType: member.Type,
					ActualType:   valueType,
					StartPos:     assignment.Value.StartPosition(),
					EndPos:       assignment.Value.EndPosition(),
				},
			)
		}
	}

	return checkerError(errs)
}

// visitIndexingExpression checks if the indexed expression is indexable,
// checks if the indexing expression can be used to index into the indexed expression,
// and returns the expected element type
//
func (checker *Checker) visitIndexingExpression(indexedExpression, indexingExpression ast.Expression) checkerResult {
	var errs []error

	indexedResult := indexedExpression.Accept(checker).(checkerResult)
	errs = append(errs, indexedResult.Errors...)
	indexedType := indexedResult.Type

	indexingResult := indexingExpression.Accept(checker).(checkerResult)
	errs = append(errs, indexingResult.Errors...)
	indexingType := indexingResult.Type

	// NOTE: check indexed type first for UX reasons

	// check indexed expression's type is indexable
	// by getting the expected element

	elementType := checker.IndexableElementType(indexedType)
	if elementType == nil {
		errs = append(errs,
			&NotIndexableTypeError{
				Type:     indexedType,
				StartPos: indexedExpression.StartPosition(),
				EndPos:   indexedExpression.EndPosition(),
			},
		)
	} else {

		// check indexing expression's type can be used to index
		// into indexed expression's type

		if !checker.IsIndexingType(indexingType, indexedType) {
			errs = append(errs,
				&NotIndexingTypeError{
					Type:     indexingType,
					StartPos: indexingExpression.StartPosition(),
					EndPos:   indexingExpression.EndPosition(),
				},
			)
		}
	}

	return checkerResult{
		Type:   elementType,
		Errors: errs,
	}
}

func (checker *Checker) VisitIdentifierExpression(expression *ast.IdentifierExpression) ast.Repr {
	variable, err := checker.findAndCheckVariable(expression)
	if err != nil {
		return checkerResult{
			// TODO: verify this OK
			Type:   &AnyType{},
			Errors: []error{err},
		}
	}

	return checkerResult{
		Type:   variable.Type,
		Errors: nil,
	}
}

func (checker *Checker) findAndCheckVariable(expression *ast.IdentifierExpression) (*Variable, error) {
	variable := checker.findVariable(expression.Identifier)
	if variable == nil {
		return nil, &NotDeclaredError{
			ExpectedKind: common.DeclarationKindValue,
			Name:         expression.Identifier,
			Pos:          expression.StartPosition(),
		}
	}

	return variable, nil
}

func (checker *Checker) visitBinaryOperation(expr *ast.BinaryExpression) (left, right checkerResult) {
	left = expr.Left.Accept(checker).(checkerResult)
	right = expr.Right.Accept(checker).(checkerResult)
	return
}

// TODO: split up

func (checker *Checker) VisitBinaryExpression(expression *ast.BinaryExpression) ast.Repr {
	var errs []error

	leftResult, rightResult := checker.visitBinaryOperation(expression)
	errs = append(errs, leftResult.Errors...)
	errs = append(errs, rightResult.Errors...)

	leftType := leftResult.Type
	rightType := rightResult.Type

	operation := expression.Operation
	operationKind := binaryOperationKind(operation)

	switch operationKind {
	case BinaryOperationKindIntegerArithmetic,
		BinaryOperationKindIntegerComparison:

		// check both types are integer subtypes

		leftIsInteger := checker.IsSubType(leftType, &IntegerType{})
		rightIsInteger := checker.IsSubType(rightType, &IntegerType{})

		if !leftIsInteger && !rightIsInteger {
			errs = append(errs,
				&InvalidBinaryOperandsError{
					Operation: operation,
					LeftType:  leftType,
					RightType: rightType,
					StartPos:  expression.StartPosition(),
					EndPos:    expression.EndPosition(),
				},
			)
		} else if !leftIsInteger {
			errs = append(errs,
				&InvalidBinaryOperandError{
					Operation:    operation,
					Side:         common.OperandSideLeft,
					ExpectedType: &IntegerType{},
					ActualType:   leftType,
					StartPos:     expression.Left.StartPosition(),
					EndPos:       expression.Left.EndPosition(),
				},
			)
		} else if !rightIsInteger {
			errs = append(errs,
				&InvalidBinaryOperandError{
					Operation:    operation,
					Side:         common.OperandSideRight,
					ExpectedType: &IntegerType{},
					ActualType:   rightType,
					StartPos:     expression.Right.StartPosition(),
					EndPos:       expression.Right.EndPosition(),
				},
			)
		}

		// check both types are equal

		if !leftType.Equal(rightType) {
			errs = append(errs,
				&InvalidBinaryOperandsError{
					Operation: operation,
					LeftType:  leftType,
					RightType: rightType,
					StartPos:  expression.StartPosition(),
					EndPos:    expression.EndPosition(),
				},
			)
		}

		switch operationKind {
		case BinaryOperationKindIntegerArithmetic:
			return checkerResult{
				Type:   leftType,
				Errors: errs,
			}
		case BinaryOperationKindIntegerComparison:
			return checkerResult{
				Type:   &BoolType{},
				Errors: errs,
			}
		}

		panic(&errors.UnreachableError{})

	case BinaryOperationKindEquality:
		// check both types are equal, and boolean subtypes or integer subtypes

		if !(leftType.Equal(rightType) &&
			(checker.IsSubType(leftType, &BoolType{}) || checker.IsSubType(leftType, &IntegerType{}))) {

			errs = append(errs,
				&InvalidBinaryOperandsError{
					Operation: operation,
					LeftType:  leftType,
					RightType: rightType,
					StartPos:  expression.StartPosition(),
					EndPos:    expression.EndPosition(),
				},
			)
		}

		return checkerResult{
			Type:   &BoolType{},
			Errors: errs,
		}

	case BinaryOperationKindBooleanLogic:

		// check both types are integer subtypes

		leftIsBool := checker.IsSubType(leftType, &BoolType{})
		rightIsBool := checker.IsSubType(rightType, &BoolType{})

		if !leftIsBool && !rightIsBool {
			errs = append(errs,
				&InvalidBinaryOperandsError{
					Operation: operation,
					LeftType:  leftType,
					RightType: rightType,
					StartPos:  expression.StartPosition(),
					EndPos:    expression.EndPosition(),
				},
			)
		} else if !leftIsBool {
			errs = append(errs,
				&InvalidBinaryOperandError{
					Operation:    operation,
					Side:         common.OperandSideLeft,
					ExpectedType: &BoolType{},
					ActualType:   leftType,
					StartPos:     expression.Left.StartPosition(),
					EndPos:       expression.Left.EndPosition(),
				},
			)
		} else if !rightIsBool {
			errs = append(errs,
				&InvalidBinaryOperandError{
					Operation:    operation,
					Side:         common.OperandSideRight,
					ExpectedType: &BoolType{},
					ActualType:   rightType,
					StartPos:     expression.Right.StartPosition(),
					EndPos:       expression.Right.EndPosition(),
				},
			)
		}

		return checkerResult{
			Type:   &BoolType{},
			Errors: errs,
		}
	}

	panic(&unsupportedOperation{
		kind:      common.OperationKindBinary,
		operation: operation,
		startPos:  expression.StartPosition(),
		endPos:    expression.EndPosition(),
	})
}

func (checker *Checker) VisitUnaryExpression(expression *ast.UnaryExpression) ast.Repr {
	var errs []error

	valueResult := expression.Expression.Accept(checker).(checkerResult)
	errs = append(errs, valueResult.Errors...)
	valueType := valueResult.Type

	switch expression.Operation {
	case ast.OperationNegate:
		if !checker.IsSubType(valueType, &BoolType{}) {
			errs = append(errs,
				&InvalidUnaryOperandError{
					Operation:    expression.Operation,
					ExpectedType: &BoolType{},
					ActualType:   valueType,
					StartPos:     expression.Expression.StartPosition(),
					EndPos:       expression.Expression.EndPosition(),
				},
			)
		}
		return checkerResult{
			Type:   valueType,
			Errors: errs,
		}

	case ast.OperationMinus:
		if !checker.IsSubType(valueType, &IntegerType{}) {
			errs = append(errs,
				&InvalidUnaryOperandError{
					Operation:    expression.Operation,
					ExpectedType: &IntegerType{},
					ActualType:   valueType,
					StartPos:     expression.Expression.StartPosition(),
					EndPos:       expression.Expression.EndPosition(),
				},
			)
		}
		return checkerResult{
			Type:   valueType,
			Errors: errs,
		}
	}

	panic(&unsupportedOperation{
		kind:      common.OperationKindUnary,
		operation: expression.Operation,
		startPos:  expression.StartPos,
		endPos:    expression.EndPos,
	})
}

func (checker *Checker) VisitExpressionStatement(statement *ast.ExpressionStatement) ast.Repr {
	return statement.Expression.Accept(checker).(checkerResult)
}

func (checker *Checker) VisitBoolExpression(expression *ast.BoolExpression) ast.Repr {
	return checkerResult{
		Type:   &BoolType{},
		Errors: nil,
	}
}

func (checker *Checker) VisitIntExpression(expression *ast.IntExpression) ast.Repr {
	return checkerResult{
		Type:   &IntType{},
		Errors: nil,
	}
}

func (checker *Checker) VisitArrayExpression(expression *ast.ArrayExpression) ast.Repr {
	var errs []error

	// visit all elements, ensure they are all the same type

	var elementType Type

	for _, value := range expression.Values {
		valueResult := value.Accept(checker).(checkerResult)
		errs = append(errs, valueResult.Errors...)
		valueType := valueResult.Type

		// infer element type from first element
		// TODO: find common super type?
		if elementType == nil {
			elementType = valueType
		} else if !checker.IsSubType(valueType, elementType) {
			errs = append(errs,
				&TypeMismatchError{
					ExpectedType: elementType,
					ActualType:   valueType,
					StartPos:     value.StartPosition(),
					EndPos:       value.EndPosition(),
				},
			)
		}
	}

	// TODO: use bottom type
	//if elementType == nil {
	//
	//}

	arrayType := &ConstantSizedType{
		Size: len(expression.Values),
		Type: elementType,
	}

	return checkerResult{
		Type:   arrayType,
		Errors: errs,
	}
}

func (checker *Checker) VisitMemberExpression(expression *ast.MemberExpression) ast.Repr {
	var errs []error

	member, err := checker.visitMember(expression)
	if err != nil {
		errs = append(errs, err.Errors...)
	}

	var memberType Type = &AnyType{}
	if member != nil {
		memberType = member.Type
	}

	return checkerResult{
		Type:   memberType,
		Errors: errs,
	}
}

func (checker *Checker) visitMember(expression *ast.MemberExpression) (*Member, *CheckerError) {
	var errs []error

	result := expression.Expression.Accept(checker).(checkerResult)
	errs = append(errs, result.Errors...)

	identifier := expression.Identifier

	var member *Member
	structureType, ok := result.Type.(*StructureType)
	if ok {
		member, ok = structureType.Members[identifier]
	}

	if !ok {
		errs = append(errs,
			&NotDeclaredMemberError{
				Type:     result.Type,
				Name:     identifier,
				StartPos: expression.StartPos,
				EndPos:   expression.EndPos,
			},
		)
	}

	return member, checkerError(errs)
}

func (checker *Checker) VisitIndexExpression(expression *ast.IndexExpression) ast.Repr {
	return checker.visitIndexingExpression(expression.Expression, expression.Index)
}

func (checker *Checker) VisitConditionalExpression(expression *ast.ConditionalExpression) ast.Repr {
	var errs []error

	thenType, elseType, err := checker.visitConditional(expression.Test, expression.Then, expression.Else)
	if err != nil {
		// NOTE: append, don't return
		errs = append(errs, err.Errors...)
	}

	if thenType == nil || elseType == nil {
		panic(&errors.UnreachableError{})
	}

	// TODO: improve
	resultType := thenType

	if !checker.IsSubType(elseType, resultType) {
		errs = append(errs,
			&TypeMismatchError{
				ExpectedType: resultType,
				ActualType:   elseType,
				StartPos:     expression.Else.StartPosition(),
				EndPos:       expression.Else.EndPosition(),
			},
		)
	}

	return checkerResult{
		Type:   resultType,
		Errors: errs,
	}
}

func (checker *Checker) VisitInvocationExpression(invocationExpression *ast.InvocationExpression) ast.Repr {
	var errs []error

	// check the invoked expression can be invoked

	invokedExpression := invocationExpression.Expression
	expressionResult := invokedExpression.Accept(checker).(checkerResult)
	errs = append(errs, expressionResult.Errors...)

	expressionType := expressionResult.Type

	var returnType Type
	functionType, ok := expressionType.(*FunctionType)
	if !ok {
		errs = append(errs,
			&NotCallableError{
				Type:     expressionType,
				StartPos: invokedExpression.StartPosition(),
				EndPos:   invokedExpression.EndPosition(),
			},
		)
	} else {
		// invoked expression has function type

		if err := checker.checkInvocationArguments(invocationExpression, functionType); err != nil {
			errs = append(errs, err.Errors...)
		}

		// if the invocation refers directly to the name of the function as stated in the declaration,
		// the argument labels need to be supplied

		if identifierExpression, ok := invokedExpression.(*ast.IdentifierExpression); ok {
			if err := checker.checkInvocationArgumentLabels(
				invocationExpression,
				identifierExpression,
			); err != nil {
				errs = append(errs, err.Errors...)
			}
		}

		returnType = functionType.ReturnType
	}

	return checkerResult{
		Type:   returnType,
		Errors: errs,
	}
}

func (checker *Checker) checkInvocationArgumentLabels(
	invocationExpression *ast.InvocationExpression,
	identifierExpression *ast.IdentifierExpression,
) *CheckerError {
	var errs []error

	variable, err := checker.findAndCheckVariable(identifierExpression)
	if err != nil {
		errs = append(errs, err)
	} else if variable != nil {
		if variable.ArgumentLabels != nil {
			argumentCount := len(invocationExpression.Arguments)

			for i, argumentLabel := range variable.ArgumentLabels {
				if i >= argumentCount {
					break
				}

				argument := invocationExpression.Arguments[i]
				providedLabel := argument.Label
				if argumentLabel == ArgumentLabelNotRequired {
					// argument label is not required,
					// check it is not provided

					if providedLabel != "" {
						errs = append(errs,
							&IncorrectArgumentLabelError{
								ActualArgumentLabel:   providedLabel,
								ExpectedArgumentLabel: "",
								StartPos:              argument.Expression.StartPosition(),
								EndPos:                argument.Expression.EndPosition(),
							},
						)
					}
				} else {
					// argument label is required,
					// check it is provided and correct
					if providedLabel == "" {
						errs = append(errs,
							&MissingArgumentLabelError{
								ExpectedArgumentLabel: argumentLabel,
								StartPos:              argument.Expression.StartPosition(),
								EndPos:                argument.Expression.EndPosition(),
							},
						)
					} else if providedLabel != argumentLabel {
						errs = append(errs,
							&IncorrectArgumentLabelError{
								ActualArgumentLabel:   providedLabel,
								ExpectedArgumentLabel: argumentLabel,
								StartPos:              argument.Expression.StartPosition(),
								EndPos:                argument.Expression.EndPosition(),
							},
						)
					}
				}
			}
		}
	}

	return checkerError(errs)
}

func (checker *Checker) checkInvocationArguments(
	invocationExpression *ast.InvocationExpression,
	functionType *FunctionType,
) *CheckerError {
	var errs []error

	argumentCount := len(invocationExpression.Arguments)

	// check the invocation's argument count matches the function's parameter count
	parameterCount := len(functionType.ParameterTypes)
	if argumentCount != parameterCount {
		errs = append(errs,
			&ArgumentCountError{
				ParameterCount: parameterCount,
				ArgumentCount:  argumentCount,
				StartPos:       invocationExpression.StartPos,
				EndPos:         invocationExpression.EndPos,
			},
		)
	}

	minCount := argumentCount
	if parameterCount < argumentCount {
		minCount = parameterCount
	}

	for i := 0; i < minCount; i++ {
		// ensure the type of the argument matches the type of the parameter

		parameterType := functionType.ParameterTypes[i]
		argument := invocationExpression.Arguments[i]

		argumentResult := argument.Expression.Accept(checker).(checkerResult)
		errs = append(errs, argumentResult.Errors...)
		argumentType := argumentResult.Type

		if argumentType != nil && !checker.IsSubType(argumentType, parameterType) {
			errs = append(errs,
				&TypeMismatchError{
					ExpectedType: parameterType,
					ActualType:   argumentType,
					StartPos:     argument.Expression.StartPosition(),
					EndPos:       argument.Expression.EndPosition(),
				},
			)
		}
	}

	return checkerError(errs)
}

func (checker *Checker) VisitFunctionExpression(expression *ast.FunctionExpression) ast.Repr {
	var errs []error

	// TODO: infer
	functionType, err := checker.functionType(expression.Parameters, expression.ReturnType)
	if err != nil {
		// NOTE: append, don't return
		errs = append(errs, err.Errors...)
	}

	if err := checker.checkFunction(
		expression.Parameters,
		functionType,
		expression.Block,
	); err != nil {
		// NOTE: append, don't return
		errs = append(errs, err.Errors...)
	}

	return checkerResult{
		Type:   functionType,
		Errors: errs,
	}
}

// ConvertType converts an AST type representation to a sema type
func (checker *Checker) ConvertType(t ast.Type) (Type, *CheckerError) {
	var errs []error

	switch t := t.(type) {
	case *ast.NominalType:
		result := checker.findType(t.Identifier)
		if result == nil {
			err := &CheckerError{
				Errors: []error{
					&NotDeclaredError{
						ExpectedKind: common.DeclarationKindType,
						Name:         t.Identifier,
						Pos:          t.Pos,
					},
				},
			}
			return &AnyType{}, err
		}
		return result, nil

	case *ast.VariableSizedType:
		elementType, err := checker.ConvertType(t.Type)
		if err != nil {
			// NOTE: append, don't return
			errs = append(errs, err.Errors...)
		}

		return &VariableSizedType{
			Type: elementType,
		}, checkerError(errs)

	case *ast.ConstantSizedType:
		elementType, err := checker.ConvertType(t.Type)
		if err != nil {
			// NOTE: append, don't return
			errs = append(errs, err.Errors...)
		}

		return &ConstantSizedType{
			Type: elementType,
			Size: t.Size,
		}, checkerError(errs)

	case *ast.FunctionType:
		var parameterTypes []Type
		for _, parameterType := range t.ParameterTypes {
			parameterType, err := checker.ConvertType(parameterType)
			if err != nil {
				// NOTE: append, don't return
				errs = append(errs, err.Errors...)
			}
			// NOTE: still append parameter type, even if there's an error
			parameterTypes = append(parameterTypes, parameterType)
		}

		returnType, err := checker.ConvertType(t.ReturnType)
		if err != nil {
			// NOTE: append, don't return
			errs = append(errs, err.Errors...)
		}

		return &FunctionType{
			ParameterTypes: parameterTypes,
			ReturnType:     returnType,
		}, checkerError(errs)
	}

	panic(&astTypeConversionError{invalidASTType: t})
}

func (checker *Checker) declareFunction(
	identifier string,
	identifierPosition *ast.Position,
	functionType *FunctionType,
	argumentLabels []string,
) *CheckerError {
	var errs []error

	// check if variable with this identifier is already declared in the current scope
	existingVariable := checker.findVariable(identifier)
	depth := checker.valueActivations.Depth()
	if existingVariable != nil && existingVariable.Depth == depth {
		errs = append(errs,
			&RedeclarationError{
				Kind:        common.DeclarationKindFunction,
				Name:        identifier,
				Pos:         identifierPosition,
				PreviousPos: existingVariable.Pos,
			},
		)
	}

	// variable with this identifier is not declared in current scope, declare it
	checker.setVariable(
		identifier,
		&Variable{
			IsConstant:     true,
			Depth:          depth,
			Type:           functionType,
			ArgumentLabels: argumentLabels,
			Pos:            identifierPosition,
		},
	)

	return checkerError(errs)
}

func (checker *Checker) enterFunction(functionType *FunctionType) {
	checker.functionContexts = append(checker.functionContexts,
		&functionContext{
			returnType: functionType.ReturnType,
		})
}

func (checker *Checker) leaveFunction() {
	lastIndex := len(checker.functionContexts) - 1
	checker.functionContexts = checker.functionContexts[:lastIndex]
}

func (checker *Checker) currentFunction() *functionContext {
	lastIndex := len(checker.functionContexts) - 1
	if lastIndex < 0 {
		return nil
	}
	return checker.functionContexts[lastIndex]
}

func (checker *Checker) functionType(parameters []*ast.Parameter, returnType ast.Type) (*FunctionType, *CheckerError) {
	var errs []error

	parameterTypes, err := checker.parameterTypes(parameters)
	if err != nil {
		// NOTE: append, don't return
		errs = append(errs, err.Errors...)
	}

	var convertedReturnType Type = &VoidType{}
	if returnType != nil {
		var err *CheckerError
		convertedReturnType, err = checker.ConvertType(returnType)
		if err != nil {
			// NOTE: append, don't return
			errs = append(errs, err.Errors...)
		}
	}

	return &FunctionType{
		ParameterTypes: parameterTypes,
		ReturnType:     convertedReturnType,
	}, checkerError(errs)
}

func (checker *Checker) parameterTypes(parameters []*ast.Parameter) ([]Type, *CheckerError) {
	var errs []error

	parameterTypes := make([]Type, len(parameters))
	for i, parameter := range parameters {
		parameterType, err := checker.ConvertType(parameter.Type)
		if err != nil {
			// NOTE: append, don't return
			errs = append(errs, err.Errors...)
		}
		// NOTE: still assigning parameter type
		parameterTypes[i] = parameterType
	}

	return parameterTypes, checkerError(errs)
}

// visitConditional checks a conditional. the test expression must be a boolean.
// the then and else elements may be expressions, in which case the types are returned.
func (checker *Checker) visitConditional(
	test ast.Expression,
	thenElement ast.Element,
	elseElement ast.Element,
) (
	thenType, elseType Type, err *CheckerError,
) {
	var errs []error

	testResult := test.Accept(checker).(checkerResult)
	errs = append(errs, testResult.Errors...)

	testType := testResult.Type

	if !checker.IsSubType(testType, &BoolType{}) {
		errs = append(errs,
			&TypeMismatchError{
				ExpectedType: &BoolType{},
				ActualType:   testType,
				StartPos:     test.StartPosition(),
				EndPos:       test.EndPosition(),
			},
		)
	}

	thenResult := thenElement.Accept(checker).(checkerResult)
	errs = append(errs, thenResult.Errors...)

	elseResult, ok := elseElement.Accept(checker).(checkerResult)
	if ok {
		errs = append(errs, elseResult.Errors...)
	}

	return thenResult.Type, elseResult.Type, checkerError(errs)
}

func (checker *Checker) VisitStructureDeclaration(structure *ast.StructureDeclaration) ast.Repr {
	var errs []error

	if err := checker.checkStructureFieldAndFunctionIdentifiers(structure); err != nil {
		errs = append(errs, err.Errors...)
	}

	if err := checker.checkStructureFields(structure.Fields); err != nil {
		errs = append(errs, err.Errors...)
	}

	structureType, err := checker.structureType(structure)
	if err != nil {
		errs = append(errs, err.Errors...)
	}

	if err := checker.declareType(structure.Identifier, structure.IdentifierPos, structureType); err != nil {
		errs = append(errs, err.Errors...)
	}

	// declare the constructor function before checking initializer and functions,
	// so the constructor function can be referred to inside them

	if err := checker.declareStructureConstructor(structure, structureType); err != nil {
		errs = append(errs, err.Errors...)
	}

	// check the initializer

	initializer := structure.Initializer
	if initializer != nil {
		if err := checker.checkStructureInitializer(initializer, structureType); err != nil {
			errs = append(errs, err.Errors...)
		}
	} else if len(structure.Fields) > 0 {
		firstField := structure.Fields[0]

		// structure has fields, but no initializer
		errs = append(errs,
			&MissingInitializerError{
				StructureType:  structureType,
				FirstFieldName: firstField.Identifier,
				FirstFieldPos:  firstField.IdentifierPos,
			},
		)
	}

	// TODO: very simple field initialization check for now.
	//  perform proper definite assignment analysis

	if structureType != nil {
		for _, field := range structure.Fields {
			name := field.Identifier
			member := structureType.Members[name]

			if !member.IsInitialized {
				errs = append(errs,
					&FieldUninitializedError{
						Name:          name,
						Pos:           field.IdentifierPos,
						StructureType: structureType,
					},
				)
			}
		}
	}

	if err := checker.checkStructureFunctions(structure.Functions, structureType); err != nil {
		errs = append(errs, err.Errors...)
	}

	return checkerResult{
		Type:   nil,
		Errors: errs,
	}
}

func (checker *Checker) declareStructureConstructor(
	structure *ast.StructureDeclaration,
	structureType *StructureType,
) *CheckerError {
	var errs []error

	functionType := &FunctionType{
		ReturnType: structureType,
	}
	var argumentLabels []string

	initializer := structure.Initializer
	if initializer != nil {
		// NOTE: IGNORING errors, because initializer will be checked separately
		// in `VisitInitializerDeclaration`, otherwise we get error duplicates

		parameterTypes, _ := checker.parameterTypes(initializer.Parameters)

		functionType = &FunctionType{
			ParameterTypes: parameterTypes,
			ReturnType:     structureType,
		}
	}

	if err := checker.declareFunction(
		structure.Identifier,
		structure.IdentifierPos,
		functionType,
		argumentLabels,
	); err != nil {
		// NOTE: append, don't return
		errs = append(errs, err.Errors...)
	}

	return checkerError(errs)
}

func (checker *Checker) structureType(structure *ast.StructureDeclaration) (*StructureType, *CheckerError) {
	var errs []error

	fieldCount := len(structure.Fields)
	functionCount := len(structure.Functions)

	members := make(map[string]*Member, fieldCount+functionCount)

	// declare a member for each field
	for _, field := range structure.Fields {
		fieldType, err := checker.ConvertType(field.Type)
		if err != nil {
			// NOTE: append, don't return
			errs = append(errs, err.Errors...)
		}

		// NOTE: still declare member
		members[field.Identifier] = &Member{
			Type:          fieldType,
			IsConstant:    field.IsConstant,
			IsInitialized: false,
		}
	}

	// declare a member for each function
	for _, function := range structure.Functions {
		functionType, err := checker.functionType(function.Parameters, function.ReturnType)
		if err != nil {
			// NOTE: append, don't return
			errs = append(errs, err.Errors...)
		}

		// NOTE: still declare member
		members[function.Identifier] = &Member{
			Type:          functionType,
			IsConstant:    true,
			IsInitialized: true,
		}
	}

	return &StructureType{
		Identifier: structure.Identifier,
		Members:    members,
	}, checkerError(errs)
}

func (checker *Checker) checkStructureFields(fields []*ast.FieldDeclaration) *CheckerError {
	var errs []error

	for _, field := range fields {
		result := field.Accept(checker).(checkerResult)
		errs = append(errs, result.Errors...)
	}

	return checkerError(errs)
}

func (checker *Checker) checkStructureInitializer(
	initializer *ast.InitializerDeclaration,
	selfType *StructureType,
) *CheckerError {
	var errs []error

	// NOTE: new activation, so `self`
	// is only visible inside initializer

	checker.valueActivations.PushCurrent()
	defer checker.valueActivations.Pop()

	checker.declareSelf(selfType)

	result := initializer.Accept(checker).(checkerResult)
	errs = append(errs, result.Errors...)

	return checkerError(errs)
}

func (checker *Checker) checkStructureFunctions(
	functions []*ast.FunctionDeclaration,
	selfType *StructureType,
) *CheckerError {
	var errs []error

	for _, function := range functions {
		func() {
			// NOTE: new activation, as function declarations
			// shouldn't be visible in other function declarations,
			// and `self` is is only visible inside function
			checker.valueActivations.PushCurrent()
			defer checker.valueActivations.Pop()

			checker.declareSelf(selfType)

			result := function.Accept(checker).(checkerResult)
			errs = append(errs, result.Errors...)
		}()
	}

	return checkerError(errs)
}

func (checker *Checker) declareSelf(selfType *StructureType) {

	// NOTE: declare `self` one depth lower ("inside" function),
	// so it can't be re-declared by the function's parameters

	depth := checker.valueActivations.Depth() + 1

	self := &Variable{
		Type:       selfType,
		IsConstant: true,
		Depth:      depth,
	}

	checker.setVariable(SelfIdentifier, self)
}

// checkStructureFieldAndFunctionIdentifiers checks the structure's fields and functions
// are unique and aren't named `init`
//
func (checker *Checker) checkStructureFieldAndFunctionIdentifiers(structure *ast.StructureDeclaration) *CheckerError {
	var errs []error

	positions := map[string]*ast.Position{}

	checkName := func(name string, pos *ast.Position, kind common.DeclarationKind) {
		if name == InitializerIdentifier {
			errs = append(errs,
				&InvalidNameError{
					Name: name,
					Pos:  structure.IdentifierPos,
				},
			)
		}

		if previousPos, ok := positions[name]; ok {
			errs = append(errs,
				&RedeclarationError{
					Name:        name,
					Pos:         pos,
					Kind:        kind,
					PreviousPos: previousPos,
				},
			)
		} else {
			positions[name] = pos
		}
	}

	for _, field := range structure.Fields {
		checkName(
			field.Identifier,
			field.IdentifierPos,
			common.DeclarationKindField,
		)
	}

	for _, function := range structure.Functions {
		checkName(
			function.Identifier,
			function.IdentifierPos,
			common.DeclarationKindFunction,
		)
	}

	return checkerError(errs)
}

func (checker *Checker) declareType(
	identifier string,
	identifierPos *ast.Position,
	newType Type,
) *CheckerError {
	var errs []error

	existingType := checker.findType(identifier)
	if existingType != nil {
		errs = append(errs,
			&RedeclarationError{
				Kind: common.DeclarationKindType,
				Name: identifier,
				Pos:  identifierPos,
				// TODO: previous pos
			},
		)
	}

	// type with this identifier is not declared in current scope, declare it
	checker.setType(identifier, newType)

	return checkerError(errs)
}

func (checker *Checker) VisitFieldDeclaration(field *ast.FieldDeclaration) ast.Repr {

	// NOTE: field type is already checked when determining structure function in `structureType`

	return checkerResult{
		Type:   nil,
		Errors: nil,
	}
}

func (checker *Checker) VisitInitializerDeclaration(initializer *ast.InitializerDeclaration) ast.Repr {
	var errs []error

	// check the initializer is named properly
	identifier := initializer.Identifier
	if identifier != InitializerIdentifier {
		errs = append(errs,
			&InvalidInitializerNameError{
				Name: identifier,
				Pos:  initializer.StartPos,
			},
		)
	}

	functionType, err := checker.functionType(initializer.Parameters, nil)
	if err != nil {
		// NOTE: append, don't return
		errs = append(errs, err.Errors...)
	}

	if err := checker.checkFunction(
		initializer.Parameters,
		functionType,
		initializer.Block,
	); err != nil {
		// NOTE: append, don't return
		errs = append(errs, err.Errors...)
	}

	return checkerResult{
		Type:   nil,
		Errors: errs,
	}
}
