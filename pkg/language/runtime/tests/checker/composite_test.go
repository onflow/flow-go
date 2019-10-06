package checker

import (
	"fmt"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/common"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema/self_field_analyzer"
	. "github.com/dapperlabs/flow-go/pkg/language/runtime/tests/utils"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCheckInvalidCompositeRedeclaringType(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              %s Int {}
            `, kind.Keyword()))

			// TODO: add support for non-structure declarations

			expectedErrorCount := 1
			if kind != common.CompositeKindStructure {
				expectedErrorCount += 1
			}

			errs := ExpectCheckerErrors(t, err, expectedErrorCount)

			assert.IsType(t, &sema.RedeclarationError{}, errs[0])

			if kind != common.CompositeKindStructure {
				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[1])
			}
		})
	}
}

func TestCheckComposite(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              %s Test {
                  pub(set) var foo: Int

                  init(foo: Int) {
                      self.foo = foo
                  }

                  pub fun getFoo(): Int {
                      return self.foo
                  }
              }
	        `, kind.Keyword()))

			// TODO: add support for non-structure declarations

			if kind == common.CompositeKindStructure {
				assert.Nil(t, err)
			} else {
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
			}
		})
	}
}

func TestCheckInitializerName(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              %s Test {
                  init() {}
              }
	        `, kind.Keyword()))

			// TODO: add support for non-structure declarations

			if kind == common.CompositeKindStructure {
				assert.Nil(t, err)
			} else {
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
			}
		})
	}
}

func TestCheckInvalidInitializerName(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              %s Test {
                  initializer() {}
              }
	        `, kind.Keyword()))

			// TODO: add support for non-structure declarations

			expectedErrorCount := 1
			if kind != common.CompositeKindStructure {
				expectedErrorCount += 1
			}

			errs := ExpectCheckerErrors(t, err, expectedErrorCount)

			assert.IsType(t, &sema.InvalidInitializerNameError{}, errs[0])

			if kind != common.CompositeKindStructure {
				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[1])
			}
		})
	}
}

func TestCheckInvalidCompositeFieldName(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              %s Test {
                  let init: Int
              }
	        `, kind.Keyword()))

			// TODO: add support for non-structure declarations

			expectedErrorCount := 2
			if kind != common.CompositeKindStructure {
				expectedErrorCount += 1
			}

			errs := ExpectCheckerErrors(t, err, expectedErrorCount)

			assert.IsType(t, &sema.InvalidNameError{}, errs[0])

			assert.IsType(t, &sema.MissingInitializerError{}, errs[1])

			if kind != common.CompositeKindStructure {
				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[2])
			}
		})
	}
}

func TestCheckInvalidCompositeFunctionName(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              %s Test {
                  fun init() {}
              }
	        `, kind.Keyword()))

			// TODO: add support for non-structure declarations

			expectedErrorCount := 1
			if kind != common.CompositeKindStructure {
				expectedErrorCount += 1
			}

			errs := ExpectCheckerErrors(t, err, expectedErrorCount)

			assert.IsType(t, &sema.InvalidNameError{}, errs[0])

			if kind != common.CompositeKindStructure {
				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[1])
			}
		})
	}
}

func TestCheckInvalidCompositeRedeclaringFields(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              %s Test {
                  let x: Int
                  let x: Int
              }
	        `, kind.Keyword()))

			// TODO: add support for non-structure declarations

			expectedErrorCount := 2
			if kind != common.CompositeKindStructure {
				expectedErrorCount += 1
			}

			errs := ExpectCheckerErrors(t, err, expectedErrorCount)

			assert.IsType(t, &sema.RedeclarationError{}, errs[0])

			assert.IsType(t, &sema.MissingInitializerError{}, errs[1])

			if kind != common.CompositeKindStructure {
				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[2])
			}
		})
	}
}

func TestCheckInvalidCompositeRedeclaringFunctions(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              %s Test {
                  fun x() {}
                  fun x() {}
              }
	        `, kind.Keyword()))

			// TODO: add support for non-structure declarations

			expectedErrorCount := 1
			if kind != common.CompositeKindStructure {
				expectedErrorCount += 1
			}

			errs := ExpectCheckerErrors(t, err, expectedErrorCount)

			assert.IsType(t, &sema.RedeclarationError{}, errs[0])

			if kind != common.CompositeKindStructure {
				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[1])
			}
		})
	}
}

