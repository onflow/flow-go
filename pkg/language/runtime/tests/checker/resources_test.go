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
				errs := ExpectCheckerErrors(t, err, 1)

				// TODO: add support for non-Any types in failable downcasting

				assert.IsType(t, &sema.UnsupportedTypeError{}, errs[0])

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
				assert.Nil(t, err)

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
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.MissingMoveAnnotationError{}, errs[0])

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
				assert.Nil(t, err)

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
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.MissingMoveAnnotationError{}, errs[0])

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
				assert.Nil(t, err)

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
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.MissingMoveAnnotationError{}, errs[0])

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

			destructor := ""
			if kind == common.CompositeKindResource {
				destructor = `
                  destroy() {
                      destroy self.t
                  }
                `
			}

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              %[1]s T {}

              %[1]s U {
                  let t: <-T
                  init(t: <-T) {
                      self.t %[2]s t
                  }

                  %[3]s
              }
            `,
				kind.Keyword(),
				kind.TransferOperator(),
				destructor,
			))

			switch kind {
			case common.CompositeKindResource:
				assert.Nil(t, err)

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

			destructor := ""
			if kind == common.CompositeKindResource {
				destructor = `
                  destroy() {
                      destroy self.t
                  }
                `
			}

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              %[1]s T {}

              %[1]s U {
                  let t: T
                  init(t: T) {
                      self.t %[2]s t
                  }

                  %[3]s
              }
            `,
				kind.Keyword(),
				kind.TransferOperator(),
				destructor,
			))

			switch kind {
			case common.CompositeKindResource:
				// NOTE: one missing move annotation error for field, one for parameter

				errs := ExpectCheckerErrors(t, err, 2)

				assert.IsType(t, &sema.MissingMoveAnnotationError{}, errs[0])
				assert.IsType(t, &sema.MissingMoveAnnotationError{}, errs[1])

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
				assert.Nil(t, err)

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

				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.MissingMoveAnnotationError{}, errs[0])

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
				assert.Nil(t, err)

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
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.MissingMoveAnnotationError{}, errs[0])

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
				assert.Nil(t, err)

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
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.MissingMoveAnnotationError{}, errs[0])

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
				assert.Nil(t, err)

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
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.MissingMoveAnnotationError{}, errs[0])

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
				errs := ExpectCheckerErrors(t, err, 2)

				assert.IsType(t, &sema.MissingMoveAnnotationError{}, errs[0])

				// TODO: add support for non-Any types in failable downcasting

				assert.IsType(t, &sema.UnsupportedTypeError{}, errs[1])

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

      fun bar() {
          let x <- foo(x: <-create X())
          destroy x
      }
    `)

	assert.Nil(t, err)

}

func TestCheckImmediateDestroy(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test() {
          destroy create X()
      }
    `)

	assert.Nil(t, err)
}

func TestCheckIndirectDestroy(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test() {
          let x <- create X()
          destroy x
      }
    `)

	assert.Nil(t, err)
}

func TestCheckInvalidResourceCreationWithoutCreate(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      let x <- X()
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.MissingCreateError{}, errs[0])

}

func TestCheckInvalidDestroy(t *testing.T) {

	_, err := ParseAndCheck(t, `
      struct X {}

      fun test() {
          destroy X()
      }
    `)

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

	assert.Nil(t, err)
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

	assert.Nil(t, err)
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

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
	assert.IsType(t, &sema.IncorrectArgumentLabelError{}, errs[1])
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

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.ResourceLossError{}, errs[0])
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

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.ResourceLossError{}, errs[0])
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

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.ResourceLossError{}, errs[0])
	})

	t.Run("ImmediateIndexing", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
            resource Foo {}

            fun test() {
                let x <- [<-create Foo(), <-create Foo()][0]
                destroy x
            }
        `)

		errs := ExpectCheckerErrors(t, err, 2)

		assert.IsType(t, &sema.ResourceLossError{}, errs[0])
		assert.IsType(t, &sema.InvalidNestedMoveError{}, errs[1])
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

		errs := ExpectCheckerErrors(t, err, 2)

		assert.IsType(t, &sema.ResourceLossError{}, errs[0])
		assert.IsType(t, &sema.InvalidNestedMoveError{}, errs[1])
	})
}

