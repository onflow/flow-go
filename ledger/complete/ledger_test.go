package complete_test

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/common/proof"
	"github.com/onflow/flow-go/ledger/common/testutils"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/ledger/complete/wal/fixtures"
	"github.com/onflow/flow-go/ledger/partial/ptrie"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestNewLedger(t *testing.T) {
	metricsCollector := &metrics.NoopCollector{}
	wal := &fixtures.NoopWAL{}
	_, err := complete.NewLedger(wal, 100, metricsCollector, zerolog.Logger{}, complete.DefaultPathFinderVersion)
	assert.NoError(t, err)

}

func TestLedger_Update(t *testing.T) {
	t.Run("empty update", func(t *testing.T) {

		wal := &fixtures.NoopWAL{}

		l, err := complete.NewLedger(wal, 100, &metrics.NoopCollector{}, zerolog.Logger{}, complete.DefaultPathFinderVersion)
		require.NoError(t, err)

		compactor := fixtures.NewNoopCompactor(l)
		<-compactor.Ready()
		defer func() {
			<-l.Done()
			<-compactor.Done()
		}()

		// create empty update
		currentState := l.InitialState()
		up, err := ledger.NewEmptyUpdate(currentState)
		require.NoError(t, err)

		newState, _, err := l.Set(up)
		require.NoError(t, err)

		// state shouldn't change
		assert.Equal(t, currentState, newState)
	})

	t.Run("non-empty update and query", func(t *testing.T) {

		// UpdateFixture
		wal := &fixtures.NoopWAL{}
		led, err := complete.NewLedger(wal, 100, &metrics.NoopCollector{}, zerolog.Logger{}, complete.DefaultPathFinderVersion)
		require.NoError(t, err)

		compactor := fixtures.NewNoopCompactor(led)
		<-compactor.Ready()
		defer func() {
			<-led.Done()
			<-compactor.Done()
		}()

		curSC := led.InitialState()

		u := testutils.UpdateFixture()
		u.SetState(curSC)

		newSc, _, err := led.Set(u)
		require.NoError(t, err)
		assert.NotEqual(t, curSC, newSc)

		q, err := ledger.NewQuery(newSc, u.Keys())
		require.NoError(t, err)

		retValues, err := led.Get(q)
		require.NoError(t, err)

		for i, v := range u.Values() {
			assert.Equal(t, v, retValues[i])
		}
	})
}

func TestLedger_Get(t *testing.T) {
	t.Run("empty query", func(t *testing.T) {

		wal := &fixtures.NoopWAL{}

		led, err := complete.NewLedger(wal, 100, &metrics.NoopCollector{}, zerolog.Logger{}, complete.DefaultPathFinderVersion)
		require.NoError(t, err)

		compactor := fixtures.NewNoopCompactor(led)
		<-compactor.Ready()
		defer func() {
			<-led.Done()
			<-compactor.Done()
		}()

		curSC := led.InitialState()
		q, err := ledger.NewEmptyQuery(curSC)
		require.NoError(t, err)

		retValues, err := led.Get(q)
		require.NoError(t, err)
		assert.Equal(t, len(retValues), 0)
	})

	t.Run("empty keys", func(t *testing.T) {

		wal := &fixtures.NoopWAL{}

		led, err := complete.NewLedger(wal, 100, &metrics.NoopCollector{}, zerolog.Logger{}, complete.DefaultPathFinderVersion)
		require.NoError(t, err)

		compactor := fixtures.NewNoopCompactor(led)
		<-compactor.Ready()
		defer func() {
			<-led.Done()
			<-compactor.Done()
		}()

		curS := led.InitialState()

		q := testutils.QueryFixture()
		q.SetState(curS)

		retValues, err := led.Get(q)
		require.NoError(t, err)

		assert.Equal(t, 2, len(retValues))
		assert.Equal(t, 0, len(retValues[0]))
		assert.Equal(t, 0, len(retValues[1]))

	})
}