func TestCheckInvalidCompositeRedeclaringFieldsAndFunctions(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              %s Test {
                  let x: Int
                  fun x() {}
              }
	        `, kind.Keyword()))

			// TODO: add support for non-structure declarations

			expectedErrorCount := 2
			if kind != common.CompositeKindStructure {
				expectedErrorCount += 1
			}

			errs := ExpectCheckerErrors(t, err, expectedErrorCount)
			assert.IsType(t, &sema.RedeclarationError{}, errs[0])

			assert.IsType(t, &sema.MissingInitializerError{}, errs[1])

			if kind != common.CompositeKindStructure {
				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[2])
			}
		})
	}
}

func TestCheckCompositeFieldsAndFunctions(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              %s Test {
                  let x: Int

                  init() {
                      self.x = 1
                  }

                  fun y() {}
              }
	        `, kind.Keyword()))

			// TODO: add support for non-structure declarations

			if kind == common.CompositeKindStructure {
				assert.Nil(t, err)
			} else {
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
			}
		})
	}
}

func TestCheckInvalidCompositeFieldType(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              %s Test {
                  let x: X
              }
	        `, kind.Keyword()))

			// TODO: add support for non-structure declarations

			expectedErrorCount := 2
			if kind != common.CompositeKindStructure {
				expectedErrorCount += 1
			}

			errs := ExpectCheckerErrors(t, err, expectedErrorCount)
			assert.IsType(t, &sema.NotDeclaredError{}, errs[0])

			assert.IsType(t, &sema.MissingInitializerError{}, errs[1])

			if kind != common.CompositeKindStructure {
				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[2])
			}
		})
	}
}

func TestCheckInvalidCompositeInitializerParameterType(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              %s Test {
                  init(x: X) {}
              }
	        `, kind.Keyword()))

			// TODO: add support for non-structure declarations

			expectedErrorCount := 1
			if kind != common.CompositeKindStructure {
				expectedErrorCount += 1
			}

			errs := ExpectCheckerErrors(t, err, expectedErrorCount)

			assert.IsType(t, &sema.NotDeclaredError{}, errs[0])

			if kind != common.CompositeKindStructure {
				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[1])
			}
		})
	}
}

func TestCheckInvalidCompositeInitializerParameters(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              %s Test {
                  init(x: Int, x: Int) {}
              }
	        `, kind.Keyword()))

			// TODO: add support for non-structure declarations

			expectedErrorCount := 1
			if kind != common.CompositeKindStructure {
				expectedErrorCount += 1
			}

			errs := ExpectCheckerErrors(t, err, expectedErrorCount)

			assert.IsType(t, &sema.RedeclarationError{}, errs[0])

			if kind != common.CompositeKindStructure {
				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[1])
			}
		})
	}
}

func TestCheckInvalidCompositeInitializer(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              %s Test {
                  init() { X }
              }
	        `, kind.Keyword()))

			// TODO: add support for non-structure declarations

			expectedErrorCount := 1
			if kind != common.CompositeKindStructure {
				expectedErrorCount += 1
			}

			errs := ExpectCheckerErrors(t, err, expectedErrorCount)

			assert.IsType(t, &sema.NotDeclaredError{}, errs[0])

			if kind != common.CompositeKindStructure {
				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[1])
			}
		})
	}
}

