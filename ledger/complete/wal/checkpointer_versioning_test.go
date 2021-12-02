package wal

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger/complete/mtrie/flattener"
)

var v1Forest = &flattener.FlattenedForest{
	Nodes: []*flattener.StorableNode{
		nil, // node 0 is special and skipped
		{
			LIndex:     0,
			RIndex:     0,
			Height:     0,
			Path:       []byte{1},
			EncPayload: []byte{2},
			HashValue:  []byte{3},
			MaxDepth:   1,
			RegCount:   1,
		}, {
			LIndex:     1,
			RIndex:     2,
			Height:     3,
			Path:       []byte{11},
			EncPayload: []byte{22},
			HashValue:  []byte{33},
			MaxDepth:   11,
			RegCount:   11,
		},
	},
	Tries: []*flattener.StorableTrie{
		{
			RootIndex: 0,
			RootHash:  []byte{4},
		},
		{
			RootIndex: 1,
			RootHash:  []byte{44},
		},
	},
}

func Test_LoadingV1Checkpoint(t *testing.T) {

	forest, err := LoadCheckpoint("test_data/checkpoint.v1")
	require.NoError(t, err)

	require.Equal(t, v1Forest, forest)
}
