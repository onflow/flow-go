package sema

import (
	"fmt"

	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/ast"
)

type ImportResolver func(location ast.ImportLocation) (*Checker, error)

func (checker *Checker) ResolveImports(resolver ImportResolver) error {
	return checker.resolveImports(
		resolver,
		map[ast.ImportLocation]bool{},
		map[ast.ImportLocation]*Checker{},
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
	resolving map[ast.ImportLocation]bool,
	resolved map[ast.ImportLocation]*Checker,
) error {

	imports := checker.Program.Imports()
	for location := range imports {
		importedChecker, ok := resolved[location]
		if !ok {
			var err error
			importedChecker, err = resolver(location)
			if err != nil {
				return err
			}
			if importedChecker != nil {
				resolved[location] = importedChecker
			}
		}
		if importedChecker == nil {
			continue
		}
		checker.ImportCheckers[location] = importedChecker
		if resolving[location] {
			return CyclicImportsError{Location: location}
		}
		resolving[location] = true
		err := importedChecker.resolveImports(resolver, resolving, resolved)
		if err != nil {
			return err
		}
		delete(resolving, location)
	}
	return nil
}
