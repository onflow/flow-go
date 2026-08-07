package complete_test

import (
	"bytes"
	"fmt"
	"math"
	"math/rand"
	"testing"

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

		newState, trieUpdate, err := l.Set(up)
		require.NoError(t, err)
		require.True(t, trieUpdate.IsEmpty())

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

		compactor, err := complete.NewCompactor(led, diskWal, zerolog.Nop(), size, checkpointDistance, checkpointsToKeep, atomic.NewBool(false), metrics.NewNoopCollector())
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

		compactor2, err := complete.NewCompactor(led2, diskWal2, zerolog.Nop(), uint(size), checkpointDistance, checkpointsToKeep, atomic.NewBool(false), metrics.NewNoopCollector())
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
			compactor, err := complete.NewCompactor(led, diskWal, zerolog.Nop(), uint(activeTries), checkpointDistance, checkpointsToKeep, atomic.NewBool(false), metrics.NewNoopCollector())
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

		compactor, err := complete.NewCompactor(led, w, zerolog.Nop(), capacity, checkpointDistance, checkpointsToKeep, atomic.NewBool(false), metrics.NewNoopCollector())
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
		require.ErrorIs(t, err, theError)
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

func TestLedger_StateCount(t *testing.T) {
	wal := &fixtures.NoopWAL{}
	l, err := complete.NewLedger(wal, 100, &metrics.NoopCollector{}, zerolog.Logger{}, complete.DefaultPathFinderVersion)
	require.NoError(t, err)

	compactor := fixtures.NewNoopCompactor(l)
	<-compactor.Ready()
	defer func() {
		<-l.Done()
		<-compactor.Done()
	}()

	// Initially should have at least the empty trie
	initialCount := l.StateCount()
	assert.GreaterOrEqual(t, initialCount, 1, "should have at least one state (empty trie)")

	// Create some updates to add more states
	state := l.InitialState()
	keys := testutils.RandomUniqueKeys(3, 2, 2, 4)
	values := testutils.RandomValues(3, 1, 32)

	// First update
	update1, err := ledger.NewUpdate(state, keys[0:1], values[0:1])
	require.NoError(t, err)
	newState1, _, err := l.Set(update1)
	require.NoError(t, err)

	// Second update from the first new state
	update2, err := ledger.NewUpdate(newState1, keys[1:2], values[1:2])
	require.NoError(t, err)
	newState2, _, err := l.Set(update2)
	require.NoError(t, err)

	// State count should have increased
	finalCount := l.StateCount()
	assert.Greater(t, finalCount, initialCount, "state count should increase after updates")
	_ = newState2 // avoid unused variable
}

func TestLedger_StateByIndex(t *testing.T) {
	wal := &fixtures.NoopWAL{}
	l, err := complete.NewLedger(wal, 100, &metrics.NoopCollector{}, zerolog.Logger{}, complete.DefaultPathFinderVersion)
	require.NoError(t, err)

	compactor := fixtures.NewNoopCompactor(l)
	<-compactor.Ready()
	defer func() {
		<-l.Done()
		<-compactor.Done()
	}()

	// Get initial state
	initialState := l.InitialState()
	stateCount := l.StateCount()
	require.Greater(t, stateCount, 0, "should have at least one state")

	// Test getting state at index 0
	state0, err := l.StateByIndex(0)
	require.NoError(t, err)
	assert.NotEqual(t, ledger.DummyState, state0, "state at index 0 should not be dummy state")

	// Test getting last state using -1
	lastState, err := l.StateByIndex(-1)
	require.NoError(t, err)
	assert.NotEqual(t, ledger.DummyState, lastState, "last state should not be dummy state")

	// Create some updates to add more states
	state := initialState
	keys := testutils.RandomUniqueKeys(3, 2, 2, 4)
	values := testutils.RandomValues(3, 1, 32)

	// Create multiple updates
	for i := 0; i < 3; i++ {
		update, err := ledger.NewUpdate(state, keys[i:i+1], values[i:i+1])
		require.NoError(t, err)
		newState, _, err := l.Set(update)
		require.NoError(t, err)
		state = newState
	}

	// Verify we can get states by index
	finalCount := l.StateCount()
	require.GreaterOrEqual(t, finalCount, 1, "should have at least one state")

	// Test getting first state
	firstState, err := l.StateByIndex(0)
	require.NoError(t, err)
	assert.NotEqual(t, ledger.DummyState, firstState)

	// Test getting last state with -1
	lastStateAfterUpdates, err := l.StateByIndex(-1)
	require.NoError(t, err)
	assert.NotEqual(t, ledger.DummyState, lastStateAfterUpdates)

	// Test getting last state with positive index
	if finalCount > 0 {
		lastStateByIndex, err := l.StateByIndex(finalCount - 1)
		require.NoError(t, err)
		assert.NotEqual(t, ledger.DummyState, lastStateByIndex)
		// Last state by index should match last state by -1
		assert.Equal(t, lastStateAfterUpdates, lastStateByIndex, "last state by -1 should match last state by positive index")
	}

	// Test out of range indices
	_, err = l.StateByIndex(finalCount)
	require.Error(t, err, "should error for index out of range")

	_, err = l.StateByIndex(-finalCount - 1)
	require.Error(t, err, "should error for negative index out of range")
}

func TestLedgerWithCompactor_StateCountAndStateByIndex(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		metricsCollector := &metrics.NoopCollector{}
		diskWal, err := wal.NewDiskWAL(zerolog.Nop(), nil, metricsCollector, dir, 100, pathfinder.PathByteSize, wal.SegmentSize)
		require.NoError(t, err)

		compactorConfig := ledger.DefaultCompactorConfig(metricsCollector)
		lwc, err := complete.NewLedgerWithCompactor(
			diskWal,
			100,
			compactorConfig,
			atomic.NewBool(false),
			metricsCollector,
			zerolog.Nop(),
			complete.DefaultPathFinderVersion,
		)
		require.NoError(t, err)

		<-lwc.Ready()
		defer func() {
			<-lwc.Done()
		}()

		// Test StateCount
		initialCount := lwc.StateCount()
		assert.GreaterOrEqual(t, initialCount, 1, "should have at least one state")

		// Test StateByIndex
		state0, err := lwc.StateByIndex(0)
		require.NoError(t, err)
		assert.NotEqual(t, ledger.DummyState, state0)

		lastState, err := lwc.StateByIndex(-1)
		require.NoError(t, err)
		assert.NotEqual(t, ledger.DummyState, lastState)

		// Create some updates
		state := lwc.InitialState()
		keys := testutils.RandomUniqueKeys(2, 2, 2, 4)
		values := testutils.RandomValues(2, 1, 32)

		update, err := ledger.NewUpdate(state, keys, values)
		require.NoError(t, err)
		newState, _, err := lwc.Set(update)
		require.NoError(t, err)

		// Verify StateCount increased
		finalCount := lwc.StateCount()
		assert.Greater(t, finalCount, initialCount, "state count should increase after update")

		// Verify we can get the new state
		lastStateAfterUpdate, err := lwc.StateByIndex(-1)
		require.NoError(t, err)
		assert.Equal(t, ledger.State(newState), lastStateAfterUpdate, "last state should match the new state")
	})
}
