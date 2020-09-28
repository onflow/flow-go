package complete_test

import (
	"bytes"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common"
	"github.com/onflow/flow-go/ledger/common/encoding"
	"github.com/onflow/flow-go/ledger/common/utils"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/partial/ptrie"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestNewLedger(t *testing.T) {
	unittest.RunWithTempDir(t, func(dbDir string) {
		metricsCollector := &metrics.NoopCollector{}
		_, err := complete.NewLedger(dbDir, 100, metricsCollector, zerolog.Logger{}, nil)
		assert.NoError(t, err)
	})
}

func TestLedger_Update(t *testing.T) {
	t.Run("empty update", func(t *testing.T) {
		unittest.RunWithTempDir(t, func(dbDir string) {

			l, err := complete.NewLedger(dbDir, 100, &metrics.NoopCollector{}, zerolog.Logger{}, nil)
			require.NoError(t, err)

			// create empty update
			currentState := l.InitState()
			up, err := ledger.NewEmptyUpdate(currentState)
			require.NoError(t, err)

			newState, err := l.Set(up)
			require.NoError(t, err)

			// state shouldn't change
			assert.True(t, bytes.Equal(currentState, newState))
		})
	})

	t.Run("non-empty update and query", func(t *testing.T) {
		unittest.RunWithTempDir(t, func(dbDir string) {
			// UpdateFixture

			led, err := complete.NewLedger(dbDir, 100, &metrics.NoopCollector{}, zerolog.Logger{}, nil)
			require.NoError(t, err)

			curSC := led.InitState()

			u := utils.UpdateFixture()
			u.SetState(curSC)

			newSc, err := led.Set(u)
			require.NoError(t, err)
			assert.False(t, bytes.Equal(curSC, newSc))

			q, err := ledger.NewQuery(newSc, u.Keys())
			require.NoError(t, err)

			retValues, err := led.Get(q)
			require.NoError(t, err)

			for i, v := range u.Values() {
				assert.Equal(t, v, retValues[i])
			}
		})
	})
}

func TestLedger_Get(t *testing.T) {
	t.Run("empty query", func(t *testing.T) {
		unittest.RunWithTempDir(t, func(dbDir string) {
			led, err := complete.NewLedger(dbDir, 100, &metrics.NoopCollector{}, zerolog.Logger{}, nil)
			require.NoError(t, err)

			curSC := led.InitState()
			q, err := ledger.NewEmptyQuery(curSC)
			require.NoError(t, err)

			retValues, err := led.Get(q)
			require.NoError(t, err)
			assert.Equal(t, len(retValues), 0)
		})
	})

	t.Run("empty keys", func(t *testing.T) {
		unittest.RunWithTempDir(t, func(dbDir string) {
			led, err := complete.NewLedger(dbDir, 100, &metrics.NoopCollector{}, zerolog.Logger{}, nil)
			require.NoError(t, err)

			curS := led.InitState()

			q := utils.QueryFixture()
			q.SetState(curS)

			retValues, err := led.Get(q)
			require.NoError(t, err)

			assert.Equal(t, 2, len(retValues))
			assert.Equal(t, 0, len(retValues[0]))
			assert.Equal(t, 0, len(retValues[1]))
		})
	})
}

