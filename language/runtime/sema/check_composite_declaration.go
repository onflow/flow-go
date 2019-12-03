package sema

import (
	"github.com/dapperlabs/flow-go/language/runtime/ast"
	"github.com/dapperlabs/flow-go/language/runtime/common"
	"github.com/dapperlabs/flow-go/language/runtime/errors"
)

func (checker *Checker) VisitCompositeDeclaration(declaration *ast.CompositeDeclaration) ast.Repr {

	compositeType := checker.Elaboration.CompositeDeclarationTypes[declaration]

	checker.containerTypes[compositeType] = true
	defer func() {
		checker.containerTypes[compositeType] = false
	}()

	checker.checkDeclarationAccessModifier(
		declaration.Access,
		declaration.DeclarationKind(),
		declaration.StartPos,
		true,
		false,
	)

	// NOTE: functions are checked separately
	checker.checkFieldsAccessModifier(declaration.Members.Fields)

	checker.checkMemberIdentifiers(
		declaration.Members.Fields,
		declaration.Members.Functions,
	)

	// Declare nested types

	checker.typeActivations.Enter()
	defer checker.typeActivations.Leave()

	checker.valueActivations.Enter()
	defer checker.valueActivations.Leave()

	nestedDeclarations := checker.Elaboration.CompositeNestedDeclarations[declaration]

	for name, nestedType := range compositeType.NestedTypes {
		nestedDeclaration := nestedDeclarations[name]

		_, err := checker.typeActivations.DeclareType(
			nestedDeclaration.DeclarationIdentifier(),
			nestedType,
			nestedDeclaration.DeclarationKind(),
			nestedDeclaration.DeclarationAccess(),
		)
		checker.report(err)

		// NOTE: Re-declare the constructor for the nested composite declaration:
		// The constructor was already declared before in `declareCompositeDeclaration`
		// for this nested declaration, but the value activation for it was only temporary,
		// so that the constructor wouldn't be visible outside of the containing declaration

		if nestedCompositeDeclaration, isCompositeDeclaration :=
			nestedDeclaration.(*ast.CompositeDeclaration); isCompositeDeclaration {

			nestedCompositeType := nestedType.(*CompositeType)
			checker.declareCompositeConstructor(nestedCompositeDeclaration, nestedCompositeType)
		}
	}

	// The initializer must initialize all members that are fields,
	// e.g. not composite functions (which are by definition constant and "initialized")

	fieldMembers := map[*Member]*ast.FieldDeclaration{}

	for _, field := range declaration.Members.Fields {
		fieldName := field.Identifier.Identifier
		member := compositeType.Members[fieldName]
		fieldMembers[member] = field
	}

	initializationInfo := NewInitializationInfo(compositeType, fieldMembers)

	checker.checkInitializers(
		declaration.Members.Initializers(),
		declaration.Members.Fields,
		compositeType,
		declaration.DeclarationKind(),
		declaration.Identifier.Identifier,
		compositeType.ConstructorParameterTypeAnnotations,
		ContainerKindComposite,
		initializationInfo,
	)

	checker.checkUnknownSpecialFunctions(declaration.Members.SpecialFunctions)

	checker.checkCompositeFunctions(declaration.Members.Functions, compositeType)

	checker.checkResourceFieldNesting(
		declaration.Members.FieldsByIdentifier(),
		compositeType.Members,
		compositeType.Kind,
	)

	// check composite conforms to interfaces.
	// NOTE: perform after completing composite type (e.g. setting constructor parameter types)

	for i, interfaceType := range compositeType.Conformances {
		conformance := declaration.Conformances[i]

		checker.checkCompositeConformance(
			compositeType,
			interfaceType,
			declaration.Identifier.Pos,
			conformance.Identifier,
		)
	}

	// NOTE: check destructors after initializer and functions

	checker.checkDestructors(
		declaration.Members.Destructors(),
		declaration.Members.FieldsByIdentifier(),
		compositeType.Members,
		compositeType,
		declaration.DeclarationKind(),
		declaration.Identifier.Identifier,
		ContainerKindComposite,
	)

	checker.checkCompositeDeclarationSupport(
		declaration.CompositeKind,
		declaration.DeclarationKind(),
		declaration.Identifier,
	)

	for _, nestedDeclaration := range nestedDeclarations {
		nestedDeclaration.Accept(checker)
	}

	return nil
}

