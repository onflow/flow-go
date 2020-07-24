package complete_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/ledger"
	"github.com/dapperlabs/flow-go/ledger/complete"
	"github.com/dapperlabs/flow-go/ledger/common/encoding"
	"github.com/dapperlabs/flow-go/ledger/common/utils"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func Test_WAL(t *testing.T) {
	numInsPerStep := 2
	keyNumberOfParts := 10
	keyPartMinByteSize := 1
	keyPartMaxByteSize := 100
	valueMaxByteSize := 2 << 16 //16kB
	size := 10
	metricsCollector := &metrics.NoopCollector{}

	unittest.RunWithTempDir(t, func(dir string) {

		// cache size intentionally is set to size to test deletion
		led, err := complete.NewLedger(dir, size, metricsCollector, nil)
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

		led2, err := complete.NewLedger(dir, size+10, metricsCollector, nil)
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