func TestCheckInvalidCompositeFunction(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              %s Test {
                  fun test() { X }
              }
	        `, kind.Keyword()))

			// TODO: add support for non-structure declarations

			expectedErrorCount := 1
			if kind != common.CompositeKindStructure {
				expectedErrorCount += 1
			}

			errs := ExpectCheckerErrors(t, err, expectedErrorCount)

			assert.IsType(t, &sema.NotDeclaredError{}, errs[0])

			if kind != common.CompositeKindStructure {
				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[1])
			}
		})
	}
}

func TestCheckCompositeInitializerSelfReference(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              %s Test {
                  init() { self }
              }
	        `, kind.Keyword()))

			// TODO: add support for non-structure declarations

			switch kind {
			case common.CompositeKindStructure:
				assert.Nil(t, err)
			case common.CompositeKindContract:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
			case common.CompositeKindResource:
				errs := ExpectCheckerErrors(t, err, 2)

				// TODO: handle `self` properly

				assert.IsType(t, &sema.ResourceLossError{}, errs[0])

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[1])
			}
		})
	}
}

func TestCheckCompositeFunctionSelfReference(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              %s Test {
                  fun test() { self }
              }
	        `, kind.Keyword()))

			// TODO: add support for non-structure declarations

			switch kind {
			case common.CompositeKindStructure:
				assert.Nil(t, err)

			case common.CompositeKindContract:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

			case common.CompositeKindResource:
				errs := ExpectCheckerErrors(t, err, 2)

				// TODO: handle `self` properly

				assert.IsType(t, &sema.ResourceLossError{}, errs[0])

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[1])
			}
		})
	}
}

func TestCheckInvalidLocalComposite(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              fun test() {
                  %s Test {}
              }
            `, kind.Keyword()))

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.InvalidDeclarationError{}, errs[0])
		})
	}
}

func TestCheckInvalidCompositeMissingInitializer(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
               %s Test {
                   let foo: Int
               }
            `, kind.Keyword()))

			// TODO: add support for non-structure declarations

			expectedErrorCount := 1
			if kind != common.CompositeKindStructure {
				expectedErrorCount += 1
			}

			errs := ExpectCheckerErrors(t, err, expectedErrorCount)

			assert.IsType(t, &sema.MissingInitializerError{}, errs[0])

			if kind != common.CompositeKindStructure {
				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[1])
			}
		})
	}
}

func TestCheckCompositeFieldAccess(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              %s Test {
                  let foo: Int

                  init() {
                      self.foo = 1
                  }

                  fun test() {
                      self.foo
                  }
              }
            `, kind.Keyword()))

			// TODO: add support for non-structure declarations

			if kind == common.CompositeKindStructure {
				assert.Nil(t, err)
			} else {
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
			}
		})
	}
}

func TestCheckInvalidCompositeFieldAccess(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              %s Test {
                  init() {
                      self.foo
                  }

                  fun test() {
                      self.bar
                  }
              }
	        `, kind.Keyword()))

			// TODO: add support for non-structure declarations

			expectedErrorCount := 3
			if kind != common.CompositeKindStructure {
				expectedErrorCount += 1
			}

			errs := ExpectCheckerErrors(t, err, expectedErrorCount)

			assert.IsType(t, &sema.NotDeclaredMemberError{}, errs[0])
			assert.Equal(t,
				"foo",
				errs[0].(*sema.NotDeclaredMemberError).Name,
			)

			assert.IsType(t, &self_field_analyzer.UninitializedFieldAccessError{}, errs[1])

			assert.IsType(t, &sema.NotDeclaredMemberError{}, errs[2])
			assert.Equal(t,
				"bar",
				errs[2].(*sema.NotDeclaredMemberError).Name,
			)

			if kind != common.CompositeKindStructure {
				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[3])
			}
		})
	}
}

func TestCheckCompositeFieldAssignment(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
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
			))

			switch kind {
			case common.CompositeKindStructure:
				assert.Nil(t, err)

			case common.CompositeKindContract:
				// TODO: add support for contract declarations

				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

			case common.CompositeKindResource:

				errs := ExpectCheckerErrors(t, err, 3)

				// TODO: remove once `self` is handled properly

				assert.IsType(t, &sema.ResourceLossError{}, errs[0])

				assert.IsType(t, &sema.ResourceLossError{}, errs[1])

				// TODO: add support for resource declarations

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[2])
			}
		})
	}
}

func TestCheckInvalidCompositeSelfAssignment(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              %[1]s Test {
                  init() {
                      self %[2]s %[3]s Test()
                  }

                  fun test() {
                      self %[2]s %[3]s Test()
                  }
              }
	        `,
				kind.Keyword(),
				kind.TransferOperator(),
				kind.ConstructionKeyword(),
			))

			// TODO: add support for non-structure declarations

			expectedErrorCount := 2
			if kind != common.CompositeKindStructure {
				expectedErrorCount += 1
			}

			errs := ExpectCheckerErrors(t, err, expectedErrorCount)
			assert.IsType(t, &sema.AssignmentToConstantError{}, errs[0])

			assert.IsType(t, &sema.AssignmentToConstantError{}, errs[1])

			if kind != common.CompositeKindStructure {
				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[2])
			}
		})
	}
}

