package committer_test

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution/computation/committer"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	led "github.com/onflow/flow-go/ledger"
	ledgermock "github.com/onflow/flow-go/ledger/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/utils/unittest"
	utils "github.com/onflow/flow-go/utils/unittest"
)

func TestLedgerViewCommitter(t *testing.T) {

	t.Run("calls to set and prove", func(t *testing.T) {

		ledger := new(ledgermock.Ledger)
		com := committer.NewLedgerViewCommitter(ledger, trace.NewNoopTracer())

		var expectedStateCommitment led.State
		copy(expectedStateCommitment[:], []byte{1, 2, 3})
		ledger.On("Set", mock.Anything).
			Return(expectedStateCommitment, nil, nil).
			Once()

		expectedProof := led.Proof([]byte{2, 3, 4})
		ledger.On("Prove", mock.Anything).
			Return(expectedProof, nil).
			Once()

		newState, proof, _, err := com.CommitView(
			&snapshot.ExecutionSnapshot{
				WriteSet: map[flow.RegisterID]flow.RegisterValue{
					unittest.RegisterIDFixture(): []byte{1},
				},
			},
			utils.StateCommitmentFixture())
		require.NoError(t, err)
		require.Equal(t, flow.StateCommitment(expectedStateCommitment), newState)
		require.Equal(t, []uint8(expectedProof), proof)
	})

}
