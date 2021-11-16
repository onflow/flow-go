package complete_test

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/encoding"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/common/proof"
	"github.com/onflow/flow-go/ledger/common/utils"
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

		curSC := led.InitialState()

		u := utils.UpdateFixture()
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

		curS := led.InitialState()

		q := utils.QueryFixture()
		q.SetState(curS)

		retValues, err := led.Get(q)
		require.NoError(t, err)

		assert.Equal(t, 2, len(retValues))
		assert.Equal(t, 0, len(retValues[0]))
		assert.Equal(t, 0, len(retValues[1]))

	})
}

func TestLedger_Proof(t *testing.T) {
	t.Run("empty query", func(t *testing.T) {
		wal := &fixtures.NoopWAL{}

		led, err := complete.NewLedger(wal, 100, &metrics.NoopCollector{}, zerolog.Logger{}, complete.DefaultPathFinderVersion)
		require.NoError(t, err)

		curSC := led.InitialState()
		q, err := ledger.NewEmptyQuery(curSC)
		require.NoError(t, err)

		retProof, err := led.Prove(q)
		require.NoError(t, err)

		proof, err := encoding.DecodeTrieBatchProof(retProof)
		require.NoError(t, err)
		assert.Equal(t, 0, len(proof.Proofs))
	})

	t.Run("non-existing keys", func(t *testing.T) {

		wal := &fixtures.NoopWAL{}

		led, err := complete.NewLedger(wal, 100, &metrics.NoopCollector{}, zerolog.Logger{}, complete.DefaultPathFinderVersion)
		require.NoError(t, err)

		curS := led.InitialState()
		q := utils.QueryFixture()
		q.SetState(curS)
		require.NoError(t, err)

		retProof, err := led.Prove(q)
		require.NoError(t, err)

		trieProof, err := encoding.DecodeTrieBatchProof(retProof)
		require.NoError(t, err)
		assert.Equal(t, 2, len(trieProof.Proofs))
		assert.True(t, proof.VerifyTrieBatchProof(trieProof, curS))

	})

	t.Run("existing keys", func(t *testing.T) {

		wal := &fixtures.NoopWAL{}
		led, err := complete.NewLedger(wal, 100, &metrics.NoopCollector{}, zerolog.Logger{}, complete.DefaultPathFinderVersion)
		require.NoError(t, err)

		curS := led.InitialState()

		u := utils.UpdateFixture()
		u.SetState(curS)

		newSc, _, err := led.Set(u)
		require.NoError(t, err)
		assert.NotEqual(t, curS, newSc)

		q, err := ledger.NewQuery(newSc, u.Keys())
		require.NoError(t, err)

		retProof, err := led.Prove(q)
		require.NoError(t, err)

		trieProof, err := encoding.DecodeTrieBatchProof(retProof)
		require.NoError(t, err)
		assert.Equal(t, 2, len(trieProof.Proofs))
		assert.True(t, proof.VerifyTrieBatchProof(trieProof, newSc))
	})
}

func Test_WAL(t *testing.T) {
	numInsPerStep := 2
	keyNumberOfParts := 10
	keyPartMinByteSize := 1
	keyPartMaxByteSize := 100
	valueMaxByteSize := 2 << 16 //16kB
	size := 10
	metricsCollector := &metrics.NoopCollector{}
	logger := zerolog.Logger{}

	unittest.RunWithTempDir(t, func(dir string) {

		diskWal, err := wal.NewDiskWAL(zerolog.Nop(), nil, metricsCollector, dir, size, pathfinder.PathByteSize, wal.SegmentSize)
		require.NoError(t, err)

		// cache size intentionally is set to size to test deletion
		led, err := complete.NewLedger(diskWal, size, metricsCollector, logger, complete.DefaultPathFinderVersion)
		require.NoError(t, err)

		var state = led.InitialState()

		//saved data after updates
		savedData := make(map[string]map[string]ledger.Value)

		for i := 0; i < size; i++ {

			keys := utils.RandomUniqueKeys(numInsPerStep, keyNumberOfParts, keyPartMinByteSize, keyPartMaxByteSize)
			values := utils.RandomValues(numInsPerStep, 1, valueMaxByteSize)
			update, err := ledger.NewUpdate(state, keys, values)
			assert.NoError(t, err)
			state, _, err = led.Set(update)
			require.NoError(t, err)
			fmt.Printf("Updated with %x\n", state)

			data := make(map[string]ledger.Value, len(keys))
			for j, key := range keys {
				encKey := encoding.EncodeKey(&key)
				data[string(encKey)] = values[j]
			}

			savedData[string(state[:])] = data
		}

		<-diskWal.Done()
		<-led.Done()

		diskWal2, err := wal.NewDiskWAL(zerolog.Nop(), nil, metricsCollector, dir, size, pathfinder.PathByteSize, wal.SegmentSize)
		require.NoError(t, err)

		led2, err := complete.NewLedger(diskWal2, size+10, metricsCollector, logger, complete.DefaultPathFinderVersion)
		require.NoError(t, err)

		// random map iteration order is a benefit here
		for state, data := range savedData {

			keys := make([]ledger.Key, 0, len(data))
			for encKey := range data {
				key, err := encoding.DecodeKey([]byte(encKey))
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
				encKey := encoding.EncodeKey(&key)
				assert.True(t, data[string(encKey)].Equals(registerValue))
			}
		}

		// test deletion
		s := led2.ForestSize()
		assert.Equal(t, s, size)

		<-diskWal2.Done()
		<-led2.Done()
	})
}

