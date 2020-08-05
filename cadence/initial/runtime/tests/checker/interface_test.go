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
	"github.com/onflow/cadence/runtime/tests/examples"
	. "github.com/onflow/cadence/runtime/tests/utils"
)

func constructorArguments(compositeKind common.CompositeKind) string {
	if compositeKind == common.CompositeKindContract {
		return ""
	}
	return "()"
}

func TestCheckInvalidLocalInterface(t *testing.T) {

	t.Parallel()

	for _, kind := range common.AllCompositeKinds {

		if !kind.SupportsInterfaces() {
			continue
		}

		t.Run(kind.Keyword(), func(t *testing.T) {

			body := "{}"
			if kind == common.CompositeKindEvent {
				body = "()"
			}

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      fun test() {
                          %[1]s interface Test %[2]s
                      }
                    `,
					kind.Keyword(),
					body,
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.InvalidDeclarationError{}, errs[0])
		})
	}
}

func TestCheckInterfaceWithFunction(t *testing.T) {

	t.Parallel()

	for _, kind := range common.CompositeKindsWithBody {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %s interface Test {
                          fun test()
                      }
                    `,
					kind.Keyword(),
				),
			)

			require.NoError(t, err)

		})
	}
}

func TestCheckInterfaceWithFunctionImplementationAndConditions(t *testing.T) {

	t.Parallel()

	for _, kind := range common.CompositeKindsWithBody {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %s interface Test {
                          fun test(x: Int) {
                              pre {
                                x == 0
                              }
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

func TestCheckInvalidInterfaceWithFunctionImplementation(t *testing.T) {

	t.Parallel()

	for _, kind := range common.CompositeKindsWithBody {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %s interface Test {
                          fun test(): Int {
                             return 1
                          }
                      }
                    `,
					kind.Keyword(),
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.InvalidImplementationError{}, errs[0])
		})
	}
}

func TestCheckInvalidInterfaceWithFunctionImplementationNoConditions(t *testing.T) {

	t.Parallel()

	for _, kind := range common.CompositeKindsWithBody {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %s interface Test {
                          fun test() {
                            // ...
                          }
                      }
                    `,
					kind.Keyword(),
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.InvalidImplementationError{}, errs[0])
		})
	}
}

func TestCheckInterfaceWithInitializer(t *testing.T) {

	t.Parallel()

	for _, kind := range common.CompositeKindsWithBody {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %s interface Test {
                          init()
                      }
                    `,
					kind.Keyword(),
				),
			)

			require.NoError(t, err)
		})
	}
}

func TestCheckInvalidInterfaceWithInitializerImplementation(t *testing.T) {

	t.Parallel()

	for _, kind := range common.CompositeKindsWithBody {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %s interface Test {
                          init() {
                            // ...
                          }
                      }
                    `,
					kind.Keyword(),
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.InvalidImplementationError{}, errs[0])
		})
	}
}

