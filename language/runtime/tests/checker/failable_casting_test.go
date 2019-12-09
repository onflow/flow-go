package checker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/language/runtime/tests/utils"
)

func TestCheckFailableCastingAny(t *testing.T) {

	checker, err := ParseAndCheck(t, `
      let x: AnyStruct = 1
      let y: Int? = x as? Int
    `)

	require.NoError(t, err)

	assert.NotEmpty(t, checker.Elaboration.CastingTargetTypes)
}

func TestCheckInvalidFailableCastingAny(t *testing.T) {

	_, err := ParseAndCheck(t, `
      let x: AnyStruct = 1
      let y: Bool? = x as? Int
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

// TODO: add support for statically known casts
func TestCheckInvalidFailableCastingStaticallyKnown(t *testing.T) {

	_, err := ParseAndCheck(t, `
      let x: Int = 1
      let y: Int? = x as? Int
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.UnsupportedTypeError{}, errs[0])
}

// TODO: add support for interfaces
// TODO: add test this is *INVALID* for resources
func TestCheckInvalidFailableCastingInterface(t *testing.T) {

	_, err := ParseAndCheck(t, `
      struct interface I {}

      struct S: I {}

      let x: I = S()
      let y: S? = x as? S
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.UnsupportedTypeError{}, errs[0])
}

// TODO: add support for "wrapped" Any: optional, array, dictionary
func TestCheckInvalidFailableCastingOptionalAny(t *testing.T) {

	_, err := ParseAndCheck(t, `
      let x: AnyStruct? = 1
      let y: Int?? = x as? Int?
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.UnsupportedTypeError{}, errs[0])
}

// TODO: add support for "wrapped" Any: optional, array, dictionary
func TestCheckInvalidFailableCastingArrayAny(t *testing.T) {

	_, err := ParseAndCheck(t, `
      let x: [AnyStruct] = [1]
      let y: [Int]? = x as? [Int]
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.UnsupportedTypeError{}, errs[0])
}

func TestCheckOptionalAnyFailableCastingNil(t *testing.T) {

	checker, err := ParseAndCheck(t, `
      let x: AnyStruct? = nil
      let y = x ?? 23
      let z = y as? Int
    `)

	require.NoError(t, err)

	assert.Equal(t,
		&sema.OptionalType{Type: &sema.AnyStructType{}},
		checker.GlobalValues["x"].Type,
	)

	// TODO: record result type of conditional and box to any in interpreter
	assert.Equal(t,
		&sema.AnyStructType{},
		checker.GlobalValues["y"].Type,
	)

	assert.Equal(t,
		&sema.OptionalType{Type: &sema.IntType{}},
		checker.GlobalValues["z"].Type,
	)
}
