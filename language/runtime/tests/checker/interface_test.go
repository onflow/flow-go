package checker

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/language/runtime/common"
	"github.com/dapperlabs/flow-go/language/runtime/errors"
	"github.com/dapperlabs/flow-go/language/runtime/sema"
	"github.com/dapperlabs/flow-go/language/runtime/stdlib"
	. "github.com/dapperlabs/flow-go/language/runtime/tests/utils"
)

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

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				require.NoError(t, err)

			case common.CompositeKindContract:
				// TODO: add support for contract interface declarations

				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

			default:
				panic(errors.NewUnreachableError())
			}
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

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				require.NoError(t, err)

			case common.CompositeKindContract:
				// TODO: add support for contract interface declarations

				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

			default:
				panic(errors.NewUnreachableError())
			}
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

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.InvalidImplementationError{}, errs[0])

			case common.CompositeKindContract:
				// TODO: add support for contract interface declarations

				errs := ExpectCheckerErrors(t, err, 2)

				assert.IsType(t, &sema.InvalidImplementationError{}, errs[0])
				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[1])

			default:
				panic(errors.NewUnreachableError())
			}
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

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.InvalidImplementationError{}, errs[0])

			case common.CompositeKindContract:
				// TODO: add support for contract interface declarations

				errs := ExpectCheckerErrors(t, err, 2)

				assert.IsType(t, &sema.InvalidImplementationError{}, errs[0])
				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[1])

			default:
				panic(errors.NewUnreachableError())
			}
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

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				require.NoError(t, err)

			case common.CompositeKindContract:
				// TODO: add support for contract interface declarations

				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

			default:
				panic(errors.NewUnreachableError())
			}
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

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.InvalidImplementationError{}, errs[0])

			case common.CompositeKindContract:
				// TODO: add support for contract interface declarations

				errs := ExpectCheckerErrors(t, err, 2)

				assert.IsType(t, &sema.InvalidImplementationError{}, errs[0])
				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[1])

			default:
				panic(errors.NewUnreachableError())
			}
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

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				require.NoError(t, err)

			case common.CompositeKindContract:
				// TODO: add support for contract interface declarations
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

			default:
				panic(errors.NewUnreachableError())
			}
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
					kind.TransferOperator(),
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

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				require.NoError(t, err)

			case common.CompositeKindContract:
				errs := ExpectCheckerErrors(t, err, 1)

				// TODO: add support for contract interface declarations

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

			default:
				panic(errors.NewUnreachableError())
			}
		})
	}
}

func TestCheckInterfaceConformanceNoRequirements(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              %[1]s interface Test {}

              %[1]s TestImpl: Test {}

              let test: %[2]sTest %[3]s %[4]s TestImpl()
	        `,
				kind.Keyword(),
				kind.Annotation(),
				kind.TransferOperator(),
				kind.ConstructionKeyword(),
			))

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				require.NoError(t, err)

			case common.CompositeKindContract:
				// TODO: add support for contract interface declarations

				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

			default:
				panic(errors.NewUnreachableError())
			}
		})
	}
}

func TestCheckInvalidInterfaceConformanceIncompatibleCompositeKinds(t *testing.T) {

	for _, firstKind := range common.CompositeKinds {
		for _, secondKind := range common.CompositeKinds {

			// TODO: add support for contract declarations

			if firstKind != common.CompositeKindStructure &&
				firstKind != common.CompositeKindResource {

				continue
			}

			if secondKind != common.CompositeKindStructure &&
				secondKind != common.CompositeKindResource {

				continue
			}

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

                          let test: %[3]sTest %[4]s %[5]s TestImpl()
	                    `,
						firstKind.Keyword(),
						secondKind.Keyword(),
						firstKind.Annotation(),
						firstKind.TransferOperator(),
						secondKind.ConstructionKeyword(),
					),
				)

				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.CompositeKindMismatchError{}, errs[0])
			})
		}
	}
}

