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

	t.Run("resource", func(t *testing.T) {
		checker, err := ParseAndCheckStorage(t,
			`
              resource R {}

              let r <- storage[R] <- nil
            `,
		)

		require.NoError(t, err)

		rType := checker.GlobalTypes["R"].Type

		assert.Equal(t,
			&sema.OptionalType{
				Type: rType,
			},
			checker.GlobalValues["r"].Type,
		)
	})

	t.Run("reference", func(t *testing.T) {
		checker, err := ParseAndCheckStorage(t,
			`
              resource R {}

              let r = storage[&R]
            `,
		)

		require.NoError(t, err)

		rType := checker.GlobalTypes["R"].Type

		assert.Equal(t,
			&sema.OptionalType{
				Type: &sema.ReferenceType{
					Type: rType,
				},
			},
			checker.GlobalValues["r"].Type,
		)
	})

	t.Run("resource array", func(t *testing.T) {
		checker, err := ParseAndCheckStorage(t,
			`
              resource R {}

              let r <- storage[[R]] <- nil
            `,
		)

		require.NoError(t, err)

		rType := checker.GlobalTypes["R"].Type

		assert.Equal(t,
			&sema.OptionalType{
				Type: &sema.VariableSizedType{
					Type: rType,
				},
			},
			checker.GlobalValues["r"].Type,
		)
	})

	t.Run("resource dictionary", func(t *testing.T) {
		checker, err := ParseAndCheckStorage(t,
			`
              resource R {}

              let r <- storage[{String: R}] <- nil
            `,
		)

		require.NoError(t, err)

		rType := checker.GlobalTypes["R"].Type

		assert.Equal(t,
			&sema.OptionalType{
				Type: &sema.DictionaryType{
					KeyType:   &sema.StringType{},
					ValueType: rType,
				},
			},
			checker.GlobalValues["r"].Type,
		)
	})
}

func TestCheckStorageIndexingAssignment(t *testing.T) {

	t.Run("resource", func(t *testing.T) {
		_, err := ParseAndCheckStorage(t,
			`
              resource R {}

              fun test() {
                  let oldR <- storage[R] <- create R()
                  destroy oldR
              }
            `,
		)

		require.NoError(t, err)
	})

	t.Run("reference", func(t *testing.T) {
		_, err := ParseAndCheckStorage(t,
			`
              resource R {}

              fun test() {
                  storage[&R] = &storage[R] as R
              }
            `,
		)

		require.NoError(t, err)
	})

	t.Run("resource array", func(t *testing.T) {
		_, err := ParseAndCheckStorage(t,
			`
              resource R {}

              fun test() {
                  let oldRs <- storage[[R]] <- [<-create R()]
                  destroy oldRs
              }
            `,
		)

		require.NoError(t, err)
	})

	t.Run("resource dictionary", func(t *testing.T) {
		_, err := ParseAndCheckStorage(t,
			`
              resource R {}

              fun test() {
                  let oldRs <- storage[{String: R}] <- {"r": <-create R()}
                  destroy oldRs
              }
            `,
		)

		require.NoError(t, err)
	})
}

func TestCheckInvalidStorageIndexingAssignment(t *testing.T) {

	t.Run("resource", func(t *testing.T) {

		_, err := ParseAndCheckStorage(t,
			`
              resource R {}

              fun test() {
                  storage[R] = "1"
              }
            `,
		)

		errs := ExpectCheckerErrors(t, err, 3)

		assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
		assert.IsType(t, &sema.IncorrectTransferOperationError{}, errs[1])
		assert.IsType(t, &sema.InvalidResourceAssignmentError{}, errs[2])
	})

	t.Run("reference", func(t *testing.T) {

		_, err := ParseAndCheckStorage(t,
			`
              resource R {}

              fun test() {
                  storage[&R] = true
              }
            `,
		)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
	})
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

	require.NoError(t, err)

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

	require.NoError(t, err)

	assert.Len(t, checker.Elaboration.IsResourceMovingStorageIndexExpression, 1)
}

func TestCheckInvalidResourceMoveOutOfStorage(t *testing.T) {

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
	assert.IsType(t, &sema.InvalidNestedResourceMoveError{}, errs[0])
}
