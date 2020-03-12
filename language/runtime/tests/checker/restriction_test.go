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
		_, err := ParseAndCheck(t, `
            resource R {}

            let r: @R{} <- create R()
        `)

		require.NoError(t, err)
	})

	t.Run("one restriction", func(t *testing.T) {
		_, err := ParseAndCheck(t, `
            resource interface I1 {}

            resource interface I2 {}

            resource R: I1, I2 {}

            let r: @R{I1} <- create R()
        `)

		require.NoError(t, err)
	})

	t.Run("reference to restriction", func(t *testing.T) {
		_, err := ParseAndCheck(t, `
            resource R {}

            let r <- create R()
            let ref: &R{} = &r as &R
        `)

		require.NoError(t, err)
	})

	t.Run("non-conformance restriction", func(t *testing.T) {
		_, err := ParseAndCheck(t, `
            resource interface I {}

            // NOTE: R does not conform to I
            resource R {}

            let r: @R{I} <- create R()
        `)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.InvalidNonConformanceRestrictionError{}, errs[0])
	})

	t.Run("duplicate restriction", func(t *testing.T) {
		_, err := ParseAndCheck(t, `
            resource interface I {}

            resource R: I {}

            // NOTE: I is duplicated
            let r: @R{I, I} <- create R()
        `)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.InvalidRestrictionTypeDuplicateError{}, errs[0])
	})

	t.Run("non-resource interface restriction", func(t *testing.T) {
		_, err := ParseAndCheck(t, `
            struct interface I {}

            resource R: I {}

            let r: @R{I} <- create R()
        `)

		errs := ExpectCheckerErrors(t, err, 2)

		assert.IsType(t, &sema.CompositeKindMismatchError{}, errs[0])
		assert.IsType(t, &sema.InvalidRestrictionTypeError{}, errs[1])
	})

	t.Run("non-resource restriction", func(t *testing.T) {
		_, err := ParseAndCheck(t, `
            struct interface I {}

            struct S: I {}

            let r: S{I} = S()
        `)

		errs := ExpectCheckerErrors(t, err, 5)

		assert.IsType(t, &sema.InvalidRestrictedTypeError{}, errs[0])
		assert.IsType(t, &sema.InvalidRestrictionTypeError{}, errs[1])
		assert.IsType(t, &sema.MissingResourceAnnotationError{}, errs[2])
		assert.IsType(t, &sema.TypeMismatchError{}, errs[3])
		assert.IsType(t, &sema.IncorrectTransferOperationError{}, errs[4])
	})

	t.Run("non-concrete resource restriction", func(t *testing.T) {
		_, err := ParseAndCheck(t, `
            resource interface I {}

            resource R: I {}

            let r: @[R]{I} <- [<-create R()]
        `)

		errs := ExpectCheckerErrors(t, err, 2)

		assert.IsType(t, &sema.InvalidRestrictedTypeError{}, errs[0])
		assert.IsType(t, &sema.TypeMismatchError{}, errs[1])
	})

	t.Run("resource interface restriction", func(t *testing.T) {
		_, err := ParseAndCheck(t, `
            resource interface I {}

            resource R: I {}

            let r: @I{} <- create R()
        `)

		errs := ExpectCheckerErrors(t, err, 2)

		assert.IsType(t, &sema.InvalidRestrictedTypeError{}, errs[0])
		assert.IsType(t, &sema.TypeMismatchError{}, errs[1])

	})
}

