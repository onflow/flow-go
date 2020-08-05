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

	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/errors"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence/runtime/sema"
)

func TestCheckEventDeclaration(t *testing.T) {

	t.Parallel()

	t.Run("ValidEvent", func(t *testing.T) {
		checker, err := ParseAndCheck(t, `
            event Transfer(to: Int, from: Int)
        `)

		require.NoError(t, err)

		transferType := checker.GlobalTypes["Transfer"].Type

		require.IsType(t, &sema.CompositeType{}, transferType)
		transferCompositeType := transferType.(*sema.CompositeType)

		require.Len(t, transferCompositeType.Members, 2)
		assert.Equal(t, &sema.IntType{}, transferCompositeType.Members["to"].TypeAnnotation.Type)
		assert.Equal(t, &sema.IntType{}, transferCompositeType.Members["from"].TypeAnnotation.Type)
	})

	t.Run("InvalidEventNonPrimitiveTypeComposite", func(t *testing.T) {

		for _, compositeKind := range common.CompositeKindsWithBody {
			if compositeKind == common.CompositeKindContract {
				continue
			}

			t.Run(compositeKind.Name(), func(t *testing.T) {

				_, err := ParseAndCheck(t,
					fmt.Sprintf(
						`
                          %[1]s Token {
                            let id: String

                            init(id: String) {
                              self.id = id
                            }
                          }

                          event Transfer(token: %[2]sToken)
                        `,
						compositeKind.Keyword(),
						compositeKind.Annotation(),
					),
				)

				switch compositeKind {
				case common.CompositeKindResource:
					errs := ExpectCheckerErrors(t, err, 3)

					assert.IsType(t, &sema.ResourceLossError{}, errs[0])
					assert.IsType(t, &sema.InvalidEventParameterTypeError{}, errs[1])
					assert.IsType(t, &sema.InvalidResourceFieldError{}, errs[2])

				case common.CompositeKindStructure:
					errs := ExpectCheckerErrors(t, err, 1)

					assert.IsType(t, &sema.InvalidEventParameterTypeError{}, errs[0])

				default:
					panic(errors.NewUnreachableError())
				}
			})
		}
	})

	t.Run("PrimitiveTypedFields", func(t *testing.T) {

		validTypes := append(
			sema.AllNumberTypes,
			&sema.StringType{},
			&sema.BoolType{},
		)

		for _, ty := range validTypes {

			t.Run(ty.String(), func(t *testing.T) {

				_, err := ParseAndCheck(t,
					fmt.Sprintf(
						`
                          event Transfer(value: %s)
                        `,
						ty.String(),
					),
				)

				require.NoError(t, err)
			})
		}
	})

	t.Run("EventParameterType", func(t *testing.T) {

		validTypes := append(
			[]sema.Type{
				&sema.StringType{},
				&sema.CharacterType{},
				&sema.BoolType{},
				&sema.AddressType{},
			},
			sema.AllNumberTypes...,
		)

		tests := validTypes[:]

		for _, validType := range validTypes {
			tests = append(tests,
				&sema.OptionalType{Type: validType},
				&sema.VariableSizedType{Type: validType},
				&sema.ConstantSizedType{Type: validType},
				&sema.DictionaryType{KeyType: validType, ValueType: validType},
			)
		}

		for _, ty := range tests {

			t.Run(ty.String(), func(t *testing.T) {

				_, err := ParseAndCheck(t,
					fmt.Sprintf(
						`
                          event Transfer(_ value: %s)
                        `,
						ty,
					),
				)

				require.NoError(t, err)
			})
		}
	})

	t.Run("RedeclaredEvent", func(t *testing.T) {
		_, err := ParseAndCheck(t, `
            event Transfer(to: Int, from: Int)
            event Transfer(to: Int)
		`)

		// NOTE: two redeclaration errors: one for type, one for function

		errs := ExpectCheckerErrors(t, err, 2)

		assert.IsType(t, &sema.RedeclarationError{}, errs[0])
		assert.IsType(t, &sema.RedeclarationError{}, errs[1])
	})
}

func TestCheckEmitEvent(t *testing.T) {

	t.Parallel()

	t.Run("ValidEvent", func(t *testing.T) {
		_, err := ParseAndCheck(t, `
            event Transfer(to: Int, from: Int)

            fun test() {
                emit Transfer(to: 1, from: 2)
            }
        `)

		require.NoError(t, err)
	})

	t.Run("MissingEmitStatement", func(t *testing.T) {
		_, err := ParseAndCheck(t, `
            event Transfer(to: Int, from: Int)

            fun test() {
                Transfer(to: 1, from: 2)
            }
        `)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.InvalidEventUsageError{}, errs[0])
	})

	t.Run("EmitNonEvent", func(t *testing.T) {
		_, err := ParseAndCheck(t, `
            fun notAnEvent(): Int { return 1 }

            fun test() {
                emit notAnEvent()
            }
        `)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.EmitNonEventError{}, errs[0])
	})

	t.Run("EmitNotDeclared", func(t *testing.T) {
		_, err := ParseAndCheck(t, `
            fun test() {
              emit notAnEvent()
            }
        `)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
	})

	t.Run("EmitImported", func(t *testing.T) {
		checker, err := ParseAndCheck(t, `
            pub event Transfer(to: Int, from: Int)
        `)
		require.NoError(t, err)

		_, err = ParseAndCheckWithOptions(t, `
              import Transfer from "imported"

              pub fun test() {
                  emit Transfer(to: 1, from: 2)
              }
            `,
			ParseAndCheckOptions{
				ImportResolver: func(location ast.Location) (program *ast.Program, e error) {
					return checker.Program, nil
				},
			},
		)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.EmitImportedEventError{}, errs[0])
	})
}
