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

	"github.com/onflow/cadence/runtime/cmd"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/errors"
	"github.com/onflow/cadence/runtime/sema"
	. "github.com/onflow/cadence/runtime/tests/utils"
)

func TestCheckInvalidCompositeRedeclaringType(t *testing.T) {

	t.Parallel()

	for _, kind := range common.AllCompositeKinds {

		body := "{}"
		if kind == common.CompositeKindEvent {
			body = "()"
		}

		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s Int %[2]s
                    `,
					kind.Keyword(),
					body,
				),
			)

			// NOTE: two redeclaration errors: one for type, one for function

			errs := ExpectCheckerErrors(t, err, 2)

			assert.IsType(t, &sema.RedeclarationError{}, errs[0])
			assert.IsType(t, &sema.RedeclarationError{}, errs[1])
		})
	}
}

func TestCheckComposite(t *testing.T) {

	t.Parallel()

	for _, kind := range common.CompositeKindsWithBody {

		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %s Test {
                          pub(set) var foo: Int

                          init(foo: Int) {
                              self.foo = foo
                          }

                          pub fun getFoo(): Int {
                              return self.foo
                          }
                      }
                    `,
					kind.Keyword(),
				),
			)

			require.NoError(t, err)
		})
	}
}

func TestCheckInitializerName(t *testing.T) {

	t.Parallel()

	for _, kind := range common.CompositeKindsWithBody {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %s Test {
                          init() {}
                      }
                    `,
					kind.Keyword(),
				),
			)

			require.NoError(t, err)
		})
	}
}

func TestCheckDestructor(t *testing.T) {

	t.Parallel()

	for _, kind := range common.CompositeKindsWithBody {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %s Test {
                          destroy() {}
                      }
                    `,
					kind.Keyword(),
				),
			)

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindContract:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.InvalidDestructorError{}, errs[0])

			case common.CompositeKindResource:
				require.NoError(t, err)

			default:
				panic(errors.NewUnreachableError())
			}
		})
	}
}

func TestCheckInvalidUnknownSpecialFunction(t *testing.T) {

	t.Parallel()

	interfacePossibilities := []bool{true, false}

	for _, kind := range common.CompositeKindsWithBody {
		for _, isInterface := range interfacePossibilities {

			interfaceKeyword := ""
			if isInterface {
				interfaceKeyword = "interface"
			}

			testName := fmt.Sprintf("%s_%s", kind.Keyword(), interfaceKeyword)

			t.Run(testName, func(t *testing.T) {

				_, err := ParseAndCheck(t,
					fmt.Sprintf(
						`
                          %[1]s %[2]s Test {
                              initializer() {}
                          }
                        `,
						kind.Keyword(),
						interfaceKeyword,
					),
				)

				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.UnknownSpecialFunctionError{}, errs[0])
			})
		}
	}
}

func TestCheckInvalidCompositeFieldNames(t *testing.T) {

	t.Parallel()

	interfacePossibilities := []bool{true, false}

	for _, kind := range common.CompositeKindsWithBody {
		for _, isInterface := range interfacePossibilities {

			interfaceKeyword := ""
			if isInterface {
				interfaceKeyword = "interface"
			}

			testName := fmt.Sprintf("%s_%s", kind.Keyword(), interfaceKeyword)

			t.Run(testName, func(t *testing.T) {

				_, err := ParseAndCheck(t,
					fmt.Sprintf(
						`
                          %[1]s %[2]s Test {
                              let init: Int
                              let destroy: Bool
                          }
                        `,
						kind.Keyword(),
						interfaceKeyword,
					),
				)

				if isInterface {
					errs := ExpectCheckerErrors(t, err, 2)

					assert.IsType(t, &sema.InvalidNameError{}, errs[0])
					assert.IsType(t, &sema.InvalidNameError{}, errs[1])
				} else {
					errs := ExpectCheckerErrors(t, err, 3)

					assert.IsType(t, &sema.InvalidNameError{}, errs[0])
					assert.IsType(t, &sema.InvalidNameError{}, errs[1])
					assert.IsType(t, &sema.MissingInitializerError{}, errs[2])
				}
			})
		}
	}
}

func TestCheckInvalidCompositeFunctionNames(t *testing.T) {

	t.Parallel()

	interfacePossibilities := []bool{true, false}

	for _, kind := range common.CompositeKindsWithBody {
		for _, isInterface := range interfacePossibilities {

			interfaceKeyword := ""
			if isInterface {
				interfaceKeyword = "interface"
			}

			body := "{}"
			if isInterface {
				body = ""
			}

			testName := fmt.Sprintf("%s_%s", kind.Keyword(), interfaceKeyword)

			t.Run(testName, func(t *testing.T) {

				_, err := ParseAndCheck(t,
					fmt.Sprintf(
						`
                          %[1]s %[2]s Test {
                              fun init() %[3]s
                              fun destroy() %[3]s
                          }
                        `,
						kind.Keyword(),
						interfaceKeyword,
						body,
					),
				)

				errs := ExpectCheckerErrors(t, err, 2)

				assert.IsType(t, &sema.InvalidNameError{}, errs[0])
				assert.IsType(t, &sema.InvalidNameError{}, errs[1])
			})
		}
	}
}

