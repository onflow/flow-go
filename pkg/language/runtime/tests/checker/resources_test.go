package checker

import (
	"fmt"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/common"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/pkg/language/runtime/tests/utils"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCheckFailableDowncastingWithMoveAnnotation(t *testing.T) {
	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
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

				errs := ExpectCheckerErrors(t, err, 2)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

				// TODO: add support for non-Any types in failable downcasting

				assert.IsType(t, &sema.UnsupportedTypeError{}, errs[1])

			case common.CompositeKindContract:

				// TODO: add support for contracts

				errs := ExpectCheckerErrors(t, err, 3)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

				assert.IsType(t, &sema.InvalidMoveAnnotationError{}, errs[1])

				// TODO: add support for non-Any types in failable downcasting

				assert.IsType(t, &sema.UnsupportedTypeError{}, errs[2])

			case common.CompositeKindStructure:

				errs := ExpectCheckerErrors(t, err, 2)

				assert.IsType(t, &sema.InvalidMoveAnnotationError{}, errs[0])

				// TODO: add support for non-Any types in failable downcasting

				assert.IsType(t, &sema.UnsupportedTypeError{}, errs[1])
			}
		})
	}
}

func TestCheckFunctionDeclarationParameterWithMoveAnnotation(t *testing.T) {
	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
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

				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

			case common.CompositeKindContract:

				// TODO: add support for contracts

				errs := ExpectCheckerErrors(t, err, 2)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

				assert.IsType(t, &sema.InvalidMoveAnnotationError{}, errs[1])

			case common.CompositeKindStructure:

				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.InvalidMoveAnnotationError{}, errs[0])
			}
		})
	}
}

func TestCheckFunctionDeclarationParameterWithoutMoveAnnotation(t *testing.T) {
	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
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

				errs := ExpectCheckerErrors(t, err, 2)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

				assert.IsType(t, &sema.MissingMoveAnnotationError{}, errs[1])

			case common.CompositeKindContract:

				// TODO: add support for contracts

				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

			case common.CompositeKindStructure:

				assert.Nil(t, err)
			}
		})
	}
}

func TestCheckFunctionDeclarationReturnTypeWithMoveAnnotation(t *testing.T) {
	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
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

				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

			case common.CompositeKindContract:

				// TODO: add support for contracts

				errs := ExpectCheckerErrors(t, err, 2)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

				assert.IsType(t, &sema.InvalidMoveAnnotationError{}, errs[1])

			case common.CompositeKindStructure:

				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.InvalidMoveAnnotationError{}, errs[0])
			}
		})
	}
}

func TestCheckFunctionDeclarationReturnTypeWithoutMoveAnnotation(t *testing.T) {
	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
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

				errs := ExpectCheckerErrors(t, err, 2)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

				assert.IsType(t, &sema.MissingMoveAnnotationError{}, errs[1])

			case common.CompositeKindContract:

				// TODO: add support for contracts

				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

			case common.CompositeKindStructure:

				assert.Nil(t, err)
			}
		})
	}
}

func TestCheckVariableDeclarationWithMoveAnnotation(t *testing.T) {
	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
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

				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

			case common.CompositeKindContract:

				// TODO: add support for contracts

				errs := ExpectCheckerErrors(t, err, 2)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

				assert.IsType(t, &sema.InvalidMoveAnnotationError{}, errs[1])

			case common.CompositeKindStructure:

				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.InvalidMoveAnnotationError{}, errs[0])
			}
		})
	}
}

func TestCheckVariableDeclarationWithoutMoveAnnotation(t *testing.T) {
	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
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

				errs := ExpectCheckerErrors(t, err, 2)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

				assert.IsType(t, &sema.MissingMoveAnnotationError{}, errs[1])

			case common.CompositeKindContract:

				// TODO: add support for contracts

				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

			case common.CompositeKindStructure:

				assert.Nil(t, err)
			}
		})
	}
}

