package sema

import (
	"github.com/dapperlabs/flow-go/language/runtime/ast"
	"github.com/dapperlabs/flow-go/language/runtime/common"
	"github.com/dapperlabs/flow-go/language/runtime/errors"
)

func (checker *Checker) VisitCompositeDeclaration(declaration *ast.CompositeDeclaration) ast.Repr {
	checker.visitCompositeDeclaration(declaration, ContainerKindComposite)

	return nil
}

// visitCompositeDeclaration checks a previously declared composite declaration.
// Checking behaviour depends on `kind`, i.e. if the composite declaration declares
// a composite (`kind` is `ContainerKindComposite`), or the composite declaration is
// nested in an interface and so acts as a type requirement (`kind` is `ContainerKindInterface`).
//
func (checker *Checker) visitCompositeDeclaration(declaration *ast.CompositeDeclaration, kind ContainerKind) {

	compositeType := checker.Elaboration.CompositeDeclarationTypes[declaration]
	if compositeType == nil {
		panic(errors.NewUnreachableError())
	}

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

	checker.checkNestedIdentifiers(
		declaration.Members.Fields,
		declaration.Members.Functions,
		declaration.InterfaceDeclarations,
		declaration.CompositeDeclarations,
	)

	// Activate new scopes for nested types

	checker.typeActivations.Enter()
	defer checker.typeActivations.Leave()

	if kind == ContainerKindComposite {
		checker.valueActivations.Enter()
		defer checker.valueActivations.Leave()
	}

	// Declare nested types

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

		if kind == ContainerKindComposite {
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
	}

	var initializationInfo *InitializationInfo

	if kind == ContainerKindComposite {
		// The initializer must initialize all members that are fields,
		// e.g. not composite functions (which are by definition constant and "initialized")

		fieldMembers := map[*Member]*ast.FieldDeclaration{}

		for _, field := range declaration.Members.Fields {
			fieldName := field.Identifier.Identifier
			member := compositeType.Members[fieldName]
			fieldMembers[member] = field
		}

		initializationInfo = NewInitializationInfo(compositeType, fieldMembers)
	}

	checker.checkInitializers(
		declaration.Members.Initializers(),
		declaration.Members.Fields,
		compositeType,
		declaration.DeclarationKind(),
		declaration.Identifier.Identifier,
		compositeType.ConstructorParameterTypeAnnotations,
		kind,
		initializationInfo,
	)

	checker.checkUnknownSpecialFunctions(declaration.Members.SpecialFunctions)

	switch kind {
	case ContainerKindComposite:
		checker.checkCompositeFunctions(declaration.Members.Functions, compositeType)

	case ContainerKindInterface:
		checker.checkInterfaceFunctions(
			declaration.Members.Functions,
			compositeType,
			declaration.CompositeKind,
			declaration.DeclarationKind(),
		)

	default:
		panic(errors.NewUnreachableError())
	}

	checker.checkResourceFieldNesting(
		declaration.Members.FieldsByIdentifier(),
		compositeType.Members,
		compositeType.Kind,
	)

	// Check conformances
	// NOTE: perform after completing composite type (e.g. setting constructor parameter types)

	// If the composite declaration is declaring a composite (`kind` is `ContainerKindComposite`),
	// rather than a type requirement (`kind` is `ContainerKindInterface`), check that the composite
	// conforms to all interfaces the composite declared it conforms to, i.e. all members match,
	// and no members are missing.

	// If the composite declaration is a type requirement (`kind` is `ContainerKindInterface`),
	// DON'T check that the composite conforms to all interfaces the composite declared it
	// conforms to – these are requirements that the composite declaration of the implementation
	// of the containing interface must conform to.
	//
	// Thus, missing members are valid, but still check that members that are declared as requirements
	// match the members of the conformances (members in the interface)

	checkMissingMembers := kind != ContainerKindInterface

	for _, interfaceType := range compositeType.Conformances {

		checker.checkCompositeConformance(
			declaration,
			compositeType,
			interfaceType,
			checkMissingMembers,
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
		kind,
	)

	// NOTE: visit interfaces first
	// DON'T use `nestedDeclarations`, because of non-deterministic order

	for _, nestedInterface := range declaration.InterfaceDeclarations {
		nestedInterface.Accept(checker)
	}

	for _, nestedComposite := range declaration.CompositeDeclarations {
		nestedComposite.Accept(checker)
	}
}

func (checker *Checker) declareNestedDeclarations(
	containerKind ContainerKind,
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

		// NOTE: don't return, so nested declarations / types are still declared
	} else {

		// Check contract's nested composite declarations and interface declarations
		// are a resource (interface) or a struct (interface)

		checkNestedDeclaration := func(
			nestedCompositeKind common.CompositeKind,
			nestedDeclarationKind common.DeclarationKind,
			identifier ast.Identifier,
		) {

			if nestedCompositeKind != common.CompositeKindResource &&
				nestedCompositeKind != common.CompositeKindStructure {

				checker.report(
					&InvalidNestedDeclarationError{
						NestedDeclarationKind:    nestedDeclarationKind,
						ContainerDeclarationKind: containerDeclarationKind,
						Range:                    ast.NewRangeFromPositioned(identifier),
					},
				)

			}
		}

		for _, nestedDeclaration := range nestedInterfaceDeclarations {
			checkNestedDeclaration(
				nestedDeclaration.CompositeKind,
				nestedDeclaration.DeclarationKind(),
				nestedDeclaration.Identifier,
			)
		}

		for _, nestedDeclaration := range nestedCompositeDeclarations {
			checkNestedDeclaration(
				nestedDeclaration.CompositeKind,
				nestedDeclaration.DeclarationKind(),
				nestedDeclaration.Identifier,
			)
		}

		// NOTE: don't return, so nested declarations / types are still declared
	}

	// Declare nested interfaces

	for _, nestedDeclaration := range nestedInterfaceDeclarations {
		if _, exists := nestedDeclarations[nestedDeclaration.Identifier.Identifier]; !exists {
			nestedDeclarations[nestedDeclaration.Identifier.Identifier] = nestedDeclaration
		}

		nestedInterfaceType := checker.declareInterfaceDeclaration(nestedDeclaration)
		nestedInterfaceTypes = append(nestedInterfaceTypes, nestedInterfaceType)
	}

	// Declare nested composites

	for _, nestedDeclaration := range nestedCompositeDeclarations {
		if _, exists := nestedDeclarations[nestedDeclaration.Identifier.Identifier]; !exists {
			nestedDeclarations[nestedDeclaration.Identifier.Identifier] = nestedDeclaration
		}

		// pre-declare composite
		nestedCompositeType := checker.declareCompositeDeclaration(nestedDeclaration, containerKind)
		nestedCompositeTypes = append(nestedCompositeTypes, nestedCompositeType)
	}

	return
}