func TestCheckRestrictedResourceTypeMemberAccess(t *testing.T) {

	t.Run("no restrictions", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
            resource R {
                let n: Int

                init(n: Int) {
                    self.n = n
                }
            }

            fun test() {
                let r: @R{} <- create R(n: 1)
                r.n
                destroy r
            }
        `)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.InvalidRestrictedTypeMemberAccessError{}, errs[0])
	})

	t.Run("restriction with member", func(t *testing.T) {
		_, err := ParseAndCheck(t, `

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
                let r: @R{I} <- create R(n: 1)
                r.n
                destroy r
            }
        `)

		require.NoError(t, err)
	})

	t.Run("restriction without member", func(t *testing.T) {
		_, err := ParseAndCheck(t, `

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
                let r: @R{I} <- create R(n: 1)
                r.n
                destroy r
            }
        `)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.InvalidRestrictedTypeMemberAccessError{}, errs[0])
	})

	t.Run("restrictions with clashing members", func(t *testing.T) {
		_, err := ParseAndCheck(t, `

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
                let r: @R{I1, I2} <- create R(n: 1)
                r.n
                destroy r
            }
        `)

		errs := ExpectCheckerErrors(t, err, 2)

		assert.IsType(t, &sema.ConformanceError{}, errs[0])
		assert.IsType(t, &sema.RestrictionMemberClashError{}, errs[1])
	})
}

func TestCheckRestrictedResourceTypeSubtyping(t *testing.T) {

	t.Run("resource type to restricted resource type with same type, no restriction", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
            resource R {}

            fun test() {
                let r: @R{} <- create R()
                destroy r
            }
        `)

		require.NoError(t, err)
	})

	t.Run("resource type to restricted resource type with same type, one restriction", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
            resource interface I1 {}

            resource interface I2 {}

            resource R: I1, I2 {}

            fun test() {
                let r: @R{I1} <- create R()
                destroy r
            }
        `)

		require.NoError(t, err)
	})

	t.Run("resource type to restricted resource type with different restricted type", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
            resource R {}

            resource S {}

            fun test() {
                let s: @S{} <- create R()
                destroy s
            }
        `)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
	})

	t.Run("restricted resource type to restricted resource type with same type, no restrictions", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
            resource R {}

            fun test() {
                let r: @R{} <- create R()
                let r2: @R{} <- r
                destroy r2
            }
        `)

		require.NoError(t, err)
	})

	t.Run("restricted resource type to restricted resource type with same type, 0 to 1 restriction", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
            resource interface I1 {}

            resource interface I2 {}

            resource R: I1, I2 {}

            fun test() {
                let r: @R{} <- create R()
                let r2: @R{I1} <- r
                destroy r2
            }
        `)

		require.NoError(t, err)
	})

	t.Run("restricted resource type to restricted resource type with same type, 1 to 2 restrictions", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
            resource interface I1 {}

            resource interface I2 {}

            resource R: I1, I2 {}

            fun test() {
                let r: @R{I2} <- create R()
                let r2: @R{I1, I2} <- r
                destroy r2
            }
        `)

		require.NoError(t, err)
	})

	t.Run("restricted resource type to restricted resource type with same type, reordered restrictions", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
            resource interface I1 {}

            resource interface I2 {}

            resource R: I1, I2 {}

            fun test() {
                let r: @R{I2, I1} <- create R()
                let r2: @R{I1, I2} <- r
                destroy r2
            }
        `)

		require.NoError(t, err)
	})

	t.Run("restricted resource type to restricted resource type with same type, fewer restrictions", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
            resource interface I1 {}

            resource interface I2 {}

            resource R: I1, I2 {}

            fun test() {
                let r: @R{I1, I2} <- create R()
                let r2: @R{I2} <- r
                destroy r2
            }
        `)

		require.NoError(t, err)
	})

	t.Run("restricted resource type to resource type", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
            resource interface I1 {}

            resource interface I2 {}

            resource R: I1, I2 {}

            fun test() {
                let r: @R{I1} <- create R()
                let r2: @R <- r
                destroy r2
            }
        `)

		require.NoError(t, err)
	})
}