func TestCheckInterfaceWithInitializerImplementationAndConditions(t *testing.T) {

	t.Parallel()

	for _, kind := range common.CompositeKindsWithBody {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %s interface Test {
                          init(x: Int) {
                              pre {
                                x == 0
                              }
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

func TestCheckInterfaceUse(t *testing.T) {

	t.Parallel()

	for _, kind := range common.AllCompositeKinds {

		if !kind.SupportsInterfaces() {
			continue
		}

		body := "{}"
		if kind == common.CompositeKindEvent {
			body = "()"
		}

		annotationType := AsInterfaceType("Test", kind)

		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheckWithPanic(t,
				fmt.Sprintf(
					`
                      pub %[1]s interface Test %[2]s

                      pub let test: %[3]s%[4]s %[5]s panic("")
                    `,
					kind.Keyword(),
					body,
					kind.Annotation(),
					annotationType,
					kind.TransferOperator(),
				),
			)

			require.NoError(t, err)
		})
	}
}

func TestCheckInterfaceConformanceNoRequirements(t *testing.T) {

	t.Parallel()

	for _, compositeKind := range common.AllCompositeKinds {

		if !compositeKind.SupportsInterfaces() {
			continue
		}

		body := "{}"
		if compositeKind == common.CompositeKindEvent {
			body = "()"
		}

		annotationType := AsInterfaceType("Test", compositeKind)

		var useCode string
		if compositeKind != common.CompositeKindContract {
			useCode = fmt.Sprintf(
				`let test: %[1]s%[2]s %[3]s %[4]s TestImpl%[5]s`,
				compositeKind.Annotation(),
				annotationType,
				compositeKind.TransferOperator(),
				compositeKind.ConstructionKeyword(),
				constructorArguments(compositeKind),
			)
		}

		t.Run(compositeKind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s interface Test %[2]s

                      %[1]s TestImpl: Test %[2]s

                      %[3]s
                    `,
					compositeKind.Keyword(),
					body,
					useCode,
				))

			require.NoError(t, err)
		})
	}
}

func TestCheckInvalidInterfaceConformanceIncompatibleCompositeKinds(t *testing.T) {

	t.Parallel()

	for _, firstKind := range common.AllCompositeKinds {

		if !firstKind.SupportsInterfaces() {
			continue
		}

		for _, secondKind := range common.AllCompositeKinds {

			if !secondKind.SupportsInterfaces() {
				continue
			}

			// only test incompatible combinations
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

			firstKindInterfaceType := AsInterfaceType("Test", firstKind)

			// NOTE: type mismatch is only tested when both kinds are not contracts
			// (which can not be passed by value)

			var useCode string
			if firstKind != common.CompositeKindContract &&
				secondKind != common.CompositeKindContract {

				useCode = fmt.Sprintf(
					`let test: %[1]s%[2]s %[3]s %[4]s TestImpl%[5]s`,
					firstKind.Annotation(),
					firstKindInterfaceType,
					firstKind.TransferOperator(),
					secondKind.ConstructionKeyword(),
					constructorArguments(secondKind),
				)
			}

			testName := fmt.Sprintf(
				"%s/%s",
				firstKind.Keyword(),
				secondKind.Keyword(),
			)

			t.Run(testName, func(t *testing.T) {

				code := fmt.Sprintf(
					`
                      %[1]s interface Test %[2]s

                      %[3]s TestImpl: Test %[4]s

                      %[5]s
                    `,
					firstKind.Keyword(),
					firstBody,
					secondKind.Keyword(),
					secondBody,
					useCode,
				)

				checker, err := ParseAndCheck(t, code)

				// NOTE: type mismatch is only tested when both kinds are not contracts
				// (which can not be passed by value)

				if firstKind != common.CompositeKindContract &&
					secondKind != common.CompositeKindContract {

					errs := ExpectCheckerErrors(t, err, 2)

					assert.IsType(t, &sema.CompositeKindMismatchError{}, errs[0])
					assert.IsType(t, &sema.TypeMismatchError{}, errs[1])

				} else {
					errs := ExpectCheckerErrors(t, err, 1)

					assert.IsType(t, &sema.CompositeKindMismatchError{}, errs[0])
				}

				require.NotNil(t, checker)

				testType := checker.GlobalTypes["Test"].Type
				testImplType := checker.GlobalTypes["TestImpl"].Type

				assert.False(t, sema.IsSubType(testImplType, testType))
			})
		}
	}
}

func TestCheckInvalidInterfaceConformanceUndeclared(t *testing.T) {

	t.Parallel()

	for _, compositeKind := range common.AllCompositeKinds {

		if !compositeKind.SupportsInterfaces() {
			continue
		}

		interfaceType := AsInterfaceType("Test", compositeKind)

		var useCode string
		if compositeKind != common.CompositeKindContract {
			useCode = fmt.Sprintf(
				`let test: %[1]s%[2]s %[3]s %[4]s TestImpl%[5]s`,
				compositeKind.Annotation(),
				interfaceType,
				compositeKind.TransferOperator(),
				compositeKind.ConstructionKeyword(),
				constructorArguments(compositeKind),
			)
		}

		body := "{}"
		if compositeKind == common.CompositeKindEvent {
			body = "()"
		}

		t.Run(compositeKind.Keyword(), func(t *testing.T) {

			checker, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s interface Test %[2]s

                      // NOTE: not declaring conformance
                      %[1]s TestImpl %[2]s

                      %[3]s
                    `,
					compositeKind.Keyword(),
					body,
					useCode,
				),
			)

			if compositeKind != common.CompositeKindContract {
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
			} else {
				require.NoError(t, err)
			}

			require.NotNil(t, checker)

			testType := checker.GlobalTypes["Test"].Type
			testImplType := checker.GlobalTypes["TestImpl"].Type

			assert.False(t, sema.IsSubType(testImplType, testType))
		})
	}
}