func TestCheckInvalidCompositeRedeclaringFields(t *testing.T) {

	t.Parallel()

	for _, kind := range common.AllCompositeKinds {

		t.Run(kind.Keyword(), func(t *testing.T) {

			var body string
			if kind == common.CompositeKindEvent {
				body = `
                  (
                      x: Int,
                      x: Int
                  )
                `
			} else {
				body = `
                  {
                      let x: Int
                      let x: Int
                  }
                `
			}

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s Test %[2]s
                    `,
					kind.Keyword(),
					body,
				),
			)

			if kind == common.CompositeKindEvent {
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.RedeclarationError{}, errs[0])
			} else {
				errs := ExpectCheckerErrors(t, err, 2)

				assert.IsType(t, &sema.RedeclarationError{}, errs[0])
				assert.IsType(t, &sema.MissingInitializerError{}, errs[1])
			}
		})
	}
}

func TestCheckInvalidCompositeRedeclaringFunctions(t *testing.T) {

	t.Parallel()

	for _, kind := range common.CompositeKindsWithBody {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %s Test {
                          fun x() {}
                          fun x() {}
                      }
                    `,
					kind.Keyword(),
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.RedeclarationError{}, errs[0])
		})
	}
}

func TestCheckInvalidCompositeRedeclaringFieldsAndFunctions(t *testing.T) {

	t.Parallel()

	for _, kind := range common.CompositeKindsWithBody {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %s Test {
                          let x: Int
                          fun x() {}
                      }
                    `,
					kind.Keyword(),
				),
			)

			errs := ExpectCheckerErrors(t, err, 2)

			assert.IsType(t, &sema.RedeclarationError{}, errs[0])
			assert.IsType(t, &sema.MissingInitializerError{}, errs[1])
		})
	}
}

func TestCheckCompositeFieldsAndFunctions(t *testing.T) {

	t.Parallel()

	for _, kind := range common.CompositeKindsWithBody {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %s Test {
                          let x: Int

                          init() {
                              self.x = 1
                          }

                          fun y() {}
                      }
                    `,
					kind.Keyword(),
				),
			)

			require.NoError(t, err)
		})
	}
}

func TestCheckInvalidCompositeFieldType(t *testing.T) {

	t.Parallel()

	for _, kind := range common.AllCompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			if kind == common.CompositeKindEvent {
				_, err := ParseAndCheck(t,
					fmt.Sprintf(
						`
                          %s Test(x: X)
                        `,
						kind.Keyword(),
					),
				)

				errs := ExpectCheckerErrors(t, err, 1)
				assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
			} else {
				_, err := ParseAndCheck(t,
					fmt.Sprintf(
						`
                          %s Test {
                              let x: X
                          }
                        `,
						kind.Keyword(),
					),
				)

				errs := ExpectCheckerErrors(t, err, 2)
				assert.IsType(t, &sema.NotDeclaredError{}, errs[0])

				assert.IsType(t, &sema.MissingInitializerError{}, errs[1])
			}
		})
	}
}

func TestCheckInvalidCompositeInitializerParameterType(t *testing.T) {

	t.Parallel()

	for _, kind := range common.CompositeKindsWithBody {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %s Test {
                          init(x: X) {}
                      }
                    `,
					kind.Keyword(),
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
		})
	}
}

func TestCheckInvalidCompositeInitializerParameters(t *testing.T) {

	t.Parallel()

	for _, kind := range common.CompositeKindsWithBody {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %s Test {
                          init(x: Int, x: Int) {}
                      }
                    `,
					kind.Keyword(),
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.RedeclarationError{}, errs[0])
		})
	}
}

func TestCheckInvalidCompositeSpecialFunction(t *testing.T) {

	t.Parallel()

	for _, kind := range common.CompositeKindsWithBody {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %s Test {
                          init() { X }
                          destroy() { Y }
                      }
                    `,
					kind.Keyword(),
				),
			)

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindContract:
				errs := ExpectCheckerErrors(t, err, 2)

				assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
				assert.IsType(t, &sema.InvalidDestructorError{}, errs[1])

			case common.CompositeKindResource:

				errs := ExpectCheckerErrors(t, err, 2)

				assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
				assert.IsType(t, &sema.NotDeclaredError{}, errs[1])

			default:
				panic(errors.NewUnreachableError())
			}
		})
	}
}

func TestCheckInvalidCompositeFunction(t *testing.T) {

	t.Parallel()

	for _, kind := range common.CompositeKindsWithBody {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %s Test {
                          fun test() { X }
                      }
                    `,
					kind.Keyword(),
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
		})
	}
}

