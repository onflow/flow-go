package sema

import (
	"github.com/raviqqe/hamt"

	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/activations"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/ast"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/common"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/errors"
)

const ArgumentLabelNotRequired = "_"
const InitializerIdentifier = "init"
const SelfIdentifier = "self"
const BeforeIdentifier = "before"
const ResultIdentifier = "result"

type functionContext struct {
	returnType Type
	loops      int
}

var beforeType = &FunctionType{
	ParameterTypes: []Type{&AnyType{}},
	ReturnType:     &AnyType{},
	Apply: func(types []Type) Type {
		return types[0]
	},
}

// Checker

type Checker struct {
	Program           *ast.Program
	PredeclaredValues map[string]ValueDeclaration
	PredeclaredTypes  map[string]TypeDeclaration
	ImportCheckers    map[ast.ImportLocation]*Checker
	errors            []error
	valueActivations  *activations.Activations
	typeActivations   *activations.Activations
	functionContexts  []*functionContext
	GlobalValues      map[string]*Variable
	GlobalTypes       map[string]Type
	inCondition       bool
	Origins           *Origins
	seenImports       map[ast.ImportLocation]bool
	isChecked         bool
	// TODO: refactor into fields on AST?
	FunctionDeclarationFunctionTypes   map[*ast.FunctionDeclaration]*FunctionType
	VariableDeclarationValueTypes      map[*ast.VariableDeclaration]Type
	VariableDeclarationTargetTypes     map[*ast.VariableDeclaration]Type
	AssignmentStatementValueTypes      map[*ast.AssignmentStatement]Type
	AssignmentStatementTargetTypes     map[*ast.AssignmentStatement]Type
	StructureDeclarationTypes          map[*ast.StructureDeclaration]*StructureType
	InitializerFunctionTypes           map[*ast.InitializerDeclaration]*FunctionType
	FunctionExpressionFunctionType     map[*ast.FunctionExpression]*FunctionType
	InvocationExpressionArgumentTypes  map[*ast.InvocationExpression][]Type
	InvocationExpressionParameterTypes map[*ast.InvocationExpression][]Type
	InterfaceDeclarationTypes          map[*ast.InterfaceDeclaration]*InterfaceType
	FailableDowncastingTypes           map[*ast.FailableDowncastExpression]Type
	ReturnStatementValueTypes          map[*ast.ReturnStatement]Type
	ReturnStatementReturnTypes         map[*ast.ReturnStatement]Type
	BinaryExpressionResultTypes        map[*ast.BinaryExpression]Type
	BinaryExpressionRightTypes         map[*ast.BinaryExpression]Type
}

func NewChecker(
	program *ast.Program,
	predeclaredValues map[string]ValueDeclaration,
	predeclaredTypes map[string]TypeDeclaration,
) (*Checker, error) {
	typeActivations := &activations.Activations{}
	typeActivations.Push(baseTypes)

	valueActivations := &activations.Activations{}
	valueActivations.Push(hamt.NewMap())

	checker := &Checker{
		Program:                            program,
		PredeclaredValues:                  predeclaredValues,
		PredeclaredTypes:                   predeclaredTypes,
		ImportCheckers:                     map[ast.ImportLocation]*Checker{},
		valueActivations:                   valueActivations,
		typeActivations:                    typeActivations,
		GlobalValues:                       map[string]*Variable{},
		GlobalTypes:                        map[string]Type{},
		Origins:                            NewOrigins(),
		seenImports:                        map[ast.ImportLocation]bool{},
		FunctionDeclarationFunctionTypes:   map[*ast.FunctionDeclaration]*FunctionType{},
		VariableDeclarationValueTypes:      map[*ast.VariableDeclaration]Type{},
		VariableDeclarationTargetTypes:     map[*ast.VariableDeclaration]Type{},
		AssignmentStatementValueTypes:      map[*ast.AssignmentStatement]Type{},
		AssignmentStatementTargetTypes:     map[*ast.AssignmentStatement]Type{},
		StructureDeclarationTypes:          map[*ast.StructureDeclaration]*StructureType{},
		InitializerFunctionTypes:           map[*ast.InitializerDeclaration]*FunctionType{},
		FunctionExpressionFunctionType:     map[*ast.FunctionExpression]*FunctionType{},
		InvocationExpressionArgumentTypes:  map[*ast.InvocationExpression][]Type{},
		InvocationExpressionParameterTypes: map[*ast.InvocationExpression][]Type{},
		InterfaceDeclarationTypes:          map[*ast.InterfaceDeclaration]*InterfaceType{},
		FailableDowncastingTypes:           map[*ast.FailableDowncastExpression]Type{},
		ReturnStatementValueTypes:          map[*ast.ReturnStatement]Type{},
		ReturnStatementReturnTypes:         map[*ast.ReturnStatement]Type{},
		BinaryExpressionResultTypes:        map[*ast.BinaryExpression]Type{},
		BinaryExpressionRightTypes:         map[*ast.BinaryExpression]Type{},
	}

	for name, declaration := range predeclaredValues {
		checker.declareValue(name, declaration)
		checker.declareGlobalValue(name)
	}

	for name, declaration := range predeclaredTypes {
		checker.declareTypeDeclaration(name, declaration)
	}

	err := checker.checkerError()
	if err != nil {
		return nil, err
	}

	return checker, nil
}

type ValueDeclaration interface {
	ValueDeclarationType() Type
	ValueDeclarationKind() common.DeclarationKind
	ValueDeclarationPosition() ast.Position
	ValueDeclarationIsConstant() bool
	ValueDeclarationArgumentLabels() []string
}

type TypeDeclaration interface {
	TypeDeclarationType() Type
	TypeDeclarationKind() common.DeclarationKind
	TypeDeclarationPosition() ast.Position
}

func (checker *Checker) declareValue(name string, declaration ValueDeclaration) {
	variable := checker.declareVariable(
		name,
		declaration.ValueDeclarationType(),
		declaration.ValueDeclarationKind(),
		declaration.ValueDeclarationPosition(),
		declaration.ValueDeclarationIsConstant(),
		declaration.ValueDeclarationArgumentLabels(),
	)
	checker.recordVariableOrigin(name, variable)
}

func (checker *Checker) declareTypeDeclaration(name string, declaration TypeDeclaration) {
	identifier := ast.Identifier{
		Identifier: name,
		Pos:        declaration.TypeDeclarationPosition(),
	}

	ty := declaration.TypeDeclarationType()
	checker.declareType(identifier, ty)
	checker.recordVariableOrigin(
		identifier.Identifier,
		&Variable{
			Kind:       declaration.TypeDeclarationKind(),
			IsConstant: true,
			Type:       ty,
			Pos:        &identifier.Pos,
		},
	)
}

func (checker *Checker) IsChecked() bool {
	return checker.isChecked
}

func (checker *Checker) Check() error {
	if !checker.IsChecked() {
		checker.errors = nil
		checker.Program.Accept(checker)
		checker.isChecked = true
	}
	err := checker.checkerError()
	if err != nil {
		return err
	}
	return nil
}

func (checker *Checker) checkerError() *CheckerError {
	if len(checker.errors) > 0 {
		return &CheckerError{
			Errors: checker.errors,
		}
	}
	return nil
}

func (checker *Checker) report(errs ...error) {
	checker.errors = append(checker.errors, errs...)
}

func (checker *Checker) IsSubType(subType Type, superType Type) bool {
	if subType.Equal(superType) {
		return true
	}

	if _, ok := superType.(*AnyType); ok {
		return true
	}

	if _, ok := subType.(*NeverType); ok {
		return true
	}

	switch typedSuperType := superType.(type) {
	case *IntegerType:
		switch subType.(type) {
		case *IntType,
			*Int8Type, *Int16Type, *Int32Type, *Int64Type,
			*UInt8Type, *UInt16Type, *UInt32Type, *UInt64Type:

			return true

		default:
			return false
		}

	case *OptionalType:
		optionalSubType, ok := subType.(*OptionalType)
		if !ok {
			// T <: U? if T <: U
			return checker.IsSubType(subType, typedSuperType.Type)
		}
		// optionals are covariant: T? <: U? if T <: U
		return checker.IsSubType(optionalSubType.Type, typedSuperType.Type)

	case *InterfaceType:
		structureSubType, ok := subType.(*StructureType)
		if !ok {
			return false
		}
		// TODO: optimize, use set
		for _, conformance := range structureSubType.Conformances {
			if typedSuperType.Equal(conformance) {
				return true
			}
		}
		return false

	case *DictionaryType:
		typedSubType, ok := subType.(*DictionaryType)
		if !ok {
			return false
		}

		return checker.IsSubType(typedSubType.KeyType, typedSuperType.KeyType) &&
			checker.IsSubType(typedSubType.ValueType, typedSuperType.ValueType)

	case *VariableSizedType:
		typedSubType, ok := subType.(*VariableSizedType)
		if !ok {
			return false
		}

		return checker.IsSubType(
			typedSubType.elementType(),
			typedSuperType.elementType(),
		)

	case *ConstantSizedType:
		typedSubType, ok := subType.(*ConstantSizedType)
		if !ok {
			return false
		}

		if typedSubType.Size != typedSuperType.Size {
			return false
		}

		return checker.IsSubType(
			typedSubType.elementType(),
			typedSuperType.elementType(),
		)
	}

	// TODO: functions

	return false
}

func (checker *Checker) IndexableElementType(indexedType Type, isAssignment bool) Type {
	switch indexedType := indexedType.(type) {
	case ArrayType:
		return indexedType.elementType()
	case *DictionaryType:
		valueType := indexedType.ValueType
		if isAssignment {
			return valueType
		} else {
			return &OptionalType{Type: valueType}
		}
	}

	return nil
}

