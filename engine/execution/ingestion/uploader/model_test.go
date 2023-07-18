package uploader

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/engine/execution/testutil"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func Test_ComputationResultToBlockDataConversion(t *testing.T) {

	cr, expectedTrieUpdates := generateComputationResult(t)

	blockData := ComputationResultToBlockData(cr)

	assert.Equal(t, cr.ExecutableBlock.Block, blockData.Block)
	assert.Equal(t, cr.ExecutableBlock.Collections(), blockData.Collections)

	allTxResults := cr.AllTransactionResults()
	require.Equal(t, len(allTxResults), len(blockData.TxResults))
	for i, result := range allTxResults {
		assert.Equal(t, result, *blockData.TxResults[i])
	}

	// Since returned events are not preserving orders,
	// use map with event.ID() as key to confirm all events
	// are included.
	allEvents := cr.AllEvents()
	require.Equal(t, len(allEvents), len(blockData.Events))

	eventsInBlockData := make(map[flow.Identifier]flow.Event)
	for _, e := range blockData.Events {
		eventsInBlockData[e.ID()] = *e
	}

	for _, expectedEvent := range allEvents {
		event, ok := eventsInBlockData[expectedEvent.ID()]
		require.True(t, ok)
		require.Equal(t, expectedEvent, event)
	}

	assert.Equal(t, len(expectedTrieUpdates), len(blockData.TrieUpdates))

	assert.Equal(t, cr.CurrentEndState(), blockData.FinalStateCommitment)
}

func generateComputationResult(
	t *testing.T,
) (
	*execution.ComputationResult,
	[]*ledger.TrieUpdate,
) {

	update1, err := ledger.NewUpdate(
		ledger.State(unittest.StateCommitmentFixture()),
		[]ledger.Key{
			ledger.NewKey([]ledger.KeyPart{ledger.NewKeyPart(3, []byte{33})}),
			ledger.NewKey([]ledger.KeyPart{ledger.NewKeyPart(1, []byte{11})}),
			ledger.NewKey([]ledger.KeyPart{ledger.NewKeyPart(2, []byte{1, 1}), ledger.NewKeyPart(3, []byte{2, 5})}),
		},
		[]ledger.Value{
			[]byte{21, 37},
			nil,
			[]byte{3, 3, 3, 3, 3},
		},
	)
	require.NoError(t, err)

	trieUpdate1, err := pathfinder.UpdateToTrieUpdate(update1, complete.DefaultPathFinderVersion)
	require.NoError(t, err)

	update2, err := ledger.NewUpdate(
		ledger.State(unittest.StateCommitmentFixture()),
		[]ledger.Key{},
		[]ledger.Value{},
	)
	require.NoError(t, err)

	trieUpdate2, err := pathfinder.UpdateToTrieUpdate(update2, complete.DefaultPathFinderVersion)
	require.NoError(t, err)

	update3, err := ledger.NewUpdate(
		ledger.State(unittest.StateCommitmentFixture()),
		[]ledger.Key{
			ledger.NewKey([]ledger.KeyPart{ledger.NewKeyPart(9, []byte{6})}),
		},
		[]ledger.Value{
			[]byte{21, 37},
		},
	)
	require.NoError(t, err)

	trieUpdate3, err := pathfinder.UpdateToTrieUpdate(update3, complete.DefaultPathFinderVersion)
	require.NoError(t, err)

	update4, err := ledger.NewUpdate(
		ledger.State(unittest.StateCommitmentFixture()),
		[]ledger.Key{
			ledger.NewKey([]ledger.KeyPart{ledger.NewKeyPart(9, []byte{6})}),
		},
		[]ledger.Value{
			[]byte{21, 37},
		},
	)
	require.NoError(t, err)

	trieUpdate4, err := pathfinder.UpdateToTrieUpdate(update4, complete.DefaultPathFinderVersion)
	require.NoError(t, err)
	return testutil.ComputationResultFixture(t), []*ledger.TrieUpdate{
		trieUpdate1,
		trieUpdate2,
		trieUpdate3,
		trieUpdate4,
	}
}
