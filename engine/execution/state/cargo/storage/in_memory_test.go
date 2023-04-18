package storage_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution/state/cargo/storage"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// TODO add tests for handling forks

func TestInMemStorage(t *testing.T) {

	t.Run("commit checks", func(t *testing.T) {
		var err error
		capacity := 10
		headers := unittest.BlockHeaderFixtures(13)
		genesis, batch1, _, _ := headers[0], headers[1:5], headers[5:9], headers[9:]

		store := storage.NewInMemoryStorage(capacity, genesis)

		// add the first batch of headers
		for _, header := range batch1 {
			update := map[flow.RegisterID]flow.RegisterValue{
				{Key: "key", Owner: "owner"}: []byte("value"),
			}
			err = store.CommitBlock(header, update)
			require.NoError(t, err)
		}
	})
}
