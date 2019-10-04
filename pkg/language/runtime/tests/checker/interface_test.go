package checker

import (
	"fmt"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/common"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/stdlib"
	. "github.com/dapperlabs/flow-go/pkg/language/runtime/tests/utils"
	. "github.com/onsi/gomega"
	"testing"
)

func TestCheckInvalidLocalInterface(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(fmt.Sprintf(`
              fun test() {
                  %s interface Test {}
              }
            `, kind.Keyword()))

			errs := ExpectCheckerErrors(err, 1)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.InvalidDeclarationError{}))
		})
	}
}

func TestCheckInterfaceWithFunction(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(fmt.Sprintf(`
              %s interface Test {
                  fun test()
              }
	        `, kind.Keyword()))

			// TODO: add support for non-structure declarations

			if kind == common.CompositeKindStructure {
				Expect(err).
					To(Not(HaveOccurred()))
			} else {
				errs := ExpectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
			}
		})
	}
}

func TestCheckInterfaceWithFunctionImplementationAndConditions(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(fmt.Sprintf(`
              %s interface Test {
                  fun test(x: Int) {
                      pre {
                        x == 0
                      }
                  }
              }
	        `, kind.Keyword()))

			// TODO: add support for non-structure declarations

			if kind == common.CompositeKindStructure {
				Expect(err).
					To(Not(HaveOccurred()))
			} else {
				errs := ExpectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
			}
		})
	}
}

func TestCheckInvalidInterfaceWithFunctionImplementation(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(fmt.Sprintf(`
              %s interface Test {
                  fun test(): Int {
                     return 1
                  }
              }
	        `, kind.Keyword()))

			// TODO: add support for non-structure declarations

			if kind == common.CompositeKindStructure {
				errs := ExpectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.InvalidImplementationError{}))
			} else {
				errs := ExpectCheckerErrors(err, 2)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.InvalidImplementationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
			}
		})
	}
}

func TestCheckInvalidInterfaceWithFunctionImplementationNoConditions(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(fmt.Sprintf(`
              %s interface Test {
                  fun test() {
                    // ...
                  }
              }
	        `, kind.Keyword()))

			// TODO: add support for non-structure declarations

			if kind == common.CompositeKindStructure {
				errs := ExpectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.InvalidImplementationError{}))
			} else {
				errs := ExpectCheckerErrors(err, 2)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.InvalidImplementationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
			}
		})
	}
}

func TestCheckInterfaceWithInitializer(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(fmt.Sprintf(`
              %s interface Test {
                  init()
              }
	        `, kind.Keyword()))

			// TODO: add support for non-structure declarations

			if kind == common.CompositeKindStructure {
				Expect(err).
					To(Not(HaveOccurred()))
			} else {
				errs := ExpectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
			}
		})
	}
}

func TestCheckInvalidInterfaceWithInitializerImplementation(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(fmt.Sprintf(`
              %s interface Test {
                  init() {
                    // ...
                  }
              }
	        `, kind.Keyword()))

			// TODO: add support for non-structure declarations

			expectedErrorCount := 1
			if kind != common.CompositeKindStructure {
				expectedErrorCount += 1
			}
			errs := ExpectCheckerErrors(err, expectedErrorCount)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.InvalidImplementationError{}))

			if kind != common.CompositeKindStructure {
				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
			}
		})
	}
}

func TestCheckInterfaceWithInitializerImplementationAndConditions(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(fmt.Sprintf(`
              %s interface Test {
                  init(x: Int) {
                      pre {
                        x == 0
                      }
                  }
              }
	        `, kind.Keyword()))

			// TODO: add support for non-structure declarations

			if kind == common.CompositeKindStructure {
				Expect(err).
					To(Not(HaveOccurred()))
			} else {
				errs := ExpectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
			}
		})
	}
}

func TestCheckInvalidInterfaceConstructorCall(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(fmt.Sprintf(`
              %s interface Test {}

              let test = Test()
	        `, kind.Keyword()))

			// TODO: add support for non-structure declarations

			if kind == common.CompositeKindStructure {
				errs := ExpectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.NotCallableError{}))
			} else {
				errs := ExpectCheckerErrors(err, 2)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.NotCallableError{}))
			}
		})
	}
}

