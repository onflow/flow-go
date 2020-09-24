package ledger_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage/ledger"
	"github.com/onflow/flow-go/storage/ledger/utils"
	"github.com/onflow/flow-go/utils/unittest"
)

func Test_WAL(t *testing.T) {

	numInsPerStep := 2
	keyByteSize := 32
	valueMaxByteSize := 2 << 16
	size := 10
	metricsCollector := &metrics.NoopCollector{}

	unittest.RunWithTempDir(t, func(dir string) {

		// cache size intentionally is set to size to test deletion
		f, err := ledger.NewMTrieStorage(dir, size, metricsCollector, nil)
		require.NoError(t, err)

		var stateCommitment = f.EmptyStateCommitment()

		//saved data after updates
		savedData := make(map[string]map[string][]byte)

		for i := 0; i < size; i++ {
			keys := utils.GetRandomKeysFixedN(numInsPerStep, keyByteSize)
			values := utils.GetRandomValues(len(keys), 0, valueMaxByteSize)

			stateCommitment, err = f.UpdateRegisters(keys, values, stateCommitment)
			require.NoError(t, err)

			fmt.Printf("Updated with %x\n", stateCommitment)

			data := make(map[string][]byte, len(keys))
			for j, key := range keys {
				data[string(key)] = values[j]
			}

			savedData[string(stateCommitment)] = data
		}

		<-f.Done()

		f2, err := ledger.NewMTrieStorage(dir, size+10, metricsCollector, nil)
		require.NoError(t, err)

		// random map iteration order is a benefit here
		for stateCommitment, data := range savedData {

			keys := make([][]byte, 0, len(data))
			for keyString := range data {
				key := []byte(keyString)
				keys = append(keys, key)
			}

			fmt.Printf("Querying with %x\n", stateCommitment)

			registerValues, err := f2.GetRegisters(keys, []byte(stateCommitment))
			require.NoError(t, err)

			for i, key := range keys {
				registerValue := registerValues[i]

				assert.Equal(t, data[string(key)], registerValue)
			}
		}

		// test deletion
		s := f2.ForestSize()
		assert.Equal(t, s, size)

		<-f2.Done()
	})
}