func TestCheckInvalidCompositeInterfaceConformanceNonInterface(t *testing.T) {

	t.Parallel()

	for _, kind := range common.AllCompositeKinds {

		if !kind.SupportsInterfaces() {
			continue
		}

		body := "{}"
		if kind == common.CompositeKindEvent {
			body = "()"
		}

		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s TestImpl: Int %[2]s
                    `,
					kind.Keyword(),
					body,
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.InvalidConformanceError{}, errs[0])
		})
	}
}

func TestCheckInterfaceFieldUse(t *testing.T) {

	t.Parallel()

	for _, compositeKind := range common.CompositeKindsWithBody {

		if compositeKind == common.CompositeKindContract {
			// Contracts cannot be instantiated
			continue
		}

		t.Run(compositeKind.Keyword(), func(t *testing.T) {

			interfaceType := AsInterfaceType("Test", compositeKind)

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s interface Test {
                          x: Int
                      }

                      %[1]s TestImpl: Test {
                          var x: Int

                          init(x: Int) {
                              self.x = x
                          }
                      }

                      let test: %[2]s%[3]s %[4]s %[5]s TestImpl(x: 1)

                      let x = test.x
                    `,
					compositeKind.Keyword(),
					compositeKind.Annotation(),
					interfaceType,
					compositeKind.TransferOperator(),
					compositeKind.ConstructionKeyword(),
				),
			)

			require.NoError(t, err)
		})
	}
}

func TestCheckInvalidInterfaceUndeclaredFieldUse(t *testing.T) {

	t.Parallel()

	for _, compositeKind := range common.CompositeKindsWithBody {

		if compositeKind == common.CompositeKindContract {
			// Contracts cannot be instantiated
			continue
		}

		interfaceType := AsInterfaceType("Test", compositeKind)

		t.Run(compositeKind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s interface Test {}

                      %[1]s TestImpl: Test {
                          var x: Int

                          init(x: Int) {
                              self.x = x
                          }
                      }

                      let test: %[2]s%[3]s %[4]s %[5]s TestImpl(x: 1)

                      let x = test.x
                    `,
					compositeKind.Keyword(),
					compositeKind.Annotation(),
					interfaceType,
					compositeKind.TransferOperator(),
					compositeKind.ConstructionKeyword(),
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.NotDeclaredMemberError{}, errs[0])
		})
	}
}

func TestCheckInterfaceFunctionUse(t *testing.T) {

	t.Parallel()

	for _, compositeKind := range common.CompositeKindsWithBody {

		var setupCode, identifier string
		if compositeKind != common.CompositeKindContract {
			identifier = "test"

			interfaceType := AsInterfaceType("Test", compositeKind)

			setupCode = fmt.Sprintf(
				`let test: %[1]s%[2]s %[3]s %[4]s TestImpl%[5]s`,
				compositeKind.Annotation(),
				interfaceType,
				compositeKind.TransferOperator(),
				compositeKind.ConstructionKeyword(),
				constructorArguments(compositeKind),
			)
		} else {
			identifier = "TestImpl"
		}

		t.Run(compositeKind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s interface Test {
                          fun test(): Int
                      }

                      %[1]s TestImpl: Test {
                          fun test(): Int {
                              return 2
                          }
                      }

                      %[2]s

                      let val = %[3]s.test()
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

func TestCheckInvalidInterfaceUndeclaredFunctionUse(t *testing.T) {

	t.Parallel()

	for _, compositeKind := range common.CompositeKindsWithBody {

		if compositeKind == common.CompositeKindContract {
			continue
		}

		t.Run(compositeKind.Keyword(), func(t *testing.T) {

			interfaceType := AsInterfaceType("Test", compositeKind)

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s interface Test {}

                      %[1]s TestImpl: Test {
                          fun test(): Int {
                              return 2
                          }
                      }

                      let test: %[2]s%[3]s %[4]s %[5]s TestImpl%[6]s

                      let val = test.test()
                    `,
					compositeKind.Keyword(),
					compositeKind.Annotation(),
					interfaceType,
					compositeKind.TransferOperator(),
					compositeKind.ConstructionKeyword(),
					constructorArguments(compositeKind),
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.NotDeclaredMemberError{}, errs[0])
		})
	}
}