func TestCheckResourceReturn(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test(): <-X {
          return <-create X()
      }
    `)

	assert.Nil(t, err)
}

func TestCheckInvalidResourceReturnMissingMove(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test(): <-X {
          return create X()
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.MissingMoveOperationError{}, errs[0])
}

func TestCheckInvalidResourceReturnMissingMoveInvalidReturnType(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test(): Y {
          return create X()
      }
    `)

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
	assert.IsType(t, &sema.MissingMoveOperationError{}, errs[1])
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

	assert.Nil(t, err)
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

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.MissingMoveOperationError{}, errs[0])
}

func TestCheckInvalidResourceArgumentMissingMoveInvalidParameterType(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun foo(_ x: Y) {}

      fun bar() {
          foo(create X())
      }
    `)

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
	assert.IsType(t, &sema.MissingMoveOperationError{}, errs[1])
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

	assert.Nil(t, err)
}

func TestCheckInvalidResourceVariableDeclarationIncorrectTransfer(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      let x = create X()
      let y = x
    `)

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.IncorrectTransferOperationError{}, errs[0])
	assert.IsType(t, &sema.IncorrectTransferOperationError{}, errs[1])
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

func TestCheckInvalidResourceAssignmentTransfer(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test() {
         let x <- create X()
         var x2 <- create X()
         destroy x2
         x2 <- x
      }
    `)

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.InvalidResourceAssignmentError{}, errs[0])
	assert.IsType(t, &sema.ResourceLossError{}, errs[1])
}

func TestCheckInvalidResourceAssignmentIncorrectTransfer(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test() {
        let x <- create X()
        var x2 <- create X()
        destroy x2
        x2 = x
      }
    `)

	errs := ExpectCheckerErrors(t, err, 3)

	assert.IsType(t, &sema.IncorrectTransferOperationError{}, errs[0])
	assert.IsType(t, &sema.InvalidResourceAssignmentError{}, errs[1])
	assert.IsType(t, &sema.ResourceLossError{}, errs[2])
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

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ResourceLossError{}, errs[0])
}

func TestCheckInvalidResourceLossThroughVariableDeclarationAfterCreation(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test() {
          let x <- create X()
          let y <- x
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ResourceLossError{}, errs[0])
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

	errs := ExpectCheckerErrors(t, err, 3)

	assert.IsType(t, &sema.InvalidResourceAssignmentError{}, errs[0])
	assert.IsType(t, &sema.ResourceLossError{}, errs[1])
	assert.IsType(t, &sema.ResourceLossError{}, errs[2])
}

func TestCheckResourceMoveThroughReturn(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test(): <-X {
          let x <- create X()
          return <-x
      }
    `)

	assert.Nil(t, err)
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

	assert.Nil(t, err)
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

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[0])
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

	errs := ExpectCheckerErrors(t, err, 3)

	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[0])

	// NOTE: still two resource losses reported for `y` and `z`

	assert.IsType(t, &sema.ResourceLossError{}, errs[1])

	assert.IsType(t, &sema.ResourceLossError{}, errs[2])
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

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[0])
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

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[0])
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

	assert.Nil(t, err)
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

	assert.Nil(t, err)
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

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[0])

	assert.ElementsMatch(t,
		errs[0].(*sema.ResourceUseAfterInvalidationError).Invalidations,
		[]sema.ResourceInvalidation{
			{
				Kind:     sema.ResourceInvalidationKindMove,
				StartPos: ast.Position{Offset: 165, Line: 9, Column: 23},
				EndPos:   ast.Position{Offset: 165, Line: 9, Column: 23},
			},
			{
				Kind:     sema.ResourceInvalidationKindMove,
				StartPos: ast.Position{Offset: 120, Line: 7, Column: 23},
				EndPos:   ast.Position{Offset: 120, Line: 7, Column: 23},
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

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ResourceLossError{}, errs[0])
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

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[0])
	assert.IsType(t, &sema.ResourceLossError{}, errs[1])
}

func TestCheckResourceMoveIntoArray(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      let x <- create X()
      let xs <- [<-x]
    `)

	assert.Nil(t, err)
}

func TestCheckInvalidResourceMoveIntoArrayMissingMoveOperation(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      let x <- create X()
      let xs <- [x]
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.MissingMoveOperationError{}, errs[0])
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

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[0])
}

