package checker

import (
	"fmt"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
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
              %[1]s T {}

              fun test(r: <-T) {
                  %[2]s r
              }
	        `,
				kind.Keyword(),
				kind.DestructionKeyword(),
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

func TestCheckFunctionDeclarationParameterWithoutMoveAnnotation(t *testing.T) {
	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {
			RegisterTestingT(t)

			_, err := ParseAndCheck(fmt.Sprintf(`
              %[1]s T {}

              fun test(r: T) {
                  %[2]s r
              }
	        `,
				kind.Keyword(),
				kind.DestructionKeyword(),
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
              %[1]s T {}

              let test = fun (r: <-T) {
                  %[2]s r
              }
	        `,
				kind.Keyword(),
				kind.DestructionKeyword(),
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

func TestCheckFunctionExpressionParameterWithoutMoveAnnotation(t *testing.T) {
	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {
			RegisterTestingT(t)

			_, err := ParseAndCheck(fmt.Sprintf(`
              %[1]s T {}

              let test = fun (r: T) {
                  %[2]s r
              }
	        `,
				kind.Keyword(),
				kind.DestructionKeyword(),
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
              %[1]s T {}

              let test: ((<-T): Void) = fun (r: <-T) {
                  %[2]s r
              }
	        `,
				kind.Keyword(),
				kind.DestructionKeyword(),
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

func TestCheckFunctionTypeParameterWithoutMoveAnnotation(t *testing.T) {
	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {
			RegisterTestingT(t)

			_, err := ParseAndCheck(fmt.Sprintf(`
              %[1]s T {}

              let test: ((T): Void) = fun (r: T) {
                  %[2]s r
              }
	        `,
				kind.Keyword(),
				kind.DestructionKeyword(),
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
	t.Run("UnassignedResource", func(t *testing.T) {
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
	})

	t.Run("ImmediateMemberAccess", func(t *testing.T) {
		RegisterTestingT(t)

		_, err := ParseAndCheck(`
			resource Foo {
				fun bar(): Int {
					return 42
				}
			}

			fun test() {
				let x = (create Foo()).bar()
			}
		`)

		// TODO: add support for resources

		errs := ExpectCheckerErrors(err, 2)

		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

		Expect(errs[1]).
			To(BeAssignableToTypeOf(&sema.ResourceLossError{}))
	})

	t.Run("ImmediateMemberAccessFunctionInvocation", func(t *testing.T) {
		RegisterTestingT(t)

		_, err := ParseAndCheck(`
			resource Foo {
				fun bar(): Int {
					return 42
				}
			}
	
			fun createResource(): <-Foo {
				return <-create Foo()
			}
	
			fun test() {
				let x = createResource().bar()
			}
		`)

		// TODO: add support for resources

		errs := ExpectCheckerErrors(err, 2)

		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

		Expect(errs[1]).
			To(BeAssignableToTypeOf(&sema.ResourceLossError{}))
	})
}

func TestCheckResourceReturn(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      resource X {}

      fun test(): <-X {
          return <-create X()
      }
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
}

func TestCheckInvalidResourceReturnMissingMove(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      resource X {}

      fun test(): <-X {
          return create X()
      }
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(err, 2)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.MissingMoveOperationError{}))
}

func TestCheckInvalidResourceReturnMissingMoveInvalidReturnType(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      resource X {}

      fun test(): Y {
          return create X()
      }
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(err, 3)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

	Expect(errs[2]).
		To(BeAssignableToTypeOf(&sema.MissingMoveOperationError{}))
}

func TestCheckInvalidNonResourceReturnWithMove(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      struct X {}

      fun test(): X {
          return <-X()
      }
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.InvalidMoveOperationError{}))
}

func TestCheckResourceArgument(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      resource X {}

      fun foo(_ x: <-X) {
          destroy x
      }

      fun bar() {
          foo(<-create X())
      }
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
}

func TestCheckInvalidResourceArgumentMissingMove(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      resource X {}

      fun foo(_ x: <-X) {
          destroy x
      }

      fun bar() {
          foo(create X())
      }
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(err, 2)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.MissingMoveOperationError{}))
}

func TestCheckInvalidResourceArgumentMissingMoveInvalidParameterType(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      resource X {}

      fun foo(_ x: Y) {}

      fun bar() {
          foo(create X())
      }
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(err, 3)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

	Expect(errs[2]).
		To(BeAssignableToTypeOf(&sema.MissingMoveOperationError{}))
}

func TestCheckInvalidNonResourceArgumentWithMove(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      struct X {}

      fun foo(_ x: X) {}

      fun bar() {
          foo(<-X())
      }
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.InvalidMoveOperationError{}))
}

func TestCheckResourceVariableDeclarationTransfer(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      resource X {}

      let x <- create X()
      let y <- x
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
}

func TestCheckInvalidResourceVariableDeclarationIncorrectTransfer(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      resource X {}

      let x = create X()
      let y = x
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(err, 3)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.IncorrectTransferOperationError{}))

	Expect(errs[2]).
		To(BeAssignableToTypeOf(&sema.IncorrectTransferOperationError{}))
}

func TestCheckInvalidNonResourceVariableDeclarationMoveTransfer(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      struct X {}

      let x = X()
      let y <- x
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.IncorrectTransferOperationError{}))
}

func TestCheckResourceAssignmentTransfer(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      resource X {}

      let x <- create X()

      fun test() {
         var x2 <- create X()
         destroy x2
         x2 <- x
      }
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
}

func TestCheckInvalidResourceAssignmentIncorrectTransfer(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      resource X {}

      let x <- create X()

      fun test() {
        var x2 <- create X()
        destroy x2
        x2 = x
      }
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(err, 2)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.IncorrectTransferOperationError{}))
}

func TestCheckInvalidNonResourceAssignmentMoveTransfer(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      struct X {}

      let x = X()
      fun test() {
        var x2 = X()
        x2 <- x
      }
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.IncorrectTransferOperationError{}))
}

func TestCheckInvalidResourceLossThroughVariableDeclaration(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      resource X {}

      fun test() {
        let x <- create X()
      }
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(err, 2)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.ResourceLossError{}))
}

func TestCheckInvalidResourceLossThroughVariableDeclarationAfterCreation(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      resource X {}

      fun test() {
          let x <- create X()
          let y <- x
      }
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(err, 2)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.ResourceLossError{}))
}

func TestCheckInvalidResourceLossThroughAssignment(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      resource X {}

      fun test() {
          var x <- create X()
          let y <- create X()
          x <- y
      }
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(err, 2)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.ResourceLossError{}))
}

func TestCheckResourceMoveThroughReturn(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      resource X {}

      fun test(): <-X {
          let x <- create X()
          return <-x
      }
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
}

func TestCheckResourceMoveThroughArgumentPassing(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      resource X {}

      fun test() {
          let x <- create X()
          absorb(<-x)
      }

      fun absorb(_ x: <-X) {
          destroy x
      }
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
}

func TestCheckInvalidResourceUseAfterMoveToFunction(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      resource X {}

      fun test() {
          let x <- create X()
          absorb(<-x)
          absorb(<-x)
      }

      fun absorb(_ x: <-X) {
          destroy x
      }
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(err, 2)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.ResourceUseAfterInvalidationError{}))
}

func TestCheckInvalidResourceUseAfterMoveToVariable(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      resource X {}

      fun test() {
          let x <- create X()
          let y <- x
          let z <- x
      }
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(err, 4)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.ResourceUseAfterInvalidationError{}))

	// NOTE: still two resource losses reported for `y` and `z`

	Expect(errs[2]).
		To(BeAssignableToTypeOf(&sema.ResourceLossError{}))

	Expect(errs[3]).
		To(BeAssignableToTypeOf(&sema.ResourceLossError{}))
}

func TestCheckInvalidResourceFieldUseAfterMoveToVariable(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      resource X {
          let id: Int
          init(id: Int) {
              self.id = id
          }
      }

      fun test(): Int {
          let x <- create X(id: 1)
          absorb(<-x)
          return x.id
      }

      fun absorb(_ x: <-X) {
          destroy x
      }
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(err, 2)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.ResourceUseAfterInvalidationError{}))
}

func TestCheckResourceUseAfterMoveInIfStatementThenBranch(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      resource X {}

      fun test() {
          let x <- create X()
          if 1 > 2 {
              absorb(<-x)
          }
          absorb(<-x)
      }

      fun absorb(_ x: <-X) {
          destroy x
      }
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(err, 2)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.ResourceUseAfterInvalidationError{}))
}

func TestCheckResourceUseInIfStatement(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      resource X {}

      fun test() {
          let x <- create X()
          if 1 > 2 {
              absorb(<-x)
          } else {
              absorb(<-x)
          }
      }

      fun absorb(_ x: <-X) {
          destroy x
      }
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
}

func TestCheckResourceUseInNestedIfStatement(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      resource X {}

      fun test() {
          let x <- create X()
          if 1 > 2 {
              if 2 > 1 {
                  absorb(<-x)
              }
          } else {
              absorb(<-x)
          }
      }

      fun absorb(_ x: <-X) {
          destroy x
      }
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
}

func TestCheckInvalidResourceUseAfterIfStatement(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      resource X {}

      fun test(): <-X {
          let x <- create X()
          if 1 > 2 {
              absorb(<-x)
          } else {
              absorb(<-x)
          }
          return <-x
      }

      fun absorb(_ x: <-X) {
          destroy x
      }
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(err, 2)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.ResourceUseAfterInvalidationError{}))

	Expect(errs[1].(*sema.ResourceUseAfterInvalidationError).Invalidations).
		To(ConsistOf(
			sema.ResourceInvalidation{
				Kind: sema.ResourceInvalidationKindMove,
				Pos:  ast.Position{Offset: 165, Line: 9, Column: 23},
			},
			sema.ResourceInvalidation{
				Kind: sema.ResourceInvalidationKindMove,
				Pos:  ast.Position{Offset: 120, Line: 7, Column: 23},
			},
		))
}

func TestCheckInvalidResourceLossAfterDestroyInIfStatementThenBranch(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      resource X {}

      fun test() {
          let x <- create X()
          if 1 > 2 {
             destroy x
          }
      }
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(err, 2)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.ResourceLossError{}))
}

func TestCheckInvalidResourceLossAndUseAfterDestroyInIfStatementThenBranch(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      resource X {
          let id: Int
          init(id: Int) {
              self.id = id
          }
      }

      fun test() {
          let x <- create X(id: 1)
          if 1 > 2 {
             destroy x
          }
          x.id
      }
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(err, 3)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.ResourceUseAfterInvalidationError{}))

	Expect(errs[2]).
		To(BeAssignableToTypeOf(&sema.ResourceLossError{}))
}

func TestCheckResourceMoveIntoArray(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      resource X {}

      let x <- create X()
      let xs <- [<-x]
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
}

func TestCheckInvalidResourceMoveIntoArrayMissingMoveOperation(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      resource X {}

      let x <- create X()
      let xs <- [x]
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(err, 2)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.MissingMoveOperationError{}))
}

func TestCheckInvalidNonResourceMoveIntoArray(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      struct X {}

      let x = X()
      let xs = [<-x]
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.InvalidMoveOperationError{}))
}

func TestCheckInvalidUseAfterResourceMoveIntoArray(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      resource X {}

      let x <- create X()
      let xs <- [<-x, <-x]
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(err, 2)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.ResourceUseAfterInvalidationError{}))
}

func TestCheckResourceMoveIntoDictionary(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      resource X {}

      let x <- create X()
      let xs <- {"x": <-x}
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
}

func TestCheckInvalidResourceMoveIntoDictionaryMissingMoveOperation(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      resource X {}

      let x <- create X()
      let xs <- {"x": x}
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(err, 2)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.MissingMoveOperationError{}))
}

func TestCheckInvalidNonResourceMoveIntoDictionary(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      struct X {}

      let x = X()
      let xs = {"x": <-x}
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.InvalidMoveOperationError{}))
}

func TestCheckInvalidUseAfterResourceMoveIntoDictionary(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      resource X {}

      let x <- create X()
      let xs <- {
          "x": <-x, 
          "x2": <-x
      }
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(err, 2)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.ResourceUseAfterInvalidationError{}))
}

func TestCheckInvalidUseAfterResourceMoveIntoDictionaryAsKey(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      resource X {}

      let x <- create X()
      let xs <- {<-x: <-x}
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(err, 2)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.ResourceUseAfterInvalidationError{}))
}

func TestCheckInvalidResourceUseAfterMoveInWhileStatement(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      resource X {}

      fun test() {
          let x <- create X()
          while true {
              destroy x
          }
          destroy x
      }
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(err, 3)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.ResourceUseAfterInvalidationError{}))

	Expect(errs[2]).
		To(BeAssignableToTypeOf(&sema.ResourceUseAfterInvalidationError{}))
}

func TestCheckResourceUseInWhileStatement(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      resource X {
          let id: Int
          init(id: Int) {
              self.id = id
          }
      }

      fun test() {
          let x <- create X(id: 1)
          while true {
              x.id
          }
          destroy x
      }
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
}

func TestCheckInvalidResourceUseInWhileStatementAfterDestroy(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      resource X {
          let id: Int
          init(id: Int) {
              self.id = id
          }
      }

      fun test() {
          let x <- create X(id: 1)
          while true {
              x.id
              destroy x
          }
          destroy x
      }
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(err, 4)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.ResourceUseAfterInvalidationError{}))

	Expect(errs[2]).
		To(BeAssignableToTypeOf(&sema.ResourceUseAfterInvalidationError{}))

	Expect(errs[3]).
		To(BeAssignableToTypeOf(&sema.ResourceUseAfterInvalidationError{}))
}

func TestCheckInvalidResourceUseInWhileStatementAfterDestroyAndLoss(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      resource X {}

      fun test() {
          let x <- create X()
          while true {
              destroy x
          }
      }
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(err, 3)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.ResourceUseAfterInvalidationError{}))

	Expect(errs[2]).
		To(BeAssignableToTypeOf(&sema.ResourceLossError{}))
}

func TestCheckInvalidResourceUseInNestedWhileStatementAfterDestroyAndLoss1(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      resource X {
          let id: Int
          init(id: Int) {
              self.id = id
          }
      }

      fun test() {
          let x <- create X(id: 1)
          while true {
              while true {
                  x.id
                  destroy x
              }
          }
      }
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(err, 4)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.ResourceUseAfterInvalidationError{}))

	Expect(errs[2]).
		To(BeAssignableToTypeOf(&sema.ResourceUseAfterInvalidationError{}))

	Expect(errs[3]).
		To(BeAssignableToTypeOf(&sema.ResourceLossError{}))
}

func TestCheckInvalidResourceUseInNestedWhileStatementAfterDestroyAndLoss2(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      resource X {
          let id: Int
          init(id: Int) {
              self.id = id
          }
      }

      fun test() {
          let x <- create X(id: 1)
          while true {
              while true {
                  x.id
              }
              destroy x
          }
      }
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(err, 4)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.ResourceUseAfterInvalidationError{}))

	Expect(errs[2]).
		To(BeAssignableToTypeOf(&sema.ResourceUseAfterInvalidationError{}))

	Expect(errs[3]).
		To(BeAssignableToTypeOf(&sema.ResourceLossError{}))
}

func TestCheckResourceUseInNestedWhileStatement(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      resource X {
          let id: Int
          init(id: Int) {
              self.id = id
          }
      }

      fun test() {
          let x <- create X(id: 1)
          while true {
              while true {
                  x.id
              }
          }
		  destroy x
      }
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
}

func TestCheckInvalidResourceLossThroughReturn(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      resource X {}

      fun test() {
          let x <- create X()
          return
          destroy x
      }
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(err, 2)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.ResourceLossError{}))
}

func TestCheckInvalidResourceLossThroughReturnInIfStatementThrenBranch(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      resource X {}

      fun test(y: Int) {
          let x <- create X()
          if y == 42 {
              return
          }
          destroy x
      }
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(err, 2)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.ResourceLossError{}))
}

func TestCheckInvalidResourceLossThroughReturnInIfStatementBranches(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      resource X {}

      fun test(y: Int) {
          let x <- create X()
          if y == 42 {
              absorb(<-x)
              return
          } else {
              return
          }
          destroy x
      }

      fun absorb(_ x: <-X) {
          destroy x
      }
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(err, 3)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.ResourceLossError{}))

	// TODO: false positive, actually because the statement is dead code:
	//   both branches of the if-statement return

	Expect(errs[2]).
		To(BeAssignableToTypeOf(&sema.ResourceUseAfterInvalidationError{}))
}

func TestCheckResourceWithMoveAndReturnInIfStatementThenAndDestroyInElse(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      resource X {}

      fun test(y: Int) {
          let x <- create X()
          if y == 42 {
              absorb(<-x)
              return
          } else {
              destroy x
          }
      }

      fun absorb(_ x: <-X) {
          destroy x
      }
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
}