func TestCheckInterfaceUse(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheckWithExtra(
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

			// TODO: add support for non-structure declarations

			if kind == common.CompositeKindStructure {
				Expect(err).
					To(Not(HaveOccurred()))
			} else {
				errs := ExpectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
			}
		})
	}
}

func TestCheckInterfaceConformanceNoRequirements(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(fmt.Sprintf(`
              %[1]s interface Test {}
    
              %[1]s TestImpl: Test {}
    
              let test: %[2]sTest %[3]s %[4]s TestImpl()
	        `,
				kind.Keyword(),
				kind.Annotation(),
				kind.TransferOperator(),
				kind.ConstructionKeyword(),
			))

			// TODO: add support for non-structure declarations

			if kind == common.CompositeKindStructure {
				Expect(err).
					To(Not(HaveOccurred()))
			} else {
				errs := ExpectCheckerErrors(err, 2)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
			}
		})
	}
}

func TestCheckInvalidInterfaceConformanceIncompatibleCompositeKinds(t *testing.T) {
	RegisterTestingT(t)

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

				_, err := ParseAndCheck(fmt.Sprintf(`
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

				// TODO: add support for non-structure declarations

				expectedErrorCount := 1
				if firstKind != common.CompositeKindStructure {
					expectedErrorCount += 1
				}
				if secondKind != common.CompositeKindStructure {
					expectedErrorCount += 1
				}
				_ = ExpectCheckerErrors(err, expectedErrorCount)
				//
				//	Expect(errs[0]).
				//		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
				//
				//	Expect(errs[1]).
				//		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
			})
		}
	}
}

func TestCheckInvalidInterfaceConformanceUndeclared(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(fmt.Sprintf(`
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

			// TODO: add support for non-structure declarations

			if kind == common.CompositeKindStructure {
				errs := ExpectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
			} else {
				errs := ExpectCheckerErrors(err, 3)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[2]).
					To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
			}
		})
	}
}

func TestCheckInvalidCompositeInterfaceConformanceNonInterface(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(fmt.Sprintf(`
              %s TestImpl: Int {}
	        `, kind.Keyword()))

			// TODO: add support for non-structure declarations

			expectedErrorCount := 1
			if kind != common.CompositeKindStructure {
				expectedErrorCount += 1
			}

			errs := ExpectCheckerErrors(err, expectedErrorCount)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.InvalidConformanceError{}))

			if kind != common.CompositeKindStructure {
				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
			}
		})
	}
}

func TestCheckInterfaceFieldUse(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(fmt.Sprintf(`
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

			// TODO: add support for non-structure declarations

			if kind == common.CompositeKindStructure {
				Expect(err).
					To(Not(HaveOccurred()))
			} else {
				errs := ExpectCheckerErrors(err, 2)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
			}
		})
	}
}

func TestCheckInvalidInterfaceUndeclaredFieldUse(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(fmt.Sprintf(`
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

			// TODO: add support for non-structure declarations

			if kind == common.CompositeKindStructure {
				errs := ExpectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.NotDeclaredMemberError{}))
			} else {
				errs := ExpectCheckerErrors(err, 3)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[2]).
					To(BeAssignableToTypeOf(&sema.NotDeclaredMemberError{}))
			}
		})
	}
}

func TestCheckInterfaceFunctionUse(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(fmt.Sprintf(`
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

			// TODO: add support for non-structure declarations

			if kind == common.CompositeKindStructure {
				Expect(err).
					To(Not(HaveOccurred()))
			} else {
				errs := ExpectCheckerErrors(err, 2)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
			}
		})
	}
}

func TestCheckInvalidInterfaceUndeclaredFunctionUse(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(fmt.Sprintf(`
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

			// TODO: add support for non-structure declarations

			if kind == common.CompositeKindStructure {
				errs := ExpectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.NotDeclaredMemberError{}))
			} else {
				errs := ExpectCheckerErrors(err, 3)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[2]).
					To(BeAssignableToTypeOf(&sema.NotDeclaredMemberError{}))
			}
		})
	}
}

