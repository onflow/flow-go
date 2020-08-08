package sema

import (
	"example.com/cadence-initial/runtime/ast"
	"example.com/cadence-initial/runtime/common"
	"example.com/cadence-initial/runtime/errors"
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
// NOTE: This function assumes that the composite type was previously declared using
// `declareCompositeType` and exists in `checker.Elaboration.CompositeDeclarationTypes`,
// and that the members and nested declarations for the composite type were declared
// through `declareCompositeMembersAndValue`.
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

	checker.declareCompositeNestedTypes(declaration, kind, true)

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
		compositeType.ConstructorParameters,
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
			declaration.DeclarationKind(),
		)

	default:
		panic(errors.NewUnreachableError())
	}

	fieldPositionGetter := func(name string) ast.Position {
		return declaration.Members.FieldPosition(name, declaration.CompositeKind)
	}

	checker.checkResourceFieldNesting(
		compositeType.Members,
		compositeType.Kind,
		fieldPositionGetter,
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

	for i, interfaceType := range compositeType.ExplicitInterfaceConformances {
		interfaceNominalType := declaration.Conformances[i]

		checker.checkCompositeConformance(
			declaration,
			compositeType,
			interfaceType,
			interfaceNominalType.Identifier,
			compositeConformanceCheckOptions{
				checkMissingMembers:            checkMissingMembers,
				interfaceTypeIsTypeRequirement: false,
			},
		)
	}

	// NOTE: check destructors after initializer and functions

	checker.withSelfResourceInvalidationAllowed(func() {
		checker.checkDestructors(
			declaration.Members.Destructors(),
			declaration.Members.FieldsByIdentifier(),
			compositeType.Members,
			compositeType,
			declaration.DeclarationKind(),
			kind,
		)
	})

	// NOTE: visit interfaces first
	// DON'T use `nestedDeclarations`, because of non-deterministic order

	for _, nestedInterface := range declaration.InterfaceDeclarations {
		nestedInterface.Accept(checker)
	}

	for _, nestedComposite := range declaration.CompositeDeclarations {
		nestedComposite.Accept(checker)
	}
}

// declareCompositeNestedTypes declares the types nested in a composite,
// and the constructors for them if `declareConstructors` is true
// and `kind` is `ContainerKindComposite`.
//
// It is used when declaring the composite's members (`declareCompositeMembersAndValue`)
// and checking the composite declaration (`visitCompositeDeclaration`).
//
// It assumes the types were previously added to the elaboration in `CompositeNestedDeclarations`,
// and the type for the declaration was added to the elaboration in `CompositeDeclarationTypes`.
//
func (checker *Checker) declareCompositeNestedTypes(
	declaration *ast.CompositeDeclaration,
	kind ContainerKind,
	declareConstructors bool,
) {
	compositeType := checker.Elaboration.CompositeDeclarationTypes[declaration]
	nestedDeclarations := checker.Elaboration.CompositeNestedDeclarations[declaration]

	for name, nestedType := range compositeType.nestedTypes {
		nestedDeclaration := nestedDeclarations[name]

		identifier := nestedDeclaration.DeclarationIdentifier()
		if identifier == nil {
			// It should be impossible to have a nested declaration
			// that does not have an identifier

			panic(errors.NewUnreachableError())
		}

		// NOTE: We allow the shadowing of types here, because the type was already previously
		// declared without allowing shadowing before. This avoids a duplicate error message.

		_, err := checker.typeActivations.DeclareType(typeDeclaration{
			identifier:               *identifier,
			ty:                       nestedType,
			declarationKind:          nestedDeclaration.DeclarationKind(),
			access:                   nestedDeclaration.DeclarationAccess(),
			allowOuterScopeShadowing: true,
		})
		checker.report(err)

		if declareConstructors && kind == ContainerKindComposite {

			// NOTE: Re-declare the constructor function for the nested composite declaration:
			// The constructor was previously declared in `declareCompositeMembersAndValue`
			// for this nested declaration, but the value activation for it was only temporary,
			// so that the constructor wouldn't be visible outside of the containing declaration

			if nestedCompositeDeclaration, isCompositeDeclaration :=
				nestedDeclaration.(*ast.CompositeDeclaration); isCompositeDeclaration {

				nestedCompositeType := nestedType.(*CompositeType)

				nestedConstructorType, nestedConstructorArgumentLabels :=
					checker.compositeConstructorType(nestedCompositeDeclaration, nestedCompositeType)

				_, err := checker.valueActivations.Declare(variableDeclaration{
					identifier:               nestedCompositeDeclaration.Identifier.Identifier,
					ty:                       nestedConstructorType,
					access:                   nestedCompositeDeclaration.Access,
					kind:                     nestedCompositeDeclaration.DeclarationKind(),
					pos:                      nestedCompositeDeclaration.Identifier.Pos,
					isConstant:               true,
					argumentLabels:           nestedConstructorArgumentLabels,
					allowOuterScopeShadowing: false,
				})
				checker.report(err)
			}
		}
	}
}

