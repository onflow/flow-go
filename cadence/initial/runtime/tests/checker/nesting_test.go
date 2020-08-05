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

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/sema"
)

func TestCheckCompositeDeclarationNesting(t *testing.T) {

	t.Parallel()

	interfacePossibilities := []bool{true, false}

	for _, outerComposite := range common.CompositeKindsWithBody {
		for _, outerIsInterface := range interfacePossibilities {
			for _, innerComposite := range common.AllCompositeKinds {
				for _, innerIsInterface := range interfacePossibilities {
					if innerIsInterface && innerComposite == common.CompositeKindEvent {
						continue
					}

					outer := outerComposite.DeclarationKind(outerIsInterface)
					inner := innerComposite.DeclarationKind(innerIsInterface)

					testName := fmt.Sprintf("%s/%s", outer, inner)

					t.Run(testName, func(t *testing.T) {

						innerBody := "{}"
						if innerComposite == common.CompositeKindEvent {
							innerBody = "()"
						}

						_, err := ParseAndCheck(t,
							fmt.Sprintf(
								`
                                  %[1]s Outer {
                                      %[2]s Inner %[3]s
                                  }
                                `,
								outer.Keywords(),
								inner.Keywords(),
								innerBody,
							),
						)

						switch outerComposite {
						case common.CompositeKindContract:

							switch innerComposite {
							case common.CompositeKindContract:
								errs := ExpectCheckerErrors(t, err, 1)

								assert.IsType(t, &sema.InvalidNestedDeclarationError{}, errs[0])

							case common.CompositeKindResource,
								common.CompositeKindStructure,
								common.CompositeKindEvent:

								require.NoError(t, err)

							default:
								t.Errorf("unknown outer composite kind %s", outerComposite)
							}

						case common.CompositeKindResource,
							common.CompositeKindStructure:

							errs := ExpectCheckerErrors(t, err, 1)

							assert.IsType(t, &sema.InvalidNestedDeclarationError{}, errs[0])

						default:
							t.Errorf("unknown outer composite kind %s", outerComposite)
						}
					})
				}
			}
		}
	}
}

func TestCheckCompositeDeclarationNestedStructUse(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      contract Test {

          struct X {}

          var x: X

          init(x: X) {
              self.x = x
          }
      }
    `)

	assert.NoError(t, err)
}

func TestCheckCompositeDeclarationNestedStructInterfaceUse(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      contract Test {

          struct interface XI {}

          struct X: XI {}

          var xi: AnyStruct{XI}

          init(xi: AnyStruct{XI}) {
              self.xi = xi
          }

          fun test() {
              self.xi = X()
          }
      }
    `)

	assert.NoError(t, err)
}

func TestCheckCompositeDeclarationNestedTypeScopingInsideNestedOuter(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      contract Test {

          struct X {

              fun test() {
                  Test
              }
          }
      }
   `)

	assert.NoError(t, err)
}

func TestCheckCompositeDeclarationNestedTypeScopingOuterInner(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      contract Test {

          struct X {}

          fun x(): X {
             return X()
          }
      }
    `)

	assert.NoError(t, err)
}

func TestCheckInvalidCompositeDeclarationNestedTypeScopingAfterInner(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      contract Test {

          struct X {}
      }

      let x: X = X()
    `)

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
	assert.IsType(t, &sema.NotDeclaredError{}, errs[1])
}

func TestCheckInvalidCompositeDeclarationNestedDuplicateNames(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      contract Test {

          struct X {}

          fun X() {}
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.RedeclarationError{}, errs[0])
}

func TestCheckCompositeDeclarationNestedConstructorAndType(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      contract Test {

          struct X {}
      }

      let x: Test.X = Test.X()
    `)

	assert.NoError(t, err)
}

func TestCheckInvalidCompositeDeclarationNestedType(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      contract Test {

          fun X() {}
      }

      let x: Test.X = Test.X()
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
}

func TestCheckInvalidNestedType(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      let x: Int.X = 1
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidNestedTypeError{}, errs[0])
}
