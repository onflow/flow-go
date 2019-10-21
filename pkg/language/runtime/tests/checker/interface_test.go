package checker

import (
	"fmt"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/common"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/stdlib"
	. "github.com/dapperlabs/flow-go/pkg/language/runtime/tests/utils"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCheckInvalidLocalInterface(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              fun test() {
                  %s interface Test {}
              }
            `, kind.Keyword()))

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.InvalidDeclarationError{}, errs[0])
		})
	}
}

func TestCheckInterfaceWithFunction(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              %s interface Test {
                  fun test()
              }
	        `, kind.Keyword()))

			// TODO: add support for non-structure / non-resource declarations

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				assert.Nil(t, err)

			default:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
			}
		})
	}
}

func TestCheckInterfaceWithFunctionImplementationAndConditions(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              %s interface Test {
                  fun test(x: Int) {
                      pre {
                        x == 0
                      }
                  }
              }
	        `, kind.Keyword()))

			// TODO: add support for non-structure / non-resource declarations

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				assert.Nil(t, err)

			default:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
			}
		})
	}
}

func TestCheckInvalidInterfaceWithFunctionImplementation(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              %s interface Test {
                  fun test(): Int {
                     return 1
                  }
              }
	        `, kind.Keyword()))

			// TODO: add support for non-structure / non-resource declarations

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.InvalidImplementationError{}, errs[0])

			default:
				errs := ExpectCheckerErrors(t, err, 2)

				assert.IsType(t, &sema.InvalidImplementationError{}, errs[0])
				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[1])
			}
		})
	}
}

func TestCheckInvalidInterfaceWithFunctionImplementationNoConditions(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              %s interface Test {
                  fun test() {
                    // ...
                  }
              }
	        `, kind.Keyword()))

			// TODO: add support for non-structure / non-resource declarations

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.InvalidImplementationError{}, errs[0])

			default:
				errs := ExpectCheckerErrors(t, err, 2)

				assert.IsType(t, &sema.InvalidImplementationError{}, errs[0])
				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[1])
			}
		})
	}
}

func TestCheckInterfaceWithInitializer(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              %s interface Test {
                  init()
              }
	        `, kind.Keyword()))

			// TODO: add support for non-structure / non-resource declarations

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				assert.Nil(t, err)

			default:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
			}
		})
	}
}

func TestCheckInvalidInterfaceWithInitializerImplementation(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              %s interface Test {
                  init() {
                    // ...
                  }
              }
	        `, kind.Keyword()))

			// TODO: add support for non-structure / non-resource declarations

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.InvalidImplementationError{}, errs[0])

			default:
				errs := ExpectCheckerErrors(t, err, 2)

				assert.IsType(t, &sema.InvalidImplementationError{}, errs[0])
				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[1])
			}
		})
	}
}

func TestCheckInterfaceWithInitializerImplementationAndConditions(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              %s interface Test {
                  init(x: Int) {
                      pre {
                        x == 0
                      }
                  }
              }
	        `, kind.Keyword()))

			// TODO: add support for non-structure / non-resource declarations

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				assert.Nil(t, err)

			default:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
			}
		})
	}
}

func TestCheckInterfaceUse(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheckWithExtra(t,
				fmt.Sprintf(`
                  %[1]s interface Test {}

                  let test: %[2]sTest %[3]s panic("")
                `,
					kind.Keyword(),
					kind.Annotation(),
					kind.TransferOperator(),
				),
				stdlib.StandardLibraryFunctions{
					stdlib.PanicFunction,
				}.ToValueDeclarations(),
				nil,
				nil,
			)

			// TODO: add support for non-structure / non-resource declarations

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				assert.Nil(t, err)

			default:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
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

			// TODO: add support for non-structure / non-resource declarations

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				assert.Nil(t, err)

			default:
				errs := ExpectCheckerErrors(t, err, 2)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[1])
			}
		})
	}
}

func TestCheckInvalidInterfaceConformanceIncompatibleCompositeKinds(t *testing.T) {

	for _, firstKind := range common.CompositeKinds {
		for _, secondKind := range common.CompositeKinds {

			// TODO: add support for non-structure / non-resource declarations

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

				_, err := ParseAndCheck(t, fmt.Sprintf(`
                  %[1]s interface Test {}

                  %[2]s TestImpl: Test {}

                  let test: %[3]sTest %[4]s %[5]s TestImpl()
	            `,
					firstKind.Keyword(),
					secondKind.Keyword(),
					firstKind.Annotation(),
					firstKind.TransferOperator(),
					secondKind.ConstructionKeyword(),
				))

				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.CompositeKindMismatchError{}, errs[0])
			})
		}
	}
}

