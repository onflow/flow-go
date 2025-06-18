package migration

import (
	"context"
	"sort"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/utils/unittest"
)

func TestSampleValidationKeysByPrefix(t *testing.T) {
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
		keys, err := sampleValidationKeysByPrefix(db, 2)
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

func TestCompareKeyValuePairsFromChannels(t *testing.T) {
	type testCase struct {
		name      string
		badgerKVs []KVPairs
		pebbleKVs []KVPairs
		expectErr string // substring to match in error, or empty for no error
	}

	prefix := []byte("pfx")
	key1 := []byte("key1")
	val1 := []byte("val1")
	key2 := []byte("key2")
	val2 := []byte("val2")
	val2diff := []byte("DIFF")

	tests := []testCase{
		{
			name:      "matching pairs",
			badgerKVs: []KVPairs{{Prefix: prefix, Pairs: []KVPair{{Key: key1, Value: val1}, {Key: key2, Value: val2}}}},
			pebbleKVs: []KVPairs{{Prefix: prefix, Pairs: []KVPair{{Key: key1, Value: val1}, {Key: key2, Value: val2}}}},
			expectErr: "",
		},
		{
			name:      "value mismatch",
			badgerKVs: []KVPairs{{Prefix: prefix, Pairs: []KVPair{{Key: key1, Value: val1}, {Key: key2, Value: val2}}}},
			pebbleKVs: []KVPairs{{Prefix: prefix, Pairs: []KVPair{{Key: key1, Value: val1}, {Key: key2, Value: val2diff}}}},
			expectErr: "value mismatch for key",
		},
		{
			name:      "key missing in pebble",
			badgerKVs: []KVPairs{{Prefix: prefix, Pairs: []KVPair{{Key: key1, Value: val1}, {Key: key2, Value: val2}}}},
			pebbleKVs: []KVPairs{{Prefix: prefix, Pairs: []KVPair{{Key: key1, Value: val1}}}},
			expectErr: "key 6b657932 exists in badger but not in pebble",
		},
		{
			name:      "key missing in badger",
			badgerKVs: []KVPairs{{Prefix: prefix, Pairs: []KVPair{{Key: key1, Value: val1}}}},
			pebbleKVs: []KVPairs{{Prefix: prefix, Pairs: []KVPair{{Key: key1, Value: val1}, {Key: key2, Value: val2}}}},
			expectErr: "key 6b657932 exists in pebble but not in badger",
		},
		{
			name:      "prefix mismatch",
			badgerKVs: []KVPairs{{Prefix: []byte("pfx1"), Pairs: []KVPair{{Key: key1, Value: val1}}}},
			pebbleKVs: []KVPairs{{Prefix: []byte("pfx2"), Pairs: []KVPair{{Key: key1, Value: val1}}}},
			expectErr: "prefix mismatch",
		},
		{
			name:      "context cancelled",
			badgerKVs: []KVPairs{{Prefix: prefix, Pairs: []KVPair{{Key: key1, Value: val1}}}},
			pebbleKVs: []KVPairs{{Prefix: prefix, Pairs: []KVPair{{Key: key1, Value: val1}}}},
			expectErr: "context cancelled",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			badgerCh := make(chan KVPairs, len(tc.badgerKVs))
			pebbleCh := make(chan KVPairs, len(tc.pebbleKVs))

			for _, kv := range tc.badgerKVs {
				badgerCh <- kv
			}
			close(badgerCh)
			for _, kv := range tc.pebbleKVs {
				pebbleCh <- kv
			}
			close(pebbleCh)

			if tc.name == "context cancelled" {
				// Cancel context before running
				cancel()
			}

			err := compareKeyValuePairsFromChannels(ctx, badgerCh, pebbleCh)
			if tc.expectErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectErr)
			}
		})
	}
}
