package checker

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/common"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/stdlib"
	. "github.com/dapperlabs/flow-go/pkg/language/runtime/tests/utils"
	"github.com/stretchr/testify/assert"
	"testing"
)

var storageValueDeclaration = map[string]sema.ValueDeclaration{
	"storage": stdlib.StandardLibraryValue{
		Name:       "storage",
		Type:       &sema.StorageType{},
		Kind:       common.DeclarationKindConstant,
		IsConstant: true,
	},
}

func TestCheckStorageIndexing(t *testing.T) {

	checker, err := ParseAndCheckWithExtra(t,
		`
		  let x = storage[Int]
          let y = storage[Bool]
	    `,
		storageValueDeclaration,
		nil,
		nil,
	)

	assert.Nil(t, err)

	assert.Equal(t,
		&sema.OptionalType{
			Type: &sema.IntType{},
		},
		checker.GlobalValues["x"].Type,
	)

	assert.Equal(t,
		&sema.OptionalType{
			Type: &sema.BoolType{},
		},
		checker.GlobalValues["y"].Type,
	)
}

func TestCheckStorageIndexingAssignment(t *testing.T) {

	_, err := ParseAndCheckWithExtra(t,
		`
          fun test() {
              storage[Int] = 1
              storage[Bool] = true
          }
	    `,
		storageValueDeclaration,
		nil,
		nil,
	)

	assert.Nil(t, err)
}

func TestCheckInvalidStorageIndexingAssignment(t *testing.T) {

	_, err := ParseAndCheckWithExtra(t,
		`
          fun test() {
              storage[Int] = "1"
              storage[Bool] = 1
          }
	    `,
		storageValueDeclaration,
		nil,
		nil,
	)

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
	assert.IsType(t, &sema.TypeMismatchError{}, errs[1])
}

func TestCheckInvalidStorageIndexingAssignmentWithExpression(t *testing.T) {

	_, err := ParseAndCheckWithExtra(t,
		`
          fun test() {
              storage["1"] = "1"
          }
	    `,
		storageValueDeclaration,
		nil,
		nil,
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidStorageIndexingError{}, errs[0])
}

func TestCheckInvalidStorageIndexingWithExpression(t *testing.T) {

	_, err := ParseAndCheckWithExtra(t,
		`
          let x = storage["1"]
	    `,
		storageValueDeclaration,
		nil,
		nil,
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidStorageIndexingError{}, errs[0])
}
