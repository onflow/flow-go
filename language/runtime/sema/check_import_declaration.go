package sema

import (
	"github.com/dapperlabs/flow-go/language/runtime/ast"
	"github.com/dapperlabs/flow-go/language/runtime/common"
)

func (checker *Checker) VisitImportDeclaration(declaration *ast.ImportDeclaration) ast.Repr {
	// Handled in `declareImportDeclaration`
	panic(&UnreachableStatementError{})
}

func (checker *Checker) declareImportDeclaration(declaration *ast.ImportDeclaration) ast.Repr {
	imports := checker.Program.ImportedPrograms()

	// Find the imported program.
	// If it is not available, report an error and return

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

	// TODO: allow repeated imports of different sets

	// Ensure the program is only imported once.
	// If it was imported before, report an error and return

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

	// Find or create a checker for the imported program

	importChecker, ok := checker.ImportCheckers[declaration.Location.ID()]
	var checkerErr *CheckerError
	if !ok || importChecker == nil {
		var err error
		importChecker, err = NewChecker(
			imported,
			declaration.Location,
			WithPredeclaredValues(checker.PredeclaredValues),
			WithPredeclaredTypes(checker.PredeclaredTypes),
		)
		if err == nil {
			checker.ImportCheckers[declaration.Location.ID()] = importChecker
		}
	}

	// Check the imported program.
	// If there is a checker error, return and import nothing

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

	// Attempt to import the requested value declarations

	foundValues, invalidAccessedValues := checker.importVariables(
		checker.valueActivations,
		declaration.Identifiers,
		importChecker.GlobalValues,
		func(name string) bool {
			// Don't import predeclared values
			if _, ok := importChecker.PredeclaredValues[name]; ok {
				return false
			}

			// Don't import base values
			if _, ok := BaseValues[name]; ok {
				return false
			}

			return true
		},
	)

	// Attempt to import the requested type declarations

	foundTypes, invalidAccessedTypes := checker.importVariables(
		checker.typeActivations,
		declaration.Identifiers,
		importChecker.GlobalTypes,
		func(name string) bool {
			// don't import predeclared types
			if _, ok := importChecker.PredeclaredTypes[name]; ok {
				return false
			}

			return true
		},
	)

	// For each identifier, report if the import is invalid due to
	// restricted access and report an error (i.e. if there is
	// both a value and type with the same name, only report a single error)

	for _, identifier := range declaration.Identifiers {

		invalidAccessVariable, isInvalidAccess := invalidAccessedValues[identifier]
		if !isInvalidAccess {
			invalidAccessVariable, isInvalidAccess = invalidAccessedTypes[identifier]
		}

		if !isInvalidAccess {
			continue
		}

		checker.report(
			&InvalidAccessError{
				Name:              identifier.Identifier,
				RestrictingAccess: invalidAccessVariable.Access,
				DeclarationKind:   invalidAccessVariable.DeclarationKind,
				Range:             ast.NewRangeFromPositioned(identifier),
			},
		)
	}

	// Determine which requested declarations could neither be found
	// in the value nor in the type declarations of the imported program.
	// For each missing import, report an error and declare both a value
	// with an invalid type and an invalid type to avoid spurious errors
	// due to uses of the inaccessible value or type

	missing := make(map[ast.Identifier]bool, len(declaration.Identifiers))
	for _, identifier := range declaration.Identifiers {
		if foundValues[identifier] || foundTypes[identifier] {
			continue
		}

		missing[identifier] = true
	}

	checker.handleMissingImports(missing, declaration.Location)

	return nil
}

func (checker *Checker) handleMissingImports(missing map[ast.Identifier]bool, importLocation ast.Location) {
	for identifier := range missing {
		checker.report(
			&NotExportedError{
				Name:           identifier.Identifier,
				ImportLocation: importLocation,
				Pos:            identifier.Pos,
			},
		)

		// NOTE: declare constant variable with invalid type to silence rest of program
		const access = ast.AccessPrivate

		_, err := checker.valueActivations.Declare(
			identifier.Identifier,
			&InvalidType{},
			access,
			common.DeclarationKindValue,
			identifier.Pos,
			true,
			nil,
		)
		checker.report(err)

		// NOTE: declare type with invalid type to silence rest of program
		_, err = checker.typeActivations.DeclareType(
			identifier,
			&InvalidType{},
			common.DeclarationKindType,
			access,
		)
		checker.report(err)
	}
}

func (checker *Checker) importVariables(
	valueActivations *ValueActivations,
	requestedIdentifiers []ast.Identifier,
	availableVariables map[string]*Variable,
	filter func(name string) bool,
) (
	found map[ast.Identifier]bool,
	invalidAccessed map[ast.Identifier]*Variable,
) {
	found = map[ast.Identifier]bool{}
	invalidAccessed = map[ast.Identifier]*Variable{}

	// Determine which identifiers are imported /
	// which variables need to be declared

	explicitlyImported := map[string]ast.Identifier{}

	var variables map[string]*Variable
	identifierLength := len(requestedIdentifiers)
	if identifierLength > 0 {
		variables = make(map[string]*Variable, identifierLength)
		for _, identifier := range requestedIdentifiers {
			name := identifier.Identifier
			variable := availableVariables[name]
			if variable == nil {
				continue
			}
			variables[name] = variable
			found[identifier] = true
			explicitlyImported[name] = identifier
		}
	} else {
		variables = availableVariables
	}

	for name, variable := range variables {

		if !filter(name) {
			continue
		}

		// If the value can't be imported due to restricted access,
		// report an error, but still import the

		// TODO: handle not-specified access modifier

		// TODO: add option to checker to specify behaviour
		//   for not-specified access modifier

		if variable.Access == ast.AccessPrivate {

			// If the value was imported explicitly,
			// report an error

			if identifier, ok := explicitlyImported[name]; ok {
				invalidAccessed[identifier] = variable
			} else {
				// Don't import not explicitly imported private values
				continue
			}
		}

		_, err := valueActivations.Declare(
			name,
			variable.Type,
			variable.Access,
			variable.DeclarationKind,
			// TODO:
			ast.Position{},
			true,
			variable.ArgumentLabels,
		)
		checker.report(err)
	}

	return
}