func (checker *Checker) declareNestedDeclarations(
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

			switch nestedCompositeKind {
			case common.CompositeKindResource,
				common.CompositeKindStructure,
				common.CompositeKindEvent:
				break

			default:
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

		nestedInterfaceType := checker.declareInterfaceType(nestedDeclaration)
		nestedInterfaceTypes = append(nestedInterfaceTypes, nestedInterfaceType)
	}

	// Declare nested composites

	for _, nestedDeclaration := range nestedCompositeDeclarations {
		if _, exists := nestedDeclarations[nestedDeclaration.Identifier.Identifier]; !exists {
			nestedDeclarations[nestedDeclaration.Identifier.Identifier] = nestedDeclaration
		}

		nestedCompositeType := checker.declareCompositeType(nestedDeclaration)
		nestedCompositeTypes = append(nestedCompositeTypes, nestedCompositeType)
	}

	return
}

// declareCompositeType declares the type for the given composite declaration
// and records it in the elaboration. It also recursively declares all types
// for all nested declarations.
//
// NOTE: The function does *not* declare any members or nested declarations.
//
// See `declareCompositeMembersAndValue` for the declaration of the composite type members.
// See `visitCompositeDeclaration` for the checking of the composite declaration.
//
func (checker *Checker) declareCompositeType(declaration *ast.CompositeDeclaration) *CompositeType {

	identifier := declaration.Identifier

	compositeType := &CompositeType{
		Location:    checker.Location,
		Kind:        declaration.CompositeKind,
		Identifier:  identifier.Identifier,
		nestedTypes: map[string]Type{},
	}

	variable, err := checker.typeActivations.DeclareType(typeDeclaration{
		identifier:               identifier,
		ty:                       compositeType,
		declarationKind:          declaration.DeclarationKind(),
		access:                   declaration.Access,
		allowOuterScopeShadowing: false,
	})
	checker.report(err)
	checker.recordVariableDeclarationOccurrence(
		identifier.Identifier,
		variable,
	)

	checker.Elaboration.CompositeDeclarationTypes[declaration] = compositeType

	// Activate new scope for nested declarations

	checker.typeActivations.Enter()
	defer checker.typeActivations.Leave()

	checker.valueActivations.Enter()
	defer checker.valueActivations.Leave()

	// Check and declare nested types

	nestedDeclarations, nestedInterfaceTypes, nestedCompositeTypes :=
		checker.declareNestedDeclarations(
			declaration.CompositeKind,
			declaration.DeclarationKind(),
			declaration.CompositeDeclarations,
			declaration.InterfaceDeclarations,
		)

	checker.Elaboration.CompositeNestedDeclarations[declaration] = nestedDeclarations

	for _, nestedInterfaceType := range nestedInterfaceTypes {
		compositeType.nestedTypes[nestedInterfaceType.Identifier] = nestedInterfaceType
		nestedInterfaceType.ContainerType = compositeType
	}

	for _, nestedCompositeType := range nestedCompositeTypes {
		compositeType.nestedTypes[nestedCompositeType.Identifier] = nestedCompositeType
		nestedCompositeType.ContainerType = compositeType
	}

	return compositeType
}

