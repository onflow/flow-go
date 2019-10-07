package checker

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/pkg/language/runtime/tests/utils"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCheckOptional(t *testing.T) {

	_, err := ParseAndCheck(t, `
      let x: Int? = 1
    `)

	assert.Nil(t, err)
}

func TestCheckInvalidOptional(t *testing.T) {

	_, err := ParseAndCheck(t, `
      let x: Int? = false
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckOptionalNesting(t *testing.T) {

	_, err := ParseAndCheck(t, `
      let x: Int?? = 1
    `)

	assert.Nil(t, err)
}

func TestCheckNil(t *testing.T) {

	_, err := ParseAndCheck(t, `
     let x: Int? = nil
   `)

	assert.Nil(t, err)
}

func TestCheckOptionalNestingNil(t *testing.T) {

	_, err := ParseAndCheck(t, `
     let x: Int?? = nil
   `)

	assert.Nil(t, err)
}

func TestCheckNilReturnValue(t *testing.T) {

	_, err := ParseAndCheck(t, `
     fun test(): Int?? {
         return nil
     }
   `)

	assert.Nil(t, err)
}

func TestCheckInvalidNonOptionalNil(t *testing.T) {

	_, err := ParseAndCheck(t, `
      let x: Int = nil
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckNilsComparison(t *testing.T) {

	_, err := ParseAndCheck(t, `
     let x = nil == nil
   `)

	assert.Nil(t, err)
}

func TestCheckOptionalNilComparison(t *testing.T) {

	_, err := ParseAndCheck(t, `
     let x: Int? = 1
     let y = x == nil
   `)

	assert.Nil(t, err)
}

func TestCheckNonOptionalNilComparison(t *testing.T) {

	_, err := ParseAndCheck(t, `
     let x: Int = 1
     let y = x == nil
   `)

	assert.Nil(t, err)
}

func TestCheckNonOptionalNilComparisonSwapped(t *testing.T) {

	_, err := ParseAndCheck(t, `
     let x: Int = 1
     let y = nil == x
   `)

	assert.Nil(t, err)
}

func TestCheckNestedOptionalNilComparison(t *testing.T) {

	_, err := ParseAndCheck(t, `
     let x: Int?? = 1
     let y = x == nil
   `)

	assert.Nil(t, err)
}

func TestCheckOptionalNilComparisonSwapped(t *testing.T) {

	_, err := ParseAndCheck(t, `
     let x: Int? = 1
     let y = nil == x
   `)

	assert.Nil(t, err)
}

func TestCheckNestedOptionalNilComparisonSwapped(t *testing.T) {

	_, err := ParseAndCheck(t, `
     let x: Int?? = 1
     let y = nil == x
   `)

	assert.Nil(t, err)
}

func TestCheckNestedOptionalComparison(t *testing.T) {

	_, err := ParseAndCheck(t, `
     let x: Int? = nil
     let y: Int?? = nil
     let z = x == y
   `)

	assert.Nil(t, err)
}

func TestCheckInvalidNestedOptionalComparison(t *testing.T) {

	_, err := ParseAndCheck(t, `
     let x: Int? = nil
     let y: Bool?? = nil
     let z = x == y
   `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidBinaryOperandsError{}, errs[0])
}

func TestCheckInvalidNonOptionalReturn(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test(x: Int?): Int {
          return x
      }
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}
