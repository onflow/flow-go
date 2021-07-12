package epochs

import (
	"context"
	"fmt"
	"testing"

	"github.com/onflow/flow-go/model/flow"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func TestEpochs(t *testing.T) {
	suite.Run(t, new(Suite))
}

// TestViewsProgress asserts epoch state transitions over two full epochs
// without any nodes joining or leaving.
func (s *Suite) TestViewsProgress() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// testCase is a utility struct that defines what epoch and phase is
	// expected at a certain view. We will populate a list of testCases for a
	// selection of views within two epochs.
	type testCase struct {
		view  uint64
		epoch uint64
		phase flow.EpochPhase
	}

	// iterate through two epochs
	for counter := 0; counter < 2; counter++ {

		// Get the current epoch to populate a list of test cases
		snapshot, err := s.client.GetLatestProtocolSnapshot(ctx)
		require.NoError(s.T(), err)
		epoch := snapshot.Epochs().Current()

		epochCounter, err := epoch.Counter()
		require.NoError(s.T(), err)
		require.Equal(s.T(), uint64(counter), epochCounter)
		epochFirstView, err := epoch.FirstView()
		require.NoError(s.T(), err)
		epochDKGPhase1Final, err := epoch.DKGPhase1FinalView()
		require.NoError(s.T(), err)
		epochDKGPhase2Final, err := epoch.DKGPhase2FinalView()
		require.NoError(s.T(), err)
		epochDKGPhase3Final, err := epoch.DKGPhase3FinalView()
		require.NoError(s.T(), err)
		epochFinal, err := epoch.FinalView()
		require.NoError(s.T(), err)

		expectedViews := []testCase{
			{epoch: epochCounter, view: epochFirstView, phase: flow.EpochPhaseStaking},
			{epoch: epochCounter, view: epochDKGPhase1Final, phase: flow.EpochPhaseSetup},
			{epoch: epochCounter, view: epochDKGPhase2Final, phase: flow.EpochPhaseSetup},
			{epoch: epochCounter, view: epochDKGPhase3Final, phase: flow.EpochPhaseSetup},
			{epoch: epochCounter, view: epochFinal, phase: flow.EpochPhaseCommitted},
		}

		for _, expectedView := range expectedViews {
			_ = s.BlockState.WaitForSealedView(s.T(), expectedView.view)

			snapshot, err := s.client.GetLatestProtocolSnapshot(ctx)
			require.NoError(s.T(), err)

			// BlockState and s.client are not necessarily in sync
			head, err := snapshot.Head()
			require.NoError(s.T(), err)
			if expectedView.view != head.View {
				log.Debug().Msgf(">>> BlockState (view: %d) and ghost client (view: %d) not synchronized", expectedView.view, head.View)
			}

			epoch := snapshot.Epochs().Current()

			currentEpochCounter, err := epoch.Counter()
			require.NoError(s.T(), err)
			require.Equal(s.T(), expectedView.epoch, currentEpochCounter, "wrong epoch")

			currentPhase, err := snapshot.Phase()
			require.NoError(s.T(), err)
			require.Equal(s.T(), expectedView.phase, currentPhase, fmt.Sprintf("wrong phase at view %d", expectedView.view))
		}
	}
}
