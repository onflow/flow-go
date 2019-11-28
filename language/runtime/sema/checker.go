package sema

import (
	"math/big"

	"github.com/rivo/uniseg"

	"github.com/dapperlabs/flow-go/language/runtime/ast"
	"github.com/dapperlabs/flow-go/language/runtime/common"
	"github.com/dapperlabs/flow-go/language/runtime/errors"
)

const ArgumentLabelNotRequired = "_"
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
	Program                 *ast.Program
	Location                ast.Location
	PredeclaredValues       map[string]ValueDeclaration
	PredeclaredTypes        map[string]TypeDeclaration
	ImportCheckers          map[ast.LocationID]*Checker
	AccessCheckMode         AccessCheckMode
	errors                  []error
	valueActivations        *VariableActivations
	resources               *Resources
	typeActivations         *VariableActivations
	containerTypes          map[Type]bool
	functionActivations     *FunctionActivations
	GlobalValues            map[string]*Variable
	GlobalTypes             map[string]*Variable
	TransactionTypes        []*TransactionType
	inCondition             bool
	Occurrences             *Occurrences
	variableOrigins         map[*Variable]*Origin
	memberOrigins           map[Type]map[string]*Origin
	seenImports             map[ast.LocationID]bool
	isChecked               bool
	inCreate                bool
	inInvocation            bool
	inAssignment            bool
	Elaboration             *Elaboration
	currentMemberExpression *ast.MemberExpression
}

type Option func(*Checker) error

func WithPredeclaredValues(predeclaredValues map[string]ValueDeclaration) Option {
	return func(checker *Checker) error {
		checker.PredeclaredValues = predeclaredValues

		for name, declaration := range predeclaredValues {
			checker.declareValue(name, declaration)
			checker.declareGlobalValue(name)
		}

		return nil
	}
}

func WithPredeclaredTypes(predeclaredTypes map[string]TypeDeclaration) Option {
	return func(checker *Checker) error {
		checker.PredeclaredTypes = predeclaredTypes

		for name, declaration := range predeclaredTypes {
			checker.declareTypeDeclaration(name, declaration)
		}

		return nil
	}
}

func WithAccessCheckMode(mode AccessCheckMode) Option {
	return func(checker *Checker) error {
		checker.AccessCheckMode = mode
		return nil
	}
}

func NewChecker(program *ast.Program, location ast.Location, options ...Option) (*Checker, error) {

	functionActivations := &FunctionActivations{}
	functionActivations.EnterFunction(&FunctionType{
		ReturnTypeAnnotation: NewTypeAnnotation(&VoidType{})},
		0,
	)

	typeActivations := NewValueActivations()
	for name, baseType := range baseTypes {
		_, err := typeActivations.DeclareType(
			ast.Identifier{Identifier: name},
			baseType,
			common.DeclarationKindType,
			ast.AccessPublic,
		)
		if err != nil {
			panic(err)
		}
	}

	checker := &Checker{
		Program:             program,
		Location:            location,
		ImportCheckers:      map[ast.LocationID]*Checker{},
		valueActivations:    NewValueActivations(),
		resources:           &Resources{},
		typeActivations:     typeActivations,
		functionActivations: functionActivations,
		GlobalValues:        map[string]*Variable{},
		GlobalTypes:         map[string]*Variable{},
		Occurrences:         NewOccurrences(),
		containerTypes:      map[Type]bool{},
		variableOrigins:     map[*Variable]*Origin{},
		memberOrigins:       map[Type]map[string]*Origin{},
		seenImports:         map[ast.LocationID]bool{},
		Elaboration:         NewElaboration(),
	}

	checker.declareBaseValues()

	for _, option := range options {
		err := option(checker)
		if err != nil {
			return nil, err
		}
	}

	err := checker.CheckerError()
	if err != nil {
		return nil, err
	}

	return checker, nil
}

func (checker *Checker) declareBaseValues() {
	for name, declaration := range BaseValues {
		checker.declareValue(name, declaration)
		checker.declareGlobalValue(name)
	}
}