func TestCheckInvalidInterfaceConformanceUndeclared(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s interface Test {}

                      // NOTE: not declaring conformance
                      %[1]s TestImpl {}

                      let test: %[2]sTest %[3]s %[4]s TestImpl()
	                `,
					kind.Keyword(),
					kind.Annotation(),
					kind.TransferOperator(),
					kind.ConstructionKeyword(),
				),
			)

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.TypeMismatchError{}, errs[0])

			case common.CompositeKindContract:
				errs := ExpectCheckerErrors(t, err, 2)

				// TODO: add support for contract interface declarations

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
				assert.IsType(t, &sema.TypeMismatchError{}, errs[1])

			default:
				panic(errors.NewUnreachableError())
			}
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

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

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
					kind.Keyword(),
					kind.Annotation(),
					kind.TransferOperator(),
					kind.ConstructionKeyword(),
				),
			)

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				require.NoError(t, err)

			case common.CompositeKindContract:
				errs := ExpectCheckerErrors(t, err, 1)

				// TODO: add support for contract interface declarations

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

			default:
				panic(errors.NewUnreachableError())
			}
		})
	}
}

func TestCheckInvalidInterfaceUndeclaredFieldUse(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

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
					kind.Keyword(),
					kind.Annotation(),
					kind.TransferOperator(),
					kind.ConstructionKeyword(),
				),
			)

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.NotDeclaredMemberError{}, errs[0])

			case common.CompositeKindContract:
				errs := ExpectCheckerErrors(t, err, 2)

				// TODO: add support for contract interface declarations

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
				assert.IsType(t, &sema.NotDeclaredMemberError{}, errs[1])

			default:
				panic(errors.NewUnreachableError())
			}
		})
	}
}

func TestCheckInterfaceFunctionUse(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

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

                      let test: %[2]sTest %[3]s %[4]s TestImpl()

                      let val = test.test()
	                `,
					kind.Keyword(),
					kind.Annotation(),
					kind.TransferOperator(),
					kind.ConstructionKeyword(),
				),
			)

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				require.NoError(t, err)

			case common.CompositeKindContract:
				// TODO: add support for contract interface declarations

				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

			default:
				panic(errors.NewUnreachableError())
			}
		})
	}
}

func TestCheckInvalidInterfaceUndeclaredFunctionUse(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s interface Test {}

                      %[1]s TestImpl: Test {
                          fun test(): Int {
                              return 2
                          }
                      }

                      let test: %[2]sTest %[3]s %[4]s TestImpl()

                      let val = test.test()
	                `,
					kind.Keyword(),
					kind.Annotation(),
					kind.TransferOperator(),
					kind.ConstructionKeyword(),
				),
			)

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.NotDeclaredMemberError{}, errs[0])

			case common.CompositeKindContract:
				errs := ExpectCheckerErrors(t, err, 2)

				// TODO: add support for contract interface declarations

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
				assert.IsType(t, &sema.NotDeclaredMemberError{}, errs[1])

			default:
				panic(errors.NewUnreachableError())
			}
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

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.ConformanceError{}, errs[0])

			case common.CompositeKindContract:
				errs := ExpectCheckerErrors(t, err, 2)

				// TODO: add support for contract interface declarations

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

				assert.IsType(t, &sema.ConformanceError{}, errs[1])

			default:
				panic(errors.NewUnreachableError())
			}
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

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.ConformanceError{}, errs[0])

			case common.CompositeKindContract:
				errs := ExpectCheckerErrors(t, err, 2)

				// TODO: add support for contract interface declarations

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
				assert.IsType(t, &sema.ConformanceError{}, errs[1])

			default:
				panic(errors.NewUnreachableError())
			}
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

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.ConformanceError{}, errs[0])

			case common.CompositeKindContract:
				errs := ExpectCheckerErrors(t, err, 2)

				// TODO: add support for contract interface declarations

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

				assert.IsType(t, &sema.ConformanceError{}, errs[1])

			default:
				panic(errors.NewUnreachableError())
			}
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

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.ConformanceError{}, errs[0])

			case common.CompositeKindContract:
				// TODO: add support for contract interface declarations

				errs := ExpectCheckerErrors(t, err, 2)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
				assert.IsType(t, &sema.ConformanceError{}, errs[1])

			default:
				panic(errors.NewUnreachableError())
			}
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

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.ConformanceError{}, errs[0])

			case common.CompositeKindContract:
				errs := ExpectCheckerErrors(t, err, 2)

				// TODO: add support for contract interface declarations

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
				assert.IsType(t, &sema.ConformanceError{}, errs[1])

			default:
				panic(errors.NewUnreachableError())
			}
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

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.ConformanceError{}, errs[0])

			case common.CompositeKindContract:
				errs := ExpectCheckerErrors(t, err, 2)

				// TODO: add support for contract interface declarations

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
				assert.IsType(t, &sema.ConformanceError{}, errs[1])

			default:
				panic(errors.NewUnreachableError())
			}
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

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.ConformanceError{}, errs[0])

			case common.CompositeKindContract:
				errs := ExpectCheckerErrors(t, err, 2)

				// TODO: add support for contract interface declarations

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
				assert.IsType(t, &sema.ConformanceError{}, errs[1])

			default:
				panic(errors.NewUnreachableError())
			}
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

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.ConformanceError{}, errs[0])

			case common.CompositeKindContract:
				// TODO: add support for contract interface declarations

				errs := ExpectCheckerErrors(t, err, 2)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
				assert.IsType(t, &sema.ConformanceError{}, errs[1])

			default:
				panic(errors.NewUnreachableError())
			}
		})
	}
}

func TestCheckInvalidInterfaceConformanceFieldMismatchAccessModifier(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              %[1]s interface Test {
                  pub(set) x: Int
              }

              %[1]s TestImpl: Test {
                  pub var x: Int

                  init(x: Int) {
                     self.x = x
                  }
              }
	        `, kind.Keyword()))

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.ConformanceError{}, errs[0])

			case common.CompositeKindContract:
				errs := ExpectCheckerErrors(t, err, 2)

				// TODO: add support for contract interface declarations

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
				assert.IsType(t, &sema.ConformanceError{}, errs[1])

			default:
				panic(errors.NewUnreachableError())
			}
		})
	}
}