func (checker *Checker) visitNestedDeclarations(
	containerCompositeKind common.CompositeKind,
	containerDeclarationKind common.DeclarationKind,
	nestedCompositeDeclarations []*ast.CompositeDeclaration,
	nestedInterfaceDeclarations []*ast.InterfaceDeclaration,
) (
	nestedDeclarations map[string]ast.Declaration,
	nestedInterfaceTypes []*InterfaceType,
	nestedCompositeTypes []*CompositeType,
) {
	nestedDeclarations = map[string]ast.Declaration{}

	// Only contracts and contract interfaces support nested composite declarations
	if containerCompositeKind != common.CompositeKindContract {

		reportInvalidNesting := func(nestedDeclarationKind common.DeclarationKind, identifier ast.Identifier) {
			checker.report(
				&InvalidNestedDeclarationError{
					NestedDeclarationKind:    nestedDeclarationKind,
					ContainerDeclarationKind: containerDeclarationKind,
					Range:                    ast.NewRangeFromPositioned(identifier),
				},
			)
		}

		if len(nestedCompositeDeclarations) > 0 {

			firstNestedCompositeDeclaration := nestedCompositeDeclarations[0]

			reportInvalidNesting(
				firstNestedCompositeDeclaration.DeclarationKind(),
				firstNestedCompositeDeclaration.Identifier,
			)

		} else if len(nestedInterfaceDeclarations) > 0 {

			firstNestedInterfaceDeclaration := nestedInterfaceDeclarations[0]

			reportInvalidNesting(
				firstNestedInterfaceDeclaration.DeclarationKind(),
				firstNestedInterfaceDeclaration.Identifier,
			)
		}

		return
	}

	// Check contract's nested composite declarations and interface declarations
	// are a resource (interface) or a struct (interface)

	checkNestedDeclaration := func(
		nestedCompositeKind common.CompositeKind,
		nestedDeclarationKind common.DeclarationKind,
		identifier ast.Identifier,
	) bool {

		if nestedCompositeKind != common.CompositeKindResource &&
			nestedCompositeKind != common.CompositeKindStructure {

			checker.report(
				&InvalidNestedDeclarationError{
					NestedDeclarationKind:    nestedDeclarationKind,
					ContainerDeclarationKind: containerDeclarationKind,
					Range:                    ast.NewRangeFromPositioned(identifier),
				},
			)

			return false
		}

		return true
	}

	for _, nestedDeclaration := range nestedInterfaceDeclarations {
		if !checkNestedDeclaration(
			nestedDeclaration.CompositeKind,
			nestedDeclaration.DeclarationKind(),
			nestedDeclaration.Identifier,
		) {
			continue
		}

		if _, exists := nestedDeclarations[nestedDeclaration.Identifier.Identifier]; !exists {
			nestedDeclarations[nestedDeclaration.Identifier.Identifier] = nestedDeclaration
		}

		// pre-declare interface
		nestedInterfaceType := checker.declareInterfaceDeclaration(nestedDeclaration)
		nestedInterfaceTypes = append(nestedInterfaceTypes, nestedInterfaceType)
	}

	for _, nestedDeclaration := range nestedCompositeDeclarations {
		if !checkNestedDeclaration(
			nestedDeclaration.CompositeKind,
			nestedDeclaration.DeclarationKind(),
			nestedDeclaration.Identifier,
		) {
			continue
		}

		if _, exists := nestedDeclarations[nestedDeclaration.Identifier.Identifier]; !exists {
			nestedDeclarations[nestedDeclaration.Identifier.Identifier] = nestedDeclaration
		}

		// pre-declare composite
		nestedCompositeType := checker.declareCompositeDeclaration(nestedDeclaration)
		nestedCompositeTypes = append(nestedCompositeTypes, nestedCompositeType)
	}

	return
}

func (checker *Checker) checkCompositeDeclarationSupport(
	compositeKind common.CompositeKind,
	declarationKind common.DeclarationKind,
	identifier ast.Identifier,
) {
	switch compositeKind {
	case common.CompositeKindStructure, common.CompositeKindResource:
		break
	default:
		if declarationKind != common.DeclarationKindContractInterface {
			return
		}

		// TODO: add support for contract interfaces

		checker.report(
			&UnsupportedDeclarationError{
				DeclarationKind: declarationKind,
				Range:           ast.NewRangeFromPositioned(identifier),
			},
		)
	}
}

