package pebble

import (
	"bytes"
	"fmt"
	"math/rand"
	"os"
	"path"
	"strconv"
	"testing"

	"github.com/cockroachdb/pebble/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/pebble/registers"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestRegisters_Initialize
func TestRegisters_Initialize(t *testing.T) {
	t.Parallel()
	p, dir := unittest.TempPebbleDBWithOpts(t, nil)
	// fail on blank database without FirstHeight and LastHeight set
	_, err := NewRegisters(p, PruningDisabled)
	require.Error(t, err)
	// verify the error type
	require.ErrorIs(t, err, storage.ErrNotBootstrapped)
	err = os.RemoveAll(dir)
	require.NoError(t, err)
}

// TestRegisters_Get tests the expected Get function behavior on a single height
func TestRegisters_Get(t *testing.T) {
	t.Parallel()
	height1 := uint64(1)
	RunWithRegistersStorageAtHeight1(t, func(r *Registers) {
		// invalid keys return correct error type
		invalidKey := flow.RegisterID{Owner: "invalid", Key: "invalid"}
		_, err := r.Get(invalidKey, height1)
		require.ErrorIs(t, err, storage.ErrNotFound)

		// insert new data
		height2 := uint64(2)
		key1 := flow.RegisterID{Owner: "owner", Key: "key1"}
		expectedValue1 := []byte("value1")
		entries := flow.RegisterEntries{
			{Key: key1, Value: expectedValue1},
		}

		err = r.Store(entries, height2)
		require.NoError(t, err)

		// happy path
		value1, err := r.Get(key1, height2)
		require.NoError(t, err)
		require.Equal(t, expectedValue1, value1)

		// out of range
		beforeFirstHeight := uint64(0)
		_, err = r.Get(key1, beforeFirstHeight)
		require.ErrorIs(t, err, storage.ErrHeightNotIndexed)
		afterLatestHeight := uint64(3)
		_, err = r.Get(key1, afterLatestHeight)
		require.ErrorIs(t, err, storage.ErrHeightNotIndexed)
	})
}

// TestRegisters_Store tests the expected store behaviour on a single height
func TestRegisters_Store(t *testing.T) {
	t.Parallel()
	RunWithRegistersStorageAtHeight1(t, func(r *Registers) {
		// insert new data
		key1 := flow.RegisterID{Owner: "owner", Key: "key1"}
		expectedValue1 := []byte("value1")
		entries := flow.RegisterEntries{
			{Key: key1, Value: expectedValue1},
		}
		height2 := uint64(2)
		err := r.Store(entries, height2)
		require.NoError(t, err)

		// idempotent at same height
		err = r.Store(entries, height2)
		require.NoError(t, err)

		// out of range
		height4 := uint64(4)
		err = r.Store(entries, height4)
		require.Error(t, err)

		height1 := uint64(1)
		err = r.Store(entries, height1)
		require.Error(t, err)

	})
}

// TestRegisters_Heights tests the expected store behaviour on a single height
func TestRegisters_Heights(t *testing.T) {
	t.Parallel()
	RunWithRegistersStorageAtHeight1(t, func(r *Registers) {
		// first and latest heights are the same
		firstHeight := r.FirstHeight()
		latestHeight := r.LatestHeight()
		require.Equal(t, firstHeight, latestHeight)
		// insert new data
		key1 := flow.RegisterID{Owner: "owner", Key: "key1"}
		expectedValue1 := []byte("value1")
		entries := flow.RegisterEntries{
			{Key: key1, Value: expectedValue1},
		}
		height2 := uint64(2)
		err := r.Store(entries, height2)
		require.NoError(t, err)

		firstHeight2 := r.FirstHeight()
		latestHeight2 := r.LatestHeight()

		// new latest height
		require.Equal(t, latestHeight2, height2)

		// same first height
		require.Equal(t, firstHeight, firstHeight2)
	})
}

// TestRegisters_Store_RoundTrip tests the round trip of a payload storage.
func TestRegisters_Store_RoundTrip(t *testing.T) {
	t.Parallel()
	minHeight := uint64(2)
	RunWithRegistersStorageAtInitialHeights(t, minHeight, minHeight, func(r *Registers) {
		key1 := flow.RegisterID{Owner: "owner", Key: "key1"}
		expectedValue1 := []byte("value1")
		entries := flow.RegisterEntries{
			{Key: key1, Value: expectedValue1},
		}
		testHeight := minHeight + 1
		// happy path
		err := r.Store(entries, testHeight)
		require.NoError(t, err)

		// lookup with exact height returns the correct value
		value1, err := r.Get(key1, testHeight)
		require.NoError(t, err)
		require.Equal(t, expectedValue1, value1)

		value11, err := r.Get(key1, testHeight)
		require.NoError(t, err)
		require.Equal(t, expectedValue1, value11)
	})
}

// TestRegisters_Store_Versioning tests the scan functionality for the most recent value
func TestRegisters_Store_Versioning(t *testing.T) {
	t.Parallel()
	RunWithRegistersStorageAtHeight1(t, func(r *Registers) {
		// Save key11 is a prefix of the key1, and we save it first.
		// It should be invisible for our prefix scan.
		key11 := flow.RegisterID{Owner: "owner", Key: "key11"}
		expectedValue11 := []byte("value11")

		key1 := flow.RegisterID{Owner: "owner", Key: "key1"}
		expectedValue1 := []byte("value1")
		entries1 := flow.RegisterEntries{
			{Key: key1, Value: expectedValue1},
			{Key: key11, Value: expectedValue11},
		}

		height2 := uint64(2)

		// check increment in height after Store()
		err := r.Store(entries1, height2)
		require.NoError(t, err)

		// Add new version of key1.
		height3 := uint64(3)
		expectedValue1ge3 := []byte("value1ge3")
		entries3 := flow.RegisterEntries{
			{Key: key1, Value: expectedValue1ge3},
		}

		// check increment in height after Store()
		err = r.Store(entries3, height3)
		require.NoError(t, err)
		updatedHeight := r.LatestHeight()
		require.Equal(t, updatedHeight, height3)

		// test old version at previous height
		value1, err := r.Get(key1, height2)
		require.NoError(t, err)
		require.Equal(t, expectedValue1, value1)

		// test new version at new height
		value1, err = r.Get(key1, height3)
		require.NoError(t, err)
		require.Equal(t, expectedValue1ge3, value1)

		// test unchanged key at incremented height
		value11, err := r.Get(key11, height3)
		require.NoError(t, err)
		require.Equal(t, expectedValue11, value11)

		// make sure the key is unavailable at height 1
		_, err = r.Get(key1, uint64(1))
		require.ErrorIs(t, err, storage.ErrNotFound)
	})
}

// TestRegisters_GetAndStoreEmptyOwner tests behavior of storing and retrieving registers with
// an empty owner value, which is used for global state variables.
func TestRegisters_GetAndStoreEmptyOwner(t *testing.T) {
	t.Parallel()
	height := uint64(2)
	emptyOwnerKey := flow.RegisterID{Owner: "", Key: "uuid"}
	zeroOwnerKey := flow.RegisterID{Owner: flow.EmptyAddress.Hex(), Key: "uuid"}
	expectedValue := []byte("first value")
	otherValue := []byte("other value")

	t.Run("empty owner", func(t *testing.T) {
		RunWithRegistersStorageAtInitialHeights(t, 1, 1, func(r *Registers) {
			// First, only set the empty Owner key, and make sure the empty value is available,
			// and the zero value returns an errors
			entries := flow.RegisterEntries{
				{Key: emptyOwnerKey, Value: expectedValue},
			}

			err := r.Store(entries, height)
			require.NoError(t, err)

			actual, err := r.Get(emptyOwnerKey, height)
			assert.NoError(t, err)
			assert.Equal(t, expectedValue, actual)

			actual, err = r.Get(zeroOwnerKey, height)
			assert.Error(t, err)
			assert.Nil(t, actual)

			// Next, add the zero value, and make sure it is returned
			entries = flow.RegisterEntries{
				{Key: zeroOwnerKey, Value: otherValue},
			}

			err = r.Store(entries, height+1)
			require.NoError(t, err)

			actual, err = r.Get(zeroOwnerKey, height+1)
			assert.NoError(t, err)
			assert.Equal(t, otherValue, actual)
		})
	})

	t.Run("zero owner", func(t *testing.T) {
		RunWithRegistersStorageAtInitialHeights(t, 1, 1, func(r *Registers) {
			// First, only set the zero Owner key, and make sure the zero value is available,
			// and the empty value returns an errors
			entries := flow.RegisterEntries{
				{Key: zeroOwnerKey, Value: expectedValue},
			}

			err := r.Store(entries, height)
			require.NoError(t, err)

			actual, err := r.Get(zeroOwnerKey, height)
			assert.NoError(t, err)
			assert.Equal(t, expectedValue, actual)

			actual, err = r.Get(emptyOwnerKey, height)
			assert.Error(t, err)
			assert.Nil(t, actual)

			// Next, add the empty value, and make sure it is returned
			entries = flow.RegisterEntries{
				{Key: emptyOwnerKey, Value: otherValue},
			}

			err = r.Store(entries, height+1)
			require.NoError(t, err)

			actual, err = r.Get(emptyOwnerKey, height+1)
			assert.NoError(t, err)
			assert.Equal(t, otherValue, actual)
		})
	})
}

// Benchmark_PayloadStorage benchmarks the SetBatch method.
func Benchmark_PayloadStorage(b *testing.B) {
	cache := pebble.NewCache(32 << 20)
	defer cache.Unref()
	opts := DefaultPebbleOptions(unittest.Logger(), cache, registers.NewMVCCComparer())

	dbpath := path.Join(b.TempDir(), "benchmark1.db")
	db, err := pebble.Open(dbpath, opts)
	require.NoError(b, err)
	s, err := NewRegisters(db, PruningDisabled)
	require.NoError(b, err)
	require.NotNil(b, s)

	owner := unittest.RandomAddressFixture()
	batchSizeKey := flow.NewRegisterID(owner, "size")
	const maxBatchSize = 1024
	var totalBatchSize int

	keyForBatchSize := func(i int) flow.RegisterID {
		return flow.NewRegisterID(owner, strconv.Itoa(i))
	}
	valueForHeightAndKey := func(i, j int) []byte {
		return []byte(fmt.Sprintf("%d-%d", i, j))
	}
	b.ResetTimer()

	// Write a random number of entries in each batch.
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		batchSize := rand.Intn(maxBatchSize) + 1
		totalBatchSize += batchSize
		entries := make(flow.RegisterEntries, 1, batchSize)
		entries[0] = flow.RegisterEntry{
			Key:   batchSizeKey,
			Value: []byte(fmt.Sprintf("%d", batchSize)),
		}
		for j := 1; j < batchSize; j++ {
			entries = append(entries, flow.RegisterEntry{
				Key:   keyForBatchSize(j),
				Value: valueForHeightAndKey(i, j),
			})
		}
		b.StartTimer()

		err = s.Store(entries, uint64(i))
		require.NoError(b, err)
	}

	b.StopTimer()

	// verify written batches
	for i := 0; i < b.N; i++ {
		// get number of batches written for height
		batchSizeBytes, err := s.Get(batchSizeKey, uint64(i))
		require.NoError(b, err)
		batchSize, err := strconv.Atoi(string(batchSizeBytes))
		require.NoError(b, err)

		// verify that all entries can be read with correct values
		for j := 1; j < batchSize; j++ {
			value, err := s.Get(keyForBatchSize(j), uint64(i))
			require.NoError(b, err)
			require.Equal(b, valueForHeightAndKey(i, j), value)
		}

		// verify that the rest of the batches either do not exist or have a previous height
		for j := batchSize; j < maxBatchSize+1; j++ {
			value, err := s.Get(keyForBatchSize(j), uint64(i))
			require.NoError(b, err)

			if len(value) > 0 {
				ij := bytes.Split(value, []byte("-"))

				// verify that we've got a value for a previous height
				height, err := strconv.Atoi(string(ij[0]))
				require.NoError(b, err)
				require.Lessf(b, height, i, "height: %d, j: %d", height, j)

				// verify that we've got a value corresponding to the index
				index, err := strconv.Atoi(string(ij[1]))
				require.NoError(b, err)
				require.Equal(b, index, j)
			}
		}
	}
}