// TestLedger_GetSingleValue tests reading value from a single path.
func TestLedger_GetSingleValue(t *testing.T) {

	wal := &fixtures.NoopWAL{}
	led, err := complete.NewLedger(
		wal,
		100,
		&metrics.NoopCollector{},
		zerolog.Logger{},
		complete.DefaultPathFinderVersion,
	)
	require.NoError(t, err)

	compactor := fixtures.NewNoopCompactor(led)
	<-compactor.Ready()
	defer func() {
		<-led.Done()
		<-compactor.Done()
	}()

	state := led.InitialState()

	t.Run("non-existent key", func(t *testing.T) {

		keys := testutils.RandomUniqueKeys(10, 2, 1, 10)

		for _, k := range keys {
			qs, err := ledger.NewQuerySingleValue(state, k)
			require.NoError(t, err)

			retValue, err := led.GetSingleValue(qs)
			require.NoError(t, err)
			assert.Equal(t, 0, len(retValue))
		}
	})

	t.Run("existent key", func(t *testing.T) {

		u := testutils.UpdateFixture()
		u.SetState(state)

		newState, _, err := led.Set(u)
		require.NoError(t, err)
		assert.NotEqual(t, state, newState)

		for i, k := range u.Keys() {
			q, err := ledger.NewQuerySingleValue(newState, k)
			require.NoError(t, err)

			retValue, err := led.GetSingleValue(q)
			require.NoError(t, err)
			assert.Equal(t, u.Values()[i], retValue)
		}
	})

	t.Run("mix of existent and non-existent keys", func(t *testing.T) {

		u := testutils.UpdateFixture()
		u.SetState(state)

		newState, _, err := led.Set(u)
		require.NoError(t, err)
		assert.NotEqual(t, state, newState)

		// Save expected values for existent keys
		expectedValues := make(map[string]ledger.Value)
		for i, key := range u.Keys() {
			encKey := ledger.EncodeKey(&key)
			expectedValues[string(encKey)] = u.Values()[i]
		}

		// Create a randomly ordered mix of existent and non-existent keys
		var queryKeys []ledger.Key
		queryKeys = append(queryKeys, u.Keys()...)
		queryKeys = append(queryKeys, testutils.RandomUniqueKeys(10, 2, 1, 10)...)

		rand.Shuffle(len(queryKeys), func(i, j int) {
			queryKeys[i], queryKeys[j] = queryKeys[j], queryKeys[i]
		})

		for _, k := range queryKeys {
			qs, err := ledger.NewQuerySingleValue(newState, k)
			require.NoError(t, err)

			retValue, err := led.GetSingleValue(qs)
			require.NoError(t, err)

			encKey := ledger.EncodeKey(&k)
			if value, ok := expectedValues[string(encKey)]; ok {
				require.Equal(t, value, retValue)
			} else {
				require.Equal(t, 0, len(retValue))
			}
		}
	})
}