func TestCheckFieldDeclarationWithMoveAnnotation(t *testing.T) {
	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
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

				errs := ExpectCheckerErrors(t, err, 2)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[1])

			case common.CompositeKindContract:

				// TODO: add support for contracts

				errs := ExpectCheckerErrors(t, err, 4)

				// NOTE: one invalid move annotation error for field, one for parameter

				assert.IsType(t, &sema.InvalidMoveAnnotationError{}, errs[0])

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[1])

				assert.IsType(t, &sema.InvalidMoveAnnotationError{}, errs[2])

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[3])

			case common.CompositeKindStructure:

				errs := ExpectCheckerErrors(t, err, 2)

				// NOTE: one invalid move annotation error for field, one for parameter

				assert.IsType(t, &sema.InvalidMoveAnnotationError{}, errs[0])

				assert.IsType(t, &sema.InvalidMoveAnnotationError{}, errs[1])
			}
		})
	}
}

func TestCheckFieldDeclarationWithoutMoveAnnotation(t *testing.T) {
	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
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

				errs := ExpectCheckerErrors(t, err, 4)

				assert.IsType(t, &sema.MissingMoveAnnotationError{}, errs[0])

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[1])

				assert.IsType(t, &sema.MissingMoveAnnotationError{}, errs[2])

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[3])

			case common.CompositeKindContract:

				// TODO: add support for contracts

				errs := ExpectCheckerErrors(t, err, 2)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[1])

			case common.CompositeKindStructure:

				assert.Nil(t, err)
			}
		})
	}
}

func TestCheckFunctionExpressionParameterWithMoveAnnotation(t *testing.T) {
	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
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

				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

			case common.CompositeKindContract:

				// TODO: add support for contracts

				errs := ExpectCheckerErrors(t, err, 2)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

				assert.IsType(t, &sema.InvalidMoveAnnotationError{}, errs[1])

			case common.CompositeKindStructure:

				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.InvalidMoveAnnotationError{}, errs[0])
			}
		})
	}
}

func TestCheckFunctionExpressionParameterWithoutMoveAnnotation(t *testing.T) {
	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
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

				errs := ExpectCheckerErrors(t, err, 2)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

				assert.IsType(t, &sema.MissingMoveAnnotationError{}, errs[1])

			case common.CompositeKindContract:

				// TODO: add support for contracts

				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

			case common.CompositeKindStructure:

				assert.Nil(t, err)
			}
		})
	}
}

func TestCheckFunctionExpressionReturnTypeWithMoveAnnotation(t *testing.T) {
	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
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

				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

			case common.CompositeKindContract:

				// TODO: add support for contracts

				errs := ExpectCheckerErrors(t, err, 2)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

				assert.IsType(t, &sema.InvalidMoveAnnotationError{}, errs[1])

			case common.CompositeKindStructure:

				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.InvalidMoveAnnotationError{}, errs[0])
			}
		})
	}
}

func TestCheckFunctionExpressionReturnTypeWithoutMoveAnnotation(t *testing.T) {
	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
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

				errs := ExpectCheckerErrors(t, err, 2)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

				assert.IsType(t, &sema.MissingMoveAnnotationError{}, errs[1])

			case common.CompositeKindContract:

				// TODO: add support for contracts

				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

			case common.CompositeKindStructure:

				assert.Nil(t, err)
			}
		})
	}
}

func TestCheckFunctionTypeParameterWithMoveAnnotation(t *testing.T) {
	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
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

				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

			case common.CompositeKindContract:

				// TODO: add support for contracts

				errs := ExpectCheckerErrors(t, err, 2)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

				assert.IsType(t, &sema.InvalidMoveAnnotationError{}, errs[1])

			case common.CompositeKindStructure:

				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.InvalidMoveAnnotationError{}, errs[0])
			}
		})
	}
}

func TestCheckFunctionTypeParameterWithoutMoveAnnotation(t *testing.T) {
	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
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

				errs := ExpectCheckerErrors(t, err, 2)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

				assert.IsType(t, &sema.MissingMoveAnnotationError{}, errs[1])

			case common.CompositeKindContract:

				// TODO: add support for contracts

				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

			case common.CompositeKindStructure:

				assert.Nil(t, err)
			}
		})
	}
}