func TestLedgerFunctionality(t *testing.T) {
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
			state := led.InitialState()
			for i := 0; i < steps; i++ {
				// add new keys
				// TODO update some of the existing keys and shuffle them
				keys := utils.RandomUniqueKeys(numInsPerStep, keyNumberOfParts, keyPartMinByteSize, keyPartMaxByteSize)
				values := utils.RandomValues(numInsPerStep, 1, valueMaxByteSize)
				update, err := ledger.NewUpdate(state, keys, values)
				assert.NoError(t, err)
				newState, _, err := led.Set(update)
				assert.NoError(t, err)

				// capture new values for future query
				for j, k := range keys {
					encKey := encoding.EncodeKey(&k)
					histStorage[string(newState[:])+string(encKey[:])] = values[j]
					latestValue[string(encKey)] = values[j]
				}

				// read values and compare values
				query, err := ledger.NewQuery(newState, keys)
				assert.NoError(t, err)
				retValues, err := led.Get(query)
				assert.NoError(t, err)
				// byte{} is returned as nil
				assert.True(t, valuesMatches(values, retValues))

				// validate proofs (check individual proof and batch proof)
				proofs, err := led.Prove(query)
				assert.NoError(t, err)

				bProof, err := encoding.DecodeTrieBatchProof(proofs)
				assert.NoError(t, err)

				// validate batch proofs
				isValid := proof.VerifyTrieBatchProof(bProof, newState)
				assert.True(t, isValid)

				// validate proofs as a batch
				_, err = ptrie.NewPSMT(ledger.RootHash(newState), bProof)
				assert.NoError(t, err)

				// query all exising keys (check no drop)
				for ek, v := range latestValue {
					k, err := encoding.DecodeKey([]byte(ek))
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
					key, err := encoding.DecodeKey(enk)
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
			<-diskWal.Done()
		})
	}
}

