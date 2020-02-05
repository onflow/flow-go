package checker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/language/runtime/tests/utils"
)

func TestCheckReferenceTypeSubTyping(t *testing.T) {

	_, err := ParseAndCheckStorage(t, `
          resource interface RI {}

          resource R: RI {}

          let ref = &storage[R] as &RI
          let ref2: &RI = ref
        `,
	)

	require.NoError(t, err)
}

func TestCheckInvalidReferenceTypeSubTyping(t *testing.T) {

	_, err := ParseAndCheckStorage(t, `
          resource interface RI {}

          // NOTE: R does not conform to RI
          resource R {}

          let ref = &storage[R] as &RI
        `,
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckReferenceTypeOuter(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource R {}

      fun test(r: &[R]) {}
    `)

	require.NoError(t, err)
}

func TestCheckReferenceTypeInner(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource R {}

      fun test(r: [&R]) {}
    `)

	require.NoError(t, err)
}

func TestCheckNestedReferenceType(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource R {}

      fun test(r: &[&R]) {}
    `)

	require.Error(t, err)
}

func TestCheckInvalidReferenceType(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test(r: &R) {}
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
}

func TestCheckReferenceExpressionWithResourceResultType(t *testing.T) {

	checker, err := ParseAndCheckStorage(t, `
          resource R {}

          let ref = &storage[R] as &R
        `,
	)

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
}

func TestCheckReferenceExpressionWithResourceInterfaceResultType(t *testing.T) {

	_, err := ParseAndCheckStorage(t, `
          resource interface T {}
          resource R: T {}

          let ref = &storage[R] as &T
        `,
	)

	require.NoError(t, err)
}

func TestCheckInvalidReferenceExpressionType(t *testing.T) {

	_, err := ParseAndCheckStorage(t, `
          resource R {}

          let ref = &storage[R] as &X
        `,
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
}

func TestCheckInvalidReferenceExpressionStorageIndexType(t *testing.T) {

	_, err := ParseAndCheckStorage(t, `
          resource R {}

          let ref = &storage[X] as &R
        `,
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
}

func TestCheckInvalidReferenceExpressionNonResourceReferencedType(t *testing.T) {

	_, err := ParseAndCheckStorage(t, `
          struct S {}
          resource R {}

          let ref = &storage[S] as &R
        `,
	)

	errs := ExpectCheckerErrors(t, err, 3)

	assert.IsType(t, &sema.TypeMismatchWithDescriptionError{}, errs[0])
	assert.IsType(t, &sema.NonResourceTypeReferenceError{}, errs[1])
	assert.IsType(t, &sema.TypeMismatchError{}, errs[2])
}

func TestCheckInvalidReferenceExpressionNonResourceResultType(t *testing.T) {

	_, err := ParseAndCheckStorage(t, `
          resource R {}
          struct S {}

          let ref = &storage[R] as &S
        `,
	)

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.NonResourceReferenceTypeError{}, errs[0])
	assert.IsType(t, &sema.TypeMismatchError{}, errs[1])
}

func TestCheckInvalidReferenceExpressionNonResourceTypes(t *testing.T) {

	_, err := ParseAndCheckStorage(t, `
          struct S {}
          struct T {}

          let ref = &storage[S] as &T
        `,
	)

	errs := ExpectCheckerErrors(t, err, 4)

	assert.IsType(t, &sema.TypeMismatchWithDescriptionError{}, errs[0])
	assert.IsType(t, &sema.NonResourceTypeReferenceError{}, errs[1])
	assert.IsType(t, &sema.NonResourceReferenceTypeError{}, errs[2])
	assert.IsType(t, &sema.TypeMismatchError{}, errs[3])
}

func TestCheckInvalidReferenceExpressionTypeMismatch(t *testing.T) {

	_, err := ParseAndCheckStorage(t, `
          resource R {}
          resource T {}

          let ref = &storage[R] as &T
        `,
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckInvalidReferenceToNonIndex(t *testing.T) {

	_, err := ParseAndCheckStorage(t, `
          resource R {}

          let r <- create R()
          let ref = &r as &R
        `,
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NonStorageReferenceError{}, errs[0])
}

func TestCheckInvalidReferenceToNonStorage(t *testing.T) {

	_, err := ParseAndCheckStorage(t, `
          resource R {}

          let rs <- [<-create R()]
          let ref = &rs[0] as &R
        `,
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NonStorageReferenceError{}, errs[0])
}

func TestCheckReferenceUse(t *testing.T) {

	_, err := ParseAndCheckStorage(t, `
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
              var r: @R? <- create R()
              storage[R] <-> r
              // there was no old value, but it must be discarded
              destroy r

              let ref = &storage[R] as &R
              ref.x = 1
              let x1 = ref.x
              ref.setX(2)
              let x2 = ref.x
              return [x1, x2]
          }
        `,
	)

	require.NoError(t, err)
}

func TestCheckReferenceUseArray(t *testing.T) {

	_, err := ParseAndCheckStorage(t, `
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
              var rs: @[R]? <- [<-create R()]
              storage[[R]] <-> rs
              // there was no old value, but it must be discarded
              destroy rs

              let ref = &storage[[R]] as &[R]
              ref[0].x = 1
              let x1 = ref[0].x
              ref[0].setX(2)
              let x2 = ref[0].x
              return [x1, x2]
          }
        `,
	)

	require.NoError(t, err)
}

func TestCheckReferenceIndexingIfReferencedIndexable(t *testing.T) {

	_, err := ParseAndCheckStorage(t, `
          resource R {}

          fun test() {
              var rs: @[R]? <- [<-create R()]
              storage[[R]] <-> rs
              // there was no old value, but it must be discarded
              destroy rs

              let ref = &storage[[R]] as &[R]
              var other <- create R()
              ref[0] <-> other
              destroy other
          }
        `,
	)

	require.NoError(t, err)
}

func TestCheckInvalidReferenceResourceLoss(t *testing.T) {

	_, err := ParseAndCheckStorage(t, `
          resource R {}

          fun test() {
              var rs: @[R]? <- [<-create R()]
              storage[[R]] <-> rs
              // there was no old value, but it must be discarded
              destroy rs

              let ref = &storage[[R]] as &[R]
              ref[0]
          }
        `,
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ResourceLossError{}, errs[0])
}

func TestCheckInvalidReferenceIndexingIfReferencedNotIndexable(t *testing.T) {

	_, err := ParseAndCheckStorage(t, `
          resource R {}

          fun test() {
              var r: @R? <- create R()
              storage[R] <-> r
              // there was no old value, but it must be discarded
              destroy r

              let ref = &storage[R] as &R
              ref[0]
          }
        `,
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotIndexableTypeError{}, errs[0])
}

func TestCheckResourceInterfaceReferenceFunctionCall(t *testing.T) {

	_, err := ParseAndCheckStorage(t, `
          resource interface I {
              fun foo()
          }

          resource R: I {
              fun foo() {}
          }

          fun test() {
              var r: @R? <- create R()
              storage[R] <-> r
              // there was no old value, but it must be discarded
              destroy r

              let ref = &storage[R] as &I
              ref.foo()
          }
        `,
	)

	require.NoError(t, err)
}

func TestCheckInvalidResourceInterfaceReferenceFunctionCall(t *testing.T) {

	_, err := ParseAndCheckStorage(t, `
          resource interface I {}

          resource R: I {
              fun foo() {}
          }

          fun test() {
              var r: @R? <- create R()
              storage[R] <-> r
              // there was no old value, but it must be discarded
              destroy r

              let ref = &storage[R] as &I
              ref.foo()
          }
        `,
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotDeclaredMemberError{}, errs[0])
}

func TestCheckAuthReferenceTypeSubTyping(t *testing.T) {

	// TODO: replace panic with actual reference expression
	// once it supports non-storage references

	t.Run("auth to non-auth", func(t *testing.T) {
		_, err := ParseAndCheckStorage(t, `
          resource R {}

          let ref: auth &R = panic("")
          let ref2: &R = ref
        `,
		)

		require.NoError(t, err)
	})

	t.Run("non-auth to auth", func(t *testing.T) {
		_, err := ParseAndCheckStorage(t, `
          resource R {}

          let ref: &R = panic("")
          let ref2: auth &R = ref
        `,
		)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
	})
}

func TestCheckReferenceExpressionReferenceType(t *testing.T) {

	t.Run("non-auth reference", func(t *testing.T) {

		checker, err := ParseAndCheckStorage(t, `
          resource R {}

          let ref = &storage[R] as &R
        `,
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

		assert.False(t, referenceType.Authorized)

	})

	t.Run("auth reference", func(t *testing.T) {

		checker, err := ParseAndCheckStorage(t, `
          resource R {}

          let ref = &storage[R] as auth &R
        `,
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

		assert.True(t, referenceType.Authorized)

	})

}