func TestCheckInvalidInterfaceConformanceInitializerExplicitMismatch(t *testing.T) {

	t.Parallel()

	for _, kind := range common.CompositeKindsWithBody {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s interface Test {
                          init(x: Int)
                      }

                      %[1]s TestImpl: Test {
                          init(x: Bool) {}
                      }
                    `,
					kind.Keyword(),
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.ConformanceError{}, errs[0])
		})
	}
}

func TestCheckInvalidInterfaceConformanceInitializerImplicitMismatch(t *testing.T) {

	t.Parallel()

	for _, kind := range common.CompositeKindsWithBody {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s interface Test {
                          init(x: Int)
                      }

                      %[1]s TestImpl: Test {
                      }
                    `,
					kind.Keyword(),
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.ConformanceError{}, errs[0])
		})
	}
}

func TestCheckInvalidInterfaceConformanceMissingFunction(t *testing.T) {

	t.Parallel()

	for _, kind := range common.CompositeKindsWithBody {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s interface Test {
                          fun test(): Int
                      }

                      %[1]s TestImpl: Test {}
                    `,
					kind.Keyword(),
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.ConformanceError{}, errs[0])
		})
	}
}

func TestCheckInvalidInterfaceConformanceFunctionMismatch(t *testing.T) {

	t.Parallel()

	for _, kind := range common.CompositeKindsWithBody {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s interface Test {
                          fun test(): Int
                      }

                      %[1]s TestImpl: Test {
                          fun test(): Bool {
                              return true
                          }
                      }
                    `,
					kind.Keyword(),
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.ConformanceError{}, errs[0])
		})
	}
}

func TestCheckInvalidInterfaceConformanceFunctionPrivateAccessModifier(t *testing.T) {

	t.Parallel()

	for _, kind := range common.CompositeKindsWithBody {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s interface Test {
                          fun test(): Int
                      }

                      %[1]s TestImpl: Test {
                          priv fun test(): Int {
                              return 1
                          }
                      }
                    `,
					kind.Keyword(),
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.ConformanceError{}, errs[0])
		})
	}
}

func TestCheckInvalidInterfaceConformanceMissingField(t *testing.T) {

	t.Parallel()

	for _, kind := range common.AllCompositeKinds {

		if !kind.SupportsInterfaces() {
			continue
		}

		var interfaceBody string
		if kind == common.CompositeKindEvent {
			interfaceBody = "(x: Int)"
		} else {
			interfaceBody = "{ x: Int }"
		}

		var conformanceBody string
		if kind == common.CompositeKindEvent {
			conformanceBody = "()"
		} else {
			conformanceBody = "{}"
		}

		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s interface Test %[2]s

                      %[1]s TestImpl: Test %[3]s

                    `,
					kind.Keyword(),
					interfaceBody,
					conformanceBody,
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.ConformanceError{}, errs[0])
		})
	}
}

