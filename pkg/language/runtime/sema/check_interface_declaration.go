package sema

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/common"
)

func (checker *Checker) VisitInterfaceDeclaration(declaration *ast.InterfaceDeclaration) ast.Repr {

	interfaceType := checker.Elaboration.InterfaceDeclarationTypes[declaration]

	// TODO: also check nested composite members

	// TODO: also check nested composite members' identifiers

	checker.checkMemberIdentifiers(
		declaration.Members.Fields,
		declaration.Members.Functions,
	)

	members, origins := checker.membersAndOrigins(
		declaration.Members.Fields,
		declaration.Members.Functions,
		false,
	)

	interfaceType.Members = members

	checker.memberOrigins[interfaceType] = origins

	checker.checkMemberIdentifiers(
		declaration.Members.Fields,
		declaration.Members.Functions,
	)

	checker.checkInitializers(
		declaration.Members.Initializers,
		declaration.Members.Fields,
		interfaceType,
		declaration.DeclarationKind(),
		declaration.Identifier.Identifier,
		interfaceType.InitializerParameterTypeAnnotations,
		initializerKindInterface,
	)

	checker.checkInterfaceFunctions(
		declaration.Members.Functions,
		interfaceType,
		declaration.DeclarationKind(),
	)

	checker.checkResourceFieldNesting(
		declaration.Members.FieldsByIdentifier(),
		interfaceType.Members,
		interfaceType.CompositeKind,
	)

	// TODO: support non-structure interfaces, such as contracts and resources

	if declaration.CompositeKind != common.CompositeKindStructure {
		checker.report(
			&UnsupportedDeclarationError{
				DeclarationKind: declaration.DeclarationKind(),
				StartPos:        declaration.Identifier.StartPosition(),
				EndPos:          declaration.Identifier.EndPosition(),
			},
		)
	}

	// TODO: support nested declarations for contracts and contract interfaces

	// report error for first nested composite declaration, if any
	if len(declaration.Members.CompositeDeclarations) > 0 {
		firstNestedCompositeDeclaration := declaration.Members.CompositeDeclarations[0]

		checker.report(
			&UnsupportedDeclarationError{
				DeclarationKind: firstNestedCompositeDeclaration.DeclarationKind(),
				StartPos:        firstNestedCompositeDeclaration.Identifier.StartPosition(),
				EndPos:          firstNestedCompositeDeclaration.Identifier.EndPosition(),
			},
		)
	}

	return nil
}

func (checker *Checker) checkInterfaceFunctions(
	functions []*ast.FunctionDeclaration,
	interfaceType Type,
	declarationKind common.DeclarationKind,
) {
	for _, function := range functions {
		// NOTE: new activation, as function declarations
		// shouldn't be visible in other function declarations,
		// and `self` is is only visible inside function

		checker.withValueScope(func() {
			// NOTE: required for
			checker.declareSelfValue(interfaceType)

			checker.visitFunctionDeclaration(function, false)

			if function.FunctionBlock != nil {
				checker.checkInterfaceFunctionBlock(
					function.FunctionBlock,
					declarationKind,
					common.DeclarationKindFunction,
				)
			}
		})
	}
}

func (checker *Checker) declareInterfaceDeclaration(declaration *ast.InterfaceDeclaration) {

	// NOTE: fields and functions might already refer to interface itself.
	// insert a dummy type for now, so lookup succeeds during conversion,
	// then fix up the type reference

	interfaceType := &InterfaceType{}

	identifier := declaration.Identifier

	err := checker.typeActivations.Declare(identifier, interfaceType)
	checker.report(err)
	checker.recordVariableDeclarationOccurrence(
		identifier.Identifier,
		&Variable{
			Identifier: identifier.Identifier,
			Kind:       declaration.DeclarationKind(),
			IsConstant: true,
			Type:       interfaceType,
			Pos:        &identifier.Pos,
		},
	)

	// NOTE: members are added in `VisitInterfaceDeclaration` –
	//   left out for now, as field and function requirements could refer to e.g. composites
	*interfaceType = InterfaceType{
		CompositeKind: declaration.CompositeKind,
		Identifier:    identifier.Identifier,
	}

	// TODO: support multiple overloaded initializers

	var parameterTypeAnnotations []*TypeAnnotation
	initializerCount := len(declaration.Members.Initializers)
	if initializerCount > 0 {
		firstInitializer := declaration.Members.Initializers[0]
		parameterTypeAnnotations = checker.parameterTypeAnnotations(firstInitializer.Parameters)

		if initializerCount > 1 {
			secondInitializer := declaration.Members.Initializers[1]

			checker.report(
				&UnsupportedOverloadingError{
					DeclarationKind: common.DeclarationKindInitializer,
					StartPos:        secondInitializer.StartPosition(),
					EndPos:          secondInitializer.EndPosition(),
				},
			)
		}
	}

	interfaceType.InitializerParameterTypeAnnotations = parameterTypeAnnotations

	checker.Elaboration.InterfaceDeclarationTypes[declaration] = interfaceType
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