func TestLedger_Proof(t *testing.T) {
	t.Run("empty query", func(t *testing.T) {
		unittest.RunWithTempDir(t, func(dbDir string) {
			led, err := complete.NewLedger(dbDir, 100, &metrics.NoopCollector{}, zerolog.Logger{}, nil)
			require.NoError(t, err)

			curSC := led.InitState()
			q, err := ledger.NewEmptyQuery(curSC)
			require.NoError(t, err)

			retProof, err := led.Prove(q)
			require.NoError(t, err)

			proof, err := encoding.DecodeTrieBatchProof(retProof)
			require.NoError(t, err)
			assert.Equal(t, 0, len(proof.Proofs))
		})
	})

	t.Run("non-existing keys", func(t *testing.T) {
		unittest.RunWithTempDir(t, func(dbDir string) {
			led, err := complete.NewLedger(dbDir, 100, &metrics.NoopCollector{}, zerolog.Logger{}, nil)
			require.NoError(t, err)

			curS := led.InitState()
			q := utils.QueryFixture()
			q.SetState(curS)
			require.NoError(t, err)

			retProof, err := led.Prove(q)
			require.NoError(t, err)

			proof, err := encoding.DecodeTrieBatchProof(retProof)
			require.NoError(t, err)
			assert.Equal(t, 2, len(proof.Proofs))
			assert.True(t, common.VerifyTrieBatchProof(proof, curS))
		})
	})

	t.Run("existing keys", func(t *testing.T) {
		unittest.RunWithTempDir(t, func(dbDir string) {
			led, err := complete.NewLedger(dbDir, 100, &metrics.NoopCollector{}, zerolog.Logger{}, nil)
			require.NoError(t, err)

			curS := led.InitState()

			u := utils.UpdateFixture()
			u.SetState(curS)

			newSc, err := led.Set(u)
			require.NoError(t, err)
			assert.False(t, bytes.Equal(curS, newSc))

			q, err := ledger.NewQuery(newSc, u.Keys())
			require.NoError(t, err)

			retProof, err := led.Prove(q)
			require.NoError(t, err)

			proof, err := encoding.DecodeTrieBatchProof(retProof)
			require.NoError(t, err)
			assert.Equal(t, 2, len(proof.Proofs))
			assert.True(t, common.VerifyTrieBatchProof(proof, newSc))
		})
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

		// cache size intentionally is set to size to test deletion
		led, err := complete.NewLedger(dir, size, metricsCollector, logger, nil)
		require.NoError(t, err)

		var state = led.InitState()

		//saved data after updates
		savedData := make(map[string]map[string]ledger.Value)

		for i := 0; i < size; i++ {

			keys := utils.RandomUniqueKeys(numInsPerStep, keyNumberOfParts, keyPartMinByteSize, keyPartMaxByteSize)
			values := utils.RandomValues(numInsPerStep, 1, valueMaxByteSize)
			update, err := ledger.NewUpdate(state, keys, values)
			assert.NoError(t, err)
			state, err = led.Set(update)
			require.NoError(t, err)
			fmt.Printf("Updated with %x\n", state)

			data := make(map[string]ledger.Value, len(keys))
			for j, key := range keys {
				encKey := encoding.EncodeKey(&key)
				data[string(encKey)] = values[j]
			}

			savedData[string(state)] = data
		}

		<-led.Done()

		led2, err := complete.NewLedger(dir, size+10, metricsCollector, logger, nil)
		require.NoError(t, err)

		// random map iteration order is a benefit here
		for state, data := range savedData {

			keys := make([]ledger.Key, 0, len(data))
			for encKey := range data {
				key, err := encoding.DecodeKey([]byte(encKey))
				assert.NoError(t, err)
				keys = append(keys, *key)
			}

			query, err := ledger.NewQuery(ledger.State(state), keys)
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
		pathByteSize := 32
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
			led, err := complete.NewLedger(dbDir, activeTries, metricsCollector, logger, nil)
			assert.NoError(t, err)
			state := led.InitState()
			for i := 0; i < steps; i++ {
				// add new keys
				// TODO update some of the existing keys and shuffle them
				keys := utils.RandomUniqueKeys(numInsPerStep, keyNumberOfParts, keyPartMinByteSize, keyPartMaxByteSize)
				values := utils.RandomValues(numInsPerStep, 1, valueMaxByteSize)
				update, err := ledger.NewUpdate(state, keys, values)
				assert.NoError(t, err)
				newState, err := led.Set(update)
				assert.NoError(t, err)

				// capture new values for future query
				for j, k := range keys {
					encKey := encoding.EncodeKey(&k)
					histStorage[string(newState)+string(encKey)] = values[j]
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
				proof, err := led.Prove(query)
				assert.NoError(t, err)

				bProof, err := encoding.DecodeTrieBatchProof(proof)
				assert.NoError(t, err)

				// validate batch proofs
				isValid := common.VerifyTrieBatchProof(bProof, newState)
				assert.True(t, isValid)

				// validate proofs as a batch
				_, err = ptrie.NewPSMT(newState, pathByteSize, bProof)
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
					state := []byte(s[:stateComSize])
					enk := []byte(s[stateComSize:])
					key, err := encoding.DecodeKey([]byte(enk))
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
		})
	}
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