func TestCheckInvalidInterfaceConformanceFieldTypeMismatch(t *testing.T) {

	t.Parallel()

	for _, kind := range common.CompositeKindsWithBody {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s interface Test {
                          x: Int
                      }

                      %[1]s TestImpl: Test {
                          var x: Bool
                          init(x: Bool) {
                             self.x = x
                          }
                      }
                    `,
					kind.Keyword(),
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.ConformanceError{}, errs[0])
		})
	}
}

func TestCheckInvalidInterfaceConformanceFieldPrivateAccessModifier(t *testing.T) {

	t.Parallel()

	for _, kind := range common.CompositeKindsWithBody {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s interface Test {
                          x: Int
                      }

                      %[1]s TestImpl: Test {
                          priv var x: Int

                          init(x: Int) {
                             self.x = x
                          }
                      }
                    `,
					kind.Keyword(),
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.ConformanceError{}, errs[0])
		})
	}
}

func TestCheckInvalidInterfaceConformanceFieldMismatchAccessModifierMoreRestrictive(t *testing.T) {

	t.Parallel()

	for _, kind := range common.CompositeKindsWithBody {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s interface Test {
                          pub(set) x: Int
                      }

                      %[1]s TestImpl: Test {
                          pub var x: Int

                          init(x: Int) {
                             self.x = x
                          }
                      }
                    `,
					kind.Keyword(),
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.ConformanceError{}, errs[0])
		})
	}
}

func TestCheckInvalidInterfaceConformanceFunctionMismatchAccessModifierMoreRestrictive(t *testing.T) {

	t.Parallel()

	for _, kind := range common.CompositeKindsWithBody {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s interface Test {
                          pub fun x()
                      }

                      %[1]s TestImpl: Test {
                          access(account) fun x() {}
                      }
                    `,
					kind.Keyword(),
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.ConformanceError{}, errs[0])
		})
	}
}

func TestCheckInterfaceConformanceFieldMorePermissiveAccessModifier(t *testing.T) {

	t.Parallel()

	for _, kind := range common.CompositeKindsWithBody {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s interface Test {
                          pub x: Int
                      }

                      %[1]s TestImpl: Test {
                          pub(set) var x: Int

                          init(x: Int) {
                             self.x = x
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

func TestCheckInvalidInterfaceConformanceKindFieldFunctionMismatch(t *testing.T) {

	t.Parallel()

	for _, kind := range common.CompositeKindsWithBody {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s interface Test {
                          x: Bool
                      }

                      %[1]s TestImpl: Test {
                          fun x(): Bool {
                              return true
                          }
                      }
                    `,
					kind.Keyword(),
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.ConformanceError{}, errs[0])
		})
	}
}

func TestCheckInvalidInterfaceConformanceKindFunctionFieldMismatch(t *testing.T) {

	t.Parallel()

	for _, kind := range common.CompositeKindsWithBody {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s interface Test {
                          fun x(): Bool
                      }

                      %[1]s TestImpl: Test {
                          var x: Bool

                          init(x: Bool) {
                             self.x = x
                          }
                      }
                    `,
					kind.Keyword(),
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.ConformanceError{}, errs[0])
		})
	}
}

func TestCheckInvalidInterfaceConformanceFieldKindLetVarMismatch(t *testing.T) {

	t.Parallel()

	for _, kind := range common.CompositeKindsWithBody {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s interface Test {
                          let x: Bool
                      }

                      %[1]s TestImpl: Test {
                          var x: Bool

                          init(x: Bool) {
                             self.x = x
                          }
                      }
                    `,
					kind.Keyword(),
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.ConformanceError{}, errs[0])
		})
	}
}

func TestCheckInvalidInterfaceConformanceFieldKindVarLetMismatch(t *testing.T) {

	t.Parallel()

	for _, kind := range common.CompositeKindsWithBody {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s interface Test {
                          var x: Bool
                      }

                      %[1]s TestImpl: Test {
                          let x: Bool

                          init(x: Bool) {
                             self.x = x
                          }
                      }
                    `,
					kind.Keyword(),
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.ConformanceError{}, errs[0])
		})
	}
}

