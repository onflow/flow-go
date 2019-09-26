package checker

import (
	"fmt"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/common"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/pkg/language/runtime/tests/utils"
	. "github.com/onsi/gomega"
	"testing"
)

func TestCheckInvalidCompositeRedeclaringType(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := ParseAndCheck(fmt.Sprintf(`
          %s Int {}
        `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		expectedErrorCount := 1
		if kind != common.CompositeKindStructure {
			expectedErrorCount += 1
		}

		errs := ExpectCheckerErrors(err, expectedErrorCount)

		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.RedeclarationError{}))

		if kind != common.CompositeKindStructure {
			Expect(errs[1]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckComposite(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := ParseAndCheck(fmt.Sprintf(`
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
			Expect(err).
				To(Not(HaveOccurred()))
		} else {
			errs := ExpectCheckerErrors(err, 1)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInitializerName(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := ParseAndCheck(fmt.Sprintf(`
          %s Test {
              init() {}
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
	}
}

func TestCheckInvalidInitializerName(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := ParseAndCheck(fmt.Sprintf(`
          %s Test {
              initializer() {}
          }
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		expectedErrorCount := 1
		if kind != common.CompositeKindStructure {
			expectedErrorCount += 1
		}

		errs := ExpectCheckerErrors(err, expectedErrorCount)

		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.InvalidInitializerNameError{}))

		if kind != common.CompositeKindStructure {
			Expect(errs[1]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidCompositeFieldName(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := ParseAndCheck(fmt.Sprintf(`
          %s Test {
              let init: Int
          }
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		expectedErrorCount := 3
		if kind != common.CompositeKindStructure {
			expectedErrorCount += 1
		}

		errs := ExpectCheckerErrors(err, expectedErrorCount)

		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.InvalidNameError{}))

		Expect(errs[1]).
			To(BeAssignableToTypeOf(&sema.MissingInitializerError{}))

		Expect(errs[2]).
			To(BeAssignableToTypeOf(&sema.FieldUninitializedError{}))

		if kind != common.CompositeKindStructure {
			Expect(errs[3]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidCompositeFunctionName(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := ParseAndCheck(fmt.Sprintf(`
          %s Test {
              fun init() {}
          }
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		expectedErrorCount := 1
		if kind != common.CompositeKindStructure {
			expectedErrorCount += 1
		}

		errs := ExpectCheckerErrors(err, expectedErrorCount)

		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.InvalidNameError{}))

		if kind != common.CompositeKindStructure {
			Expect(errs[1]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidCompositeRedeclaringFields(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := ParseAndCheck(fmt.Sprintf(`
          %s Test {
              let x: Int
              let x: Int
          }
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		expectedErrorCount := 4
		if kind != common.CompositeKindStructure {
			expectedErrorCount += 1
		}

		errs := ExpectCheckerErrors(err, expectedErrorCount)

		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.RedeclarationError{}))

		Expect(errs[1]).
			To(BeAssignableToTypeOf(&sema.MissingInitializerError{}))

		Expect(errs[2]).
			To(BeAssignableToTypeOf(&sema.FieldUninitializedError{}))

		Expect(errs[3]).
			To(BeAssignableToTypeOf(&sema.FieldUninitializedError{}))

		if kind != common.CompositeKindStructure {
			Expect(errs[4]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidCompositeRedeclaringFunctions(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := ParseAndCheck(fmt.Sprintf(`
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

		errs := ExpectCheckerErrors(err, expectedErrorCount)

		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.RedeclarationError{}))

		if kind != common.CompositeKindStructure {
			Expect(errs[1]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidCompositeRedeclaringFieldsAndFunctions(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := ParseAndCheck(fmt.Sprintf(`
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

		errs := ExpectCheckerErrors(err, expectedErrorCount)
		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.RedeclarationError{}))

		Expect(errs[1]).
			To(BeAssignableToTypeOf(&sema.MissingInitializerError{}))

		if kind != common.CompositeKindStructure {
			Expect(errs[2]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckCompositeFieldsAndFunctions(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := ParseAndCheck(fmt.Sprintf(`
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
			Expect(err).
				To(Not(HaveOccurred()))
		} else {
			errs := ExpectCheckerErrors(err, 1)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidCompositeFieldType(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := ParseAndCheck(fmt.Sprintf(`
          %s Test {
              let x: X
          }
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		expectedErrorCount := 3
		if kind != common.CompositeKindStructure {
			expectedErrorCount += 1
		}

		errs := ExpectCheckerErrors(err, expectedErrorCount)
		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))

		Expect(errs[1]).
			To(BeAssignableToTypeOf(&sema.MissingInitializerError{}))

		Expect(errs[2]).
			To(BeAssignableToTypeOf(&sema.FieldUninitializedError{}))

		if kind != common.CompositeKindStructure {
			Expect(errs[3]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidCompositeInitializerParameterType(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := ParseAndCheck(fmt.Sprintf(`
          %s Test {
              init(x: X) {}
          }
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		expectedErrorCount := 1
		if kind != common.CompositeKindStructure {
			expectedErrorCount += 1
		}

		errs := ExpectCheckerErrors(err, expectedErrorCount)

		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))

		if kind != common.CompositeKindStructure {
			Expect(errs[1]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidCompositeInitializerParameters(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := ParseAndCheck(fmt.Sprintf(`
          %s Test {
              init(x: Int, x: Int) {}
          }
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		expectedErrorCount := 1
		if kind != common.CompositeKindStructure {
			expectedErrorCount += 1
		}

		errs := ExpectCheckerErrors(err, expectedErrorCount)

		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.RedeclarationError{}))

		if kind != common.CompositeKindStructure {
			Expect(errs[1]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidCompositeInitializer(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := ParseAndCheck(fmt.Sprintf(`
          %s Test {
              init() { X }
          }
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		expectedErrorCount := 1
		if kind != common.CompositeKindStructure {
			expectedErrorCount += 1
		}

		errs := ExpectCheckerErrors(err, expectedErrorCount)

		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))

		if kind != common.CompositeKindStructure {
			Expect(errs[1]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidCompositeFunction(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := ParseAndCheck(fmt.Sprintf(`
          %s Test {
              fun test() { X }
          }
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		expectedErrorCount := 1
		if kind != common.CompositeKindStructure {
			expectedErrorCount += 1
		}

		errs := ExpectCheckerErrors(err, expectedErrorCount)

		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))

		if kind != common.CompositeKindStructure {
			Expect(errs[1]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckCompositeInitializerSelfReference(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := ParseAndCheck(fmt.Sprintf(`
          %s Test {
              init() { self }
          }
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		switch kind {
		case common.CompositeKindStructure:
			Expect(err).
				To(Not(HaveOccurred()))
		case common.CompositeKindContract:
			errs := ExpectCheckerErrors(err, 1)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		case common.CompositeKindResource:
			errs := ExpectCheckerErrors(err, 2)

			// TODO: handle `self` properly

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.ResourceLossError{}))

			Expect(errs[1]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckCompositeFunctionSelfReference(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := ParseAndCheck(fmt.Sprintf(`
          %s Test {
              fun test() { self }
          }
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		switch kind {
		case common.CompositeKindStructure:
			Expect(err).
				To(Not(HaveOccurred()))

		case common.CompositeKindContract:
			errs := ExpectCheckerErrors(err, 1)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

		case common.CompositeKindResource:
			errs := ExpectCheckerErrors(err, 2)

			// TODO: handle `self` properly

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.ResourceLossError{}))

			Expect(errs[1]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidLocalComposite(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := ParseAndCheck(fmt.Sprintf(`
          fun test() {
              %s Test {}
          }
        `, kind.Keyword()))

		errs := ExpectCheckerErrors(err, 1)

		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.InvalidDeclarationError{}))
	}
}

func TestCheckInvalidCompositeMissingInitializer(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := ParseAndCheck(fmt.Sprintf(`
           %s Test {
               let foo: Int
           }
        `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		expectedErrorCount := 2
		if kind != common.CompositeKindStructure {
			expectedErrorCount += 1
		}

		errs := ExpectCheckerErrors(err, expectedErrorCount)

		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.MissingInitializerError{}))

		Expect(errs[1]).
			To(BeAssignableToTypeOf(&sema.FieldUninitializedError{}))

		if kind != common.CompositeKindStructure {
			Expect(errs[2]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckCompositeFieldAccess(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := ParseAndCheck(fmt.Sprintf(`
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
			Expect(err).
				To(Not(HaveOccurred()))
		} else {
			errs := ExpectCheckerErrors(err, 1)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidCompositeFieldAccess(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := ParseAndCheck(fmt.Sprintf(`
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

		expectedErrorCount := 2
		if kind != common.CompositeKindStructure {
			expectedErrorCount += 1
		}

		errs := ExpectCheckerErrors(err, expectedErrorCount)

		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.NotDeclaredMemberError{}))
		Expect(errs[0].(*sema.NotDeclaredMemberError).Name).
			To(Equal("foo"))

		Expect(errs[1]).
			To(BeAssignableToTypeOf(&sema.NotDeclaredMemberError{}))
		Expect(errs[1].(*sema.NotDeclaredMemberError).Name).
			To(Equal("bar"))

		if kind != common.CompositeKindStructure {
			Expect(errs[2]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckCompositeFieldAssignment(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := ParseAndCheck(fmt.Sprintf(`
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

		// TODO: add support for non-structure declarations

		if kind == common.CompositeKindStructure {
			Expect(err).
				To(Not(HaveOccurred()))
		} else {
			errs := ExpectCheckerErrors(err, 1)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidCompositeSelfAssignment(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := ParseAndCheck(fmt.Sprintf(`
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

		errs := ExpectCheckerErrors(err, expectedErrorCount)
		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.AssignmentToConstantError{}))

		Expect(errs[1]).
			To(BeAssignableToTypeOf(&sema.AssignmentToConstantError{}))

		if kind != common.CompositeKindStructure {
			Expect(errs[2]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidCompositeFieldAssignment(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := ParseAndCheck(fmt.Sprintf(`
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

		errs := ExpectCheckerErrors(err, expectedErrorCount)
		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.NotDeclaredMemberError{}))
		Expect(errs[0].(*sema.NotDeclaredMemberError).Name).
			To(Equal("foo"))

		Expect(errs[1]).
			To(BeAssignableToTypeOf(&sema.NotDeclaredMemberError{}))
		Expect(errs[1].(*sema.NotDeclaredMemberError).Name).
			To(Equal("bar"))

		if kind != common.CompositeKindStructure {
			Expect(errs[2]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidCompositeFieldAssignmentWrongType(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := ParseAndCheck(fmt.Sprintf(`
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

		errs := ExpectCheckerErrors(err, expectedErrorCount)
		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))

		Expect(errs[1]).
			To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))

		if kind != common.CompositeKindStructure {
			Expect(errs[2]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidCompositeFieldConstantAssignment(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := ParseAndCheck(fmt.Sprintf(`
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

		errs := ExpectCheckerErrors(err, expectedErrorCount)

		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.AssignmentToConstantMemberError{}))

		if kind != common.CompositeKindStructure {
			Expect(errs[1]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckCompositeFunctionCall(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := ParseAndCheck(fmt.Sprintf(`
          %s Test {
              fun foo() {}

              fun bar() {
                  self.foo()
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
	}
}

func TestCheckInvalidCompositeFunctionCall(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := ParseAndCheck(fmt.Sprintf(`
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

		errs := ExpectCheckerErrors(err, expectedErrorCount)

		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.NotDeclaredMemberError{}))

		if kind != common.CompositeKindStructure {
			Expect(errs[1]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidCompositeFunctionAssignment(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := ParseAndCheck(fmt.Sprintf(`
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

		errs := ExpectCheckerErrors(err, expectedErrorCount)

		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.AssignmentToConstantMemberError{}))
		Expect(errs[0].(*sema.AssignmentToConstantMemberError).Name).
			To(Equal("foo"))

		Expect(errs[1]).
			To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))

		if kind != common.CompositeKindStructure {
			Expect(errs[2]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckCompositeInstantiation(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := ParseAndCheck(fmt.Sprintf(`
          %[1]s Test {

              init(x: Int) {
                  let test: %[2]sTest %[3]s %[4]s Test(x: 1)
              }

              fun test() {
                  let test: %[2]sTest %[3]s %[4]s Test(x: 2)
              }
          }

          let test: %[2]sTest %[3]s %[4]s Test(x: 3)
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
			errs := ExpectCheckerErrors(err, 1)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidSameCompositeRedeclaration(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := ParseAndCheck(fmt.Sprintf(`
          let x = 1
          %[1]s Foo {}
          %[1]s Foo {}
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		expectedErrorCount := 2
		if kind != common.CompositeKindStructure {
			expectedErrorCount += 2
		}

		errs := ExpectCheckerErrors(err, expectedErrorCount)

		// NOTE: two errors: one because type is redeclared,
		// the other because the global is redeclared

		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.RedeclarationError{}))

		Expect(errs[1]).
			To(BeAssignableToTypeOf(&sema.RedeclarationError{}))

		if kind != common.CompositeKindStructure {
			Expect(errs[2]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			Expect(errs[3]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidDifferentCompositeRedeclaration(t *testing.T) {
	RegisterTestingT(t)

	for _, firstKind := range common.CompositeKinds {
		for _, secondKind := range common.CompositeKinds {

			// only check different kinds
			if firstKind == secondKind {
				continue
			}

			_, err := ParseAndCheck(fmt.Sprintf(`
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

			errs := ExpectCheckerErrors(err, expectedErrorCount)

			// NOTE: two errors: one because type is redeclared,
			// the other because the global is redeclared

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.RedeclarationError{}))

			Expect(errs[1]).
				To(BeAssignableToTypeOf(&sema.RedeclarationError{}))

			if firstKind != common.CompositeKindStructure &&
				secondKind != common.CompositeKindStructure {

				Expect(errs[2]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[3]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			} else if firstKind != common.CompositeKindStructure ||
				secondKind != common.CompositeKindStructure {

				Expect(errs[2]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
			}
		}
	}
}

func TestCheckInvalidForwardReference(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      let x = y
      let y = x
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))
}

func TestCheckInvalidIncompatibleSameCompositeTypes(t *testing.T) {
	// tests that composite typing is nominal, not structural,
	// and composite kind is considered

	RegisterTestingT(t)

	for _, firstKind := range common.CompositeKinds {
		for _, secondKind := range common.CompositeKinds {

			_, err := ParseAndCheck(fmt.Sprintf(`
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

			errs := ExpectCheckerErrors(err, expectedErrorCount)

			if firstKind != common.CompositeKindStructure &&
				secondKind != common.CompositeKindStructure {

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

				Expect(errs[1]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			} else if firstKind != common.CompositeKindStructure ||
				secondKind != common.CompositeKindStructure {

				Expect(errs[0]).
					To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
			}

			Expect(errs[expectedErrorCount-1]).
				To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))

		}
	}
}

func TestCheckInvalidCompositeFunctionWithSelfParameter(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := ParseAndCheck(fmt.Sprintf(`
          %s Foo {
              fun test(self: Int) {}
          }
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		expectedErrorCount := 1
		if kind != common.CompositeKindStructure {
			expectedErrorCount += 1
		}

		errs := ExpectCheckerErrors(err, expectedErrorCount)

		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.RedeclarationError{}))

		if kind != common.CompositeKindStructure {
			Expect(errs[1]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidCompositeInitializerWithSelfParameter(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := ParseAndCheck(fmt.Sprintf(`
          %s Foo {
              init(self: Int) {}
          }
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		expectedErrorCount := 1
		if kind != common.CompositeKindStructure {
			expectedErrorCount += 1
		}

		errs := ExpectCheckerErrors(err, expectedErrorCount)

		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.RedeclarationError{}))

		if kind != common.CompositeKindStructure {
			Expect(errs[1]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckCompositeInitializesConstant(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := ParseAndCheck(fmt.Sprintf(`
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
			Expect(err).
				To(Not(HaveOccurred()))
		} else {
			errs := ExpectCheckerErrors(err, 1)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckCompositeInitializerWithArgumentLabel(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := ParseAndCheck(fmt.Sprintf(`
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
			Expect(err).
				To(Not(HaveOccurred()))
		} else {
			errs := ExpectCheckerErrors(err, 1)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidCompositeInitializerCallWithMissingArgumentLabel(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := ParseAndCheck(fmt.Sprintf(`
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
			errs := ExpectCheckerErrors(err, 1)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.MissingArgumentLabelError{}))
		} else {
			errs := ExpectCheckerErrors(err, 2)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			Expect(errs[1]).
				To(BeAssignableToTypeOf(&sema.MissingArgumentLabelError{}))
		}
	}
}

func TestCheckCompositeFunctionWithArgumentLabel(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := ParseAndCheck(fmt.Sprintf(`
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
			Expect(err).
				To(Not(HaveOccurred()))
		} else {
			errs := ExpectCheckerErrors(err, 1)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidCompositeFunctionCallWithMissingArgumentLabel(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := ParseAndCheck(fmt.Sprintf(`
          %s Test {

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
			errs := ExpectCheckerErrors(err, 1)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.MissingArgumentLabelError{}))
		} else {
			errs := ExpectCheckerErrors(err, 2)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

			Expect(errs[1]).
				To(BeAssignableToTypeOf(&sema.MissingArgumentLabelError{}))
		}
	}
}

func TestCheckCompositeConstructorReferenceInInitializerAndFunction(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		checker, err := ParseAndCheck(fmt.Sprintf(`
          %s Test {

              init() {
                  Test
              }

              fun test(): %[2]sTest {
                  return %[3]s Test()
              }
          }

          fun test(): %[2]sTest {
              return %[3]s Test()
          }

          fun test2(): %[2]sTest {
              let test %[4]s %[3]s Test()
              return %[2]s test.test()
          }
        `,
			kind.Keyword(),
			kind.Annotation(),
			kind.ConstructionKeyword(),
			kind.TransferOperator(),
		))

		// TODO: add support for non-structure declarations

		if kind == common.CompositeKindStructure {
			Expect(err).
				To(Not(HaveOccurred()))

			testType := checker.FindType("Test")

			Expect(testType).
				To(BeAssignableToTypeOf(&sema.CompositeType{}))

			structureType := testType.(*sema.CompositeType)

			Expect(structureType.Identifier).
				To(Equal("Test"))

			testFunctionMember := structureType.Members["test"]

			Expect(testFunctionMember.Type).
				To(BeAssignableToTypeOf(&sema.FunctionType{}))

			testFunctionType := testFunctionMember.Type.(*sema.FunctionType)

			Expect(testFunctionType.ReturnTypeAnnotation.Type).
				To(BeIdenticalTo(structureType))
		} else {
			errs := ExpectCheckerErrors(err, 1)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckInvalidCompositeFieldMissingVariableKind(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := ParseAndCheck(fmt.Sprintf(`
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

		errs := ExpectCheckerErrors(err, expectedErrorCount)

		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.InvalidVariableKindError{}))

		if kind != common.CompositeKindStructure {
			Expect(errs[1]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckCompositeFunction(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := ParseAndCheck(fmt.Sprintf(`
            %[1]s X {
                fun foo(): ((): %[2]sX) {
                    return self.bar
                }

                fun bar(): %[2]sX {
                    return self
                }
            }
	    `, kind.Keyword(), kind.Annotation()))

		// TODO: add support for non-structure declarations

		if kind == common.CompositeKindStructure {
			Expect(err).
				To(Not(HaveOccurred()))
		} else {
			errs := ExpectCheckerErrors(err, 1)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}

func TestCheckCompositeReferenceBeforeDeclaration(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := ParseAndCheck(fmt.Sprintf(`
          var tests = 0

          fun test(): %[1]sTest {
              return %[2]s Test()
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
			Expect(err).
				To(Not(HaveOccurred()))
		} else {
			errs := ExpectCheckerErrors(err, 1)

			Expect(errs[0]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}