func TestCheckResourceMoveIntoDictionary(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      let x <- create X()
      let xs <- {"x": <-x}
    `)

	assert.Nil(t, err)
}

func TestCheckInvalidResourceMoveIntoDictionaryMissingMoveOperation(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      let x <- create X()
      let xs <- {"x": x}
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.MissingMoveOperationError{}, errs[0])
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

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[0])
}

func TestCheckInvalidUseAfterResourceMoveIntoDictionaryAsKey(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      let x <- create X()
      let xs <- {<-x: <-x}
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[0])
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

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[0])
	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[1])
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

	assert.Nil(t, err)
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

	errs := ExpectCheckerErrors(t, err, 3)

	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[0])
	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[1])
	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[2])
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

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[0])
	assert.IsType(t, &sema.ResourceLossError{}, errs[1])
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

	errs := ExpectCheckerErrors(t, err, 3)

	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[0])
	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[1])
	assert.IsType(t, &sema.ResourceLossError{}, errs[2])
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

	errs := ExpectCheckerErrors(t, err, 3)

	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[0])
	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[1])
	assert.IsType(t, &sema.ResourceLossError{}, errs[2])
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

	assert.Nil(t, err)
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

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.ResourceLossError{}, errs[0])
	assert.IsType(t, &sema.UnreachableStatementError{}, errs[1])
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

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ResourceLossError{}, errs[0])
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

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.ResourceLossError{}, errs[0])
	assert.IsType(t, &sema.UnreachableStatementError{}, errs[1])
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

	assert.Nil(t, err)
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

	assert.Nil(t, err)
}

func TestCheckResourceNesting(t *testing.T) {

	compositeKindPossibilities := []common.CompositeKind{
		common.CompositeKindResource,
		common.CompositeKindStructure,
	}
	interfacePossibilities := []bool{true, false}

	for _, innerCompositeKind := range compositeKindPossibilities {
		for _, innerIsInterface := range interfacePossibilities {
			for _, outerCompositeKind := range compositeKindPossibilities {
				for _, outerIsInterface := range interfacePossibilities {

					testName := fmt.Sprintf(
						"%s %v/%s %v",
						innerCompositeKind.Keyword(),
						innerIsInterface,
						outerCompositeKind.Keyword(),
						outerIsInterface,
					)

					t.Run(testName, func(t *testing.T) {
						testResourceNesting(
							t,
							innerCompositeKind,
							innerIsInterface,
							outerCompositeKind,
							outerIsInterface,
						)
					})
				}
			}
		}
	}
}

func testResourceNesting(
	t *testing.T,
	innerCompositeKind common.CompositeKind,
	innerIsInterface bool,
	outerCompositeKind common.CompositeKind,
	outerIsInterface bool,
) {
	innerInterfaceKeyword := ""
	if innerIsInterface {
		innerInterfaceKeyword = "interface"
	}

	outerInterfaceKeyword := ""
	if outerIsInterface {
		outerInterfaceKeyword = "interface"
	}

	// Prepare the initializer, if needed.
	// `outerCompositeKind` is the container composite kind.
	// If it is concrete, i.e. not an interface, it needs an initializer.

	initializer := ""
	if !outerIsInterface {
		initializer = fmt.Sprintf(
			`
              init(t: %[1]sT) {
                  self.t %[2]s t
              }
            `,
			innerCompositeKind.Annotation(),
			innerCompositeKind.TransferOperator(),
		)
	}

	destructor := ""
	if !outerIsInterface &&
		outerCompositeKind == common.CompositeKindResource &&
		innerCompositeKind == common.CompositeKindResource {

		destructor = `
          destroy() {
              destroy self.t
          }
        `
	}

	// Prepare the full program defining an empty composite,
	// and a second composite which contains the first

	program := fmt.Sprintf(
		`
          %[1]s %[2]s T {}

          %[3]s %[4]s U {
              let t: %[5]sT
              %[6]s
              %[7]s
          }
        `,
		innerCompositeKind.Keyword(),
		innerInterfaceKeyword,
		outerCompositeKind.Keyword(),
		outerInterfaceKeyword,
		innerCompositeKind.Annotation(),
		initializer,
		destructor,
	)

	_, err := ParseAndCheck(t, program)

	// TODO: add support for non-structure / non-resource declarations

	switch outerCompositeKind {
	case common.CompositeKindStructure:
		switch innerCompositeKind {
		case common.CompositeKindStructure:
			assert.Nil(t, err)
		case common.CompositeKindResource:
			errs := ExpectCheckerErrors(t, err, 1)
			assert.IsType(t, &sema.InvalidResourceFieldError{}, errs[0])
		}

	case common.CompositeKindResource:
		assert.Nil(t, err)
	}
}

// TestCheckResourceInterfaceConformance tests the check
// of conformance of resources to resource interfaces.
//
func TestCheckResourceInterfaceConformance(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource interface X {
          fun test()
      }

      resource Y: X {
          fun test() {}
      }
    `)

	assert.Nil(t, err)
}