// TestRegisters_ByKeyPrefix_ExactKey tests ByKeyPrefix with an exact key (no prefix
// matches), verifying it yields one entry per owner and uses nextOwnerStart after a match.
func TestRegisters_ByKeyPrefix_ExactKey(t *testing.T) {
	t.Parallel()

	addrA := unittest.RandomAddressFixture()
	addrB := unittest.RandomAddressFixture()
	addrC := unittest.RandomAddressFixture() // no contracts, only other registers
	addrD := unittest.RandomAddressFixture() // contract_names updated across two heights

	regA := flow.ContractNamesRegisterID(addrA)
	regB := flow.ContractNamesRegisterID(addrB)
	regD := flow.ContractNamesRegisterID(addrD)

	valA := []byte("namesA")
	valB := []byte("namesB")
	valDOld := []byte("namesDold")
	valDNew := []byte("namesDnew")

	RunWithRegistersStorageAtInitialHeights(t, 1, 1, func(r *Registers) {
		// height 2: A gets contract_names
		require.NoError(t, r.Store(flow.RegisterEntries{{Key: regA, Value: valA}}, 2))
		// height 3: D gets its first contract_names
		require.NoError(t, r.Store(flow.RegisterEntries{{Key: regD, Value: valDOld}}, 3))
		// height 4: B gets contract_names; C gets a non-contract register
		require.NoError(t, r.Store(flow.RegisterEntries{
			{Key: regB, Value: valB},
			{Key: flow.RegisterID{Owner: string(addrC.Bytes()), Key: flow.AccountStatusKey}, Value: []byte("status")},
		}, 4))
		// height 5: D updates its contract_names
		require.NoError(t, r.Store(flow.RegisterEntries{{Key: regD, Value: valDNew}}, 5))

		t.Run("yields all accounts with contracts at or before query height", func(t *testing.T) {
			results := collectRegistersByKey(t, r.ByKeyPrefix(flow.ContractNamesKey, 5, nil))
			assert.Len(t, results, 3)
			assert.Equal(t, flow.RegisterValue(valA), results[regA])
			assert.Equal(t, flow.RegisterValue(valB), results[regB])
			assert.Equal(t, flow.RegisterValue(valDNew), results[regD])
		})

		t.Run("yields older version when newest is above target height", func(t *testing.T) {
			results := collectRegistersByKey(t, r.ByKeyPrefix(flow.ContractNamesKey, 4, nil))
			assert.Len(t, results, 3)
			assert.Equal(t, flow.RegisterValue(valDOld), results[regD])
		})

		t.Run("excludes accounts whose contracts were deployed after target height", func(t *testing.T) {
			results := collectRegistersByKey(t, r.ByKeyPrefix(flow.ContractNamesKey, 2, nil))
			assert.Len(t, results, 1)
			assert.Equal(t, flow.RegisterValue(valA), results[regA])
		})

		t.Run("empty result when no contracts exist at or before target height", func(t *testing.T) {
			results := collectRegistersByKey(t, r.ByKeyPrefix(flow.ContractNamesKey, 1, nil))
			assert.Empty(t, results)
		})

		t.Run("account with only non-contract registers is not yielded", func(t *testing.T) {
			results := collectRegistersByKey(t, r.ByKeyPrefix(flow.ContractNamesKey, 5, nil))
			_, hasC := results[flow.ContractNamesRegisterID(addrC)]
			assert.False(t, hasC)
		})

		t.Run("early termination stops iteration", func(t *testing.T) {
			count := 0
			for _, err := range r.ByKeyPrefix(flow.ContractNamesKey, 5, nil) {
				require.NoError(t, err)
				count++
				break
			}
			assert.Equal(t, 1, count)
		})
	})
}