func TestCheckFunctionTypeReturnTypeWithMoveAnnotation(t *testing.T) {
	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
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

				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

			case common.CompositeKindContract:

				// TODO: add support for contracts

				errs := ExpectCheckerErrors(t, err, 2)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

				assert.IsType(t, &sema.InvalidMoveAnnotationError{}, errs[1])

			case common.CompositeKindStructure:

				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.InvalidMoveAnnotationError{}, errs[0])
			}
		})
	}
}

func TestCheckFunctionTypeReturnTypeWithoutMoveAnnotation(t *testing.T) {
	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
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

				errs := ExpectCheckerErrors(t, err, 2)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

				assert.IsType(t, &sema.MissingMoveAnnotationError{}, errs[1])

			case common.CompositeKindContract:

				// TODO: add support for contracts

				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

			case common.CompositeKindStructure:

				assert.Nil(t, err)
			}
		})
	}
}

func TestCheckFailableDowncastingWithoutMoveAnnotation(t *testing.T) {
	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
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

				errs := ExpectCheckerErrors(t, err, 3)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

				assert.IsType(t, &sema.MissingMoveAnnotationError{}, errs[1])

				// TODO: add support for non-Any types in failable downcasting

				assert.IsType(t, &sema.UnsupportedTypeError{}, errs[2])

			case common.CompositeKindContract:

				// TODO: add support for contracts

				errs := ExpectCheckerErrors(t, err, 2)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

				// TODO: add support for non-Any types in failable downcasting

				assert.IsType(t, &sema.UnsupportedTypeError{}, errs[1])

			case common.CompositeKindStructure:

				// TODO: add support for non-Any types in failable downcasting

				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.UnsupportedTypeError{}, errs[0])
			}
		})
	}
}

func TestCheckUnaryMove(t *testing.T) {

	_, err := ParseAndCheck(t, `
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

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

}

func TestCheckImmediateDestroy(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test() {
          destroy create X()
      }
	`)

	// TODO: add create expression once supported
	// TODO: add support for resources

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
}

func TestCheckIndirectDestroy(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test() {
          let x <- create X()
          destroy x
      }
	`)

	// TODO: add create expression once supported
	// TODO: add support for resources

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
}

func TestCheckInvalidResourceCreationWithoutCreate(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

	  let x <- X()
	`)

	// TODO: add create expression once supported
	// TODO: add support for resources

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

	assert.IsType(t, &sema.MissingCreateError{}, errs[1])

}

