package committer_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution/computation/committer"
	"github.com/onflow/flow-go/engine/execution/storehouse"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete"
	ledgermock "github.com/onflow/flow-go/ledger/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestLedgerViewCommitter(t *testing.T) {

	// verify after committing a snapshot, proof will be generated,
	// and changes are saved in storage snapshot
	t.Run("CommitView should return proof and statecommitment", func(t *testing.T) {

		l := ledgermock.NewLedger(t)
		committer := committer.NewLedgerViewCommitter(l, trace.NewNoopTracer())

		// CommitDelta will call ledger.Set and ledger.Prove

		reg := unittest.MakeOwnerReg("key1", "val1")
		startState := unittest.StateCommitmentFixture()

		update, err := ledger.NewUpdate(ledger.State(startState), []ledger.Key{convert.RegisterIDToLedgerKey(reg.Key)}, []ledger.Value{reg.Value})
		require.NoError(t, err)

		expectedTrieUpdate, err := pathfinder.UpdateToTrieUpdate(update, complete.DefaultPathFinderVersion)
		require.NoError(t, err)

		endState := unittest.StateCommitmentFixture()
		require.NotEqual(t, startState, endState)

		// mock ledger.Set
		l.On("Set", mock.Anything).
			Return(func(update *ledger.Update) (newState ledger.State, trieUpdate *ledger.TrieUpdate, err error) {
				if update.State().Equals(ledger.State(startState)) {
					return ledger.State(endState), expectedTrieUpdate, nil
				}
				return ledger.DummyState, nil, fmt.Errorf("wrong update")
			}).
			Once()

			// mock ledger.Prove
		expectedProof := ledger.Proof([]byte{2, 3, 4})
		l.On("Prove", mock.Anything).
			Return(func(query *ledger.Query) (proof ledger.Proof, err error) {
				if query.Size() != 1 {
					return nil, fmt.Errorf("wrong query size: %v", query.Size())
				}

				k := convert.RegisterIDToLedgerKey(reg.Key)
				if !query.Keys()[0].Equals(&k) {
					return nil, fmt.Errorf("in correct query key for prove: %v", query.Keys()[0])
				}

				return expectedProof, nil
			}).
			Once()

			// previous block's storage snapshot
		oldReg := unittest.MakeOwnerReg("key1", "oldvalue")
		previousBlockSnapshot := storehouse.NewExecutingBlockSnapshot(
			snapshot.MapStorageSnapshot{
				oldReg.Key: oldReg.Value,
			},
			flow.StateCommitment(update.State()),
		)

		// this block's register updates
		blockUpdates := &snapshot.ExecutionSnapshot{
			WriteSet: map[flow.RegisterID]flow.RegisterValue{
				reg.Key: oldReg.Value,
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

	})

}
