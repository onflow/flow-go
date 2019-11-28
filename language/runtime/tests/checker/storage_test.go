package checker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/language/runtime/common"
	"github.com/dapperlabs/flow-go/language/runtime/sema"
	"github.com/dapperlabs/flow-go/language/runtime/stdlib"
	. "github.com/dapperlabs/flow-go/language/runtime/tests/utils"
)

var storageValueDeclaration = stdlib.StandardLibraryValue{
	Name:       "storage",
	Type:       &sema.StorageType{},
	Kind:       common.DeclarationKindConstant,
	IsConstant: true,
}

func ParseAndCheckStorage(t *testing.T, code string) (*sema.Checker, error) {
	return ParseAndCheckWithOptions(t,
		code,
		ParseAndCheckOptions{
			Options: []sema.Option{
				sema.WithPredeclaredValues(map[string]sema.ValueDeclaration{
					"storage": storageValueDeclaration,
				}),
				sema.WithAccessCheckMode(sema.AccessCheckModeNotSpecifiedUnrestricted),
			},
		},
	)
}

func TestCheckStorageIndexing(t *testing.T) {

	checker, err := ParseAndCheckStorage(t,
		`
          let a = storage[Int]
          let b = storage[Bool]
          let c = storage[[Int]]
          let d = storage[{String: Int}]
        `,
	)

	require.Nil(t, err)

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

	_, err := ParseAndCheckStorage(t,
		`
          fun test() {
              storage[Int] = 1
              storage[Bool] = true
          }
        `,
	)

	assert.Nil(t, err)
}

func TestCheckInvalidStorageIndexingAssignment(t *testing.T) {

	_, err := ParseAndCheckStorage(t,
		`
          fun test() {
              storage[Int] = "1"
              storage[Bool] = 1
          }
        `,
	)

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
	assert.IsType(t, &sema.TypeMismatchError{}, errs[1])
}

func TestCheckInvalidStorageIndexingAssignmentWithExpression(t *testing.T) {

	_, err := ParseAndCheckStorage(t,
		`
          fun test() {
              storage["1"] = "1"
          }
        `,
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidTypeIndexingError{}, errs[0])
}

func TestCheckInvalidStorageIndexingWithExpression(t *testing.T) {

	_, err := ParseAndCheckStorage(t,
		`
          let x = storage["1"]
        `,
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidTypeIndexingError{}, errs[0])
}

func TestCheckStorageIndexingWithResourceTypeInVariableDeclaration(t *testing.T) {

	checker, err := ParseAndCheckStorage(t,
		`
          resource R {}

          fun test() {
              let r <- storage[R] <- create R()
              destroy r
          }
        `,
	)

	require.Nil(t, err)

	assert.Len(t, checker.Elaboration.IsResourceMovingStorageIndexExpression, 1)
}

func TestCheckStorageIndexingWithResourceTypeInSwap(t *testing.T) {

	checker, err := ParseAndCheckStorage(t,
		`
          resource R {}

          fun test() {
              var r: <-R? <- create R()
              storage[R] <-> r
              destroy r
          }
        `,
	)

	require.Nil(t, err)

	assert.Len(t, checker.Elaboration.IsResourceMovingStorageIndexExpression, 1)
}

func TestCheckInvalid(t *testing.T) {

	_, err := ParseAndCheckStorage(t, `
      resource R {}

      fun test() {
          consume(<-storage[R])
      }

      fun consume(_ r: <-R?) {
          destroy r
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)
	assert.IsType(t, &sema.InvalidNestedMoveError{}, errs[0])
}
