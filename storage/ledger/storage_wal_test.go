package ledger_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/storage/ledger"
	"github.com/dapperlabs/flow-go/storage/ledger/utils"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func Test_WAL(t *testing.T) {

	numInsPerStep := 2
	keyByteSize := 32
	valueMaxByteSize := 2 << 16
	size := 10

	unittest.RunWithTempDir(t, func(dir string) {

		f, err := ledger.NewTrieStorage(dir)
		require.NoError(t, err)

		var stateCommitment = f.EmptyStateCommitment()

		//saved data after updates
		savedData := make(map[string]map[string][]byte, 0)

		for i := 0; i < size; i++ {
			keys := utils.GetRandomKeysFixedN(numInsPerStep, keyByteSize)
			values := utils.GetRandomValues(len(keys), valueMaxByteSize)

			stateCommitment, err = f.UpdateRegisters(keys, values, stateCommitment)
			require.NoError(t, err)

			data := make(map[string][]byte, len(keys))
			for j, key := range keys {
				data[string(key)] = values[j]
			}

			savedData[string(stateCommitment)] = data
		}

		<-f.Done()

		f2, err := ledger.NewTrieStorage(dir)
		require.NoError(t, err)

		// random map iteration order is a benefit here
		for stateCommitment, data := range savedData {

			keys := make([][]byte, 0, len(data))
			for keyString := range data {
				key := []byte(keyString)
				keys = append(keys, key)
			}
			registerValues, err := f2.GetRegisters(keys, []byte(stateCommitment))
			require.NoError(t, err)

			for i, key := range keys {
				registerValue := registerValues[i]

				assert.Equal(t, data[string(key)], registerValue)
			}
		}

		<-f2.Done()
	})
}