func (checker *Checker) declareValue(name string, declaration ValueDeclaration) {
	variable, err := checker.valueActivations.Declare(
		name,
		declaration.ValueDeclarationType(),
		// TODO: add access to ValueDeclaration and use declaration's access instead here
		ast.AccessPublic,
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
	// TODO: add access to TypeDeclaration and use declaration's access instead here
	const access = ast.AccessPublic

	variable, err := checker.typeActivations.DeclareType(
		identifier,
		ty,
		declaration.TypeDeclarationKind(),
		access,
	)
	checker.report(err)
	checker.recordVariableDeclarationOccurrence(identifier.Identifier, variable)
}

func (checker *Checker) FindType(name string) Type {
	variable := checker.typeActivations.Find(name)
	if variable == nil {
		return nil
	}
	return variable.Type
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
	err := checker.CheckerError()
	if err != nil {
		return err
	}
	return nil
}

func (checker *Checker) CheckerError() *CheckerError {
	if len(checker.errors) > 0 {
		return &CheckerError{
			Errors: checker.errors,
		}
	}
	return nil
}

func (checker *Checker) report(err error) {
	if err == nil {
		return
	}
	checker.errors = append(checker.errors, err)
}

//TODO Once we have a flag which allows us to distinguish builtin from user defined
// types we can remove this silly list
// See https://github.com/dapperlabs/flow-go/issues/1627
var blacklist = map[string]interface{}{
	"Int":     nil,
	"Int8":    nil,
	"Int16":   nil,
	"Int32":   nil,
	"Int64":   nil,
	"UInt8":   nil,
	"UInt16":  nil,
	"UInt32":  nil,
	"UInt64":  nil,
	"Address": nil,
}

func (checker *Checker) UserDefinedValues() map[string]*Variable {
	ret := map[string]*Variable{}
	for key, value := range checker.GlobalValues {
		if _, ok := blacklist[key]; ok == true {
			continue
		}
		if _, ok := checker.PredeclaredValues[key]; ok {
			continue
		}

		if _, ok := checker.PredeclaredTypes[key]; ok {
			continue
		}
		if typeValue, ok := checker.GlobalTypes[key]; ok {
			ret[key] = typeValue
			continue
		}
		ret[key] = value
	}
	return ret
}

func (checker *Checker) VisitProgram(program *ast.Program) ast.Repr {

	for _, declaration := range program.ImportDeclarations() {
		checker.declareImportDeclaration(declaration)
	}

	// pre-declare interfaces, composites, and functions (check afterwards)

	for _, declaration := range program.InterfaceDeclarations() {
		checker.declareInterfaceDeclaration(declaration)
	}

	for _, declaration := range program.CompositeDeclarations() {
		checker.declareCompositeDeclaration(declaration)
	}

	for _, declaration := range program.FunctionDeclarations() {
		checker.declareGlobalFunctionDeclaration(declaration)
	}

	for _, declaration := range program.EventDeclarations() {
		checker.declareEventDeclaration(declaration)
	}

	for _, declaration := range program.TransactionDeclarations() {
		checker.declareTransactionDeclaration(declaration)
	}

	// check all declarations

	for _, declaration := range program.Declarations {

		// Skip import declarations, they are already handled above
		if _, isImport := declaration.(*ast.ImportDeclaration); isImport {
			continue
		}

		declaration.Accept(checker)
		checker.declareGlobalDeclaration(declaration)
	}

	return nil
}

func (checker *Checker) declareGlobalFunctionDeclaration(declaration *ast.FunctionDeclaration) {
	functionType := checker.functionType(declaration.ParameterList, declaration.ReturnTypeAnnotation)
	checker.Elaboration.FunctionDeclarationFunctionTypes[declaration] = functionType
	checker.declareFunctionDeclaration(declaration, functionType)
}

func (checker *Checker) checkTransfer(transfer *ast.Transfer, valueType Type) {
	if valueType.IsResourceType() {
		if transfer.Operation != ast.TransferOperationMove {
			checker.report(
				&IncorrectTransferOperationError{
					ActualOperation:   transfer.Operation,
					ExpectedOperation: ast.TransferOperationMove,
					Range:             ast.NewRangeFromPositioned(transfer),
				},
			)
		}
	} else if !valueType.IsInvalidType() {
		if transfer.Operation == ast.TransferOperationMove {
			checker.report(
				&IncorrectTransferOperationError{
					ActualOperation:   transfer.Operation,
					ExpectedOperation: ast.TransferOperationCopy,
					Range:             ast.NewRangeFromPositioned(transfer),
				},
			)
		}
	}
}

func (checker *Checker) IsTypeCompatible(expression ast.Expression, valueType Type, targetType Type) bool {
	switch typedExpression := expression.(type) {
	case *ast.IntExpression:
		unwrappedTargetType := UnwrapOptionalType(targetType)

		// If the target type is `Never`, the checks below will be performed
		// (as `Never` is the subtype of all types), but the checks are not valid

		if IsSubType(unwrappedTargetType, &NeverType{}) {
			break
		}

		if IsSubType(unwrappedTargetType, &IntegerType{}) {
			checker.checkIntegerLiteral(typedExpression, unwrappedTargetType)

			return true

		} else if IsSubType(unwrappedTargetType, &AddressType{}) {
			checker.checkAddressLiteral(typedExpression)

			return true
		}

	case *ast.ArrayExpression:

		// Variable sized array literals are compatible with constant sized target types
		// if their element type matches and the element count matches

		if variableSizedValueType, isVariableSizedValue :=
			valueType.(*VariableSizedType); isVariableSizedValue {

			if constantSizedTargetType, isConstantSizedTarget :=
				targetType.(*ConstantSizedType); isConstantSizedTarget {

				valueElementType := variableSizedValueType.ElementType(false)
				targetElementType := constantSizedTargetType.ElementType(false)

				// TODO: report helpful error when counts mismatch

				literalCount := len(typedExpression.Values)

				if IsSubType(valueElementType, targetElementType) &&
					literalCount == int(constantSizedTargetType.Size) {

					return true
				}
			}
		}

	case *ast.StringExpression:
		unwrappedTargetType := UnwrapOptionalType(targetType)

		if IsSubType(unwrappedTargetType, &CharacterType{}) {
			checker.checkCharacterLiteral(typedExpression)

			return true
		}
	}

	return IsSubType(valueType, targetType)
}

// checkIntegerLiteral checks that the value of the integer literal
// fits into range of the target integer type
//
func (checker *Checker) checkIntegerLiteral(expression *ast.IntExpression, integerType Type) {
	ranged := integerType.(Ranged)
	rangeMin := ranged.Min()
	rangeMax := ranged.Max()

	if checker.checkRange(expression.Value, rangeMin, rangeMax) {
		return
	}

	checker.report(
		&InvalidIntegerLiteralRangeError{
			ExpectedType:     integerType,
			ExpectedRangeMin: rangeMin,
			ExpectedRangeMax: rangeMax,
			Range:            ast.NewRangeFromPositioned(expression),
		},
	)
}

// checkAddressLiteral checks that the value of the integer literal
// fits into the range of an address (160 bits / 20 bytes),
// and is hexadecimal
//
func (checker *Checker) checkAddressLiteral(expression *ast.IntExpression) {
	ranged := &AddressType{}
	rangeMin := ranged.Min()
	rangeMax := ranged.Max()

	if expression.Base != 16 {
		checker.report(
			&InvalidAddressLiteralError{
				Range: ast.NewRangeFromPositioned(expression),
			},
		)
	}

	if checker.checkRange(expression.Value, rangeMin, rangeMax) {
		return
	}

	checker.report(
		&InvalidAddressLiteralError{
			Range: ast.NewRangeFromPositioned(expression),
		},
	)
}

func (checker *Checker) checkRange(value, min, max *big.Int) bool {
	return (min == nil || value.Cmp(min) >= 0) &&
		(max == nil || value.Cmp(max) <= 0)
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
	// The check is only necessary for resources.
	// Bail out early if the value is not a resource

	if !valueType.IsResourceType() {
		return
	}

	// Check the moved expression is wrapped in a unary expression with the move operation (<-).
	// Report an error if not and bail out if it is missing or another unary operator is used

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
		ty := checker.ConvertType(t.Type)
		return &OptionalType{ty}

	case *ast.DictionaryType:
		keyType := checker.ConvertType(t.KeyType)
		valueType := checker.ConvertType(t.ValueType)

		if !IsValidDictionaryKeyType(keyType) {
			checker.report(
				&InvalidDictionaryKeyTypeError{
					Type:  keyType,
					Range: ast.NewRangeFromPositioned(t.KeyType),
				},
			)
		}

		return &DictionaryType{
			KeyType:   keyType,
			ValueType: valueType,
		}

	case *ast.ReferenceType:
		ty := checker.ConvertType(t.Type)
		return &ReferenceType{ty}
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
	parameterList *ast.ParameterList,
	returnTypeAnnotation *ast.TypeAnnotation,
) *FunctionType {
	convertedParameterTypeAnnotations :=
		checker.parameterTypeAnnotations(parameterList)

	convertedReturnTypeAnnotation :=
		checker.ConvertTypeAnnotation(returnTypeAnnotation)

	return &FunctionType{
		ParameterTypeAnnotations: convertedParameterTypeAnnotations,
		ReturnTypeAnnotation:     convertedReturnTypeAnnotation,
	}
}

func (checker *Checker) parameterTypeAnnotations(parameterList *ast.ParameterList) []*TypeAnnotation {

	parameterTypeAnnotations := make([]*TypeAnnotation, len(parameterList.Parameters))

	for i, parameter := range parameterList.Parameters {
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
			DeclarationKind: variable.DeclarationKind,
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

func (checker *Checker) leaveValueScope(checkResourceLoss bool) {
	if checkResourceLoss {
		checker.checkResourceLoss(checker.valueActivations.Depth())
	}
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
			variable.DeclarationKind != common.DeclarationKindSelf &&
			variable.DeclarationKind != common.DeclarationKindResult &&
			!checker.resources.Get(variable).DefinitivelyInvalidated {

			checker.report(
				&ResourceLossError{
					Range: ast.Range{
						StartPos: *variable.Pos,
						EndPos:   variable.Pos.Shifted(len(name) - 1),
					},
				},
			)

		}
	}
}

func (checker *Checker) recordResourceInvalidation(
	expression ast.Expression,
	valueType Type,
	kind ResourceInvalidationKind,
) {
	if !valueType.IsResourceType() {
		return
	}

	reportInvalidNestedMove := func() {
		checker.report(
			&InvalidNestedMoveError{
				StartPos: expression.StartPosition(),
				EndPos:   expression.EndPosition(),
			},
		)
	}

	// TODO: improve handling of `self`: only allow invalidation once

	accessedSelfMember := checker.accessedSelfMember(expression)

	switch expression.(type) {
	case *ast.MemberExpression:
		if accessedSelfMember == nil {
			reportInvalidNestedMove()
			return
		}

	case *ast.IndexExpression:
		reportInvalidNestedMove()
		return
	}

	invalidation := ResourceInvalidation{
		Kind:     kind,
		StartPos: expression.StartPosition(),
		EndPos:   expression.EndPosition(),
	}

	if accessedSelfMember != nil {
		checker.resources.AddInvalidation(accessedSelfMember, invalidation)
		return
	}

	switch typedExpression := expression.(type) {
	case *ast.IdentifierExpression:

		variable := checker.findAndCheckVariable(typedExpression.Identifier, false)
		if variable == nil {
			return
		}

		checker.resources.AddInvalidation(variable, invalidation)

	case *ast.CreateExpression:
	case *ast.InvocationExpression:
	case *ast.ArrayExpression:
	case *ast.DictionaryExpression:
	case *ast.NilExpression:
	case *ast.CastingExpression:
	case *ast.BinaryExpression:
		// (nil-coalescing)
	default:
		panic(errors.NewUnreachableError())
	}
}

func (checker *Checker) checkWithResources(
	check TypeCheckFunc,
	temporaryResources *Resources,
) Type {
	originalResources := checker.resources
	checker.resources = temporaryResources
	defer func() {
		checker.resources = originalResources
	}()

	return check()
}

func (checker *Checker) checkWithReturnInfo(
	check TypeCheckFunc,
	temporaryReturnInfo *ReturnInfo,
) Type {
	functionActivation := checker.functionActivations.Current()
	initialReturnInfo := functionActivation.ReturnInfo
	functionActivation.ReturnInfo = temporaryReturnInfo
	defer func() {
		functionActivation.ReturnInfo = initialReturnInfo
	}()

	return check()
}

func (checker *Checker) checkWithInitializedMembers(
	check TypeCheckFunc,
	temporaryInitializedMembers *MemberSet,
) Type {
	if temporaryInitializedMembers != nil {
		functionActivation := checker.functionActivations.Current()
		initializationInfo := functionActivation.InitializationInfo
		initialInitializedMembers := initializationInfo.InitializedFieldMembers
		initializationInfo.InitializedFieldMembers = temporaryInitializedMembers
		defer func() {
			initializationInfo.InitializedFieldMembers = initialInitializedMembers
		}()
	}

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

	// Get the base expression of the given expression, i.e. get the accessed expression
	// as long as there is one.
	//
	// For example, in the expression `foo[0].bar`, both the wrapping member access
	// expression `bar` and the wrapping indexing expression `[0]` are removed,
	// leaving the base expression `foo`

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
			Range: ast.NewRangeFromPositioned(expression),
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

// checkPotentiallyUnevaluated runs the given type checking function
// under the assumption that the checked expression might not be evaluated.
// That means that resource invalidation and returns are not definite,
// but only potential
//
func (checker *Checker) checkPotentiallyUnevaluated(check TypeCheckFunc) Type {
	functionActivation := checker.functionActivations.Current()

	initialReturnInfo := functionActivation.ReturnInfo
	temporaryReturnInfo := initialReturnInfo.Clone()

	var temporaryInitializedMembers *MemberSet
	if functionActivation.InitializationInfo != nil {
		initialInitializedMembers := functionActivation.InitializationInfo.InitializedFieldMembers
		temporaryInitializedMembers = initialInitializedMembers.Clone()
	}

	initialResources := checker.resources
	temporaryResources := initialResources.Clone()

	result := checker.checkBranch(
		check,
		temporaryReturnInfo,
		temporaryInitializedMembers,
		temporaryResources,
	)

	functionActivation.ReturnInfo.MaybeReturned =
		functionActivation.ReturnInfo.MaybeReturned ||
			temporaryReturnInfo.MaybeReturned

	// NOTE: the definitive return state does not change

	checker.resources.MergeBranches(temporaryResources, nil)

	return result
}

func (checker *Checker) ResetErrors() {
	checker.errors = nil
}

func (checker *Checker) checkDeclarationAccessModifier(
	access ast.Access,
	declarationKind common.DeclarationKind,
	startPos ast.Position,
	isConstant bool,
	allowAuth bool,
) {
	if checker.functionActivations.IsLocal() {

		if access != ast.AccessNotSpecified {
			checker.report(
				&InvalidAccessModifierError{
					Access:          access,
					DeclarationKind: declarationKind,
					Pos:             startPos,
				},
			)
		}
	} else {

		switch access {
		case ast.AccessPublicSettable:
			// Public settable access for a constant is not sensible

			if isConstant {
				checker.report(
					&InvalidAccessModifierError{
						Access:          access,
						DeclarationKind: declarationKind,
						Pos:             startPos,
					},
				)
			}

		case ast.AccessNotSpecified:
			// In strict mode, access modifiers must be given

			if checker.AccessCheckMode == AccessCheckModeStrict {
				checker.report(
					&MissingAccessModifierError{
						DeclarationKind: declarationKind,
						Pos:             startPos,
					},
				)
			}

		case ast.AccessAuthorized:
			if !allowAuth {
				checker.report(
					&InvalidAccessModifierError{
						Access:          access,
						DeclarationKind: declarationKind,
						Pos:             startPos,
					},
				)
			}
		}
	}
}

func (checker *Checker) checkFieldsAccessModifier(fields []*ast.FieldDeclaration) {
	for _, field := range fields {
		isConstant := field.VariableKind == ast.VariableKindConstant

		checker.checkDeclarationAccessModifier(
			field.Access,
			field.DeclarationKind(),
			field.StartPos,
			isConstant,
			true,
		)
	}
}

// checkCharacterLiteral checks that the string literal is a valid character,
// i.e. it has exactly one grapheme cluster.
//
func (checker *Checker) checkCharacterLiteral(expression *ast.StringExpression) {
	length := uniseg.GraphemeClusterCount(expression.Value)

	if length == 1 {
		return
	}

	checker.report(
		&InvalidCharacterLiteralError{
			Length: length,
			Range:  ast.NewRangeFromPositioned(expression),
		},
	)
}

func (checker *Checker) isReadableAccess(access ast.Access) bool {
	switch checker.AccessCheckMode {
	case AccessCheckModeStrict,
		AccessCheckModeNotSpecifiedRestricted:

		return access == ast.AccessAuthorized ||
			access == ast.AccessPublic ||
			access == ast.AccessPublicSettable

	case AccessCheckModeNotSpecifiedUnrestricted:

		return access == ast.AccessNotSpecified ||
			access == ast.AccessAuthorized ||
			access == ast.AccessPublic ||
			access == ast.AccessPublicSettable

	case AccessCheckModeNone:
		return true

	default:
		panic(errors.NewUnreachableError())
	}
}

func (checker *Checker) isWriteableAccess(access ast.Access) bool {
	switch checker.AccessCheckMode {
	case AccessCheckModeStrict,
		AccessCheckModeNotSpecifiedRestricted:

		return access == ast.AccessAuthorized ||
			access == ast.AccessPublicSettable

	case AccessCheckModeNotSpecifiedUnrestricted:

		return access == ast.AccessNotSpecified ||
			access == ast.AccessAuthorized ||
			access == ast.AccessPublicSettable

	case AccessCheckModeNone:
		return true

	default:
		panic(errors.NewUnreachableError())
	}
}
