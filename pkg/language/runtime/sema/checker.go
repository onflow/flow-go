package sema

import (
	"fmt"
	goRuntime "runtime"

	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/activations"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/ast"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/common"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/errors"
)

const ArgumentLabelNotRequired = "_"

type functionContext struct {
	returnType Type
}

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
	// TODO: improve
	return subType.Equal(superType)
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
		_, ok := indexingType.(integerType)
		return ok
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
	// recover internal panics and return them as an error
	defer func() {
		if r := recover(); r != nil {
			var ok bool
			// don't recover Go errors
			err, ok = r.(goRuntime.Error)
			if ok {
				panic(err)
			}
			err, ok = r.(error)
			if !ok {
				err = fmt.Errorf("%v", r)
			}
		}
	}()

	checker.Program.Accept(checker)

	return nil
}

func (checker *Checker) VisitProgram(program *ast.Program) ast.Repr {
	for _, declaration := range program.Declarations {
		declaration.Accept(checker)
		checker.declareGlobal(declaration)
	}
	return nil
}

func (checker *Checker) VisitFunctionDeclaration(declaration *ast.FunctionDeclaration) ast.Repr {
	functionType := checker.functionType(declaration.Parameters, declaration.ReturnType)
	checker.declareFunction(declaration.Identifier, declaration.IdentifierPos, functionType)

	checker.checkFunction(
		declaration.Parameters,
		functionType,
		declaration.Block,
	)

	return nil
}

func (checker *Checker) checkFunction(
	parameters []*ast.Parameter,
	functionType *FunctionType,
	block *ast.Block,
) {
	checker.pushActivations()
	defer checker.popActivations()

	checker.checkParameterNames(parameters)
	checker.checkArgumentLabels(parameters)
	checker.bindParameters(parameters)

	checker.enterFunction(functionType)
	defer checker.leaveFunction()

	block.Accept(checker)
}

// checkParameterNames checks that all parameter names are unique
//
func (checker *Checker) checkParameterNames(parameters []*ast.Parameter) {
	identifierPositions := map[string]*ast.Position{}

	for _, parameter := range parameters {
		identifier := parameter.Identifier
		if previousPos, ok := identifierPositions[identifier]; ok {
			panic(&RedeclarationError{
				Kind:        common.DeclarationKindParameter,
				Name:        identifier,
				Pos:         parameter.IdentifierPos,
				PreviousPos: previousPos,
			})
		}
		identifierPositions[identifier] = parameter.IdentifierPos
	}
}

// checkArgumentLabels checks that all argument labels (if any) are unique
//
func (checker *Checker) checkArgumentLabels(parameters []*ast.Parameter) {
	argumentLabelPositions := map[string]*ast.Position{}

	for _, parameter := range parameters {
		label := parameter.Label
		if label == "" || label == ArgumentLabelNotRequired {
			continue
		}

		if previousPos, ok := argumentLabelPositions[label]; ok {
			panic(&RedeclarationError{
				Kind:        common.DeclarationKindArgumentLabel,
				Name:        label,
				Pos:         parameter.LabelPos,
				PreviousPos: previousPos,
			})
		}

		argumentLabelPositions[label] = parameter.LabelPos
	}
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
	valueType := declaration.Value.Accept(checker).(Type)
	declarationType := valueType
	// does the declaration have an explicit type annotation?
	if declaration.Type != nil {
		declarationType = checker.ConvertType(declaration.Type)

		// check the value type is a subtype of the declaration type
		if !checker.IsSubType(valueType, declarationType) {
			panic(&TypeMismatchError{
				ExpectedType: declarationType,
				ActualType:   valueType,
				StartPos:     declaration.Value.StartPosition(),
				EndPos:       declaration.Value.EndPosition(),
			})
		}
	}
	checker.declareVariable(declaration, declarationType)

	return nil
}

