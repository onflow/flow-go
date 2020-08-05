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
	"github.com/onflow/cadence-initial/runtime/ast"
	"github.com/onflow/cadence-initial/runtime/common"
)

func (checker *Checker) VisitImportDeclaration(_ *ast.ImportDeclaration) ast.Repr {
	// Handled in `declareImportDeclaration`
	panic(&UnreachableStatementError{})
}

func (checker *Checker) declareImportDeclaration(declaration *ast.ImportDeclaration) ast.Repr {
	imports := checker.Program.ImportedPrograms()

	locationID := declaration.Location.ID()

	// Find the imported program.
	// If it is not available, report an error and return

	imported := imports[locationID]
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

	if checker.seenImports[locationID] {
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
	checker.seenImports[locationID] = true

	importChecker, checkerErr := checker.EnsureLoaded(
		declaration.Location,
		func() *ast.Program {
			return imported
		},
	)
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

// EnsureLoaded finds or create a checker for the imported program and checks it.
//
func (checker *Checker) EnsureLoaded(location ast.Location, loadProgram func() *ast.Program) (*Checker, *CheckerError) {
	locationID := location.ID()

	subChecker, ok := checker.allCheckers[locationID]
	if ok {
		return subChecker, nil
	}

	if !ok || subChecker == nil {
		var err error
		subChecker, err = NewChecker(
			loadProgram(),
			location,
			WithPredeclaredValues(checker.PredeclaredValues),
			WithPredeclaredTypes(checker.PredeclaredTypes),
			WithAccessCheckMode(checker.accessCheckMode),
			WithValidTopLevelDeclarationsHandler(checker.validTopLevelDeclarationsHandler),
			WithAllCheckers(checker.allCheckers),
			WithCheckHandler(checker.checkHandler),
		)
		if err == nil {
			checker.allCheckers[locationID] = subChecker
		}
	}

	// Check the imported program.
	// If there is a checker error, return and import nothing

	// NOTE: ignore generic `error` result, get internal *CheckerError
	_ = subChecker.Check()
	checkerErr := subChecker.CheckerError()

	return subChecker, checkerErr
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

		_, err := checker.valueActivations.Declare(variableDeclaration{
			identifier:               identifier.Identifier,
			ty:                       &InvalidType{},
			access:                   access,
			kind:                     common.DeclarationKindValue,
			pos:                      identifier.Pos,
			isConstant:               true,
			allowOuterScopeShadowing: false,
		})
		checker.report(err)

		// NOTE: declare type with invalid type to silence rest of program
		_, err = checker.typeActivations.DeclareType(typeDeclaration{
			identifier:               identifier,
			ty:                       &InvalidType{},
			declarationKind:          common.DeclarationKindType,
			access:                   access,
			allowOuterScopeShadowing: false,
		})
		checker.report(err)
	}
}

func (checker *Checker) importVariables(
	valueActivations *VariableActivations,
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
	identifiersCount := len(requestedIdentifiers)
	if identifiersCount > 0 {
		variables = make(map[string]*Variable, identifiersCount)
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

		// If the variable can't be imported due to restricted access,
		// report an error, but still import the variable

		access := variable.Access

		if !checker.isReadableAccess(access) {

			// If the variable was imported explicitly, report an error

			if identifier, ok := explicitlyImported[name]; ok {
				invalidAccessed[identifier] = variable
			} else {
				// Don't import not explicitly imported inaccessible variable
				continue
			}
		}

		_, err := valueActivations.Declare(variableDeclaration{
			identifier: name,
			ty:         variable.Type,
			// TODO: implies that type is "re-exported"
			access: access,
			kind:   variable.DeclarationKind,
			// TODO:
			pos:                      ast.Position{},
			isConstant:               true,
			argumentLabels:           variable.ArgumentLabels,
			allowOuterScopeShadowing: false,
		})
		checker.report(err)
	}

	return
}