func TestCheckRestrictedResourceTypeNoType(t *testing.T) {

	const types = `
      resource interface I1 {}

      resource interface I2 {}
    `

	t.Run("resource: empty", func(t *testing.T) {

		checker, err := ParseAndCheckWithPanic(t,
			types+`
              let r: @{} <- panic("")
            `,
		)

		require.NoError(t, err)

		rType := checker.GlobalValues["r"].Type
		require.IsType(t, &sema.RestrictedResourceType{}, rType)

		ty := rType.(*sema.RestrictedResourceType)

		assert.IsType(t, &sema.AnyResourceType{}, ty.Type)

		require.Len(t, ty.Restrictions, 0)
	})

	t.Run("resource: one", func(t *testing.T) {

		checker, err := ParseAndCheckWithPanic(t,
			types+`
              let r: @{I1} <- panic("")
            `,
		)

		require.NoError(t, err)

		rType := checker.GlobalValues["r"].Type
		require.IsType(t, &sema.RestrictedResourceType{}, rType)

		ty := rType.(*sema.RestrictedResourceType)

		assert.IsType(t, &sema.AnyResourceType{}, ty.Type)

		require.Len(t, ty.Restrictions, 1)
		assert.Same(t,
			checker.GlobalTypes["I1"].Type,
			ty.Restrictions[0],
		)
	})

	t.Run("resource: two", func(t *testing.T) {

		checker, err := ParseAndCheckWithPanic(t,
			types+`
              let r: @{I1, I2} <- panic("")
            `,
		)

		require.NoError(t, err)

		rType := checker.GlobalValues["r"].Type
		require.IsType(t, &sema.RestrictedResourceType{}, rType)

		ty := rType.(*sema.RestrictedResourceType)

		assert.IsType(t, &sema.AnyResourceType{}, ty.Type)

		require.Len(t, ty.Restrictions, 2)
		assert.Same(t,
			checker.GlobalTypes["I1"].Type,
			ty.Restrictions[0],
		)
		assert.Same(t,
			checker.GlobalTypes["I2"].Type,
			ty.Restrictions[1],
		)
	})

	t.Run("reference: empty", func(t *testing.T) {

		checker, err := ParseAndCheckWithPanic(t,
			types+`
              let ref: &{} = panic("")
            `,
		)

		require.NoError(t, err)

		refType := checker.GlobalValues["ref"].Type
		require.IsType(t, &sema.ReferenceType{}, refType)

		rType := refType.(*sema.ReferenceType).Type
		require.IsType(t, &sema.RestrictedResourceType{}, rType)

		ty := rType.(*sema.RestrictedResourceType)

		assert.IsType(t, &sema.AnyResourceType{}, ty.Type)

		require.Len(t, ty.Restrictions, 0)
	})

	t.Run("reference: one", func(t *testing.T) {

		checker, err := ParseAndCheckWithPanic(t,
			types+`
              let ref: &{I1} = panic("")
            `,
		)

		require.NoError(t, err)

		refType := checker.GlobalValues["ref"].Type
		require.IsType(t, &sema.ReferenceType{}, refType)

		rType := refType.(*sema.ReferenceType).Type
		require.IsType(t, &sema.RestrictedResourceType{}, rType)

		ty := rType.(*sema.RestrictedResourceType)

		assert.IsType(t, &sema.AnyResourceType{}, ty.Type)

		require.Len(t, ty.Restrictions, 1)
		assert.Same(t,
			checker.GlobalTypes["I1"].Type,
			ty.Restrictions[0],
		)
	})

	t.Run("reference: two", func(t *testing.T) {

		checker, err := ParseAndCheckWithPanic(t,
			types+`
              let ref: &{I1, I2} = panic("")
            `,
		)

		require.NoError(t, err)

		refType := checker.GlobalValues["ref"].Type
		require.IsType(t, &sema.ReferenceType{}, refType)

		rType := refType.(*sema.ReferenceType).Type
		require.IsType(t, &sema.RestrictedResourceType{}, rType)

		ty := rType.(*sema.RestrictedResourceType)

		assert.IsType(t, &sema.AnyResourceType{}, ty.Type)

		require.Len(t, ty.Restrictions, 2)
		assert.Same(t,
			checker.GlobalTypes["I1"].Type,
			ty.Restrictions[0],
		)
		assert.Same(t,
			checker.GlobalTypes["I2"].Type,
			ty.Restrictions[1],
		)
	})
}