func TestLedgerValueSizes(t *testing.T) {
	t.Run("empty query", func(t *testing.T) {

		wal := &fixtures.NoopWAL{}
		led, err := complete.NewLedger(
			wal,
			100,
			&metrics.NoopCollector{},
			zerolog.Logger{},
			complete.DefaultPathFinderVersion,
		)
		require.NoError(t, err)

		compactor := fixtures.NewNoopCompactor(led)
		<-compactor.Ready()
		defer func() {
			<-led.Done()
			<-compactor.Done()
		}()

		curState := led.InitialState()
		q, err := ledger.NewEmptyQuery(curState)
		require.NoError(t, err)

		retSizes, err := led.ValueSizes(q)
		require.NoError(t, err)
		require.Equal(t, 0, len(retSizes))
	})

	t.Run("non-existent keys", func(t *testing.T) {

		wal := &fixtures.NoopWAL{}
		led, err := complete.NewLedger(
			wal,
			100,
			&metrics.NoopCollector{},
			zerolog.Logger{},
			complete.DefaultPathFinderVersion,
		)
		require.NoError(t, err)

		compactor := fixtures.NewNoopCompactor(led)
		<-compactor.Ready()
		defer func() {
			<-led.Done()
			<-compactor.Done()
		}()

		curState := led.InitialState()
		q := testutils.QueryFixture()
		q.SetState(curState)

		retSizes, err := led.ValueSizes(q)
		require.NoError(t, err)
		require.Equal(t, len(q.Keys()), len(retSizes))
		for _, size := range retSizes {
			assert.Equal(t, 0, size)
		}
	})

	t.Run("existent keys", func(t *testing.T) {

		wal := &fixtures.NoopWAL{}
		led, err := complete.NewLedger(
			wal,
			100,
			&metrics.NoopCollector{},
			zerolog.Logger{},
			complete.DefaultPathFinderVersion,
		)
		require.NoError(t, err)

		compactor := fixtures.NewNoopCompactor(led)
		<-compactor.Ready()
		defer func() {
			<-led.Done()
			<-compactor.Done()
		}()

		curState := led.InitialState()
		u := testutils.UpdateFixture()
		u.SetState(curState)

		newState, _, err := led.Set(u)
		require.NoError(t, err)
		assert.NotEqual(t, curState, newState)

		q, err := ledger.NewQuery(newState, u.Keys())
		require.NoError(t, err)

		retSizes, err := led.ValueSizes(q)
		require.NoError(t, err)
		require.Equal(t, len(q.Keys()), len(retSizes))
		for i, size := range retSizes {
			assert.Equal(t, u.Values()[i].Size(), size)
		}
	})

	t.Run("mix of existent and non-existent keys", func(t *testing.T) {

		wal := &fixtures.NoopWAL{}
		led, err := complete.NewLedger(
			wal,
			100,
			&metrics.NoopCollector{},
			zerolog.Logger{},
			complete.DefaultPathFinderVersion,
		)
		require.NoError(t, err)

		compactor := fixtures.NewNoopCompactor(led)
		<-compactor.Ready()
		defer func() {
			<-led.Done()
			<-compactor.Done()
		}()

		curState := led.InitialState()
		u := testutils.UpdateFixture()
		u.SetState(curState)

		newState, _, err := led.Set(u)
		require.NoError(t, err)
		assert.NotEqual(t, curState, newState)

		// Save expected value sizes for existent keys
		expectedValueSizes := make(map[string]int)
		for i, key := range u.Keys() {
			encKey := ledger.EncodeKey(&key)
			expectedValueSizes[string(encKey)] = len(u.Values()[i])
		}

		// Create a randomly ordered mix of existent and non-existent keys
		var queryKeys []ledger.Key
		queryKeys = append(queryKeys, u.Keys()...)
		queryKeys = append(queryKeys, testutils.RandomUniqueKeys(10, 2, 1, 10)...)

		rand.Shuffle(len(queryKeys), func(i, j int) {
			queryKeys[i], queryKeys[j] = queryKeys[j], queryKeys[i]
		})

		q, err := ledger.NewQuery(newState, queryKeys)
		require.NoError(t, err)

		retSizes, err := led.ValueSizes(q)
		require.NoError(t, err)
		require.Equal(t, len(q.Keys()), len(retSizes))
		for i, key := range q.Keys() {
			encKey := ledger.EncodeKey(&key)
			assert.Equal(t, expectedValueSizes[string(encKey)], retSizes[i])
		}
	})
}

func TestLedger_Proof(t *testing.T) {
	t.Run("empty query", func(t *testing.T) {
		wal := &fixtures.NoopWAL{}

		led, err := complete.NewLedger(wal, 100, &metrics.NoopCollector{}, zerolog.Logger{}, complete.DefaultPathFinderVersion)
		require.NoError(t, err)

		compactor := fixtures.NewNoopCompactor(led)
		<-compactor.Ready()
		defer func() {
			<-led.Done()
			<-compactor.Done()
		}()

		curSC := led.InitialState()
		q, err := ledger.NewEmptyQuery(curSC)
		require.NoError(t, err)

		retProof, err := led.Prove(q)
		require.NoError(t, err)

		proof, err := ledger.DecodeTrieBatchProof(retProof)
		require.NoError(t, err)
		assert.Equal(t, 0, len(proof.Proofs))
	})

	t.Run("non-existing keys", func(t *testing.T) {

		wal := &fixtures.NoopWAL{}

		led, err := complete.NewLedger(wal, 100, &metrics.NoopCollector{}, zerolog.Logger{}, complete.DefaultPathFinderVersion)
		require.NoError(t, err)

		compactor := fixtures.NewNoopCompactor(led)
		<-compactor.Ready()
		defer func() {
			<-led.Done()
			<-compactor.Done()
		}()

		curS := led.InitialState()
		q := testutils.QueryFixture()
		q.SetState(curS)
		require.NoError(t, err)

		retProof, err := led.Prove(q)
		require.NoError(t, err)

		trieProof, err := ledger.DecodeTrieBatchProof(retProof)
		require.NoError(t, err)
		assert.Equal(t, 2, len(trieProof.Proofs))
		assert.True(t, proof.VerifyTrieBatchProof(trieProof, curS))

	})

	t.Run("existing keys", func(t *testing.T) {

		wal := &fixtures.NoopWAL{}
		led, err := complete.NewLedger(wal, 100, &metrics.NoopCollector{}, zerolog.Logger{}, complete.DefaultPathFinderVersion)
		require.NoError(t, err)

		compactor := fixtures.NewNoopCompactor(led)
		<-compactor.Ready()
		defer func() {
			<-led.Done()
			<-compactor.Done()
		}()

		curS := led.InitialState()

		u := testutils.UpdateFixture()
		u.SetState(curS)

		newSc, _, err := led.Set(u)
		require.NoError(t, err)
		assert.NotEqual(t, curS, newSc)

		q, err := ledger.NewQuery(newSc, u.Keys())
		require.NoError(t, err)

		retProof, err := led.Prove(q)
		require.NoError(t, err)

		trieProof, err := ledger.DecodeTrieBatchProof(retProof)
		require.NoError(t, err)
		assert.Equal(t, 2, len(trieProof.Proofs))
		assert.True(t, proof.VerifyTrieBatchProof(trieProof, newSc))
	})
}

