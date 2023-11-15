package committer_test

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution/computation/committer"
	"github.com/onflow/flow-go/engine/execution/storehouse"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete"
	ledgermock "github.com/onflow/flow-go/ledger/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/utils/unittest"
)

// TODO: move to a common place
const (
	KeyPartOwner = uint16(0)
	// @deprecated - controller was used only by the very first
	// version of cadence for access controll which was retired later on
	// KeyPartController = uint16(1)
	KeyPartKey = uint16(2)
)

func RegisterIDToKey(reg flow.RegisterID) ledger.Key {
	return ledger.NewKey([]ledger.KeyPart{
		ledger.NewKeyPart(KeyPartOwner, []byte(reg.Owner)),
		ledger.NewKeyPart(KeyPartKey, []byte(reg.Key)),
	})
}

func TestLedgerViewCommitter(t *testing.T) {

	// verify after committing a snapshot, proof will be generated,
	// and changes are saved in storage snapshot
	t.Run("CommitView should return proof and statecommitment", func(t *testing.T) {

		l := new(ledgermock.Ledger)
		committer := committer.NewLedgerViewCommitter(l, trace.NewNoopTracer())

		// CommitDelta will call ledger.Set and ledger.Prove

		reg := flow.NewRegisterID("owner", "key2")
		value := []byte{1}
		startState := unittest.StateCommitmentFixture()

		update, err := ledger.NewUpdate(ledger.State(startState), []ledger.Key{RegisterIDToKey(reg)}, []ledger.Value{value})
		require.NoError(t, err)

		expectedTrieUpdate, err := pathfinder.UpdateToTrieUpdate(update, complete.DefaultPathFinderVersion)
		require.NoError(t, err)

		endState := unittest.StateCommitmentFixture()
		require.NotEqual(t, startState, endState)

		// mock ledger.Set
		l.On("Set", mock.Anything).
			Return(ledger.State(endState), expectedTrieUpdate, nil).
			Once()

			// mock ledger.Prove
		expectedProof := ledger.Proof([]byte{2, 3, 4})
		l.On("Prove", mock.Anything).
			Return(expectedProof, nil).
			Once()

			// previous block's storage snapshot
		previousBlockSnapshot := storehouse.NewExecutingBlockSnapshot(
			snapshot.MapStorageSnapshot{
				flow.NewRegisterID("owner", "key1"): []byte{1},
			},
			flow.StateCommitment(update.State()),
		)

		// this block's register updates
		blockUpdates := &snapshot.ExecutionSnapshot{
			WriteSet: map[flow.RegisterID]flow.RegisterValue{
				reg: value,
			},
		}

		newCommit, proof, trieUpdate, newStorageSnapshot, err := committer.CommitView(
			blockUpdates,
			previousBlockSnapshot,
		)

		require.NoError(t, err)

		// verify CommitView returns expected proof and statecommitment
		require.Equal(t, previousBlockSnapshot.Commitment(), flow.StateCommitment(trieUpdate.RootHash))
		require.Equal(t, newCommit, newStorageSnapshot.Commitment())
		require.Equal(t, endState, newCommit)
		require.Equal(t, []uint8(expectedProof), proof)
		require.True(t, expectedTrieUpdate.Equals(trieUpdate))

		// TOOD(leo): verify ledger.Set and ledger.Prove are called and received expected arguments
	})

}
