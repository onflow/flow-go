package checker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/language/runtime/tests/utils"
)

func TestCheckCastingIntLiteralToInt8(t *testing.T) {

	checker, err := ParseAndCheck(t, `
      let x = 1 as Int8
    `)

	require.Nil(t, err)

	assert.Equal(t,
		&sema.Int8Type{},
		checker.GlobalValues["x"].Type,
	)

	assert.NotEmpty(t, checker.Elaboration.CastingTargetTypes)
}

func TestCheckInvalidCastingIntLiteralToString(t *testing.T) {

	_, err := ParseAndCheck(t, `
      let x = 1 as String
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckCastingIntLiteralToAnyStruct(t *testing.T) {

	checker, err := ParseAndCheck(t, `
      let x = 1 as AnyStruct
    `)

	require.Nil(t, err)

	assert.Equal(t,
		&sema.AnyStructType{},
		checker.GlobalValues["x"].Type,
	)

	assert.NotEmpty(t, checker.Elaboration.CastingTargetTypes)
}

func TestCheckCastingResourceToAnyResource(t *testing.T) {

	checker, err := ParseAndCheck(t, `
      resource R {}

      fun test() {
          let r <- create R()
          let x <- r as @AnyResource
          destroy x
      }
    `)

	require.Nil(t, err)

	assert.NotEmpty(t, checker.Elaboration.CastingTargetTypes)
}

func TestCheckCastingArrayLiteral(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun zipOf3(a: [AnyStruct; 3], b: [Int; 3]): [[AnyStruct; 2]; 3] {
          return [
              [a[0], b[0]] as [AnyStruct; 2],
              [a[1], b[1]] as [AnyStruct; 2],
              [a[2], b[2]] as [AnyStruct; 2]
          ]
      }
    `)

	require.Nil(t, err)
}
