package sema

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/common"
)

func (checker *Checker) VisitImportDeclaration(declaration *ast.ImportDeclaration) ast.Repr {
	imports := checker.Program.ImportedPrograms()

	imported := imports[declaration.Location.ID()]
	if imported == nil {
		checker.report(
			&UnresolvedImportError{
				ImportLocation: declaration.Location,
				Range:          ast.NewRangeFromPositioned(declaration),
			},
		)
		return nil
	}

	if checker.seenImports[declaration.Location.ID()] {
		checker.report(
			&RepeatedImportError{
				ImportLocation: declaration.Location,
				Range: ast.Range{
					StartPos: declaration.LocationPos,
					EndPos:   declaration.LocationPos,
				},
			},
		)
		return nil
	}
	checker.seenImports[declaration.Location.ID()] = true

	importChecker, ok := checker.ImportCheckers[declaration.Location.ID()]
	var checkerErr *CheckerError
	if !ok || importChecker == nil {
		var err error
		importChecker, err = NewChecker(
			imported,
			checker.PredeclaredValues,
			checker.PredeclaredTypes,
			declaration.Location,
		)
		if err == nil {
			checker.ImportCheckers[declaration.Location.ID()] = importChecker
		}
	}

	// NOTE: ignore generic `error` result, get internal *CheckerError
	_ = importChecker.Check()
	checkerErr = importChecker.CheckerError()

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

	for identifier := range missing {
		checker.report(
			&NotExportedError{
				Name:           identifier.Identifier,
				ImportLocation: declaration.Location,
				Pos:            identifier.Pos,
			},
		)

		// NOTE: declare constant variable with invalid type to silence rest of program
		_, err := checker.valueActivations.Declare(
			identifier.Identifier,
			&InvalidType{},
			common.DeclarationKindValue,
			identifier.Pos,
			true,
			nil,
		)
		checker.report(err)

		// NOTE: declare type with invalid type to silence rest of program
		err = checker.typeActivations.Declare(identifier, &InvalidType{})
		checker.report(err)
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

		_, err := checker.valueActivations.Declare(
			name,
			variable.Type,
			variable.DeclarationKind,
			declaration.LocationPos,
			true,
			variable.ArgumentLabels,
		)
		checker.report(err)
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
		err := checker.typeActivations.Declare(identifier, ty)
		checker.report(err)
	}
}
