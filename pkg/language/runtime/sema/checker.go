package sema

import (
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/activations"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/ast"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/common"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/errors"
	"strings"
)

const ArgumentLabelNotRequired = "_"

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

func (checker *Checker) Check() (err error) {
	result := checker.Program.Accept(checker).(checkerResult)
	return checkerError(result.Errors)
}

func (checker *Checker) VisitProgram(program *ast.Program) ast.Repr {
	var errs []error

	for _, declaration := range program.Declarations {

		result := declaration.Accept(checker).(checkerResult)
		errs = append(errs, result.Errors...)

		err := checker.declareGlobal(declaration)
		if err != nil {
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
			&InvalidAccessError{
				DeclarationKind: common.DeclarationKindFunction,
				Access:          declaration.Access,
				Pos:             declaration.StartPos,
			},
		)
	}

	functionType := checker.functionType(declaration.Parameters, declaration.ReturnType)

	argumentLabels := make([]string, len(declaration.Parameters))

	for i, parameter := range declaration.Parameters {
		argumentLabel := parameter.Label
		// if no argument label is given, the parameter name
		// is used as the argument labels and is required
		if argumentLabel == "" {
			argumentLabel = parameter.Identifier
		}
		argumentLabels[i] = argumentLabel
	}

	// declare the function before checking it,
	// so it can be referred to inside the function

	err := checker.declareFunction(
		declaration.Identifier,
		declaration.IdentifierPos,
		functionType,
		argumentLabels,
	)
	if err != nil {
		errs = append(errs, err.Errors...)
	}

	// check the function after declaring,
	// so it can be referred to inside the function

	err = checker.checkFunction(
		declaration.Parameters,
		functionType,
		declaration.Block,
	)
	if err != nil {
		errs = append(errs, err.Errors...)
	}

	return checkerResult{
		Type:   nil,
		Errors: errs,
	}
}

func (checker *Checker) checkFunction(
	parameters []*ast.Parameter,
	functionType *FunctionType,
	block *ast.Block,
) *CheckerError {
	var errs []error

	checker.pushActivations()
	defer checker.popActivations()

	// check parameter names
	err := checker.checkParameterNames(parameters)
	if err != nil {
		errs = append(errs, err.Errors...)
	}

	// check argument labels
	err = checker.checkArgumentLabels(parameters)
	if err != nil {
		errs = append(errs, err.Errors...)
	}

	checker.bindParameters(parameters)

	checker.enterFunction(functionType)
	defer checker.leaveFunction()

	result := block.Accept(checker).(checkerResult)
	errs = append(errs, result.Errors...)

	return checkerError(errs)
}

