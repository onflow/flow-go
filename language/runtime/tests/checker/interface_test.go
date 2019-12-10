package checker

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/language/runtime/common"
	"github.com/dapperlabs/flow-go/language/runtime/sema"
	"github.com/dapperlabs/flow-go/language/runtime/stdlib"
	. "github.com/dapperlabs/flow-go/language/runtime/tests/utils"
)

func constructorArguments(compositeKind common.CompositeKind) string {
	if compositeKind == common.CompositeKindContract {
		return ""
	}
	return "()"
}

func TestCheckInvalidLocalInterface(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      fun test() {
                          %s interface Test {}
                      }
                    `,
					kind.Keyword(),
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.InvalidDeclarationError{}, errs[0])
		})
	}
}

func TestCheckInterfaceWithFunction(t *testing.T) {

	for _, kind := range common.CompositeKinds {
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

	for _, kind := range common.CompositeKinds {
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

	for _, kind := range common.CompositeKinds {
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

	for _, kind := range common.CompositeKinds {
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

	for _, kind := range common.CompositeKinds {
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

	for _, kind := range common.CompositeKinds {
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

	for _, kind := range common.CompositeKinds {
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

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheckWithOptions(t,
				fmt.Sprintf(
					`
                      pub %[1]s interface Test {}

                      pub let test: %[2]sTest %[3]s panic("")
                    `,
					kind.Keyword(),
					kind.Annotation(),
					kind.AssignmentOperator(),
				),
				ParseAndCheckOptions{
					Options: []sema.Option{
						sema.WithPredeclaredValues(
							stdlib.StandardLibraryFunctions{
								stdlib.PanicFunction,
							}.ToValueDeclarations(),
						),
					},
				},
			)

			require.NoError(t, err)
		})
	}
}

func TestCheckInterfaceConformanceNoRequirements(t *testing.T) {

	for _, compositeKind := range common.CompositeKinds {

		t.Run(compositeKind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s interface Test {}

                      %[1]s TestImpl: Test {}

                      let test: %[2]sTest %[3]s %[4]s TestImpl%[5]s
	                `,
					compositeKind.Keyword(),
					compositeKind.Annotation(),
					compositeKind.AssignmentOperator(),
					compositeKind.ConstructionKeyword(),
					constructorArguments(compositeKind),
				))

			require.NoError(t, err)
		})
	}
}

func TestCheckInvalidInterfaceConformanceIncompatibleCompositeKinds(t *testing.T) {

	for _, firstKind := range common.CompositeKinds {
		for _, secondKind := range common.CompositeKinds {

			// only test incompatible combinations
			if firstKind == secondKind {
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
                          %[1]s interface Test {}

                          %[2]s TestImpl: Test {}

                          let test: %[3]sTest %[4]s %[5]s TestImpl%[6]s
	                    `,
						firstKind.Keyword(),
						secondKind.Keyword(),
						firstKind.Annotation(),
						firstKind.AssignmentOperator(),
						secondKind.ConstructionKeyword(),
						constructorArguments(secondKind),
					),
				)

				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.CompositeKindMismatchError{}, errs[0])
			})
		}
	}
}

func TestCheckInvalidInterfaceConformanceUndeclared(t *testing.T) {

	for _, compositeKind := range common.CompositeKinds {

		t.Run(compositeKind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s interface Test {}

                      // NOTE: not declaring conformance
                      %[1]s TestImpl {}

                      let test: %[2]sTest %[3]s %[4]s TestImpl%[5]s
	                `,
					compositeKind.Keyword(),
					compositeKind.Annotation(),
					compositeKind.AssignmentOperator(),
					compositeKind.ConstructionKeyword(),
					constructorArguments(compositeKind),
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
		})
	}
}