func TestCheckInvalidInterfaceConformanceInitializerExplicitMismatch(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(fmt.Sprintf(`
              %[1]s interface Test {
                  init(x: Int)
              }

              %[1]s TestImpl: Test {
                  init(x: Bool) {}
              }
	        `, kind.Keyword()))

			// TODO: add support for non-structure declarations

			if kind == common.CompositeKindStructure {
				errs := ExpectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.ConformanceError{}))
			} else {
				errs := ExpectCheckerErrors(err, 3)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.ConformanceError{}))

				Expect(errs[2]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
			}
		})
	}
}

func TestCheckInvalidInterfaceConformanceInitializerImplicitMismatch(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(fmt.Sprintf(`
              %[1]s interface Test {
                  init(x: Int)
              }

              %[1]s TestImpl: Test {
              }
	        `, kind.Keyword()))

			// TODO: add support for non-structure declarations

			if kind == common.CompositeKindStructure {
				errs := ExpectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.ConformanceError{}))
			} else {
				errs := ExpectCheckerErrors(err, 3)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.ConformanceError{}))

				Expect(errs[2]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
			}
		})
	}
}

func TestCheckInvalidInterfaceConformanceMissingFunction(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(fmt.Sprintf(`
              %[1]s interface Test {
                  fun test(): Int
              }

              %[1]s TestImpl: Test {}
	        `, kind.Keyword()))

			// TODO: add support for non-structure declarations

			if kind == common.CompositeKindStructure {
				errs := ExpectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.ConformanceError{}))
			} else {
				errs := ExpectCheckerErrors(err, 3)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.ConformanceError{}))

				Expect(errs[2]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
			}
		})
	}
}

func TestCheckInvalidInterfaceConformanceFunctionMismatch(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(fmt.Sprintf(`
              %[1]s interface Test {
                  fun test(): Int
              }

              %[1]s TestImpl: Test {
                  fun test(): Bool {
                      return true
                  }
              }
	        `, kind.Keyword()))

			// TODO: add support for non-structure declarations

			if kind == common.CompositeKindStructure {

				errs := ExpectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.ConformanceError{}))
			} else {
				errs := ExpectCheckerErrors(err, 3)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.ConformanceError{}))

				Expect(errs[2]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
			}
		})
	}
}

func TestCheckInvalidInterfaceConformanceMissingField(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(fmt.Sprintf(`
              %[1]s interface Test {
                   x: Int
              }

              %[1]s TestImpl: Test {}

	        `, kind.Keyword()))

			// TODO: add support for non-structure declarations

			if kind == common.CompositeKindStructure {
				errs := ExpectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.ConformanceError{}))
			} else {
				errs := ExpectCheckerErrors(err, 3)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.ConformanceError{}))

				Expect(errs[2]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
			}
		})
	}
}

func TestCheckInvalidInterfaceConformanceFieldTypeMismatch(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(fmt.Sprintf(`
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

			// TODO: add support for non-structure declarations

			if kind == common.CompositeKindStructure {
				errs := ExpectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.ConformanceError{}))
			} else {

				errs := ExpectCheckerErrors(err, 3)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.ConformanceError{}))

				Expect(errs[2]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
			}
		})
	}
}

func TestCheckInvalidInterfaceConformanceKindFieldFunctionMismatch(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(fmt.Sprintf(`
              %[1]s interface Test {
                  x: Bool
              }

              %[1]s TestImpl: Test {
                  fun x(): Bool {
                      return true
                  }
              }
	        `, kind.Keyword()))

			// TODO: add support for non-structure declarations

			if kind == common.CompositeKindStructure {
				errs := ExpectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.ConformanceError{}))
			} else {

				errs := ExpectCheckerErrors(err, 3)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.ConformanceError{}))

				Expect(errs[2]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
			}
		})
	}
}

func TestCheckInvalidInterfaceConformanceKindFunctionFieldMismatch(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(fmt.Sprintf(`
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

			// TODO: add support for non-structure declarations

			if kind == common.CompositeKindStructure {
				errs := ExpectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.ConformanceError{}))
			} else {

				errs := ExpectCheckerErrors(err, 3)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.ConformanceError{}))

				Expect(errs[2]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
			}
		})
	}
}

