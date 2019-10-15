package sema

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/common"
)

const ArgumentLabelNotRequired = "_"
const InitializerIdentifier = "init"
const SelfIdentifier = "self"
const BeforeIdentifier = "before"
const ResultIdentifier = "result"

// TODO: move annotations

var beforeType = &FunctionType{
	ParameterTypeAnnotations: NewTypeAnnotations(
		&AnyType{},
	),
	ReturnTypeAnnotation: NewTypeAnnotation(
		&AnyType{},
	),
	GetReturnType: func(argumentTypes []Type) Type {
		return argumentTypes[0]
	},
}

// Checker

type Checker struct {
	Program             *ast.Program
	PredeclaredValues   map[string]ValueDeclaration
	PredeclaredTypes    map[string]TypeDeclaration
	ImportCheckers      map[ast.ImportLocation]*Checker
	errors              []error
	valueActivations    *ValueActivations
	resources           *Resources
	typeActivations     *TypeActivations
	functionActivations *FunctionActivations
	GlobalValues        map[string]*Variable
	GlobalTypes         map[string]Type
	inCondition         bool
	Occurrences         *Occurrences
	variableOrigins     map[*Variable]*Origin
	memberOrigins       map[Type]map[string]*Origin
	seenImports         map[ast.ImportLocation]bool
	isChecked           bool
	inCreate            bool
	Elaboration         *Elaboration
}