func TestCheckInterfaceConformanceFieldMorePermissiveAccessModifier(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              %[1]s interface Test {
                  pub x: Int
              }

              %[1]s TestImpl: Test {
                  pub(set) var x: Int

                  init(x: Int) {
                     self.x = x
                  }
              }
	        `, kind.Keyword()))

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				require.NoError(t, err)

			case common.CompositeKindContract:
				errs := ExpectCheckerErrors(t, err, 1)

				// TODO: add support for contract interface declarations

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

			default:
				panic(errors.NewUnreachableError())
			}
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

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.ConformanceError{}, errs[0])

			case common.CompositeKindContract:
				errs := ExpectCheckerErrors(t, err, 2)

				// TODO: add support for contract interface declarations

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
				assert.IsType(t, &sema.ConformanceError{}, errs[1])

			default:
				panic(errors.NewUnreachableError())
			}
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

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.ConformanceError{}, errs[0])

			case common.CompositeKindContract:
				errs := ExpectCheckerErrors(t, err, 2)

				// TODO: add support for contract interface declarations

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
				assert.IsType(t, &sema.ConformanceError{}, errs[1])

			default:
				panic(errors.NewUnreachableError())
			}
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

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.ConformanceError{}, errs[0])

			case common.CompositeKindContract:
				errs := ExpectCheckerErrors(t, err, 2)

				// TODO: add support for contract interface declarations

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
				assert.IsType(t, &sema.ConformanceError{}, errs[1])

			default:
				panic(errors.NewUnreachableError())
			}
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

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.ConformanceError{}, errs[0])

			case common.CompositeKindContract:
				errs := ExpectCheckerErrors(t, err, 2)

				// TODO: add support for contract interface declarations

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
				assert.IsType(t, &sema.ConformanceError{}, errs[1])

			default:
				panic(errors.NewUnreachableError())
			}
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

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.DuplicateConformanceError{}, errs[0])

			case common.CompositeKindContract:
				errs := ExpectCheckerErrors(t, err, 3)

				assert.IsType(t, &sema.DuplicateConformanceError{}, errs[0])

				// TODO: add support for contract interface declarations

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[1])
				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[2])

			default:
				panic(errors.NewUnreachableError())
			}
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

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.NotDeclaredError{}, errs[0])

			case common.CompositeKindContract:
				errs := ExpectCheckerErrors(t, err, 2)

				// TODO: add support for contract interface declarations

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
				assert.IsType(t, &sema.NotDeclaredError{}, errs[1])

			default:
				panic(errors.NewUnreachableError())
			}
		})
	}
}

func TestCheckInterfaceWithFieldHavingStructType(t *testing.T) {

	for _, firstKind := range common.CompositeKinds {
		for _, secondKind := range common.CompositeKinds {

			// TODO: add support for contract declarations

			if firstKind != common.CompositeKindStructure &&
				firstKind != common.CompositeKindResource {

				continue
			}

			if secondKind != common.CompositeKindStructure &&
				secondKind != common.CompositeKindResource {

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

			// TODO: add support for contract declarations

			if firstKind != common.CompositeKindStructure &&
				firstKind != common.CompositeKindResource {

				continue
			}

			if secondKind != common.CompositeKindStructure &&
				secondKind != common.CompositeKindResource {

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

				switch compositeKind {
				case common.CompositeKindResource, common.CompositeKindStructure:
					require.NoError(t, err)

				case common.CompositeKindContract:
					errs := ExpectCheckerErrors(t, err, 1)

					// TODO: add support for contract interface declarations

					assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

				default:
					panic(errors.NewUnreachableError())
				}
			})
		}
	}
}