// TestRegisters_ByKeyPrefix tests ByKeyPrefix, which scans pebble for all registers
// whose key starts with a given prefix using a single iterator.
func TestRegisters_ByKeyPrefix(t *testing.T) {
	t.Parallel()

	addrA := unittest.RandomAddressFixture()
	addrB := unittest.RandomAddressFixture()
	addrC := unittest.RandomAddressFixture() // no code registers

	// addrA: two contracts, one updated across heights
	regAFoo := flow.RegisterID{Owner: string(addrA.Bytes()), Key: "code.Foo"}
	regABar := flow.RegisterID{Owner: string(addrA.Bytes()), Key: "code.Bar"}
	// addrB: one contract
	regBBaz := flow.RegisterID{Owner: string(addrB.Bytes()), Key: "code.Baz"}
	// addrC: only a non-code register
	regCStatus := flow.RegisterID{Owner: string(addrC.Bytes()), Key: flow.AccountStatusKey}

	valAFoo1 := []byte("foo-code-v1")
	valAFoo2 := []byte("foo-code-v2")
	valABar := []byte("bar-code")
	valBBaz := []byte("baz-code")

	RunWithRegistersStorageAtInitialHeights(t, 1, 1, func(r *Registers) {
		// height 2: addrA deploys Foo (v1); addrC gets a non-code register
		require.NoError(t, r.Store(flow.RegisterEntries{
			{Key: regAFoo, Value: valAFoo1},
			{Key: regCStatus, Value: []byte("status")},
		}, 2))
		// height 3: addrA deploys Bar; addrB deploys Baz
		require.NoError(t, r.Store(flow.RegisterEntries{
			{Key: regABar, Value: valABar},
			{Key: regBBaz, Value: valBBaz},
		}, 3))
		// height 4: addrA updates Foo to v2
		require.NoError(t, r.Store(flow.RegisterEntries{
			{Key: regAFoo, Value: valAFoo2},
		}, 4))

		t.Run("yields one entry per matching register across all owners", func(t *testing.T) {
			results := collectRegistersByKey(t, r.ByKeyPrefix(flow.CodeKeyPrefix, 4, nil))
			assert.Len(t, results, 3) // Foo, Bar, Baz
			assert.Equal(t, flow.RegisterValue(valAFoo2), results[regAFoo])
			assert.Equal(t, flow.RegisterValue(valABar), results[regABar])
			assert.Equal(t, flow.RegisterValue(valBBaz), results[regBBaz])
		})

		t.Run("yields older version when newest is above target height", func(t *testing.T) {
			results := collectRegistersByKey(t, r.ByKeyPrefix(flow.CodeKeyPrefix, 3, nil))
			assert.Equal(t, flow.RegisterValue(valAFoo1), results[regAFoo])
		})

		t.Run("excludes registers deployed after target height", func(t *testing.T) {
			results := collectRegistersByKey(t, r.ByKeyPrefix(flow.CodeKeyPrefix, 2, nil))
			assert.Len(t, results, 1)
			assert.Equal(t, flow.RegisterValue(valAFoo1), results[regAFoo])
		})

		t.Run("empty result when no matching registers exist at or before target height", func(t *testing.T) {
			results := collectRegistersByKey(t, r.ByKeyPrefix(flow.CodeKeyPrefix, 1, nil))
			assert.Empty(t, results)
		})

		t.Run("non-matching registers are not yielded", func(t *testing.T) {
			results := collectRegistersByKey(t, r.ByKeyPrefix(flow.CodeKeyPrefix, 4, nil))
			_, hasStatus := results[regCStatus]
			assert.False(t, hasStatus)
		})

		t.Run("early termination stops iteration", func(t *testing.T) {
			count := 0
			for _, err := range r.ByKeyPrefix(flow.CodeKeyPrefix, 4, nil) {
				require.NoError(t, err)
				count++
				break
			}
			assert.Equal(t, 1, count)
		})
	})
}