func TestCheckInvalidCompositeInterfaceConformanceNonInterface(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %s TestImpl: Int {}
	                `,
					kind.Keyword(),
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.InvalidConformanceError{}, errs[0])
		})
	}
}

func TestCheckInterfaceFieldUse(t *testing.T) {

	for _, compositeKind := range common.CompositeKinds {

		if compositeKind == common.CompositeKindContract {
			// Contracts cannot be instantiated
			continue
		}

		t.Run(compositeKind.Keyword(), func(t *testing.T) {

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

                      let test: %[2]sTest %[3]s %[4]s TestImpl(x: 1)

                      let x = test.x
                    `,
					compositeKind.Keyword(),
					compositeKind.Annotation(),
					compositeKind.AssignmentOperator(),
					compositeKind.ConstructionKeyword(),
				),
			)

			require.NoError(t, err)
		})
	}
}

func TestCheckInvalidInterfaceUndeclaredFieldUse(t *testing.T) {

	for _, compositeKind := range common.CompositeKinds {

		if compositeKind == common.CompositeKindContract {
			// Contracts cannot be instantiated
			continue
		}

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

                      let test: %[2]sTest %[3]s %[4]s TestImpl(x: 1)

                      let x = test.x
    	            `,
					compositeKind.Keyword(),
					compositeKind.Annotation(),
					compositeKind.AssignmentOperator(),
					compositeKind.ConstructionKeyword(),
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.NotDeclaredMemberError{}, errs[0])
		})
	}
}

func TestCheckInterfaceFunctionUse(t *testing.T) {

	for _, compositeKind := range common.CompositeKinds {

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

                      let test: %[2]sTest %[3]s %[4]s TestImpl%[5]s

                      let val = test.test()
	                `,
					compositeKind.Keyword(),
					compositeKind.Annotation(),
					compositeKind.AssignmentOperator(),
					compositeKind.ConstructionKeyword(),
					constructorArguments(compositeKind),
				),
			)

			require.NoError(t, err)
		})
	}
}

func TestCheckInvalidInterfaceUndeclaredFunctionUse(t *testing.T) {

	for _, compositeKind := range common.CompositeKinds {

		t.Run(compositeKind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s interface Test {}

                      %[1]s TestImpl: Test {
                          fun test(): Int {
                              return 2
                          }
                      }

                      let test: %[2]sTest %[3]s %[4]s TestImpl%[5]s

                      let val = test.test()
	                `,
					compositeKind.Keyword(),
					compositeKind.Annotation(),
					compositeKind.AssignmentOperator(),
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

	for _, kind := range common.CompositeKinds {
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

	for _, kind := range common.CompositeKinds {
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

	for _, kind := range common.CompositeKinds {
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

	for _, kind := range common.CompositeKinds {
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

	for _, kind := range common.CompositeKinds {
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

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s interface Test {
                           x: Int
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

func TestCheckInvalidInterfaceConformanceFieldTypeMismatch(t *testing.T) {

	for _, kind := range common.CompositeKinds {
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

	for _, kind := range common.CompositeKinds {
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

func TestCheckInvalidInterfaceConformanceFieldMismatchAccessModifier(t *testing.T) {

	for _, kind := range common.CompositeKinds {
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

func TestCheckInterfaceConformanceFieldMorePermissiveAccessModifier(t *testing.T) {

	for _, kind := range common.CompositeKinds {
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

	for _, kind := range common.CompositeKinds {
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

	for _, kind := range common.CompositeKinds {
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

	for _, kind := range common.CompositeKinds {
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

	for _, kind := range common.CompositeKinds {
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

func TestCheckInvalidInterfaceConformanceRepetition(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s interface X {}

                      %[1]s interface Y {}

                      %[1]s TestImpl: X, Y, X {}
	                `,
					kind.Keyword(),
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.DuplicateConformanceError{}, errs[0])
		})
	}
}