func Test_WAL(t *testing.T) {
	const (
		numInsPerStep      = 2
		keyNumberOfParts   = 10
		keyPartMinByteSize = 1
		keyPartMaxByteSize = 100
		valueMaxByteSize   = 2 << 16 //16kB
		size               = 10
		checkpointDistance = math.MaxInt // A large number to prevent checkpoint creation.
		checkpointsToKeep  = 1
	)

	metricsCollector := &metrics.NoopCollector{}
	logger := zerolog.Logger{}

	unittest.RunWithTempDir(t, func(dir string) {

		diskWal, err := wal.NewDiskWAL(zerolog.Nop(), nil, metricsCollector, dir, size, pathfinder.PathByteSize, wal.SegmentSize)
		require.NoError(t, err)

		// cache size intentionally is set to size to test deletion
		led, err := complete.NewLedger(diskWal, size, metricsCollector, logger, complete.DefaultPathFinderVersion)
		require.NoError(t, err)

		compactor, err := complete.NewCompactor(led, diskWal, zerolog.Nop(), size, checkpointDistance, checkpointsToKeep, atomic.NewBool(false))
		require.NoError(t, err)

		<-compactor.Ready()

		var state = led.InitialState()

		//saved data after updates
		savedData := make(map[string]map[string]ledger.Value)

		for i := 0; i < size; i++ {

			keys := testutils.RandomUniqueKeys(numInsPerStep, keyNumberOfParts, keyPartMinByteSize, keyPartMaxByteSize)
			values := testutils.RandomValues(numInsPerStep, 1, valueMaxByteSize)
			update, err := ledger.NewUpdate(state, keys, values)
			assert.NoError(t, err)
			state, _, err = led.Set(update)
			require.NoError(t, err)

			data := make(map[string]ledger.Value, len(keys))
			for j, key := range keys {
				encKey := ledger.EncodeKey(&key)
				data[string(encKey)] = values[j]
			}

			savedData[string(state[:])] = data
		}

		<-led.Done()
		<-compactor.Done()

		diskWal2, err := wal.NewDiskWAL(zerolog.Nop(), nil, metricsCollector, dir, size, pathfinder.PathByteSize, wal.SegmentSize)
		require.NoError(t, err)

		led2, err := complete.NewLedger(diskWal2, size+10, metricsCollector, logger, complete.DefaultPathFinderVersion)
		require.NoError(t, err)

		compactor2, err := complete.NewCompactor(led2, diskWal2, zerolog.Nop(), uint(size), checkpointDistance, checkpointsToKeep, atomic.NewBool(false))
		require.NoError(t, err)

		<-compactor2.Ready()

		// random map iteration order is a benefit here
		for state, data := range savedData {

			keys := make([]ledger.Key, 0, len(data))
			for encKey := range data {
				key, err := ledger.DecodeKey([]byte(encKey))
				assert.NoError(t, err)
				keys = append(keys, *key)
			}

			var ledgerState ledger.State
			copy(ledgerState[:], state)
			query, err := ledger.NewQuery(ledgerState, keys)
			assert.NoError(t, err)
			registerValues, err := led2.Get(query)
			require.NoError(t, err)

			for i, key := range keys {
				registerValue := registerValues[i]
				encKey := ledger.EncodeKey(&key)
				assert.True(t, data[string(encKey)].Equals(registerValue))
			}
		}

		<-led2.Done()
		<-compactor2.Done()
	})
}

