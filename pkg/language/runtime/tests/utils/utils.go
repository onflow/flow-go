package utils

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/parser"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
	. "github.com/onsi/gomega"
)

func ParseAndCheck(code string) (*sema.Checker, error) {
	return ParseAndCheckWithExtra(code, nil, nil, nil)
}

func ParseAndCheckWithExtra(
	code string,
	values map[string]sema.ValueDeclaration,
	types map[string]sema.TypeDeclaration,
	resolver ast.ImportResolver,
) (*sema.Checker, error) {
	program, _, err := parser.ParseProgram(code)

	Expect(err).
		To(Not(HaveOccurred()))

	if resolver != nil {
		err := program.ResolveImports(resolver)
		if err != nil {
			return nil, err
		}
	}

	checker, err := sema.NewChecker(program, values, types)
	if err != nil {
		return checker, err
	}

	err = checker.Check()
	return checker, err
}

func ExpectCheckerErrors(err error, len int) []error {
	if len <= 0 && err == nil {
		return nil
	}

	Expect(err).To(HaveOccurred())

	Expect(err).
		To(BeAssignableToTypeOf(&sema.CheckerError{}))

	errs := err.(*sema.CheckerError).Errors

	Expect(errs).To(HaveLen(len))

	return errs
}