func TestCheckCompositeInitializerSelfUse(t *testing.T) {

	t.Parallel()

	for _, kind := range common.CompositeKindsWithBody {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %s Test {
                          init() { self }
                          destroy() { self }
                      }
                    `,
					kind.Keyword(),
				),
			)

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindContract:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.InvalidDestructorError{}, errs[0])

			case common.CompositeKindResource:
				errs := ExpectCheckerErrors(t, err, 2)

				// TODO: handle `self` properly

				assert.IsType(t, &sema.ResourceLossError{}, errs[0])
				assert.IsType(t, &sema.ResourceLossError{}, errs[1])

			default:
				panic(errors.NewUnreachableError())
			}
		})
	}
}

func TestCheckCompositeFunctionSelfUse(t *testing.T) {

	t.Parallel()

	for _, kind := range common.CompositeKindsWithBody {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %s Test {
                          fun test() { self }
                      }
                    `,
					kind.Keyword(),
				),
			)

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindContract:
				require.NoError(t, err)

			case common.CompositeKindResource:
				errs := ExpectCheckerErrors(t, err, 1)

				// TODO: handle `self` properly

				assert.IsType(t, &sema.ResourceLossError{}, errs[0])

			default:
				panic(errors.NewUnreachableError())

			}
		})
	}
}

func TestCheckInvalidCompositeMissingInitializer(t *testing.T) {

	t.Parallel()

	for _, kind := range common.CompositeKindsWithBody {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                       %s Test {
                           let foo: Int
                       }
                    `,
					kind.Keyword(),
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.MissingInitializerError{}, errs[0])
		})
	}
}

func TestCheckInvalidResourceMissingDestructor(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
       resource Test {
           let test: @Test
           init(test: @Test) {
               self.test <- test
           }
       }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.MissingDestructorError{}, errs[0])
}

func TestCheckResourceWithDestructor(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
       resource Test {
           let test: @Test

           init(test: @Test) {
               self.test <- test
           }

           destroy() {
               destroy self.test
           }
       }
    `)

	require.NoError(t, err)
}

func TestCheckInvalidResourceFieldWithMissingResourceAnnotation(t *testing.T) {

	t.Parallel()

	interfacePossibilities := []bool{true, false}

	for _, isInterface := range interfacePossibilities {

		interfaceKeyword := ""
		if isInterface {
			interfaceKeyword = "interface"
		}

		t.Run(interfaceKeyword, func(t *testing.T) {

			initializerBody := ""
			if !isInterface {
				initializerBody = `
                  {
                    self.test <- test
                  }
                `
			}

			destructorBody := ""
			if !isInterface {
				destructorBody = `
                  {
                      destroy self.test
                  }
                `
			}

			annotationType := "Test"
			if isInterface {
				annotationType = "AnyResource{Test}"
			}

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      resource %[1]s Test {
                          let test: %[2]s

                          init(test: @%[2]s) %[3]s

                          destroy() %[4]s
                      }
                    `,
					interfaceKeyword,
					annotationType,
					initializerBody,
					destructorBody,
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.MissingResourceAnnotationError{}, errs[0])
		})
	}
}

func TestCheckCompositeFieldAccess(t *testing.T) {

	t.Parallel()

	for _, kind := range common.CompositeKindsWithBody {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %s Test {
                          let foo: Int

                          init() {
                              self.foo = 1
                          }

                          fun test() {
                              self.foo
                          }
                      }
                    `,
					kind.Keyword(),
				),
			)

			require.NoError(t, err)
		})
	}
}

func TestCheckInvalidCompositeFieldAccess(t *testing.T) {

	t.Parallel()

	for _, kind := range common.CompositeKindsWithBody {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %s Test {
                          init() {
                              self.foo
                          }

                          fun test() {
                              self.bar
                          }
                      }
                    `,
					kind.Keyword(),
				),
			)

			expectedErrorCount := 2

			errs := ExpectCheckerErrors(t, err, expectedErrorCount)

			require.IsType(t,
				&sema.NotDeclaredMemberError{},
				errs[0],
			)
			assert.Equal(t,
				"foo",
				errs[0].(*sema.NotDeclaredMemberError).Name,
			)

			require.IsType(t,
				&sema.NotDeclaredMemberError{},
				errs[1],
			)
			assert.Equal(t,
				"bar",
				errs[1].(*sema.NotDeclaredMemberError).Name,
			)
		})
	}
}

func TestCheckCompositeFieldAssignment(t *testing.T) {

	t.Parallel()

	for _, kind := range common.CompositeKindsWithBody {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s Test {
                          var foo: Int

                          init() {
                              self.foo = 1
                              let alsoSelf %[2]s self
                              alsoSelf.foo = 2
                          }

                          fun test() {
                              self.foo = 3
                              let alsoSelf %[2]s self
                              alsoSelf.foo = 4
                          }
                      }
                    `,
					kind.Keyword(),
					kind.TransferOperator(),
				),
			)

			switch kind {
			case common.CompositeKindStructure:
				require.NoError(t, err)

			case common.CompositeKindContract:
				errs := ExpectCheckerErrors(t, err, 2)

				assert.IsType(t, &sema.InvalidMoveError{}, errs[0])
				assert.IsType(t, &sema.InvalidMoveError{}, errs[1])

			case common.CompositeKindResource:

				errs := ExpectCheckerErrors(t, err, 4)

				assert.IsType(t, &sema.InvalidSelfInvalidationError{}, errs[0])
				assert.IsType(t, &sema.ResourceLossError{}, errs[1])
				assert.IsType(t, &sema.InvalidSelfInvalidationError{}, errs[2])
				assert.IsType(t, &sema.ResourceLossError{}, errs[3])

			default:
				panic(errors.NewUnreachableError())
			}
		})
	}
}

