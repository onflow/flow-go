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

func TestCheckCastingIntLiteralToAny(t *testing.T) {

	checker, err := ParseAndCheck(t, `
      let x = 1 as Any
    `)

	require.Nil(t, err)

	assert.Equal(t,
		&sema.AnyType{},
		checker.GlobalValues["x"].Type,
	)

	assert.NotEmpty(t, checker.Elaboration.CastingTargetTypes)
}