func NewChecker(
	program *ast.Program,
	predeclaredValues map[string]ValueDeclaration,
	predeclaredTypes map[string]TypeDeclaration,
) (*Checker, error) {

	checker := &Checker{
		Program:             program,
		PredeclaredValues:   predeclaredValues,
		PredeclaredTypes:    predeclaredTypes,
		ImportCheckers:      map[ast.ImportLocation]*Checker{},
		valueActivations:    NewValueActivations(),
		resources:           &Resources{},
		typeActivations:     NewTypeActivations(baseTypes),
		functionActivations: &FunctionActivations{},
		GlobalValues:        map[string]*Variable{},
		GlobalTypes:         map[string]Type{},
		Occurrences:         NewOccurrences(),
		variableOrigins:     map[*Variable]*Origin{},
		memberOrigins:       map[Type]map[string]*Origin{},
		seenImports:         map[ast.ImportLocation]bool{},
		Elaboration:         NewElaboration(),
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

func (checker *Checker) declareValue(name string, declaration ValueDeclaration) {
	variable, err := checker.valueActivations.Declare(
		name,
		declaration.ValueDeclarationType(),
		declaration.ValueDeclarationKind(),
		declaration.ValueDeclarationPosition(),
		declaration.ValueDeclarationIsConstant(),
		declaration.ValueDeclarationArgumentLabels(),
	)
	checker.report(err)
	checker.recordVariableDeclarationOccurrence(name, variable)
}

func (checker *Checker) declareTypeDeclaration(name string, declaration TypeDeclaration) {
	identifier := ast.Identifier{
		Identifier: name,
		Pos:        declaration.TypeDeclarationPosition(),
	}

	ty := declaration.TypeDeclarationType()
	err := checker.typeActivations.Declare(identifier, ty)
	checker.report(err)
	checker.recordVariableDeclarationOccurrence(
		identifier.Identifier,
		&Variable{
			Identifier: identifier.Identifier,
			Kind:       declaration.TypeDeclarationKind(),
			IsConstant: true,
			Type:       ty,
			Pos:        &identifier.Pos,
		},
	)
}

func (checker *Checker) FindType(name string) Type {
	return checker.typeActivations.Find(name)
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
	for _, err := range errs {
		if err == nil {
			continue
		}
		checker.errors = append(checker.errors, errs...)
	}
}

func (checker *Checker) VisitProgram(program *ast.Program) ast.Repr {

	// pre-declare interfaces, composites, and functions (check afterwards)

	for _, declaration := range program.InterfaceDeclarations() {
		checker.declareInterfaceDeclaration(declaration)
	}

	for _, declaration := range program.CompositeDeclarations() {
		checker.declareCompositeDeclaration(declaration)
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

func (checker *Checker) checkTransfer(transfer *ast.Transfer, valueType Type) {
	if valueType.IsResourceType() {
		if transfer.Operation != ast.TransferOperationMove {
			checker.report(
				&IncorrectTransferOperationError{
					ActualOperation:   transfer.Operation,
					ExpectedOperation: ast.TransferOperationMove,
					Pos:               transfer.Pos,
				},
			)
		}
	} else {
		if transfer.Operation == ast.TransferOperationMove {
			checker.report(
				&IncorrectTransferOperationError{
					ActualOperation:   transfer.Operation,
					ExpectedOperation: ast.TransferOperationCopy,
					Pos:               transfer.Pos,
				},
			)
		}
	}
}

func (checker *Checker) IsTypeCompatible(expression ast.Expression, valueType Type, targetType Type) bool {
	switch typedExpression := expression.(type) {
	case *ast.IntExpression:
		unwrappedTargetType := UnwrapOptionalType(targetType)

		// check if literal value fits range can't be checked when target is Never
		//
		if IsSubType(unwrappedTargetType, &IntegerType{}) &&
			!IsSubType(unwrappedTargetType, &NeverType{}) {

			checker.checkIntegerLiteral(typedExpression, unwrappedTargetType)

			return true
		}
	}

	return IsSubType(valueType, targetType)
}

// checkIntegerLiteral checks that the value of the integer literal
// fits into range of the target integer type
//
func (checker *Checker) checkIntegerLiteral(expression *ast.IntExpression, integerType Type) {
	intRange := integerType.(Ranged)
	literalValue := expression.Value
	rangeMin := intRange.Min()
	rangeMax := intRange.Max()
	if (rangeMin != nil && literalValue.Cmp(rangeMin) == -1) ||
		(rangeMax != nil && literalValue.Cmp(rangeMax) == 1) {

		checker.report(
			&InvalidIntegerLiteralRangeError{
				ExpectedType:     integerType,
				ExpectedRangeMin: rangeMin,
				ExpectedRangeMax: rangeMax,
				StartPos:         expression.StartPosition(),
				EndPos:           expression.EndPosition(),
			},
		)
	}
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
	variable := checker.valueActivations.Find(name)
	if variable == nil {
		return
	}
	checker.GlobalValues[name] = variable
}

func (checker *Checker) declareGlobalType(name string) {
	ty := checker.typeActivations.Find(name)
	if ty == nil {
		return
	}
	checker.GlobalTypes[name] = ty
}

func (checker *Checker) checkResourceMoveOperation(valueExpression ast.Expression, valueType Type) {
	if !valueType.IsResourceType() {
		return
	}

	unaryExpression, ok := valueExpression.(*ast.UnaryExpression)
	if !ok || unaryExpression.Operation != ast.OperationMove {
		checker.report(
			&MissingMoveOperationError{
				Pos: valueExpression.StartPosition(),
			},
		)
		return
	}

	checker.recordResourceInvalidation(
		unaryExpression.Expression,
		valueType,
		ResourceInvalidationKindMove,
	)
}

func (checker *Checker) resourceVariable(exp ast.Expression, valueType Type) (variable *Variable, pos ast.Position) {
	if !valueType.IsResourceType() {
		return
	}

	identifierExpression, ok := exp.(*ast.IdentifierExpression)
	if !ok {
		return
	}

	variable = checker.findAndCheckVariable(identifierExpression.Identifier, false)
	if variable == nil {
		return
	}

	return variable, identifierExpression.Pos
}

func (checker *Checker) inLoop() bool {
	return checker.functionActivations.Current().InLoop()
}

func (checker *Checker) findAndCheckVariable(identifier ast.Identifier, recordOccurrence bool) *Variable {
	variable := checker.valueActivations.Find(identifier.Identifier)
	if variable == nil {
		checker.report(
			&NotDeclaredError{
				ExpectedKind: common.DeclarationKindVariable,
				Name:         identifier.Identifier,
				Pos:          identifier.StartPosition(),
			},
		)
		return nil
	}

	if recordOccurrence {
		checker.recordVariableReferenceOccurrence(
			identifier.StartPosition(),
			identifier.EndPosition(),
			variable,
		)
	}

	return variable
}

// ConvertType converts an AST type representation to a sema type
func (checker *Checker) ConvertType(t ast.Type) Type {
	switch t := t.(type) {
	case *ast.NominalType:
		identifier := t.Identifier.Identifier
		result := checker.typeActivations.Find(identifier)
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
		var parameterTypeAnnotations []*TypeAnnotation
		for _, parameterTypeAnnotation := range t.ParameterTypeAnnotations {
			parameterTypeAnnotation := checker.ConvertTypeAnnotation(parameterTypeAnnotation)
			parameterTypeAnnotations = append(parameterTypeAnnotations,
				parameterTypeAnnotation,
			)
		}

		returnTypeAnnotation := checker.ConvertTypeAnnotation(t.ReturnTypeAnnotation)

		return &FunctionType{
			ParameterTypeAnnotations: parameterTypeAnnotations,
			ReturnTypeAnnotation:     returnTypeAnnotation,
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

// ConvertTypeAnnotation converts an AST type annotation representation
// to a sema type annotation
//
func (checker *Checker) ConvertTypeAnnotation(typeAnnotation *ast.TypeAnnotation) *TypeAnnotation {
	convertedType := checker.ConvertType(typeAnnotation.Type)
	return &TypeAnnotation{
		Move: typeAnnotation.Move,
		Type: convertedType,
	}
}

func (checker *Checker) functionType(
	parameters ast.Parameters,
	returnTypeAnnotation *ast.TypeAnnotation,
) *FunctionType {
	convertedParameterTypeAnnotations :=
		checker.parameterTypeAnnotations(parameters)

	convertedReturnTypeAnnotation :=
		checker.ConvertTypeAnnotation(returnTypeAnnotation)

	return &FunctionType{
		ParameterTypeAnnotations: convertedParameterTypeAnnotations,
		ReturnTypeAnnotation:     convertedReturnTypeAnnotation,
	}
}

func (checker *Checker) parameterTypeAnnotations(parameters ast.Parameters) []*TypeAnnotation {

	parameterTypeAnnotations := make([]*TypeAnnotation, len(parameters))

	for i, parameter := range parameters {
		convertedParameterType := checker.ConvertType(parameter.TypeAnnotation.Type)
		parameterTypeAnnotations[i] = &TypeAnnotation{
			Move: parameter.TypeAnnotation.Move,
			Type: convertedParameterType,
		}
	}

	return parameterTypeAnnotations
}

func (checker *Checker) recordVariableReferenceOccurrence(startPos, endPos ast.Position, variable *Variable) {
	origin, ok := checker.variableOrigins[variable]
	if !ok {
		origin = &Origin{
			Type:            variable.Type,
			DeclarationKind: variable.Kind,
			StartPos:        variable.Pos,
			// TODO:
			EndPos: variable.Pos,
		}
		checker.variableOrigins[variable] = origin
	}
	checker.Occurrences.Put(startPos, endPos, origin)
}

func (checker *Checker) recordVariableDeclarationOccurrence(name string, variable *Variable) {
	if variable.Pos == nil {
		return
	}
	startPos := *variable.Pos
	endPos := variable.Pos.Shifted(len(name) - 1)
	checker.recordVariableReferenceOccurrence(startPos, endPos, variable)
}

func (checker *Checker) recordFieldDeclarationOrigin(
	field *ast.FieldDeclaration,
	fieldType Type,
) *Origin {
	startPosition := field.Identifier.StartPosition()
	endPosition := field.Identifier.EndPosition()

	origin := &Origin{
		Type:            fieldType,
		DeclarationKind: common.DeclarationKindField,
		StartPos:        &startPosition,
		EndPos:          &endPosition,
	}

	checker.Occurrences.Put(
		field.StartPos,
		field.EndPos,
		origin,
	)

	return origin
}

func (checker *Checker) recordFunctionDeclarationOrigin(
	function *ast.FunctionDeclaration,
	functionType *FunctionType,
) *Origin {
	startPosition := function.Identifier.StartPosition()
	endPosition := function.Identifier.EndPosition()

	origin := &Origin{
		Type:            functionType,
		DeclarationKind: common.DeclarationKindFunction,
		StartPos:        &startPosition,
		EndPos:          &endPosition,
	}

	checker.Occurrences.Put(
		startPosition,
		endPosition,
		origin,
	)
	return origin
}

func (checker *Checker) enterValueScope() {
	checker.valueActivations.Enter()
}

func (checker *Checker) leaveValueScope() {
	checker.checkResourceLoss(checker.valueActivations.Depth())
	checker.valueActivations.Leave()
}

// TODO: prune resource variables declared in function's scope
//    from `checker.resources`, so they don't get checked anymore
//    when detecting resource use after invalidation in loops

// checkResourceLoss reports an error if there is a variable in the current scope
// that has a resource type and which was not moved or destroyed
//
func (checker *Checker) checkResourceLoss(depth int) {

	for name, variable := range checker.valueActivations.VariablesDeclaredInAndBelow(depth) {

		// TODO: handle `self` and `result` properly

		if variable.Type.IsResourceType() &&
			variable.Kind != common.DeclarationKindSelf &&
			variable.Kind != common.DeclarationKindResult &&
			!checker.resources.Get(variable).DefinitivelyInvalidated {

			checker.report(
				&ResourceLossError{
					StartPos: *variable.Pos,
					EndPos:   variable.Pos.Shifted(len(name) - 1),
				},
			)

		}
	}
}

func (checker *Checker) withValueScope(f func()) {
	checker.enterValueScope()
	defer checker.leaveValueScope()
	f()
}

func (checker *Checker) recordResourceInvalidation(exp ast.Expression, valueType Type, kind ResourceInvalidationKind) {
	variable, pos := checker.resourceVariable(exp, valueType)
	if variable == nil {
		return
	}
	checker.resources.AddInvalidation(variable,
		ResourceInvalidation{
			Kind: kind,
			Pos:  pos,
		},
	)
}

func (checker *Checker) checkWithResources(
	check func() Type,
	temporaryResources *Resources,
) Type {
	originalResources := checker.resources
	checker.resources = temporaryResources
	defer func() {
		checker.resources = originalResources
	}()

	return check()
}

// checkAccessResourceLoss checks for a resource loss caused by an expression which is accessed
// (indexed or member). This is basically any expression that does not have an identifier
// as its "base" expression.
//
// For example, function invocations, array literals, or dictionary literals will cause a resource loss
// if the expression is accessed immediately: e.g.
//   - `returnResource()[0]`
//   - `[<-create R(), <-create R()][0]`,
//   - `{"resource": <-create R()}.length`
//
// Safe expressions are identifier expressions, an indexing expression into a safe expression,
// or a member access on a safe expression.
//
func (checker *Checker) checkAccessResourceLoss(expressionType Type, expression ast.Expression) {
	if !expressionType.IsResourceType() {
		return
	}

	baseExpression := expression

	for {
		accessExpression, isAccess := baseExpression.(ast.AccessExpression)
		if !isAccess {
			break
		}
		baseExpression = accessExpression.AccessedExpression()
	}

	if _, isIdentifier := baseExpression.(*ast.IdentifierExpression); isIdentifier {
		return
	}

	checker.report(
		&ResourceLossError{
			StartPos: expression.StartPosition(),
			EndPos:   expression.EndPosition(),
		},
	)
}

// checkResourceFieldNesting checks if any resource fields are nested
// in non resource composites (concrete or interface)
//
func (checker *Checker) checkResourceFieldNesting(
	fields map[string]*ast.FieldDeclaration,
	members map[string]*Member,
	compositeKind common.CompositeKind,
) {
	if compositeKind == common.CompositeKindResource {
		return
	}

	for name, member := range members {
		if !member.Type.IsResourceType() {
			continue
		}

		field := fields[name]

		checker.report(
			&InvalidResourceFieldError{
				Name: name,
				Pos:  field.Identifier.Pos,
			},
		)
	}
}
