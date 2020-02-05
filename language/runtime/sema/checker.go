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

var beforeType = &FunctionType{
	Parameters: []*Parameter{
		{
			Label:          ArgumentLabelNotRequired,
			Identifier:     "value",
			TypeAnnotation: NewTypeAnnotation(&AnyStructType{}),
		},
	},
	ReturnTypeAnnotation: NewTypeAnnotation(
		&AnyStructType{},
	),
	ReturnTypeGetter: func(argumentTypes []Type) Type {
		return argumentTypes[0]
	},
}

// Checker

type Checker struct {
	Program                            *ast.Program
	Location                           ast.Location
	PredeclaredValues                  map[string]ValueDeclaration
	PredeclaredTypes                   map[string]TypeDeclaration
	allCheckers                        map[ast.LocationID]*Checker
	accessCheckMode                    AccessCheckMode
	errors                             []error
	valueActivations                   *VariableActivations
	resources                          *Resources
	typeActivations                    *VariableActivations
	containerTypes                     map[Type]bool
	functionActivations                *FunctionActivations
	GlobalValues                       map[string]*Variable
	GlobalTypes                        map[string]*Variable
	TransactionTypes                   []*TransactionType
	inCondition                        bool
	Occurrences                        *Occurrences
	variableOrigins                    map[*Variable]*Origin
	memberOrigins                      map[Type]map[string]*Origin
	seenImports                        map[ast.LocationID]bool
	isChecked                          bool
	inCreate                           bool
	inInvocation                       bool
	inAssignment                       bool
	allowSelfResourceFieldInvalidation bool
	Elaboration                        *Elaboration
	currentMemberExpression            *ast.MemberExpression
	validTopLevelDeclarationsHandler   func(ast.Location) []common.DeclarationKind
	beforeExtractor                    *BeforeExtractor
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

// WithAccessCheckMode returns a checker option which sets
// the given mode for access control checks.
//
func WithAccessCheckMode(mode AccessCheckMode) Option {
	return func(checker *Checker) error {
		checker.accessCheckMode = mode
		return nil
	}
}

// WithValidTopLevelDeclarationsHandler returns a checker option which sets
// the given handler as function which is used to determine
// the slice of declaration kinds which are valid at the top-level
// for a given location.
//
func WithValidTopLevelDeclarationsHandler(handler func(location ast.Location) []common.DeclarationKind) Option {
	return func(checker *Checker) error {
		checker.validTopLevelDeclarationsHandler = handler
		return nil
	}
}

// WithAllCheckers returns a checker option which sets
// the given map of checkers as the map of all checkers.
//
func WithAllCheckers(allCheckers map[ast.LocationID]*Checker) Option {
	return func(checker *Checker) error {
		checker.SetAllCheckers(allCheckers)
		return nil
	}
}

func NewChecker(program *ast.Program, location ast.Location, options ...Option) (*Checker, error) {

	if location == nil {
		return nil, &MissingLocationError{}
	}

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

	checker.beforeExtractor = NewBeforeExtractor(checker.report)

	checker.declareBaseValues()

	defaultOptions := []Option{
		WithAllCheckers(map[ast.LocationID]*Checker{}),
	}

	for _, option := range append(defaultOptions, options...) {
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

// SetAllCheckers sets the given map of checkers as the map of all checkers.
//
func (checker *Checker) SetAllCheckers(allCheckers map[ast.LocationID]*Checker) {
	checker.allCheckers = allCheckers

	// Register self
	checker.allCheckers[checker.Location.ID()] = checker
}

func (checker *Checker) declareBaseValues() {
	for name, declaration := range BaseValues {
		variable := checker.declareValue(name, declaration)
		variable.IsBaseValue = true
		checker.declareGlobalValue(name)
	}
}

func (checker *Checker) declareValue(name string, declaration ValueDeclaration) *Variable {
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
	return variable
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

func (checker *Checker) UserDefinedValues() map[string]*Variable {
	variables := map[string]*Variable{}

	for key, value := range checker.GlobalValues {
		if value.IsBaseValue {
			continue
		}

		if _, ok := checker.PredeclaredValues[key]; ok {
			continue
		}

		if _, ok := checker.PredeclaredTypes[key]; ok {
			continue
		}

		if typeValue, ok := checker.GlobalTypes[key]; ok {
			variables[key] = typeValue
			continue
		}

		variables[key] = value
	}

	return variables
}

func (checker *Checker) VisitProgram(program *ast.Program) ast.Repr {

	for _, declaration := range program.ImportDeclarations() {
		checker.declareImportDeclaration(declaration)
	}

	// Declare interface and composite types

	for _, declaration := range program.InterfaceDeclarations() {
		checker.declareInterfaceType(declaration)
	}

	for _, declaration := range program.CompositeDeclarations() {
		checker.declareCompositeType(declaration)
	}

	// Declare interfaces' and composites' members

	for _, declaration := range program.InterfaceDeclarations() {
		checker.declareInterfaceMembers(declaration)
	}

	for _, declaration := range program.CompositeDeclarations() {
		checker.declareCompositeMembersAndValue(declaration, ContainerKindComposite)
	}

	// Declare events, functions, and transactions

	for _, declaration := range program.FunctionDeclarations() {
		checker.declareGlobalFunctionDeclaration(declaration)
	}

	for _, declaration := range program.TransactionDeclarations() {
		checker.declareTransactionDeclaration(declaration)
	}

	// Check all declarations

	checker.checkTopLevelDeclarationValidity(program.Declarations)

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

func (checker *Checker) checkTopLevelDeclarationValidity(declarations []ast.Declaration) {
	if checker.validTopLevelDeclarationsHandler == nil {
		return
	}

	validDeclarationKinds := map[common.DeclarationKind]bool{}

	validTopLevelDeclarations := checker.validTopLevelDeclarationsHandler(checker.Location)
	if validTopLevelDeclarations == nil {
		return
	}

	for _, declarationKind := range validTopLevelDeclarations {
		validDeclarationKinds[declarationKind] = true
	}

	for _, declaration := range declarations {
		isValid := validDeclarationKinds[declaration.DeclarationKind()]
		if isValid {
			continue
		}

		var errorRange ast.Range

		identifier := declaration.DeclarationIdentifier()
		if identifier == nil {
			position := declaration.StartPosition()
			errorRange = ast.Range{
				StartPos: position,
				EndPos:   position,
			}
		} else {
			errorRange = ast.NewRangeFromPositioned(identifier)
		}

		checker.report(
			&InvalidTopLevelDeclarationError{
				DeclarationKind: declaration.DeclarationKind(),
				Range:           errorRange,
			},
		)
	}
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

func (checker *Checker) checkTypeCompatibility(expression ast.Expression, valueType Type, targetType Type) bool {
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

				literalCount := len(typedExpression.Values)

				if IsSubType(valueElementType, targetElementType) {

					if literalCount == constantSizedTargetType.Size {
						return true
					}
					checker.report(
						&ConstantSizedArrayLiteralSizeError{
							ExpectedSize: constantSizedTargetType.Size,
							ActualSize:   literalCount,
							Range:        typedExpression.Range,
						},
					)
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
	identifier := declaration.DeclarationIdentifier()
	if identifier == nil {
		return
	}
	name := identifier.Identifier
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
		return checker.convertNominalType(t)

	case *ast.VariableSizedType:
		return checker.convertVariableSizedType(t)

	case *ast.ConstantSizedType:
		return checker.convertConstantSizedType(t)

	case *ast.FunctionType:
		return checker.convertFunctionType(t)

	case *ast.OptionalType:
		return checker.convertOptionalType(t)

	case *ast.DictionaryType:
		return checker.convertDictionaryType(t)

	case *ast.ReferenceType:
		return checker.convertReferenceType(t)

	case *ast.RestrictedType:
		return checker.convertRestrictedType(t)
	}

	panic(&astTypeConversionError{invalidASTType: t})
}

func (checker *Checker) convertRestrictedType(t *ast.RestrictedType) Type {
	typeResult := checker.ConvertType(t.Type)

	// The restricted type must be a concrete resource type

	resourceType, ok := typeResult.(*CompositeType)
	if !ok || resourceType.Kind != common.CompositeKindResource {
		checker.report(
			&InvalidRestrictedTypeError{
				Type:  resourceType,
				Range: ast.NewRangeFromPositioned(t.Type),
			},
		)
	}

	// Convert the restrictions

	var restrictions []*InterfaceType
	restrictionRanges := make(map[*InterfaceType]ast.Range, len(t.Restrictions))

	memberSet := map[string]*InterfaceType{}

	for _, restriction := range t.Restrictions {
		restrictionResult := checker.ConvertType(restriction)

		// The restriction must be a resource interface type

		interfaceType, ok := restrictionResult.(*InterfaceType)
		if !ok || interfaceType.CompositeKind != common.CompositeKindResource {
			checker.report(
				&InvalidRestrictionTypeError{
					Type:  interfaceType,
					Range: ast.NewRangeFromPositioned(restriction),
				},
			)
			continue
		}

		restrictions = append(restrictions, interfaceType)

		// The restriction must not be duplicated

		if _, exists := restrictionRanges[interfaceType]; exists {
			checker.report(
				&InvalidRestrictionTypeDuplicateError{
					Type:  interfaceType,
					Range: ast.NewRangeFromPositioned(restriction),
				},
			)
		} else {
			restrictionRanges[interfaceType] =
				ast.NewRangeFromPositioned(restriction)
		}

		// The restrictions may not have clashing members

		// TODO: also include interface conformances's members
		//   once interfaces can have conformances

		for name := range interfaceType.Members {
			if previousDeclaringInterfaceType, ok := memberSet[name]; ok {
				checker.report(
					&RestrictionMemberClashError{
						Name:                  name,
						RedeclaringType:       interfaceType,
						OriginalDeclaringType: previousDeclaringInterfaceType,
						Range:                 ast.NewRangeFromPositioned(restriction),
					},
				)
			} else {
				memberSet[name] = interfaceType
			}
		}
	}

	// If the restricted type is a concrete resource type,
	// check that the restrictions are conformances

	if resourceType != nil && resourceType.Kind == common.CompositeKindResource {

		// Prepare a set of all the conformances of the resource

		allConformances := resourceType.AllConformances()
		conformancesSet := make(map[*InterfaceType]bool, len(allConformances))
		for _, conformance := range allConformances {
			conformancesSet[conformance] = true
		}

		for _, restriction := range restrictions {
			// The restriction must be an explicit or implicit conformance
			// of the resource (restricted type)

			if !conformancesSet[restriction] {
				checker.report(
					&InvalidNonConformanceRestrictionError{
						Type:  restriction,
						Range: restrictionRanges[restriction],
					},
				)
			}
		}
	}

	return &RestrictedResourceType{
		Type:         resourceType,
		Restrictions: restrictions,
	}
}

func (checker *Checker) convertReferenceType(t *ast.ReferenceType) Type {
	ty := checker.ConvertType(t.Type)
	return &ReferenceType{
		Type: ty,
	}
}

func (checker *Checker) convertDictionaryType(t *ast.DictionaryType) Type {
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
}

func (checker *Checker) convertOptionalType(t *ast.OptionalType) Type {
	ty := checker.ConvertType(t.Type)
	return &OptionalType{
		Type: ty,
	}
}

func (checker *Checker) convertFunctionType(t *ast.FunctionType) Type {
	var parameters []*Parameter
	for _, parameterTypeAnnotation := range t.ParameterTypeAnnotations {
		parameterTypeAnnotation := checker.ConvertTypeAnnotation(parameterTypeAnnotation)
		parameters = append(parameters,
			&Parameter{
				TypeAnnotation: parameterTypeAnnotation,
			},
		)
	}

	returnTypeAnnotation := checker.ConvertTypeAnnotation(t.ReturnTypeAnnotation)

	return &FunctionType{
		Parameters:           parameters,
		ReturnTypeAnnotation: returnTypeAnnotation,
	}
}

func (checker *Checker) convertConstantSizedType(t *ast.ConstantSizedType) Type {
	elementType := checker.ConvertType(t.Type)
	return &ConstantSizedType{
		Type: elementType,
		Size: t.Size,
	}
}

func (checker *Checker) convertVariableSizedType(t *ast.VariableSizedType) Type {
	elementType := checker.ConvertType(t.Type)
	return &VariableSizedType{
		Type: elementType,
	}
}

func (checker *Checker) convertNominalType(t *ast.NominalType) Type {
	identifier := t.Identifier.Identifier
	result := checker.FindType(identifier)
	if result == nil {
		checker.report(
			&NotDeclaredError{
				ExpectedKind: common.DeclarationKindType,
				Name:         identifier,
				Pos:          t.StartPosition(),
			},
		)
		return &InvalidType{}
	}

	var resolvedIdentifiers []ast.Identifier

	for _, identifier := range t.NestedIdentifiers {
		switch typedResult := result.(type) {
		case *CompositeType:
			result = typedResult.NestedTypes[identifier.Identifier]

		case *InterfaceType:
			result = typedResult.NestedTypes[identifier.Identifier]

		default:
			if !typedResult.IsInvalidType() {
				checker.report(
					&InvalidNestedTypeError{
						Type: &ast.NominalType{
							Identifier:        t.Identifier,
							NestedIdentifiers: resolvedIdentifiers,
						},
					},
				)
			}

			return &InvalidType{}
		}

		resolvedIdentifiers = append(resolvedIdentifiers, identifier)

		if result == nil {
			nonExistentType := &ast.NominalType{
				Identifier:        t.Identifier,
				NestedIdentifiers: resolvedIdentifiers,
			}
			checker.report(
				&NotDeclaredError{
					ExpectedKind: common.DeclarationKindType,
					Name:         nonExistentType.String(),
					Pos:          t.StartPosition(),
				},
			)
			return &InvalidType{}
		}
	}

	return result
}

// ConvertTypeAnnotation converts an AST type annotation representation
// to a sema type annotation
//
func (checker *Checker) ConvertTypeAnnotation(typeAnnotation *ast.TypeAnnotation) *TypeAnnotation {
	convertedType := checker.ConvertType(typeAnnotation.Type)
	return &TypeAnnotation{
		IsResource: typeAnnotation.IsResource,
		Type:       convertedType,
	}
}

func (checker *Checker) functionType(
	parameterList *ast.ParameterList,
	returnTypeAnnotation *ast.TypeAnnotation,
) *FunctionType {
	convertedParameters := checker.parameters(parameterList)
	convertedReturnTypeAnnotation :=
		checker.ConvertTypeAnnotation(returnTypeAnnotation)

	return &FunctionType{
		Parameters:           convertedParameters,
		ReturnTypeAnnotation: convertedReturnTypeAnnotation,
	}
}

func (checker *Checker) parameters(parameterList *ast.ParameterList) []*Parameter {

	parameters := make([]*Parameter, len(parameterList.Parameters))

	for i, parameter := range parameterList.Parameters {
		convertedParameterType := checker.ConvertType(parameter.TypeAnnotation.Type)

		// NOTE: copying resource annotation from source type annotation as-is,
		// so a potential error is properly reported

		parameters[i] = &Parameter{
			Label:      parameter.Label,
			Identifier: parameter.Identifier.Identifier,
			TypeAnnotation: &TypeAnnotation{
				IsResource: parameter.TypeAnnotation.IsResource,
				Type:       convertedParameterType,
			},
		}
	}

	return parameters
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
	identifier ast.Identifier,
	startPos, endPos ast.Position,
	fieldType Type,
) *Origin {
	startPosition := identifier.StartPosition()
	endPosition := identifier.EndPosition()

	origin := &Origin{
		Type:            fieldType,
		DeclarationKind: common.DeclarationKindField,
		StartPos:        &startPosition,
		EndPos:          &endPosition,
	}

	checker.Occurrences.Put(
		startPos,
		endPos,
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
	invalidationKind ResourceInvalidationKind,
) {
	if !valueType.IsResourceType() {
		return
	}

	reportInvalidNestedMove := func() {
		checker.report(
			&InvalidNestedResourceMoveError{
				StartPos: expression.StartPosition(),
				EndPos:   expression.EndPosition(),
			},
		)
	}

	accessedSelfMember := checker.accessedSelfMember(expression)

	switch expression.(type) {
	case *ast.MemberExpression:
		if accessedSelfMember == nil || !checker.allowSelfResourceFieldInvalidation {
			reportInvalidNestedMove()
			return
		}

	case *ast.IndexExpression:
		reportInvalidNestedMove()
		return
	}

	invalidation := ResourceInvalidation{
		Kind:     invalidationKind,
		StartPos: expression.StartPosition(),
		EndPos:   expression.EndPosition(),
	}

	if checker.allowSelfResourceFieldInvalidation && accessedSelfMember != nil {
		checker.resources.AddInvalidation(accessedSelfMember, invalidation)
		return
	}

	identifierExpression, ok := expression.(*ast.IdentifierExpression)
	if !ok {
		return
	}

	variable := checker.findAndCheckVariable(identifierExpression.Identifier, false)
	if variable == nil {
		return
	}

	if variable.DeclarationKind == common.DeclarationKindSelf {
		checker.report(
			&InvalidSelfInvalidationError{
				InvalidationKind: invalidationKind,
				StartPos:         expression.StartPosition(),
				EndPos:           expression.EndPosition(),
			},
		)
	}

	checker.resources.AddInvalidation(variable, invalidation)
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
		// NOTE: check type, not resource annotation:
		// the field could have a wrong annotation
		if !member.TypeAnnotation.Type.IsResourceType() {
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

		isTypeDeclaration := declarationKind.IsTypeDeclaration()

		switch access {
		case ast.AccessPublicSettable:
			// Public settable access for a constant is not sensible
			// and type declarations must be public for now

			if isConstant || isTypeDeclaration {
				checker.report(
					&InvalidAccessModifierError{
						Access:          access,
						DeclarationKind: declarationKind,
						Pos:             startPos,
					},
				)
			}

		case ast.AccessPrivate:
			// Type declarations must be public for now

			if isTypeDeclaration {

				checker.report(
					&InvalidAccessModifierError{
						Access:          access,
						DeclarationKind: declarationKind,
						Pos:             startPos,
					},
				)
			}

		case ast.AccessContract,
			ast.AccessAccount:

			// Type declarations must be public for now

			if isTypeDeclaration {
				checker.report(
					&InvalidAccessModifierError{
						Access:          access,
						DeclarationKind: declarationKind,
						Pos:             startPos,
					},
				)
			}

		case ast.AccessNotSpecified:

			// Type declarations cannot be effectively private for now

			if isTypeDeclaration &&
				checker.accessCheckMode == AccessCheckModeNotSpecifiedRestricted {

				checker.report(
					&MissingAccessModifierError{
						DeclarationKind: declarationKind,
						Pos:             startPos,
					},
				)
			}

			// In strict mode, access modifiers must be given

			if checker.accessCheckMode == AccessCheckModeStrict {
				checker.report(
					&MissingAccessModifierError{
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
	switch checker.accessCheckMode {
	case AccessCheckModeStrict,
		AccessCheckModeNotSpecifiedRestricted:

		return access == ast.AccessPublic ||
			access == ast.AccessPublicSettable

	case AccessCheckModeNotSpecifiedUnrestricted:

		return access == ast.AccessNotSpecified ||
			access == ast.AccessPublic ||
			access == ast.AccessPublicSettable

	case AccessCheckModeNone:
		return true

	default:
		panic(errors.NewUnreachableError())
	}
}

func (checker *Checker) isWriteableAccess(access ast.Access) bool {
	switch checker.accessCheckMode {
	case AccessCheckModeStrict,
		AccessCheckModeNotSpecifiedRestricted:

		return access == ast.AccessPublicSettable

	case AccessCheckModeNotSpecifiedUnrestricted:

		return access == ast.AccessNotSpecified ||
			access == ast.AccessPublicSettable

	case AccessCheckModeNone:
		return true

	default:
		panic(errors.NewUnreachableError())
	}
}

func (checker *Checker) withSelfResourceInvalidationAllowed(f func()) {
	allowSelfResourceFieldInvalidation := checker.allowSelfResourceFieldInvalidation
	checker.allowSelfResourceFieldInvalidation = true
	defer func() {
		checker.allowSelfResourceFieldInvalidation = allowSelfResourceFieldInvalidation
	}()

	f()
}

func (checker *Checker) predeclaredMembers(containerType Type) []*Member {
	var predeclaredMembers []*Member

	// Contracts have a predeclared `account: Account` field

	if compositeType, ok := containerType.(*CompositeType); ok &&
		compositeType.Kind == common.CompositeKindContract {

		predeclaredMembers = append(predeclaredMembers,
			&Member{
				Predeclared:     true,
				ContainerType:   compositeType,
				Access:          ast.AccessPrivate,
				Identifier:      ast.Identifier{Identifier: "account"},
				TypeAnnotation:  &TypeAnnotation{Type: &AccountType{}},
				DeclarationKind: common.DeclarationKindField,
				VariableKind:    ast.VariableKindConstant,
			},
		)
	}

	return predeclaredMembers
}

func (checker *Checker) checkVariableMove(expression ast.Expression) {

	identifierExpression, ok := expression.(*ast.IdentifierExpression)
	if !ok {
		return
	}

	variable := checker.valueActivations.Find(identifierExpression.Identifier.Identifier)
	if variable == nil {
		return
	}

	reportInvalidMove := func(declarationKind common.DeclarationKind) {
		checker.report(
			&InvalidMoveError{
				Name:            variable.Identifier,
				DeclarationKind: declarationKind,
				Pos:             identifierExpression.StartPosition(),
			},
		)
	}

	switch ty := variable.Type.(type) {
	case *TransactionType:
		reportInvalidMove(common.DeclarationKindTransaction)

	case CompositeKindedType:
		kind := ty.GetCompositeKind()
		if kind == common.CompositeKindContract {
			reportInvalidMove(common.DeclarationKindContract)
		}
	}
}

func (checker *Checker) rewritePostConditions(postConditions []*ast.Condition) PostConditionsRewrite {

	var beforeStatements []ast.Statement
	rewrittenPostConditions := make([]*ast.Condition, len(postConditions))

	for i, postCondition := range postConditions {

		// copy condition and set expression to rewritten one
		newPostCondition := *postCondition

		testExtraction := checker.beforeExtractor.ExtractBefore(postCondition.Test)

		extractedExpressions := testExtraction.ExtractedExpressions

		newPostCondition.Test = testExtraction.RewrittenExpression

		if postCondition.Message != nil {
			messageExtraction := checker.beforeExtractor.ExtractBefore(postCondition.Message)

			newPostCondition.Message = messageExtraction.RewrittenExpression

			extractedExpressions = append(
				extractedExpressions,
				messageExtraction.ExtractedExpressions...,
			)
		}

		for _, extractedExpression := range extractedExpressions {

			// NOTE: no need to check the before statements or update elaboration here:
			// The before statements are visited/checked later

			variableDeclaration := &ast.VariableDeclaration{
				Identifier: extractedExpression.Identifier,
				Transfer: &ast.Transfer{
					Operation: ast.TransferOperationCopy,
				},
				Value: extractedExpression.Expression,
			}

			beforeStatements = append(beforeStatements,
				variableDeclaration,
			)
		}

		rewrittenPostConditions[i] = &newPostCondition
	}

	return PostConditionsRewrite{
		BeforeStatements:        beforeStatements,
		RewrittenPostConditions: rewrittenPostConditions,
	}
}