// TestCheckInvalidResourceInterfaceConformance tests the check
// of conformance of resources to resource interfaces.
//
func TestCheckInvalidResourceInterfaceConformance(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource interface X {
          fun test()
      }

      resource Y: X {}
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ConformanceError{}, errs[0])
}

// TestCheckResourceInterfaceUseAsType tests if a resource interface
// can be used as a type, and if a resource is a subtype of the interface
// if it conforms to it.
//
func TestCheckResourceInterfaceUseAsType(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource interface X {}

      resource Y: X {}

      let x: <-X <- create Y()
    `)

	assert.Nil(t, err)
}

func TestCheckResourceArrayIndexing(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource Foo {
          var bar: Int
          init(bar: Int) {
              self.bar = bar
          }
      }

      fun test(): Int {
        let foo <- create Foo(bar: 1)
        let foos <- [<-[<-foo]]
        let bar = foos[0][0].bar
        destroy foos
        return bar
      }
    `)

	assert.Nil(t, err)
}

func TestCheckInvalidResourceLossReturnResourceAndMemberAccess(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {
          let id: Int

          init(id: Int) {
              self.id = id
          }
      }

      fun test(): Int {
          return createX().id
      }

      fun createX(): <-X {
          return <-create X(id: 1)
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ResourceLossError{}, errs[0])
}

func TestCheckInvalidResourceLossAfterMoveThroughArrayIndexing(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test() {
          let xs <- [<-create X()]
          foo(x: <-xs[0])
      }

      fun foo(x: <-X) {
          destroy x
      }
    `)

	errs := ExpectCheckerErrors(t, err, 2)
	assert.IsType(t, &sema.InvalidNestedMoveError{}, errs[0])
	assert.IsType(t, &sema.ResourceLossError{}, errs[1])
}

func TestCheckInvalidResourceLossThroughFunctionResultAccess(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource Foo {
          var bar: Int
          init(bar: Int) {
              self.bar = bar
          }
      }

      fun createFoo(): <-Foo {
          return <- create Foo(bar: 1)
      }

      fun test(): Int {
          return createFoo().bar
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ResourceLossError{}, errs[0])
}

// TestCheckResourceInterfaceDestruction tests if resources
// can be passed to resource interface parameters,
// and if resource interfaces can be destroyed.
//
func TestCheckResourceInterfaceDestruction(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource interface X {}

      resource Y: X {}

      fun foo(x: <-X) {
          destroy x
      }

      fun bar() {
          foo(x: <-create Y())
      }
    `)

	assert.Nil(t, err)
}

// TestCheckInvalidResourceFieldMoveThroughVariableDeclaration tests if resources nested
// as a field in another resource cannot be moved out of the containing resource through
// a variable declaration. This would partially invalidate the containing resource
//
func TestCheckInvalidResourceFieldMoveThroughVariableDeclaration(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource Foo {}

      resource Bar {
          let foo: <-Foo

          init(foo: <-Foo) {
              self.foo <- foo
          }

          destroy() {
              destroy self.foo
          }
      }

      fun test(): <-[Foo] {
          let foo <- create Foo()
          let bar <- create Bar(foo: <-foo)
          let foo2 <- bar.foo
          let foo3 <- bar.foo
          destroy bar
          return <-[<-foo2, <-foo3]
      }
    `)

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.InvalidNestedMoveError{}, errs[0])
	assert.IsType(t, &sema.InvalidNestedMoveError{}, errs[1])
}

// TestCheckInvalidResourceFieldMoveThroughParameter tests if resources nested
// as a field in another resource cannot be moved out of the containing resource
// by passing the field as an argument to a function. This would partially invalidate
// the containing resource
//
func TestCheckInvalidResourceFieldMoveThroughParameter(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource Foo {}

      resource Bar {
          let foo: <-Foo

          init(foo: <-Foo) {
              self.foo <- foo
          }

          destroy() {
              destroy self.foo
          }
      }

      fun identity(_ foo: <-Foo): <-Foo {
          return <-foo
      }

      fun test(): <-[Foo] {
          let foo <- create Foo()
          let bar <- create Bar(foo: <-foo)
          let foo2 <- identity(<-bar.foo)
          let foo3 <- identity(<-bar.foo)
          destroy bar
          return <-[<-foo2, <-foo3]
      }
    `)

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.InvalidNestedMoveError{}, errs[0])
	assert.IsType(t, &sema.InvalidNestedMoveError{}, errs[1])
}
