package checker

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/language/runtime/sema"
	"github.com/dapperlabs/flow-go/language/runtime/stdlib"
	. "github.com/dapperlabs/flow-go/language/runtime/tests/utils"
)

func TestCheckNilCoalescingNilIntToOptional(t *testing.T) {

	_, err := ParseAndCheck(t, `
      let one = 1
      let none: Int? = nil
      let x: Int? = none ?? one
    `)

	assert.Nil(t, err)
}

func TestCheckNilCoalescingNilIntToOptionals(t *testing.T) {

	_, err := ParseAndCheck(t, `
      let one = 1
      let none: Int?? = nil
      let x: Int? = none ?? one
    `)

	assert.Nil(t, err)
}

func TestCheckNilCoalescingNilIntToOptionalNilLiteral(t *testing.T) {

	_, err := ParseAndCheck(t, `
      let one = 1
      let x: Int? = nil ?? one
    `)

	assert.Nil(t, err)
}

func TestCheckInvalidNilCoalescingMismatch(t *testing.T) {

	_, err := ParseAndCheck(t, `
      let x: Int? = nil ?? false
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckNilCoalescingRightSubtype(t *testing.T) {

	_, err := ParseAndCheck(t, `
      let x: Int? = nil ?? nil
    `)

	assert.Nil(t, err)
}

func TestCheckNilCoalescingNilInt(t *testing.T) {

	_, err := ParseAndCheck(t, `
      let one = 1
      let none: Int? = nil
      let x: Int = none ?? one
    `)

	assert.Nil(t, err)
}

func TestCheckInvalidNilCoalescingOptionalsInt(t *testing.T) {

	_, err := ParseAndCheck(t, `
      let one = 1
      let none: Int?? = nil
      let x: Int = none ?? one
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckNilCoalescingNilLiteralInt(t *testing.T) {

	_, err := ParseAndCheck(t, `
     let one = 1
     let x: Int = nil ?? one
   `)

	assert.Nil(t, err)
}

func TestCheckInvalidNilCoalescingMismatchNonOptional(t *testing.T) {

	_, err := ParseAndCheck(t, `
     let x: Int = nil ?? false
   `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckInvalidNilCoalescingRightSubtype(t *testing.T) {

	_, err := ParseAndCheck(t, `
     let x: Int = nil ?? nil
   `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckInvalidNilCoalescingNonMatchingTypes(t *testing.T) {

	_, err := ParseAndCheck(t, `
      let x: Int? = 1
      let y = x ?? false
   `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidBinaryOperandError{}, errs[0])
}

func TestCheckNilCoalescingAny(t *testing.T) {

	_, err := ParseAndCheck(t, `
     let x: Any? = 1
     let y = x ?? false
  `)

	assert.Nil(t, err)
}

func TestCheckNilCoalescingOptionalRightHandSide(t *testing.T) {

	checker, err := ParseAndCheck(t, `
     let x: Int? = 1
     let y: Int? = 2
     let z = x ?? y
  `)

	assert.Nil(t, err)

	assert.IsType(t, &sema.OptionalType{Type: &sema.IntType{}}, checker.GlobalValues["z"].Type)
}

func TestCheckNilCoalescingBothOptional(t *testing.T) {

	checker, err := ParseAndCheck(t, `
     let x: Int?? = 1
     let y: Int? = 2
     let z = x ?? y
  `)

	assert.Nil(t, err)

	assert.IsType(t, &sema.OptionalType{Type: &sema.IntType{}}, checker.GlobalValues["z"].Type)
}

func TestCheckNilCoalescingWithNever(t *testing.T) {

	_, err := ParseAndCheckWithOptions(t,
		`
          let x: Int? = nil
          let y = x ?? panic("nope")
        `,
		ParseAndCheckOptions{
			Values: stdlib.StandardLibraryFunctions{
				stdlib.PanicFunction,
			}.ToValueDeclarations(),
		},
	)

	assert.Nil(t, err)
}
