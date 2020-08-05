/*
 * Cadence - The resource-oriented smart contract programming language
 *
 * Copyright 2019-2020 Dapper Labs, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package checker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence/runtime/sema"
)

func TestCheckRestrictedType(t *testing.T) {

	t.Parallel()

	t.Run("resource: no restrictions", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
            resource R {}

            let r: @R{} <- create R()
        `)

		require.NoError(t, err)
	})

	t.Run("struct: no restrictions", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
            struct S {}

            let r: S{} = S()
        `)

		require.NoError(t, err)
	})

	t.Run("resource: one restriction", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
            resource interface I1 {}

            resource interface I2 {}

            resource R: I1, I2 {}

            let r: @R{I1} <- create R()
        `)

		require.NoError(t, err)
	})

	t.Run("struct: one restriction", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
            struct interface I1 {}

            struct interface I2 {}

            struct S: I1, I2 {}

            let r: S{I1} = S()
        `)

		require.NoError(t, err)
	})

	t.Run("reference to resource restriction", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
            resource R {}

            let r <- create R()
            let ref: &R{} = &r as &R
        `)

		require.NoError(t, err)
	})

	t.Run("reference to struct restriction", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
            struct S {}

            let s = S()
            let ref: &S{} = &s as &S
        `)

		require.NoError(t, err)
	})

	t.Run("resource: non-conformance restriction", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
            resource interface I {}

            // NOTE: R does not conform to I
            resource R {}

            let r: @R{I} <- create R()
        `)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.InvalidNonConformanceRestrictionError{}, errs[0])
	})

	t.Run("struct: non-conformance restriction", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
            struct interface I {}

            // NOTE: S does not conform to I
            struct S {}

            let s: S{I} = S()
        `)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.InvalidNonConformanceRestrictionError{}, errs[0])
	})

	t.Run("resource: duplicate restriction", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
            resource interface I {}

            resource R: I {}

            // NOTE: I is duplicated
            let r: @R{I, I} <- create R()
        `)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.InvalidRestrictionTypeDuplicateError{}, errs[0])
	})

	t.Run("struct: duplicate restriction", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
            struct interface I {}

            struct S: I {}

            // NOTE: I is duplicated
            let s: S{I, I} = S()
        `)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.InvalidRestrictionTypeDuplicateError{}, errs[0])
	})

	t.Run("restricted resource, with structure interface restriction", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
            struct interface I {}

            resource R: I {}

            let r: @R{I} <- create R()
        `)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.CompositeKindMismatchError{}, errs[0])
	})

	t.Run("restricted struct, with resource interface restriction", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
            resource interface I {}

            struct S: I {}

            let s: S{I} = S()
        `)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.CompositeKindMismatchError{}, errs[0])
	})

	t.Run("resource: non-concrete restricted type", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
            resource interface I {}

            resource R: I {}

            let r: @[R]{I} <- [<-create R()]
        `)

		errs := ExpectCheckerErrors(t, err, 2)

		assert.IsType(t, &sema.InvalidRestrictedTypeError{}, errs[0])
		assert.IsType(t, &sema.TypeMismatchError{}, errs[1])
	})

	t.Run("struct: non-concrete restricted type", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
            struct interface I {}

            struct S: I {}

            let s: [S]{I} = [S()]
        `)

		errs := ExpectCheckerErrors(t, err, 2)

		assert.IsType(t, &sema.InvalidRestrictedTypeError{}, errs[0])
		assert.IsType(t, &sema.TypeMismatchError{}, errs[1])
	})

	t.Run("restricted resource interface ", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
            resource interface I {}

            resource R: I {}

            let r: @I{} <- create R()
        `)

		errs := ExpectCheckerErrors(t, err, 2)

		assert.IsType(t, &sema.InvalidRestrictedTypeError{}, errs[0])
		assert.IsType(t, &sema.TypeMismatchError{}, errs[1])
	})

	t.Run("restricted struct interface", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
            struct interface I {}

            struct S: I {}

            let s: I{} = S()
        `)

		errs := ExpectCheckerErrors(t, err, 2)

		assert.IsType(t, &sema.InvalidRestrictedTypeError{}, errs[0])
		assert.IsType(t, &sema.TypeMismatchError{}, errs[1])
	})
}

func TestCheckRestrictedTypeMemberAccess(t *testing.T) {

	t.Parallel()

	t.Run("no restrictions: resource", func(t *testing.T) {

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

	t.Run("no restrictions: struct", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
            struct S {
                let n: Int

                init(n: Int) {
                    self.n = n
                }
            }

            fun test() {
                let s: S{} = S(n: 1)
                s.n
            }
        `)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.InvalidRestrictedTypeMemberAccessError{}, errs[0])
	})

	t.Run("restriction with member: resource", func(t *testing.T) {

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

	t.Run("restriction with member: struct", func(t *testing.T) {

		_, err := ParseAndCheck(t, `

            struct interface I {
                let n: Int
            }

            struct S: I {
                let n: Int

                init(n: Int) {
                    self.n = n
                }
            }

            fun test() {
                let s: S{I} = S(n: 1)
                s.n
            }
        `)

		require.NoError(t, err)
	})

	t.Run("restriction without member: resource", func(t *testing.T) {

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

	t.Run("restriction without member: struct", func(t *testing.T) {

		_, err := ParseAndCheck(t, `

            struct interface I {
                // NOTE: no declaration for 'n'
            }

            struct S: I {
                let n: Int

                init(n: Int) {
                    self.n = n
                }
            }

            fun test() {
                let s: S{I} = S(n: 1)
                s.n
            }
        `)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.InvalidRestrictedTypeMemberAccessError{}, errs[0])
	})

	t.Run("restrictions with clashing members: resource", func(t *testing.T) {

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

	t.Run("restrictions with clashing members: struct", func(t *testing.T) {

		_, err := ParseAndCheck(t, `

            struct interface I1 {
                let n: Int
            }

            struct interface I2 {
                let n: Bool
            }

            struct S: I1, I2 {
                let n: Int

                init(n: Int) {
                    self.n = n
                }
            }

            fun test() {
                let s: S{I1, I2} = S(n: 1)
                s.n
            }
        `)

		errs := ExpectCheckerErrors(t, err, 2)

		assert.IsType(t, &sema.ConformanceError{}, errs[0])
		assert.IsType(t, &sema.RestrictionMemberClashError{}, errs[1])
	})
}

func TestCheckRestrictedTypeSubtyping(t *testing.T) {

	t.Parallel()

	t.Run("resource type to restricted type with same type, no restriction", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
            resource R {}

            fun test() {
                let r: @R{} <- create R()
                destroy r
            }
        `)

		require.NoError(t, err)
	})

	t.Run("struct type to restricted type with same type, no restriction", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
            struct S {}

            let s: S{} = S()
        `)

		require.NoError(t, err)
	})

	t.Run("resource type to restricted type with same type, one restriction", func(t *testing.T) {

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

	t.Run("struct type to restricted type with same type, one restriction", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
            struct interface I1 {}

            struct interface I2 {}

            struct S: I1, I2 {}

            let s: S{I1} = S()
        `)

		require.NoError(t, err)
	})

	t.Run("resource type to restricted type with different restricted type", func(t *testing.T) {

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

	t.Run("struct type to restricted type with different restricted type", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
            struct R {}

            struct S {}

            let s: S{} = R()
        `)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
	})

	t.Run("restricted resource type to restricted type with same type, no restrictions", func(t *testing.T) {

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

	t.Run("restricted struct type to restricted type with same type, no restrictions", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
            struct S {}

            fun test() {
                let s: S{} = S()
                let s2: S{} = s
            }
        `)

		require.NoError(t, err)
	})

	t.Run("restricted resource type to restricted type with same type, 0 to 1 restriction", func(t *testing.T) {

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

	t.Run("restricted struct type to restricted type with same type, 0 to 1 restriction", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
            struct interface I1 {}

            struct interface I2 {}

            struct S: I1, I2 {}

            let s: S{} = S()
            let s2: S{I1} = s
        `)

		require.NoError(t, err)
	})

	t.Run("restricted resource type to restricted type with same type, 1 to 2 restrictions", func(t *testing.T) {

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

	t.Run("restricted struct type to restricted type with same type, 1 to 2 restrictions", func(t *testing.T) {

		_, err := ParseAndCheck(t, `

            struct interface I1 {}

            struct interface I2 {}

            struct S: I1, I2 {}

            let s: S{I2} = S()
            let s2: S{I1, I2} = s
        `)

		require.NoError(t, err)
	})

	t.Run("restricted resource type to restricted type with same type, reordered restrictions", func(t *testing.T) {

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

	t.Run("restricted struct type to restricted type with same type, reordered restrictions", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
            struct interface I1 {}

            struct interface I2 {}

            struct S: I1, I2 {}

            let s: S{I2, I1} = S()
            let s2: S{I1, I2} = s
        `)

		require.NoError(t, err)
	})

	t.Run("restricted resource type to restricted type with same type, fewer restrictions", func(t *testing.T) {

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

	t.Run("restricted struct type to restricted type with same type, fewer restrictions", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
            struct interface I1 {}

            struct interface I2 {}

            struct S: I1, I2 {}

            let s: S{I1, I2} = S()
            let s2: S{I2} = s
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

	t.Run("restricted struct type to struct type", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
            struct interface I1 {}

            struct interface I2 {}

            struct S: I1, I2 {}

            let s: S{I1} = S()
            let s2: S = s
        `)

		require.NoError(t, err)
	})
}

func TestCheckRestrictedTypeNoType(t *testing.T) {

	t.Parallel()

	const resourceTypes = `
      resource interface I1 {}

      resource interface I2 {}
    `

	const structTypes = `
      struct interface I1 {}

      struct interface I2 {}
    `

	t.Run("resource: empty", func(t *testing.T) {

		_, err := ParseAndCheckWithPanic(t,
			resourceTypes+`
              let r: @{} <- panic("")
            `,
		)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.AmbiguousRestrictedTypeError{}, errs[0])
	})

	t.Run("struct: empty", func(t *testing.T) {

		_, err := ParseAndCheckWithPanic(t,
			structTypes+`
              let s: {} = panic("")
            `,
		)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.AmbiguousRestrictedTypeError{}, errs[0])
	})

	t.Run("resource: one", func(t *testing.T) {

		checker, err := ParseAndCheckWithPanic(t,
			resourceTypes+`
              let r: @{I1} <- panic("")
            `,
		)

		require.NoError(t, err)

		rType := checker.GlobalValues["r"].Type
		require.IsType(t, &sema.RestrictedType{}, rType)

		ty := rType.(*sema.RestrictedType)

		assert.IsType(t, &sema.AnyResourceType{}, ty.Type)

		require.Len(t, ty.Restrictions, 1)
		assert.Same(t,
			checker.GlobalTypes["I1"].Type,
			ty.Restrictions[0],
		)
	})

	t.Run("struct: one", func(t *testing.T) {

		checker, err := ParseAndCheckWithPanic(t,
			structTypes+`
              let s: {I1} = panic("")
            `,
		)

		require.NoError(t, err)

		rType := checker.GlobalValues["s"].Type
		require.IsType(t, &sema.RestrictedType{}, rType)

		ty := rType.(*sema.RestrictedType)

		assert.IsType(t, &sema.AnyStructType{}, ty.Type)

		require.Len(t, ty.Restrictions, 1)
		assert.Same(t,
			checker.GlobalTypes["I1"].Type,
			ty.Restrictions[0],
		)
	})

	t.Run("resource: two", func(t *testing.T) {

		checker, err := ParseAndCheckWithPanic(t,
			resourceTypes+`
              let r: @{I1, I2} <- panic("")
            `,
		)

		require.NoError(t, err)

		rType := checker.GlobalValues["r"].Type
		require.IsType(t, &sema.RestrictedType{}, rType)

		ty := rType.(*sema.RestrictedType)

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

	t.Run("struct: two", func(t *testing.T) {

		checker, err := ParseAndCheckWithPanic(t,
			structTypes+`
              let s: {I1, I2} = panic("")
            `,
		)

		require.NoError(t, err)

		rType := checker.GlobalValues["s"].Type
		require.IsType(t, &sema.RestrictedType{}, rType)

		ty := rType.(*sema.RestrictedType)

		assert.IsType(t, &sema.AnyStructType{}, ty.Type)

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

		_, err := ParseAndCheckWithPanic(t,
			resourceTypes+`
              let ref: &{} = panic("")
            `,
		)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.AmbiguousRestrictedTypeError{}, errs[0])
	})

	t.Run("resource reference: one", func(t *testing.T) {

		checker, err := ParseAndCheckWithPanic(t,
			resourceTypes+`
              let ref: &{I1} = panic("")
            `,
		)

		require.NoError(t, err)

		refType := checker.GlobalValues["ref"].Type
		require.IsType(t, &sema.ReferenceType{}, refType)

		rType := refType.(*sema.ReferenceType).Type
		require.IsType(t, &sema.RestrictedType{}, rType)

		ty := rType.(*sema.RestrictedType)

		assert.IsType(t, &sema.AnyResourceType{}, ty.Type)

		require.Len(t, ty.Restrictions, 1)
		assert.Same(t,
			checker.GlobalTypes["I1"].Type,
			ty.Restrictions[0],
		)
	})

	t.Run("struct reference: one", func(t *testing.T) {

		checker, err := ParseAndCheckWithPanic(t,
			structTypes+`
              let ref: &{I1} = panic("")
            `,
		)

		require.NoError(t, err)

		refType := checker.GlobalValues["ref"].Type
		require.IsType(t, &sema.ReferenceType{}, refType)

		rType := refType.(*sema.ReferenceType).Type
		require.IsType(t, &sema.RestrictedType{}, rType)

		ty := rType.(*sema.RestrictedType)

		assert.IsType(t, &sema.AnyStructType{}, ty.Type)

		require.Len(t, ty.Restrictions, 1)
		assert.Same(t,
			checker.GlobalTypes["I1"].Type,
			ty.Restrictions[0],
		)
	})

	t.Run("resource reference: two", func(t *testing.T) {

		checker, err := ParseAndCheckWithPanic(t,
			resourceTypes+`
              let ref: &{I1, I2} = panic("")
            `,
		)

		require.NoError(t, err)

		refType := checker.GlobalValues["ref"].Type
		require.IsType(t, &sema.ReferenceType{}, refType)

		rType := refType.(*sema.ReferenceType).Type
		require.IsType(t, &sema.RestrictedType{}, rType)

		ty := rType.(*sema.RestrictedType)

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

	t.Run("struct reference: two", func(t *testing.T) {

		checker, err := ParseAndCheckWithPanic(t,
			structTypes+`
              let ref: &{I1, I2} = panic("")
            `,
		)

		require.NoError(t, err)

		refType := checker.GlobalValues["ref"].Type
		require.IsType(t, &sema.ReferenceType{}, refType)

		rType := refType.(*sema.ReferenceType).Type
		require.IsType(t, &sema.RestrictedType{}, rType)

		ty := rType.(*sema.RestrictedType)

		assert.IsType(t, &sema.AnyStructType{}, ty.Type)

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

func TestCheckRestrictedTypeConformanceOrder(t *testing.T) {

	t.Run("valid", func(t *testing.T) {

		// Test that the conformances for a composite are declared
		// before functions using them are checked

		_, err := ParseAndCheckWithPanic(t, `
          contract C {
              resource interface RI {}
              resource R: RI {}
              fun foo(): &R{RI} { panic("") }
          }
        `)

		require.NoError(t, err)
	})

	t.Run("invalid", func(t *testing.T) {

		_, err := ParseAndCheckWithPanic(t, `
          contract C {
              resource interface RI {}
              resource R {}
              fun foo(): &R{RI} { panic("") }
          }
        `)

		// TODO: remove duplicate

		errs := ExpectCheckerErrors(t, err, 2)

		assert.IsType(t, &sema.InvalidNonConformanceRestrictionError{}, errs[0])
		assert.IsType(t, &sema.InvalidNonConformanceRestrictionError{}, errs[1])
	})

}