func Test_ExportCheckpointAt(t *testing.T) {
	t.Run("noop migration", func(t *testing.T) {
		// the exported state has two key/value pairs
		// (/1/1/22/2, "A") and (/1/3/22/4, "B")
		// this tests the migration at the specific state
		// without any special migration so we expect both
		// register to show up in the new trie and with the same values
		unittest.RunWithTempDir(t, func(dbDir string) {
			unittest.RunWithTempDir(t, func(dir2 string) {

				diskWal, err := wal.NewDiskWAL(zerolog.Nop(), nil, metrics.NewNoopCollector(), dbDir, 100, pathfinder.PathByteSize, wal.SegmentSize)
				require.NoError(t, err)
				led, err := complete.NewLedger(diskWal, 100, &metrics.NoopCollector{}, zerolog.Logger{}, complete.DefaultPathFinderVersion)
				require.NoError(t, err)

				state := led.InitialState()
				u := utils.UpdateFixture()
				u.SetState(state)

				state, _, err = led.Set(u)
				require.NoError(t, err)

				newState, err := led.ExportCheckpointAt(state, []ledger.Migration{noOpMigration}, []ledger.Reporter{}, complete.DefaultPathFinderVersion, dir2, "root.checkpoint")
				require.NoError(t, err)
				assert.Equal(t, newState, state)

				diskWal2, err := wal.NewDiskWAL(zerolog.Nop(), nil, metrics.NewNoopCollector(), dir2, 100, pathfinder.PathByteSize, wal.SegmentSize)
				require.NoError(t, err)
				led2, err := complete.NewLedger(diskWal2, 100, &metrics.NoopCollector{}, zerolog.Logger{}, complete.DefaultPathFinderVersion)
				require.NoError(t, err)

				q, err := ledger.NewQuery(state, u.Keys())
				require.NoError(t, err)

				retValues, err := led2.Get(q)
				require.NoError(t, err)

				for i, v := range u.Values() {
					assert.Equal(t, v, retValues[i])
				}

				<-diskWal.Done()
				<-diskWal2.Done()
			})
		})
	})
	t.Run("migration by value", func(t *testing.T) {
		// the exported state has two key/value pairs
		// ("/1/1/22/2", "A") and ("/1/3/22/4", "B")
		// during the migration we change all keys with value "A" to "C"
		// so in this case the resulting exported trie is ("/1/1/22/2", "C"), ("/1/3/22/4", "B")
		unittest.RunWithTempDir(t, func(dbDir string) {
			unittest.RunWithTempDir(t, func(dir2 string) {

				diskWal, err := wal.NewDiskWAL(zerolog.Nop(), nil, metrics.NewNoopCollector(), dbDir, 100, pathfinder.PathByteSize, wal.SegmentSize)
				require.NoError(t, err)
				led, err := complete.NewLedger(diskWal, 100, &metrics.NoopCollector{}, zerolog.Logger{}, complete.DefaultPathFinderVersion)
				require.NoError(t, err)

				state := led.InitialState()
				u := utils.UpdateFixture()
				u.SetState(state)

				state, _, err = led.Set(u)
				require.NoError(t, err)

				newState, err := led.ExportCheckpointAt(state, []ledger.Migration{migrationByValue}, []ledger.Reporter{}, complete.DefaultPathFinderVersion, dir2, "root.checkpoint")
				require.NoError(t, err)

				diskWal2, err := wal.NewDiskWAL(zerolog.Nop(), nil, metrics.NewNoopCollector(), dir2, 100, pathfinder.PathByteSize, wal.SegmentSize)
				require.NoError(t, err)
				led2, err := complete.NewLedger(diskWal2, 100, &metrics.NoopCollector{}, zerolog.Logger{}, complete.DefaultPathFinderVersion)
				require.NoError(t, err)

				q, err := ledger.NewQuery(newState, u.Keys())
				require.NoError(t, err)

				retValues, err := led2.Get(q)
				require.NoError(t, err)

				assert.Equal(t, retValues[0], ledger.Value([]byte{'C'}))
				assert.Equal(t, retValues[1], ledger.Value([]byte{'B'}))

				<-diskWal.Done()
				<-diskWal2.Done()
			})
		})
	})
	t.Run("migration by key", func(t *testing.T) {
		// the exported state has two key/value pairs
		// ("/1/1/22/2", "A") and ("/1/3/22/4", "B")
		// during the migration we change the value to "D" for key "zero"
		// so in this case the resulting exported trie is ("/1/1/22/2", "D"), ("/1/3/22/4", "B")
		unittest.RunWithTempDir(t, func(dbDir string) {
			unittest.RunWithTempDir(t, func(dir2 string) {

				diskWal, err := wal.NewDiskWAL(zerolog.Nop(), nil, metrics.NewNoopCollector(), dbDir, 100, pathfinder.PathByteSize, wal.SegmentSize)
				require.NoError(t, err)
				led, err := complete.NewLedger(diskWal, 100, &metrics.NoopCollector{}, zerolog.Logger{}, complete.DefaultPathFinderVersion)
				require.NoError(t, err)

				state := led.InitialState()
				u := utils.UpdateFixture()
				u.SetState(state)

				state, _, err = led.Set(u)
				require.NoError(t, err)

				newState, err := led.ExportCheckpointAt(state, []ledger.Migration{migrationByKey}, []ledger.Reporter{}, complete.DefaultPathFinderVersion, dir2, "root.checkpoint")
				require.NoError(t, err)

				diskWal2, err := wal.NewDiskWAL(zerolog.Nop(), nil, metrics.NewNoopCollector(), dir2, 100, pathfinder.PathByteSize, wal.SegmentSize)
				require.NoError(t, err)
				led2, err := complete.NewLedger(diskWal2, 100, &metrics.NoopCollector{}, zerolog.Logger{}, complete.DefaultPathFinderVersion)
				require.NoError(t, err)

				q, err := ledger.NewQuery(newState, u.Keys())
				require.NoError(t, err)

				retValues, err := led2.Get(q)
				require.NoError(t, err)

				assert.Equal(t, retValues[0], ledger.Value([]byte{'D'}))
				assert.Equal(t, retValues[1], ledger.Value([]byte{'B'}))

				<-diskWal.Done()
				<-diskWal2.Done()
			})
		})
	})
}

func TestWALUpdateIsRunInParallel(t *testing.T) {

	// The idea of this test is - WAL update should be run in parallel
	// so we block it until we can find a new trie with expected state
	// this doesn't really proves WAL update is run in parallel, but at least
	// checks if its run after the trie update

	wg := sync.WaitGroup{}
	wg.Add(1)

	w := &LongRunningDummyWAL{
		updateFn: func(update *ledger.TrieUpdate) error {
			wg.Wait() //wg will let work after the trie has been updated
			return nil
		},
	}

	led, err := complete.NewLedger(w, 100, &metrics.NoopCollector{}, zerolog.Logger{}, complete.DefaultPathFinderVersion)
	require.NoError(t, err)

	key := ledger.NewKey([]ledger.KeyPart{ledger.NewKeyPart(0, []byte{1, 2, 3})})

	values := []ledger.Value{[]byte{1, 2, 3}}
	update, err := ledger.NewUpdate(led.InitialState(), []ledger.Key{key}, values)
	require.NoError(t, err)

	// this state should correspond to fresh state with given update
	decoded, err := hex.DecodeString("097b7f74413bc03200889c34c6979eacbad58345ef7c0c65e8057a071440df75")
	var expectedState ledger.State
	copy(expectedState[:], decoded)
	require.NoError(t, err)

	query, err := ledger.NewQuery(expectedState, []ledger.Key{key})
	require.NoError(t, err)

	go func() {
		newState, _, err := led.Set(update)
		require.NoError(t, err)
		require.Equal(t, newState, expectedState)
	}()

	require.Eventually(t, func() bool {
		retrievedValues, err := led.Get(query)
		if err != nil {
			return false
		}

		require.NoError(t, err)
		require.Equal(t, values, retrievedValues)

		wg.Done()

		return true
	}, 500*time.Millisecond, 5*time.Millisecond)
}

