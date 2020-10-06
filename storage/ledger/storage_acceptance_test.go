package ledger_test

import (
	"bytes"
	"math/rand"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/ledger/common/encoding"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage/ledger/utils"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common"
	completeLedger "github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/partial"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/unittest"
)

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

type kv struct {
	key   ledger.Key
	value ledger.Value
}

type skv struct {
	key   ledger.Key
	value ledger.Value
	sc    flow.StateCommitment
}

func TestLedgerFunctionality(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	// You can manually increase this for more coverage
	experimentRep := 2

	metricsCollector := &metrics.NoopCollector{}

	for e := 0; e < experimentRep; e++ {
		maxNumInsPerStep := 100
		numHistLookupPerStep := 10
		//keyByteSize := 32
		valueMaxByteSize := 64
		activeTries := 1000
		steps := 40                         // number of steps
		histStorage := make(map[string]skv) // historic storage string(key, statecommitment) -> skv
		latestValue := make(map[string]kv)  // key to value
		unittest.RunWithTempDir(t, func(dbDir string) {
			led, err := completeLedger.NewLedger(dbDir, activeTries, metricsCollector, zerolog.Nop(), nil)
			assert.NoError(t, err)
			stateCommitment := led.InitialState()
			for i := 0; i < steps; i++ {
				// add new keys
				// TODO update some of the existing keys and shuffle them
				registerIDs := utils.GetRandomRegisterIDs(maxNumInsPerStep)
				values := utils.GetRandomValues(len(registerIDs), 0, valueMaxByteSize)

				keys := state.RegisterIDSToKeys(registerIDs)

				update, err := ledger.NewUpdate(stateCommitment, keys, values)
				require.NoError(t, err)

				newState, err := led.Set(update)
				assert.NoError(t, err)

				// capture new values for future query
				for j, k := range keys {
					histStorage[k.String()+string(newState)] = skv{
						key:   k,
						value: values[j],
						sc:    newState,
					}

					latestValue[k.String()] = kv{
						key:   k,
						value: values[j],
					}
				}

				// TODO set some to nil

				query, err := ledger.NewQuery(newState, keys)
				require.NoError(t, err)

				// read values and compare values
				retValues, err := led.Get(query)
				assert.NoError(t, err)
				// byte{} is returned as nil
				assert.True(t, valuesMatches(values, retValues))

				// validate proofs (check individual proof and batch proof)
				//retValues, proofs, err := led.GetRegistersWithProof(keys, newState)
				proof, err := led.Prove(query)
				assert.NoError(t, err)

				batchProof, err := encoding.DecodeTrieBatchProof(proof)
				require.NoError(t, err)

				// validate individual proofs
				isValid := common.VerifyTrieBatchProof(batchProof, newState)
				assert.NoError(t, err)
				assert.True(t, isValid)

				// validate proofs as a batch
				_, err = partial.NewLedger(proof, newState)
				require.NoError(t, err)

				// query all exising keys (check no drop)
				for _, v := range latestValue {

					query, err := ledger.NewQuery(newState, []ledger.Key{v.key})
					require.NoError(t, err)

					rv, err := led.Get(query)
					assert.NoError(t, err)

					assert.True(t, valuesMatches([]ledger.Value{v.value}, rv))
				}

				// query some of historic values (map return is random)
				j := 0
				for _, v := range histStorage {
					//value := histStorage[s]
					//key := []byte(s[:keyByteSize])
					//state := []byte(s[keyByteSize:])

					query, err := ledger.NewQuery(v.sc, []ledger.Key{v.key})
					require.NoError(t, err)

					rv, err := led.Get(query)

					//
					//rv, err := led.GetRegisters([][]byte{key}, state)
					assert.NoError(t, err)
					assert.True(t, valuesMatches([]ledger.Value{v.value}, rv))
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
