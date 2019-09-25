package checker

import (
	"fmt"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/common"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/parser"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/pkg/language/runtime/tests/utils"
	. "github.com/onsi/gomega"
	"testing"
)

func TestCheckInvalidImport(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
       import "unknown"
    `)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnresolvedImportError{}))
}

func TestCheckInvalidRepeatedImport(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheckWithExtra(
		`
           import "unknown"
           import "unknown"
        `,
		nil,
		nil,
		func(location ast.ImportLocation) (program *ast.Program, e error) {
			return &ast.Program{}, nil
		},
	)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.RepeatedImportError{}))
}

func TestCheckImportAll(t *testing.T) {
	RegisterTestingT(t)

	checker, err := ParseAndCheck(`
	   fun answer(): Int {
	       return 42
		}
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	_, err = ParseAndCheckWithExtra(
		`
           import "imported"

           let x = answer()
        `,
		nil,
		nil,
		func(location ast.ImportLocation) (program *ast.Program, e error) {
			return checker.Program, nil
		},
	)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidImportUnexported(t *testing.T) {
	RegisterTestingT(t)

	checker, err := ParseAndCheck(`
       let x = 1
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	_, err = ParseAndCheckWithExtra(
		`
           import answer from "imported"

           let x = answer()
        `,
		nil,
		nil,
		func(location ast.ImportLocation) (program *ast.Program, e error) {
			return checker.Program, nil
		},
	)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotExportedError{}))
}

func TestCheckImportSome(t *testing.T) {
	RegisterTestingT(t)

	checker, err := ParseAndCheck(`
	   fun answer(): Int {
	       return 42
       }

       let x = 1
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	_, err = ParseAndCheckWithExtra(
		`
           import answer from "imported"

           let x = answer()
        `,
		nil,
		nil,
		func(location ast.ImportLocation) (program *ast.Program, e error) {
			return checker.Program, nil
		},
	)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidImportedError(t *testing.T) {
	RegisterTestingT(t)

	// NOTE: only parse, don't check imported program.
	// will be checked by checker checking importing program

	imported, _, err := parser.ParseProgram(`
       let x: Bool = 1
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	_, err = ParseAndCheckWithExtra(
		`
           import x from "imported"
        `,
		nil,
		nil,
		func(location ast.ImportLocation) (program *ast.Program, e error) {
			return imported, nil
		},
	)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.ImportedProgramError{}))
}

func TestCheckImportTypes(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		checker, err := ParseAndCheck(fmt.Sprintf(`
	       %s Test {}
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		if kind == common.CompositeKindStructure {
			Expect(err).
				To(Not(HaveOccurred()))
		} else {
			errs := ExpectCheckerErrors(err, 1)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}

		_, err = ParseAndCheckWithExtra(
			`
               import "imported"

               let x: Test = Test()
            `,
			nil,
			nil,
			func(location ast.ImportLocation) (program *ast.Program, e error) {
				return checker.Program, nil
			},
		)

		// TODO: add support for non-structure declarations

		if kind == common.CompositeKindStructure {
			Expect(err).
				To(Not(HaveOccurred()))
		} else {
			errs := ExpectCheckerErrors(err, 3)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.ImportedProgramError{}))
		}

	}
}
