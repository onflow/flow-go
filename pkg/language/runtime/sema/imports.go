package sema

import (
	"fmt"

	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
)

type ImportResolver func(location ast.ImportLocation) (*Checker, error)

func (checker *Checker) ResolveImports(resolver ImportResolver) error {
	return checker.resolveImports(
		resolver,
		map[ast.LocationID]bool{},
		map[ast.LocationID]*Checker{},
	)
}

type CyclicImportsError struct {
	Location ast.ImportLocation
}

func (e CyclicImportsError) Error() string {
	return fmt.Sprintf("cyclic import of %s", e.Location)
}

func (checker *Checker) resolveImports(
	resolver ImportResolver,
	resolving map[ast.LocationID]bool,
	resolved map[ast.LocationID]*Checker,
) error {
	locations := checker.Program.ImportLocations()

	for _, location := range locations {

		importedChecker, ok := resolved[location.ID()]
		if !ok {
			var err error
			importedChecker, err = resolver(location)
			if err != nil {
				return err
			}
			if importedChecker != nil {
				resolved[location.ID()] = importedChecker
			}
		}

		if importedChecker == nil {
			continue
		}

		checker.ImportCheckers[location.ID()] = importedChecker
		if resolving[location.ID()] {
			return CyclicImportsError{Location: location}
		}

		resolving[location.ID()] = true
		err := importedChecker.resolveImports(resolver, resolving, resolved)
		if err != nil {
			return err
		}

		delete(resolving, location.ID())
	}

	return nil
}
