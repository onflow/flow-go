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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/sema"
)

func TestCheckReferenceTypeOuter(t *testing.T) {

	t.Parallel()

	t.Run("resource", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
          resource R {}

          fun test(r: &[R]) {}
        `)

		require.NoError(t, err)
	})

	t.Run("struct", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
          struct S {}

          fun test(s: &[S]) {}
        `)

		require.NoError(t, err)
	})
}

func TestCheckReferenceTypeInner(t *testing.T) {

	t.Parallel()

	t.Run("resource", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
          resource R {}

          fun test(r: [&R]) {}
        `)

		require.NoError(t, err)
	})

	t.Run("struct", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
          struct S {}

          fun test(s: [&S]) {}
        `)

		require.NoError(t, err)
	})

}

func TestCheckNestedReferenceType(t *testing.T) {

	t.Parallel()

	t.Run("resource", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
          resource R {}

          fun test(r: &[&R]) {}
        `)

		require.NoError(t, err)
	})

	t.Run("struct", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
          struct S {}

          fun test(s: &[&S]) {}
        `)

		require.NoError(t, err)
	})
}

func TestCheckInvalidReferenceType(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      fun test(t: &T) {}
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[0])

}

func TestCheckReferenceExpressionWithCompositeResultType(t *testing.T) {

	t.Parallel()

	t.Run("resource", func(t *testing.T) {

		checker, err := ParseAndCheck(t, `
          resource R {}

          let r <- create R()
          let ref = &r as &R
        `)

		require.NoError(t, err)

		refValueType := checker.GlobalValues["ref"].Type

		assert.IsType(t,
			&sema.ReferenceType{},
			refValueType,
		)

		assert.IsType(t,
			&sema.CompositeType{},
			refValueType.(*sema.ReferenceType).Type,
		)
	})

	t.Run("struct", func(t *testing.T) {

		checker, err := ParseAndCheck(t, `
          struct S {}

          let s = S()
          let ref = &s as &S
        `)

		require.NoError(t, err)

		refValueType := checker.GlobalValues["ref"].Type

		assert.IsType(t,
			&sema.ReferenceType{},
			refValueType,
		)

		assert.IsType(t,
			&sema.CompositeType{},
			refValueType.(*sema.ReferenceType).Type,
		)
	})
}

func TestCheckReferenceExpressionWithInterfaceResultType(t *testing.T) {

	t.Parallel()

	t.Run("resource", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
          resource interface I {}
          resource R: I {}

          let r <- create R()
          let ref = &r as &I
        `)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
	})

	t.Run("struct", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
          struct interface I {}
          struct S: I {}

          let s = S()
          let ref = &s as &I
        `)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
	})
}

func TestCheckReferenceExpressionWithRestrictedAnyResultType(t *testing.T) {

	t.Parallel()

	t.Run("resource", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
          resource interface I {}
          resource R: I {}

          let r <- create R()
          let ref = &r as &AnyResource{I}
        `)

		require.NoError(t, err)
	})

	t.Run("struct", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
          struct interface I {}
          struct S: I {}

          let s = S()
          let ref = &s as &AnyStruct{I}
        `)

		require.NoError(t, err)
	})
}

func TestCheckInvalidReferenceExpressionType(t *testing.T) {

	t.Parallel()

	t.Run("resource", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
          resource R {}

          let r <- create R()
          let ref = &r as &X
        `)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
	})

	t.Run("struct", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
          struct S {}

          let s = S()
          let ref = &s as &X
        `)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
	})
}

func TestCheckInvalidReferenceExpressionTypeMismatchStructResource(t *testing.T) {

	t.Parallel()

	t.Run("struct / resource", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
          struct S {}
          resource R {}

          let s = S()
          let ref = &s as &R
        `)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
	})

	t.Run("resource / struct", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
          struct S {}
          resource R {}

          let r <- create R()
          let ref = &r as &S
        `)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
	})
}