func TestCheckInvalidCompositeSelfAssignment(t *testing.T) {

	t.Parallel()

	tests := map[common.CompositeKind]func(error){
		common.CompositeKindStructure: func(err error) {
			errs := ExpectCheckerErrors(t, err, 2)

			assert.IsType(t, &sema.AssignmentToConstantError{}, errs[0])
			assert.IsType(t, &sema.AssignmentToConstantError{}, errs[1])
		},
		common.CompositeKindResource: func(err error) {
			errs := ExpectCheckerErrors(t, err, 4)

			assert.IsType(t, &sema.AssignmentToConstantError{}, errs[0])
			assert.IsType(t, &sema.InvalidResourceAssignmentError{}, errs[1])
			assert.IsType(t, &sema.AssignmentToConstantError{}, errs[2])
			assert.IsType(t, &sema.InvalidResourceAssignmentError{}, errs[3])
		},
		common.CompositeKindContract: func(err error) {
			errs := ExpectCheckerErrors(t, err, 4)

			assert.IsType(t, &sema.AssignmentToConstantError{}, errs[0])
			assert.IsType(t, &sema.InvalidMoveError{}, errs[1])
			assert.IsType(t, &sema.AssignmentToConstantError{}, errs[2])
			assert.IsType(t, &sema.InvalidMoveError{}, errs[3])
		},
	}

	for compositeKind, check := range tests {

		t.Run(compositeKind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s Test {
                          init() {
                              self %[2]s %[3]s Test%[4]s
                          }

                          fun test() {
                              self %[2]s %[3]s Test%[4]s
                          }
                      }
                    `,
					compositeKind.Keyword(),
					compositeKind.TransferOperator(),
					compositeKind.ConstructionKeyword(),
					constructorArguments(compositeKind),
				),
			)

			check(err)
		})
	}
}

func TestCheckInvalidCompositeFieldAssignment(t *testing.T) {

	t.Parallel()

	for _, kind := range common.CompositeKindsWithBody {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %s Test {
                          init() {
                              self.foo = 1
                          }

                          fun test() {
                              self.bar = 2
                          }
                      }
                    `,
					kind.Keyword(),
				),
			)

			errs := ExpectCheckerErrors(t, err, 2)

			require.IsType(t, &sema.NotDeclaredMemberError{}, errs[0])
			assert.Equal(t,
				"foo",
				errs[0].(*sema.NotDeclaredMemberError).Name,
			)

			require.IsType(t, &sema.NotDeclaredMemberError{}, errs[1])
			assert.Equal(t,
				"bar",
				errs[1].(*sema.NotDeclaredMemberError).Name,
			)
		})
	}
}

func TestCheckInvalidCompositeFieldAssignmentWrongType(t *testing.T) {

	t.Parallel()

	for _, kind := range common.CompositeKindsWithBody {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %s Test {
                          var foo: Int

                          init() {
                              self.foo = true
                          }

                          fun test() {
                              self.foo = false
                          }
                      }
                    `,
					kind.Keyword(),
				),
			)

			errs := ExpectCheckerErrors(t, err, 2)

			assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
			assert.IsType(t, &sema.TypeMismatchError{}, errs[1])
		})
	}
}

func TestCheckInvalidCompositeFieldConstantAssignment(t *testing.T) {

	t.Parallel()

	for _, kind := range common.CompositeKindsWithBody {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %s Test {
                          let foo: Int

                          init() {
                              // initialization is fine
                              self.foo = 1
                          }

                          fun test() {
                              // assignment is invalid
                              self.foo = 2
                          }
                      }
                    `,
					kind.Keyword(),
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.AssignmentToConstantMemberError{}, errs[0])
		})
	}
}