func (checker *Checker) declareCompositeDeclaration(declaration *ast.CompositeDeclaration, kind ContainerKind) *CompositeType {

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

	constructorMembers := map[string]*Member{}

	(func() {
		// Activate new scopes for nested types

		checker.typeActivations.Enter()
		defer checker.typeActivations.Leave()

		checker.valueActivations.Enter()
		defer checker.valueActivations.Leave()

		// Check and declare nested types before checking members

		nestedDeclarations, nestedInterfaceTypes, nestedCompositeTypes :=
			checker.declareNestedDeclarations(
				kind,
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

		// Declare nested composites' constructors as members of the containing composite

		for _, nestedCompositeDeclaration := range declaration.CompositeDeclarations {
			identifier := nestedCompositeDeclaration.Identifier

			// Find the constructor declaration
			nestedCompositeDeclarationVariable :=
				checker.valueActivations.Find(identifier.Identifier)

			constructorMembers[nestedCompositeDeclarationVariable.Identifier] = &Member{
				Identifier:      identifier,
				Access:          nestedCompositeDeclaration.Access,
				ContainerType:   compositeType,
				TypeAnnotation:  NewTypeAnnotation(nestedCompositeDeclarationVariable.Type),
				DeclarationKind: nestedCompositeDeclarationVariable.DeclarationKind,
				VariableKind:    ast.VariableKindConstant,
			}
		}

		// Check conformances and members

		conformances := checker.conformances(declaration, compositeType)
		compositeType.Conformances = conformances

		members, origins := checker.membersAndOrigins(
			compositeType,
			declaration.Members.Fields,
			declaration.Members.Functions,
			kind != ContainerKindInterface,
		)

		compositeType.Members = members
		checker.memberOrigins[compositeType] = origins

		// NOTE: determine initializer parameter types while nested types are in scope,
		// and after declaring nested types as the initializer may use nested type in parameters

		compositeType.ConstructorParameterTypeAnnotations =
			checker.initializerParameterTypeAnnotations(declaration.Members.Initializers())
	})()

	// Declare constructor after the nested scope, so it is visible after the declaration

	constructorFunction := checker.declareCompositeConstructor(declaration, compositeType)
	constructorFunction.Members = constructorMembers

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

func (checker *Checker) conformances(declaration *ast.CompositeDeclaration, compositeType *CompositeType) []*InterfaceType {

	var interfaceTypes []*InterfaceType
	seenConformances := map[string]bool{}

	for _, conformance := range declaration.Conformances {
		convertedType := checker.ConvertType(conformance)

		if interfaceType, ok := convertedType.(*InterfaceType); ok {
			interfaceTypes = append(interfaceTypes, interfaceType)

			conformanceIdentifier := conformance.String()

			if seenConformances[conformanceIdentifier] {
				checker.report(
					&DuplicateConformanceError{
						CompositeType: compositeType,
						InterfaceType: interfaceType,
						Range:         ast.NewRangeFromPositioned(conformance.Identifier),
					},
				)
			}

			seenConformances[conformanceIdentifier] = true

		} else if !convertedType.IsInvalidType() {
			checker.report(
				&InvalidConformanceError{
					Type: convertedType,
					Pos:  conformance.StartPosition(),
				},
			)
		}
	}

	return interfaceTypes
}

func (checker *Checker) checkCompositeConformance(
	compositeDeclaration *ast.CompositeDeclaration,
	compositeType *CompositeType,
	interfaceType *InterfaceType,
	checkMissingMembers bool,
) {
	var missingMembers []*Member
	var memberMismatches []MemberMismatch
	var missingNestedCompositeTypes []*CompositeType
	var initializerMismatch *InitializerMismatch

	// Ensure the composite kinds match, e.g. a structure shouldn't be able
	// to conform to a resource interface

	if compositeType.Kind != interfaceType.CompositeKind {
		checker.report(
			&CompositeKindMismatchError{
				ExpectedKind: interfaceType.CompositeKind,
				ActualKind:   compositeType.Kind,
				Range:        ast.NewRangeFromPositioned(compositeDeclaration.Identifier),
			},
		)
	}

	// Check initializer requirement

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

	// Determine missing members and member conformance

	for name, interfaceMember := range interfaceType.Members {

		compositeMember, ok := compositeType.Members[name]
		if !ok {
			if checkMissingMembers {
				missingMembers = append(missingMembers, interfaceMember)
			}
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

	// Determine missing nested composite type definitions

	for name, typeRequirement := range interfaceType.NestedTypes {

		// Only nested composite declarations are type requirements of the interface

		requiredCompositeType, ok := typeRequirement.(*CompositeType)
		if !ok {
			continue
		}

		nestedCompositeType, ok := compositeType.NestedTypes[name]
		if !ok {
			missingNestedCompositeTypes = append(missingNestedCompositeTypes, requiredCompositeType)
			continue
		}

		checker.checkTypeRequirement(nestedCompositeType, compositeDeclaration, requiredCompositeType)
	}

	if len(missingMembers) > 0 ||
		len(memberMismatches) > 0 ||
		len(missingNestedCompositeTypes) > 0 ||
		initializerMismatch != nil {

		checker.report(
			&ConformanceError{
				CompositeType:               compositeType,
				InterfaceType:               interfaceType,
				Pos:                         compositeDeclaration.Identifier.Pos,
				InitializerMismatch:         initializerMismatch,
				MissingMembers:              missingMembers,
				MemberMismatches:            memberMismatches,
				MissingNestedCompositeTypes: missingNestedCompositeTypes,
			},
		)
	}
}

// TODO: return proper error
func (checker *Checker) memberSatisfied(compositeMember, interfaceMember *Member) bool {
	// Check type

	// TODO: subtype?
	if !compositeMember.TypeAnnotation.Type.
		Equal(interfaceMember.TypeAnnotation.Type) {

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

// checkTypeRequirement checks conformance of a nested type declaration
// to a type requirement of an interface.
//
func (checker *Checker) checkTypeRequirement(
	declaredType Type,
	containerDeclaration *ast.CompositeDeclaration,
	requiredCompositeType *CompositeType,
) {

	// A nested interface doesn't satisfy the type requirement,
	// it must be a composite

	if declaredInterfaceType, ok := declaredType.(*InterfaceType); ok {

		// Find the interface declaration of the interface type

		var errorRange ast.Range
		var foundInterfaceDeclaration bool

		for _, nestedInterfaceDeclaration := range containerDeclaration.InterfaceDeclarations {
			nestedInterfaceIdentifier := nestedInterfaceDeclaration.Identifier.Identifier
			if nestedInterfaceIdentifier == declaredInterfaceType.Identifier {
				foundInterfaceDeclaration = true
				errorRange = ast.NewRangeFromPositioned(nestedInterfaceDeclaration.Identifier)
				break
			}
		}

		if !foundInterfaceDeclaration {
			panic(errors.NewUnreachableError())
		}

		checker.report(
			&DeclarationKindMismatchError{
				ExpectedDeclarationKind: requiredCompositeType.Kind.DeclarationKind(false),
				ActualDeclarationKind:   declaredInterfaceType.CompositeKind.DeclarationKind(true),
				Range:                   errorRange,
			},
		)

		return
	}

	// If the nested type is neither an interface nor a composite,
	// something must be wrong in the checker

	declaredCompositeType, ok := declaredType.(*CompositeType)
	if !ok {
		panic(errors.NewUnreachableError())
	}

	// Find the composite declaration of the composite type

	var compositeDeclaration *ast.CompositeDeclaration

	for _, nestedCompositeDeclaration := range containerDeclaration.CompositeDeclarations {
		nestedCompositeIdentifier := nestedCompositeDeclaration.Identifier.Identifier
		if nestedCompositeIdentifier == declaredCompositeType.Identifier {
			compositeDeclaration = nestedCompositeDeclaration
			break
		}
	}

	if compositeDeclaration == nil {
		panic(errors.NewUnreachableError())
	}

	// Check that the composite declaration declares at least the conformances
	// that the type requirement stated

	for _, requiredConformance := range requiredCompositeType.Conformances {
		found := false
		for _, conformance := range declaredCompositeType.Conformances {
			if conformance == requiredConformance {
				found = true
				break
			}
		}
		if !found {
			checker.report(&MissingConformanceError{
				CompositeType: declaredCompositeType,
				InterfaceType: requiredConformance,
				Range:         ast.NewRangeFromPositioned(compositeDeclaration.Identifier),
			})
		}
	}

	// Check the conformance of the composite to the type requirement
	// like a top-level composite declaration to an interface type

	interfaceType := requiredCompositeType.InterfaceType()

	checker.checkCompositeConformance(
		compositeDeclaration,
		declaredCompositeType,
		interfaceType,
		true,
	)
}

func (checker *Checker) declareCompositeConstructor(
	compositeDeclaration *ast.CompositeDeclaration,
	compositeType *CompositeType,
) *SpecialFunctionType {

	functionType := &SpecialFunctionType{
		FunctionType: &FunctionType{
			ReturnTypeAnnotation: NewTypeAnnotation(compositeType),
		},
	}

	var argumentLabels []string

	// TODO: support multiple overloaded initializers

	initializers := compositeDeclaration.Members.Initializers()
	if len(initializers) > 0 {
		firstInitializer := initializers[0]

		argumentLabels = firstInitializer.ParameterList.ArgumentLabels()

		functionType.ParameterTypeAnnotations = compositeType.ConstructorParameterTypeAnnotations

		checker.Elaboration.SpecialFunctionTypes[firstInitializer] = functionType
	}

	_, err := checker.valueActivations.DeclareFunction(
		compositeDeclaration.Identifier,
		compositeDeclaration.Access,
		functionType,
		argumentLabels,
	)
	checker.report(err)

	return functionType
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

	predeclaredMembers := checker.predeclaredMembers(containerType)
	invalidIdentifiers := make(map[string]bool, len(predeclaredMembers))

	for _, predeclaredMember := range predeclaredMembers {
		name := predeclaredMember.Identifier.Identifier
		members[name] = predeclaredMember
		invalidIdentifiers[name] = true
	}

	checkInvalidIdentifier := func(declaration ast.Declaration) bool {
		identifier := declaration.DeclarationIdentifier()
		if invalidIdentifiers == nil || !invalidIdentifiers[identifier.Identifier] {
			return true
		}

		checker.report(
			&InvalidDeclarationError{
				Identifier: identifier.Identifier,
				Kind:       declaration.DeclarationKind(),
				Range:      ast.NewRangeFromPositioned(identifier),
			},
		)

		return false
	}

	// declare a member for each field
	for _, field := range fields {
		if !checkInvalidIdentifier(field) {
			continue
		}

		identifier := field.Identifier.Identifier

		fieldTypeAnnotation := checker.ConvertTypeAnnotation(field.TypeAnnotation)

		checker.checkTypeAnnotation(fieldTypeAnnotation, field.TypeAnnotation.StartPos)

		members[identifier] = &Member{
			ContainerType:   containerType,
			Access:          field.Access,
			Identifier:      field.Identifier,
			DeclarationKind: common.DeclarationKindField,
			TypeAnnotation:  fieldTypeAnnotation,
			VariableKind:    field.VariableKind,
		}

		origins[identifier] =
			checker.recordFieldDeclarationOrigin(field, fieldTypeAnnotation.Type)

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
		if !checkInvalidIdentifier(function) {
			continue
		}

		identifier := function.Identifier.Identifier

		functionType := checker.functionType(function.ParameterList, function.ReturnTypeAnnotation)

		argumentLabels := function.ParameterList.ArgumentLabels()

		fieldTypeAnnotation := &TypeAnnotation{Type: functionType}

		members[identifier] = &Member{
			ContainerType:   containerType,
			Access:          function.Access,
			Identifier:      function.Identifier,
			DeclarationKind: common.DeclarationKindFunction,
			TypeAnnotation:  fieldTypeAnnotation,
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

// checkNestedIdentifiers checks that nested identifiers, i.e. fields, functions,
// and nested interfaces and composites, are unique and aren't named `init` or `destroy`
//
func (checker *Checker) checkNestedIdentifiers(
	fields []*ast.FieldDeclaration,
	functions []*ast.FunctionDeclaration,
	nestedInterfaceDeclarations []*ast.InterfaceDeclaration,
	nestedCompositeDeclarations []*ast.CompositeDeclaration,
) {
	positions := map[string]ast.Position{}

	for _, field := range fields {
		checker.checkNestedIdentifier(
			field.Identifier,
			common.DeclarationKindField,
			positions,
		)
	}

	for _, function := range functions {
		checker.checkNestedIdentifier(
			function.Identifier,
			common.DeclarationKindFunction,
			positions,
		)
	}

	for _, interfaceDeclaration := range nestedInterfaceDeclarations {
		checker.checkNestedIdentifier(
			interfaceDeclaration.Identifier,
			interfaceDeclaration.DeclarationKind(),
			positions,
		)
	}

	for _, compositeDeclaration := range nestedCompositeDeclarations {
		checker.checkNestedIdentifier(
			compositeDeclaration.Identifier,
			compositeDeclaration.DeclarationKind(),
			positions,
		)
	}
}

// checkNestedIdentifier checks that the nested identifier is unique
// and isn't named `init` or `destroy`
//
func (checker *Checker) checkNestedIdentifier(
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
		// NOTE: check type, not move annotation:
		// the field could have a wrong annotation
		if !member.TypeAnnotation.Type.IsResourceType() {
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
		// NOTE: check type, not move annotation:
		// the field could have a wrong annotation
		if !member.TypeAnnotation.Type.IsResourceType() {
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