func (checker *Checker) declareCompositeDeclaration(declaration *ast.CompositeDeclaration) *CompositeType {

	identifier := declaration.Identifier

	// NOTE: fields and functions might already refer to declaration itself.
	// insert a dummy type for now, so lookup succeeds during conversion,
	// then fix up the type reference

	// NOTE: it is important to already specify the kind, as fields may refer
	// to the type and check the annotation of the field

	compositeType := &CompositeType{
		Location:    checker.Location,
		Kind:        declaration.CompositeKind,
		Identifier:  identifier.Identifier,
		NestedTypes: map[string]Type{},
	}

	variable, err := checker.typeActivations.DeclareType(
		identifier,
		compositeType,
		declaration.DeclarationKind(),
		declaration.Access,
	)
	checker.report(err)
	checker.recordVariableDeclarationOccurrence(
		identifier.Identifier,
		variable,
	)

	(func() {
		// Activate new scopes for nested types

		checker.typeActivations.Enter()
		defer checker.typeActivations.Leave()

		checker.valueActivations.Enter()
		defer checker.valueActivations.Leave()

		// Check and declare nested types before checking members

		nestedDeclarations, nestedInterfaceTypes, nestedCompositeTypes :=
			checker.visitNestedDeclarations(
				declaration.CompositeKind,
				declaration.DeclarationKind(),
				declaration.CompositeDeclarations,
				declaration.InterfaceDeclarations,
			)

		checker.Elaboration.CompositeNestedDeclarations[declaration] = nestedDeclarations

		for _, nestedInterfaceType := range nestedInterfaceTypes {
			compositeType.NestedTypes[nestedInterfaceType.Identifier] = nestedInterfaceType
			nestedInterfaceType.ContainerType = compositeType
		}

		for _, nestedCompositeType := range nestedCompositeTypes {
			compositeType.NestedTypes[nestedCompositeType.Identifier] = nestedCompositeType
			nestedCompositeType.ContainerType = compositeType
		}

		// Check conformances and members

		conformances := checker.conformances(declaration)

		members, origins := checker.membersAndOrigins(
			compositeType,
			declaration.Members.Fields,
			declaration.Members.Functions,
			true,
		)

		compositeType.Members = members
		compositeType.Conformances = conformances

		checker.memberOrigins[compositeType] = origins

		// NOTE: determine initializer parameter types while nested types are in scope,
		// and after declaring nested types as the initializer may use nested type in parameters

		compositeType.ConstructorParameterTypeAnnotations =
			checker.initializerParameterTypeAnnotations(declaration.Members.Initializers())
	})()

	// Declare constructor after the nested scope, so it is visible after the declaration

	checker.declareCompositeConstructor(declaration, compositeType)

	checker.Elaboration.CompositeDeclarationTypes[declaration] = compositeType

	return compositeType
}

func (checker *Checker) initializerParameterTypeAnnotations(initializers []*ast.SpecialFunctionDeclaration) []*TypeAnnotation {
	// TODO: support multiple overloaded initializers
	var parameterTypeAnnotations []*TypeAnnotation

	initializerCount := len(initializers)
	if initializerCount > 0 {
		firstInitializer := initializers[0]
		parameterTypeAnnotations = checker.parameterTypeAnnotations(firstInitializer.ParameterList)

		if initializerCount > 1 {
			secondInitializer := initializers[1]

			checker.report(
				&UnsupportedOverloadingError{
					DeclarationKind: common.DeclarationKindInitializer,
					Range:           ast.NewRangeFromPositioned(secondInitializer),
				},
			)
		}
	}
	return parameterTypeAnnotations
}

func (checker *Checker) conformances(declaration *ast.CompositeDeclaration) []*InterfaceType {

	var interfaceTypes []*InterfaceType
	seenConformances := map[string]bool{}

	compositeIdentifier := declaration.Identifier.Identifier

	for _, conformance := range declaration.Conformances {
		convertedType := checker.ConvertType(conformance)

		if interfaceType, ok := convertedType.(*InterfaceType); ok {
			interfaceTypes = append(interfaceTypes, interfaceType)

		} else if !convertedType.IsInvalidType() {
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
					CompositeIdentifier: compositeIdentifier,
					Conformance:         conformance,
				},
			)

		}
		seenConformances[conformanceIdentifier] = true
	}
	return interfaceTypes
}