// declareCompositeMembersAndValue declares the members and the value
// (e.g. constructor function for non-contract types; instance for contracts)
// for the given composite declaration, and recursively for all nested declarations.
//
// NOTE: This function assumes that the composite type was previously declared using
// `declareCompositeType` and exists in `checker.Elaboration.CompositeDeclarationTypes`.
//
func (checker *Checker) declareCompositeMembersAndValue(
	declaration *ast.CompositeDeclaration,
	kind ContainerKind,
) {
	compositeType := checker.Elaboration.CompositeDeclarationTypes[declaration]
	if compositeType == nil {
		panic(errors.NewUnreachableError())
	}

	declarationMembers := map[string]*Member{}

	(func() {
		// Activate new scopes for nested types

		checker.typeActivations.Enter()
		defer checker.typeActivations.Leave()

		checker.valueActivations.Enter()
		defer checker.valueActivations.Leave()

		checker.declareCompositeNestedTypes(declaration, kind, false)

		// Resolve conformances

		conformances := checker.explicitInterfaceConformances(declaration, compositeType)
		compositeType.ExplicitInterfaceConformances = conformances

		// NOTE: determine initializer parameter types while nested types are in scope,
		// and after declaring nested types as the initializer may use nested type in parameters

		initializers := declaration.Members.Initializers()
		compositeType.ConstructorParameters = checker.initializerParameters(initializers)

		// Declare nested declarations' members

		for _, nestedInterfaceDeclaration := range declaration.InterfaceDeclarations {
			checker.declareInterfaceMembers(nestedInterfaceDeclaration)
		}

		for _, nestedCompositeDeclaration := range declaration.CompositeDeclarations {
			checker.declareCompositeMembersAndValue(nestedCompositeDeclaration, kind)

			// Declare nested composites' values (constructor/instance) as members of the containing composite

			identifier := nestedCompositeDeclaration.Identifier

			// Find the value declaration
			nestedCompositeDeclarationVariable :=
				checker.valueActivations.Find(identifier.Identifier)

			declarationMembers[nestedCompositeDeclarationVariable.Identifier] = &Member{
				Identifier:      identifier,
				Access:          nestedCompositeDeclaration.Access,
				ContainerType:   compositeType,
				TypeAnnotation:  NewTypeAnnotation(nestedCompositeDeclarationVariable.Type),
				DeclarationKind: nestedCompositeDeclarationVariable.DeclarationKind,
				VariableKind:    ast.VariableKindConstant,
			}
		}

		// Declare implicit type requirement conformances, if any,
		// after nested types are declared, and
		// after explicit conformances are declared.
		//
		// For each nested composite type, check if a conformance
		// declares a nested composite type with the same identifier,
		// in which case it is a type requirement,
		// and this nested composite type implicitly conforms to it.

		for nestedTypeIdentifier, nestedType := range compositeType.NestedTypes() {
			nestedCompositeType, ok := nestedType.(*CompositeType)
			if !ok {
				continue
			}

			for _, compositeTypeConformance := range compositeType.ExplicitInterfaceConformances {
				conformanceNestedTypes := compositeTypeConformance.NestedTypes()
				if typeRequirement, ok := conformanceNestedTypes[nestedTypeIdentifier].(*CompositeType); ok {
					nestedCompositeType.AddImplicitTypeRequirementConformance(typeRequirement)
				}
			}
		}

		// Declare members
		// NOTE: *After* declaring nested composite and interface declarations

		var members map[string]*Member
		var origins map[string]*Origin

		// Event members are derived from the initializer's parameter list

		if declaration.CompositeKind == common.CompositeKindEvent {
			members, origins = checker.eventMembersAndOrigins(
				initializers[0],
				compositeType,
			)
		} else {
			members, origins = checker.nonEventMembersAndOrigins(
				compositeType,
				declaration.Members.Fields,
				declaration.Members.Functions,
				kind,
			)
		}

		compositeType.Members = members
		checker.memberOrigins[compositeType] = origins
	})()

	// Always determine composite constructor type

	constructorType, constructorArgumentLabels := checker.compositeConstructorType(declaration, compositeType)
	constructorType.Members = declarationMembers

	// If the composite is a contract, declare a value – the contract is a singleton.
	// For all other kinds, declare constructor.

	// NOTE: perform declarations after the nested scope, so they are visible after the declaration

	if compositeType.Kind == common.CompositeKindContract {
		_, err := checker.valueActivations.Declare(variableDeclaration{
			identifier: declaration.Identifier.Identifier,
			ty:         compositeType,
			// NOTE: contracts are always public
			access:                   ast.AccessPublic,
			kind:                     common.DeclarationKindContract,
			pos:                      declaration.Identifier.Pos,
			isConstant:               true,
			argumentLabels:           nil,
			allowOuterScopeShadowing: false,
		})
		checker.report(err)

		for name, declarationMember := range declarationMembers {
			if compositeType.Members[name] != nil {
				continue
			}
			compositeType.Members[name] = declarationMember
		}
	} else {

		// Resource and event constructors are effectively always private,
		// i.e. they should be only constructable by the locations that declare them.
		//
		// Instead of enforcing this be declaring the access as private here,
		// we allow the declared access level and check the construction in the respective
		// construction expressions, i.e. create expressions for resources
		// and emit statements for events.
		//
		// This improves the user experience for the developer:
		// If the access would be enforced as private, an import of the composite
		// would fail with an "not declared" error.

		_, err := checker.valueActivations.Declare(variableDeclaration{
			identifier:               declaration.Identifier.Identifier,
			ty:                       constructorType,
			access:                   declaration.Access,
			kind:                     declaration.DeclarationKind(),
			pos:                      declaration.Identifier.Pos,
			isConstant:               true,
			argumentLabels:           constructorArgumentLabels,
			allowOuterScopeShadowing: false,
		})
		checker.report(err)
	}
}

