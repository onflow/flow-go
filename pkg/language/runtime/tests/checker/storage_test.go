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
          let a = storage[Int]
          let b = storage[Bool]
          let c = storage[[Int]]
          let d = storage[{String: Int}]
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
		checker.GlobalValues["a"].Type,
	)

	assert.Equal(t,
		&sema.OptionalType{
			Type: &sema.BoolType{},
		},
		checker.GlobalValues["b"].Type,
	)

	assert.Equal(t,
		&sema.OptionalType{
			Type: &sema.VariableSizedType{
				Type: &sema.IntType{},
			},
		},
		checker.GlobalValues["c"].Type,
	)

	assert.Equal(t,
		&sema.OptionalType{
			Type: &sema.DictionaryType{
				KeyType:   &sema.StringType{},
				ValueType: &sema.IntType{},
			},
		},
		checker.GlobalValues["d"].Type,
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
