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
	locations, imports := checker.Program.Imports()
	for locationID := range imports {
		location := locations[locationID]

		importedChecker, ok := resolved[locationID]
		if !ok {
			var err error
			importedChecker, err = resolver(location)
			if err != nil {
				return err
			}
			if importedChecker != nil {
				resolved[locationID] = importedChecker
			}
		}

		if importedChecker == nil {
			continue
		}

		checker.ImportCheckers[locationID] = importedChecker
		if resolving[locationID] {
			return CyclicImportsError{Location: location}
		}

		resolving[locationID] = true
		err := importedChecker.resolveImports(resolver, resolving, resolved)
		if err != nil {
			return err
		}
		delete(resolving, locationID)
	}
	return nil
}
