package cohort1

import (
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/integration/tests/epochs"
	"github.com/onflow/flow-go/model/flow"
)

func TestDynamicBootstrapInEFM(t *testing.T) {
	suite.Run(t, new(DynamicBootstrapInEFMSuite))
}

type DynamicBootstrapInEFMSuite struct {
	epochs.DynamicEpochTransitionSuite
}

// TestDynamicBootstrapInEFM tests the dynamic bootstrap in EFM. First, the test pauses the collection node to trigger EFM.
// After triggering EFM, the test waits for the EpochPhaseFallback phase of the first epoch to begin and then starts an observer.
// We specifically start an observer node since it can join the network anytime.
// Finally, we ensure that the node makes progress and finalizes blocks after dynamic bootstrap in EFM.
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
		LogLevel:      zerolog.WarnLevel,
	}
	testContainer := s.Net.AddObserver(s.T(), observerConf)
	testContainer.WriteRootSnapshot(snapshot)
	testContainer.Container.Start(s.Ctx)
	s.TimedLogf("successfully started observer")

	observerClient, err := testContainer.TestnetClient()
	require.NoError(s.T(), err)

	// ensure node makes progress and finalizes blocks after dynamic bootstrap in EFM
	targetFinalizedView := header.View + 10
	require.Eventually(s.T(), func() bool {
		observerSnapshot, err := observerClient.GetLatestProtocolSnapshot(s.Ctx)
		require.NoError(s.T(), err)
		finalized, err := observerSnapshot.Head()
		require.NoError(s.T(), err)
		return finalized.View >= targetFinalizedView
	}, 30*time.Second, time.Second)
}
