package checker

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/pkg/language/runtime/tests/utils"
	. "github.com/onsi/gomega"
	"testing"
)

func TestCheckOptional(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      let x: Int? = 1
    `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidOptional(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      let x: Int? = false
    `)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckOptionalNesting(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      let x: Int?? = 1
    `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckNil(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
     let x: Int? = nil
   `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckOptionalNestingNil(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
     let x: Int?? = nil
   `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckNilReturnValue(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
     fun test(): Int?? {
         return nil
     }
   `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidNonOptionalNil(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      let x: Int = nil
    `)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckNilsComparison(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
     let x = nil == nil
   `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckOptionalNilComparison(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
     let x: Int? = 1
     let y = x == nil
   `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckNonOptionalNilComparison(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
     let x: Int = 1
     let y = x == nil
   `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckNonOptionalNilComparisonSwapped(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
     let x: Int = 1
     let y = nil == x
   `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckNestedOptionalNilComparison(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
     let x: Int?? = 1
     let y = x == nil
   `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckOptionalNilComparisonSwapped(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
     let x: Int? = 1
     let y = nil == x
   `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckNestedOptionalNilComparisonSwapped(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
     let x: Int?? = 1
     let y = nil == x
   `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckNestedOptionalComparison(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
     let x: Int? = nil
     let y: Int?? = nil
     let z = x == y
   `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidNestedOptionalComparison(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
     let x: Int? = nil
     let y: Bool?? = nil
     let z = x == y
   `)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.InvalidBinaryOperandsError{}))
}

func TestCheckInvalidNonOptionalReturn(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test(x: Int?): Int {
          return x
      }
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}