func TestCheckCompositeFunctionCall(t *testing.T) {

	t.Parallel()

	for _, kind := range common.CompositeKindsWithBody {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %s Test {
                          fun foo() {}

                          fun bar() {
                              self.foo()
                          }
                      }
                    `,
					kind.Keyword(),
				),
			)

			require.NoError(t, err)
		})
	}
}

func TestCheckInvalidCompositeFunctionCall(t *testing.T) {

	t.Parallel()

	for _, kind := range common.CompositeKindsWithBody {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %s Test {
                          fun foo() {}

                          fun bar() {
                              self.baz()
                          }
                      }
                    `,
					kind.Keyword(),
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.NotDeclaredMemberError{}, errs[0])
		})
	}
}

func TestCheckInvalidCompositeFunctionAssignment(t *testing.T) {

	t.Parallel()

	for _, kind := range common.CompositeKindsWithBody {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %s Test {
                          fun foo() {}

                          fun bar() {
                              self.foo = 2
                          }
                      }
                    `,
					kind.Keyword(),
				),
			)

			errs := ExpectCheckerErrors(t, err, 2)

			assert.IsType(t, &sema.TypeMismatchError{}, errs[0])

			require.IsType(t, &sema.AssignmentToConstantMemberError{}, errs[1])
			assert.Equal(t,
				"foo",
				errs[1].(*sema.AssignmentToConstantMemberError).Name,
			)
		})
	}
}

func TestCheckCompositeInstantiation(t *testing.T) {

	t.Parallel()

	for _, compositeKind := range common.CompositeKindsWithBody {

		if compositeKind == common.CompositeKindContract {
			// Contracts cannot be instantiated
			continue
		}

		t.Run(compositeKind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s Test {

                          init(x: Int) {
                              let test: %[2]sTest %[3]s %[4]s Test(x: 1)
                              %[5]s test
                          }

                          fun test() {
                              let test: %[2]sTest %[3]s %[4]s Test(x: 2)
                              %[5]s test
                          }
                      }

                      let test: %[2]sTest %[3]s %[4]s Test(x: 3)
                    `,
					compositeKind.Keyword(),
					compositeKind.Annotation(),
					compositeKind.TransferOperator(),
					compositeKind.ConstructionKeyword(),
					compositeKind.DestructionKeyword(),
				),
			)

			require.NoError(t, err)
		})
	}
}

func TestCheckInvalidSameCompositeRedeclaration(t *testing.T) {

	t.Parallel()

	for _, kind := range common.AllCompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			body := "{}"
			if kind == common.CompositeKindEvent {
				body = "()"
			}

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      let x = 1
                      %[1]s Foo %[2]s
                      %[1]s Foo %[2]s
                    `,
					kind.Keyword(),
					body,
				),
			)

			errs := ExpectCheckerErrors(t, err, 2)

			// NOTE: two errors: one because type is redeclared,
			// the other because the global is redeclared

			assert.IsType(t, &sema.RedeclarationError{}, errs[0])
			assert.IsType(t, &sema.RedeclarationError{}, errs[1])
		})
	}
}

func TestCheckInvalidDifferentCompositeRedeclaration(t *testing.T) {

	t.Parallel()

	for _, firstKind := range common.AllCompositeKinds {
		for _, secondKind := range common.AllCompositeKinds {

			// only check different kinds
			if firstKind == secondKind {
				continue
			}

			firstBody := "{}"
			if firstKind == common.CompositeKindEvent {
				firstBody = "()"
			}

			secondBody := "{}"
			if secondKind == common.CompositeKindEvent {
				secondBody = "()"
			}

			testName := fmt.Sprintf(
				"%s/%s",
				firstKind.Keyword(),
				secondKind.Keyword(),
			)

			t.Run(testName, func(t *testing.T) {

				_, err := ParseAndCheck(t,
					fmt.Sprintf(
						`
                          let x = 1
                          %[1]s Foo %[2]s
                          %[3]s Foo %[4]s
                        `,
						firstKind.Keyword(),
						firstBody,
						secondKind.Keyword(),
						secondBody,
					),
				)

				errs := ExpectCheckerErrors(t, err, 2)

				// NOTE: two errors: one because type is redeclared,
				// the other because the global is redeclared

				assert.IsType(t, &sema.RedeclarationError{}, errs[0])
				assert.IsType(t, &sema.RedeclarationError{}, errs[1])
			})
		}
	}
}

func TestCheckInvalidForwardReference(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      let x = y
      let y = x
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
}

func TestCheckInvalidIncompatibleSameCompositeTypes(t *testing.T) {

	t.Parallel()

	// tests that composite typing is nominal, not structural,
	// and composite kind is considered

	for _, firstKind := range common.CompositeKindsWithBody {
		for _, secondKind := range common.CompositeKindsWithBody {

			if firstKind == common.CompositeKindContract ||
				secondKind == common.CompositeKindContract {

				continue
			}

			testName := fmt.Sprintf(
				"%s/%s",
				firstKind.Keyword(),
				secondKind.Keyword(),
			)

			t.Run(testName, func(t *testing.T) {

				_, err := ParseAndCheck(t,
					fmt.Sprintf(
						`
                          %[1]s Foo {
                              init() {}
                          }

                          %[2]s Bar {
                              init() {}
                          }

                          let foo: %[3]sFoo %[4]s %[5]s Bar%[6]s
                        `,
						firstKind.Keyword(),
						secondKind.Keyword(),
						firstKind.Annotation(),
						firstKind.TransferOperator(),
						secondKind.ConstructionKeyword(),
						constructorArguments(secondKind),
					),
				)

				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
			})
		}
	}
}

func TestCheckInvalidCompositeFunctionWithSelfParameter(t *testing.T) {

	t.Parallel()

	for _, kind := range common.CompositeKindsWithBody {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %s Foo {
                          fun test(self: Int) {}
                      }
                    `,
					kind.Keyword(),
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.RedeclarationError{}, errs[0])
		})
	}
}

