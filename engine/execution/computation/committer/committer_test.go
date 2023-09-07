package committer_test

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution/computation/committer"
	"github.com/onflow/flow-go/engine/execution/storehouse"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	led "github.com/onflow/flow-go/ledger"
	ledgermock "github.com/onflow/flow-go/ledger/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestLedgerViewCommitter(t *testing.T) {

	// verify after committing a snapshot, proof will be generated,
	// and changes are saved in storage snapshot
	t.Run("CommitView should return proof and statecommitment", func(t *testing.T) {

		ledger := new(ledgermock.Ledger)
		committer := committer.NewLedgerViewCommitter(ledger, trace.NewNoopTracer())

		// CommitDelta will call ledger.Set and ledger.Prove

		// mock ledger.Set
		expectedStateCommitment := unittest.StateCommitmentFixture()
		ledger.On("Set", mock.Anything).
			Return(expectedStateCommitment, nil, nil).
			Once()

			// mock ledger.Prove
		expectedProof := led.Proof([]byte{2, 3, 4})
		ledger.On("Prove", mock.Anything).
			Return(expectedProof, nil).
			Once()

			// previous block's storage snapshot
		previousBlockSnapshot := storehouse.NewExecutingBlockSnapshot(
			snapshot.MapStorageSnapshot{
				flow.NewRegisterID("owner", "key1"): []byte{1},
			},
			unittest.StateCommitmentFixture(),
		)

		// this block's register updates
		blockUpdates := &snapshot.ExecutionSnapshot{
			WriteSet: map[flow.RegisterID]flow.RegisterValue{
				flow.NewRegisterID("owner", "key2"): []byte{1},
			},
		}

		newCommit, proof, trieUpdate, newStorageSnapshot, err := committer.CommitView(
			blockUpdates,
			previousBlockSnapshot,
		)

		require.NoError(t, err)

		// verify CommitView returns expected proof and statecommitment
		require.Equal(t, newCommit, flow.StateCommitment(trieUpdate.RootHash))
		require.Equal(t, newCommit, newStorageSnapshot.Commitment())
		require.Equal(t, flow.StateCommitment(expectedStateCommitment), newCommit)
		require.Equal(t, []uint8(expectedProof), proof)

		// TOOD(leo): verify ledger.Set and ledger.Prove are called and received expected arguments
	})

}
