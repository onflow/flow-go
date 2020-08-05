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

package checker

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/errors"
	"github.com/onflow/cadence/runtime/parser"
	"github.com/onflow/cadence/runtime/sema"
)

func TestCheckInvalidImport(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
       import "unknown"
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.UnresolvedImportError{}, errs[0])
}

func TestCheckInvalidRepeatedImport(t *testing.T) {

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

	for _, compositeKind := range common.AllCompositeKinds {

		if !compositeKind.SupportsInterfaces() {
			continue
		}

		t.Run(compositeKind.Keyword(), func(t *testing.T) {

			body := "{}"
			if compositeKind == common.CompositeKindEvent {
				body = "()"
			}

			checker, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                       pub %[1]s Test %[2]s

                       pub %[1]s interface TestInterface %[2]s
                    `,
					compositeKind.Keyword(),
					body,
				),
			)

			require.NoError(t, err)

			var useCode string
			if compositeKind != common.CompositeKindContract {
				useCode = fmt.Sprintf(
					`pub let x: %[1]sTest %[2]s %[3]s Test%[4]s`,
					compositeKind.Annotation(),
					compositeKind.TransferOperator(),
					compositeKind.ConstructionKeyword(),
					constructorArguments(compositeKind),
				)
			}

			_, err = ParseAndCheckWithOptions(t,
				fmt.Sprintf(
					`
                      import "imported"

                      pub %[1]s TestImpl: TestInterface {}

                      %[2]s
                    `,
					compositeKind.Keyword(),
					useCode,
				),
				ParseAndCheckOptions{
					ImportResolver: func(location ast.Location) (program *ast.Program, e error) {
						return checker.Program, nil
					},
				},
			)

			switch compositeKind {
			case common.CompositeKindStructure, common.CompositeKindContract:
				require.NoError(t, err)

			case common.CompositeKindResource:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.InvalidResourceCreationError{}, errs[0])

			default:
				panic(errors.NewUnreachableError())
			}
		})
	}
}

func TestCheckInvalidImportCycle(t *testing.T) {

	t.Parallel()

	// NOTE: only parse, don't check imported program.
	// will be checked by checker checking importing program

	const code = `import 0x1`
	imported, _, err := parser.ParseProgram(code)

	require.NoError(t, err)

	_, err = ParseAndCheckWithOptions(t,
		code,
		ParseAndCheckOptions{
			ImportResolver: func(location ast.Location) (program *ast.Program, e error) {
				return imported, nil
			},
		},
	)

	assert.IsType(t, ast.CyclicImportsError{}, err)
}