func TestLedgerFunctionality(t *testing.T) {
	const (
		checkpointDistance = math.MaxInt // A large number to prevent checkpoint creation.
		checkpointsToKeep  = 1
	)

	rand.Seed(time.Now().UnixNano())
	// You can manually increase this for more coverage
	experimentRep := 2
	metricsCollector := &metrics.NoopCollector{}
	logger := zerolog.Logger{}

	for e := 0; e < experimentRep; e++ {
		numInsPerStep := 100
		numHistLookupPerStep := 10
		keyNumberOfParts := 10
		keyPartMinByteSize := 1
		keyPartMaxByteSize := 100
		stateComSize := 32
		valueMaxByteSize := 2 << 16 //16kB
		activeTries := 1000
		steps := 40                                  // number of steps
		histStorage := make(map[string]ledger.Value) // historic storage string(key, state) -> value
		latestValue := make(map[string]ledger.Value) // key to value
		unittest.RunWithTempDir(t, func(dbDir string) {
			diskWal, err := wal.NewDiskWAL(zerolog.Nop(), nil, metricsCollector, dbDir, activeTries, pathfinder.PathByteSize, wal.SegmentSize)
			require.NoError(t, err)
			led, err := complete.NewLedger(diskWal, activeTries, metricsCollector, logger, complete.DefaultPathFinderVersion)
			assert.NoError(t, err)
			compactor, err := complete.NewCompactor(led, diskWal, zerolog.Nop(), uint(activeTries), checkpointDistance, checkpointsToKeep, atomic.NewBool(false))
			require.NoError(t, err)
			<-compactor.Ready()

			state := led.InitialState()
			for i := 0; i < steps; i++ {
				// add new keys
				// TODO update some of the existing keys and shuffle them
				keys := testutils.RandomUniqueKeys(numInsPerStep, keyNumberOfParts, keyPartMinByteSize, keyPartMaxByteSize)
				values := testutils.RandomValues(numInsPerStep, 1, valueMaxByteSize)
				update, err := ledger.NewUpdate(state, keys, values)
				assert.NoError(t, err)
				newState, _, err := led.Set(update)
				assert.NoError(t, err)

				// capture new values for future query
				for j, k := range keys {
					encKey := ledger.EncodeKey(&k)
					histStorage[string(newState[:])+string(encKey)] = values[j]
					latestValue[string(encKey)] = values[j]
				}

				// read values and compare values
				query, err := ledger.NewQuery(newState, keys)
				assert.NoError(t, err)
				retValues, err := led.Get(query)
				assert.NoError(t, err)
				// byte{} is returned as nil
				assert.True(t, valuesMatches(values, retValues))

				// get value sizes and compare them
				retSizes, err := led.ValueSizes(query)
				assert.NoError(t, err)
				assert.Equal(t, len(query.Keys()), len(retSizes))
				for i, size := range retSizes {
					assert.Equal(t, values[i].Size(), size)
				}

				// validate proofs (check individual proof and batch proof)
				proofs, err := led.Prove(query)
				assert.NoError(t, err)

				bProof, err := ledger.DecodeTrieBatchProof(proofs)
				assert.NoError(t, err)

				// validate batch proofs
				isValid := proof.VerifyTrieBatchProof(bProof, newState)
				assert.True(t, isValid)

				// validate proofs as a batch
				_, err = ptrie.NewPSMT(ledger.RootHash(newState), bProof)
				assert.NoError(t, err)

				// query all exising keys (check no drop)
				for ek, v := range latestValue {
					k, err := ledger.DecodeKey([]byte(ek))
					assert.NoError(t, err)
					query, err := ledger.NewQuery(newState, []ledger.Key{*k})
					assert.NoError(t, err)
					rv, err := led.Get(query)
					assert.NoError(t, err)
					assert.True(t, v.Equals(rv[0]))
				}

				// query some of historic values (map return is random)
				j := 0
				for s := range histStorage {
					value := histStorage[s]
					var state ledger.State
					copy(state[:], s[:stateComSize])
					enk := []byte(s[stateComSize:])
					key, err := ledger.DecodeKey(enk)
					assert.NoError(t, err)
					query, err := ledger.NewQuery(state, []ledger.Key{*key})
					assert.NoError(t, err)
					rv, err := led.Get(query)
					assert.NoError(t, err)
					assert.True(t, value.Equals(rv[0]))
					j++
					if j >= numHistLookupPerStep {
						break
					}
				}
				state = newState
			}
			<-led.Done()
			<-compactor.Done()
		})
	}
}

