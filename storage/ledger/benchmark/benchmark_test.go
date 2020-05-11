package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/prometheus/tsdb/wal"
	"github.com/stretchr/testify/require"
	"gotest.tools/assert"

	"github.com/dapperlabs/flow-go/model/flow"
	mtrieWAL "github.com/dapperlabs/flow-go/storage/ledger/mtrie/wal"
	"github.com/dapperlabs/flow-go/storage/ledger/utils"

	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestBenchamark(t *testing.T) {
	StorageBenchmark()
}

type updateValues struct {
	StateCommitment flow.StateCommitment
	Keys            [][]byte
	Values          [][]byte
}

func TestWAL(t *testing.T) {

	numInsPerStep := 2
	keyByteSize := 16
	valueMaxByteSize := 2 << 16
	size := 10

	unittest.RunWithTempDir(t, func(dir string) {

		start := time.Now()

		w, err := wal.New(nil, nil, dir)

		updates := make([]updateValues, size)

		for i := 0; i < size; i++ {
			keys := utils.GetRandomKeysFixedN(numInsPerStep, keyByteSize)
			values := utils.GetRandomValues(len(keys), valueMaxByteSize)
			stateCommitment := unittest.StateCommitmentFixture()

			updates[i].Keys = keys
			updates[i].Values = values
			updates[i].StateCommitment = stateCommitment

			data := mtrieWAL.EncodeUpdate(stateCommitment, keys, values)

			err = w.Log(data)
			require.NoError(t, err)
		}

		w.Close()

		elapsed := time.Since(start)
		fmt.Printf("Writing - %s\n", elapsed)

		start = time.Now()

		sr, err := wal.NewSegmentsReader(dir)
		require.NoError(t, err)

		reader := wal.NewReader(sr)

		retrievedUpdates := make([]updateValues, size)
		i := 0

		for reader.Next() {
			record := reader.Record()
			operation, commitment, keys, values, err := mtrieWAL.Decode(record)
			require.NoError(t, err)
			assert.Equal(t, mtrieWAL.WALUpdate, operation)

			retrievedUpdates[i].Keys = keys
			retrievedUpdates[i].Values = values
			retrievedUpdates[i].StateCommitment = commitment

			i++
		}

		elapsed = time.Since(start)
		fmt.Printf("Reading - %s\n", elapsed)

		start = time.Now()

		assert.Equal(t, size, i)
		assert.DeepEqual(t, updates, retrievedUpdates)

		elapsed = time.Since(start)
		fmt.Printf("Comparing - %s", elapsed)
	})
}