func TestCheckInterfaceConformanceFunctionArgumentLabelMatch(t *testing.T) {

	t.Parallel()

	for _, kind := range common.CompositeKindsWithBody {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s interface Test {
                          fun x(z: Int)
                      }

                      %[1]s TestImpl: Test {
                          fun x(z: Int) {}
                      }
                    `,
					kind.Keyword(),
				),
			)

			require.NoError(t, err)
		})
	}
}

func TestCheckInvalidInterfaceConformanceFunctionArgumentLabelMismatch(t *testing.T) {

	t.Parallel()

	for _, kind := range common.CompositeKindsWithBody {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s interface Test {
                          fun x(y: Int)
                      }

                      %[1]s TestImpl: Test {
                          fun x(z: Int) {}
                      }
                    `,
					kind.Keyword(),
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.ConformanceError{}, errs[0])
		})
	}
}

func TestCheckInvalidInterfaceConformanceRepetition(t *testing.T) {

	t.Parallel()

	for _, kind := range common.AllCompositeKinds {

		if !kind.SupportsInterfaces() {
			continue
		}

		body := "{}"
		if kind == common.CompositeKindEvent {
			body = "()"
		}

		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s interface X %[2]s

                      %[1]s interface Y %[2]s

                      %[1]s TestImpl: X, Y, X {}
                    `,
					kind.Keyword(),
					body,
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.DuplicateConformanceError{}, errs[0])
		})
	}
}

func TestCheckInvalidInterfaceTypeAsValue(t *testing.T) {

	t.Parallel()

	for _, kind := range common.AllCompositeKinds {

		if !kind.SupportsInterfaces() {
			continue
		}

		body := "{}"
		if kind == common.CompositeKindEvent {
			body = "()"
		}

		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s interface X %[2]s

                      let x = X
                    `,
					kind.Keyword(),
					body,
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
		})
	}
}

func TestCheckInterfaceWithFieldHavingStructType(t *testing.T) {

	t.Parallel()

	for _, firstKind := range common.CompositeKindsWithBody {
		for _, secondKind := range common.CompositeKindsWithBody {

			testName := fmt.Sprintf(
				"%s/%s",
				firstKind.Keyword(),
				secondKind.Keyword(),
			)

			t.Run(testName, func(t *testing.T) {

				_, err := ParseAndCheck(t,
					fmt.Sprintf(
						`
                          %[1]s S {}

                          %[2]s interface I {
                              s: %[3]sS
                          }
                        `,
						firstKind.Keyword(),
						secondKind.Keyword(),
						firstKind.Annotation(),
					),
				)

				// `firstKind` is the nested composite kind.
				// `secondKind` is the container composite kind.
				// Resource composites can only be nested in resource composite kinds.

				if firstKind == common.CompositeKindResource {
					switch secondKind {
					case common.CompositeKindResource,
						common.CompositeKindContract:

						require.NoError(t, err)

					default:
						errs := ExpectCheckerErrors(t, err, 1)

						assert.IsType(t, &sema.InvalidResourceFieldError{}, errs[0])
					}
				} else {
					require.NoError(t, err)
				}
			})
		}
	}
}