func TestCheckInvalidCompositeFieldAssignment(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              %s Test {
                  init() {
                      self.foo = 1
                  }

                  fun test() {
                      self.bar = 2
                  }
              }
	        `, kind.Keyword()))

			// TODO: add support for non-structure declarations

			expectedErrorCount := 2
			if kind != common.CompositeKindStructure {
				expectedErrorCount += 1
			}

			errs := ExpectCheckerErrors(t, err, expectedErrorCount)
			assert.IsType(t, &sema.NotDeclaredMemberError{}, errs[0])
			assert.Equal(t,
				"foo",
				errs[0].(*sema.NotDeclaredMemberError).Name,
			)

			assert.IsType(t, &sema.NotDeclaredMemberError{}, errs[1])
			assert.Equal(t,
				"bar",
				errs[1].(*sema.NotDeclaredMemberError).Name,
			)

			if kind != common.CompositeKindStructure {
				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[2])
			}
		})
	}
}

func TestCheckInvalidCompositeFieldAssignmentWrongType(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              %s Test {
                  var foo: Int

                  init() {
                      self.foo = true
                  }

                  fun test() {
                      self.foo = false
                  }
              }
	        `, kind.Keyword()))

			// TODO: add support for non-structure declarations

			expectedErrorCount := 2
			if kind != common.CompositeKindStructure {
				expectedErrorCount += 1
			}

			errs := ExpectCheckerErrors(t, err, expectedErrorCount)
			assert.IsType(t, &sema.TypeMismatchError{}, errs[0])

			assert.IsType(t, &sema.TypeMismatchError{}, errs[1])

			if kind != common.CompositeKindStructure {
				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[2])
			}
		})
	}
}

func TestCheckInvalidCompositeFieldConstantAssignment(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
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
	        `, kind.Keyword()))

			// TODO: add support for non-structure declarations

			expectedErrorCount := 1
			if kind != common.CompositeKindStructure {
				expectedErrorCount += 1
			}

			errs := ExpectCheckerErrors(t, err, expectedErrorCount)

			assert.IsType(t, &sema.AssignmentToConstantMemberError{}, errs[0])

			if kind != common.CompositeKindStructure {
				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[1])
			}
		})
	}
}

func TestCheckCompositeFunctionCall(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              %s Test {
                  fun foo() {}

                  fun bar() {
                      self.foo()
                  }
              }
	        `, kind.Keyword()))

			// TODO: add support for non-structure declarations

			if kind == common.CompositeKindStructure {
				assert.Nil(t, err)
			} else {
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
			}
		})
	}
}

