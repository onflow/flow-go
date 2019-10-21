package checker

import (
	"fmt"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/common"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/parser"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/pkg/language/runtime/tests/utils"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCheckInvalidImport(t *testing.T) {

	_, err := ParseAndCheck(t, `
       import "unknown"
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.UnresolvedImportError{}, errs[0])
}

func TestCheckInvalidRepeatedImport(t *testing.T) {

	_, err := ParseAndCheckWithExtra(t,
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

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.RepeatedImportError{}, errs[0])
}

func TestCheckImportAll(t *testing.T) {

	checker, err := ParseAndCheck(t, `
       fun answer(): Int {
           return 42
        }
    `)

	assert.Nil(t, err)

	_, err = ParseAndCheckWithExtra(t,
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

	assert.Nil(t, err)
}

func TestCheckInvalidImportUnexported(t *testing.T) {

	checker, err := ParseAndCheck(t, `
       let x = 1
    `)

	assert.Nil(t, err)

	_, err = ParseAndCheckWithExtra(t,
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

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotExportedError{}, errs[0])
}

func TestCheckImportSome(t *testing.T) {

	checker, err := ParseAndCheck(t, `
       fun answer(): Int {
           return 42
       }

       let x = 1
    `)

	assert.Nil(t, err)

	_, err = ParseAndCheckWithExtra(t,
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

	assert.Nil(t, err)
}

func TestCheckInvalidImportedError(t *testing.T) {

	// NOTE: only parse, don't check imported program.
	// will be checked by checker checking importing program

	imported, _, err := parser.ParseProgram(`
       let x: Bool = 1
    `)

	assert.Nil(t, err)

	_, err = ParseAndCheckWithExtra(t,
		`
           import x from "imported"
        `,
		nil,
		nil,
		func(location ast.ImportLocation) (program *ast.Program, e error) {
			return imported, nil
		},
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ImportedProgramError{}, errs[0])
}

func TestCheckImportTypes(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			checker, err := ParseAndCheck(t, fmt.Sprintf(`
               %s Test {}
            `, kind.Keyword()))

			// TODO: add support for non-structure / non-resource declarations

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				assert.Nil(t, err)

			default:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
			}

			_, err = ParseAndCheckWithExtra(t,
				fmt.Sprintf(
					`
                      import "imported"

                      let x: %[1]sTest %[2]s %[3]s Test()
                    `,
					kind.Annotation(),
					kind.TransferOperator(),
					kind.ConstructionKeyword(),
				),
				nil,
				nil,
				func(location ast.ImportLocation) (program *ast.Program, e error) {
					return checker.Program, nil
				},
			)

			// TODO: add support for non-structure / non-resource declarations

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				assert.Nil(t, err)

			default:
				errs := ExpectCheckerErrors(t, err, 3)

				assert.IsType(t, &sema.ImportedProgramError{}, errs[0])
			}

		})
	}
}
