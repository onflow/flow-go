package operation_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/dbtest"
)

func TestIterateKeysInPrefixRange(t *testing.T) {
	dbtest.RunWithStorages(t, func(t *testing.T, r storage.Reader, withWriter dbtest.WithWriter) {
		// Define the prefix range
		prefixStart := []byte{0x10}
		prefixEnd := []byte{0x20}

		// Create a range of keys around the prefix start/end values
		keys := [][]byte{
			// before start -> not included in range
			{0x09, 0xff},
			// within the start prefix -> included in range
			{0x10, 0x00},
			{0x10, 0xff},
			// between start and end -> included in range
			{0x15, 0x00},
			{0x1A, 0xff},
			// within the end prefix -> included in range
			{0x20, 0x00},
			{0x20, 0xff},
			// after end -> not included in range
			{0x21, 0x00},
		}

		// The first and last keys are outside the prefix range, so we omit them
		// from keysInRange, which is the set of keys we expect in the iteration
		keysInRange := keys[1 : len(keys)-1]

		// Insert the keys into the storage
		require.NoError(t, withWriter(func(writer storage.Writer) error {
			for _, key := range keys {
				value := []byte{0x00} // value are skipped, doesn't matter
				err := operation.Upsert(key, value)(writer)
				if err != nil {
					return err
				}
			}
			return nil
		}))

		// Forward iteration and check boundaries
		var found [][]byte
		require.NoError(t, operation.IterateKeysInPrefixRange(prefixStart, prefixEnd, func(key []byte) error {
			found = append(found, key)
			return nil
		})(r), "should iterate forward without error")
		require.ElementsMatch(t, keysInRange, found, "forward iteration should return the correct keys in range")
	})
}

func TestTraverse(t *testing.T) {
	dbtest.RunWithStorages(t, func(t *testing.T, r storage.Reader, withWriter dbtest.WithWriter) {
		keyVals := map[[2]byte]uint64{
			{0x41, 0xff}: 3,
			{0x42, 0x00}: 11,
			{0xff}:       13,
			{0x42, 0x56}: 17,
			{0x00}:       19,
			{0x42, 0xff}: 23,
			{0x43, 0x00}: 33,
		}
		expected := []uint64{11, 23}

		// Insert the keys and values into storage
		require.NoError(t, withWriter(func(writer storage.Writer) error {
			for key, val := range keyVals {
				err := operation.Upsert(key[:], val)(writer)
				if err != nil {
					return err
				}
			}
			return nil
		}))

		actual := make([]uint64, 0, len(keyVals))

		// Define the iteration logic
		iterationFunc := func() (operation.CheckFunc, operation.CreateFunc, operation.HandleFunc) {
			check := func(key []byte) (bool, error) {
				// Skip the key {0x42, 0x56}
				return !bytes.Equal(key, []byte{0x42, 0x56}), nil
			}
			var val uint64
			create := func() interface{} {
				return &val
			}
			handle := func() error {
				actual = append(actual, val)
				return nil
			}
			return check, create, handle
		}

		// Traverse the keys starting with prefix {0x42}
		err := operation.Traverse([]byte{0x42}, iterationFunc, storage.DefaultIteratorOptions())(r)
		require.NoError(t, err, "traverse should not return an error")

		// Assert that the actual values match the expected values
		require.Equal(t, expected, actual, "traversed values should match expected values")
	})
}

func TestFindHighestAtOrBelow(t *testing.T) {
	// Helper function to insert an entity into the storage
	insertEntity := func(writer storage.Writer, prefix []byte, height uint64, entity Entity) error {
		key := append(prefix, operation.EncodeKeyPart(height)...)
		return operation.Upsert(key, entity)(writer)
	}

	// Entities to be inserted
	entities := []struct {
		height uint64
		entity Entity
	}{
		{5, Entity{ID: 41}},
		{10, Entity{ID: 42}},
		{15, Entity{ID: 43}},
	}

	// Run test with multiple storage backends
	dbtest.RunWithStorages(t, func(t *testing.T, r storage.Reader, withWriter dbtest.WithWriter) {
		prefix := []byte("test_prefix")

		// Insert entities into the storage
		require.NoError(t, withWriter(func(writer storage.Writer) error {
			for _, e := range entities {
				if err := insertEntity(writer, prefix, e.height, e.entity); err != nil {
					return err
				}
			}
			return nil
		}))

		// Declare entity to store the results of FindHighestAtOrBelow
		var entity Entity

		// Test cases
		tests := []struct {
			name           string
			height         uint64
			expectedValue  uint64
			expectError    bool
			expectedErrMsg string
		}{
			{"target first height exists", 5, 41, false, ""},
			{"target height exists", 10, 42, false, ""},
			{"target height above", 11, 42, false, ""},
			{"target height above highest", 20, 43, false, ""},
			{"target height below lowest", 4, 0, true, storage.ErrNotFound.Error()},
			{"empty prefix", 5, 0, true, "prefix must not be empty"},
		}

		// Execute test cases
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				prefixToUse := prefix

				if tt.name == "empty prefix" {
					prefixToUse = []byte{}
				}

				err := operation.FindHighestAtOrBelow(
					prefixToUse,
					tt.height,
					&entity)(r)

				if tt.expectError {
					require.Error(t, err, fmt.Sprintf("expected error but got nil, entity: %v", entity))
					require.Contains(t, err.Error(), tt.expectedErrMsg)
				} else {
					require.NoError(t, err)
					require.Equal(t, tt.expectedValue, entity.ID)
				}
			})
		}
	})
}