func (checker *Checker) initializerParameters(initializers []*ast.SpecialFunctionDeclaration) []*Parameter {
	// TODO: support multiple overloaded initializers
	var parameters []*Parameter

	initializerCount := len(initializers)
	if initializerCount > 0 {
		firstInitializer := initializers[0]
		parameters = checker.parameters(firstInitializer.ParameterList)

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
	return parameters
}

func (checker *Checker) explicitInterfaceConformances(
	declaration *ast.CompositeDeclaration,
	compositeType *CompositeType,
) []*InterfaceType {

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

type compositeConformanceCheckOptions struct {
	checkMissingMembers            bool
	interfaceTypeIsTypeRequirement bool
}

func (checker *Checker) checkCompositeConformance(
	compositeDeclaration *ast.CompositeDeclaration,
	compositeType *CompositeType,
	interfaceType *InterfaceType,
	compositeKindMismatchIdentifier ast.Identifier,
	options compositeConformanceCheckOptions,
) {
	var missingMembers []*Member
	var memberMismatches []MemberMismatch
	var missingNestedCompositeTypes []*CompositeType
	var initializerMismatch *InitializerMismatch

	// Ensure the composite kinds match, e.g. a structure shouldn't be able
	// to conform to a resource interface

	if interfaceType.CompositeKind != compositeType.Kind {
		checker.report(
			&CompositeKindMismatchError{
				ExpectedKind: compositeType.Kind,
				ActualKind:   interfaceType.CompositeKind,
				Range:        ast.NewRangeFromPositioned(compositeKindMismatchIdentifier),
			},
		)
	}

	// Check initializer requirement

	// TODO: add support for overloaded initializers

	if interfaceType.InitializerParameters != nil {

		initializerType := &FunctionType{
			Parameters:           compositeType.ConstructorParameters,
			ReturnTypeAnnotation: NewTypeAnnotation(&VoidType{}),
		}
		interfaceInitializerType := &FunctionType{
			Parameters:           interfaceType.InitializerParameters,
			ReturnTypeAnnotation: NewTypeAnnotation(&VoidType{}),
		}

		// TODO: subtype?
		if !initializerType.Equal(interfaceInitializerType) {
			initializerMismatch = &InitializerMismatch{
				CompositeParameters: compositeType.ConstructorParameters,
				InterfaceParameters: interfaceType.InitializerParameters,
			}
		}
	}

	// Determine missing members and member conformance

	for name, interfaceMember := range interfaceType.Members {

		// Conforming types do not provide a concrete member
		// for the member in the interface if it is predeclared

		if interfaceMember.Predeclared {
			continue
		}

		compositeMember, ok := compositeType.Members[name]
		if !ok {
			if options.checkMissingMembers {
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

	for name, typeRequirement := range interfaceType.nestedTypes {

		// Only nested composite declarations are type requirements of the interface

		requiredCompositeType, ok := typeRequirement.(*CompositeType)
		if !ok {
			continue
		}

		nestedCompositeType, ok := compositeType.nestedTypes[name]
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
				CompositeType:                  compositeType,
				InterfaceType:                  interfaceType,
				Pos:                            compositeDeclaration.Identifier.Pos,
				InitializerMismatch:            initializerMismatch,
				MissingMembers:                 missingMembers,
				MemberMismatches:               memberMismatches,
				MissingNestedCompositeTypes:    missingNestedCompositeTypes,
				InterfaceTypeIsTypeRequirement: options.interfaceTypeIsTypeRequirement,
			},
		)
	}
}

// TODO: return proper error
func (checker *Checker) memberSatisfied(compositeMember, interfaceMember *Member) bool {

	// Check declaration kind

	if compositeMember.DeclarationKind != interfaceMember.DeclarationKind {
		return false
	}

	// Check type

	compositeMemberType := compositeMember.TypeAnnotation.Type
	interfaceMemberType := interfaceMember.TypeAnnotation.Type

	if !compositeMemberType.IsInvalidType() &&
		!interfaceMemberType.IsInvalidType() {

		switch interfaceMember.DeclarationKind {
		case common.DeclarationKindField:
			// If the member is just a field, check the types are equal

			// TODO: subtype?
			if !compositeMemberType.Equal(interfaceMemberType) {
				return false
			}

		case common.DeclarationKindFunction:
			// If the member is a function, check the types are equal,
			// but also check that the argument labels match

			interfaceMemberFunctionType := interfaceMemberType.(*FunctionType)
			compositeMemberFunctionType := compositeMemberType.(*FunctionType)

			// TODO: parameters may be supertype, return type may be subtype
			if !compositeMemberFunctionType.EqualIncludingArgumentLabels(interfaceMemberFunctionType) {
				return false
			}

		default:
			panic(errors.NewUnreachableError())
		}
	}

	// Check variable kind

	if interfaceMember.VariableKind != ast.VariableKindNotSpecified &&
		compositeMember.VariableKind != interfaceMember.VariableKind {

		return false
	}

	// Check access

	effectiveInterfaceMemberAccess := checker.effectiveInterfaceMemberAccess(interfaceMember.Access)
	effectiveCompositeMemberAccess := checker.effectiveCompositeMemberAccess(compositeMember.Access)

	return !effectiveCompositeMemberAccess.IsLessPermissiveThan(effectiveInterfaceMemberAccess)
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

	for _, requiredConformance := range requiredCompositeType.ExplicitInterfaceConformances {
		found := false
		for _, conformance := range declaredCompositeType.ExplicitInterfaceConformances {
			if conformance == requiredConformance {
				found = true
				break
			}
		}
		if !found {
			checker.report(
				&MissingConformanceError{
					CompositeType: declaredCompositeType,
					InterfaceType: requiredConformance,
					Range:         ast.NewRangeFromPositioned(compositeDeclaration.Identifier),
				},
			)
		}
	}

	// Check the conformance of the composite to the type requirement
	// like a top-level composite declaration to an interface type

	requiredInterfaceType := requiredCompositeType.InterfaceType()

	checker.checkCompositeConformance(
		compositeDeclaration,
		declaredCompositeType,
		requiredInterfaceType,
		compositeDeclaration.Identifier,
		compositeConformanceCheckOptions{
			checkMissingMembers:            true,
			interfaceTypeIsTypeRequirement: true,
		},
	)
}

func (checker *Checker) compositeConstructorType(
	compositeDeclaration *ast.CompositeDeclaration,
	compositeType *CompositeType,
) (
	constructorFunctionType *SpecialFunctionType,
	argumentLabels []string,
) {

	constructorFunctionType = &SpecialFunctionType{
		FunctionType: &FunctionType{
			ReturnTypeAnnotation: NewTypeAnnotation(compositeType),
		},
	}

	// TODO: support multiple overloaded initializers

	initializers := compositeDeclaration.Members.Initializers()
	if len(initializers) > 0 {
		firstInitializer := initializers[0]

		argumentLabels = firstInitializer.ParameterList.EffectiveArgumentLabels()

		constructorFunctionType.Parameters = compositeType.ConstructorParameters

		// NOTE: Don't use `constructorFunctionType`, as it has a return type.
		//   The initializer itself has a `Void` return type.

		checker.Elaboration.SpecialFunctionTypes[firstInitializer] =
			&SpecialFunctionType{
				FunctionType: &FunctionType{
					Parameters:           constructorFunctionType.Parameters,
					ReturnTypeAnnotation: NewTypeAnnotation(&VoidType{}),
				},
			}
	}

	return constructorFunctionType, argumentLabels
}

func (checker *Checker) nonEventMembersAndOrigins(
	containerType Type,
	fields []*ast.FieldDeclaration,
	functions []*ast.FunctionDeclaration,
	containerKind ContainerKind,
) (
	members map[string]*Member,
	origins map[string]*Origin,
) {
	requireVariableKind := containerKind != ContainerKindInterface
	requireNonPrivateMemberAccess := containerKind == ContainerKindInterface

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
		checker.checkTypeAnnotation(fieldTypeAnnotation, field.TypeAnnotation)

		const declarationKind = common.DeclarationKindField

		effectiveAccess := checker.effectiveMemberAccess(field.Access, containerKind)

		if requireNonPrivateMemberAccess &&
			effectiveAccess == ast.AccessPrivate {

			checker.report(
				&InvalidAccessModifierError{
					DeclarationKind: declarationKind,
					Access:          field.Access,
					Explanation:     "private fields can never be used",
					Pos:             field.StartPos,
				},
			)
		}

		members[identifier] = &Member{
			ContainerType:   containerType,
			Access:          field.Access,
			Identifier:      field.Identifier,
			DeclarationKind: declarationKind,
			TypeAnnotation:  fieldTypeAnnotation,
			VariableKind:    field.VariableKind,
		}

		origins[identifier] =
			checker.recordFieldDeclarationOrigin(
				field.Identifier,
				field.StartPos,
				field.EndPos,
				fieldTypeAnnotation.Type,
			)

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

		argumentLabels := function.ParameterList.EffectiveArgumentLabels()

		fieldTypeAnnotation := &TypeAnnotation{Type: functionType}

		const declarationKind = common.DeclarationKindFunction

		effectiveAccess := checker.effectiveMemberAccess(function.Access, containerKind)

		if requireNonPrivateMemberAccess &&
			effectiveAccess == ast.AccessPrivate {

			checker.report(
				&InvalidAccessModifierError{
					DeclarationKind: declarationKind,
					Access:          function.Access,
					Explanation:     "private functions can never be used",
					Pos:             function.StartPos,
				},
			)
		}

		members[identifier] = &Member{
			ContainerType:   containerType,
			Access:          function.Access,
			Identifier:      function.Identifier,
			DeclarationKind: declarationKind,
			TypeAnnotation:  fieldTypeAnnotation,
			VariableKind:    ast.VariableKindConstant,
			ArgumentLabels:  argumentLabels,
		}

		origins[identifier] =
			checker.recordFunctionDeclarationOrigin(function, functionType)
	}

	return members, origins
}

func (checker *Checker) eventMembersAndOrigins(
	initializer *ast.SpecialFunctionDeclaration,
	containerType *CompositeType,
) (
	members map[string]*Member,
	origins map[string]*Origin,
) {
	parameters := initializer.ParameterList.Parameters

	members = make(map[string]*Member, len(parameters))
	origins = make(map[string]*Origin, len(parameters))

	for i, parameter := range parameters {
		typeAnnotation := containerType.ConstructorParameters[i].TypeAnnotation

		identifier := parameter.Identifier

		members[identifier.Identifier] = &Member{
			ContainerType:   containerType,
			Access:          ast.AccessPublic,
			Identifier:      identifier,
			DeclarationKind: common.DeclarationKindField,
			TypeAnnotation:  typeAnnotation,
			VariableKind:    ast.VariableKindConstant,
		}

		origins[identifier.Identifier] =
			checker.recordFieldDeclarationOrigin(
				identifier,
				parameter.StartPos,
				parameter.EndPos,
				typeAnnotation.Type,
			)
	}

	return
}

func (checker *Checker) checkInitializers(
	initializers []*ast.SpecialFunctionDeclaration,
	fields []*ast.FieldDeclaration,
	containerType Type,
	containerDeclarationKind common.DeclarationKind,
	initializerParameters []*Parameter,
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
		initializerParameters,
		containerKind,
		initializationInfo,
	)

	// If the initializer is for an event,
	// ensure all parameters are valid

	if compositeType, ok := containerType.(*CompositeType); ok &&
		compositeType.Kind == common.CompositeKindEvent {

		checker.checkEventParameters(
			initializer.ParameterList,
			initializerParameters,
		)
	}
}

// checkNoInitializerNoFields checks that if there are no initializers,
// then there should also be no fields. Otherwise the fields will be uninitialized.
// In interfaces this is allowed.
//
func (checker *Checker) checkNoInitializerNoFields(
	fields []*ast.FieldDeclaration,
	containerType Type,
	containerKind ContainerKind,
) {
	// If there are no fields, or the container is an interface,
	// no initializer needs to be declared

	if len(fields) == 0 || containerKind == ContainerKindInterface {
		return
	}

	// An initializer should be declared but does not exist.
	// Report an error for the first field

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
	parameters []*Parameter,
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
		Parameters:           parameters,
		ReturnTypeAnnotation: NewTypeAnnotation(&VoidType{}),
	}

	checker.checkFunction(
		specialFunction.ParameterList,
		nil,
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
				specialFunction.Kind,
			)
		}

	case ContainerKindComposite:
		// Event declarations have an empty initializer as it is synthesized

		compositeType := containerType.(*CompositeType)
		if compositeType.Kind != common.CompositeKindEvent &&
			specialFunction.FunctionBlock == nil {

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
					mustExit:          true,
					declareFunction:   false,
					checkResourceLoss: true,
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

func (checker *Checker) VisitFieldDeclaration(_ *ast.FieldDeclaration) ast.Repr {
	// NOTE: field type is already checked when determining composite function in `compositeType`

	panic(errors.NewUnreachableError())
}

// checkUnknownSpecialFunctions checks that the special function declarations
// are supported, i.e., they are either initializers or destructors
//
func (checker *Checker) checkUnknownSpecialFunctions(functions []*ast.SpecialFunctionDeclaration) {
	for _, function := range functions {
		switch function.Kind {
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
		// NOTE: check type, not resource annotation:
		// the field could have a wrong annotation
		if !member.TypeAnnotation.Type.IsResourceType() {
			continue
		}

		checker.report(
			&MissingDestructorError{
				ContainerType:  containerType,
				FirstFieldName: memberName,
				FirstFieldPos:  fields[memberName].Identifier.Pos,
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
	containerKind ContainerKind,
) {

	if len(destructor.ParameterList.Parameters) != 0 {
		checker.report(
			&InvalidDestructorParametersError{
				Range: ast.NewRangeFromPositioned(destructor.ParameterList),
			},
		)
	}

	parameters := checker.parameters(destructor.ParameterList)

	checker.checkSpecialFunction(
		destructor,
		containerType,
		containerDeclarationKind,
		parameters,
		containerKind,
		nil,
	)

	checker.checkCompositeResourceInvalidated(containerType)
}

// checkCompositeResourceInvalidated checks that if the container is a resource,
// that all resource fields are invalidated (moved or destroyed)
//
func (checker *Checker) checkCompositeResourceInvalidated(containerType Type) {
	compositeType, isComposite := containerType.(*CompositeType)
	if !isComposite || compositeType.Kind != common.CompositeKindResource {
		return
	}

	checker.checkResourceFieldsInvalidated(containerType, compositeType.Members)
}

// checkResourceFieldsInvalidated checks that all resource fields for a container
// type are invalidated.
//
func (checker *Checker) checkResourceFieldsInvalidated(containerType Type, members map[string]*Member) {
	for _, member := range members {

		// NOTE: check the of the type annotation, not the type annotation's
		// resource marker: the field could have an incorrect type annotation
		// that is missing the resource marker even though it is required

		if !member.TypeAnnotation.Type.IsResourceType() {
			continue
		}

		info := checker.resources.Get(member)
		if !info.DefinitivelyInvalidated {
			checker.report(
				&ResourceFieldNotInvalidatedError{
					FieldName: member.Identifier.Identifier,
					Type:      containerType,
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