func TestCheckInvalidDestroy(t *testing.T) {

	_, err := ParseAndCheck(t, `
      struct X {}

      fun test() {
          destroy X()
      }
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidDestructionError{}, errs[0])
}

func TestCheckUnaryCreateAndDestroy(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test() {
          var x <- create X()
          destroy x
      }
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
}

func TestCheckUnaryCreateAndDestroyWithInitializer(t *testing.T) {

	_, err := ParseAndCheck(t, `
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

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
}

func TestCheckInvalidUnaryCreateAndDestroyWithWrongInitializerArguments(t *testing.T) {

	_, err := ParseAndCheck(t, `
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

	errs := ExpectCheckerErrors(t, err, 3)

	assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

	assert.IsType(t, &sema.TypeMismatchError{}, errs[1])

	assert.IsType(t, &sema.IncorrectArgumentLabelError{}, errs[2])
}

func TestCheckInvalidUnaryCreateStruct(t *testing.T) {

	_, err := ParseAndCheck(t, `
      struct X {}

      fun test() {
          create X()
      }
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidConstructionError{}, errs[0])
}

func TestCheckInvalidResourceLoss(t *testing.T) {
	t.Run("UnassignedResource", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
			resource X {}
			
			fun test() {
				create X()
			}
		`)

		// TODO: add support for resources

		errs := ExpectCheckerErrors(t, err, 2)

		assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

		assert.IsType(t, &sema.ResourceLossError{}, errs[1])
	})

	t.Run("ImmediateMemberAccess", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
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

		errs := ExpectCheckerErrors(t, err, 2)

		assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

		assert.IsType(t, &sema.ResourceLossError{}, errs[1])
	})

	t.Run("ImmediateMemberAccessFunctionInvocation", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
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

		errs := ExpectCheckerErrors(t, err, 2)

		assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

		assert.IsType(t, &sema.ResourceLossError{}, errs[1])
	})

	t.Run("ImmediateIndexing", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
			resource Foo {}

			fun test() {
				let x <- [<-create Foo(), <-create Foo()][0]
                destroy x
			}
		`)

		// TODO: add support for resources

		errs := ExpectCheckerErrors(t, err, 2)

		assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

		assert.IsType(t, &sema.ResourceLossError{}, errs[1])
	})

	t.Run("ImmediateIndexingFunctionInvocation", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
			resource Foo {}

			fun test() {
				let x <- makeFoos()[0]
                destroy x
			}

            fun makeFoos(): <-[Foo] {
                return <-[
                    <-create Foo(),
                    <-create Foo()
                ]
            }
		`)

		// TODO: add support for resources

		errs := ExpectCheckerErrors(t, err, 2)

		assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

		assert.IsType(t, &sema.ResourceLossError{}, errs[1])
	})
}

func TestCheckResourceReturn(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test(): <-X {
          return <-create X()
      }
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
}

func TestCheckInvalidResourceReturnMissingMove(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test(): <-X {
          return create X()
      }
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

	assert.IsType(t, &sema.MissingMoveOperationError{}, errs[1])
}

func TestCheckInvalidResourceReturnMissingMoveInvalidReturnType(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test(): Y {
          return create X()
      }
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(t, err, 3)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[0])

	assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[1])

	assert.IsType(t, &sema.MissingMoveOperationError{}, errs[2])
}

func TestCheckInvalidNonResourceReturnWithMove(t *testing.T) {

	_, err := ParseAndCheck(t, `
      struct X {}

      fun test(): X {
          return <-X()
      }
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidMoveOperationError{}, errs[0])
}

func TestCheckResourceArgument(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun foo(_ x: <-X) {
          destroy x
      }

      fun bar() {
          foo(<-create X())
      }
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
}

func TestCheckInvalidResourceArgumentMissingMove(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun foo(_ x: <-X) {
          destroy x
      }

      fun bar() {
          foo(create X())
      }
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

	assert.IsType(t, &sema.MissingMoveOperationError{}, errs[1])
}

func TestCheckInvalidResourceArgumentMissingMoveInvalidParameterType(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun foo(_ x: Y) {}

      fun bar() {
          foo(create X())
      }
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(t, err, 3)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[0])

	assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[1])

	assert.IsType(t, &sema.MissingMoveOperationError{}, errs[2])
}

func TestCheckInvalidNonResourceArgumentWithMove(t *testing.T) {

	_, err := ParseAndCheck(t, `
      struct X {}

      fun foo(_ x: X) {}

      fun bar() {
          foo(<-X())
      }
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidMoveOperationError{}, errs[0])
}

func TestCheckResourceVariableDeclarationTransfer(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      let x <- create X()
      let y <- x
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
}

func TestCheckInvalidResourceVariableDeclarationIncorrectTransfer(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      let x = create X()
      let y = x
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(t, err, 3)

	assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

	assert.IsType(t, &sema.IncorrectTransferOperationError{}, errs[1])

	assert.IsType(t, &sema.IncorrectTransferOperationError{}, errs[2])
}

func TestCheckInvalidNonResourceVariableDeclarationMoveTransfer(t *testing.T) {

	_, err := ParseAndCheck(t, `
      struct X {}

      let x = X()
      let y <- x
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.IncorrectTransferOperationError{}, errs[0])
}

func TestCheckResourceAssignmentTransfer(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      let x <- create X()

      fun test() {
         var x2 <- create X()
         destroy x2
         x2 <- x
      }
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
}

func TestCheckInvalidResourceAssignmentIncorrectTransfer(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      let x <- create X()

      fun test() {
        var x2 <- create X()
        destroy x2
        x2 = x
      }
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

	assert.IsType(t, &sema.IncorrectTransferOperationError{}, errs[1])
}

