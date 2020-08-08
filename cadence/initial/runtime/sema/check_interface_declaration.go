/*
 * Cadence - The resource-oriented smart contract programming language
 *
 * Copyright 2019-2020 Dapper Labs, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package sema

import (
	"example.com/cadence-initial/runtime/ast"
	"example.com/cadence-initial/runtime/common"
	"example.com/cadence-initial/runtime/errors"
)

// VisitInterfaceDeclaration checks the given interface declaration.
//
// NOTE: This function assumes that the interface type was previously declared using
// `declareInterfaceType` and exists in `checker.Elaboration.InterfaceDeclarationTypes`,
// and that the members and nested declarations for the interface type were declared
// through `declareInterfaceMembers`.
//
func (checker *Checker) VisitInterfaceDeclaration(declaration *ast.InterfaceDeclaration) ast.Repr {

	const kind = ContainerKindInterface

	interfaceType := checker.Elaboration.InterfaceDeclarationTypes[declaration]
	if interfaceType == nil {
		panic(errors.NewUnreachableError())
	}

	checker.containerTypes[interfaceType] = true
	defer func() {
		checker.containerTypes[interfaceType] = false
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

	// Activate new scope for nested types

	checker.typeActivations.Enter()
	defer checker.typeActivations.Leave()

	// Declare nested types

	checker.declareInterfaceNestedTypes(declaration)

	checker.checkInitializers(
		declaration.Members.Initializers(),
		declaration.Members.Fields,
		interfaceType,
		declaration.DeclarationKind(),
		interfaceType.InitializerParameters,
		kind,
		nil,
	)

	checker.checkUnknownSpecialFunctions(declaration.Members.SpecialFunctions)

	checker.checkInterfaceFunctions(
		declaration.Members.Functions,
		interfaceType,
		declaration.DeclarationKind(),
	)

	fieldPositionGetter := func(name string) ast.Position {
		return declaration.Members.FieldPosition(name, declaration.CompositeKind)
	}

	checker.checkResourceFieldNesting(
		interfaceType.Members,
		interfaceType.CompositeKind,
		fieldPositionGetter,
	)

	checker.checkDestructors(
		declaration.Members.Destructors(),
		declaration.Members.FieldsByIdentifier(),
		interfaceType.Members,
		interfaceType,
		declaration.DeclarationKind(),
		kind,
	)

	// NOTE: visit interfaces first
	// DON'T use `nestedDeclarations`, because of non-deterministic order

	for _, nestedInterface := range declaration.InterfaceDeclarations {
		nestedInterface.Accept(checker)
	}

	for _, nestedComposite := range declaration.CompositeDeclarations {
		// Composite declarations nested in interface declarations are type requirements,
		// i.e. they should be checked like interfaces

		checker.visitCompositeDeclaration(nestedComposite, kind)
	}

	return nil
}

// declareInterfaceNestedTypes declares the types nested in an interface.
// It is used when declaring the interface's members (`declareInterfaceMembers`)
// and checking the interface declaration (`VisitInterfaceDeclaration`).
//
// It assumes the types were previously added to the elaboration in `InterfaceNestedDeclarations`,
// and the type for the declaration was added to the elaboration in `InterfaceDeclarationTypes`.
//
func (checker *Checker) declareInterfaceNestedTypes(
	declaration *ast.InterfaceDeclaration,
) {

	interfaceType := checker.Elaboration.InterfaceDeclarationTypes[declaration]
	nestedDeclarations := checker.Elaboration.InterfaceNestedDeclarations[declaration]

	for name, nestedType := range interfaceType.nestedTypes {
		nestedDeclaration := nestedDeclarations[name]

		identifier := nestedDeclaration.DeclarationIdentifier()
		if identifier == nil {
			// It should be impossible to have a nested declaration
			// that does not have an identifier

			panic(errors.NewUnreachableError())
		}

		_, err := checker.typeActivations.DeclareType(typeDeclaration{
			identifier:               *identifier,
			ty:                       nestedType,
			declarationKind:          nestedDeclaration.DeclarationKind(),
			access:                   nestedDeclaration.DeclarationAccess(),
			allowOuterScopeShadowing: false,
		})
		checker.report(err)
	}
}

func (checker *Checker) checkInterfaceFunctions(
	functions []*ast.FunctionDeclaration,
	selfType Type,
	declarationKind common.DeclarationKind,
) {
	for _, function := range functions {
		// NOTE: new activation, as function declarations
		// shouldn't be visible in other function declarations,
		// and `self` is is only visible inside function

		func() {
			checker.enterValueScope()
			defer checker.leaveValueScope(false)

			checker.declareSelfValue(selfType)

			checker.visitFunctionDeclaration(
				function,
				functionDeclarationOptions{
					mustExit:          false,
					declareFunction:   false,
					checkResourceLoss: false,
				},
			)

			if function.FunctionBlock != nil {
				checker.checkInterfaceSpecialFunctionBlock(
					function.FunctionBlock,
					declarationKind,
					common.DeclarationKindFunction,
				)
			}
		}()
	}
}

// declareInterfaceType declares the type for the given interface declaration
// and records it in the elaboration. It also recursively declares all types
// for all nested declarations.
//
// NOTE: The function does *not* declare any members
//
// See `declareInterfaceMembers` for the declaration of the interface type members.
// See `VisitInterfaceDeclaration` for the checking of the interface declaration.
//
func (checker *Checker) declareInterfaceType(declaration *ast.InterfaceDeclaration) *InterfaceType {

	identifier := declaration.Identifier

	interfaceType := &InterfaceType{
		Location:      checker.Location,
		Identifier:    identifier.Identifier,
		CompositeKind: declaration.CompositeKind,
		nestedTypes:   map[string]Type{},
	}

	variable, err := checker.typeActivations.DeclareType(typeDeclaration{
		identifier:               identifier,
		ty:                       interfaceType,
		declarationKind:          declaration.DeclarationKind(),
		access:                   declaration.Access,
		allowOuterScopeShadowing: false,
	})
	checker.report(err)
	checker.recordVariableDeclarationOccurrence(
		identifier.Identifier,
		variable,
	)

	checker.Elaboration.InterfaceDeclarationTypes[declaration] = interfaceType

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

	checker.Elaboration.InterfaceNestedDeclarations[declaration] = nestedDeclarations

	for _, nestedInterfaceType := range nestedInterfaceTypes {
		interfaceType.nestedTypes[nestedInterfaceType.Identifier] = nestedInterfaceType
		nestedInterfaceType.ContainerType = interfaceType
	}

	for _, nestedCompositeType := range nestedCompositeTypes {
		interfaceType.nestedTypes[nestedCompositeType.Identifier] = nestedCompositeType
		nestedCompositeType.ContainerType = interfaceType
	}

	return interfaceType
}

// declareInterfaceMembers declares the members for the given interface declaration,
// and recursively for all nested declarations.
//
// NOTE: This function assumes that the interface type and the nested declarations' types
// were previously declared using `declareInterfaceType` and exists
// in the elaboration's `InterfaceDeclarationTypes` and `InterfaceNestedDeclarations` fields.
//
func (checker *Checker) declareInterfaceMembers(declaration *ast.InterfaceDeclaration) {

	interfaceType := checker.Elaboration.InterfaceDeclarationTypes[declaration]
	if interfaceType == nil {
		panic(errors.NewUnreachableError())
	}

	// Activate new scope for nested declarations

	checker.typeActivations.Enter()
	defer checker.typeActivations.Leave()

	checker.valueActivations.Enter()
	defer checker.valueActivations.Leave()

	// Declare nested types

	checker.declareInterfaceNestedTypes(declaration)

	// Declare members

	members, origins := checker.nonEventMembersAndOrigins(
		interfaceType,
		declaration.Members.Fields,
		declaration.Members.Functions,
		ContainerKindInterface,
	)

	interfaceType.Members = members
	checker.memberOrigins[interfaceType] = origins

	// NOTE: determine initializer parameter types while nested types are in scope,
	// and after declaring nested types as the initializer may use nested type in parameters

	interfaceType.InitializerParameters =
		checker.initializerParameters(declaration.Members.Initializers())

	// Declare nested declarations' members

	for _, nestedInterfaceDeclaration := range declaration.InterfaceDeclarations {
		checker.declareInterfaceMembers(nestedInterfaceDeclaration)
	}

	for _, nestedCompositeDeclaration := range declaration.CompositeDeclarations {
		checker.declareCompositeMembersAndValue(nestedCompositeDeclaration, ContainerKindInterface)
	}
}

func (checker *Checker) checkInterfaceSpecialFunctionBlock(
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
	} else if (block.PreConditions == nil || len(*block.PreConditions) == 0) &&
		(block.PostConditions == nil || len(*block.PostConditions) == 0) {

		checker.report(
			&InvalidImplementationError{
				Pos:             block.StartPos,
				ContainerKind:   containerKind,
				ImplementedKind: implementedKind,
			},
		)
	}
}