func Test_ExportCheckpointAt(t *testing.T) {
	for _, version := range []int{5, 6} {
		t.Run(fmt.Sprintf("noop migration version %v", version), func(t *testing.T) {
			// the exported state has two key/value pairs
			// (/1/1/22/2, "A") and (/1/3/22/4, "B")
			// this tests the migration at the specific state
			// without any special migration so we expect both
			// register to show up in the new trie and with the same values
			unittest.RunWithTempDir(t, func(dbDir string) {
				unittest.RunWithTempDir(t, func(dir2 string) {

					const (
						capacity           = 100
						checkpointDistance = math.MaxInt // A large number to prevent checkpoint creation.
						checkpointsToKeep  = 1
					)

					diskWal, err := wal.NewDiskWAL(zerolog.Nop(), nil, metrics.NewNoopCollector(), dbDir, capacity, pathfinder.PathByteSize, wal.SegmentSize)
					require.NoError(t, err)
					led, err := complete.NewLedger(diskWal, capacity, &metrics.NoopCollector{}, zerolog.Logger{}, complete.DefaultPathFinderVersion)
					require.NoError(t, err)
					compactor, err := complete.NewCompactor(led, diskWal, zerolog.Nop(), capacity, checkpointDistance, checkpointsToKeep, atomic.NewBool(false))
					require.NoError(t, err)
					<-compactor.Ready()

					state := led.InitialState()
					u := testutils.UpdateFixture()
					u.SetState(state)

					state, _, err = led.Set(u)
					require.NoError(t, err)

					newState, err := led.ExportCheckpointAt(state, []ledger.Migration{noOpMigration}, []ledger.Reporter{}, []ledger.Reporter{}, complete.DefaultPathFinderVersion, dir2, "root.checkpoint", version)
					require.NoError(t, err)
					assert.Equal(t, newState, state)

					diskWal2, err := wal.NewDiskWAL(zerolog.Nop(), nil, metrics.NewNoopCollector(), dir2, capacity, pathfinder.PathByteSize, wal.SegmentSize)
					require.NoError(t, err)
					led2, err := complete.NewLedger(diskWal2, capacity, &metrics.NoopCollector{}, zerolog.Logger{}, complete.DefaultPathFinderVersion)
					require.NoError(t, err)
					compactor2, err := complete.NewCompactor(led2, diskWal2, zerolog.Nop(), capacity, checkpointDistance, checkpointsToKeep, atomic.NewBool(false))
					require.NoError(t, err)
					<-compactor2.Ready()

					q, err := ledger.NewQuery(state, u.Keys())
					require.NoError(t, err)

					retValues, err := led2.Get(q)
					require.NoError(t, err)

					for i, v := range u.Values() {
						assert.Equal(t, v, retValues[i])
					}

					<-led.Done()
					<-compactor.Done()
					<-led2.Done()
					<-compactor2.Done()
				})
			})
		})
		t.Run(fmt.Sprintf("migration by value: version %v", version), func(t *testing.T) {
			// the exported state has two key/value pairs
			// ("/1/1/22/2", "A") and ("/1/3/22/4", "B")
			// during the migration we change all keys with value "A" to "C"
			// so in this case the resulting exported trie is ("/1/1/22/2", "C"), ("/1/3/22/4", "B")
			unittest.RunWithTempDir(t, func(dbDir string) {
				unittest.RunWithTempDir(t, func(dir2 string) {

					const (
						capacity           = 100
						checkpointDistance = math.MaxInt // A large number to prevent checkpoint creation.
						checkpointsToKeep  = 1
					)

					diskWal, err := wal.NewDiskWAL(zerolog.Nop(), nil, metrics.NewNoopCollector(), dbDir, capacity, pathfinder.PathByteSize, wal.SegmentSize)
					require.NoError(t, err)
					led, err := complete.NewLedger(diskWal, capacity, &metrics.NoopCollector{}, zerolog.Logger{}, complete.DefaultPathFinderVersion)
					require.NoError(t, err)
					compactor, err := complete.NewCompactor(led, diskWal, zerolog.Nop(), capacity, checkpointDistance, checkpointsToKeep, atomic.NewBool(false))
					require.NoError(t, err)
					<-compactor.Ready()

					state := led.InitialState()
					u := testutils.UpdateFixture()
					u.SetState(state)

					state, _, err = led.Set(u)
					require.NoError(t, err)

					newState, err := led.ExportCheckpointAt(state, []ledger.Migration{migrationByValue}, []ledger.Reporter{}, []ledger.Reporter{}, complete.DefaultPathFinderVersion, dir2, "root.checkpoint", version)
					require.NoError(t, err)

					diskWal2, err := wal.NewDiskWAL(zerolog.Nop(), nil, metrics.NewNoopCollector(), dir2, capacity, pathfinder.PathByteSize, wal.SegmentSize)
					require.NoError(t, err)
					led2, err := complete.NewLedger(diskWal2, capacity, &metrics.NoopCollector{}, zerolog.Logger{}, complete.DefaultPathFinderVersion)
					require.NoError(t, err)
					compactor2, err := complete.NewCompactor(led2, diskWal2, zerolog.Nop(), capacity, checkpointDistance, checkpointsToKeep, atomic.NewBool(false))
					require.NoError(t, err)
					<-compactor2.Ready()

					q, err := ledger.NewQuery(newState, u.Keys())
					require.NoError(t, err)

					retValues, err := led2.Get(q)
					require.NoError(t, err)

					assert.Equal(t, retValues[0], ledger.Value([]byte{'C'}))
					assert.Equal(t, retValues[1], ledger.Value([]byte{'B'}))

					<-led.Done()
					<-compactor.Done()
					<-led2.Done()
					<-compactor2.Done()
				})
			})
		})
		t.Run(fmt.Sprintf("migration by key: version %v", version), func(t *testing.T) {
			// the exported state has two key/value pairs
			// ("/1/1/22/2", "A") and ("/1/3/22/4", "B")
			// during the migration we change the value to "D" for key "zero"
			// so in this case the resulting exported trie is ("/1/1/22/2", "D"), ("/1/3/22/4", "B")
			unittest.RunWithTempDir(t, func(dbDir string) {
				unittest.RunWithTempDir(t, func(dir2 string) {

					const (
						capacity           = 100
						checkpointDistance = math.MaxInt // A large number to prevent checkpoint creation.
						checkpointsToKeep  = 1
					)

					diskWal, err := wal.NewDiskWAL(zerolog.Nop(), nil, metrics.NewNoopCollector(), dbDir, capacity, pathfinder.PathByteSize, wal.SegmentSize)
					require.NoError(t, err)
					led, err := complete.NewLedger(diskWal, capacity, &metrics.NoopCollector{}, zerolog.Logger{}, complete.DefaultPathFinderVersion)
					require.NoError(t, err)
					compactor, err := complete.NewCompactor(led, diskWal, zerolog.Nop(), capacity, checkpointDistance, checkpointsToKeep, atomic.NewBool(false))
					require.NoError(t, err)
					<-compactor.Ready()

					state := led.InitialState()
					u := testutils.UpdateFixture()
					u.SetState(state)

					state, _, err = led.Set(u)
					require.NoError(t, err)

					newState, err := led.ExportCheckpointAt(state, []ledger.Migration{migrationByKey}, []ledger.Reporter{}, []ledger.Reporter{}, complete.DefaultPathFinderVersion, dir2, "root.checkpoint", version)
					require.NoError(t, err)

					diskWal2, err := wal.NewDiskWAL(zerolog.Nop(), nil, metrics.NewNoopCollector(), dir2, capacity, pathfinder.PathByteSize, wal.SegmentSize)
					require.NoError(t, err)
					led2, err := complete.NewLedger(diskWal2, capacity, &metrics.NoopCollector{}, zerolog.Logger{}, complete.DefaultPathFinderVersion)
					require.NoError(t, err)
					compactor2, err := complete.NewCompactor(led2, diskWal2, zerolog.Nop(), capacity, checkpointDistance, checkpointsToKeep, atomic.NewBool(false))
					require.NoError(t, err)
					<-compactor2.Ready()

					q, err := ledger.NewQuery(newState, u.Keys())
					require.NoError(t, err)

					retValues, err := led2.Get(q)
					require.NoError(t, err)

					assert.Equal(t, retValues[0], ledger.Value([]byte{'D'}))
					assert.Equal(t, retValues[1], ledger.Value([]byte{'B'}))

					<-led.Done()
					<-compactor.Done()
					<-led2.Done()
					<-compactor2.Done()
				})
			})
		})
	}
}

