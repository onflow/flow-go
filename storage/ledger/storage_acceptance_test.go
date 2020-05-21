package ledger_test

import (
	"bytes"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/storage/ledger"
	ptriep "github.com/dapperlabs/flow-go/storage/ledger/ptrie"
	"github.com/dapperlabs/flow-go/storage/ledger/utils"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func valuesMatches(expected [][]byte, got [][]byte) bool {
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

func TestLedgerFunctionality(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	// You can manually increase this for more coverage
	experimentRep := 2
	for e := 0; e < experimentRep; e++ {
		maxNumInsPerStep := 100
		numHistLookupPerStep := 10
		keyByteSize := 32
		valueMaxByteSize := 64
		activeTries := 1000
		trieHeight := keyByteSize*8 + 1        // 257
		steps := 40                            // number of steps
		histStorage := make(map[string][]byte) // historic storage string(key, statecommitment) -> value
		latestValue := make(map[string][]byte) // key to value
		unittest.RunWithTempDir(t, func(dbDir string) {
			led, err := ledger.NewMTrieStorage(dbDir, activeTries, nil)
			assert.NoError(t, err)
			stateCommitment := led.EmptyStateCommitment()
			for i := 0; i < steps; i++ {
				// add new keys
				// TODO update some of the existing keys and shuffle them
				keys := utils.GetRandomKeysRandN(maxNumInsPerStep, keyByteSize)
				values := utils.GetRandomValues(len(keys), valueMaxByteSize)
				newState, err := led.UpdateRegisters(keys, values, stateCommitment)
				assert.NoError(t, err)

				// capture new values for future query
				for j, k := range keys {
					histStorage[string(k)+string(newState)] = values[j]
					latestValue[string(k)] = values[j]
				}

				// TODO set some to nil

				// read values and compare values
				retValues, err := led.GetRegisters(keys, newState)
				assert.NoError(t, err)
				// byte{} is returned as nil
				assert.True(t, valuesMatches(values, retValues))

				// validate proofs (check individual proof and batch proof)
				retValues, proofs, err := led.GetRegistersWithProof(keys, newState)
				assert.NoError(t, err)
				v := ledger.NewTrieVerifier(trieHeight)

				// validate individual proofs
				isValid, err := v.VerifyRegistersProof(keys, retValues, proofs, newState)
				assert.NoError(t, err)
				assert.True(t, isValid)

				// validate proofs as a batch
				_, err = ptriep.NewPSMT(newState, trieHeight, keys, retValues, proofs)
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