func TestCheckInvalidInterfaceConformanceFieldKindLetVarMismatch(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(fmt.Sprintf(`
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

			// TODO: add support for non-structure declarations

			if kind == common.CompositeKindStructure {
				errs := ExpectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.ConformanceError{}))
			} else {
				errs := ExpectCheckerErrors(err, 3)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.ConformanceError{}))

				Expect(errs[2]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
			}
		})
	}
}

func TestCheckInvalidInterfaceConformanceFieldKindVarLetMismatch(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(fmt.Sprintf(`
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

			// TODO: add support for non-structure declarations

			if kind == common.CompositeKindStructure {
				errs := ExpectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.ConformanceError{}))
			} else {

				errs := ExpectCheckerErrors(err, 3)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.ConformanceError{}))

				Expect(errs[2]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
			}
		})
	}
}

func TestCheckInvalidInterfaceConformanceRepetition(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(fmt.Sprintf(`
              %[1]s interface X {}

              %[1]s interface Y {}

              %[1]s TestImpl: X, Y, X {}
	        `, kind.Keyword()))

			// TODO: add support for non-structure declarations

			expectedErrorCount := 1
			if kind != common.CompositeKindStructure {
				expectedErrorCount += 3
			}

			errs := ExpectCheckerErrors(err, expectedErrorCount)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.DuplicateConformanceError{}))

			if kind != common.CompositeKindStructure {

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.DuplicateConformanceError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[2]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[3]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
			}
		})
	}
}

func TestCheckInterfaceTypeAsValue(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			checker, err := ParseAndCheck(fmt.Sprintf(`
              %s interface X {}

              let x = X
	        `, kind.Keyword()))

			// TODO: add support for non-structure declarations

			if kind == common.CompositeKindStructure {
				Expect(err).
					To(Not(HaveOccurred()))

				Expect(checker.GlobalValues["x"].Type).
					To(BeAssignableToTypeOf(&sema.InterfaceMetaType{}))
			} else {
				errs := ExpectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
			}
		})
	}
}

func TestCheckInterfaceWithFieldHavingStructType(t *testing.T) {
	RegisterTestingT(t)

	for _, firstKind := range common.CompositeKinds {
		for _, secondKind := range common.CompositeKinds {

			testName := fmt.Sprintf(
				"%s/%s",
				firstKind.Keyword(),
				secondKind.Keyword(),
			)

			t.Run(testName, func(t *testing.T) {

				_, err := ParseAndCheck(fmt.Sprintf(`
                  %[1]s S {}

                  %[2]s interface I {
                      s: %[3]sS
                  }
	            `,
					firstKind.Keyword(),
					secondKind.Keyword(),
					firstKind.Annotation(),
				))

				expectedErrorCount := 0
				if firstKind != common.CompositeKindStructure {
					expectedErrorCount += 1
				}
				if secondKind != common.CompositeKindStructure {
					expectedErrorCount += 1
				}

				if expectedErrorCount == 0 {
					Expect(err).
						To(Not(HaveOccurred()))
				} else {
					errs := ExpectCheckerErrors(err, expectedErrorCount)

					for i := 0; i < expectedErrorCount; i += 1 {
						Expect(errs[i]).
							To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
					}
				}
			})
		}
	}
}

func TestCheckInterfaceWithFunctionHavingStructType(t *testing.T) {
	RegisterTestingT(t)

	for _, firstKind := range common.CompositeKinds {
		for _, secondKind := range common.CompositeKinds {

			testName := fmt.Sprintf(
				"%s/%s",
				firstKind.Keyword(),
				secondKind.Keyword(),
			)

			t.Run(testName, func(t *testing.T) {

				_, err := ParseAndCheck(fmt.Sprintf(`
                  %[1]s S {}

                  %[2]s interface I {
                      fun s(): %[3]sS
                  }
	            `,
					firstKind.Keyword(),
					secondKind.Keyword(),
					firstKind.Annotation(),
				))

				expectedErrorCount := 0
				if firstKind != common.CompositeKindStructure {
					expectedErrorCount += 1
				}
				if secondKind != common.CompositeKindStructure {
					expectedErrorCount += 1
				}

				if expectedErrorCount == 0 {
					Expect(err).
						To(Not(HaveOccurred()))
				} else {
					errs := ExpectCheckerErrors(err, expectedErrorCount)

					for i := 0; i < expectedErrorCount; i += 1 {
						Expect(errs[i]).
							To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
					}
				}
			})
		}
	}
}