func TestCheckInvalidInterfaceConformanceUndeclared(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              %[1]s interface Test {}

              // NOTE: not declaring conformance
              %[1]s TestImpl {}

              let test: %[2]sTest %[3]s %[4]s TestImpl()
	        `,
				kind.Keyword(),
				kind.Annotation(),
				kind.TransferOperator(),
				kind.ConstructionKeyword(),
			))

			// TODO: add support for non-structure / non-resource declarations

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.TypeMismatchError{}, errs[0])

			default:
				errs := ExpectCheckerErrors(t, err, 3)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[1])
				assert.IsType(t, &sema.TypeMismatchError{}, errs[2])
			}
		})
	}
}

func TestCheckInvalidCompositeInterfaceConformanceNonInterface(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              %s TestImpl: Int {}
	        `, kind.Keyword()))

			// TODO: add support for non-structure / non-resource declarations

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.InvalidConformanceError{}, errs[0])

			default:
				errs := ExpectCheckerErrors(t, err, 2)

				assert.IsType(t, &sema.InvalidConformanceError{}, errs[0])
				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[1])
			}
		})
	}
}

func TestCheckInterfaceFieldUse(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
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
			))

			// TODO: add support for non-structure / non-resource declarations

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				assert.Nil(t, err)

			default:
				errs := ExpectCheckerErrors(t, err, 2)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[1])
			}
		})
	}
}

func TestCheckInvalidInterfaceUndeclaredFieldUse(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
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
			))

			// TODO: add support for non-structure / non-resource declarations

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.NotDeclaredMemberError{}, errs[0])

			default:
				errs := ExpectCheckerErrors(t, err, 3)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[1])
				assert.IsType(t, &sema.NotDeclaredMemberError{}, errs[2])
			}
		})
	}
}

func TestCheckInterfaceFunctionUse(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
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
			))

			// TODO: add support for non-structure / non-resource declarations

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				assert.Nil(t, err)

			default:
				errs := ExpectCheckerErrors(t, err, 2)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[1])
			}
		})
	}
}

func TestCheckInvalidInterfaceUndeclaredFunctionUse(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
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
			))

			// TODO: add support for non-structure / non-resource declarations

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.NotDeclaredMemberError{}, errs[0])

			default:
				errs := ExpectCheckerErrors(t, err, 3)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[1])
				assert.IsType(t, &sema.NotDeclaredMemberError{}, errs[2])
			}
		})
	}
}

func TestCheckInvalidInterfaceConformanceInitializerExplicitMismatch(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              %[1]s interface Test {
                  init(x: Int)
              }

              %[1]s TestImpl: Test {
                  init(x: Bool) {}
              }
	        `, kind.Keyword()))

			// TODO: add support for non-structure / non-resource declarations

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.ConformanceError{}, errs[0])

			default:
				errs := ExpectCheckerErrors(t, err, 3)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
				assert.IsType(t, &sema.ConformanceError{}, errs[1])
				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[2])
			}
		})
	}
}

func TestCheckInvalidInterfaceConformanceInitializerImplicitMismatch(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              %[1]s interface Test {
                  init(x: Int)
              }

              %[1]s TestImpl: Test {
              }
	        `, kind.Keyword()))

			// TODO: add support for non-structure / non-resource declarations

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.ConformanceError{}, errs[0])

			default:
				errs := ExpectCheckerErrors(t, err, 3)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
				assert.IsType(t, &sema.ConformanceError{}, errs[1])
				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[2])
			}
		})
	}
}

func TestCheckInvalidInterfaceConformanceMissingFunction(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              %[1]s interface Test {
                  fun test(): Int
              }

              %[1]s TestImpl: Test {}
	        `, kind.Keyword()))

			// TODO: add support for non-structure / non-resource declarations

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.ConformanceError{}, errs[0])

			default:
				errs := ExpectCheckerErrors(t, err, 3)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
				assert.IsType(t, &sema.ConformanceError{}, errs[1])
				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[2])
			}
		})
	}
}

func TestCheckInvalidInterfaceConformanceFunctionMismatch(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              %[1]s interface Test {
                  fun test(): Int
              }

              %[1]s TestImpl: Test {
                  fun test(): Bool {
                      return true
                  }
              }
	        `, kind.Keyword()))

			// TODO: add support for non-structure / non-resource declarations

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.ConformanceError{}, errs[0])

			default:
				errs := ExpectCheckerErrors(t, err, 3)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
				assert.IsType(t, &sema.ConformanceError{}, errs[1])
				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[2])
			}
		})
	}
}

func TestCheckInvalidInterfaceConformanceMissingField(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              %[1]s interface Test {
                   x: Int
              }

              %[1]s TestImpl: Test {}

	        `, kind.Keyword()))

			// TODO: add support for non-structure / non-resource declarations

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.ConformanceError{}, errs[0])

			default:
				errs := ExpectCheckerErrors(t, err, 3)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
				assert.IsType(t, &sema.ConformanceError{}, errs[1])
				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[2])
			}
		})
	}
}

func TestCheckInvalidInterfaceConformanceFieldTypeMismatch(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              %[1]s interface Test {
                  x: Int
              }

              %[1]s TestImpl: Test {
                  var x: Bool
                  init(x: Bool) {
                     self.x = x
                  }
              }
	        `, kind.Keyword()))

			// TODO: add support for non-structure / non-resource declarations

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.ConformanceError{}, errs[0])

			default:
				errs := ExpectCheckerErrors(t, err, 3)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
				assert.IsType(t, &sema.ConformanceError{}, errs[1])
				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[2])
			}
		})
	}
}

