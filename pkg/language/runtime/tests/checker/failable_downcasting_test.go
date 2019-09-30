package checker

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/pkg/language/runtime/tests/utils"
	. "github.com/onsi/gomega"
	"testing"
)

func TestCheckFailableDowncastingAny(t *testing.T) {
	RegisterTestingT(t)

	checker, err := ParseAndCheck(`
      let x: Any = 1
      let y: Int? = x as? Int
    `)

	Expect(err).
		To(Not(HaveOccurred()))

	Expect(checker.Elaboration.FailableDowncastingTypes).
		To(Not(BeEmpty()))
}

func TestCheckInvalidFailableDowncastingAny(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      let x: Any = 1
      let y: Bool? = x as? Int
    `)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

// TODO: add support for statically known casts
func TestCheckInvalidFailableDowncastingStaticallyKnown(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      let x: Int = 1
      let y: Int? = x as? Int
    `)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedTypeError{}))
}

// TODO: add support for interfaces
// TODO: add test this is *INVALID* for resources
func TestCheckInvalidFailableDowncastingInterface(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      struct interface I {}

      struct S: I {}

      let x: I = S()
      let y: S? = x as? S
    `)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedTypeError{}))
}

// TODO: add support for "wrapped" Any: optional, array, dictionary
func TestCheckInvalidFailableDowncastingOptionalAny(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      let x: Any? = 1
      let y: Int?? = x as? Int?
    `)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedTypeError{}))
}

// TODO: add support for "wrapped" Any: optional, array, dictionary
func TestCheckInvalidFailableDowncastingArrayAny(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      let x: [Any] = [1]
      let y: [Int]? = x as? [Int]
    `)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedTypeError{}))
}

func TestCheckOptionalAnyFailableDowncastingNil(t *testing.T) {
	RegisterTestingT(t)

	checker, err := ParseAndCheck(`
      let x: Any? = nil
      let y = x ?? 23
      let z = y as? Int
    `)

	Expect(err).
		To(Not(HaveOccurred()))

	Expect(checker.GlobalValues["x"].Type).
		To(Equal(&sema.OptionalType{Type: &sema.AnyType{}}))

	// TODO: record result type of conditional and box to any in interpreter
	Expect(checker.GlobalValues["y"].Type).
		To(Equal(&sema.AnyType{}))

	Expect(checker.GlobalValues["z"].Type).
		To(Equal(&sema.OptionalType{Type: &sema.IntType{}}))
}