func (checker *Checker) IsIndexingType(indexingType Type, indexedType Type) bool {
	switch indexedType := indexedType.(type) {
	// arrays can be indexed with integers
	case ArrayType:
		return checker.IsSubType(indexingType, &IntegerType{})
	// dictionaries can be indexed with the dictionary type's key type
	case *DictionaryType:
		return checker.IsSubType(indexingType, indexedType.KeyType)
	}

	return false
}

func (checker *Checker) IsConcatenatableType(ty Type) bool {
	_, isArrayType := ty.(ArrayType)
	return checker.IsSubType(ty, &StringType{}) || isArrayType
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

func (checker *Checker) FindType(name string) Type {
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

	// pre-declare interfaces, structures, and functions (check afterwards)

	for _, declaration := range program.InterfaceDeclarations() {
		checker.declareInterfaceDeclaration(declaration)
	}

	for _, declaration := range program.StructureDeclarations() {
		checker.declareStructureDeclaration(declaration)
	}

	for _, declaration := range program.FunctionDeclarations() {
		checker.declareFunctionDeclaration(declaration)
	}

	// check all declarations

	for _, declaration := range program.Declarations {
		declaration.Accept(checker)
		checker.declareGlobalDeclaration(declaration)
	}

	return nil
}

func (checker *Checker) VisitFunctionDeclaration(declaration *ast.FunctionDeclaration) ast.Repr {

	checker.checkFunctionAccessModifier(declaration)

	// global functions were previously declared, see `declareFunctionDeclaration`

	functionType := checker.FunctionDeclarationFunctionTypes[declaration]
	if functionType == nil {
		functionType = checker.declareFunctionDeclaration(declaration)
	}

	checker.checkFunction(
		declaration.Parameters,
		functionType,
		declaration.FunctionBlock,
	)

	return nil
}

func (checker *Checker) declareFunctionDeclaration(declaration *ast.FunctionDeclaration) *FunctionType {

	functionType := checker.functionType(declaration.Parameters, declaration.ReturnTypeAnnotation)
	argumentLabels := checker.argumentLabels(declaration.Parameters)

	checker.FunctionDeclarationFunctionTypes[declaration] = functionType

	checker.declareFunction(
		declaration.Identifier,
		functionType,
		argumentLabels,
		true,
	)

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

func (checker *Checker) argumentLabels(parameters []*ast.Parameter) []string {
	argumentLabels := make([]string, len(parameters))

	for i, parameter := range parameters {
		argumentLabel := parameter.Label
		// if no argument label is given, the parameter name
		// is used as the argument labels and is required
		if argumentLabel == "" {
			argumentLabel = parameter.Identifier.Identifier
		}
		argumentLabels[i] = argumentLabel
	}

	return argumentLabels
}

func (checker *Checker) checkFunction(
	parameters []*ast.Parameter,
	functionType *FunctionType,
	functionBlock *ast.FunctionBlock,
) {
	checker.pushActivations()
	defer checker.popActivations()

	// check argument labels
	checker.checkArgumentLabels(parameters)

	checker.declareParameters(parameters, functionType.ParameterTypes)

	if functionBlock != nil {
		func() {
			// check the function's block
			checker.enterFunction(functionType)
			defer checker.leaveFunction()

			checker.visitFunctionBlock(functionBlock, functionType.ReturnType)
		}()
	}
}

// checkArgumentLabels checks that all argument labels (if any) are unique
//
func (checker *Checker) checkArgumentLabels(parameters []*ast.Parameter) {

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
// ensuring names are unique and constants don'T already exist
//
func (checker *Checker) declareParameters(parameters []*ast.Parameter, parameterTypes []Type) {

	depth := checker.valueActivations.Depth()

	for i, parameter := range parameters {
		identifier := parameter.Identifier

		// check if variable with this identifier is already declared in the current scope
		existingVariable := checker.findVariable(identifier.Identifier)
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

		parameterType := parameterTypes[i]

		variable := &Variable{
			Kind:       common.DeclarationKindParameter,
			IsConstant: true,
			Type:       parameterType,
			Depth:      depth,
			Pos:        &identifier.Pos,
		}
		checker.setVariable(identifier.Identifier, variable)
		checker.recordVariableOrigin(identifier.Identifier, variable)
	}
}

func (checker *Checker) VisitVariableDeclaration(declaration *ast.VariableDeclaration) ast.Repr {
	checker.visitVariableDeclaration(declaration, false)
	return nil
}

func (checker *Checker) visitVariableDeclaration(declaration *ast.VariableDeclaration, isOptionalBinding bool) {
	valueType := declaration.Value.Accept(checker).(Type)

	checker.VariableDeclarationValueTypes[declaration] = valueType

	// if the variable declaration is a optional binding, the value must be optional

	var valueIsOptional bool
	var optionalValueType *OptionalType

	if isOptionalBinding {
		optionalValueType, valueIsOptional = valueType.(*OptionalType)
		if !valueIsOptional {
			checker.report(
				&TypeMismatchError{
					ExpectedType: &OptionalType{},
					ActualType:   valueType,
					StartPos:     declaration.Value.StartPosition(),
					EndPos:       declaration.Value.EndPosition(),
				},
			)
		}
	}

	declarationType := valueType

	// does the declaration have an explicit type annotation?
	if declaration.TypeAnnotation != nil {
		declarationType = checker.ConvertType(declaration.TypeAnnotation.Type)

		// check the value type is a subtype of the declaration type
		if declarationType != nil && valueType != nil && !isInvalidType(valueType) && !isInvalidType(declarationType) {

			if isOptionalBinding {
				if optionalValueType != nil &&
					(optionalValueType.Equal(declarationType) ||
						!checker.IsSubType(optionalValueType.Type, declarationType)) {

					checker.report(
						&TypeMismatchError{
							ExpectedType: declarationType,
							ActualType:   optionalValueType.Type,
							StartPos:     declaration.Value.StartPosition(),
							EndPos:       declaration.Value.EndPosition(),
						},
					)
				}

			} else {
				if !checker.IsSubType(valueType, declarationType) {
					checker.report(
						&TypeMismatchError{
							ExpectedType: declarationType,
							ActualType:   valueType,
							StartPos:     declaration.Value.StartPosition(),
							EndPos:       declaration.Value.EndPosition(),
						},
					)
				}
			}
		}
	} else if isOptionalBinding && optionalValueType != nil {
		declarationType = optionalValueType.Type
	}

	checker.VariableDeclarationTargetTypes[declaration] = declarationType

	variable := checker.declareVariable(
		declaration.Identifier.Identifier,
		declarationType,
		declaration.DeclarationKind(),
		declaration.Identifier.Pos,
		declaration.IsConstant,
		nil,
	)
	checker.recordVariableOrigin(declaration.Identifier.Identifier, variable)
}

func (checker *Checker) declareVariable(
	identifier string,
	ty Type,
	kind common.DeclarationKind,
	pos ast.Position,
	isConstant bool,
	argumentLabels []string,
) *Variable {

	depth := checker.valueActivations.Depth()

	// check if variable with this name is already declared in the current scope
	existingVariable := checker.findVariable(identifier)
	if existingVariable != nil && existingVariable.Depth == depth {
		checker.report(
			&RedeclarationError{
				Kind:        kind,
				Name:        identifier,
				Pos:         pos,
				PreviousPos: existingVariable.Pos,
			},
		)
	}

	// variable with this name is not declared in current scope, declare it
	variable := &Variable{
		Kind:           kind,
		IsConstant:     isConstant,
		Depth:          depth,
		Type:           ty,
		Pos:            &pos,
		ArgumentLabels: argumentLabels,
	}
	checker.setVariable(identifier, variable)
	return variable
}

func (checker *Checker) declareGlobalDeclaration(declaration ast.Declaration) {
	name := declaration.DeclarationName()
	if name == "" {
		return
	}
	checker.declareGlobalValue(name)
	checker.declareGlobalType(name)
}

func (checker *Checker) declareGlobalValue(name string) {
	variable := checker.findVariable(name)
	if variable == nil {
		return
	}
	checker.GlobalValues[name] = variable
}

func (checker *Checker) declareGlobalType(name string) {
	ty := checker.FindType(name)
	if ty == nil {
		return
	}
	checker.GlobalTypes[name] = ty
}

func (checker *Checker) VisitBlock(block *ast.Block) ast.Repr {

	checker.pushActivations()
	defer checker.popActivations()

	checker.visitStatements(block.Statements)

	return nil
}

func (checker *Checker) visitStatements(statements []ast.Statement) {

	// check all statements
	for _, statement := range statements {

		// check statement is not a local structure or interface declaration

		if _, ok := statement.(*ast.StructureDeclaration); ok {
			checker.report(
				&InvalidDeclarationError{
					Kind:     common.DeclarationKindStructure,
					StartPos: statement.StartPosition(),
					EndPos:   statement.EndPosition(),
				},
			)

			continue
		}

		if interfaceDeclaration, ok := statement.(*ast.InterfaceDeclaration); ok {
			checker.report(
				&InvalidDeclarationError{
					Kind:     interfaceDeclaration.DeclarationKind(),
					StartPos: statement.StartPosition(),
					EndPos:   statement.EndPosition(),
				},
			)

			continue
		}

		// check statement

		statement.Accept(checker)
	}
}

func (checker *Checker) VisitFunctionBlock(functionBlock *ast.FunctionBlock) ast.Repr {
	// NOTE: see visitFunctionBlock
	panic(&errors.UnreachableError{})
}

func (checker *Checker) visitFunctionBlock(functionBlock *ast.FunctionBlock, returnType Type) {

	checker.pushActivations()
	defer checker.popActivations()

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

	if _, ok := returnType.(*VoidType); !ok {
		checker.declareVariable(
			ResultIdentifier,
			returnType,
			common.DeclarationKindConstant,
			ast.Position{},
			true,
			nil,
		)
		// TODO: record origin - but what position?
	}

	checker.visitConditions(functionBlock.PostConditions)
}

func (checker *Checker) declareBefore() {

	checker.declareVariable(
		BeforeIdentifier,
		beforeType,
		common.DeclarationKindFunction,
		ast.Position{},
		true,
		nil,
	)
	// TODO: record origin – but what position?
}

func (checker *Checker) visitConditions(conditions []*ast.Condition) {

	// flag the checker to be inside a condition.
	// this flag is used to detect illegal expressions,
	// see e.g. VisitFunctionExpression

	wasInCondition := checker.inCondition
	checker.inCondition = true
	defer func() {
		checker.inCondition = wasInCondition
	}()

	// check all conditions: check the expression
	// and ensure the result is boolean

	for _, condition := range conditions {
		condition.Accept(checker)
	}
}

func (checker *Checker) VisitCondition(condition *ast.Condition) ast.Repr {

	// check test expression is boolean

	testType := condition.Test.Accept(checker).(Type)

	if !isInvalidType(testType) && !checker.IsSubType(testType, &BoolType{}) {
		checker.report(
			&TypeMismatchError{
				ExpectedType: &BoolType{},
				ActualType:   testType,
				StartPos:     condition.Test.StartPosition(),
				EndPos:       condition.Test.EndPosition(),
			},
		)
	}

	// check message expression results in a string

	if condition.Message != nil {

		messageType := condition.Message.Accept(checker).(Type)

		if !isInvalidType(messageType) && !checker.IsSubType(messageType, &StringType{}) {
			checker.report(
				&TypeMismatchError{
					ExpectedType: &StringType{},
					ActualType:   testType,
					StartPos:     condition.Message.StartPosition(),
					EndPos:       condition.Message.EndPosition(),
				},
			)
		}
	}

	return nil
}

func (checker *Checker) VisitReturnStatement(statement *ast.ReturnStatement) ast.Repr {

	// check value type matches enclosing function's return type

	if statement.Expression == nil {
		return nil
	}

	valueType := statement.Expression.Accept(checker).(Type)
	valueIsInvalid := isInvalidType(valueType)

	returnType := checker.currentFunction().returnType

	checker.ReturnStatementValueTypes[statement] = valueType
	checker.ReturnStatementReturnTypes[statement] = returnType

	if valueType != nil {
		if valueIsInvalid {
			// return statement has expression, but function has Void return type?
			if _, ok := returnType.(*VoidType); ok {
				checker.report(
					&InvalidReturnValueError{
						StartPos: statement.Expression.StartPosition(),
						EndPos:   statement.Expression.EndPosition(),
					},
				)
			}
		} else {
			if !valueIsInvalid &&
				!isInvalidType(returnType) &&
				!checker.IsSubType(valueType, returnType) {

				checker.report(
					&TypeMismatchError{
						ExpectedType: returnType,
						ActualType:   valueType,
						StartPos:     statement.Expression.StartPosition(),
						EndPos:       statement.Expression.EndPosition(),
					},
				)
			}
		}
	}

	return nil
}

func (checker *Checker) VisitBreakStatement(statement *ast.BreakStatement) ast.Repr {

	// check statement is inside loop

	if checker.currentFunction().loops == 0 {
		checker.report(
			&ControlStatementError{
				ControlStatement: common.ControlStatementBreak,
				StartPos:         statement.StartPos,
				EndPos:           statement.EndPos,
			},
		)
	}

	return nil
}

func (checker *Checker) VisitContinueStatement(statement *ast.ContinueStatement) ast.Repr {

	// check statement is inside loop

	if checker.currentFunction().loops == 0 {
		checker.report(
			&ControlStatementError{
				ControlStatement: common.ControlStatementContinue,
				StartPos:         statement.StartPos,
				EndPos:           statement.EndPos,
			},
		)
	}

	return nil
}

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
		func() {
			checker.pushActivations()
			defer checker.popActivations()

			checker.visitVariableDeclaration(test, true)

			thenElement.Accept(checker)
		}()

		elseElement.Accept(checker)

	default:
		panic(&errors.UnreachableError{})
	}

	return nil
}

func (checker *Checker) VisitWhileStatement(statement *ast.WhileStatement) ast.Repr {

	testExpression := statement.Test
	testType := testExpression.Accept(checker).(Type)

	if !checker.IsSubType(testType, &BoolType{}) {
		checker.report(
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

	statement.Block.Accept(checker)

	return nil
}

func (checker *Checker) VisitAssignment(assignment *ast.AssignmentStatement) ast.Repr {

	valueType := assignment.Value.Accept(checker).(Type)
	checker.AssignmentStatementValueTypes[assignment] = valueType

	targetType := checker.visitAssignmentValueType(assignment, valueType)
	checker.AssignmentStatementTargetTypes[assignment] = targetType

	return nil
}

func (checker *Checker) visitAssignmentValueType(assignment *ast.AssignmentStatement, valueType Type) (targetType Type) {
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
) (targetType Type) {
	identifier := target.Identifier.Identifier

	// check identifier was declared before
	variable := checker.findVariable(identifier)
	if variable == nil {
		checker.report(
			&NotDeclaredError{
				ExpectedKind: common.DeclarationKindVariable,
				Name:         identifier,
				Pos:          target.StartPosition(),
			},
		)

		return &InvalidType{}
	} else {
		// check identifier is not a constant
		if variable.IsConstant {
			checker.report(
				&AssignmentToConstantError{
					Name:     identifier,
					StartPos: target.StartPosition(),
					EndPos:   target.EndPosition(),
				},
			)
		}

		// check value type is subtype of variable type
		if !isInvalidType(valueType) &&
			!checker.IsSubType(valueType, variable.Type) {

			checker.report(
				&TypeMismatchError{
					ExpectedType: variable.Type,
					ActualType:   valueType,
					StartPos:     assignment.Value.StartPosition(),
					EndPos:       assignment.Value.EndPosition(),
				},
			)
		}

		return variable.Type
	}
}

func (checker *Checker) visitIndexExpressionAssignment(
	assignment *ast.AssignmentStatement,
	target *ast.IndexExpression,
	valueType Type,
) (elementType Type) {

	elementType = checker.visitIndexingExpression(target.Expression, target.Index, true)

	if elementType == nil {
		return &InvalidType{}
	}

	if !isInvalidType(elementType) &&
		!checker.IsSubType(valueType, elementType) {

		checker.report(
			&TypeMismatchError{
				ExpectedType: elementType,
				ActualType:   valueType,
				StartPos:     assignment.Value.StartPosition(),
				EndPos:       assignment.Value.EndPosition(),
			},
		)
	}

	return elementType
}

func (checker *Checker) visitMemberExpressionAssignment(
	assignment *ast.AssignmentStatement,
	target *ast.MemberExpression,
	valueType Type,
) (memberType Type) {

	member := checker.visitMember(target)

	if member == nil {
		return
	}

	// check member is not constant

	if member.VariableKind == ast.VariableKindConstant {
		if member.IsInitialized {
			checker.report(
				&AssignmentToConstantMemberError{
					Name:     target.Identifier.Identifier,
					StartPos: assignment.Value.StartPosition(),
					EndPos:   assignment.Value.EndPosition(),
				},
			)
		}
	}

	member.IsInitialized = true

	// if value type is valid, check value can be assigned to member
	if !isInvalidType(valueType) &&
		!checker.IsSubType(valueType, member.Type) {

		checker.report(
			&TypeMismatchError{
				ExpectedType: member.Type,
				ActualType:   valueType,
				StartPos:     assignment.Value.StartPosition(),
				EndPos:       assignment.Value.EndPosition(),
			},
		)
	}

	return member.Type
}

// visitIndexingExpression checks if the indexed expression is indexable,
// checks if the indexing expression can be used to index into the indexed expression,
// and returns the expected element type
//
func (checker *Checker) visitIndexingExpression(
	indexedExpression ast.Expression,
	indexingExpression ast.Expression,
	isAssignment bool,
) Type {

	indexedType := indexedExpression.Accept(checker).(Type)
	indexingType := indexingExpression.Accept(checker).(Type)

	// NOTE: check indexed type first for UX reasons

	// check indexed expression's type is indexable
	// by getting the expected element

	if isInvalidType(indexedType) {
		return &InvalidType{}
	}

	elementType := checker.IndexableElementType(indexedType, isAssignment)
	if elementType == nil {
		elementType = &InvalidType{}

		checker.report(
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
			checker.report(
				&NotIndexingTypeError{
					Type:     indexingType,
					StartPos: indexingExpression.StartPosition(),
					EndPos:   indexingExpression.EndPosition(),
				},
			)
		}
	}

	return elementType
}

func (checker *Checker) VisitIdentifierExpression(expression *ast.IdentifierExpression) ast.Repr {
	variable := checker.findAndCheckVariable(expression, true)
	if variable == nil {
		return &InvalidType{}
	}

	return variable.Type
}

func (checker *Checker) findAndCheckVariable(expression *ast.IdentifierExpression, recordOrigin bool) *Variable {
	identifier := expression.Identifier.Identifier
	variable := checker.findVariable(identifier)
	if variable == nil {
		checker.report(
			&NotDeclaredError{
				ExpectedKind: common.DeclarationKindValue,
				Name:         identifier,
				Pos:          expression.StartPosition(),
			},
		)
		return nil
	}

	if recordOrigin {
		checker.recordOrigin(
			expression.StartPosition(),
			expression.EndPosition(),
			variable,
		)
	}

	return variable
}

func (checker *Checker) visitBinaryOperation(expr *ast.BinaryExpression) (left, right Type) {
	left = expr.Left.Accept(checker).(Type)
	right = expr.Right.Accept(checker).(Type)
	return
}

func (checker *Checker) VisitBinaryExpression(expression *ast.BinaryExpression) ast.Repr {

	leftType, rightType := checker.visitBinaryOperation(expression)

	leftIsInvalid := isInvalidType(leftType)
	rightIsInvalid := isInvalidType(rightType)
	anyInvalid := leftIsInvalid || rightIsInvalid

	operation := expression.Operation
	operationKind := binaryOperationKind(operation)

	switch operationKind {
	case BinaryOperationKindIntegerArithmetic,
		BinaryOperationKindIntegerComparison:

		return checker.checkBinaryExpressionIntegerArithmeticOrComparison(
			expression, operation, operationKind,
			leftType, rightType,
			leftIsInvalid, rightIsInvalid, anyInvalid,
		)

	case BinaryOperationKindEquality:

		return checker.checkBinaryExpressionEquality(
			expression, operation, operationKind,
			leftType, rightType,
			leftIsInvalid, rightIsInvalid, anyInvalid,
		)

	case BinaryOperationKindBooleanLogic:

		return checker.checkBinaryExpressionBooleanLogic(
			expression, operation, operationKind,
			leftType, rightType,
			leftIsInvalid, rightIsInvalid, anyInvalid,
		)

	case BinaryOperationKindNilCoalescing:
		resultType := checker.checkBinaryExpressionNilCoalescing(
			expression, operation, operationKind,
			leftType, rightType,
			leftIsInvalid, rightIsInvalid, anyInvalid,
		)

		checker.BinaryExpressionResultTypes[expression] = resultType
		checker.BinaryExpressionRightTypes[expression] = rightType

		return resultType

	case BinaryOperationKindConcatenation:
		return checker.checkBinaryExpressionConcatenation(
			expression, operation, operationKind,
			leftType, rightType,
			leftIsInvalid, rightIsInvalid, anyInvalid,
		)
	}

	panic(&unsupportedOperation{
		kind:      common.OperationKindBinary,
		operation: operation,
		startPos:  expression.StartPosition(),
		endPos:    expression.EndPosition(),
	})
}

func (checker *Checker) checkBinaryExpressionIntegerArithmeticOrComparison(
	expression *ast.BinaryExpression,
	operation ast.Operation,
	operationKind BinaryOperationKind,
	leftType, rightType Type,
	leftIsInvalid, rightIsInvalid, anyInvalid bool,
) Type {
	// check both types are integer subtypes

	leftIsInteger := checker.IsSubType(leftType, &IntegerType{})
	rightIsInteger := checker.IsSubType(rightType, &IntegerType{})

	if !leftIsInteger && !rightIsInteger {
		if !anyInvalid {
			checker.report(
				&InvalidBinaryOperandsError{
					Operation: operation,
					LeftType:  leftType,
					RightType: rightType,
					StartPos:  expression.StartPosition(),
					EndPos:    expression.EndPosition(),
				},
			)
		}
	} else if !leftIsInteger {
		if !leftIsInvalid {
			checker.report(
				&InvalidBinaryOperandError{
					Operation:    operation,
					Side:         common.OperandSideLeft,
					ExpectedType: &IntegerType{},
					ActualType:   leftType,
					StartPos:     expression.Left.StartPosition(),
					EndPos:       expression.Left.EndPosition(),
				},
			)
		}
	} else if !rightIsInteger {
		if !rightIsInvalid {
			checker.report(
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
	}
	// check both types are equal
	if !leftType.Equal(rightType) {
		checker.report(
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
		return leftType
	case BinaryOperationKindIntegerComparison:
		return &BoolType{}
	}

	panic(&errors.UnreachableError{})
}

func (checker *Checker) checkBinaryExpressionEquality(
	expression *ast.BinaryExpression,
	operation ast.Operation,
	operationKind BinaryOperationKind,
	leftType, rightType Type,
	leftIsInvalid, rightIsInvalid, anyInvalid bool,
) (resultType Type) {
	// check both types are equal, and boolean subtypes or integer subtypes

	resultType = &BoolType{}

	if !anyInvalid &&
		leftType != nil &&
		!(checker.isValidEqualityType(leftType) &&
			checker.compatibleEqualityTypes(leftType, rightType)) {

		checker.report(
			&InvalidBinaryOperandsError{
				Operation: operation,
				LeftType:  leftType,
				RightType: rightType,
				StartPos:  expression.StartPosition(),
				EndPos:    expression.EndPosition(),
			},
		)
	}

	return
}

func (checker *Checker) isValidEqualityType(ty Type) bool {
	if checker.IsSubType(ty, &BoolType{}) {
		return true
	}

	if checker.IsSubType(ty, &IntegerType{}) {
		return true
	}

	if checker.IsSubType(ty, &StringType{}) {
		return true
	}

	if _, ok := ty.(*OptionalType); ok {
		return true
	}

	return false
}

func (checker *Checker) compatibleEqualityTypes(leftType, rightType Type) bool {
	unwrappedLeft := checker.unwrapOptionalType(leftType)
	unwrappedRight := checker.unwrapOptionalType(rightType)

	if unwrappedLeft.Equal(unwrappedRight) {
		return true
	}

	if _, ok := unwrappedLeft.(*NeverType); ok {
		return true
	}

	if _, ok := unwrappedRight.(*NeverType); ok {
		return true
	}

	return false
}

// unwrapOptionalType returns the type if it is not an optional type,
// or the inner-most type if it is (optional types are repeatedly unwrapped)
//
func (checker *Checker) unwrapOptionalType(ty Type) Type {
	for {
		optionalType, ok := ty.(*OptionalType)
		if !ok {
			return ty
		}
		ty = optionalType.Type
	}
}

func (checker *Checker) checkBinaryExpressionBooleanLogic(
	expression *ast.BinaryExpression,
	operation ast.Operation,
	operationKind BinaryOperationKind,
	leftType, rightType Type,
	leftIsInvalid, rightIsInvalid, anyInvalid bool,
) Type {
	// check both types are integer subtypes

	leftIsBool := checker.IsSubType(leftType, &BoolType{})
	rightIsBool := checker.IsSubType(rightType, &BoolType{})

	if !leftIsBool && !rightIsBool {
		if !anyInvalid {
			checker.report(
				&InvalidBinaryOperandsError{
					Operation: operation,
					LeftType:  leftType,
					RightType: rightType,
					StartPos:  expression.StartPosition(),
					EndPos:    expression.EndPosition(),
				},
			)
		}
	} else if !leftIsBool {
		if !leftIsInvalid {
			checker.report(
				&InvalidBinaryOperandError{
					Operation:    operation,
					Side:         common.OperandSideLeft,
					ExpectedType: &BoolType{},
					ActualType:   leftType,
					StartPos:     expression.Left.StartPosition(),
					EndPos:       expression.Left.EndPosition(),
				},
			)
		}
	} else if !rightIsBool {
		if !rightIsInvalid {
			checker.report(
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
	}

	return &BoolType{}
}

func (checker *Checker) checkBinaryExpressionNilCoalescing(
	expression *ast.BinaryExpression,
	operation ast.Operation,
	operationKind BinaryOperationKind,
	leftType, rightType Type,
	leftIsInvalid, rightIsInvalid, anyInvalid bool,
) Type {
	leftOptional, leftIsOptional := leftType.(*OptionalType)

	if !leftIsInvalid {
		if !leftIsOptional {
			checker.report(
				&InvalidBinaryOperandError{
					Operation:    operation,
					Side:         common.OperandSideLeft,
					ExpectedType: &OptionalType{},
					ActualType:   leftType,
					StartPos:     expression.Left.StartPosition(),
					EndPos:       expression.Left.EndPosition(),
				},
			)
		}
	}

	if leftIsInvalid || !leftIsOptional {
		return &InvalidType{}
	}

	leftInner := leftOptional.Type

	if _, ok := leftInner.(*NeverType); ok {
		return rightType
	} else {
		canNarrow := false

		if !rightIsInvalid {
			if !checker.IsSubType(rightType, leftOptional) {
				checker.report(
					&InvalidBinaryOperandError{
						Operation:    operation,
						Side:         common.OperandSideRight,
						ExpectedType: leftOptional,
						ActualType:   rightType,
						StartPos:     expression.Right.StartPosition(),
						EndPos:       expression.Right.EndPosition(),
					},
				)
			} else {
				canNarrow = checker.IsSubType(rightType, leftInner)
			}
		}

		if !canNarrow {
			return leftOptional
		}
		return leftInner
	}
}

func (checker *Checker) checkBinaryExpressionConcatenation(
	expression *ast.BinaryExpression,
	operation ast.Operation,
	operationKind BinaryOperationKind,
	leftType, rightType Type,
	leftIsInvalid, rightIsInvalid, anyInvalid bool,
) Type {

	// check both types are concatenatable
	leftIsConcat := checker.IsConcatenatableType(leftType)
	rightIsConcat := checker.IsConcatenatableType(rightType)

	if !leftIsConcat && !rightIsConcat {
		if !anyInvalid {
			checker.report(
				&InvalidBinaryOperandsError{
					Operation: operation,
					LeftType:  leftType,
					RightType: rightType,
					StartPos:  expression.StartPosition(),
					EndPos:    expression.EndPosition(),
				},
			)
		}
	} else if !leftIsConcat {
		if !leftIsInvalid {
			checker.report(
				&InvalidBinaryOperandError{
					Operation:    operation,
					Side:         common.OperandSideLeft,
					ExpectedType: rightType,
					ActualType:   leftType,
					StartPos:     expression.Left.StartPosition(),
					EndPos:       expression.Left.EndPosition(),
				},
			)
		}
	} else if !rightIsConcat {
		if !rightIsInvalid {
			checker.report(
				&InvalidBinaryOperandError{
					Operation:    operation,
					Side:         common.OperandSideRight,
					ExpectedType: leftType,
					ActualType:   rightType,
					StartPos:     expression.Right.StartPosition(),
					EndPos:       expression.Right.EndPosition(),
				},
			)
		}
	}

	// check both types are equal
	if !leftType.Equal(rightType) {
		checker.report(
			&InvalidBinaryOperandsError{
				Operation: operation,
				LeftType:  leftType,
				RightType: rightType,
				StartPos:  expression.StartPosition(),
				EndPos:    expression.EndPosition(),
			},
		)
	}

	return leftType
}

func (checker *Checker) VisitUnaryExpression(expression *ast.UnaryExpression) ast.Repr {

	valueType := expression.Expression.Accept(checker).(Type)

	switch expression.Operation {
	case ast.OperationNegate:
		if !checker.IsSubType(valueType, &BoolType{}) {
			checker.report(
				&InvalidUnaryOperandError{
					Operation:    expression.Operation,
					ExpectedType: &BoolType{},
					ActualType:   valueType,
					StartPos:     expression.Expression.StartPosition(),
					EndPos:       expression.Expression.EndPosition(),
				},
			)
		}
		return valueType

	case ast.OperationMinus:
		if !checker.IsSubType(valueType, &IntegerType{}) {
			checker.report(
				&InvalidUnaryOperandError{
					Operation:    expression.Operation,
					ExpectedType: &IntegerType{},
					ActualType:   valueType,
					StartPos:     expression.Expression.StartPosition(),
					EndPos:       expression.Expression.EndPosition(),
				},
			)
		}
		return valueType

	// TODO: check
	case ast.OperationMove:
		return valueType
	}

	panic(&unsupportedOperation{
		kind:      common.OperationKindUnary,
		operation: expression.Operation,
		startPos:  expression.StartPos,
		endPos:    expression.EndPos,
	})
}

func (checker *Checker) VisitExpressionStatement(statement *ast.ExpressionStatement) ast.Repr {
	statement.Expression.Accept(checker)
	return nil
}

func (checker *Checker) VisitBoolExpression(expression *ast.BoolExpression) ast.Repr {
	return &BoolType{}
}

func (checker *Checker) VisitNilExpression(expression *ast.NilExpression) ast.Repr {
	// TODO: verify
	return &OptionalType{
		Type: &NeverType{},
	}
}

func (checker *Checker) VisitIntExpression(expression *ast.IntExpression) ast.Repr {
	return &IntType{}
}

func (checker *Checker) VisitStringExpression(expression *ast.StringExpression) ast.Repr {
	return &StringType{}
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
			checker.report(
				&TypeMismatchError{
					ExpectedType: elementType,
					ActualType:   valueType,
					StartPos:     value.StartPosition(),
					EndPos:       value.EndPosition(),
				},
			)
		}
	}

	if elementType == nil {
		elementType = &NeverType{}
	}

	return &VariableSizedType{
		Type: elementType,
	}
}

func (checker *Checker) VisitDictionaryExpression(expression *ast.DictionaryExpression) ast.Repr {

	// visit all entries, ensure key are all the same type,
	// and values are all the same type

	var keyType, valueType Type

	for _, entry := range expression.Entries {
		entryKeyType := entry.Key.Accept(checker).(Type)
		entryValueType := entry.Value.Accept(checker).(Type)

		// infer key type from first entry's key
		// TODO: find common super type?
		if keyType == nil {
			keyType = entryKeyType
		} else if !checker.IsSubType(entryKeyType, keyType) {
			checker.report(
				&TypeMismatchError{
					ExpectedType: keyType,
					ActualType:   entryKeyType,
					StartPos:     entry.Key.StartPosition(),
					EndPos:       entry.Key.EndPosition(),
				},
			)
		}

		// infer value type from first entry's value
		// TODO: find common super type?
		if valueType == nil {
			valueType = entryValueType
		} else if !checker.IsSubType(entryValueType, valueType) {
			checker.report(
				&TypeMismatchError{
					ExpectedType: valueType,
					ActualType:   entryValueType,
					StartPos:     entry.Value.StartPosition(),
					EndPos:       entry.Value.EndPosition(),
				},
			)
		}
	}

	if keyType == nil {
		keyType = &NeverType{}
	}
	if valueType == nil {
		valueType = &NeverType{}
	}

	return &DictionaryType{
		KeyType:   keyType,
		ValueType: valueType,
	}
}

func (checker *Checker) VisitMemberExpression(expression *ast.MemberExpression) ast.Repr {

	member := checker.visitMember(expression)

	var memberType Type = &InvalidType{}
	if member != nil {
		memberType = member.Type
	}

	return memberType
}

func (checker *Checker) visitMember(expression *ast.MemberExpression) *Member {

	expressionType := expression.Expression.Accept(checker).(Type)

	identifier := expression.Identifier.Identifier

	var member *Member
	switch ty := expressionType.(type) {
	case *StructureType:
		member = ty.Members[identifier]
	case *InterfaceType:
		member = ty.Members[identifier]
	case *StringType:
		member = stringMembers[identifier]
	case ArrayType:
		member = getArrayMember(ty, identifier)
	case *DictionaryType:
		member = getDictionaryMember(ty, identifier)
	}

	if !isInvalidType(expressionType) && member == nil {
		checker.report(
			&NotDeclaredMemberError{
				Type:     expressionType,
				Name:     identifier,
				StartPos: expression.Identifier.StartPosition(),
				EndPos:   expression.Identifier.EndPosition(),
			},
		)
	}

	return member
}

func (checker *Checker) VisitIndexExpression(expression *ast.IndexExpression) ast.Repr {
	return checker.visitIndexingExpression(expression.Expression, expression.Index, false)
}

func (checker *Checker) VisitConditionalExpression(expression *ast.ConditionalExpression) ast.Repr {

	thenType, elseType := checker.visitConditional(expression.Test, expression.Then, expression.Else)

	if thenType == nil || elseType == nil {
		panic(&errors.UnreachableError{})
	}

	// TODO: improve
	resultType := thenType

	if !checker.IsSubType(elseType, resultType) {
		checker.report(
			&TypeMismatchError{
				ExpectedType: resultType,
				ActualType:   elseType,
				StartPos:     expression.Else.StartPosition(),
				EndPos:       expression.Else.EndPosition(),
			},
		)
	}

	return resultType
}

func (checker *Checker) VisitInvocationExpression(invocationExpression *ast.InvocationExpression) ast.Repr {

	// check the invoked expression can be invoked

	invokedExpression := invocationExpression.InvokedExpression
	expressionType := invokedExpression.Accept(checker).(Type)

	var returnType Type = &InvalidType{}
	functionType, ok := expressionType.(*FunctionType)
	if !ok {

		if !isInvalidType(expressionType) {
			checker.report(
				&NotCallableError{
					Type:     expressionType,
					StartPos: invokedExpression.StartPosition(),
					EndPos:   invokedExpression.EndPosition(),
				},
			)
		}
	} else {
		// invoked expression has function type

		argumentTypes := checker.checkInvocationArguments(invocationExpression, functionType)

		// if the invocation refers directly to the name of the function as stated in the declaration,
		// or the invocation refers to a function of a structure (member),
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

		parameterTypes := functionType.ParameterTypes
		if len(argumentTypes) == len(parameterTypes) &&
			functionType.Apply != nil {

			returnType = functionType.Apply(argumentTypes)
		} else {
			returnType = functionType.ReturnType
		}

		checker.InvocationExpressionArgumentTypes[invocationExpression] = argumentTypes
		checker.InvocationExpressionParameterTypes[invocationExpression] = parameterTypes
	}

	return returnType
}

func (checker *Checker) checkIdentifierInvocationArgumentLabels(
	invocationExpression *ast.InvocationExpression,
	identifierExpression *ast.IdentifierExpression,
) {

	variable := checker.findAndCheckVariable(identifierExpression, false)

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
						StartPos:              *argument.LabelStartPos,
						EndPos:                *argument.LabelEndPos,
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
						StartPos:              argument.Expression.StartPosition(),
						EndPos:                argument.Expression.EndPosition(),
					},
				)
			} else if providedLabel != argumentLabel {
				checker.report(
					&IncorrectArgumentLabelError{
						ActualArgumentLabel:   providedLabel,
						ExpectedArgumentLabel: argumentLabel,
						StartPos:              *argument.LabelStartPos,
						EndPos:                *argument.LabelEndPos,
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
	parameterCount := len(functionType.ParameterTypes)
	if argumentCount != parameterCount {

		// TODO: improve
		if functionType.RequiredArgumentCount == nil ||
			argumentCount < *functionType.RequiredArgumentCount {

			checker.report(
				&ArgumentCountError{
					ParameterCount: parameterCount,
					ArgumentCount:  argumentCount,
					StartPos:       invocationExpression.StartPosition(),
					EndPos:         invocationExpression.EndPosition(),
				},
			)
		}
	}

	minCount := argumentCount
	if parameterCount < argumentCount {
		minCount = parameterCount
	}

	for i := 0; i < minCount; i++ {
		// ensure the type of the argument matches the type of the parameter

		parameterType := functionType.ParameterTypes[i]
		argument := invocationExpression.Arguments[i]

		argumentType := argument.Expression.Accept(checker).(Type)

		argumentTypes = append(argumentTypes, argumentType)

		if !checker.IsSubType(argumentType, parameterType) {
			checker.report(
				&TypeMismatchError{
					ExpectedType: parameterType,
					ActualType:   argumentType,
					StartPos:     argument.Expression.StartPosition(),
					EndPos:       argument.Expression.EndPosition(),
				},
			)
		}
	}

	return argumentTypes
}

func (checker *Checker) VisitFunctionExpression(expression *ast.FunctionExpression) ast.Repr {

	// TODO: infer
	functionType := checker.functionType(expression.Parameters, expression.ReturnTypeAnnotation)

	checker.FunctionExpressionFunctionType[expression] = functionType

	checker.checkFunction(
		expression.Parameters,
		functionType,
		expression.FunctionBlock,
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

// ConvertType converts an AST type representation to a sema type
func (checker *Checker) ConvertType(t ast.Type) Type {
	switch t := t.(type) {
	case *ast.NominalType:
		identifier := t.Identifier.Identifier
		result := checker.FindType(identifier)
		if result == nil {
			checker.report(
				&NotDeclaredError{
					ExpectedKind: common.DeclarationKindType,
					Name:         identifier,
					Pos:          t.Pos,
				},
			)
			return &InvalidType{}
		}
		return result

	case *ast.VariableSizedType:
		elementType := checker.ConvertType(t.Type)
		return &VariableSizedType{
			Type: elementType,
		}

	case *ast.ConstantSizedType:
		elementType := checker.ConvertType(t.Type)
		return &ConstantSizedType{
			Type: elementType,
			Size: t.Size,
		}

	case *ast.FunctionType:
		var parameterTypes []Type
		for _, parameterTypeAnnotation := range t.ParameterTypeAnnotations {
			parameterType := checker.ConvertType(parameterTypeAnnotation.Type)
			parameterTypes = append(parameterTypes, parameterType)
		}

		returnType := checker.ConvertType(t.ReturnTypeAnnotation.Type)

		return &FunctionType{
			ParameterTypes: parameterTypes,
			ReturnType:     returnType,
		}

	case *ast.OptionalType:
		result := checker.ConvertType(t.Type)
		return &OptionalType{result}

	case *ast.DictionaryType:
		keyType := checker.ConvertType(t.KeyType)
		valueType := checker.ConvertType(t.ValueType)

		return &DictionaryType{
			KeyType:   keyType,
			ValueType: valueType,
		}
	}

	panic(&astTypeConversionError{invalidASTType: t})
}

func (checker *Checker) declareFunction(
	identifier ast.Identifier,
	functionType *FunctionType,
	argumentLabels []string,
	recordOrigin bool,
) {
	name := identifier.Identifier

	// check if variable with this identifier is already declared in the current scope
	existingVariable := checker.findVariable(name)
	depth := checker.valueActivations.Depth()
	if existingVariable != nil && existingVariable.Depth == depth {
		checker.report(
			&RedeclarationError{
				Kind:        common.DeclarationKindFunction,
				Name:        name,
				Pos:         identifier.Pos,
				PreviousPos: existingVariable.Pos,
			},
		)
	}

	// variable with this identifier is not declared in current scope, declare it
	variable := &Variable{
		Kind:           common.DeclarationKindFunction,
		IsConstant:     true,
		Depth:          depth,
		Type:           functionType,
		ArgumentLabels: argumentLabels,
		Pos:            &identifier.Pos,
	}
	checker.setVariable(name, variable)
	if recordOrigin {
		checker.recordVariableOrigin(name, variable)
	}
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

func (checker *Checker) functionType(
	parameters []*ast.Parameter,
	returnTypeAnnotation *ast.TypeAnnotation,
) *FunctionType {

	parameterTypes := checker.parameterTypes(parameters)
	convertedReturnType := checker.ConvertType(returnTypeAnnotation.Type)

	return &FunctionType{
		ParameterTypes: parameterTypes,
		ReturnType:     convertedReturnType,
	}
}

func (checker *Checker) parameterTypes(parameters []*ast.Parameter) []Type {

	parameterTypes := make([]Type, len(parameters))

	for i, parameter := range parameters {
		parameterType := checker.ConvertType(parameter.TypeAnnotation.Type)
		parameterTypes[i] = parameterType
	}

	return parameterTypes
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

	if !checker.IsSubType(testType, &BoolType{}) {
		checker.report(
			&TypeMismatchError{
				ExpectedType: &BoolType{},
				ActualType:   testType,
				StartPos:     test.StartPosition(),
				EndPos:       test.EndPosition(),
			},
		)
	}

	thenResult := thenElement.Accept(checker)
	if thenResult != nil {
		thenType = thenResult.(Type)
	}

	elseResult := elseElement.Accept(checker)
	if elseResult != nil {
		elseType = elseResult.(Type)
	}

	return
}

func (checker *Checker) VisitStructureDeclaration(structure *ast.StructureDeclaration) ast.Repr {

	structureType := checker.StructureDeclarationTypes[structure]

	checker.checkMemberIdentifiers(structure.Fields, structure.Functions)

	checker.checkInitializers(
		structure.Initializers,
		structure.Fields,
		structureType,
		structure.Identifier.Identifier,
		structureType.ConstructorParameterTypes,
		initializerKindStructure,
	)

	if structureType != nil {
		checker.checkFieldsInitialized(structure, structureType)

		checker.checkStructureFunctions(structure.Functions, structureType)

		// check structure conforms to interfaces.
		// NOTE: perform after completing structure type (e.g. setting constructor parameter types)

		for _, interfaceType := range structureType.Conformances {
			checker.checkStructureConformance(structureType, interfaceType, structure.Identifier.Pos)
		}
	}

	return nil
}

func (checker *Checker) declareStructureDeclaration(structure *ast.StructureDeclaration) {

	// NOTE: fields and functions might already refer to structure itself.
	// insert a dummy type for now, so lookup succeeds during conversion,
	// then fix up the type reference

	structureType := &StructureType{}

	identifier := structure.Identifier

	checker.declareType(identifier, structureType)
	checker.recordVariableOrigin(
		identifier.Identifier,
		&Variable{
			Kind:       common.DeclarationKindStructure,
			IsConstant: true,
			Type:       structureType,
			Pos:        &identifier.Pos,
		},
	)

	conformances := checker.conformances(structure)

	members := checker.members(
		structure.Fields,
		structure.Functions,
		true,
	)

	*structureType = StructureType{
		Identifier:   identifier.Identifier,
		Members:      members,
		Conformances: conformances,
	}

	// TODO: support multiple overloaded initializers

	var parameterTypes []Type
	initializerCount := len(structure.Initializers)
	if initializerCount > 0 {
		firstInitializer := structure.Initializers[0]
		parameterTypes = checker.parameterTypes(firstInitializer.Parameters)

		if initializerCount > 1 {
			checker.report(
				&UnsupportedOverloadingError{
					DeclarationKind: common.DeclarationKindInitializer,
					StartPos:        firstInitializer.StartPosition(),
					EndPos:          firstInitializer.EndPosition(),
				},
			)
		}
	}

	structureType.ConstructorParameterTypes = parameterTypes

	checker.StructureDeclarationTypes[structure] = structureType

	// declare constructor

	checker.declareStructureConstructor(structure, structureType, parameterTypes)
}

func (checker *Checker) conformances(structure *ast.StructureDeclaration) []*InterfaceType {

	var interfaceTypes []*InterfaceType
	seenConformances := map[string]bool{}

	structureIdentifier := structure.Identifier.Identifier

	for _, conformance := range structure.Conformances {
		convertedType := checker.ConvertType(conformance)

		if interfaceType, ok := convertedType.(*InterfaceType); ok {
			interfaceTypes = append(interfaceTypes, interfaceType)

		} else if !isInvalidType(convertedType) {
			checker.report(
				&InvalidConformanceError{
					Type: convertedType,
					Pos:  conformance.Pos,
				},
			)
		}

		conformanceIdentifier := conformance.Identifier.Identifier

		if seenConformances[conformanceIdentifier] {
			checker.report(
				&DuplicateConformanceError{
					StructureIdentifier: structureIdentifier,
					Conformance:         conformance,
				},
			)

		}
		seenConformances[conformanceIdentifier] = true
	}
	return interfaceTypes
}

func (checker *Checker) checkStructureConformance(
	structureType *StructureType,
	interfaceType *InterfaceType,
	pos ast.Position,
) {
	var missingMembers []*Member
	var memberMismatches []MemberMismatch
	var initializerMismatch *InitializerMismatch

	if interfaceType.InitializerParameterTypes != nil {

		structureInitializerType := &FunctionType{
			ParameterTypes: structureType.ConstructorParameterTypes,
			ReturnType:     &VoidType{},
		}
		interfaceInitializerType := &FunctionType{
			ParameterTypes: interfaceType.InitializerParameterTypes,
			ReturnType:     &VoidType{},
		}

		// TODO: subtype?
		if !structureInitializerType.Equal(interfaceInitializerType) {
			initializerMismatch = &InitializerMismatch{
				StructureParameterTypes: structureType.ConstructorParameterTypes,
				InterfaceParameterTypes: interfaceType.InitializerParameterTypes,
			}
		}
	}

	for name, interfaceMember := range interfaceType.Members {

		structureMember, ok := structureType.Members[name]
		if !ok {
			missingMembers = append(missingMembers, interfaceMember)
			continue
		}

		if !checker.memberSatisfied(structureMember, interfaceMember) {
			memberMismatches = append(memberMismatches,
				MemberMismatch{
					StructureMember: structureMember,
					InterfaceMember: interfaceMember,
				},
			)
		}
	}

	if len(missingMembers) > 0 ||
		len(memberMismatches) > 0 ||
		initializerMismatch != nil {

		checker.report(
			&ConformanceError{
				StructureType:       structureType,
				InterfaceType:       interfaceType,
				Pos:                 pos,
				InitializerMismatch: initializerMismatch,
				MissingMembers:      missingMembers,
				MemberMismatches:    memberMismatches,
			},
		)
	}
}

func (checker *Checker) memberSatisfied(structureMember, interfaceMember *Member) bool {
	// TODO: subtype?
	if !structureMember.Type.Equal(interfaceMember.Type) {
		return false
	}

	if interfaceMember.VariableKind != ast.VariableKindNotSpecified &&
		structureMember.VariableKind != interfaceMember.VariableKind {

		return false
	}

	return true
}

// TODO: very simple field initialization check for now.
//  perform proper definite assignment analysis
//
func (checker *Checker) checkFieldsInitialized(
	structure *ast.StructureDeclaration,
	structureType *StructureType,
) {

	for _, field := range structure.Fields {
		name := field.Identifier.Identifier
		member := structureType.Members[name]

		if !member.IsInitialized {
			checker.report(
				&FieldUninitializedError{
					Name:          name,
					Pos:           field.Identifier.Pos,
					StructureType: structureType,
				},
			)
		}
	}
}

func (checker *Checker) declareStructureConstructor(
	structure *ast.StructureDeclaration,
	structureType *StructureType,
	parameterTypes []Type,
) {
	functionType := &FunctionType{
		ReturnType: structureType,
	}

	var argumentLabels []string

	// TODO: support multiple overloaded initializers

	if len(structure.Initializers) > 0 {
		firstInitializer := structure.Initializers[0]

		argumentLabels = checker.argumentLabels(firstInitializer.Parameters)

		functionType = &FunctionType{
			ParameterTypes: parameterTypes,
			ReturnType:     structureType,
		}

		checker.InitializerFunctionTypes[firstInitializer] = functionType
	}

	checker.declareFunction(
		structure.Identifier,
		functionType,
		argumentLabels,
		false,
	)
}

func (checker *Checker) members(
	fields []*ast.FieldDeclaration,
	functions []*ast.FunctionDeclaration,
	requireVariableKind bool,
) map[string]*Member {

	fieldCount := len(fields)
	functionCount := len(functions)

	members := make(map[string]*Member, fieldCount+functionCount)

	// declare a member for each field
	for _, field := range fields {
		fieldType := checker.ConvertType(field.TypeAnnotation.Type)

		members[field.Identifier.Identifier] = &Member{
			Type:          fieldType,
			VariableKind:  field.VariableKind,
			IsInitialized: false,
		}

		if requireVariableKind &&
			field.VariableKind == ast.VariableKindNotSpecified {

			checker.report(
				&InvalidVariableKindError{
					Kind:     field.VariableKind,
					StartPos: field.Identifier.Pos,
					EndPos:   field.Identifier.Pos,
				},
			)
		}
	}

	// declare a member for each function
	for _, function := range functions {
		functionType := checker.functionType(function.Parameters, function.ReturnTypeAnnotation)

		argumentLabels := checker.argumentLabels(function.Parameters)

		members[function.Identifier.Identifier] = &Member{
			Type:           functionType,
			VariableKind:   ast.VariableKindConstant,
			IsInitialized:  true,
			ArgumentLabels: argumentLabels,
		}
	}

	return members
}

func (checker *Checker) checkInitializers(
	initializers []*ast.InitializerDeclaration,
	fields []*ast.FieldDeclaration,
	structureType Type,
	typeIdentifier string,
	initializerParameterTypes []Type,
	initializerKind initializerKind,
) {
	count := len(initializers)

	if count == 0 {
		checker.checkNoInitializerNoFields(fields, initializerKind, typeIdentifier)
		return
	}

	// TODO: check all initializers:
	//  parameter initializerParameterTypes needs to be a slice

	initializer := initializers[0]
	checker.checkInitializer(
		initializer,
		fields,
		structureType,
		typeIdentifier,
		initializerParameterTypes,
		initializerKind,
	)
}

// checkNoInitializerNoFields checks that if there are no initializers
// there are also no fields – otherwise the fields will be uninitialized.
// In interfaces this is allowed.
//
func (checker *Checker) checkNoInitializerNoFields(
	fields []*ast.FieldDeclaration,
	initializerKind initializerKind,
	typeIdentifier string,
) {
	if len(fields) == 0 || initializerKind == initializerKindInterface {
		return
	}

	// report error for first field
	firstField := fields[0]

	checker.report(
		&MissingInitializerError{
			TypeIdentifier: typeIdentifier,
			FirstFieldName: firstField.Identifier.Identifier,
			FirstFieldPos:  firstField.Identifier.Pos,
		},
	)
}

func (checker *Checker) checkInitializer(
	initializer *ast.InitializerDeclaration,
	fields []*ast.FieldDeclaration,
	structureType Type,
	typeIdentifier string,
	initializerParameterTypes []Type,
	initializerKind initializerKind,
) {
	// NOTE: new activation, so `self`
	// is only visible inside initializer

	checker.valueActivations.PushCurrent()
	defer checker.valueActivations.Pop()

	checker.declareSelfValue(structureType)

	// check the initializer is named properly
	identifier := initializer.Identifier.Identifier
	if identifier != InitializerIdentifier {
		checker.report(
			&InvalidInitializerNameError{
				Name: identifier,
				Pos:  initializer.StartPos,
			},
		)
	}

	functionType := &FunctionType{
		ParameterTypes: initializerParameterTypes,
		ReturnType:     &VoidType{},
	}

	checker.checkFunction(
		initializer.Parameters,
		functionType,
		initializer.FunctionBlock,
	)

	if initializerKind == initializerKindInterface &&
		initializer.FunctionBlock != nil {

		// TODO: use correct container declaration kind
		checker.checkInterfaceFunctionBlock(
			initializer.FunctionBlock,
			common.DeclarationKindStructure,
			common.DeclarationKindInitializer,
		)
	}
}

func (checker *Checker) checkStructureFunctions(
	functions []*ast.FunctionDeclaration,
	selfType *StructureType,
) {
	for _, function := range functions {
		func() {
			// NOTE: new activation, as function declarations
			// shouldn'T be visible in other function declarations,
			// and `self` is is only visible inside function
			checker.valueActivations.PushCurrent()
			defer checker.valueActivations.Pop()

			checker.declareSelfValue(selfType)

			function.Accept(checker)
		}()
	}
}

func (checker *Checker) declareSelfValue(selfType Type) {

	// NOTE: declare `self` one depth lower ("inside" function),
	// so it can'T be re-declared by the function's parameters

	depth := checker.valueActivations.Depth() + 1

	self := &Variable{
		Kind:       common.DeclarationKindConstant,
		Type:       selfType,
		IsConstant: true,
		Depth:      depth,
		Pos:        nil,
	}
	checker.setVariable(SelfIdentifier, self)
	checker.recordVariableOrigin(SelfIdentifier, self)
}

// checkMemberIdentifiers checks the fields and functions are unique and aren'T named `init`
//
func (checker *Checker) checkMemberIdentifiers(
	fields []*ast.FieldDeclaration,
	functions []*ast.FunctionDeclaration,
) {

	positions := map[string]ast.Position{}

	for _, field := range fields {
		checker.checkMemberIdentifier(
			field.Identifier,
			common.DeclarationKindField,
			positions,
		)
	}

	for _, function := range functions {
		checker.checkMemberIdentifier(
			function.Identifier,
			common.DeclarationKindFunction,
			positions,
		)
	}
}

func (checker *Checker) checkMemberIdentifier(
	identifier ast.Identifier,
	kind common.DeclarationKind,
	positions map[string]ast.Position,
) {
	name := identifier.Identifier
	pos := identifier.Pos

	if name == InitializerIdentifier {
		checker.report(
			&InvalidNameError{
				Name: name,
				Pos:  pos,
			},
		)
	}

	if previousPos, ok := positions[name]; ok {
		checker.report(
			&RedeclarationError{
				Name:        name,
				Pos:         pos,
				Kind:        kind,
				PreviousPos: &previousPos,
			},
		)
	} else {
		positions[name] = pos
	}
}

func (checker *Checker) declareType(
	identifier ast.Identifier,
	newType Type,
) {
	name := identifier.Identifier

	existingType := checker.FindType(name)
	if existingType != nil {
		checker.report(
			&RedeclarationError{
				Kind: common.DeclarationKindType,
				Name: name,
				Pos:  identifier.Pos,
				// TODO: previous pos
			},
		)
	}

	// type with this identifier is not declared in current scope, declare it
	checker.setType(name, newType)
}

func (checker *Checker) VisitFieldDeclaration(field *ast.FieldDeclaration) ast.Repr {

	// NOTE: field type is already checked when determining structure function in `structureType`

	panic(&errors.UnreachableError{})
}

func (checker *Checker) VisitInitializerDeclaration(initializer *ast.InitializerDeclaration) ast.Repr {

	// NOTE: already checked in `checkInitializer`

	panic(&errors.UnreachableError{})
}

func (checker *Checker) VisitInterfaceDeclaration(declaration *ast.InterfaceDeclaration) ast.Repr {

	interfaceType := checker.InterfaceDeclarationTypes[declaration]

	interfaceType.Members = checker.members(
		declaration.Fields,
		declaration.Functions,
		false,
	)

	checker.checkMemberIdentifiers(declaration.Fields, declaration.Functions)

	checker.checkInitializers(
		declaration.Initializers,
		declaration.Fields,
		interfaceType,
		declaration.Identifier.Identifier,
		interfaceType.InitializerParameterTypes,
		initializerKindInterface,
	)

	checker.checkInterfaceFunctions(
		declaration.Functions,
		interfaceType,
		declaration.DeclarationKind(),
	)

	return nil
}

func (checker *Checker) checkInterfaceFunctions(
	functions []*ast.FunctionDeclaration,
	interfaceType Type,
	declarationKind common.DeclarationKind,
) {
	for _, function := range functions {
		func() {
			// NOTE: new activation, as function declarations
			// shouldn'T be visible in other function declarations,
			// and `self` is is only visible inside function
			checker.valueActivations.PushCurrent()
			defer checker.valueActivations.Pop()

			// NOTE: required for
			checker.declareSelfValue(interfaceType)

			function.Accept(checker)

			if function.FunctionBlock != nil {
				checker.checkInterfaceFunctionBlock(
					function.FunctionBlock,
					declarationKind,
					common.DeclarationKindFunction,
				)
			}
		}()
	}
}

func (checker *Checker) declareInterfaceDeclaration(declaration *ast.InterfaceDeclaration) {

	// NOTE: fields and functions might already refer to structure itself.
	// insert a dummy type for now, so lookup succeeds during conversion,
	// then fix up the type reference

	interfaceType := &InterfaceType{}

	identifier := declaration.Identifier

	checker.declareType(identifier, interfaceType)
	checker.recordVariableOrigin(
		identifier.Identifier,
		&Variable{
			Kind:       declaration.DeclarationKind(),
			IsConstant: true,
			Type:       interfaceType,
			Pos:        &identifier.Pos,
		},
	)

	// NOTE: members are added in `VisitInterfaceDeclaration` –
	//   left out for now, as field and function requirements could refer to e.g. structures
	*interfaceType = InterfaceType{
		Identifier: identifier.Identifier,
	}

	// TODO: support multiple overloaded initializers

	var parameterTypes []Type
	initializerCount := len(declaration.Initializers)
	if initializerCount > 0 {
		firstInitializer := declaration.Initializers[0]
		parameterTypes = checker.parameterTypes(firstInitializer.Parameters)

		if initializerCount > 1 {
			checker.report(
				&UnsupportedOverloadingError{
					DeclarationKind: common.DeclarationKindInitializer,
					StartPos:        firstInitializer.StartPosition(),
					EndPos:          firstInitializer.EndPosition(),
				},
			)
		}
	}

	interfaceType.InitializerParameterTypes = parameterTypes

	checker.InterfaceDeclarationTypes[declaration] = interfaceType

	// declare value

	checker.declareInterfaceMetaType(declaration, interfaceType)
}

func (checker *Checker) checkInterfaceFunctionBlock(
	block *ast.FunctionBlock,
	containerKind common.DeclarationKind,
	implementedKind common.DeclarationKind,
) {

	if len(block.Statements) > 0 {
		checker.report(
			&InvalidImplementationError{
				Pos:             block.Statements[0].StartPosition(),
				ContainerKind:   containerKind,
				ImplementedKind: implementedKind,
			},
		)
	} else if len(block.PreConditions) == 0 &&
		len(block.PostConditions) == 0 {

		checker.report(
			&InvalidImplementationError{
				Pos:             block.StartPos,
				ContainerKind:   containerKind,
				ImplementedKind: implementedKind,
			},
		)
	}
}

func (checker *Checker) declareInterfaceMetaType(
	declaration *ast.InterfaceDeclaration,
	interfaceType *InterfaceType,
) {
	metaType := &InterfaceMetaType{
		InterfaceType: interfaceType,
	}

	checker.declareVariable(
		declaration.Identifier.Identifier,
		metaType,
		// TODO: check
		declaration.DeclarationKind(),
		declaration.Identifier.Pos,
		true,
		nil,
	)
}

func (checker *Checker) recordOrigin(startPos, endPos ast.Position, variable *Variable) {
	checker.Origins.Put(startPos, endPos, variable)
}

func (checker *Checker) recordVariableOrigin(name string, variable *Variable) {
	if variable.Pos == nil {
		return
	}
	startPos := *variable.Pos
	endPos := variable.Pos.Shifted(len(name) - 1)
	checker.recordOrigin(startPos, endPos, variable)
}

func (checker *Checker) VisitImportDeclaration(declaration *ast.ImportDeclaration) ast.Repr {

	imports := checker.Program.Imports()
	imported := imports[declaration.Location]
	if imported == nil {
		checker.report(
			&UnresolvedImportError{
				ImportLocation: declaration.Location,
				StartPos:       declaration.LocationPos,
				EndPos:         declaration.LocationPos,
			},
		)
		return nil
	}

	if checker.seenImports[declaration.Location] {
		checker.report(
			&RepeatedImportError{
				ImportLocation: declaration.Location,
				StartPos:       declaration.LocationPos,
				EndPos:         declaration.LocationPos,
			},
		)
		return nil
	}
	checker.seenImports[declaration.Location] = true

	importChecker, ok := checker.ImportCheckers[declaration.Location]
	var checkerErr *CheckerError
	if !ok || importChecker == nil {
		var err error
		importChecker, err = NewChecker(
			imported,
			checker.PredeclaredValues,
			checker.PredeclaredTypes,
		)
		if err == nil {
			checker.ImportCheckers[declaration.Location] = importChecker
		}
	}

	// NOTE: ignore generic `error` result, get internal *CheckerError
	_ = importChecker.Check()
	checkerErr = importChecker.checkerError()

	if checkerErr != nil {
		checker.report(
			&ImportedProgramError{
				CheckerError:   checkerErr,
				ImportLocation: declaration.Location,
				Pos:            declaration.LocationPos,
			},
		)
		return nil
	}

	missing := make(map[ast.Identifier]bool, len(declaration.Identifiers))
	for _, identifier := range declaration.Identifiers {
		missing[identifier] = true
	}

	checker.importValues(declaration, importChecker, missing)
	checker.importTypes(declaration, importChecker, missing)

	for identifier, _ := range missing {
		checker.report(
			&NotExportedError{
				Name:           identifier.Identifier,
				ImportLocation: declaration.Location,
				Pos:            identifier.Pos,
			},
		)

		// NOTE: declare constant variable with invalid type to silence rest of program
		checker.declareVariable(
			identifier.Identifier,
			&InvalidType{},
			common.DeclarationKindValue,
			identifier.Pos,
			true,
			nil,
		)

		// NOTE: declare type with invalid type to silence rest of program
		checker.declareType(identifier, &InvalidType{})
	}

	return nil
}

func (checker *Checker) importValues(
	declaration *ast.ImportDeclaration,
	importChecker *Checker,
	missing map[ast.Identifier]bool,
) {
	// TODO: consider access modifiers

	// determine which identifiers are imported /
	// which variables need to be declared

	var variables map[string]*Variable
	identifierLength := len(declaration.Identifiers)
	if identifierLength > 0 {
		variables = make(map[string]*Variable, identifierLength)
		for _, identifier := range declaration.Identifiers {
			name := identifier.Identifier
			variable := importChecker.GlobalValues[name]
			if variable == nil {
				continue
			}
			variables[name] = variable
			delete(missing, identifier)
		}
	} else {
		variables = importChecker.GlobalValues
	}

	for name, variable := range variables {

		// TODO: improve position
		// TODO: allow cross-module variables?

		// don't import predeclared values
		if _, ok := importChecker.PredeclaredValues[name]; ok {
			continue
		}

		checker.declareVariable(
			name,
			variable.Type,
			variable.Kind,
			declaration.LocationPos,
			true,
			variable.ArgumentLabels,
		)
	}
}

func (checker *Checker) importTypes(
	declaration *ast.ImportDeclaration,
	importChecker *Checker,
	missing map[ast.Identifier]bool,
) {
	// TODO: consider access modifiers

	// determine which identifiers are imported /
	// which types need to be declared

	var types map[string]Type
	identifierLength := len(declaration.Identifiers)
	if identifierLength > 0 {
		types = make(map[string]Type, identifierLength)
		for _, identifier := range declaration.Identifiers {
			name := identifier.Identifier
			ty := importChecker.GlobalTypes[name]
			if ty == nil {
				continue
			}
			types[name] = ty
			delete(missing, identifier)
		}
	} else {
		types = importChecker.GlobalTypes
	}

	for name, ty := range types {

		// TODO: improve position
		// TODO: allow cross-module types?

		// don't import predeclared values
		if _, ok := importChecker.PredeclaredValues[name]; ok {
			continue
		}

		identifier := ast.Identifier{
			Identifier: name,
			Pos:        declaration.LocationPos,
		}
		checker.declareType(identifier, ty)
	}
}

func (checker *Checker) VisitFailableDowncastExpression(expression *ast.FailableDowncastExpression) ast.Repr {

	leftHandExpression := expression.Expression
	leftHandType := leftHandExpression.Accept(checker).(Type)
	rightHandType := checker.ConvertType(expression.TypeAnnotation.Type)
	checker.FailableDowncastingTypes[expression] = rightHandType

	// TODO: non-Any types (interfaces, wrapped (e.g Any?, [Any], etc.)) are not supported for now

	if _, ok := leftHandType.(*AnyType); !ok {

		checker.report(
			&UnsupportedTypeError{
				Type:     leftHandType,
				StartPos: leftHandExpression.StartPosition(),
				EndPos:   leftHandExpression.EndPosition(),
			},
		)
	}

	return &OptionalType{Type: rightHandType}
}