func TestCheckInvalidInterfaceConformanceKindFieldFunctionMismatch(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              %[1]s interface Test {
                  x: Bool
              }

              %[1]s TestImpl: Test {
                  fun x(): Bool {
                      return true
                  }
              }
	        `, kind.Keyword()))

			// TODO: add support for non-structure / non-resource declarations

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.ConformanceError{}, errs[0])

			default:
				errs := ExpectCheckerErrors(t, err, 3)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
				assert.IsType(t, &sema.ConformanceError{}, errs[1])
				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[2])
			}
		})
	}
}

func TestCheckInvalidInterfaceConformanceKindFunctionFieldMismatch(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              %[1]s interface Test {
                  fun x(): Bool
              }

              %[1]s TestImpl: Test {
                  var x: Bool

                  init(x: Bool) {
                     self.x = x
                  }
              }
	        `, kind.Keyword()))

			// TODO: add support for non-structure / non-resource declarations

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.ConformanceError{}, errs[0])

			default:
				errs := ExpectCheckerErrors(t, err, 3)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
				assert.IsType(t, &sema.ConformanceError{}, errs[1])
				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[2])
			}
		})
	}
}

func TestCheckInvalidInterfaceConformanceFieldKindLetVarMismatch(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              %[1]s interface Test {
                  let x: Bool
              }

              %[1]s TestImpl: Test {
                  var x: Bool

                  init(x: Bool) {
                     self.x = x
                  }
              }
	        `, kind.Keyword()))

			// TODO: add support for non-structure / non-resource declarations

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.ConformanceError{}, errs[0])

			default:
				errs := ExpectCheckerErrors(t, err, 3)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
				assert.IsType(t, &sema.ConformanceError{}, errs[1])
				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[2])
			}
		})
	}
}

func TestCheckInvalidInterfaceConformanceFieldKindVarLetMismatch(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              %[1]s interface Test {
                  var x: Bool
              }

              %[1]s TestImpl: Test {
                  let x: Bool

                  init(x: Bool) {
                     self.x = x
                  }
              }
	        `, kind.Keyword()))

			// TODO: add support for non-structure / non-resource declarations

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.ConformanceError{}, errs[0])

			default:
				errs := ExpectCheckerErrors(t, err, 3)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
				assert.IsType(t, &sema.ConformanceError{}, errs[1])
				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[2])
			}
		})
	}
}

func TestCheckInvalidInterfaceConformanceRepetition(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              %[1]s interface X {}

              %[1]s interface Y {}

              %[1]s TestImpl: X, Y, X {}
	        `, kind.Keyword()))

			// TODO: add support for non-structure / non-resource declarations

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.DuplicateConformanceError{}, errs[0])

			default:
				errs := ExpectCheckerErrors(t, err, 4)

				assert.IsType(t, &sema.DuplicateConformanceError{}, errs[0])
				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[1])
				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[2])
				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[3])
			}
		})
	}
}

func TestCheckInvalidInterfaceTypeAsValue(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              %s interface X {}

              let x = X
	        `, kind.Keyword()))

			// TODO: add support for non-structure / non-resource declarations

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.NotDeclaredError{}, errs[0])

			default:
				errs := ExpectCheckerErrors(t, err, 2)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
				assert.IsType(t, &sema.NotDeclaredError{}, errs[1])
			}
		})
	}
}

func TestCheckInterfaceWithFieldHavingStructType(t *testing.T) {

	for _, firstKind := range common.CompositeKinds {
		for _, secondKind := range common.CompositeKinds {

			// TODO: add support for non-structure / non-resource declarations

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

				_, err := ParseAndCheck(t, fmt.Sprintf(`
                  %[1]s S {}

                  %[2]s interface I {
                      s: %[3]sS
                  }
	            `,
					firstKind.Keyword(),
					secondKind.Keyword(),
					firstKind.Annotation(),
				))

				// `firstKind` is the nested composite kind.
				// `secondKind` is the container composite kind.
				// Resource composites can only be nested in resource composite kinds.

				if firstKind == common.CompositeKindResource &&
					secondKind != common.CompositeKindResource {

					errs := ExpectCheckerErrors(t, err, 1)

					assert.IsType(t, &sema.InvalidResourceFieldError{}, errs[0])
				} else {
					assert.Nil(t, err)
				}
			})
		}
	}
}

func TestCheckInterfaceWithFunctionHavingStructType(t *testing.T) {

	for _, firstKind := range common.CompositeKinds {
		for _, secondKind := range common.CompositeKinds {

			// TODO: add support for non-structure / non-resource declarations

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

				_, err := ParseAndCheck(t, fmt.Sprintf(`
                  %[1]s S {}

                  %[2]s interface I {
                      fun s(): %[3]sS
                  }
	            `,
					firstKind.Keyword(),
					secondKind.Keyword(),
					firstKind.Annotation(),
				))

				assert.Nil(t, err)
			})
		}
	}
}
