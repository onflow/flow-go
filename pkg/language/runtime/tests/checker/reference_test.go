package checker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/pkg/language/runtime/tests/utils"
)

func TestCheckReferenceTypeOuter(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource R {}

      fun test(r: &[R]) {}
    `)

	assert.Nil(t, err)
}

func TestCheckReferenceTypeInner(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource R {}

      fun test(r: [&R]) {}
    `)

	assert.Nil(t, err)
}

func TestCheckNestedReferenceType(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource R {}

      fun test(r: &[&R]) {}
    `)

	assert.Nil(t, err)
}

func TestCheckInvalidReferenceType(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test(r: &R) {}
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
}

func TestCheckReferenceExpressionWithResourceResultType(t *testing.T) {

	checker, err := ParseAndCheckWithExtra(t, `
          resource R {}

          let ref = &storage[R] as R
        `,
		storageValueDeclaration,
		nil,
		nil,
		nil,
	)

	require.Nil(t, err)

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

	_, err := ParseAndCheckWithExtra(t, `
          resource interface T {}
          resource R: T {}

          let ref = &storage[R] as T
        `,
		storageValueDeclaration,
		nil,
		nil,
		nil,
	)

	assert.Nil(t, err)
}

func TestCheckInvalidReferenceExpressionType(t *testing.T) {

	_, err := ParseAndCheckWithExtra(t, `
          resource R {}

          let ref = &storage[R] as X
        `,
		storageValueDeclaration,
		nil,
		nil,
		nil,
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
}

func TestCheckInvalidReferenceExpressionStorageIndexType(t *testing.T) {

	_, err := ParseAndCheckWithExtra(t, `
          resource R {}

          let ref = &storage[X] as R
        `,
		storageValueDeclaration,
		nil,
		nil,
		nil,
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
}

func TestCheckInvalidReferenceExpressionNonResourceReferencedType(t *testing.T) {

	_, err := ParseAndCheckWithExtra(t, `
          struct R {}
          resource T {}

          let ref = &storage[R] as T
        `,
		storageValueDeclaration,
		nil,
		nil,
		nil,
	)

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.NonResourceReferenceError{}, errs[0])
	assert.IsType(t, &sema.TypeMismatchError{}, errs[1])
}

func TestCheckInvalidReferenceExpressionNonResourceResultType(t *testing.T) {

	_, err := ParseAndCheckWithExtra(t, `
          resource R {}
          struct T {}

          let ref = &storage[R] as T
        `,
		storageValueDeclaration,
		nil,
		nil,
		nil,
	)

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.NonResourceReferenceError{}, errs[0])
	assert.IsType(t, &sema.TypeMismatchError{}, errs[1])
}

func TestCheckInvalidReferenceExpressionNonResourceTypes(t *testing.T) {

	_, err := ParseAndCheckWithExtra(t, `
          struct R {}
          struct T {}

          let ref = &storage[R] as T
        `,
		storageValueDeclaration,
		nil,
		nil,
		nil,
	)

	errs := ExpectCheckerErrors(t, err, 3)

	assert.IsType(t, &sema.NonResourceReferenceError{}, errs[0])
	assert.IsType(t, &sema.NonResourceReferenceError{}, errs[1])
	assert.IsType(t, &sema.TypeMismatchError{}, errs[2])
}

func TestCheckInvalidReferenceExpressionTypeMismatch(t *testing.T) {

	_, err := ParseAndCheckWithExtra(t, `
          resource R {}
          resource T {}

          let ref = &storage[R] as T
        `,
		storageValueDeclaration,
		nil,
		nil,
		nil,
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckInvalidReferenceToNonIndex(t *testing.T) {

	_, err := ParseAndCheckWithExtra(t, `
          resource R {}

          let r <- create R()
          let ref = &r as R
        `,
		storageValueDeclaration,
		nil,
		nil,
		nil,
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NonStorageReferenceError{}, errs[0])
}

func TestCheckInvalidReferenceToNonStorage(t *testing.T) {

	_, err := ParseAndCheckWithExtra(t, `
          resource R {}

          let rs <- [<-create R()]
          let ref = &rs[0] as R
        `,
		storageValueDeclaration,
		nil,
		nil,
		nil,
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NonStorageReferenceError{}, errs[0])
}

func TestCheckReferenceUse(t *testing.T) {

	_, err := ParseAndCheckWithExtra(t, `
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
              var r: <-R? <- create R()
              storage[R] <-> r
              // there was no old value, but it must be discarded
              destroy r

              let ref = &storage[R] as R
              ref.x = 1
              let x1 = ref.x
              ref.setX(2)
              let x2 = ref.x
              return [x1, x2]
          }
        `,
		storageValueDeclaration,
		nil,
		nil,
	)

	assert.Nil(t, err)
}

func TestCheckReferenceUseArray(t *testing.T) {

	_, err := ParseAndCheckWithExtra(t, `
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
              var rs: <-[R]? <- [<-create R()]
              storage[[R]] <-> rs
              // there was no old value, but it must be discarded
              destroy rs

              let ref = &storage[[R]] as [R]
              ref[0].x = 1
              let x1 = ref[0].x
              ref[0].setX(2)
              let x2 = ref[0].x
              return [x1, x2]
          }
        `,
		storageValueDeclaration,
		nil,
		nil,
	)

	assert.Nil(t, err)
}

func TestCheckReferenceIndexingIfReferencedIndexable(t *testing.T) {

	_, err := ParseAndCheckWithExtra(t, `
          resource R {}

          fun test() {
              var rs: <-[R]? <- [<-create R()]
              storage[[R]] <-> rs
              // there was no old value, but it must be discarded
              destroy rs

              let ref = &storage[[R]] as [R]
              var other <- create R()
              ref[0] <-> other
              destroy other
          }
        `,
		storageValueDeclaration,
		nil,
		nil,
	)

	assert.Nil(t, err)
}

func TestCheckInvalidReferenceResourceLoss(t *testing.T) {

	_, err := ParseAndCheckWithExtra(t, `
          resource R {}

          fun test() {
              var rs: <-[R]? <- [<-create R()]
              storage[[R]] <-> rs
              // there was no old value, but it must be discarded
              destroy rs

              let ref = &storage[[R]] as [R]
              ref[0]
          }
        `,
		storageValueDeclaration,
		nil,
		nil,
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ResourceLossError{}, errs[0])
}

func TestCheckInvalidReferenceIndexingIfReferencedNotIndexable(t *testing.T) {

	_, err := ParseAndCheckWithExtra(t, `
          resource R {}

          fun test() {
              var r: <-R? <- create R()
              storage[R] <-> r
              // there was no old value, but it must be discarded
              destroy r

              let ref = &storage[R] as R
              ref[0]
          }
        `,
		storageValueDeclaration,
		nil,
		nil,
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotIndexableTypeError{}, errs[0])
}
