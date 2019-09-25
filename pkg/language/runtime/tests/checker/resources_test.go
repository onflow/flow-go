package checker

import (
	"fmt"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/common"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/pkg/language/runtime/tests/utils"
	. "github.com/onsi/gomega"
	"testing"
)

func TestCheckFailableDowncastingWithMoveAnnotation(t *testing.T) {
	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {
			RegisterTestingT(t)

			_, err := ParseAndCheck(fmt.Sprintf(`
              %[1]s T {}

              let test %[2]s %[3]s T() as? <-T
	        `,
				kind.Keyword(),
				kind.TransferOperator(),
				kind.ConstructionKeyword(),
			))

			switch kind {
			case common.CompositeKindResource:

				// TODO: add support for resources

				errs := ExpectCheckerErrors(err, 2)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				// TODO: add support for non-Any types in failable downcasting

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.UnsupportedTypeError{}))

			case common.CompositeKindContract:

				// TODO: add support for contracts

				errs := ExpectCheckerErrors(err, 3)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.InvalidMoveAnnotationError{}))

				// TODO: add support for non-Any types in failable downcasting

				Expect(errs[2]).
					To(BeAssignableToTypeOf(&sema.UnsupportedTypeError{}))

			case common.CompositeKindStructure:

				errs := ExpectCheckerErrors(err, 2)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.InvalidMoveAnnotationError{}))

				// TODO: add support for non-Any types in failable downcasting

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.UnsupportedTypeError{}))
			}
		})
	}
}

func TestCheckFunctionDeclarationParameterWithMoveAnnotation(t *testing.T) {
	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {
			RegisterTestingT(t)

			_, err := ParseAndCheck(fmt.Sprintf(`
              %s T {}

              fun test(r: <-T) {}
	        `, kind.Keyword()))

			switch kind {
			case common.CompositeKindResource:

				// TODO: add support for resources

				errs := ExpectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			case common.CompositeKindContract:

				// TODO: add support for contracts

				errs := ExpectCheckerErrors(err, 2)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.InvalidMoveAnnotationError{}))

			case common.CompositeKindStructure:

				errs := ExpectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.InvalidMoveAnnotationError{}))
			}
		})
	}
}

func TestCheckFunctionDeclarationParameterWithoutMoveAnnotation(t *testing.T) {
	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {
			RegisterTestingT(t)

			_, err := ParseAndCheck(fmt.Sprintf(`
              %s T {}

              fun test(r: T) {}
	        `, kind.Keyword()))

			switch kind {
			case common.CompositeKindResource:

				// TODO: add support for resources

				errs := ExpectCheckerErrors(err, 2)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.MissingMoveAnnotationError{}))

			case common.CompositeKindContract:

				// TODO: add support for contracts

				errs := ExpectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			case common.CompositeKindStructure:

				Expect(err).
					To(Not(HaveOccurred()))
			}
		})
	}
}

func TestCheckFunctionDeclarationReturnTypeWithMoveAnnotation(t *testing.T) {
	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {
			RegisterTestingT(t)

			_, err := ParseAndCheck(fmt.Sprintf(`
              %[1]s T {}

              fun test(): <-T {
                  return %[2]s %[3]s T()
              }
	        `,
				kind.Keyword(),
				kind.Annotation(),
				kind.ConstructionKeyword(),
			))

			switch kind {
			case common.CompositeKindResource:

				// TODO: add support for resources

				errs := ExpectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			case common.CompositeKindContract:

				// TODO: add support for contracts

				errs := ExpectCheckerErrors(err, 2)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.InvalidMoveAnnotationError{}))

			case common.CompositeKindStructure:

				errs := ExpectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.InvalidMoveAnnotationError{}))
			}
		})
	}
}

func TestCheckFunctionDeclarationReturnTypeWithoutMoveAnnotation(t *testing.T) {
	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {
			RegisterTestingT(t)

			_, err := ParseAndCheck(fmt.Sprintf(`
              %[1]s T {}

              fun test(): T {
                  return %[2]s %[3]s T()
              }
	        `,
				kind.Keyword(),
				kind.Annotation(),
				kind.ConstructionKeyword(),
			))

			switch kind {
			case common.CompositeKindResource:

				// TODO: add support for resources

				errs := ExpectCheckerErrors(err, 2)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.MissingMoveAnnotationError{}))

			case common.CompositeKindContract:

				// TODO: add support for contracts

				errs := ExpectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			case common.CompositeKindStructure:

				Expect(err).
					To(Not(HaveOccurred()))
			}
		})
	}
}

