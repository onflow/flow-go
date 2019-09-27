package checker

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/stdlib"
	. "github.com/dapperlabs/flow-go/pkg/language/runtime/tests/utils"
	. "github.com/onsi/gomega"
	"testing"
)

func TestCheckNilCoalescingNilIntToOptional(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      let one = 1
      let none: Int? = nil
      let x: Int? = none ?? one
    `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckNilCoalescingNilIntToOptionals(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      let one = 1
      let none: Int?? = nil
      let x: Int? = none ?? one
    `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckNilCoalescingNilIntToOptionalNilLiteral(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      let one = 1
      let x: Int? = nil ?? one
    `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidNilCoalescingMismatch(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      let x: Int? = nil ?? false
    `)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckNilCoalescingRightSubtype(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      let x: Int? = nil ?? nil
    `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckNilCoalescingNilInt(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      let one = 1
      let none: Int? = nil
      let x: Int = none ?? one
    `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidNilCoalescingOptionalsInt(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      let one = 1
      let none: Int?? = nil
      let x: Int = none ?? one
    `)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckNilCoalescingNilLiteralInt(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
     let one = 1
     let x: Int = nil ?? one
   `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidNilCoalescingMismatchNonOptional(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
     let x: Int = nil ?? false
   `)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckInvalidNilCoalescingRightSubtype(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
     let x: Int = nil ?? nil
   `)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckInvalidNilCoalescingNonMatchingTypes(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      let x: Int? = 1
      let y = x ?? false
   `)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.InvalidBinaryOperandError{}))
}

func TestCheckNilCoalescingAny(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
     let x: Any? = 1
     let y = x ?? false
  `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckNilCoalescingOptionalRightHandSide(t *testing.T) {
	RegisterTestingT(t)

	checker, err := ParseAndCheck(`
     let x: Int? = 1
     let y: Int? = 2
     let z = x ?? y
  `)

	Expect(err).
		To(Not(HaveOccurred()))

	Expect(checker.GlobalValues["z"].Type).
		To(BeAssignableToTypeOf(&sema.OptionalType{Type: &sema.IntType{}}))
}

func TestCheckNilCoalescingBothOptional(t *testing.T) {
	RegisterTestingT(t)

	checker, err := ParseAndCheck(`
     let x: Int?? = 1
     let y: Int? = 2
     let z = x ?? y
  `)

	Expect(err).
		To(Not(HaveOccurred()))

	Expect(checker.GlobalValues["z"].Type).
		To(BeAssignableToTypeOf(&sema.OptionalType{Type: &sema.IntType{}}))
}

func TestCheckNilCoalescingWithNever(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheckWithExtra(
		`
          let x: Int? = nil
          let y = x ?? panic("nope")
        `,
		stdlib.StandardLibraryFunctions{
			stdlib.PanicFunction,
		}.ToValueDeclarations(),
		nil,
		nil,
	)

	Expect(err).
		To(Not(HaveOccurred()))
}
