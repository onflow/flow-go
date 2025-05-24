package migration

import (
	"sort"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/require"
)

func TestCollectValidationKeysByPrefix(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		// Insert test keys
		testKeys := []string{
			"\x01\x02keyA", // group: 0x01, 0x02
			"\x01\x02keyB",
			"\x01\x03keyC", // group: 0x01, 0x03
			"\x02\x00keyD", // group: 0x02, 0x00
			"\x02\x00keyE", // group: 0x02, 0x00
			"\x02\x00keyF", // group: 0x02, 0x00
			"\xff\xfflast",
		}
		require.NoError(t, db.Update(func(txn *badger.Txn) error {
			for _, k := range testKeys {
				err := txn.Set([]byte(k), []byte("val_"+k))
				require.NoError(t, err)
			}
			return nil
		}))

		// Run key collection
		keys, err := collectValidationKeysByPrefix(db, 2)
		require.NoError(t, err)

		// Convert to string for easier comparison
		var keyStrs []string
		for _, k := range keys {
			keyStrs = append(keyStrs, string(k))
		}
		sort.Strings(keyStrs)

		// Expected keys are min and max for each 2-byte prefix group
		expected := []string{
			"\x01\x02keyA", "\x01\x02keyB", // same group have both min and max
			"\x01\x03keyC", // only one key in this group
			"\x02\x00keyD", // min key of 0x02,0x00
			"\x02\x00keyF", // max key of 0x02,0x00
			"\xff\xfflast", // last key in this prefix
		}
		sort.Strings(expected)
		require.ElementsMatch(t, expected, keyStrs)
	})
}