func TestCheckInvalidCompositeFunctionCall(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              %s Test {
                  fun foo() {}

                  fun bar() {
                      self.baz()
                  }
              }
	        `, kind.Keyword()))

			// TODO: add support for non-structure declarations

			expectedErrorCount := 1
			if kind != common.CompositeKindStructure {
				expectedErrorCount += 1
			}

			errs := ExpectCheckerErrors(t, err, expectedErrorCount)

			assert.IsType(t, &sema.NotDeclaredMemberError{}, errs[0])

			if kind != common.CompositeKindStructure {
				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[1])
			}
		})
	}
}

func TestCheckInvalidCompositeFunctionAssignment(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              %s Test {
                  fun foo() {}

                  fun bar() {
                      self.foo = 2
                  }
              }
	        `, kind.Keyword()))

			// TODO: add support for non-structure declarations

			expectedErrorCount := 2
			if kind != common.CompositeKindStructure {
				expectedErrorCount += 1
			}

			errs := ExpectCheckerErrors(t, err, expectedErrorCount)

			assert.IsType(t, &sema.AssignmentToConstantMemberError{}, errs[0])
			assert.Equal(t,
				"foo",
				errs[0].(*sema.AssignmentToConstantMemberError).Name,
			)

			assert.IsType(t, &sema.TypeMismatchError{}, errs[1])

			if kind != common.CompositeKindStructure {
				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[2])
			}
		})
	}
}

func TestCheckCompositeInstantiation(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
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
				kind.Keyword(),
				kind.Annotation(),
				kind.TransferOperator(),
				kind.ConstructionKeyword(),
				kind.DestructionKeyword(),
			))

			// TODO: add support for non-structure declarations

			if kind == common.CompositeKindStructure {
				assert.Nil(t, err)
			} else {
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
			}
		})
	}
}

func TestCheckInvalidSameCompositeRedeclaration(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              let x = 1
              %[1]s Foo {}
              %[1]s Foo {}
	        `, kind.Keyword()))

			// TODO: add support for non-structure declarations

			expectedErrorCount := 2
			if kind != common.CompositeKindStructure {
				expectedErrorCount += 2
			}

			errs := ExpectCheckerErrors(t, err, expectedErrorCount)

			// NOTE: two errors: one because type is redeclared,
			// the other because the global is redeclared

			assert.IsType(t, &sema.RedeclarationError{}, errs[0])

			assert.IsType(t, &sema.RedeclarationError{}, errs[1])

			if kind != common.CompositeKindStructure {
				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[2])

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[3])
			}
		})
	}
}

func TestCheckInvalidDifferentCompositeRedeclaration(t *testing.T) {

	for _, firstKind := range common.CompositeKinds {
		for _, secondKind := range common.CompositeKinds {

			// only check different kinds
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
                  let x = 1
                  %[1]s Foo {}
                  %[2]s Foo {}
	            `,
					firstKind.Keyword(),
					secondKind.Keyword(),
				))

				// TODO: add support for non-structure declarations

				expectedErrorCount := 2
				if firstKind != common.CompositeKindStructure {
					expectedErrorCount += 1
				}
				if secondKind != common.CompositeKindStructure {
					expectedErrorCount += 1
				}

				errs := ExpectCheckerErrors(t, err, expectedErrorCount)

				// NOTE: two errors: one because type is redeclared,
				// the other because the global is redeclared

				assert.IsType(t, &sema.RedeclarationError{}, errs[0])

				assert.IsType(t, &sema.RedeclarationError{}, errs[1])

				if firstKind != common.CompositeKindStructure &&
					secondKind != common.CompositeKindStructure {

					assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[2])

					assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[3])

				} else if firstKind != common.CompositeKindStructure ||
					secondKind != common.CompositeKindStructure {

					assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[2])
				}
			})
		}
	}
}

