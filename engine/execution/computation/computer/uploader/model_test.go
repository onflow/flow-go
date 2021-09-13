package uploader

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func Test_ComputationResultToBlockDataConversion(t *testing.T) {

	cr := generateComputationResult(t)

	blockData := ComputationResultToBlockData(cr)

	assert.Equal(t, cr.ExecutableBlock.Block, blockData.Block)
	assert.Equal(t, cr.ExecutableBlock.Collections(), blockData.Collections)
	require.Equal(t, len(cr.TransactionResults), len(blockData.TxResults))
	for i, result := range cr.TransactionResults {
		assert.Equal(t, result, *blockData.TxResults[i])
	}

	eventsCombined := make([]flow.Event, 0)
	for _, eventsList := range cr.Events {
		eventsCombined = append(eventsCombined, eventsList...)
	}
	require.Equal(t, len(eventsCombined), len(blockData.Events))

	for i, event := range eventsCombined {
		assert.Equal(t, event, *blockData.Events[i])
	}

	assert.Equal(t, cr.TrieUpdates, blockData.TrieUpdates)

	assert.Equal(t, cr.StateCommitments[len(cr.StateCommitments)-1], blockData.FinalStateCommitment)
}

func generateComputationResult(t *testing.T) *execution.ComputationResult {

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

	return &execution.ComputationResult{
		ExecutableBlock: unittest.ExecutableBlockFixture([][]flow.Identifier{
			{unittest.IdentifierFixture()},
			{unittest.IdentifierFixture()},
			{unittest.IdentifierFixture()},
		}),
		StateSnapshots: nil,
		StateCommitments: []flow.StateCommitment{
			unittest.StateCommitmentFixture(),
			unittest.StateCommitmentFixture(),
			unittest.StateCommitmentFixture(),
			unittest.StateCommitmentFixture(),
		},
		Proofs: nil,
		Events: []flow.EventsList{
			{
				unittest.EventFixture("what", 0, 0, unittest.IdentifierFixture(), 2),
				unittest.EventFixture("ever", 0, 1, unittest.IdentifierFixture(), 22),
			},
			{},
			{
				unittest.EventFixture("what", 2, 0, unittest.IdentifierFixture(), 2),
				unittest.EventFixture("ever", 2, 1, unittest.IdentifierFixture(), 22),
				unittest.EventFixture("ever", 2, 2, unittest.IdentifierFixture(), 2),
				unittest.EventFixture("ever", 2, 3, unittest.IdentifierFixture(), 22),
			},
			{}, // system chunk events
		},
		EventsHashes:  nil,
		ServiceEvents: nil,
		TransactionResults: []flow.TransactionResult{
			{
				TransactionID:   unittest.IdentifierFixture(),
				ErrorMessage:    "",
				ComputationUsed: 23,
			},
			{
				TransactionID:   unittest.IdentifierFixture(),
				ErrorMessage:    "fail",
				ComputationUsed: 1,
			},
		},
		ComputationUsed: 0,
		StateReads:      0,
		TrieUpdates: []*ledger.TrieUpdate{
			trieUpdate1,
			trieUpdate2,
			trieUpdate3,
			trieUpdate4,
		},
	}
}
