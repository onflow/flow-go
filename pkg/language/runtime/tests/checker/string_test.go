package checker

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/pkg/language/runtime/tests/utils"
	. "github.com/onsi/gomega"
	"testing"
)

func TestCheckCharacter(t *testing.T) {
	RegisterTestingT(t)

	checker, err := ParseAndCheck(`
        let x: Character = "x"
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	Expect(checker.GlobalValues["x"].Type).
		To(Equal(&sema.CharacterType{}))
}

func TestCheckCharacterUnicodeScalar(t *testing.T) {
	RegisterTestingT(t)

	checker, err := ParseAndCheck(`
        let x: Character = "\u{1F1FA}\u{1F1F8}"
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	Expect(checker.GlobalValues["x"].Type).
		To(Equal(&sema.CharacterType{}))
}

func TestCheckString(t *testing.T) {
	RegisterTestingT(t)

	checker, err := ParseAndCheck(`
        let x = "x"
	`)

	Expect(err).
		To(Not(HaveOccurred()))

	Expect(checker.GlobalValues["x"].Type).
		To(Equal(&sema.StringType{}))
}

func TestCheckStringConcat(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
	  fun test(): String {
	 	  let a = "abc"
		  let b = "def"
		  let c = a.concat(b)
		  return c
      }
    `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidStringConcat(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test(): String {
		  let a = "abc"
		  let b = [1, 2]
		  let c = a.concat(b)
		  return c
      }
    `)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckStringConcatBound(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test(): String {
		  let a = "abc"
		  let b = "def"
		  let c = a.concat
		  return c(b)
      }
    `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckStringSlice(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
	  fun test(): String {
	 	  let a = "abcdef"
		  return a.slice(from: 0, upTo: 1)
      }
    `)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidStringSlice(t *testing.T) {
	t.Run("MissingBothArgumentLabels", func(t *testing.T) {
		RegisterTestingT(t)

		_, err := ParseAndCheck(`
		  let a = "abcdef"
		  let x = a.slice(0, 1)
		`)

		errs := ExpectCheckerErrors(err, 2)

		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.MissingArgumentLabelError{}))
		Expect(errs[1]).
			To(BeAssignableToTypeOf(&sema.MissingArgumentLabelError{}))
	})

	t.Run("MissingOneArgumentLabel", func(t *testing.T) {
		RegisterTestingT(t)

		_, err := ParseAndCheck(`
		  let a = "abcdef"
		  let x = a.slice(from: 0, 1)
		`)

		errs := ExpectCheckerErrors(err, 1)

		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.MissingArgumentLabelError{}))
	})

	t.Run("InvalidArgumentType", func(t *testing.T) {
		RegisterTestingT(t)

		_, err := ParseAndCheck(`
		  let a = "abcdef"
		  let x = a.slice(from: "a", upTo: "b")
		`)

		errs := ExpectCheckerErrors(err, 2)

		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
		Expect(errs[1]).
			To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
	})
}

func TestCheckStringSliceBound(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test(): String {
		  let a = "abcdef"
		  let c = a.slice
		  return c(from: 0, upTo: 1)
      }
    `)

	Expect(err).
		To(Not(HaveOccurred()))
}

// TODO: prevent invalid character literals
// func TestCheckInvalidCharacterLiteral(t *testing.T) {
// 	RegisterTestingT(t)
//
// 	_, err := ParseAndCheck(`
//         let x: Character = "abc"
// 	`)
//
// 	errs := ExpectCheckerErrors(err, 1)
//
// 	Expect(errs[0]).
// 		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
// }

// TODO: prevent assignment with invalid character literal
// func TestCheckStringIndexingAssignmentWithInvalidCharacterLiteral(t *testing.T) {
// 	RegisterTestingT(t)
//
// 	_, err := ParseAndCheck(`
//       fun test() {
//           let z = "abc"
//           z[0] = "def"
//       }
// 	`)
//
// 	errs := ExpectCheckerErrors(err, 1)
//
// 	Expect(errs[0]).
// 		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
// }

func TestCheckStringIndexing(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test() {
          let z = "abc"
          let y: Character = z[0]
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckStringIndexingAssignment(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test() {
		  let z = "abc"
		  let y: Character = "d"
          z[0] = y
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckStringIndexingAssignmentWithCharacterLiteral(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test() {
          let z = "abc"
          z[0] = "d"
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}