func TestCheckInvalidNonResourceAssignmentMoveTransfer(t *testing.T) {

	_, err := ParseAndCheck(t, `
      struct X {}

      let x = X()
      fun test() {
        var x2 = X()
        x2 <- x
      }
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.IncorrectTransferOperationError{}, errs[0])
}

func TestCheckInvalidResourceLossThroughVariableDeclaration(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test() {
        let x <- create X()
      }
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

	assert.IsType(t, &sema.ResourceLossError{}, errs[1])
}

func TestCheckInvalidResourceLossThroughVariableDeclarationAfterCreation(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test() {
          let x <- create X()
          let y <- x
      }
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

	assert.IsType(t, &sema.ResourceLossError{}, errs[1])
}

func TestCheckInvalidResourceLossThroughAssignment(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test() {
          var x <- create X()
          let y <- create X()
          x <- y
      }
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

	assert.IsType(t, &sema.ResourceLossError{}, errs[1])
}

func TestCheckResourceMoveThroughReturn(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test(): <-X {
          let x <- create X()
          return <-x
      }
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
}

func TestCheckResourceMoveThroughArgumentPassing(t *testing.T) {

	_, err := ParseAndCheck(t, `
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

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
}

func TestCheckInvalidResourceUseAfterMoveToFunction(t *testing.T) {

	_, err := ParseAndCheck(t, `
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

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[1])
}

func TestCheckInvalidResourceUseAfterMoveToVariable(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test() {
          let x <- create X()
          let y <- x
          let z <- x
      }
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(t, err, 4)

	assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[1])

	// NOTE: still two resource losses reported for `y` and `z`

	assert.IsType(t, &sema.ResourceLossError{}, errs[2])

	assert.IsType(t, &sema.ResourceLossError{}, errs[3])
}

func TestCheckInvalidResourceFieldUseAfterMoveToVariable(t *testing.T) {

	_, err := ParseAndCheck(t, `
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

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[1])
}

func TestCheckResourceUseAfterMoveInIfStatementThenBranch(t *testing.T) {

	_, err := ParseAndCheck(t, `
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

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[1])
}

func TestCheckResourceUseInIfStatement(t *testing.T) {

	_, err := ParseAndCheck(t, `
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

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
}

func TestCheckResourceUseInNestedIfStatement(t *testing.T) {

	_, err := ParseAndCheck(t, `
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

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
}

////

func TestCheckInvalidResourceUseAfterIfStatement(t *testing.T) {

	_, err := ParseAndCheck(t, `
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

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[1])

	assert.ElementsMatch(t,
		errs[1].(*sema.ResourceUseAfterInvalidationError).Invalidations,
		[]sema.ResourceInvalidation{
			{
				Kind: sema.ResourceInvalidationKindMove,
				Pos:  ast.Position{Offset: 165, Line: 9, Column: 23},
			},
			{
				Kind: sema.ResourceInvalidationKindMove,
				Pos:  ast.Position{Offset: 120, Line: 7, Column: 23},
			},
		},
	)
}

func TestCheckInvalidResourceLossAfterDestroyInIfStatementThenBranch(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test() {
          let x <- create X()
          if 1 > 2 {
             destroy x
          }
      }
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

	assert.IsType(t, &sema.ResourceLossError{}, errs[1])
}

func TestCheckInvalidResourceLossAndUseAfterDestroyInIfStatementThenBranch(t *testing.T) {

	_, err := ParseAndCheck(t, `
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

	errs := ExpectCheckerErrors(t, err, 3)

	assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[1])

	assert.IsType(t, &sema.ResourceLossError{}, errs[2])
}

func TestCheckResourceMoveIntoArray(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      let x <- create X()
      let xs <- [<-x]
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
}

func TestCheckInvalidResourceMoveIntoArrayMissingMoveOperation(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      let x <- create X()
      let xs <- [x]
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

	assert.IsType(t, &sema.MissingMoveOperationError{}, errs[1])
}

func TestCheckInvalidNonResourceMoveIntoArray(t *testing.T) {

	_, err := ParseAndCheck(t, `
      struct X {}

      let x = X()
      let xs = [<-x]
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidMoveOperationError{}, errs[0])
}

func TestCheckInvalidUseAfterResourceMoveIntoArray(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      let x <- create X()
      let xs <- [<-x, <-x]
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[1])
}

func TestCheckResourceMoveIntoDictionary(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      let x <- create X()
      let xs <- {"x": <-x}
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
}

func TestCheckInvalidResourceMoveIntoDictionaryMissingMoveOperation(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      let x <- create X()
      let xs <- {"x": x}
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

	assert.IsType(t, &sema.MissingMoveOperationError{}, errs[1])
}

func TestCheckInvalidNonResourceMoveIntoDictionary(t *testing.T) {

	_, err := ParseAndCheck(t, `
      struct X {}

      let x = X()
      let xs = {"x": <-x}
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidMoveOperationError{}, errs[0])
}

func TestCheckInvalidUseAfterResourceMoveIntoDictionary(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      let x <- create X()
      let xs <- {
          "x": <-x, 
          "x2": <-x
      }
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[1])
}

func TestCheckInvalidUseAfterResourceMoveIntoDictionaryAsKey(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      let x <- create X()
      let xs <- {<-x: <-x}
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[1])
}

func TestCheckInvalidResourceUseAfterMoveInWhileStatement(t *testing.T) {

	_, err := ParseAndCheck(t, `
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

	errs := ExpectCheckerErrors(t, err, 3)

	assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[1])

	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[2])
}

