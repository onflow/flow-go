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