func (checker *Checker) declareVariable(declaration *ast.VariableDeclaration, ty Type) {
	// check if variable with this name is already declared in the current scope
	variable := checker.findVariable(declaration.Identifier)
	depth := checker.valueActivations.Depth()
	if variable != nil && variable.Depth == depth {
		panic(&RedeclarationError{
			Kind: declaration.DeclarationKind(),
			Name: declaration.Identifier,
			Pos:  declaration.IdentifierPosition(),
		})
	}

	// variable with this name is not declared in current scope, declare it
	variable = &Variable{
		IsConstant: declaration.IsConstant,
		Depth:      depth,
		Type:       ty,
	}
	checker.setVariable(declaration.Identifier, variable)
}

func (checker *Checker) declareGlobal(declaration ast.Declaration) {
	name := declaration.DeclarationName()
	if _, exists := checker.Globals[name]; exists {
		panic(&RedeclarationError{
			Kind: declaration.DeclarationKind(),
			Name: name,
			Pos:  declaration.IdentifierPosition(),
		})
	}
	checker.Globals[name] = checker.findVariable(name)
}

func (checker *Checker) VisitBlock(block *ast.Block) ast.Repr {
	checker.pushActivations()
	defer checker.popActivations()

	for _, statement := range block.Statements {
		statement.Accept(checker)
	}

	return nil
}

func (checker *Checker) VisitReturnStatement(statement *ast.ReturnStatement) ast.Repr {

	// check value type matches enclosing function's return type

	valueType := statement.Expression.Accept(checker).(Type)
	returnType := checker.currentFunction().returnType

	if !checker.IsSubType(valueType, returnType) {
		panic(&TypeMismatchError{
			ExpectedType: returnType,
			ActualType:   valueType,
			StartPos:     statement.Expression.StartPosition(),
			EndPos:       statement.Expression.EndPosition(),
		})
	}

	return nil
}

func (checker *Checker) VisitIfStatement(statement *ast.IfStatement) ast.Repr {

	var elseElement ast.Element = ast.NotAnElement{}
	if statement.Else != nil {
		elseElement = statement.Else
	}

	checker.visitConditional(statement.Test, statement.Then, elseElement)

	return nil
}

func (checker *Checker) VisitWhileStatement(statement *ast.WhileStatement) ast.Repr {
	test := statement.Test
	testType := test.Accept(checker).(Type)

	if !testType.Equal(&BoolType{}) {
		panic(&TypeMismatchError{
			ExpectedType: &BoolType{},
			ActualType:   testType,
			StartPos:     test.StartPosition(),
			EndPos:       test.EndPosition(),
		})
	}

	statement.Block.Accept(checker)

	return nil
}

func (checker *Checker) VisitAssignment(assignment *ast.AssignmentStatement) ast.Repr {
	ty := assignment.Value.Accept(checker).(Type)
	checker.visitAssignmentValueType(assignment, ty)

	return nil
}