func TestCheckInterfaceWithFunctionHavingStructType(t *testing.T) {

	t.Parallel()

	for _, firstKind := range common.CompositeKindsWithBody {
		for _, secondKind := range common.CompositeKindsWithBody {

			testName := fmt.Sprintf(
				"%s/%s",
				firstKind.Keyword(),
				secondKind.Keyword(),
			)

			t.Run(testName, func(t *testing.T) {

				_, err := ParseAndCheck(t,
					fmt.Sprintf(
						`
                          %[1]s S {}

                          %[2]s interface I {
                              fun s(): %[3]sS
                          }
                        `,
						firstKind.Keyword(),
						secondKind.Keyword(),
						firstKind.Annotation(),
					),
				)

				require.NoError(t, err)
			})
		}
	}
}

func TestCheckInterfaceUseCompositeInInitializer(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      struct Foo {}

      struct interface Bar {
          init(foo: Foo)
      }
    `)

	require.NoError(t, err)
}

func TestCheckInterfaceSelfUse(t *testing.T) {

	t.Parallel()

	declarationKinds := []common.DeclarationKind{
		common.DeclarationKindInitializer,
		common.DeclarationKindFunction,
	}

	for _, compositeKind := range common.CompositeKindsWithBody {
		for _, declarationKind := range declarationKinds {

			testName := fmt.Sprintf("%s %s", compositeKind, declarationKind)

			innerDeclaration := ""
			switch declarationKind {
			case common.DeclarationKindInitializer:
				innerDeclaration = declarationKind.Keywords()

			case common.DeclarationKindFunction:
				innerDeclaration = fmt.Sprintf("%s test", declarationKind.Keywords())

			default:
				panic(errors.NewUnreachableError())
			}

			t.Run(testName, func(t *testing.T) {

				_, err := ParseAndCheck(t,
					fmt.Sprintf(
						`
                          %[1]s interface Bar {
                              balance: Int

                              %[2]s(balance: Int) {
                                  post {
                                      self.balance == balance
                                  }
                              }
                          }
                        `,
						compositeKind.Keyword(),
						innerDeclaration,
					),
				)

				require.NoError(t, err)
			})
		}
	}
}

func TestCheckInvalidContractInterfaceConformanceMissingTypeRequirement(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t,
		`
          contract interface Test {
              struct Nested {}
          }

          contract TestImpl: Test {
              // missing 'Nested'
          }
        `,
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ConformanceError{}, errs[0])
}

func TestCheckInvalidContractInterfaceConformanceTypeRequirementKindMismatch(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t,
		`
          contract interface Test {
              struct Nested {}
          }

          contract TestImpl: Test {
              // expected struct, not struct interface
              struct interface Nested {}
          }
        `,
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.DeclarationKindMismatchError{}, errs[0])
}

func TestCheckInvalidContractInterfaceConformanceTypeRequirementMismatch(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t,
		`
         contract interface Test {
             struct Nested {}
         }

         contract TestImpl: Test {
             // expected struct
             resource Nested {}
         }
        `,
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.CompositeKindMismatchError{}, errs[0])
}

func TestCheckContractInterfaceTypeRequirement(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t,
		`
          contract interface Test {
              struct Nested {
                  fun test(): Int
              }
          }
        `,
	)

	require.NoError(t, err)
}

func TestCheckInvalidContractInterfaceTypeRequirementFunctionImplementation(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t,
		`
          contract interface Test {
              struct Nested {
                  fun test(): Int {
                      return 1
                  }
              }
          }
        `,
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidImplementationError{}, errs[0])
}

func TestCheckInvalidContractInterfaceTypeRequirementMissingFunction(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t,
		`
          contract interface Test {
              struct Nested {
                  fun test(): Int
              }
          }

          contract TestImpl: Test {
             struct Nested {
                 // missing function 'test'
             }
          }
        `,
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ConformanceError{}, errs[0])
}

func TestCheckContractInterfaceTypeRequirementWithFunction(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t,
		`
          contract interface Test {
              struct Nested {
                  fun test(): Int
              }
          }

          contract TestImpl: Test {
             struct Nested {
                  fun test(): Int {
                      return 1
                  }
             }
          }
        `,
	)

	require.NoError(t, err)
}

func TestCheckContractInterfaceTypeRequirementConformanceMissingMembers(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t,
		`
          contract interface Test {

              struct interface NestedInterface {
                  fun test(): Bool
              }

              struct Nested: NestedInterface {
                  // missing function 'test' is valid:
                  // 'Nested' is a requirement, not an actual declaration
              }
          }
        `,
	)

	require.NoError(t, err)
}

func TestCheckInvalidContractInterfaceTypeRequirementConformance(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t,
		`
          contract interface Test {

              struct interface NestedInterface {
                  fun test(): Bool
              }

              struct Nested: NestedInterface {
                  // return type mismatch, should be 'Bool'
                  fun test(): Int
              }
          }
        `,
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ConformanceError{}, errs[0])
}

func TestCheckInvalidContractInterfaceTypeRequirementConformanceMissingFunction(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t,
		`
          contract interface Test {

              struct interface NestedInterface {
                  fun test(): Bool
              }

              struct Nested: NestedInterface {}
          }

          contract TestImpl: Test {

              struct Nested: Test.NestedInterface {
                  // missing function 'test'
              }
          }
        `,
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ConformanceError{}, errs[0])
}

func TestCheckInvalidContractInterfaceTypeRequirementMissingConformance(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t,
		`
          contract interface Test {

              struct interface NestedInterface {
                  fun test(): Bool
              }

              struct Nested: NestedInterface {}
          }

          contract TestImpl: Test {

              // missing conformance to 'Test.NestedInterface'
              struct Nested {
                  fun test(): Bool {
                      return true
                  }
              }
          }
        `,
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.MissingConformanceError{}, errs[0])
}

func TestCheckContractInterfaceTypeRequirementImplementation(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t,
		`
          struct interface OtherInterface {}

          contract interface Test {

              struct interface NestedInterface {
                  fun test(): Bool
              }

              struct Nested: NestedInterface {}
          }

          contract TestImpl: Test {

              struct Nested: Test.NestedInterface, OtherInterface {
                  fun test(): Bool {
                      return true
                  }
              }
          }
        `,
	)

	require.NoError(t, err)
}

func TestCheckContractInterfaceFungibleToken(t *testing.T) {

	t.Parallel()

	const code = examples.FungibleTokenContractInterface
	_, err := ParseAndCheck(t, code)

	if !assert.NoError(t, err) {
		cmd.PrettyPrintError(err, "", map[string]string{"": code})
	}
}

func TestCheckContractInterfaceFungibleTokenConformance(t *testing.T) {

	t.Parallel()

	code := examples.FungibleTokenContractInterface + "\n" + examples.ExampleFungibleTokenContract

	_, err := ParseAndCheckWithPanic(t, code)

	if !assert.NoError(t, err) {
		cmd.PrettyPrintError(err, "", map[string]string{"": code})
	}
}

func TestCheckContractInterfaceFungibleTokenUse(t *testing.T) {

	t.Parallel()

	code := examples.FungibleTokenContractInterface + "\n" +
		examples.ExampleFungibleTokenContract + "\n" + `

      fun test(): Int {
          let publisher <- ExampleToken.sprout(balance: 100)
          let receiver <- ExampleToken.sprout(balance: 0)

          let withdrawn <- publisher.withdraw(amount: 60)
          receiver.deposit(vault: <-withdrawn)

          let publisherBalance = publisher.balance
          let receiverBalance = receiver.balance

          destroy publisher
          destroy receiver

          return receiverBalance
      }
    `

	_, err := ParseAndCheckWithPanic(t, code)

	if !assert.NoError(t, err) {
		cmd.PrettyPrintError(err, "", map[string]string{"": code})
	}
}
