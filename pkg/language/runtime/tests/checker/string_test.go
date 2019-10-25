package checker

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/pkg/language/runtime/tests/utils"
)

func TestCheckCharacter(t *testing.T) {

	checker, err := ParseAndCheck(t, `
        let x: Character = "x"
	`)

	assert.Nil(t, err)

	assert.Equal(t,
		&sema.CharacterType{},
		checker.GlobalValues["x"].Type,
	)
}

func TestCheckCharacterUnicodeScalar(t *testing.T) {

	checker, err := ParseAndCheck(t, `
        let x: Character = "\u{1F1FA}\u{1F1F8}"
	`)

	assert.Nil(t, err)

	assert.Equal(t,
		&sema.CharacterType{},
		checker.GlobalValues["x"].Type,
	)
}

func TestCheckString(t *testing.T) {

	checker, err := ParseAndCheck(t, `
        let x = "x"
	`)

	assert.Nil(t, err)

	assert.Equal(t,
		&sema.StringType{},
		checker.GlobalValues["x"].Type,
	)
}

func TestCheckStringConcat(t *testing.T) {

	_, err := ParseAndCheck(t, `
	  fun test(): String {
	 	  let a = "abc"
		  let b = "def"
		  let c = a.concat(b)
		  return c
      }
    `)

	assert.Nil(t, err)
}

func TestCheckInvalidStringConcat(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test(): String {
		  let a = "abc"
		  let b = [1, 2]
		  let c = a.concat(b)
		  return c
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckStringConcatBound(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test(): String {
		  let a = "abc"
		  let b = "def"
		  let c = a.concat
		  return c(b)
      }
    `)

	assert.Nil(t, err)
}

func TestCheckStringSlice(t *testing.T) {

	_, err := ParseAndCheck(t, `
	  fun test(): String {
	 	  let a = "abcdef"
		  return a.slice(from: 0, upTo: 1)
      }
    `)

	assert.Nil(t, err)
}

func TestCheckInvalidStringSlice(t *testing.T) {
	t.Run("MissingBothArgumentLabels", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
		  let a = "abcdef"
		  let x = a.slice(0, 1)
		`)

		errs := ExpectCheckerErrors(t, err, 2)

		assert.IsType(t, &sema.MissingArgumentLabelError{}, errs[0])
		assert.IsType(t, &sema.MissingArgumentLabelError{}, errs[1])
	})

	t.Run("MissingOneArgumentLabel", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
		  let a = "abcdef"
		  let x = a.slice(from: 0, 1)
		`)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.MissingArgumentLabelError{}, errs[0])
	})

	t.Run("InvalidArgumentType", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
		  let a = "abcdef"
		  let x = a.slice(from: "a", upTo: "b")
		`)

		errs := ExpectCheckerErrors(t, err, 2)

		assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
		assert.IsType(t, &sema.TypeMismatchError{}, errs[1])
	})
}

func TestCheckStringSliceBound(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test(): String {
		  let a = "abcdef"
		  let c = a.slice
		  return c(from: 0, upTo: 1)
      }
    `)

	assert.Nil(t, err)
}

// TODO: prevent invalid character literals
// func TestCheckInvalidCharacterLiteral(t *testing.T) {
// 	//
// 	_, err := ParseAndCheck(t, `
//         let x: Character = "abc"
// 	`)
//
// 	errs := ExpectCheckerErrors(t, err, 1)
//
// 	Expect(errs[0]).
// 		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
// }

// TODO: prevent assignment with invalid character literal
// func TestCheckStringIndexingAssignmentWithInvalidCharacterLiteral(t *testing.T) {
// 	//
// 	_, err := ParseAndCheck(t, `
//       fun test() {
//           let z = "abc"
//           z[0] = "def"
//       }
// 	`)
//
// 	errs := ExpectCheckerErrors(t, err, 1)
//
// 	Expect(errs[0]).
// 		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
// }

func TestCheckStringIndexing(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
          let z = "abc"
          let y: Character = z[0]
      }
	`)

	assert.Nil(t, err)
}

func TestCheckStringIndexingAssignment(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
		  let z = "abc"
		  let y: Character = "d"
          z[0] = y
      }
	`)

	assert.Nil(t, err)
}

func TestCheckStringIndexingAssignmentWithCharacterLiteral(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
          let z = "abc"
          z[0] = "d"
      }
	`)

	assert.Nil(t, err)
}
