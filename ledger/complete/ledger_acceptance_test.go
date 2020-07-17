package complete_test

import (
	"bytes"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/ledger"
	"github.com/dapperlabs/flow-go/ledger/common"
	"github.com/dapperlabs/flow-go/ledger/complete"
	"github.com/dapperlabs/flow-go/ledger/partial/ptrie"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestLedgerFunctionality(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	// You can manually increase this for more coverage
	experimentRep := 2

	metricsCollector := &metrics.NoopCollector{}

	for e := 0; e < experimentRep; e++ {
		numInsPerStep := 100
		numHistLookupPerStep := 10
		pathByteSize := 32
		keyNumberOfParts := 10
		keyPartMinByteSize := 1
		keyPartMaxByteSize := 100
		valueMaxByteSize := 2 << 16 //16kB
		activeTries := 1000
		steps := 40                                  // number of steps
		histStorage := make(map[string]ledger.Value) // historic storage string(key, statecommitment) -> value
		latestValue := make(map[string]ledger.Value) // key to value
		unittest.RunWithTempDir(t, func(dbDir string) {
			led, err := complete.NewLedger(dbDir, activeTries, metricsCollector, nil)
			assert.NoError(t, err)
			stateCommitment := led.EmptyStateCommitment()
			for i := 0; i < steps; i++ {
				// add new keys
				// TODO update some of the existing keys and shuffle them
				keys := common.RandomUniqueKeys(numInsPerStep, keyNumberOfParts, keyPartMinByteSize, keyPartMaxByteSize)
				values := common.RandomValues(numInsPerStep, 1, valueMaxByteSize)
				update, err := ledger.NewUpdate(stateCommitment, keys, values)
				assert.NoError(t, err)
				newState, err := led.Set(update)
				assert.NoError(t, err)

				// capture new values for future query
				for j, k := range keys {
					histStorage[k.String()+string(newState)] = values[j]
					latestValue[k.String()] = values[j]
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
				v := common.Verifiy.NewTrieVerifier(keyByteSize)

				// validate individual proofs
				isValid, err := v.VerifyRegistersProof(keys, retValues, proofs, newState)
				assert.NoError(t, err)
				assert.True(t, isValid)

				// validate proofs as a batch
				_, err = ptrie.NewPSMT(newState, keyByteSize, keys, retValues, proofs)
				assert.NoError(t, err)

				// query all exising keys (check no drop)
				for k, v := range latestValue {
					rv, err := led.GetRegisters([][]byte{[]byte(k)}, newState)
					assert.NoError(t, err)
					assert.True(t, valuesMatches([][]byte{v}, rv))
				}

				// query some of historic values (map return is random)
				j := 0
				for s := range histStorage {
					value := histStorage[s]
					key := []byte(s[:keyByteSize])
					state := []byte(s[keyByteSize:])
					rv, err := led.GetRegisters([][]byte{key}, state)
					assert.NoError(t, err)
					assert.True(t, valuesMatches([][]byte{value}, rv))
					j++
					if j >= numHistLookupPerStep {
						break
					}
				}
				stateCommitment = newState
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