// checkParameterNames checks that all parameter names are unique
//
func (checker *Checker) checkParameterNames(parameters []*ast.Parameter) *CheckerError {
	var errs []error
	identifierPositions := map[string]*ast.Position{}

	for _, parameter := range parameters {
		identifier := parameter.Identifier
		if previousPos, ok := identifierPositions[identifier]; ok {
			errs = append(errs,
				&RedeclarationError{
					Kind:        common.DeclarationKindParameter,
					Name:        identifier,
					Pos:         parameter.IdentifierPos,
					PreviousPos: previousPos,
				},
			)
		}
		identifierPositions[identifier] = parameter.IdentifierPos
	}

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

func (checker *Checker) bindParameters(parameters []*ast.Parameter) {
	// declare a constant variable for each parameter

	depth := checker.valueActivations.Depth()

	for _, parameter := range parameters {
		ty := checker.ConvertType(parameter.Type)
		checker.setVariable(
			parameter.Identifier,
			&Variable{
				IsConstant: true,
				Type:       ty,
				Depth:      depth,
			},
		)
	}
}

func (checker *Checker) VisitVariableDeclaration(declaration *ast.VariableDeclaration) ast.Repr {
	valueResult := declaration.Value.Accept(checker).(checkerResult)
	valueType := valueResult.Type
	errs := valueResult.Errors

	declarationType := valueType
	// does the declaration have an explicit type annotation?
	if declaration.Type != nil {
		declarationType = checker.ConvertType(declaration.Type)

		// check the value type is a subtype of the declaration type
		if !checker.IsSubType(valueType, declarationType) {
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

	err := checker.declareVariable(declaration, declarationType)
	if err != nil {
		errs = append(errs, err.Errors...)
	}

	return checkerResult{
		Type:   nil,
		Errors: errs,
	}
}

func (checker *Checker) declareVariable(declaration *ast.VariableDeclaration, ty Type) *CheckerError {
	var errs []error

	// check if variable with this name is already declared in the current scope
	variable := checker.findVariable(declaration.Identifier)
	depth := checker.valueActivations.Depth()
	if variable != nil && variable.Depth == depth {
		errs = append(errs,
			&RedeclarationError{
				Kind: declaration.DeclarationKind(),
				Name: declaration.Identifier,
				Pos:  declaration.IdentifierPosition(),
			},
		)
	}

	// variable with this name is not declared in current scope, declare it
	variable = &Variable{
		IsConstant: declaration.IsConstant,
		Depth:      depth,
		Type:       ty,
	}
	checker.setVariable(declaration.Identifier, variable)

	return checkerError(errs)
}

func (checker *Checker) declareGlobal(declaration ast.Declaration) *CheckerError {
	var errs []error

	name := declaration.DeclarationName()
	if _, exists := checker.Globals[name]; exists {
		errs = append(errs,
			&RedeclarationError{
				Kind: declaration.DeclarationKind(),
				Name: name,
				Pos:  declaration.IdentifierPosition(),
			},
		)
	}

	checker.Globals[name] = checker.findVariable(name)

	return checkerError(errs)
}

func (checker *Checker) VisitBlock(block *ast.Block) ast.Repr {
	var errs []error

	checker.pushActivations()
	defer checker.popActivations()

	for _, statement := range block.Statements {
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

	_, _, err := checker.visitConditional(statement.Test, statement.Then, elseElement)
	if err != nil {
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

	err := checker.visitAssignmentValueType(assignment, valueType)
	if err != nil {
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

	case *ast.MemberExpression:
		// TODO: no structures/dictionaries yet
		panic(&errors.UnreachableError{})

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
				StartPos:     target.StartPosition(),
				EndPos:       target.EndPosition(),
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
			StartPos:     expression.StartPosition(),
			EndPos:       expression.EndPosition(),
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

func (checker *Checker) VisitMemberExpression(*ast.MemberExpression) ast.Repr {
	// TODO: no structures/dictionaries yet
	panic(&errors.UnreachableError{})
}

func (checker *Checker) VisitIndexExpression(expression *ast.IndexExpression) ast.Repr {
	return checker.visitIndexingExpression(expression.Expression, expression.Index)
}

func (checker *Checker) VisitConditionalExpression(expression *ast.ConditionalExpression) ast.Repr {
	var errs []error

	thenType, elseType, err := checker.visitConditional(expression.Test, expression.Then, expression.Else)
	if err != nil {
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

	argumentCount := len(invocationExpression.Arguments)

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
	}

	// if the invocation refers directly to the name of the function as stated in the declaration,
	// the argument labels need to be supplied

	if identifierExpression, ok := invokedExpression.(*ast.IdentifierExpression); ok {

		variable, err := checker.findAndCheckVariable(identifierExpression)
		if err != nil {
			errs = append(errs, err)
		} else if variable != nil {
			if variable.ArgumentLabels != nil {

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
	}

	var returnType Type
	if functionType != nil {
		returnType = functionType.ReturnType
	}

	return checkerResult{
		Type:   returnType,
		Errors: errs,
	}
}

func (checker *Checker) VisitFunctionExpression(expression *ast.FunctionExpression) ast.Repr {
	var errs []error

	// TODO: infer
	functionType := checker.functionType(expression.Parameters, expression.ReturnType)

	err := checker.checkFunction(
		expression.Parameters,
		functionType,
		expression.Block,
	)
	if err != nil {
		errs = append(errs, err.Errors...)
	}

	return checkerResult{
		Type:   functionType,
		Errors: errs,
	}
}

// ConvertType converts an AST type representation to a sema type
func (checker *Checker) ConvertType(t ast.Type) Type {
	switch t := t.(type) {
	case *ast.NominalType:
		result := checker.findType(t.Identifier)
		if result == nil {
			panic(&NotDeclaredError{
				ExpectedKind: common.DeclarationKindType,
				Name:         t.Identifier,
				// TODO: add start and end position to ast.Type
				StartPos: t.Pos,
				EndPos:   t.Pos,
			})
		}
		return result

	case *ast.VariableSizedType:
		return &VariableSizedType{
			Type: checker.ConvertType(t.Type),
		}

	case *ast.ConstantSizedType:
		return &ConstantSizedType{
			Type: checker.ConvertType(t.Type),
			Size: t.Size,
		}

	case *ast.FunctionType:
		var parameterTypes []Type
		for _, parameterType := range t.ParameterTypes {
			parameterTypes = append(parameterTypes,
				checker.ConvertType(parameterType),
			)
		}

		returnType := checker.ConvertType(t.ReturnType)

		return &FunctionType{
			ParameterTypes: parameterTypes,
			ReturnType:     returnType,
		}
	}

	panic(&astTypeConversionError{invalidASTType: t})
}

func (checker *Checker) enterFunction(functionType *FunctionType) {
	checker.functionContexts = append(checker.functionContexts,
		&functionContext{
			returnType: functionType.ReturnType,
		})
}

func (checker *Checker) declareFunction(
	identifier string,
	identifierPosition *ast.Position,
	functionType *FunctionType,
	argumentLabels []string,
) *CheckerError {
	var errs []error

	// check if variable with this identifier is already declared in the current scope
	variable := checker.findVariable(identifier)
	depth := checker.valueActivations.Depth()
	if variable != nil && variable.Depth == depth {
		errs = append(errs,
			&RedeclarationError{
				Kind: common.DeclarationKindFunction,
				Name: identifier,
				Pos:  identifierPosition,
			},
		)
	}

	// variable with this identifier is not declared in current scope, declare it
	variable = &Variable{
		IsConstant:     true,
		Depth:          depth,
		Type:           functionType,
		ArgumentLabels: argumentLabels,
	}
	checker.setVariable(identifier, variable)

	return checkerError(errs)
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

func (checker *Checker) functionType(parameters []*ast.Parameter, returnType ast.Type) *FunctionType {
	parameterTypes := make([]Type, len(parameters))
	for i, parameter := range parameters {
		parameterTypes[i] = checker.ConvertType(parameter.Type)
	}

	return &FunctionType{
		ParameterTypes: parameterTypes,
		ReturnType:     checker.ConvertType(returnType),
	}
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