func TestCheckInvalidCompositeInitializerWithSelfParameter(t *testing.T) {

	t.Parallel()

	for _, kind := range common.CompositeKindsWithBody {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %s Foo {
                          init(self: Int) {}
                      }
                    `,
					kind.Keyword(),
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.RedeclarationError{}, errs[0])
		})
	}
}

func TestCheckCompositeInitializesConstant(t *testing.T) {

	t.Parallel()

	for _, compositeKind := range common.CompositeKindsWithBody {

		var setupCode string

		if compositeKind != common.CompositeKindContract {
			setupCode = fmt.Sprintf(
				`let test %[1]s %[2]s Test%[3]s`,
				compositeKind.TransferOperator(),
				compositeKind.ConstructionKeyword(),
				constructorArguments(compositeKind),
			)
		}

		t.Run(compositeKind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s Test {
                          let foo: Int

                          init() {
                              self.foo = 42
                          }
                      }

                      %[2]s
                    `,
					compositeKind.Keyword(),
					setupCode,
				),
			)

			require.NoError(t, err)
		})
	}
}

func TestCheckCompositeInitializerWithArgumentLabel(t *testing.T) {

	t.Parallel()

	for _, compositeKind := range common.CompositeKindsWithBody {

		if compositeKind == common.CompositeKindContract {
			// Contracts cannot be instantiated
			continue
		}

		t.Run(compositeKind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s Test {

                          init(x: Int) {}
                      }

                      let test %[2]s %[3]s Test(x: 1)
                    `,
					compositeKind.Keyword(),
					compositeKind.TransferOperator(),
					compositeKind.ConstructionKeyword(),
				),
			)

			require.NoError(t, err)
		})
	}
}

func TestCheckInvalidCompositeInitializerCallWithMissingArgumentLabel(t *testing.T) {

	t.Parallel()

	for _, compositeKind := range common.CompositeKindsWithBody {

		if compositeKind == common.CompositeKindContract {
			// Contracts cannot be instantiated
			continue
		}

		t.Run(compositeKind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s Test {

                          init(x: Int) {}
                      }

                      let test %[2]s %[3]s Test(1)
                    `,
					compositeKind.Keyword(),
					compositeKind.TransferOperator(),
					compositeKind.ConstructionKeyword(),
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.MissingArgumentLabelError{}, errs[0])
		})
	}
}

func TestCheckCompositeFunctionWithArgumentLabel(t *testing.T) {

	t.Parallel()

	for _, compositeKind := range common.CompositeKindsWithBody {

		var setupCode, identifier string

		if compositeKind == common.CompositeKindContract {
			identifier = "Test"
		} else {
			setupCode = fmt.Sprintf(
				`let test %[1]s %[2]s Test%[3]s`,
				compositeKind.TransferOperator(),
				compositeKind.ConstructionKeyword(),
				constructorArguments(compositeKind),
			)
			identifier = "test"
		}

		t.Run(compositeKind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s Test {

                          fun test(x: Int) {}
                      }

                      %[2]s
                      let void = %[3]s.test(x: 1)
                    `,
					compositeKind.Keyword(),
					setupCode,
					identifier,
				),
			)

			require.NoError(t, err)
		})
	}
}

func TestCheckInvalidCompositeFunctionCallWithMissingArgumentLabel(t *testing.T) {

	t.Parallel()

	for _, compositeKind := range common.CompositeKindsWithBody {

		var setupCode, identifier string

		if compositeKind == common.CompositeKindContract {
			identifier = "Test"
		} else {
			setupCode = fmt.Sprintf(
				`let test %[1]s %[2]s Test%[3]s`,
				compositeKind.TransferOperator(),
				compositeKind.ConstructionKeyword(),
				constructorArguments(compositeKind),
			)
			identifier = "test"
		}

		t.Run(compositeKind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s Test {

                          fun test(x: Int) {}
                      }

                      %[2]s
                      let void = %[3]s.test(1)
                    `,
					compositeKind.Keyword(),
					setupCode,
					identifier,
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.MissingArgumentLabelError{}, errs[0])
		})
	}
}

