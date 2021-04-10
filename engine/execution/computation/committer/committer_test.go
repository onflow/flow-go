package committer_test

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution/computation/committer"
	fvmUtils "github.com/onflow/flow-go/fvm/utils"
	led "github.com/onflow/flow-go/ledger"
	ledgermock "github.com/onflow/flow-go/ledger/mock"
	"github.com/onflow/flow-go/module/trace"
	utils "github.com/onflow/flow-go/utils/unittest"
)

func TestLedgerViewCommitter(t *testing.T) {

	t.Run("calls to set and prove", func(t *testing.T) {

		ledger := new(ledgermock.Ledger)
		com := committer.NewLedgerViewCommitter(ledger, trace.NewNoopTracer())

		expectedStateCommitment := led.State([]byte{1, 2, 3})
		ledger.On("Set", mock.Anything).
			Return(expectedStateCommitment, nil).
			Once()

		expectedProof := led.Proof([]byte{2, 3, 4})
		ledger.On("Prove", mock.Anything).
			Return(expectedProof, nil).
			Once()

		view := fvmUtils.NewSimpleView()

		err := view.Set(
			"owner",
			"controller",
			"key",
			[]byte{1},
		)
		require.NoError(t, err)

		newState, proof, err := com.CommitView(view, utils.StateCommitmentFixture())
		require.NoError(t, err)
		require.Equal(t, []uint8(expectedStateCommitment), newState)
		require.Equal(t, []uint8(expectedProof), proof)
	})

}
