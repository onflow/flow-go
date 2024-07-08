package cohort1

import (
	"github.com/onflow/flow-go/integration/testnet"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/integration/tests/epochs"
	"github.com/onflow/flow-go/model/flow"
)

func TestDynamicBootstrapInEFM(t *testing.T) {
	suite.Run(t, new(DynamicBootstrapInEFMSuite))
}

type DynamicBootstrapInEFMSuite struct {
	epochs.DynamicEpochTransitionSuite
}

func (s *DynamicBootstrapInEFMSuite) TestDynamicBootstrapInEFM() {
	// pause collection node to trigger EFM because of failed DKG
	ln := s.GetContainersByRole(flow.RoleCollection)[0]
	_ = ln.Pause()

	s.TimedLogf("waiting for EpochPhaseFallback phase of first epoch to begin")
	s.AwaitEpochPhase(s.Ctx, 0, flow.EpochPhaseFallback, 2*time.Minute, time.Second)
	s.TimedLogf("successfully reached EpochPhaseFallback phase of first epoch")

	snapshot, err := s.Client.GetLatestProtocolSnapshot(s.Ctx)
	require.NoError(s.T(), err)
	epochProtocolState, err := snapshot.EpochProtocolState()
	require.NoError(s.T(), err)
	require.True(s.T(), epochProtocolState.EpochFallbackTriggered())

	header, err := snapshot.Head()
	require.NoError(s.T(), err)
	segment, err := snapshot.SealingSegment()
	require.NoError(s.T(), err)
	s.TimedLogf("retrieved header after entering EpochPhaseFallback phase: root_height=%d, root_view=%d, segment_heights=[%d-%d], segment_views=[%d-%d]",
		header.Height, header.View,
		segment.Sealed().Header.Height, segment.Highest().Header.Height,
		segment.Sealed().Header.View, segment.Highest().Header.View)

	observerConf := testnet.ObserverConfig{
		ContainerName: "observer_1",
	}
	testContainer := s.Net.AddObserver(s.T(), observerConf)
	testContainer.WriteRootSnapshot(snapshot)
	testContainer.Container.Start(s.Ctx)

	s.TimedLogf("successfully started observer")

	observerClient, err := testContainer.TestnetClient()
	require.NoError(s.T(), err)

	// ensure node makes progress and finalizes blocks after dynamic bootstrap in EFM
	require.Eventually(s.T(), func() bool {
		observerSnapshot, err := observerClient.GetLatestProtocolSnapshot(s.Ctx)
		require.NoError(s.T(), err)
		finalized, err := observerSnapshot.Head()
		require.NoError(s.T(), err)
		//s.TimedLogf("observer finalized view %d", finalized.View)
		return finalized.View >= header.View+10
	}, 30*time.Second, time.Second)
}