func TestWALUpdateFailuresBubbleUp(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {

		const (
			capacity           = 100
			checkpointDistance = math.MaxInt // A large number to prevent checkpoint creation.
			checkpointsToKeep  = 1
		)

		theError := fmt.Errorf("error error")

		metricsCollector := &metrics.NoopCollector{}

		diskWAL, err := wal.NewDiskWAL(zerolog.Nop(), nil, metricsCollector, dir, capacity, pathfinder.PathByteSize, wal.SegmentSize)
		require.NoError(t, err)

		w := &CustomUpdateWAL{
			DiskWAL: diskWAL,
			updateFn: func(update *ledger.TrieUpdate) (int, bool, error) {
				return 0, false, theError
			},
		}

		led, err := complete.NewLedger(w, capacity, &metrics.NoopCollector{}, zerolog.Logger{}, complete.DefaultPathFinderVersion)
		require.NoError(t, err)

		compactor, err := complete.NewCompactor(led, w, zerolog.Nop(), capacity, checkpointDistance, checkpointsToKeep, atomic.NewBool(false))
		require.NoError(t, err)

		<-compactor.Ready()

		defer func() {
			<-led.Done()
			<-compactor.Done()
		}()

		key := ledger.NewKey([]ledger.KeyPart{ledger.NewKeyPart(0, []byte{1, 2, 3})})

		values := []ledger.Value{[]byte{1, 2, 3}}
		update, err := ledger.NewUpdate(led.InitialState(), []ledger.Key{key}, values)
		require.NoError(t, err)

		_, _, err = led.Set(update)
		require.Error(t, err)
		require.True(t, errors.Is(err, theError))
	})
}

