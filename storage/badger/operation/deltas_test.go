// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestDeltaInsertRetrieve(t *testing.T) {

	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		number := uint64(9)
		id := flow.Identity{
			NodeID:  flow.Identifier{0x01},
			Role:    flow.Role(2),
			Address: "a",
			Stake:   3,
		}

		err := db.Update(InsertDelta(number, id.Role, id.NodeID, int64(id.Stake)))
		require.Nil(t, err)

		var delta int64
		err = db.View(RetrieveDelta(number, id.Role, id.NodeID, &delta))
		require.Nil(t, err)

		assert.Equal(t, int64(id.Stake), delta)
	})
}

func TestDeltasTraverse(t *testing.T) {

	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		ids := flow.IdentityList{
			{NodeID: flow.Identifier{0x01}, Role: flow.Role(1), Address: "a1"},
			{NodeID: flow.Identifier{0x02}, Role: flow.Role(2), Address: "a2"},
			{NodeID: flow.Identifier{0x03}, Role: flow.Role(3), Address: "a3"},
			{NodeID: flow.Identifier{0x04}, Role: flow.Role(4), Address: "a4"},
		}

		expected := map[flow.Identifier]int64{
			ids[0].NodeID: 300,
			ids[1].NodeID: 500,
			ids[2].NodeID: 200,
			ids[3].NodeID: 700,
		}

		vectors := []struct {
			Number uint64
			Deltas map[int]int64
		}{
			{
				Number: 0,
				Deltas: map[int]int64{
					0: 100,
					1: 500,
					2: 800,
				},
			},
			{
				Number: 1,
				Deltas: map[int]int64{
					0: 100,
					2: -100,
				},
			},
			{
				Number: 4,
				Deltas: map[int]int64{
					0: 100,
					2: -500,
					3: 700,
				},
			},
		}

		for _, v := range vectors {
			for i, delta := range v.Deltas {
				id := ids[i]
				err := db.Update(InsertDelta(v.Number, id.Role, id.NodeID, delta))
				require.Nil(t, err)
			}
		}

		actual := make(map[flow.Identifier]int64)
		process := func(nodeID flow.Identifier, delta int64) error {
			actual[nodeID] += delta
			return nil
		}

		err := db.View(TraverseDeltas(0, 4, nil, process))
		require.Nil(t, err)

		assert.Equal(t, expected, actual)
	})
}