func TestCheckResourceUseInWhileStatement(t *testing.T) {

	_, err := ParseAndCheck(t, `
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

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
}

func TestCheckInvalidResourceUseInWhileStatementAfterDestroy(t *testing.T) {

	_, err := ParseAndCheck(t, `
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

	errs := ExpectCheckerErrors(t, err, 4)

	assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[1])

	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[2])

	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[3])
}

func TestCheckInvalidResourceUseInWhileStatementAfterDestroyAndLoss(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test() {
          let x <- create X()
          while true {
              destroy x
          }
      }
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(t, err, 3)

	assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[1])

	assert.IsType(t, &sema.ResourceLossError{}, errs[2])
}

func TestCheckInvalidResourceUseInNestedWhileStatementAfterDestroyAndLoss1(t *testing.T) {

	_, err := ParseAndCheck(t, `
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

	errs := ExpectCheckerErrors(t, err, 4)

	assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[1])

	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[2])

	assert.IsType(t, &sema.ResourceLossError{}, errs[3])
}

func TestCheckInvalidResourceUseInNestedWhileStatementAfterDestroyAndLoss2(t *testing.T) {

	_, err := ParseAndCheck(t, `
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

	errs := ExpectCheckerErrors(t, err, 4)

	assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[1])

	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[2])

	assert.IsType(t, &sema.ResourceLossError{}, errs[3])
}

func TestCheckResourceUseInNestedWhileStatement(t *testing.T) {

	_, err := ParseAndCheck(t, `
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

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
}

func TestCheckInvalidResourceLossThroughReturn(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test() {
          let x <- create X()
          return
          destroy x
      }
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

	assert.IsType(t, &sema.ResourceLossError{}, errs[1])
}

func TestCheckInvalidResourceLossThroughReturnInIfStatementThrenBranch(t *testing.T) {

	_, err := ParseAndCheck(t, `
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

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

	assert.IsType(t, &sema.ResourceLossError{}, errs[1])
}

func TestCheckInvalidResourceLossThroughReturnInIfStatementBranches(t *testing.T) {

	_, err := ParseAndCheck(t, `
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

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

	assert.IsType(t, &sema.ResourceLossError{}, errs[1])
}

func TestCheckResourceWithMoveAndReturnInIfStatementThenAndDestroyInElse(t *testing.T) {

	_, err := ParseAndCheck(t, `
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

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
}

func TestCheckResourceWithMoveAndReturnInIfStatementThenBranch(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test(y: Int) {
          let x <- create X()
          if y == 42 {
              absorb(<-x)
              return
          }
          destroy x
      }

      fun absorb(_ x: <-X) {
          destroy x
      }
	`)

	// TODO: add support for resources

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
}
