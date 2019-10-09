package checker

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/pkg/language/runtime/tests/utils"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCheckFailableDowncastingAny(t *testing.T) {

	checker, err := ParseAndCheck(t, `
      let x: Any = 1
      let y: Int? = x as? Int
    `)

	assert.Nil(t, err)

	assert.NotEmpty(t, checker.Elaboration.FailableDowncastingTypes)
}

func TestCheckInvalidFailableDowncastingAny(t *testing.T) {

	_, err := ParseAndCheck(t, `
      let x: Any = 1
      let y: Bool? = x as? Int
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

// TODO: add support for statically known casts
func TestCheckInvalidFailableDowncastingStaticallyKnown(t *testing.T) {

	_, err := ParseAndCheck(t, `
      let x: Int = 1
      let y: Int? = x as? Int
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.UnsupportedTypeError{}, errs[0])
}

// TODO: add support for interfaces
// TODO: add test this is *INVALID* for resources
func TestCheckInvalidFailableDowncastingInterface(t *testing.T) {

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
func TestCheckInvalidFailableDowncastingOptionalAny(t *testing.T) {

	_, err := ParseAndCheck(t, `
      let x: Any? = 1
      let y: Int?? = x as? Int?
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.UnsupportedTypeError{}, errs[0])
}

// TODO: add support for "wrapped" Any: optional, array, dictionary
func TestCheckInvalidFailableDowncastingArrayAny(t *testing.T) {

	_, err := ParseAndCheck(t, `
      let x: [Any] = [1]
      let y: [Int]? = x as? [Int]
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.UnsupportedTypeError{}, errs[0])
}

func TestCheckOptionalAnyFailableDowncastingNil(t *testing.T) {

	checker, err := ParseAndCheck(t, `
      let x: Any? = nil
      let y = x ?? 23
      let z = y as? Int
    `)

	assert.Nil(t, err)

	assert.Equal(t,
		&sema.OptionalType{Type: &sema.AnyType{}},
		checker.GlobalValues["x"].Type,
	)

	// TODO: record result type of conditional and box to any in interpreter
	assert.Equal(t,
		&sema.AnyType{},
		checker.GlobalValues["y"].Type,
	)

	assert.Equal(t,
		&sema.OptionalType{Type: &sema.IntType{}},
		checker.GlobalValues["z"].Type,
	)
}