func TestCheckVariableDeclarationWithMoveAnnotation(t *testing.T) {
	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {
			RegisterTestingT(t)

			_, err := ParseAndCheck(fmt.Sprintf(`
              %[1]s T {}

              let test: <-T %[2]s %[3]s T()
	        `,
				kind.Keyword(),
				kind.TransferOperator(),
				kind.ConstructionKeyword(),
			))

			switch kind {
			case common.CompositeKindResource:

				// TODO: add support for resources

				errs := ExpectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			case common.CompositeKindContract:

				// TODO: add support for contracts

				errs := ExpectCheckerErrors(err, 2)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.InvalidMoveAnnotationError{}))

			case common.CompositeKindStructure:

				errs := ExpectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.InvalidMoveAnnotationError{}))
			}
		})
	}
}

func TestCheckVariableDeclarationWithoutMoveAnnotation(t *testing.T) {
	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {
			RegisterTestingT(t)

			_, err := ParseAndCheck(fmt.Sprintf(`
              %[1]s T {}

              let test: T %[2]s %[3]s T()
	        `,
				kind.Keyword(),
				kind.TransferOperator(),
				kind.ConstructionKeyword(),
			))

			switch kind {
			case common.CompositeKindResource:

				// TODO: add support for resources

				errs := ExpectCheckerErrors(err, 2)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.MissingMoveAnnotationError{}))

			case common.CompositeKindContract:

				// TODO: add support for contracts

				errs := ExpectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			case common.CompositeKindStructure:

				Expect(err).
					To(Not(HaveOccurred()))
			}
		})
	}
}

func TestCheckFieldDeclarationWithMoveAnnotation(t *testing.T) {
	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {
			RegisterTestingT(t)

			_, err := ParseAndCheck(fmt.Sprintf(`
              %[1]s T {}

              %[1]s U {
                  let t: <-T
                  init(t: <-T) {
                      self.t %[2]s t
                  }
              }
	        `,
				kind.Keyword(),
				kind.TransferOperator(),
			))

			switch kind {
			case common.CompositeKindResource:

				// TODO: add support for resources

				errs := ExpectCheckerErrors(err, 2)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			case common.CompositeKindContract:

				// TODO: add support for contracts

				errs := ExpectCheckerErrors(err, 4)

				// NOTE: one invalid move annotation error for field, one for parameter

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.InvalidMoveAnnotationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[2]).
					To(BeAssignableToTypeOf(&sema.InvalidMoveAnnotationError{}))

				Expect(errs[3]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			case common.CompositeKindStructure:

				errs := ExpectCheckerErrors(err, 2)

				// NOTE: one invalid move annotation error for field, one for parameter

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.InvalidMoveAnnotationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.InvalidMoveAnnotationError{}))
			}
		})
	}
}

func TestCheckFieldDeclarationWithoutMoveAnnotation(t *testing.T) {
	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {
			RegisterTestingT(t)

			_, err := ParseAndCheck(fmt.Sprintf(`
              %[1]s T {}

              %[1]s U {
                  let t: T
                  init(t: T) {
                      self.t %[2]s t
                  }
              }
	        `,
				kind.Keyword(),
				kind.TransferOperator(),
			))

			switch kind {
			case common.CompositeKindResource:

				// TODO: add support for resources

				// NOTE: one missing move annotation error for field, one for parameter

				errs := ExpectCheckerErrors(err, 4)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.MissingMoveAnnotationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[2]).
					To(BeAssignableToTypeOf(&sema.MissingMoveAnnotationError{}))

				Expect(errs[3]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			case common.CompositeKindContract:

				// TODO: add support for contracts

				errs := ExpectCheckerErrors(err, 2)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			case common.CompositeKindStructure:

				Expect(err).
					To(Not(HaveOccurred()))
			}
		})
	}
}

func TestCheckFunctionExpressionParameterWithMoveAnnotation(t *testing.T) {
	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {
			RegisterTestingT(t)

			_, err := ParseAndCheck(fmt.Sprintf(`
              %s T {}

              let test = fun (r: <-T) {}
	        `, kind.Keyword()))

			switch kind {
			case common.CompositeKindResource:

				// TODO: add support for resources

				errs := ExpectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			case common.CompositeKindContract:

				// TODO: add support for contracts

				errs := ExpectCheckerErrors(err, 2)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.InvalidMoveAnnotationError{}))

			case common.CompositeKindStructure:

				errs := ExpectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.InvalidMoveAnnotationError{}))
			}
		})
	}
}