func TestWALUpdateFailuresBubbleUp(t *testing.T) {

	theError := fmt.Errorf("error error")

	w := &LongRunningDummyWAL{
		updateFn: func(update *ledger.TrieUpdate) error {
			return theError
		},
	}

	led, err := complete.NewLedger(w, 100, &metrics.NoopCollector{}, zerolog.Logger{}, complete.DefaultPathFinderVersion)
	require.NoError(t, err)

	key := ledger.NewKey([]ledger.KeyPart{ledger.NewKeyPart(0, []byte{1, 2, 3})})

	values := []ledger.Value{[]byte{1, 2, 3}}
	update, err := ledger.NewUpdate(led.InitialState(), []ledger.Key{key}, values)
	require.NoError(t, err)

	_, _, err = led.Set(update)
	require.Error(t, err)
	require.True(t, errors.Is(err, theError))
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
		if p.Value.Equals([]byte{'A'}) {
			pp := ledger.Payload{Key: p.Key, Value: ledger.Value([]byte{'C'})}
			ret = append(ret, pp)
		} else {
			ret = append(ret, p)
		}
	}
	return ret, nil
}

type LongRunningDummyWAL struct {
	fixtures.NoopWAL
	updateFn func(update *ledger.TrieUpdate) error
}

func (w *LongRunningDummyWAL) RecordUpdate(update *ledger.TrieUpdate) error {
	return w.updateFn(update)
}

func migrationByKey(p []ledger.Payload) ([]ledger.Payload, error) {
	ret := make([]ledger.Payload, 0, len(p))
	for _, p := range p {
		if p.Key.String() == "/1/1/22/2" {
			pp := ledger.Payload{Key: p.Key, Value: ledger.Value([]byte{'D'})}
			ret = append(ret, pp)
		} else {
			ret = append(ret, p)
		}
	}
	return ret, nil
}