func TestCheckCompositeConstructorUseInInitializerAndFunction(t *testing.T) {

	t.Parallel()

	for _, compositeKind := range common.CompositeKindsWithBody {

		if compositeKind == common.CompositeKindContract {
			continue
		}

		t.Run(compositeKind.Keyword(), func(t *testing.T) {

			checker, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s Test {

                          init() {
                              Test
                          }

                          fun test(): %[2]sTest {
                              return %[4]s%[5]s Test%[7]s
                          }
                      }

                      fun test(): %[2]sTest {
                          return %[4]s%[5]s Test%[7]s
                      }

                      fun test2(): %[2]sTest {
                          let test %[3]s %[5]s Test%[7]s
                          let res %[3]s test.test()
                          %[6]s test
                          return %[4]sres
                      }
                    `,
					compositeKind.Keyword(),
					compositeKind.Annotation(),
					compositeKind.TransferOperator(),
					compositeKind.MoveOperator(),
					compositeKind.ConstructionKeyword(),
					compositeKind.DestructionKeyword(),
					constructorArguments(compositeKind),
				),
			)

			require.NoError(t, err)

			testType := checker.FindType("Test")

			assert.IsType(t, &sema.CompositeType{}, testType)

			structureType := testType.(*sema.CompositeType)

			assert.Equal(t,
				"Test",
				structureType.Identifier,
			)

			testFunctionMember := structureType.Members["test"]

			assert.IsType(t, &sema.FunctionType{}, testFunctionMember.TypeAnnotation.Type)

			testFunctionType := testFunctionMember.TypeAnnotation.Type.(*sema.FunctionType)

			actual := testFunctionType.ReturnTypeAnnotation.Type
			if actual != structureType {
				assert.Fail(t, "not structureType", actual)
			}
		})
	}
}

func TestCheckInvalidCompositeFieldMissingVariableKind(t *testing.T) {

	t.Parallel()

	for _, kind := range common.CompositeKindsWithBody {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %s X {
                          x: Int

                          init(x: Int) {
                              self.x = x
                          }
                      }
                    `,
					kind.Keyword(),
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.InvalidVariableKindError{}, errs[0])
		})
	}
}

func TestCheckCompositeFunction(t *testing.T) {

	t.Parallel()

	for _, kind := range common.CompositeKindsWithBody {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s X {
                          fun foo(): %[2]sX {
                              return %[3]s self.bar()
                          }

                          fun bar(): %[2]sX {
                              return %[3]s self
                          }
                      }
                    `,
					kind.Keyword(),
					kind.Annotation(),
					kind.MoveOperator(),
				),
			)

			switch kind {
			case common.CompositeKindStructure:
				require.NoError(t, err)

			case common.CompositeKindContract:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.InvalidMoveError{}, errs[0])

			case common.CompositeKindResource:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.InvalidSelfInvalidationError{}, errs[0])

			default:
				panic(errors.NewUnreachableError())
			}
		})
	}
}

func TestCheckCompositeReferenceBeforeDeclaration(t *testing.T) {

	t.Parallel()

	for _, compositeKind := range common.CompositeKindsWithBody {

		if compositeKind == common.CompositeKindContract {
			continue
		}

		t.Run(compositeKind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      var tests = 0

                      fun test(): %[1]sTest {
                          return %[2]s %[3]s Test%[4]s
                      }

                      %[5]s Test {
                         init() {
                             tests = tests + 1
                         }
                      }
                    `,
					compositeKind.Annotation(),
					compositeKind.MoveOperator(),
					compositeKind.ConstructionKeyword(),
					constructorArguments(compositeKind),
					compositeKind.Keyword(),
				),
			)

			require.NoError(t, err)
		})
	}
}