func TestCheckInvalidForwardReference(t *testing.T) {

	_, err := ParseAndCheck(t, `
      let x = y
      let y = x
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
}

func TestCheckInvalidIncompatibleSameCompositeTypes(t *testing.T) {
	// tests that composite typing is nominal, not structural,
	// and composite kind is considered

	for _, firstKind := range common.CompositeKinds {
		for _, secondKind := range common.CompositeKinds {

			testName := fmt.Sprintf(
				"%s/%s",
				firstKind.Keyword(),
				secondKind.Keyword(),
			)

			t.Run(testName, func(t *testing.T) {

				_, err := ParseAndCheck(t, fmt.Sprintf(`
                  %[1]s Foo {
                      init() {}
                  }

                  %[2]s Bar {
                      init() {}
                  }

                  let foo: %[3]sFoo %[4]s %[5]s Bar()
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

				errs := ExpectCheckerErrors(t, err, expectedErrorCount)

				if firstKind != common.CompositeKindStructure &&
					secondKind != common.CompositeKindStructure {

					assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

					assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[1])

				} else if firstKind != common.CompositeKindStructure ||
					secondKind != common.CompositeKindStructure {

					assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
				}

				assert.IsType(t, &sema.TypeMismatchError{}, errs[expectedErrorCount-1])

			})
		}
	}
}

func TestCheckInvalidCompositeFunctionWithSelfParameter(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              %s Foo {
                  fun test(self: Int) {}
              }
	        `, kind.Keyword()))

			// TODO: add support for non-structure declarations

			expectedErrorCount := 1
			if kind != common.CompositeKindStructure {
				expectedErrorCount += 1
			}

			errs := ExpectCheckerErrors(t, err, expectedErrorCount)

			assert.IsType(t, &sema.RedeclarationError{}, errs[0])

			if kind != common.CompositeKindStructure {
				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[1])
			}
		})
	}
}

func TestCheckInvalidCompositeInitializerWithSelfParameter(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              %s Foo {
                  init(self: Int) {}
              }
	        `, kind.Keyword()))

			// TODO: add support for non-structure declarations

			expectedErrorCount := 1
			if kind != common.CompositeKindStructure {
				expectedErrorCount += 1
			}

			errs := ExpectCheckerErrors(t, err, expectedErrorCount)

			assert.IsType(t, &sema.RedeclarationError{}, errs[0])

			if kind != common.CompositeKindStructure {
				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[1])
			}
		})
	}
}

func TestCheckCompositeInitializesConstant(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              %[1]s Test {
                  let foo: Int

                  init() {
                      self.foo = 42
                  }
              }

	          let test %[2]s %[3]s Test()
	        `,
				kind.Keyword(),
				kind.TransferOperator(),
				kind.ConstructionKeyword(),
			))

			// TODO: add support for non-structure declarations

			if kind == common.CompositeKindStructure {
				assert.Nil(t, err)
			} else {
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
			}
		})
	}
}

func TestCheckCompositeInitializerWithArgumentLabel(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              %[1]s Test {

                  init(x: Int) {}
              }

	          let test %[2]s %[3]s Test(x: 1)
	        `,
				kind.Keyword(),
				kind.TransferOperator(),
				kind.ConstructionKeyword(),
			))

			// TODO: add support for non-structure declarations

			if kind == common.CompositeKindStructure {
				assert.Nil(t, err)
			} else {
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
			}
		})
	}
}

func TestCheckInvalidCompositeInitializerCallWithMissingArgumentLabel(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              %[1]s Test {

                  init(x: Int) {}
              }

	          let test %[2]s %[3]s Test(1)
	        `,
				kind.Keyword(),
				kind.TransferOperator(),
				kind.ConstructionKeyword(),
			))

			// TODO: add support for non-structure declarations

			if kind == common.CompositeKindStructure {
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.MissingArgumentLabelError{}, errs[0])
			} else {
				errs := ExpectCheckerErrors(t, err, 2)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

				assert.IsType(t, &sema.MissingArgumentLabelError{}, errs[1])
			}
		})
	}
}

func TestCheckCompositeFunctionWithArgumentLabel(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              %[1]s Test {

                  fun test(x: Int) {}
              }

              let test %[2]s %[3]s Test()
	          let void = test.test(x: 1)
	        `,
				kind.Keyword(),
				kind.TransferOperator(),
				kind.ConstructionKeyword(),
			))

			// TODO: add support for non-structure declarations

			if kind == common.CompositeKindStructure {
				assert.Nil(t, err)
			} else {
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
			}
		})
	}
}