func TestCheckInvalidReferenceExpressionDifferentStructs(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      struct S {}
      struct T {}

      let s = S()
      let ref = &s as &T
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckInvalidReferenceExpressionTypeMismatchDifferentResources(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      resource R {}
      resource T {}

      let r <- create R()
      let ref = &r as &T
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckReference(t *testing.T) {

	t.Parallel()

	t.Run("struct variable", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
          struct S {}

          let s = S()
          let ref = &s as &S
        `)

		require.NoError(t, err)
	})

	t.Run("resource variable", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
          resource R {}

          let r <- create R()
          let ref = &r as &R
        `)

		require.NoError(t, err)
	})

	t.Run("resource array indexing", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
          resource R {}

          let rs <- [<-create R()]
          let ref = &rs[0] as &R
        `)

		require.NoError(t, err)
	})
}

func TestCheckReferenceUse(t *testing.T) {

	t.Parallel()

	t.Run("resource", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
          resource R {
              var x: Int

              init() {
                  self.x = 0
              }

              fun setX(_ newX: Int) {
                  self.x = newX
              }
          }

          fun test(): [Int] {
              let r <- create R()
              let ref = &r as &R
              ref.x = 1
              let x1 = ref.x
              ref.setX(2)
              let x2 = ref.x
              destroy r
              return [x1, x2]
          }
        `)

		require.NoError(t, err)
	})

	t.Run("struct", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
          struct S {
              var x: Int

              init() {
                  self.x = 0
              }

              fun setX(_ newX: Int) {
                  self.x = newX
              }
          }

          fun test(): [Int] {
              let s = S()
              let ref = &s as &S
              ref.x = 1
              let x1 = ref.x
              ref.setX(2)
              let x2 = ref.x
              return [x1, x2]
          }
        `)

		require.NoError(t, err)
	})
}

func TestCheckReferenceUseArray(t *testing.T) {

	t.Parallel()

	t.Run("resource", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
          resource R {
              var x: Int

              init() {
                  self.x = 0
              }

              fun setX(_ newX: Int) {
                  self.x = newX
              }
          }

          fun test(): [Int] {
              let rs <- [<-create R()]
              let ref = &rs as &[R]
              ref[0].x = 1
              let x1 = ref[0].x
              ref[0].setX(2)
              let x2 = ref[0].x
              destroy rs
              return [x1, x2]
          }
        `)

		require.NoError(t, err)
	})

	t.Run("struct", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
          struct S {
              var x: Int

              init() {
                  self.x = 0
              }

              fun setX(_ newX: Int) {
                  self.x = newX
              }
          }

          fun test(): [Int] {
              let s = [S()]
              let ref = &s as &[S]
              ref[0].x = 1
              let x1 = ref[0].x
              ref[0].setX(2)
              let x2 = ref[0].x
              return [x1, x2]
          }
        `)

		require.NoError(t, err)
	})
}

func TestCheckReferenceIndexingIfReferencedIndexable(t *testing.T) {

	t.Parallel()

	t.Run("resource", func(t *testing.T) {

		_, err := ParseAndCheck(t, `

          resource R {}

          fun test() {
              let rs <- [<-create R()]
              let ref = &rs as &[R]
              var other <- create R()
              ref[0] <-> other
              destroy rs
              destroy other
          }
        `)

		require.NoError(t, err)
	})

	t.Run("struct", func(t *testing.T) {

		_, err := ParseAndCheck(t, `

          struct S {}

          fun test() {
              let s = [S()]
              let ref = &s as &[S]
              var other = S()
              ref[0] <-> other
          }
        `)

		require.NoError(t, err)
	})
}

func TestCheckInvalidReferenceResourceLoss(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      resource R {}

      fun test() {
          let rs <- [<-create R()]
          let ref = &rs as &[R]
          ref[0]
          destroy rs
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ResourceLossError{}, errs[0])
}

func TestCheckInvalidReferenceIndexingIfReferencedNotIndexable(t *testing.T) {

	t.Parallel()

	t.Run("resource", func(t *testing.T) {

		_, err := ParseAndCheck(t, `

          resource R {}

          fun test() {
              let r <- create R()
              let ref = &r as &R
              ref[0]
              destroy r
          }
        `)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.NotIndexableTypeError{}, errs[0])
	})

	t.Run("struct", func(t *testing.T) {

		_, err := ParseAndCheck(t, `

          struct S {}

          fun test() {
              let s = S()
              let ref = &s as &S
              ref[0]
          }
        `)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.NotIndexableTypeError{}, errs[0])
	})
}

func TestCheckResourceInterfaceReferenceFunctionCall(t *testing.T) {

	t.Parallel()

	t.Run("resource", func(t *testing.T) {

		_, err := ParseAndCheck(t, `

          resource interface I {
              fun foo()
          }

          resource R: I {
              fun foo() {}
          }

          fun test() {
              let r <- create R()
              let ref = &r as &AnyResource{I}
              ref.foo()
              destroy r
          }
        `)

		require.NoError(t, err)
	})

	t.Run("struct", func(t *testing.T) {

		_, err := ParseAndCheck(t, `

          struct interface I {
              fun foo()
          }

          struct S: I {
              fun foo() {}
          }

          fun test() {
              let s = S()
              let ref = &s as &AnyStruct{I}
              ref.foo()
          }
        `)

		require.NoError(t, err)
	})
}

func TestCheckInvalidResourceInterfaceReferenceFunctionCall(t *testing.T) {

	t.Parallel()

	t.Run("resource", func(t *testing.T) {

		_, err := ParseAndCheck(t, `

          resource interface I {}

          resource R: I {
              fun foo() {}
          }

          fun test() {
              let r <- create R()
              let ref = &r as &AnyResource{I}
              ref.foo()
              destroy r
          }
        `)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.NotDeclaredMemberError{}, errs[0])
	})

	t.Run("struct", func(t *testing.T) {

		_, err := ParseAndCheck(t, `

          struct interface I {}

          struct S: I {
              fun foo() {}
          }

          fun test() {
              let s = S()
              let ref = &s as &AnyStruct{I}
              ref.foo()
          }
        `)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.NotDeclaredMemberError{}, errs[0])
	})
}

func TestCheckReferenceExpressionReferenceType(t *testing.T) {

	t.Parallel()

	for _, kind := range []common.CompositeKind{
		common.CompositeKindResource,
		common.CompositeKindStructure,
	} {

		for _, auth := range []bool{true, false} {

			authKeyword := ""
			if auth {
				authKeyword = "auth"
			}

			t.Run(fmt.Sprintf("%s, auth: %v", kind.Name(), auth), func(t *testing.T) {

				checker, err := ParseAndCheck(t,
					fmt.Sprintf(
						`
                          %[1]s T {}

                          let t %[2]s %[3]s T()
                          let ref = &t as %[4]s &T
                        `,
						kind.Keyword(),
						kind.TransferOperator(),
						kind.ConstructionKeyword(),
						authKeyword,
					),
				)

				require.NoError(t, err)

				refValueType := checker.GlobalValues["ref"].Type

				require.IsType(t,
					&sema.ReferenceType{},
					refValueType,
				)

				referenceType := refValueType.(*sema.ReferenceType)

				assert.IsType(t,
					&sema.CompositeType{},
					referenceType.Type,
				)

				assert.Equal(t, referenceType.Authorized, auth)
			})
		}
	}
}

func TestCheckReferenceExpressionOfOptional(t *testing.T) {

	t.Parallel()

	t.Run("resource", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
          resource R {}

          let r: @R? <- create R()
          let ref = &r as &R
        `)

		errs := ExpectCheckerErrors(t, err, 2)

		assert.IsType(t, &sema.OptionalTypeReferenceError{}, errs[0])
		assert.IsType(t, &sema.TypeMismatchError{}, errs[1])
	})

	t.Run("struct", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
          struct S {}

          let s: S? = S()
          let ref = &s as &S
        `)

		errs := ExpectCheckerErrors(t, err, 2)

		assert.IsType(t, &sema.OptionalTypeReferenceError{}, errs[0])
		assert.IsType(t, &sema.TypeMismatchError{}, errs[1])
	})
}

func TestCheckInvalidReferenceExpressionNonReferenceAmbiguous(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      let y = &x as {}
    `)

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
	assert.IsType(t, &sema.AmbiguousRestrictedTypeError{}, errs[1])
}

func TestCheckInvalidReferenceExpressionNonReferenceAnyResource(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      let y = &x as AnyResource{}
    `)

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
	assert.IsType(t, &sema.NonReferenceTypeReferenceError{}, errs[1])
}

func TestCheckInvalidReferenceExpressionNonReferenceAnyStruct(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      let y = &x as AnyStruct{}
    `)

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
	assert.IsType(t, &sema.NonReferenceTypeReferenceError{}, errs[1])
}