func valuesMatches(expected []ledger.Value, got []ledger.Value) bool {
	if len(expected) != len(got) {
		return false
	}
	// replace nils
	for i, v := range got {
		if v == nil {
			got[i] = []byte{}
		}
		if !bytes.Equal(expected[i], got[i]) {
			return false
		}
	}
	return true
}

func noOpMigration(p []ledger.Payload) ([]ledger.Payload, error) {
	return p, nil
}

func migrationByValue(p []ledger.Payload) ([]ledger.Payload, error) {
	ret := make([]ledger.Payload, 0, len(p))
	for _, p := range p {
		if p.Value().Equals([]byte{'A'}) {
			k, err := p.Key()
			if err != nil {
				return nil, err
			}
			pp := *ledger.NewPayload(k, ledger.Value([]byte{'C'}))
			ret = append(ret, pp)
		} else {
			ret = append(ret, p)
		}
	}
	return ret, nil
}

type CustomUpdateWAL struct {
	*wal.DiskWAL
	updateFn func(update *ledger.TrieUpdate) (int, bool, error)
}

func (w *CustomUpdateWAL) RecordUpdate(update *ledger.TrieUpdate) (int, bool, error) {
	return w.updateFn(update)
}

func migrationByKey(p []ledger.Payload) ([]ledger.Payload, error) {
	ret := make([]ledger.Payload, 0, len(p))
	for _, p := range p {
		k, err := p.Key()
		if err != nil {
			return nil, err
		}
		if k.String() == "/1/1/22/2" {
			pp := *ledger.NewPayload(k, ledger.Value([]byte{'D'}))
			ret = append(ret, pp)
		} else {
			ret = append(ret, p)
		}
	}
	return ret, nil
}
