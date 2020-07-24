package complete_test

import (
	"bytes"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/ledger"
	"github.com/dapperlabs/flow-go/ledger/common"
	"github.com/dapperlabs/flow-go/ledger/common/encoding"
	"github.com/dapperlabs/flow-go/ledger/common/utils"
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
		stateComSize := 32
		valueMaxByteSize := 2 << 16 //16kB
		activeTries := 1000
		steps := 40                                  // number of steps
		histStorage := make(map[string]ledger.Value) // historic storage string(key, state) -> value
		latestValue := make(map[string]ledger.Value) // key to value
		unittest.RunWithTempDir(t, func(dbDir string) {
			led, err := complete.NewLedger(dbDir, activeTries, metricsCollector, nil)
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