func TestCheckInvalidCompositeFunctionCallWithMissingArgumentLabel(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              %[1]s Test {
    
                  fun test(x: Int) {}
              }
    
              let test %[2]s %[3]s Test()
	          let void = test.test(1)
	        `,
				kind.Keyword(),
				kind.TransferOperator(),
				kind.ConstructionKeyword(),
			))

			// TODO: add support for non-structure declarations

			if kind == common.CompositeKindStructure {
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.MissingArgumentLabelError{}, errs[0])
			} else {
				errs := ExpectCheckerErrors(t, err, 2)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

				assert.IsType(t, &sema.MissingArgumentLabelError{}, errs[1])
			}
		})
	}
}

func TestCheckCompositeConstructorReferenceInInitializerAndFunction(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			checker, err := ParseAndCheck(t, fmt.Sprintf(`
              %[1]s Test {

                  init() {
                      Test
                  }

                  fun test(): %[2]sTest {
                      return %[2]s%[3]s Test()
                  }
              }

              fun test(): %[2]sTest {
                  return %[2]s%[3]s Test()
              }

              fun test2(): %[2]sTest {
                  let test %[4]s %[3]s Test()
                  let res %[4]s test.test()
                  %[5]s test
                  return %[2]sres
              }
            `,
				kind.Keyword(),
				kind.Annotation(),
				kind.ConstructionKeyword(),
				kind.TransferOperator(),
				kind.DestructionKeyword(),
			))

			// TODO: add support for non-structure declarations

			if kind == common.CompositeKindStructure {
				assert.Nil(t, err)

				testType := checker.FindType("Test")

				assert.IsType(t, &sema.CompositeType{}, testType)

				structureType := testType.(*sema.CompositeType)

				assert.Equal(t,
					"Test",
					structureType.Identifier,
				)

				testFunctionMember := structureType.Members["test"]

				assert.IsType(t, &sema.FunctionType{}, testFunctionMember.Type)

				testFunctionType := testFunctionMember.Type.(*sema.FunctionType)

				actual := testFunctionType.ReturnTypeAnnotation.Type
				if actual != structureType {
					assert.Fail(t, "not structureType", actual)
				}
			} else {
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
			}
		})
	}
}

func TestCheckInvalidCompositeFieldMissingVariableKind(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              %s X {
                  x: Int

                  init(x: Int) {
                      self.x = x
                  }
              }
	        `, kind.Keyword()))

			// TODO: add support for non-structure declarations

			expectedErrorCount := 1
			if kind != common.CompositeKindStructure {
				expectedErrorCount += 1
			}

			errs := ExpectCheckerErrors(t, err, expectedErrorCount)

			assert.IsType(t, &sema.InvalidVariableKindError{}, errs[0])

			if kind != common.CompositeKindStructure {
				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[1])
			}
		})
	}
}

func TestCheckCompositeFunction(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              %[1]s X {
                  fun foo(): ((): %[2]sX) {
                      return self.bar
                  }

                  fun bar(): %[2]sX {
                      return %[2]s self
                  }
              }
	        `, kind.Keyword(), kind.Annotation()))

			// TODO: add support for non-structure declarations

			if kind == common.CompositeKindStructure {
				assert.Nil(t, err)
			} else {
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
			}
		})
	}
}

func TestCheckCompositeReferenceBeforeDeclaration(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
              var tests = 0

              fun test(): %[1]sTest {
                  return %[1]s %[2]s Test()
              }

              %[3]s Test {
                 init() {
                     tests = tests + 1
                 }
              }
            `,
				kind.Annotation(),
				kind.ConstructionKeyword(),
				kind.Keyword(),
			))

			// TODO: add support for non-structure declarations

			if kind == common.CompositeKindStructure {
				assert.Nil(t, err)
			} else {
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
			}
		})
	}
}