func TestCheckInvalidInterfaceTypeAsValue(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %s interface X {}

                      let x = X
	                `,
					kind.Keyword(),
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
		})
	}
}

func TestCheckInterfaceWithFieldHavingStructType(t *testing.T) {

	for _, firstKind := range common.CompositeKinds {
		for _, secondKind := range common.CompositeKinds {

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

				if firstKind == common.CompositeKindResource &&
					secondKind != common.CompositeKindResource {

					errs := ExpectCheckerErrors(t, err, 1)

					assert.IsType(t, &sema.InvalidResourceFieldError{}, errs[0])
				} else {
					require.NoError(t, err)
				}
			})
		}
	}
}

func TestCheckInterfaceWithFunctionHavingStructType(t *testing.T) {

	for _, firstKind := range common.CompositeKinds {
		for _, secondKind := range common.CompositeKinds {

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

	_, err := ParseAndCheck(t, `
      struct Foo {}

      struct interface Bar {
          init(foo: Foo)
      }
    `)

	require.NoError(t, err)
}

func TestCheckInterfaceSelfUse(t *testing.T) {

	declarationKinds := []common.DeclarationKind{
		common.DeclarationKindInitializer,
		common.DeclarationKindFunction,
	}

	for _, compositeKind := range common.CompositeKinds {
		for _, declarationKind := range declarationKinds {

			testName := fmt.Sprintf("%s %s", compositeKind, declarationKind)

			innerDeclaration := ""
			switch declarationKind {
			case common.DeclarationKindInitializer:
				innerDeclaration = declarationKind.Keywords()
			case common.DeclarationKindFunction:
				innerDeclaration = fmt.Sprintf("%s test", declarationKind.Keywords())
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

const fungibleTokenContractInterface = `
  pub contract interface FungibleToken {

	  pub resource interface Provider {

		  pub fun withdraw(amount: Int): @Vault
	  }

	  pub resource interface Receiver {

		  pub fun deposit(vault: @Vault)
	  }

	  pub resource Vault: Provider, Receiver {

		  pub balance: Int

		  init(balance: Int)
	  }

	  pub fun absorb(vault: @Vault)

	  pub fun sprout(): @Vault
  }
`

func TestCheckContractInterfaceFungibleToken(t *testing.T) {

	_, err := ParseAndCheck(t, fungibleTokenContractInterface)

	require.NoError(t, err)
}

const validExampleFungibleTokenContract = `
  pub contract ExampleToken: FungibleToken {

     pub resource Vault: FungibleToken.Receiver, FungibleToken.Provider {

         pub var balance: Int

         init(balance: Int) {
             self.balance = balance
         }

         pub fun withdraw(amount: Int): @Vault {
             self.balance = self.balance - amount
             return <-create Vault(balance: amount)
         }

         pub fun deposit(from: @Vault) {
            self.balance = self.balance + from.balance
            destroy from
         }
     }

     pub fun absorb(vault: @Vault) {
         destroy vault
     }

     pub fun sprout(): @Vault {
         return <-create Vault(balance: 0)
     }
  }
`

func TestCheckContractInterfaceFungibleTokenConformance(t *testing.T) {

	code := fungibleTokenContractInterface + "\n" + validExampleFungibleTokenContract

	_, err := ParseAndCheck(t, code)

	assert.NoError(t, err)
}

func TestCheckContractInterfaceFungibleTokenUse(t *testing.T) {

	code := fungibleTokenContractInterface + "\n" +
		validExampleFungibleTokenContract + "\n" + `

      fun test(): Int {
          let contract = ExampleToken

          // valid, because code is in the same location
          let publisher <- create ExampleToken.Vault(balance: 100)

          let receiver <- contract.sprout()

          let withdrawn <- publisher.withdraw(amount: 60)
          receiver.deposit(from: <-withdrawn)

          let publisherBalance = publisher.balance
          let receiverBalance = receiver.balance

          destroy publisher
          destroy receiver

          return receiverBalance
      }
	`

	_, err := ParseAndCheck(t, code)

	assert.NoError(t, err)
}