func TestCheckFunctionExpressionParameterWithoutMoveAnnotation(t *testing.T) {
	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {
			RegisterTestingT(t)

			_, err := ParseAndCheck(fmt.Sprintf(`
              %s T {}

              let test = fun (r: T) {}
	        `, kind.Keyword()))

			switch kind {
			case common.CompositeKindResource:

				// TODO: add support for resources

				errs := ExpectCheckerErrors(err, 2)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.MissingMoveAnnotationError{}))

			case common.CompositeKindContract:

				// TODO: add support for contracts

				errs := ExpectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			case common.CompositeKindStructure:

				Expect(err).
					To(Not(HaveOccurred()))
			}
		})
	}
}

func TestCheckFunctionExpressionReturnTypeWithMoveAnnotation(t *testing.T) {
	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {
			RegisterTestingT(t)

			_, err := ParseAndCheck(fmt.Sprintf(`
              %[1]s T {}

              let test = fun (): <-T {
                  return %[2]s %[3]s T()
              }
	        `,
				kind.Keyword(),
				kind.Annotation(),
				kind.ConstructionKeyword(),
			))

			switch kind {
			case common.CompositeKindResource:

				// TODO: add support for resources

				errs := ExpectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			case common.CompositeKindContract:

				// TODO: add support for contracts

				errs := ExpectCheckerErrors(err, 2)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.InvalidMoveAnnotationError{}))

			case common.CompositeKindStructure:

				errs := ExpectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.InvalidMoveAnnotationError{}))
			}
		})
	}
}

func TestCheckFunctionExpressionReturnTypeWithoutMoveAnnotation(t *testing.T) {
	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {
			RegisterTestingT(t)

			_, err := ParseAndCheck(fmt.Sprintf(`
              %[1]s T {}

              let test = fun (): T {
                  return %[2]s %[3]s T()
              }
	        `,
				kind.Keyword(),
				kind.Annotation(),
				kind.ConstructionKeyword(),
			))

			switch kind {
			case common.CompositeKindResource:

				// TODO: add support for resources

				errs := ExpectCheckerErrors(err, 2)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.MissingMoveAnnotationError{}))

			case common.CompositeKindContract:

				// TODO: add support for contracts

				errs := ExpectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			case common.CompositeKindStructure:

				Expect(err).
					To(Not(HaveOccurred()))
			}
		})
	}
}

func TestCheckFunctionTypeParameterWithMoveAnnotation(t *testing.T) {
	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {
			RegisterTestingT(t)

			_, err := ParseAndCheck(fmt.Sprintf(`
              %s T {}

              let test: ((<-T): Void) = fun (r: <-T) {}
	        `, kind.Keyword()))

			switch kind {
			case common.CompositeKindResource:

				// TODO: add support for resources

				errs := ExpectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			case common.CompositeKindContract:

				// TODO: add support for contracts

				errs := ExpectCheckerErrors(err, 2)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.InvalidMoveAnnotationError{}))

			case common.CompositeKindStructure:

				errs := ExpectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.InvalidMoveAnnotationError{}))
			}
		})
	}
}

func TestCheckFunctionTypeParameterWithoutMoveAnnotation(t *testing.T) {
	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {
			RegisterTestingT(t)

			_, err := ParseAndCheck(fmt.Sprintf(`
              %s T {}

              let test: ((T): Void) = fun (r: T) {}
	        `, kind.Keyword()))

			switch kind {
			case common.CompositeKindResource:

				// TODO: add support for resources

				errs := ExpectCheckerErrors(err, 2)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.MissingMoveAnnotationError{}))

			case common.CompositeKindContract:

				// TODO: add support for contracts

				errs := ExpectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			case common.CompositeKindStructure:

				Expect(err).
					To(Not(HaveOccurred()))
			}
		})
	}
}

func TestCheckFunctionTypeReturnTypeWithMoveAnnotation(t *testing.T) {
	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {
			RegisterTestingT(t)

			_, err := ParseAndCheck(fmt.Sprintf(`
              %[1]s T {}

              let test: ((): <-T) = fun (): <-T {
                  return %[2]s %[3]s T()
              }
	        `,
				kind.Keyword(),
				kind.Annotation(),
				kind.ConstructionKeyword(),
			))

			switch kind {
			case common.CompositeKindResource:

				// TODO: add support for resources

				errs := ExpectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			case common.CompositeKindContract:

				// TODO: add support for contracts

				errs := ExpectCheckerErrors(err, 2)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.InvalidMoveAnnotationError{}))

			case common.CompositeKindStructure:

				errs := ExpectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.InvalidMoveAnnotationError{}))
			}
		})
	}
}

