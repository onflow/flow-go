package epochs

import (
	"context"
	"fmt"
	"testing"

	"github.com/onflow/flow-go/model/flow"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func TestEpochs(t *testing.T) {
	suite.Run(t, new(Suite))
}

func (s *Suite) TestViewsProgress() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	snapshot, err := s.client.GetLatestProtocolSnapshot(ctx)
	require.NoError(s.T(), err)

	type testView struct {
		view  uint64
		epoch uint64
		phase flow.EpochPhase
	}

	for counter := 0; counter < 1; counter++ {
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

		expectedViews := []testView{
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