func (checker *Checker) visitAssignmentValueType(assignment *ast.AssignmentStatement, valueType Type) {
	switch target := assignment.Target.(type) {
	case *ast.IdentifierExpression:
		checker.visitIdentifierExpressionAssignment(assignment, target, valueType)
		return

	case *ast.IndexExpression:
		elementType := checker.visitIndexingExpression(target.Expression, target.Index)
		if !checker.IsSubType(valueType, elementType) {
			panic(&TypeMismatchError{
				ExpectedType: elementType,
				ActualType:   valueType,
				StartPos:     assignment.Value.StartPosition(),
				EndPos:       assignment.Value.EndPosition(),
			})
		}

		return

	case *ast.MemberExpression:
		// TODO:
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
) {
	identifier := target.Identifier

	// check identifier was declared before
	variable := checker.findVariable(identifier)
	if variable == nil {
		panic(&NotDeclaredError{
			ExpectedKind: common.DeclarationKindVariable,
			Name:         identifier,
			StartPos:     target.StartPosition(),
			EndPos:       target.EndPosition(),
		})
	}

	// check identifier is not a constant
	if variable.IsConstant {
		panic(&AssignmentToConstantError{
			Name:     identifier,
			StartPos: target.StartPosition(),
			EndPos:   target.EndPosition(),
		})
	}

	// check value type is subtype of variable type
	if !checker.IsSubType(valueType, variable.Type) {
		panic(&TypeMismatchError{
			ExpectedType: variable.Type,
			ActualType:   valueType,
			StartPos:     assignment.Value.StartPosition(),
			EndPos:       assignment.Value.EndPosition(),
		})
	}
}

// visitIndexingExpression checks if the indexed expression is indexable,
// checks if the indexing expression can be used to index into the indexed expression,
// and returns the expected element type
//
func (checker *Checker) visitIndexingExpression(indexedExpression, indexingExpression ast.Expression) Type {
	indexedType := indexedExpression.Accept(checker).(Type)
	indexingType := indexingExpression.Accept(checker).(Type)

	// NOTE: check indexed type first for UX reasons

	// check indexed expression's type is indexable
	// by getting the expected element

	elementType := checker.IndexableElementType(indexedType)
	if elementType == nil {
		panic(&NotIndexableTypeError{
			Type:     indexedType,
			StartPos: indexedExpression.StartPosition(),
			EndPos:   indexedExpression.EndPosition(),
		})
	}

	// check indexing expression's type can be used to index
	// into indexed expression's type

	if !checker.IsIndexingType(indexingType, indexedType) {
		panic(&NotIndexingTypeError{
			Type:     indexingType,
			StartPos: indexingExpression.StartPosition(),
			EndPos:   indexingExpression.EndPosition(),
		})
	}

	return elementType
}

func (checker *Checker) VisitIdentifierExpression(expression *ast.IdentifierExpression) ast.Repr {
	variable := checker.findVariable(expression.Identifier)
	if variable == nil {
		panic(&NotDeclaredError{
			ExpectedKind: common.DeclarationKindValue,
			Name:         expression.Identifier,
			StartPos:     expression.StartPosition(),
			EndPos:       expression.EndPosition(),
		})
	}

	return variable.Type
}

func (checker *Checker) VisitBinaryExpression(expression *ast.BinaryExpression) ast.Repr {
	// TODO:
	return nil
}

func (checker *Checker) VisitUnaryExpression(expression *ast.UnaryExpression) ast.Repr {
	// TODO:
	return nil
}

func (checker *Checker) VisitExpressionStatement(statement *ast.ExpressionStatement) ast.Repr {
	statement.Expression.Accept(checker)

	return nil
}

func (checker *Checker) VisitBoolExpression(expression *ast.BoolExpression) ast.Repr {
	return &BoolType{}
}

func (checker *Checker) VisitIntExpression(expression *ast.IntExpression) ast.Repr {
	return &IntType{}
}

func (checker *Checker) VisitArrayExpression(expression *ast.ArrayExpression) ast.Repr {
	// visit all elements, ensure they are all the same type

	var elementType Type

	for _, value := range expression.Values {
		valueType := value.Accept(checker).(Type)

		// infer element type from first element
		// TODO: find common super type?
		if elementType == nil {
			elementType = valueType
		} else if !checker.IsSubType(valueType, elementType) {
			panic(&TypeMismatchError{
				ExpectedType: elementType,
				ActualType:   valueType,
				StartPos:     value.StartPosition(),
				EndPos:       value.EndPosition(),
			})
		}
	}

	// TODO: use bottom type
	//if elementType == nil {
	//
	//}

	return &ConstantSizedType{
		Size: len(expression.Values),
		Type: elementType,
	}
}

func (checker *Checker) VisitMemberExpression(*ast.MemberExpression) ast.Repr {
	// TODO:
	panic(&errors.UnreachableError{})
	return nil
}

func (checker *Checker) VisitIndexExpression(expression *ast.IndexExpression) ast.Repr {
	return checker.visitIndexingExpression(expression.Expression, expression.Index)
}

func (checker *Checker) VisitConditionalExpression(expression *ast.ConditionalExpression) ast.Repr {

	thenType, elseType := checker.visitConditional(expression.Test, expression.Then, expression.Else)

	if thenType == nil || elseType == nil {
		panic(&errors.UnreachableError{})
	}

	// TODO: improve
	resultType := thenType

	if !checker.IsSubType(elseType, resultType) {
		panic(&TypeMismatchError{
			ExpectedType: resultType,
			ActualType:   elseType,
			StartPos:     expression.Else.StartPosition(),
			EndPos:       expression.Else.EndPosition(),
		})
	}

	return resultType
}

func (checker *Checker) VisitInvocationExpression(invocationExpression *ast.InvocationExpression) ast.Repr {

	// check the invoked expression can be invoked

	invokedExpression := invocationExpression.Expression
	expressionType := invokedExpression.Accept(checker).(Type)

	functionType, ok := expressionType.(*FunctionType)
	if !ok {
		panic(&NotCallableError{
			Type:     expressionType,
			StartPos: invokedExpression.StartPosition(),
			EndPos:   invokedExpression.EndPosition(),
		})
	}

	// check the invocation's argument count matches the function's parameter count

	parameterCount := len(functionType.ParameterTypes)
	argumentCount := len(invocationExpression.Arguments)

	if argumentCount != parameterCount {
		panic(&ArgumentCountError{
			ParameterCount: parameterCount,
			ArgumentCount:  argumentCount,
			StartPos:       invocationExpression.StartPos,
			EndPos:         invocationExpression.EndPos,
		})
	}

	for i := 0; i < parameterCount; i++ {
		// ensure the type of the argument matches the type of the parameter

		parameterType := functionType.ParameterTypes[i]
		argument := invocationExpression.Arguments[i]
		argumentType := argument.Expression.Accept(checker).(Type)

		if !checker.IsSubType(argumentType, parameterType) {
			panic(&TypeMismatchError{
				ExpectedType: parameterType,
				ActualType:   argumentType,
				StartPos:     argument.Expression.StartPosition(),
				EndPos:       argument.Expression.EndPosition(),
			})
		}
	}

	// TODO: ensure argument labels of arguments match argument labels of parameters

	return functionType.ReturnType
}

func (checker *Checker) VisitFunctionExpression(expression *ast.FunctionExpression) ast.Repr {
	// TODO: infer
	functionType := checker.functionType(expression.Parameters, expression.ReturnType)
	checker.checkFunction(
		expression.Parameters,
		functionType,
		expression.Block,
	)

	return nil
}

// ConvertType converts an AST type representation to a sema type
func (checker *Checker) ConvertType(t ast.Type) Type {
	switch t := t.(type) {
	case *ast.BaseType:
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
) {
	// check if variable with this identifier is already declared in the current scope
	variable := checker.findVariable(identifier)
	depth := checker.valueActivations.Depth()
	if variable != nil && variable.Depth == depth {
		panic(&RedeclarationError{
			Kind: common.DeclarationKindFunction,
			Name: identifier,
			Pos:  identifierPosition,
		})
	}

	// variable with this identifier is not declared in current scope, declare it
	variable = &Variable{
		IsConstant: true,
		Depth:      depth,
		Type:       functionType,
	}
	checker.setVariable(identifier, variable)
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
	thenType, elseType Type,
) {
	testType := test.Accept(checker).(Type)

	if !testType.Equal(&BoolType{}) {
		panic(&TypeMismatchError{
			ExpectedType: &BoolType{},
			ActualType:   testType,
			StartPos:     test.StartPosition(),
			EndPos:       test.EndPosition(),
		})
	}

	thenType, _ = thenElement.Accept(checker).(Type)
	elseType, _ = elseElement.Accept(checker).(Type)

	return
}
