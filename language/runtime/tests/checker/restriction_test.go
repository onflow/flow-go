package checker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/language/runtime/tests/utils"
)

// TODO: replace panics with actual resource instantiation once subtyping is implemented

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

	t.Run("reference to restriction", func(t *testing.T) {
		_, err := ParseAndCheckWithPanic(t, `
            resource R {}

            let r: &R{} = panic("")
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

	t.Run("resource interface restriction", func(t *testing.T) {
		_, err := ParseAndCheckWithPanic(t, `
            resource interface I {}

            let r: @I{} <- panic("")
        `)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.InvalidRestrictedTypeError{}, errs[0])
	})
}

func TestCheckRestrictedResourceTypeMemberAccess(t *testing.T) {

	t.Run("no restrictions", func(t *testing.T) {

		_, err := ParseAndCheckWithPanic(t, `
            resource R {
                let n: Int

                init(n: Int) {
                    self.n = n
                }
            }

            fun test() {
                let r: @R{} <- panic("")
                r.n
                destroy r
            }
        `)

		errs := ExpectCheckerErrors(t, err, 2)

		assert.IsType(t, &sema.UnreachableStatementError{}, errs[0])
		assert.IsType(t, &sema.InvalidRestrictedTypeMemberAccessError{}, errs[1])
	})

	t.Run("restriction with member", func(t *testing.T) {
		_, err := ParseAndCheckWithPanic(t, `

            resource interface I {
                let n: Int
            }

            resource R: I {
                let n: Int

                init(n: Int) {
                    self.n = n
                }
            }

            fun test() {
                let r: @R{I} <- panic("")
                r.n
                destroy r
            }
        `)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.UnreachableStatementError{}, errs[0])
	})

	t.Run("restriction without member", func(t *testing.T) {
		_, err := ParseAndCheckWithPanic(t, `

            resource interface I {
                // NOTE: no declaration for 'n'
            }

            resource R: I {
                let n: Int

                init(n: Int) {
                    self.n = n
                }
            }

            fun test() {
                let r: @R{I} <- panic("")
                r.n
                destroy r
            }
        `)

		errs := ExpectCheckerErrors(t, err, 2)

		assert.IsType(t, &sema.UnreachableStatementError{}, errs[0])
		assert.IsType(t, &sema.InvalidRestrictedTypeMemberAccessError{}, errs[1])
	})

	t.Run("restrictions with clashing members", func(t *testing.T) {
		_, err := ParseAndCheckWithPanic(t, `

            resource interface I1 {
                let n: Int
            }

            resource interface I2 {
                let n: Bool
            }

            resource R: I1, I2 {
                let n: Int

                init(n: Int) {
                    self.n = n
                }
            }

            fun test() {
                let r: @R{I1, I2} <- panic("")
                r.n
                destroy r
            }
        `)

		errs := ExpectCheckerErrors(t, err, 3)

		assert.IsType(t, &sema.ConformanceError{}, errs[0])
		assert.IsType(t, &sema.RestrictionMemberClashError{}, errs[1])
		assert.IsType(t, &sema.UnreachableStatementError{}, errs[2])
	})
}
