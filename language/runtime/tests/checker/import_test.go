package checker

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/language/runtime/ast"
	"github.com/dapperlabs/flow-go/language/runtime/common"
	"github.com/dapperlabs/flow-go/language/runtime/parser"
	"github.com/dapperlabs/flow-go/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/language/runtime/tests/utils"
)

func TestCheckInvalidImport(t *testing.T) {

	_, err := ParseAndCheck(t, `
       import "unknown"
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.UnresolvedImportError{}, errs[0])
}

func TestCheckInvalidRepeatedImport(t *testing.T) {

	_, err := ParseAndCheckWithOptions(t,
		`
           import "unknown"
           import "unknown"
        `,
		ParseAndCheckOptions{
			ImportResolver: func(location ast.Location) (program *ast.Program, e error) {
				return &ast.Program{}, nil
			},
		},
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.RepeatedImportError{}, errs[0])
}

func TestCheckImportAll(t *testing.T) {

	checker, err := ParseAndCheck(t, `
      pub fun answer(): Int {
          return 42
      }
    `)

	require.NoError(t, err)

	_, err = ParseAndCheckWithOptions(t,
		`
          import "imported"

          pub let x = answer()
        `,
		ParseAndCheckOptions{
			ImportResolver: func(location ast.Location) (program *ast.Program, e error) {
				return checker.Program, nil
			},
		},
	)

	require.NoError(t, err)
}

func TestCheckInvalidImportUnexported(t *testing.T) {

	checker, err := ParseAndCheck(t, `
       pub let x = 1
    `)

	require.NoError(t, err)

	_, err = ParseAndCheckWithOptions(t,
		`
           import answer from "imported"

           pub let x = answer()
        `,
		ParseAndCheckOptions{
			ImportResolver: func(location ast.Location) (program *ast.Program, e error) {
				return checker.Program, nil
			},
		},
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotExportedError{}, errs[0])
}

func TestCheckImportSome(t *testing.T) {

	checker, err := ParseAndCheck(t, `
      pub fun answer(): Int {
          return 42
      }

      pub let x = 1
    `)

	require.NoError(t, err)

	_, err = ParseAndCheckWithOptions(t,
		`
          import answer from "imported"

          pub let x = answer()
        `,
		ParseAndCheckOptions{
			ImportResolver: func(location ast.Location) (program *ast.Program, e error) {
				return checker.Program, nil
			},
		},
	)

	require.NoError(t, err)
}

func TestCheckInvalidImportedError(t *testing.T) {

	// NOTE: only parse, don't check imported program.
	// will be checked by checker checking importing program

	imported, _, err := parser.ParseProgram(`
       let x: Bool = 1
    `)

	require.NoError(t, err)

	_, err = ParseAndCheckWithOptions(t,
		`
           import x from "imported"
        `,
		ParseAndCheckOptions{
			ImportResolver: func(location ast.Location) (program *ast.Program, e error) {
				return imported, nil
			},
		},
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ImportedProgramError{}, errs[0])
}

func TestCheckImportTypes(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			checker, err := ParseAndCheck(t, fmt.Sprintf(`
               pub %[1]s Test {}

               pub %[1]s interface TestInterface {}
            `,
				kind.Keyword(),
			))

			// TODO: add support for non-structure / non-resource declarations

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				require.NoError(t, err)

			default:
				errs := ExpectCheckerErrors(t, err, 2)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[1])
			}

			_, err = ParseAndCheckWithOptions(t,
				fmt.Sprintf(
					`
                      import "imported"

                      pub %[1]s TestImpl: TestInterface {}

                      pub let x: %[2]sTest %[3]s %[4]s Test()
                    `,
					kind.Keyword(),
					kind.Annotation(),
					kind.TransferOperator(),
					kind.ConstructionKeyword(),
				),
				ParseAndCheckOptions{
					ImportResolver: func(location ast.Location) (program *ast.Program, e error) {
						return checker.Program, nil
					},
				},
			)

			// TODO: add support for non-structure / non-resource declarations

			switch kind {
			case common.CompositeKindStructure:
				require.NoError(t, err)

			case common.CompositeKindResource:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.CreateImportedResourceError{}, errs[0])

			default:
				errs := ExpectCheckerErrors(t, err, 5)

				assert.IsType(t, &sema.ImportedProgramError{}, errs[0])
				assert.IsType(t, &sema.NotDeclaredError{}, errs[1])
				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[2])
				assert.IsType(t, &sema.NotDeclaredError{}, errs[3])
				assert.IsType(t, &sema.NotDeclaredError{}, errs[4])
			}
		})
	}
}
