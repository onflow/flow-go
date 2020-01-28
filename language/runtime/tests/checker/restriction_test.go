package checker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/language/runtime/tests/utils"
)

func TestCheckRestrictedResourceType(t *testing.T) {

	t.Run("no restrictions", func(t *testing.T) {
		_, err := ParseAndCheckWithPanic(t, `
            resource R {}

            let r: @R{} <- panic("") 
        `)

		require.NoError(t, err)
	})

	t.Run("one restriction", func(t *testing.T) {
		_, err := ParseAndCheckWithPanic(t, `
            resource interface I1 {}

            resource interface I2 {}

            resource R: I1, I2 {}

            let r: @R{I1} <- panic("") 
        `)

		require.NoError(t, err)
	})

	t.Run("non-conformance restriction", func(t *testing.T) {
		_, err := ParseAndCheckWithPanic(t, `
            resource interface I {}

            // NOTE: R does not conform to I
            resource R {}

            let r: @R{I} <- panic("") 
        `)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.InvalidNonConformanceRestrictionError{}, errs[0])
	})

	t.Run("duplicate restriction", func(t *testing.T) {
		_, err := ParseAndCheckWithPanic(t, `
            resource interface I {}

            resource R: I {}

            // NOTE: I is duplicated
            let r: @R{I, I} <- panic("") 
        `)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.InvalidRestrictionTypeDuplicateError{}, errs[0])
	})

	t.Run("non-resource interface restriction", func(t *testing.T) {
		_, err := ParseAndCheckWithPanic(t, `
            struct interface I {}

            resource R: I {}

            let r: @R{I} <- panic("") 
        `)

		errs := ExpectCheckerErrors(t, err, 2)

		assert.IsType(t, &sema.CompositeKindMismatchError{}, errs[0])
		assert.IsType(t, &sema.InvalidRestrictionTypeError{}, errs[1])
	})

	t.Run("non-resource restriction", func(t *testing.T) {
		_, err := ParseAndCheckWithPanic(t, `
            struct interface I {}

            struct S: I {}

            let r: S{I} <- panic("") 
        `)

		errs := ExpectCheckerErrors(t, err, 3)

		assert.IsType(t, &sema.InvalidRestrictedTypeError{}, errs[0])
		assert.IsType(t, &sema.InvalidRestrictionTypeError{}, errs[1])
		assert.IsType(t, &sema.MissingResourceAnnotationError{}, errs[2])
	})

	t.Run("non-concrete resource restriction", func(t *testing.T) {
		_, err := ParseAndCheckWithPanic(t, `
            resource interface I {}

            resource R: I {}

            let r: @[R]{I} <- panic("")
        `)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.InvalidRestrictedTypeError{}, errs[0])
	})
}