func (checker *Checker) checkCompositeConformance(
	compositeType *CompositeType,
	interfaceType *InterfaceType,
	compositeIdentifierPos ast.Position,
	interfaceIdentifier ast.Identifier,
) {
	var missingMembers []*Member
	var memberMismatches []MemberMismatch
	var initializerMismatch *InitializerMismatch

	// ensure the composite kinds match, e.g. a structure shouldn't be able
	// to conform to a resource interface

	if interfaceType.CompositeKind != compositeType.Kind {
		checker.report(
			&CompositeKindMismatchError{
				ExpectedKind: compositeType.Kind,
				ActualKind:   interfaceType.CompositeKind,
				Range:        ast.NewRangeFromPositioned(interfaceIdentifier),
			},
		)
	}

	// TODO: add support for overloaded initializers

	if interfaceType.InitializerParameterTypeAnnotations != nil {

		initializerType := &FunctionType{
			ParameterTypeAnnotations: compositeType.ConstructorParameterTypeAnnotations,
			ReturnTypeAnnotation:     NewTypeAnnotation(&VoidType{}),
		}
		interfaceInitializerType := &FunctionType{
			ParameterTypeAnnotations: interfaceType.InitializerParameterTypeAnnotations,
			ReturnTypeAnnotation:     NewTypeAnnotation(&VoidType{}),
		}

		// TODO: subtype?
		if !initializerType.Equal(interfaceInitializerType) {
			initializerMismatch = &InitializerMismatch{
				CompositeParameterTypes: compositeType.ConstructorParameterTypeAnnotations,
				InterfaceParameterTypes: interfaceType.InitializerParameterTypeAnnotations,
			}
		}
	}

	for name, interfaceMember := range interfaceType.Members {

		compositeMember, ok := compositeType.Members[name]
		if !ok {
			missingMembers = append(missingMembers, interfaceMember)
			continue
		}

		if !checker.memberSatisfied(compositeMember, interfaceMember) {
			memberMismatches = append(memberMismatches,
				MemberMismatch{
					CompositeMember: compositeMember,
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
				CompositeType:       compositeType,
				InterfaceType:       interfaceType,
				Pos:                 compositeIdentifierPos,
				InitializerMismatch: initializerMismatch,
				MissingMembers:      missingMembers,
				MemberMismatches:    memberMismatches,
			},
		)
	}
}

// TODO: return proper error
func (checker *Checker) memberSatisfied(compositeMember, interfaceMember *Member) bool {
	// Check type

	// TODO: subtype?
	if !compositeMember.Type.Equal(interfaceMember.Type) {
		return false
	}

	// Check variable kind

	if interfaceMember.VariableKind != ast.VariableKindNotSpecified &&
		compositeMember.VariableKind != interfaceMember.VariableKind {

		return false
	}

	// Check access

	if compositeMember.Access == ast.AccessPrivate {
		return false
	}

	if interfaceMember.DeclarationKind == common.DeclarationKindField &&
		interfaceMember.Access != ast.AccessNotSpecified &&
		compositeMember.Access.IsLessPermissiveThan(interfaceMember.Access) {

		return false
	}

	return true
}

func (checker *Checker) declareCompositeConstructor(
	compositeDeclaration *ast.CompositeDeclaration,
	compositeType *CompositeType,
) {
	functionType := &SpecialFunctionType{
		&FunctionType{
			ReturnTypeAnnotation: NewTypeAnnotation(
				compositeType,
			),
		},
	}

	var argumentLabels []string

	// TODO: support multiple overloaded initializers

	initializers := compositeDeclaration.Members.Initializers
	if len(initializers()) > 0 {
		firstInitializer := initializers()[0]

		argumentLabels = firstInitializer.ParameterList.ArgumentLabels()

		functionType = &SpecialFunctionType{
			FunctionType: &FunctionType{
				ParameterTypeAnnotations: compositeType.ConstructorParameterTypeAnnotations,
				ReturnTypeAnnotation:     NewTypeAnnotation(compositeType),
			},
		}

		checker.Elaboration.SpecialFunctionTypes[firstInitializer] = functionType
	}

	_, err := checker.valueActivations.DeclareFunction(
		compositeDeclaration.Identifier,
		compositeDeclaration.Access,
		functionType,
		argumentLabels,
	)
	checker.report(err)
}

func (checker *Checker) membersAndOrigins(
	containerType Type,
	fields []*ast.FieldDeclaration,
	functions []*ast.FunctionDeclaration,
	requireVariableKind bool,
) (
	members map[string]*Member,
	origins map[string]*Origin,
) {
	memberCount := len(fields) + len(functions)
	members = make(map[string]*Member, memberCount)
	origins = make(map[string]*Origin, memberCount)

	// declare a member for each field
	for _, field := range fields {
		fieldTypeAnnotation := checker.ConvertTypeAnnotation(field.TypeAnnotation)

		fieldType := fieldTypeAnnotation.Type

		checker.checkTypeAnnotation(fieldTypeAnnotation, field.TypeAnnotation.StartPos)

		identifier := field.Identifier.Identifier

		members[identifier] = &Member{
			ContainerType:   containerType,
			Access:          field.Access,
			Identifier:      field.Identifier,
			DeclarationKind: common.DeclarationKindField,
			Type:            fieldType,
			VariableKind:    field.VariableKind,
		}

		origins[identifier] =
			checker.recordFieldDeclarationOrigin(field, fieldType)

		if requireVariableKind &&
			field.VariableKind == ast.VariableKindNotSpecified {

			checker.report(
				&InvalidVariableKindError{
					Kind:  field.VariableKind,
					Range: ast.NewRangeFromPositioned(field.Identifier),
				},
			)
		}
	}

	// declare a member for each function
	for _, function := range functions {
		functionType := checker.functionType(function.ParameterList, function.ReturnTypeAnnotation)

		argumentLabels := function.ParameterList.ArgumentLabels()

		identifier := function.Identifier.Identifier

		members[identifier] = &Member{
			ContainerType:   containerType,
			Access:          function.Access,
			Identifier:      function.Identifier,
			DeclarationKind: common.DeclarationKindFunction,
			Type:            functionType,
			VariableKind:    ast.VariableKindConstant,
			ArgumentLabels:  argumentLabels,
		}

		origins[identifier] =
			checker.recordFunctionDeclarationOrigin(function, functionType)
	}

	return members, origins
}

func (checker *Checker) checkInitializers(
	initializers []*ast.SpecialFunctionDeclaration,
	fields []*ast.FieldDeclaration,
	containerType Type,
	containerDeclarationKind common.DeclarationKind,
	containerTypeIdentifier string,
	initializerParameterTypeAnnotations []*TypeAnnotation,
	containerKind ContainerKind,
	initializationInfo *InitializationInfo,
) {
	count := len(initializers)

	if count == 0 {
		checker.checkNoInitializerNoFields(fields, containerType, containerKind)
		return
	}

	// TODO: check all initializers:
	//  parameter initializerParameterTypeAnnotations needs to be a slice

	initializer := initializers[0]
	checker.checkSpecialFunction(
		initializer,
		containerType,
		containerDeclarationKind,
		containerTypeIdentifier,
		initializerParameterTypeAnnotations,
		containerKind,
		initializationInfo,
	)
}

// checkNoInitializerNoFields checks that if there are no initializers
// there are also no fields – otherwise the fields will be uninitialized.
// In interfaces this is allowed.
//
func (checker *Checker) checkNoInitializerNoFields(
	fields []*ast.FieldDeclaration,
	containerType Type,
	containerKind ContainerKind,
) {
	if len(fields) == 0 || containerKind == ContainerKindInterface {
		return
	}

	// report error for first field
	firstField := fields[0]

	checker.report(
		&MissingInitializerError{
			ContainerType:  containerType,
			FirstFieldName: firstField.Identifier.Identifier,
			FirstFieldPos:  firstField.Identifier.Pos,
		},
	)
}

func (checker *Checker) checkSpecialFunction(
	specialFunction *ast.SpecialFunctionDeclaration,
	containerType Type,
	containerDeclarationKind common.DeclarationKind,
	typeIdentifier string,
	parameterTypeAnnotations []*TypeAnnotation,
	containerKind ContainerKind,
	initializationInfo *InitializationInfo,
) {
	// NOTE: new activation, so `self`
	// is only visible inside the special function

	checkResourceLoss := containerKind != ContainerKindInterface

	checker.enterValueScope()
	defer checker.leaveValueScope(checkResourceLoss)

	checker.declareSelfValue(containerType)

	functionType := &FunctionType{
		ParameterTypeAnnotations: parameterTypeAnnotations,
		ReturnTypeAnnotation:     NewTypeAnnotation(&VoidType{}),
	}

	checker.checkFunction(
		specialFunction.ParameterList,
		ast.Position{},
		functionType,
		specialFunction.FunctionBlock,
		true,
		initializationInfo,
		checkResourceLoss,
	)

	switch containerKind {
	case ContainerKindInterface:
		if specialFunction.FunctionBlock != nil {

			checker.checkInterfaceSpecialFunctionBlock(
				specialFunction.FunctionBlock,
				containerDeclarationKind,
				specialFunction.DeclarationKind,
			)
		}

	case ContainerKindComposite:
		if specialFunction.FunctionBlock == nil {
			checker.report(
				&MissingFunctionBodyError{
					Pos: specialFunction.EndPosition(),
				},
			)
		}
	}
}

func (checker *Checker) checkCompositeFunctions(
	functions []*ast.FunctionDeclaration,
	selfType *CompositeType,
) {
	inResource := selfType.Kind == common.CompositeKindResource

	for _, function := range functions {
		// NOTE: new activation, as function declarations
		// shouldn't be visible in other function declarations,
		// and `self` is is only visible inside function

		func() {
			checker.enterValueScope()
			defer checker.leaveValueScope(true)

			checker.declareSelfValue(selfType)

			checker.visitFunctionDeclaration(
				function,
				functionDeclarationOptions{
					mustExit:                true,
					declareFunction:         false,
					checkResourceLoss:       true,
					allowAuthAccessModifier: inResource,
				},
			)
		}()

		if function.FunctionBlock == nil {
			checker.report(
				&MissingFunctionBodyError{
					Pos: function.EndPosition(),
				},
			)
		}
	}
}

func (checker *Checker) declareSelfValue(selfType Type) {

	// NOTE: declare `self` one depth lower ("inside" function),
	// so it can't be re-declared by the function's parameters

	depth := checker.valueActivations.Depth() + 1

	self := &Variable{
		Identifier:      SelfIdentifier,
		Access:          ast.AccessPublic,
		DeclarationKind: common.DeclarationKindSelf,
		Type:            selfType,
		IsConstant:      true,
		Depth:           depth,
		Pos:             nil,
	}
	checker.valueActivations.Set(SelfIdentifier, self)
	checker.recordVariableDeclarationOccurrence(SelfIdentifier, self)
}

// checkMemberIdentifiers checks the fields and functions are unique and aren't named `init`
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

	// TODO: provide a more helpful error

	switch name {
	case common.DeclarationKindInitializer.Keywords(),
		common.DeclarationKindDestructor.Keywords():

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

func (checker *Checker) VisitFieldDeclaration(field *ast.FieldDeclaration) ast.Repr {
	// NOTE: field type is already checked when determining composite function in `compositeType`

	panic(errors.NewUnreachableError())
}

// checkUnknownSpecialFunctions checks that the special function declarations
// are supported, i.e., they are either initializers or destructors
//
func (checker *Checker) checkUnknownSpecialFunctions(functions []*ast.SpecialFunctionDeclaration) {
	for _, function := range functions {
		switch function.DeclarationKind {
		case common.DeclarationKindInitializer, common.DeclarationKindDestructor:
			continue

		default:
			checker.report(
				&UnknownSpecialFunctionError{
					Pos: function.Identifier.Pos,
				},
			)
		}
	}
}

func (checker *Checker) checkDestructors(
	destructors []*ast.SpecialFunctionDeclaration,
	fields map[string]*ast.FieldDeclaration,
	members map[string]*Member,
	containerType Type,
	containerDeclarationKind common.DeclarationKind,
	containerTypeIdentifier string,
	containerKind ContainerKind,
) {
	count := len(destructors)

	// only resource and resource interface declarations may
	// declare a destructor

	if !containerType.IsResourceType() {
		if count > 0 {
			firstDestructor := destructors[0]

			checker.report(
				&InvalidDestructorError{
					Range: ast.NewRangeFromPositioned(firstDestructor.Identifier),
				},
			)
		}

		return
	}

	if count == 0 {
		checker.checkNoDestructorNoResourceFields(members, fields, containerType, containerKind)
		return
	}

	firstDestructor := destructors[0]
	checker.checkDestructor(
		firstDestructor,
		containerType,
		containerDeclarationKind,
		containerTypeIdentifier,
		containerKind,
	)

	// destructor overloading is not supported

	if count > 1 {
		secondDestructor := destructors[1]

		checker.report(
			&UnsupportedOverloadingError{
				DeclarationKind: common.DeclarationKindDestructor,
				Range:           ast.NewRangeFromPositioned(secondDestructor),
			},
		)
	}
}

// checkNoDestructorNoResourceFields checks that if there is no destructor there are
// also no fields which have a resource type – otherwise those fields will be lost.
// In interfaces this is allowed.
//
func (checker *Checker) checkNoDestructorNoResourceFields(
	members map[string]*Member,
	fields map[string]*ast.FieldDeclaration,
	containerType Type,
	containerKind ContainerKind,
) {
	if containerKind == ContainerKindInterface {
		return
	}

	for memberName, member := range members {
		if !member.Type.IsResourceType() {
			continue
		}

		checker.report(
			&MissingDestructorError{
				ContainerType:  containerType,
				FirstFieldName: memberName,
				FirstFieldPos:  fields[memberName].StartPos,
			},
		)

		// only report for first member
		return
	}
}

func (checker *Checker) checkDestructor(
	destructor *ast.SpecialFunctionDeclaration,
	containerType Type,
	containerDeclarationKind common.DeclarationKind,
	containerTypeIdentifier string,
	containerKind ContainerKind,
) {

	if len(destructor.ParameterList.Parameters) != 0 {
		checker.report(
			&InvalidDestructorParametersError{
				Range: ast.NewRangeFromPositioned(destructor.ParameterList),
			},
		)
	}

	parameterTypeAnnotations :=
		checker.parameterTypeAnnotations(destructor.ParameterList)

	checker.checkSpecialFunction(
		destructor,
		containerType,
		containerDeclarationKind,
		containerTypeIdentifier,
		parameterTypeAnnotations,
		containerKind,
		nil,
	)

	checker.checkCompositeResourceInvalidated(containerType, containerTypeIdentifier)
}

// checkCompositeResourceInvalidated checks that if the container is a resource,
// that all resource fields are invalidated (moved or destroyed)
//
func (checker *Checker) checkCompositeResourceInvalidated(containerType Type, containerTypeIdentifier string) {
	compositeType, isComposite := containerType.(*CompositeType)
	if !isComposite || compositeType.Kind != common.CompositeKindResource {
		return
	}

	checker.checkResourceFieldsInvalidated(containerTypeIdentifier, compositeType.Members)
}

// checkResourceFieldsInvalidated checks that all resource fields for a container
// type are invalidated.
//
func (checker *Checker) checkResourceFieldsInvalidated(containerTypeIdentifier string, members map[string]*Member) {
	for _, member := range members {
		if !member.Type.IsResourceType() {
			return
		}

		info := checker.resources.Get(member)
		if !info.DefinitivelyInvalidated {
			checker.report(
				&ResourceFieldNotInvalidatedError{
					FieldName: member.Identifier.Identifier,
					TypeName:  containerTypeIdentifier,
					Pos:       member.Identifier.StartPosition(),
				},
			)
		}
	}
}

// checkResourceUseAfterInvalidation checks if a resource (variable or composite member)
// is used after it was previously invalidated (moved or destroyed)
//
func (checker *Checker) checkResourceUseAfterInvalidation(resource interface{}, useIdentifier ast.Identifier) {
	resourceInfo := checker.resources.Get(resource)
	if resourceInfo.Invalidations.Size() == 0 {
		return
	}

	checker.report(
		&ResourceUseAfterInvalidationError{
			StartPos:      useIdentifier.StartPosition(),
			EndPos:        useIdentifier.EndPosition(),
			Invalidations: resourceInfo.Invalidations.All(),
		},
	)
}