// TestRegisters_ByKeyPrefix_GlobalRegisters verifies that ByKeyPrefix does not terminate
// early when global registers (empty owner) are present in pebble between account registers
// whose first address bytes straddle the '/' byte (0x2F). Before the fix, nextOwnerStart("")
// returned [codeRegister+1] which equals the iterator's upper bound, causing the scan to
// stop before processing any account whose first address byte is > 0x2F.
func TestRegisters_ByKeyPrefix_GlobalRegisters(t *testing.T) {
	t.Parallel()

	// addrLow: first address byte 0x01 < '/' (0x2F) — pebble key sorts before global registers
	addrLow := flow.BytesToAddress([]byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01})
	// addrHigh: first address byte 0x30 > '/' (0x2F) — pebble key sorts after global registers
	addrHigh := flow.BytesToAddress([]byte{0x30, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01})

	regLow := flow.RegisterID{Owner: string(addrLow.Bytes()), Key: "code.Low"}
	regHigh := flow.RegisterID{Owner: string(addrHigh.Bytes()), Key: "code.High"}
	// global register (empty owner): pebble key [codeRegister, '/', ...] sorts between addrLow and addrHigh
	regGlobal := flow.RegisterID{Owner: "", Key: flow.AddressStateKey}

	RunWithRegistersStorageAtInitialHeights(t, 1, 1, func(r *Registers) {
		require.NoError(t, r.Store(flow.RegisterEntries{
			{Key: regLow, Value: []byte("low-code")},
			{Key: regHigh, Value: []byte("high-code")},
			{Key: regGlobal, Value: []byte("global-state")},
		}, 2))

		results := collectRegistersByKey(t, r.ByKeyPrefix(flow.CodeKeyPrefix, 2, nil))
		assert.Len(t, results, 2)
		assert.Equal(t, flow.RegisterValue([]byte("low-code")), results[regLow])
		assert.Equal(t, flow.RegisterValue([]byte("high-code")), results[regHigh])
	})
}

// collectRegistersByKey drains a ByKey iterator into a map keyed by register ID.
func collectRegistersByKey(t *testing.T, iter storage.IndexIterator[flow.RegisterValue, flow.RegisterID]) map[flow.RegisterID]flow.RegisterValue {
	t.Helper()
	results := make(map[flow.RegisterID]flow.RegisterValue)
	for entry, err := range iter {
		require.NoError(t, err)
		val, err := entry.Value()
		require.NoError(t, err)
		results[entry.Cursor()] = val
	}
	return results
}

func RunWithRegistersStorageAtHeight1(tb testing.TB, f func(r *Registers)) {
	defaultHeight := uint64(1)
	RunWithRegistersStorageAtInitialHeights(tb, defaultHeight, defaultHeight, f)
}