func TestCheckInvalidDestructorParameters(t *testing.T) {

	t.Parallel()

	interfacePossibilities := []bool{true, false}

	for _, isInterface := range interfacePossibilities {

		interfaceKeyword := ""
		if isInterface {
			interfaceKeyword = "interface"
		}

		destructorBody := ""
		if !isInterface {
			destructorBody = "{}"
		}

		t.Run(interfaceKeyword, func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      resource %[1]s Test {
                          destroy(x: Int) %[2]s
                      }
                    `,
					interfaceKeyword,
					destructorBody,
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.InvalidDestructorParametersError{}, errs[0])
		})
	}
}

func TestCheckInvalidResourceWithDestructorMissingFieldInvalidation(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
       resource Test {
           let test: @Test

           init(test: @Test) {
               self.test <- test
           }

           destroy() {}
       }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ResourceFieldNotInvalidatedError{}, errs[0])
}

// This tests prevents a potential regression in `checkResourceFieldsInvalidated`:
// See https://github.com/dapperlabs/flow-go/issues/2533
//
// The function contained a bug in which field invalidation was skipped for all remaining members
// once a non-resource member was encountered, instead of just skipping the non-resource member
// and continuing the check for the remaining members.

func TestCheckInvalidResourceWithDestructorMissingFieldInvalidationFirstFieldNonResource(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
       resource Test {
           let a: Int
           let b: @Test

           init(b: @Test) {
               self.a = 1
               self.b <- b
           }

           destroy() {}
       }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ResourceFieldNotInvalidatedError{}, errs[0])
}

func TestCheckInvalidResourceWithDestructorMissingDefinitiveFieldInvalidation(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
       resource Test {
           let test: @Test

           init(test: @Test) {
               self.test <- test
           }

           destroy() {
               if false {
                   destroy self.test
               }
           }
       }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ResourceFieldNotInvalidatedError{}, errs[0])
}

func TestCheckResourceWithDestructorAndStructField(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
       struct S {}

       resource Test {
           let s: S

           init(s: S) {
               self.s = s
           }

           destroy() {}
       }
    `)

	require.NoError(t, err)
}

func TestCheckInvalidResourceDestructorMoveInvalidation(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
       resource Test {
           let test: @Test

           init(test: @Test) {
               self.test <- test
           }

           destroy() {
               absorb(<-self.test)
               absorb(<-self.test)
           }
       }

       fun absorb(_ test: @Test) {
           destroy test
       }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[0])
}

func TestCheckInvalidResourceDestructorRepeatedDestruction(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
       resource Test {
           let test: @Test

           init(test: @Test) {
               self.test <- test
           }

           destroy() {
               destroy self.test
               destroy self.test
           }
       }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[0])
}

func TestCheckInvalidResourceDestructorCapturing(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
       var duplicate: ((): @Test)? = nil

       resource Test {
           let test: @Test

           init(test: @Test) {
               self.test <- test
           }

           destroy() {
               duplicate = fun (): @Test {
                   return <-self.test
               }
           }
       }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ResourceCapturingError{}, errs[0])
}

func TestCheckInvalidStructureFunctionWithMissingBody(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
        struct Test {
            pub fun getFoo(): Int
        }
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.MissingFunctionBodyError{}, errs[0])
}

func TestCheckInvalidStructureInitializerWithMissingBody(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
        struct Test {
            init()
        }
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.MissingFunctionBodyError{}, errs[0])
}

func TestCheckMutualTypeUseTopLevel(t *testing.T) {

	t.Parallel()

	interfacePossibilities := []bool{true, false}

	for _, firstKind := range common.CompositeKindsWithBody {
		for _, firstIsInterface := range interfacePossibilities {
			for _, secondKind := range common.CompositeKindsWithBody {
				for _, secondIsInterface := range interfacePossibilities {

					firstInterfaceKeyword := ""
					firstTypeAnnotation := "A"
					if firstIsInterface {
						firstInterfaceKeyword = "interface"
						firstTypeAnnotation = AsInterfaceType("A", firstKind)
					}

					secondInterfaceKeyword := ""
					secondTypeAnnotation := "B"
					if secondIsInterface {
						secondInterfaceKeyword = "interface"
						secondTypeAnnotation = AsInterfaceType("B", secondKind)
					}

					testName := fmt.Sprintf(
						"%s_%s/%s_%s",
						firstKind.Keyword(),
						firstInterfaceKeyword,
						secondKind.Keyword(),
						secondInterfaceKeyword,
					)

					firstBody := ""
					if !firstIsInterface {
						firstBody = fmt.Sprintf(
							"{ %s b }",
							secondKind.DestructionKeyword(),
						)
					}

					secondBody := ""
					if !secondIsInterface {
						secondBody = fmt.Sprintf(
							"{ %s a }",
							firstKind.DestructionKeyword(),
						)
					}

					t.Run(testName, func(t *testing.T) {

						code := fmt.Sprintf(
							`
                              %[1]s %[2]s A {
                                  fun use(_ b: %[3]s%[4]s) %[5]s
                              }

                              %[6]s %[7]s B {
                                  fun use(_ a: %[8]s%[9]s) %[10]s
                              }
                            `,
							firstKind.Keyword(),
							firstInterfaceKeyword,
							secondKind.Annotation(),
							secondTypeAnnotation,
							firstBody,
							secondKind.Keyword(),
							secondInterfaceKeyword,
							firstKind.Annotation(),
							firstTypeAnnotation,
							secondBody,
						)

						_, err := ParseAndCheck(t, code)

						if !assert.NoError(t, err) {
							cmd.PrettyPrintError(err, "", map[string]string{"": code})
						}
					})
				}
			}
		}
	}
}