func TestCheckFunctionTypeReturnTypeWithoutMoveAnnotation(t *testing.T) {
	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {
			RegisterTestingT(t)

			_, err := ParseAndCheck(fmt.Sprintf(`
              %[1]s T {}

              let test: ((): T) = fun (): T {
                  return %[2]s %[3]s T()
              }
	        `,
				kind.Keyword(),
				kind.Annotation(),
				kind.ConstructionKeyword(),
			))

			switch kind {
			case common.CompositeKindResource:

				// TODO: add support for resources

				errs := ExpectCheckerErrors(err, 2)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.MissingMoveAnnotationError{}))

			case common.CompositeKindContract:

				// TODO: add support for contracts

				errs := ExpectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			case common.CompositeKindStructure:

				Expect(err).
					To(Not(HaveOccurred()))
			}
		})
	}
}

func TestCheckFailableDowncastingWithoutMoveAnnotation(t *testing.T) {
	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {
			RegisterTestingT(t)

			_, err := ParseAndCheck(fmt.Sprintf(`
              %[1]s T {}

              let test %[2]s %[3]s T() as? T
	        `,
				kind.Keyword(),
				kind.TransferOperator(),
				kind.ConstructionKeyword(),
			))

			switch kind {
			case common.CompositeKindResource:

				// TODO: add support for resources

				errs := ExpectCheckerErrors(err, 3)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.MissingMoveAnnotationError{}))

				// TODO: add support for non-Any types in failable downcasting

				Expect(errs[2]).
					To(BeAssignableToTypeOf(&sema.UnsupportedTypeError{}))

			case common.CompositeKindContract:

				// TODO: add support for contracts

				errs := ExpectCheckerErrors(err, 2)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				// TODO: add support for non-Any types in failable downcasting

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.UnsupportedTypeError{}))

			case common.CompositeKindStructure:

				// TODO: add support for non-Any types in failable downcasting

				errs := ExpectCheckerErrors(err, 1)

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedTypeError{}))
			}
		})
	}
}

func TestCheckUnaryMove(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      resource X {}

      fun foo(x: <-X): <-X {
          return <-x
      }

      var x <- foo(x: <-create X())

      fun bar() {
          x <- create X()
      }
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

}

func TestCheckImmediateDestroy(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      resource X {}

      fun test() {
          destroy create X()
      }
	`)

	// TODO: add create expression once supported
	// TODO: add support for resources

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
}

func TestCheckIndirectDestroy(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      resource X {}

      fun test() {
          let x <- create X()
          destroy x
      }
	`)

	// TODO: add create expression once supported
	// TODO: add support for resources

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
}

func TestCheckInvalidResourceCreationWithoutCreate(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      resource X {}

	  let x <- X()
	`)

	// TODO: add create expression once supported
	// TODO: add support for resources

	errs := ExpectCheckerErrors(err, 2)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.MissingCreateError{}))

}

func TestCheckInvalidDestroy(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      struct X {}

      fun test() {
          destroy X()
      }
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.InvalidDestructionError{}))
}

func TestCheckUnaryCreateAndDestroy(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      resource X {}

      fun test() {
          var x <- create X()
          destroy x
      }
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
}

func TestCheckUnaryCreateAndDestroyWithInitializer(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      resource X {
          let x: Int
          init(x: Int) {
              self.x = x
          }
      }

      fun test() {
          var x <- create X(x: 1)
          destroy x
      }
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
}

func TestCheckInvalidUnaryCreateAndDestroyWithWrongInitializerArguments(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      resource X {
          let x: Int
          init(x: Int) {
              self.x = x
          }
      }

      fun test() {
          var x <- create X(y: true)
          destroy x
      }
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(err, 3)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))

	Expect(errs[2]).
		To(BeAssignableToTypeOf(&sema.IncorrectArgumentLabelError{}))
}

func TestCheckInvalidUnaryCreateStruct(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      struct X {}

      fun test() {
          create X()
      }
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.InvalidConstructionError{}))
}

func TestCheckInvalidResourceLoss(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      resource X {}

      fun test() {
          create X()
      }
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(err, 2)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.ResourceLossError{}))
}
