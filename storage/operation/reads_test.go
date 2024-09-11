package operation_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
)

func TestIterateKeysInPrefixRange(t *testing.T) {
	RunWithStorages(t, func(t *testing.T, r storage.Reader, withWriterTx WithWriter) {
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

		// Keys expected to be in the prefix range
		keysInRange := keys[1:7] // these keys are between the start and end

		// Insert the keys into the storage
		withWriterTx(t, func(writer storage.Writer) error {
			for _, key := range keys {
				err := operation.Upsert(key, []byte{0x00})(writer)
				if err != nil {
					return err
				}
			}
			return nil
		})

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
}

func TestFindHighestAtOrBelow(t *testing.T) {
}