func TestBugOnTestnet(t *testing.T) {

	dir := "/Users/ramtin/go/src/github.com/onflow/flow-go/ledger/debugpack"

	// state before update 0f045d7f5e7ae523c08cea0159786eac043b5f16d1e731f32f2ef0875c23d96d
	// expected correct state after update is e5a2886fe5005f8ec0dfecc09238730625e15a5db103d6ef40a90a491ac57294
	// we get 9b5207c85232bfb5b77a0e149cbd31d3461634a0d0077f19ace01c257e0157ea
	// these registers suposed to be zero len after update (check the content of the update)
	// but they end up with values
	// > {"Key":{"KeyParts":[{"Type":0,"Value":"6e0797ac987005f5"},{"Type":1,"Value":""},{"Type":2,"Value":"24000000000bdfa98b"}]},"Value":"00086e0797ac987005f5000000000bdfa98c83005b0000000000000170c25f343004ef3ad4c25f42b60ce926a6c25f9f906127f26bc2601a429afc6072c260685e291b0100c261156ed521481bc261b49a81543302c261db1e3fa0b6e5c262498d98ec5290c2637900e1211d8ec263f26e2278f43fc2651b8b43712104c26524c5e25ab545c2657c00f6c33d71c266512be68d471cc266fc4df7ee7721c2670fcaabd258cfc2679a60065529b1c2682b0bc3b2639ac2683a862b4944f1c268d5f4b73253a1c269699dd7da1c7ac26986a3d3dc8b84c26a829d8028f899c26aac988e4d99c8c26b14005b6e6193c26bdb16e4242560c26c1a6ceef70c94c26c1ae7e4e6fc75c26c881b2a419109c26c89b1d5474991c26d149b227075cac26d57e5bc1f2837c26d94e0b20419e7c26e3f526470e87ac26e4949def79f68c26e6a5f7957bc17c26eed88ab1dc71ec26f0c470c14b3adc26f5fe4e7f3bcbcc26f65b83003231fc26f9df772d902b7c26fa4ce0253d8edc26fc18673337901c26fff2228f95ab9c270383fc60d530d9b000000000000002e82d8876836315f3130363231f582d887693132395f3130373330f582d8876839305f3130373936f582d887693131335f3130373634f582d887693135335f3130333235f582d887693132395f3131313535f582d8876838365f3130313037f582d8876835385f3131303837f582d8876838385f3130393234f582d8876836375f3130373436f582d8876839305f3130323933f582d887693136355f3131303438f582d887693133325f3130333034f582d887693134305f3130353637f582d887693136395f3130343738f582d887693132395f3130323539f582d887693131375f3130333530f582d887693133345f3130383233f582d8876838395f3130393639f582d8876834315f3130373934f582d887693132395f3131313331f582d887693134365f3130303931f582d8876837385f3130323439f582d887693133305f3130323335f582d887693132375f3130333835f582d887693135315f3130343637f582d887693131335f3131313636f582d8876837305f3130343432f582d8876839345f3131303434f582d8876839355f3130353736f582d887693130345f3130383133f582d8876836365f3131303439f582d8876836305f3130343432f582d8876838305f3130393530f582d8876838325f3130333439f582d887693137305f3130353630f582d8876837325f3130323135f582d887693130365f3130393731f582d887693131385f3130363030f582d8876839365f3130313637f582d887693130385f3130343537f582d8876835385f3130313634f582d887693132375f3130393733f582d887693135325f3130383234f582d8876838335f3130353139f582d887693135315f3131303334f5"}
	// > {"Key":{"KeyParts":[{"Type":0,"Value":"6e0797ac987005f5"},{"Type":1,"Value":""},{"Type":2,"Value":"24000000000bdfabbf"}]},"Value":"00086e0797ac987005f5000000000bdfabc083005b0000000000000170f38dade79ccc6a05f38daf29f522ffbef38dd9dc7b34bc97f38e925701b6f441f38eca4abcf7a1daf38f8a447466bb50f38ff81e027c633af39004b544234088f390899683d74decf390ae1d70bcf712f3918a432fcfc822f3921e911464f0eef392a93eda782021f3951757582ed1a7f395cf9065f981fbf395dfee0d06c681f3964023b0d4067bf39694146e82a22bf3969e17c3f5dd44f39700e25e710daef3971d72ea609ab3f397da927531a551f398e12d666879cdf3996672b6a7a40df3998104f8ea4c8df399e77c13e4b3a4f399ef25236afa90f39a61b0cc328a61f39a8400e1a1c2dff39aaafcf6973f2cf39abdcc6abc2612f39b08ae50a0adcff39b471104fa606ef39bc3519fe5ed55f39bf86d29417ddbf39bf9d0cb9438cbf39ca53a4d99a0c2f39e2d672e06c921f39f4715d9e6f5ecf39f73a8607c7964f39fd40222b43f75f3a0e7d550d77c11f3a1b61614a425bff3a1e8a7d47f46e4f3a27daa99dfdc3cf3a2f6684a3c16529b000000000000002e82d8876835355f3130353236f582d887693134395f3130343036f582d8876838375f3130323733f582d8876834315f3130393632f582d887693130315f3131313033f582d887693134395f3131303538f582d8876839305f3130383034f582d887693136325f3130393433f582d887693136335f3130373235f582d887693132315f3130323036f582d887693135315f3130393938f582d887693131385f3130313437f582d8876837365f3130363630f582d8876837395f3130373037f582d8876836395f3131313532f582d8876834365f3131303631f582d887693131315f3130363231f582d8876838395f3130353332f582d887693133365f3131313238f582d8876834355f3130363835f582d887693132325f3131303532f582d887693132375f3130353739f582d887693130375f3130333236f582d8876838355f3131313330f582d887693130375f3131313336f582d8876836345f3130313331f582d887693136375f3130383839f582d8876835385f3131313135f582d887693134335f3130313938f582d8876838365f3131303731f582d8876838395f3130383036f582d8876835365f3130323430f582d887693131315f3130303838f582d887693131385f3130333533f582d887693130305f3130313535f582d887693136325f3130313038f582d8876835355f3131313231f582d887693132355f3130333530f582d887693135315f3130333532f582d887693135315f3130393034f582d887693133365f3130363434f582d887693133335f3131313436f582d8876837355f3130323135f582d8876837305f3131323135f582d887693131375f3130383833f582d8876837315f3130393339f5"}
	// > {"Key":{"KeyParts":[{"Type":0,"Value":"6e0797ac987005f5"},{"Type":1,"Value":""},{"Type":2,"Value":"24000000000bdfa6a9"}]},"Value":"00086e0797ac987005f5000000000bdfa6aa83005b000000000000017082192ee0572cadb88219a4a167d65f308219d02b825d4170821a7c6f318cc5e5821acd67ba52dc26821b5838b4ebe2ce821be2f0b79ab6aa821c99de030e3463821c9bd08cefeae0821da190c945db94821dc8de541e6c68821e1b70ee53dee1821f2d7d7314b6b0821fac4c33fef7e682204b2447fd153e822194ce7bb7b1988221f27f174db8098222335707ffe5c08222f5b9f8c993a482231d0b3e2239898223a2476c60be088223c67cabdb0add822454c8b881ccf682252c2ac8df207f8225484052ae5cd08225acea0af425638225d535901478ca82266ac62aa14e3c822684ce548cc9238227410f4049d3e2822754a7545da7e682278078deb5b5ba8227a4f29f331c4d8227d711faca5bf582280fa184c6a49b8228907fe12247458228994659c09e038228c7c2dd27053682293a523f625ecd8229664a92b952c28229794969dd711d822baf35ebf98cb8822d2e057b1de881822d736fabd7198b822db1484377230d822dede10f837f789b000000000000002e82d8876834345f3130343235f582d8876839325f3130343934f582d887693133385f3130313333f582d8876838325f3131303133f582d8876836305f3130313935f582d887693131355f3130303932f582d887693131335f3130343033f582d8876838345f3130353831f582d887693133395f3130313539f582d8876835355f3130363831f582d8876834315f3130313539f582d8876834365f3130343035f582d887693131385f3130363833f582d887693130315f3130393135f582d887693131325f3130373835f582d887693133365f3130313439f582d887693135355f3130383533f582d887693137305f3130393539f582d887693132365f3130373436f582d8876834345f3130363835f582d887693135375f3130323834f582d8876838335f3130313633f582d8876836335f3131303430f582d8876836385f3130373537f582d8876839395f3130373033f582d887693132395f3130393531f582d887693136345f3130393735f582d8876838365f3130303835f582d887693134345f3131303236f582d887693135375f3130333732f582d887693130385f3130323935f582d8876838385f3131313733f582d887693133325f3130353137f582d887693136355f3130303637f582d887693132305f3130313230f582d8876839395f3130363231f582d887693133345f3131313835f582d8876839385f3130373833f582d887693135345f3131313632f582d8876837345f3130313137f582d887693134345f3130363532f582d8876839325f3130343831f582d8876835355f3130383136f582d887693134375f3130353735f582d8876836355f3131313539f582d887693132375f3131303131f5"}
	// > {"Key":{"KeyParts":[{"Type":0,"Value":"6e0797ac987005f5"},{"Type":1,"Value":""},{"Type":2,"Value":"24000000000bdfa77f"}]},"Value":"00086e0797ac987005f5000000000bdfa78083005b000000000000017094d353a35efcd7be94d3acba43a2c64194d3c11161e3d03994d3f495f93bd3e494d57defaf51d38294d58c5b102b42f894d5a9fa0d47c37a94d614818f56d3da94d676d1ed1a03e794d76bbe5a92f8cd94d76e885accd60294d7d3b1674a380794d8227504ae562a94d8a761de44e93994d985b2692d545d94da8411316b390294da934920c0774194da9bdca899bb9094dac8fd0ccb6c1594dacb27dbc1392694db01e7d0aa4cef94db0cba00ebaa4c94dc303d3d44db4d94dc659c64ab966194dd5d5bda1d677694dd7ee0c5a2df0994dddf46f0b4a6a194de5480dcfb35f594de6cd58a9ddb6794de70e96f05fe1d94df533d8d5f512294df88f321027f2d94dfa881a51ec46f94dfac83871e7e2c94e043fecad6870a94e0aef4e97b559a94e0e0a4441e413f94e0fd002b03385b94e101e11614f8da94e16170a407effa94e17a4bd2ce9c8694e1a2066186305d94e1f000952d78e594e2cf7c5bd462ca94e2f017a63c547c94e334678273330c9b000000000000002e82d8876836305f3130363034f582d887693135395f3130343231f582d8876836335f3130383439f582d887693133365f3130383239f582d887693135345f3131313830f582d887693136335f3130383838f582d887693133365f3130323434f582d8876837335f3130373435f582d8876836305f3130333335f582d887693133315f3130393033f582d887693133335f3130303932f582d887693130305f3130353530f582d8876836365f3130333131f582d887693134385f3131313037f582d887693136385f3130323738f582d887693133325f3130393636f582d8876838315f3130393136f582d887693135375f3130393137f582d8876836335f3130353137f582d887693131375f3130393930f582d8876839325f3131303832f582d8876837395f3130353236f582d887693136345f3130363637f582d8876838385f3131313437f582d887693132345f3131313839f582d887693135305f3131303530f582d887693132315f3130333334f582d8876837365f3130363738f582d887693135345f3130353131f582d887693130395f3130363532f582d887693136365f3130373135f582d887693134325f3131313233f582d887693132385f3130333434f582d8876834345f3130393533f582d887693130355f3130333435f582d887693133345f3130393134f582d8876838355f3130393037f582d8876836315f3130353030f582d8876839365f3130363935f582d887693134335f3130363633f582d8876837315f3130313438f582d8876839365f3131303331f582d887693134305f3131313238f582d8876834355f3130373230f582d887693133305f3130363930f582d887693134325f3130393638f5"}
	// > {"Key":{"KeyParts":[{"Type":0,"Value":"6e0797ac987005f5"},{"Type":1,"Value":""},{"Type":2,"Value":"24000000000bdfa3b0"}]},"Value":"00086e0797ac987005f5000000000bdfa3b183005b00000000000001703fa773ec40d8fc803fa7a3bdfcb45c263fa7bd9d512d41f93fa9b99cb682cefb3fa9e9b192ef03c73fa9fc01afbf3a553faa43ea8917c3e83faa9a4818c952b23faabaf71e6fdd2c3fab36f4548cf2773fab7623a7d210183fac2f59feba493d3facdefda8385d373facf2ed566a63203fad1e8ef51ff50a3fad91837b0eef003faf600bc63972f83fb044d437b87df93fb06f17a2e97f383fb14ad274b220113fb23529da2946833fb34404c1b2e0a13fb43282cc6b8a633fb4a544dd30d3fa3fb4c5ffea6990993fb4ed3b8e53b1903fb53ac5219f407d3fb557e55f30c82c3fb5692ce247298c3fb6206bc2e04ce23fb6f926b421259b3fb78b4f96160c4c3fb7bac531acb11c3fb7ce5e202ded453fb817995953e5e23fb821a5ad30ddee3fb833cb3d8942ec3fb913430c51adf93fb9a429ed2fdb763fb9c0867288f6693fb9e611ea951ffa3fba94e319e036993fbbb5027a5aec0f3fbbbe408a1ea1703fbc594bcfed09993fbd34acb3321eb49b000000000000002e82d887693134395f3130353337f582d8876835355f3130313632f582d887693137305f3130323234f582d887693131345f3130303638f582d887693132305f3130353234f582d8876838345f3130343930f582d8876837335f3131313831f582d887693134385f3130353531f582d8876838325f3130333635f582d887693136365f3131323339f582d887693136385f3130353236f582d887693136385f3130323038f582d8876839335f3131313139f582d887693130355f3130323038f582d8876836345f3130313231f582d8876835375f3130323538f582d887693132325f3130363737f582d887693135365f3131313432f582d887693135345f3131313136f582d887693131385f3130373131f582d887693131385f3130393933f582d887693136335f3130333637f582d887693133345f3130313437f582d8876838315f3130393835f582d8876839375f3130393135f582d8876839325f3130323638f582d887693130305f3130343536f582d8876836385f3130363937f582d887693130385f3130323437f582d8876839375f3130333439f582d887693135315f3130393535f582d887693131335f3130313630f582d8876836335f3130343438f582d8876834365f3130333534f582d887693133355f3131313731f582d8876838315f3131303432f582d887693131385f3130313037f582d887693136365f3130313236f582d887693132385f3130353539f582d8876838375f3130373837f582d887693134325f3130323030f582d887693131395f3130323433f582d8876834355f3130313634f582d887693135385f3130383338f582d8876838375f3131303233f582d8876838375f3130373533f5"}
	// > {"Key":{"KeyParts":[{"Type":0,"Value":"6e0797ac987005f5"},{"Type":1,"Value":""},{"Type":2,"Value":"24000000000bdfa1ee"}]},"Value":"00086e0797ac987005f5000000000bdfa1ef83005b0000000000000170184cba8f5c6732b2184d940d0385d355184e28109231f55e184eb6c720efb31b184fc87f62d66e45184fd137bce94242185016333430754218510eb561e2ecbb18513e76ebe6b3561851697f8ee39f4b18517553737bf3cb185206306e1e8aa91853265c357a720b1853d73dafd5274518545872a67349c41854af007abaf9ef18551697fd6cb0721855dfd428014fea18566c4153471fc91856ed11d2a83a25185701e3b6bf6f7b18574af74b0b78bf185927fc4104be1e18592f822352884218595c11d6d6d4c6185a1fa7104e3efb185a58f8fd6cec6d185abf75221b7f31185bdc253f401375185dfb2fc773a1bf185dfd4d5e6f1aca185ed084d5e3b1f2185f18e3d9fb8e88185f3ad1abac3e471860c1299f26ab281860f3b31d98e2a718627afc22bc00d81862958486fc5dee1862d27a3eb3ceee18636e22ff1df6cf186401ef7ea9ba18186481a176a9bd7b1864cd046953080b1864da72a50fd49218666de275f18f051866bed4868273e99b000000000000002e82d8876839355f3130323138f582d887693132375f3130313430f582d887693130375f3130323331f582d887693134325f3130353337f582d8876836345f3130393434f582d887693133345f3130343431f582d8876839325f3130363136f582d887693134375f3130353132f582d887693136345f3130373636f582d8876835395f3130353036f582d8876838315f3131313334f582d8876838305f3130363336f582d8876837395f3130333130f582d887693135395f3130323736f582d8876835385f3130323736f582d887693132305f3130353835f582d8876839355f3130393837f582d887693131335f3130313836f582d8876839305f3130333832f582d887693134365f3131313639f582d887693136395f3130363235f582d887693132325f3130383039f582d8876835355f3130353536f582d887693136355f3130343333f582d887693130335f3130373033f582d887693131365f3130393334f582d8876837335f3130363132f582d8876837325f3130353036f582d8876834355f3130343733f582d8876838345f3130323536f582d8876837325f3130373235f582d887693134375f3130303836f582d887693134315f3130363938f582d887693132325f3131303034f582d8876838385f3130313034f582d887693136325f3131313439f582d887693134355f3130363833f582d8876839395f3130353536f582d8876837395f3130313031f582d8876839305f3130303935f582d887693135305f3130353430f582d887693133315f3130353037f582d887693130375f3130353931f582d887693131345f3131313333f582d8876834315f3130373838f582d887693132315f3130383333f5"}
	// > {"Key":{"KeyParts":[{"Type":0,"Value":"6e0797ac987005f5"},{"Type":1,"Value":""},{"Type":2,"Value":"24000000000bdfaa22"}]},"Value":"00086e0797ac987005f5000000000bdfaa2383005b0000000000000170cf97f5bf9ef7a298cf983121a165ecdccf98449a0eb3a14ccf98e243bfaebf05cf999909ab93dc3dcf9aa343607286e8cf9b7837f4cd8f61cf9bb1cc3f25dec7cf9bcf4a1df9b204cf9cd2016d99b963cf9cd7ca968bf919cf9d1a7399807399cf9d55953020f3a3cf9dccd016a31b7acf9e65c374637939cf9e80c0cb6485d6cf9e9fb1b5ff5f29cf9f1940af1a0c06cf9f557be8113589cf9fd7cd2ffcd70acfa08780dfcdd3cbcfa0bfa0919eb10bcfa218b8e0abbea4cfa2219e67c73366cfa22babf6ff7b17cfa2459434ab0283cfa2bf97e67d73a7cfa2d02cb790faa4cfa3258413f5dd99cfa384009052d72ecfa392030179a780cfa3bde3accefdd6cfa5f28b12b91b21cfa6a8aa64ffdee1cfa790adacb09e6dcfa7fc8b027634a7cfa855ccdf454699cfaa253c10b7abafcfaa6a588b7f4caccfab56f6284e5dfdcfabcc245e2fadf2cfabd47b4fbf2971cfac999ee1cc853ecfad80270249af2acfae0a5b6884ed8fcfae553d0607d8d09b000000000000002e82d887693136375f3130373836f582d887693130355f3130303932f582d887693130375f3131303937f582d8876839305f3130393831f582d8876837385f3130393639f582d8876837375f3130313238f582d887693131335f3130363330f582d8876838365f3131313630f582d887693130385f3130393936f582d8876835365f3130323438f582d887693136335f3130353334f582d887693135305f3130313631f582d887693131375f3130353634f582d8876836395f3131313631f582d887693132375f3130343034f582d887693130355f3130393432f582d887693136355f3130363233f582d8876834345f3131323035f582d887693133345f3130373131f582d887693134365f3130313535f582d8876838365f3130333839f582d887693132395f3130373538f582d887693133335f3130353132f582d8876838365f3130343635f582d8876837375f3130353138f582d887693135375f3130373131f582d887693131315f3130343131f582d8876835395f3130393135f582d8876836315f3130363934f582d8876838335f3130343534f582d887693132315f3130393330f582d887693132345f3130363230f582d8876835355f3130323630f582d8876838365f3130303839f582d8876836305f3130333836f582d887693131325f3130333138f582d887693134315f3130363130f582d8876836365f3131303935f582d887693134345f3130313733f582d887693134375f3130353736f582d887693134325f3130343833f582d8876838385f3130313338f582d887693131385f3130383039f582d8876837355f3131303832f582d8876835375f3130313639f582d887693133345f3130343039f5"}

	w, err := wal.NewDiskWAL(zerolog.Nop(), nil, metrics.NewNoopCollector(), dir, 100, pathfinder.PathByteSize, wal.SegmentSize)
	require.NoError(t, err)

	led, err := complete.NewLedger(w, 100, &metrics.NoopCollector{}, zerolog.Logger{}, complete.DefaultPathFinderVersion)
	require.NoError(t, err)

	encodedUpdate, err := ioutil.ReadFile(dir + "/update.bin")
	if err != nil {
		log.Fatal(err)
	}

	_, _, update, err := wal.Decode(encodedUpdate)
	require.NoError(t, err)

	// a sample register that has to be set to zero
	expectedOwner, err := hex.DecodeString("6e0797ac987005f5")
	require.NoError(t, err)
	expectedKey, err := hex.DecodeString("24000000000bdfaa22")
	require.NoError(t, err)
	found := false
	for _, p := range update.Payloads {
		if bytes.Equal(p.Key.KeyParts[0].Value, expectedOwner) &&
			bytes.Equal(p.Key.KeyParts[2].Value, expectedKey) {
			require.Equal(t, 0, len(p.Value))
			found = true
		}
		// fmt.Println("path: ", update.Paths[i])
	}
	require.True(t, found)

	newRoot, err := led.Forest().Update(update)
	require.NoError(t, err)
	fmt.Println("state after update:", newRoot.String())

	keyParts := make([]ledger.KeyPart, 0)
	keyParts = append(keyParts, ledger.NewKeyPart(0, expectedOwner))
	keyParts = append(keyParts, ledger.NewKeyPart(1, []byte("")))
	keyParts = append(keyParts, ledger.NewKeyPart(2, expectedKey))
	lookupKey := []ledger.Key{{KeyParts: keyParts}}
	q, err := ledger.NewQuery(ledger.State(newRoot), lookupKey)
	require.NoError(t, err)

	v, err := led.Get(q)
	require.NoError(t, err)
	require.Equal(t, 0, len(v[0]))
}
