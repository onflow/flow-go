package checker

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/language/runtime/common"
	"github.com/dapperlabs/flow-go/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/language/runtime/tests/utils"
)

func TestCheckOptional(t *testing.T) {

	_, err := ParseAndCheck(t, `
      let x: Int? = 1
    `)

	require.NoError(t, err)
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

	require.NoError(t, err)
}

func TestCheckNil(t *testing.T) {

	_, err := ParseAndCheck(t, `
     let x: Int? = nil
   `)

	require.NoError(t, err)
}

func TestCheckOptionalNestingNil(t *testing.T) {

	_, err := ParseAndCheck(t, `
     let x: Int?? = nil
   `)

	require.NoError(t, err)
}

func TestCheckNilReturnValue(t *testing.T) {

	_, err := ParseAndCheck(t, `
     fun test(): Int?? {
         return nil
     }
   `)

	require.NoError(t, err)
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

	require.NoError(t, err)
}

func TestCheckOptionalNilComparison(t *testing.T) {

	_, err := ParseAndCheck(t, `
     let x: Int? = 1
     let y = x == nil
   `)

	require.NoError(t, err)
}

func TestCheckNonOptionalNilComparison(t *testing.T) {

	_, err := ParseAndCheck(t, `
     let x: Int = 1
     let y = x == nil
   `)

	require.NoError(t, err)
}

func TestCheckNonOptionalNilComparisonSwapped(t *testing.T) {

	_, err := ParseAndCheck(t, `
     let x: Int = 1
     let y = nil == x
     let z = x == nil
   `)

	require.NoError(t, err)
}

func TestCheckOptionalNilComparisonSwapped(t *testing.T) {

	_, err := ParseAndCheck(t, `
     let x: Int? = 1
     let y = nil == x
   `)

	require.NoError(t, err)
}

func TestCheckNestedOptionalNilComparisonSwapped(t *testing.T) {

	_, err := ParseAndCheck(t, `
     let x: Int?? = 1
     let y = nil == x
   `)

	require.NoError(t, err)
}

func TestCheckNestedOptionalComparison(t *testing.T) {

	_, err := ParseAndCheck(t, `
     let x: Int? = nil
     let y: Int?? = nil
     let z = x == y
   `)

	require.NoError(t, err)
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

func TestCheckCompositeNilEquality(t *testing.T) {

	for _, kind := range common.CompositeKinds {

		_, err := ParseAndCheck(t,
			fmt.Sprintf(
				`
                  %[1]s X {}

                  let x: %[2]sX? %[3]s %[4]s X()

                  let a = x == nil
                  let b = nil == x
                `,
				kind.Keyword(),
				kind.Annotation(),
				kind.TransferOperator(),
				kind.ConstructionKeyword(),
			),
		)

		require.NoError(t, err)
	}
}

func TestCheckInvalidCompositeNilEquality(t *testing.T) {

	for _, kind := range common.CompositeKinds {

		_, err := ParseAndCheck(t,
			fmt.Sprintf(
				`
                  %[1]s X {}

                  let x: %[2]sX? %[3]s %[4]s X()
                  let y: %[2]sX? %[3]s nil

                  let a = x == y
                `,
				kind.Keyword(),
				kind.Annotation(),
				kind.TransferOperator(),
				kind.ConstructionKeyword(),
			),
		)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.InvalidBinaryOperandsError{}, errs[0])
	}
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

func TestCheckInvalidOptionalIntegerConversion(t *testing.T) {

	_, err := ParseAndCheck(t, `
      let x: Int8? = 1
      let y: Int16? = x
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}
