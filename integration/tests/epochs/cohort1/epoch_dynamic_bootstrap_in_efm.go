package cohort1

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/crypto"
	sdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go/cmd/bootstrap/cmd"
	"github.com/onflow/flow-go/cmd/bootstrap/utils"
	"github.com/onflow/flow-go/integration/tests/epochs"
	"github.com/onflow/flow-go/model/flow"
)

func TestDynamicBootstrapInEFM(t *testing.T) {
	suite.Run(t, new(DynamicBootstrapInEFMSuite))
}

type DynamicBootstrapInEFMSuite struct {
	epochs.DynamicEpochTransitionSuite
}

func (s *DynamicBootstrapInEFMSuite) SetupTest() {
	// use extremely short DKG phase to ensure we will enter EFM because of missing DKG
	s.StakingAuctionLen = 10
	s.DKGPhaseLen = 1
	s.EpochLen = 200
	s.EpochCommitSafetyThreshold = 50

	// run the generic setup, which starts up the network
	s.BaseSuite.SetupTest()
}

func (s *DynamicBootstrapInEFMSuite) TestDynamicBootstrapInEFM() {
	s.TimedLogf("waiting for EpochPhaseFallback phase of first epoch to begin")
	s.AwaitEpochPhase(s.Ctx, 0, flow.EpochPhaseFallback, time.Minute, 500*time.Millisecond)
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

	networkSeed := cmd.GenerateRandomSeed(crypto.KeyGenSeedMinLen)
	networkKey, err := utils.GeneratePublicNetworkingKey(networkSeed)
	require.NoError(s.T(), err)

	observerInfo := &epochs.StakedNodeOperationInfo{
		NodeID:                  flow.Identifier{},
		Role:                    flow.RoleAccess,
		StakingAccountAddress:   sdk.Address{},
		FullAccountKey:          nil,
		StakingAccountKey:       nil,
		NetworkingKey:           networkKey,
		StakingKey:              nil,
		MachineAccountAddress:   sdk.Address{},
		MachineAccountKey:       nil,
		MachineAccountPublicKey: nil,
		ContainerName:           "observer_1",
	}

	testContainer := s.NewTestContainerOnNetwork(flow.RoleAccess, observerInfo)
	testContainer.WriteRootSnapshot(snapshot)
	testContainer.Container.Start(s.Ctx)

	observerClient, err := testContainer.TestnetClient()
	require.NoError(s.T(), err)

	// ensure node makes progress and finalizes blocks after dynamic bootstrap in EFM
	require.Eventually(s.T(), func() bool {
		observerSnapshot, err := observerClient.GetLatestProtocolSnapshot(s.Ctx)
		require.NoError(s.T(), err)
		finalized, err := observerSnapshot.Head()
		require.NoError(s.T(), err)
		return finalized.View >= header.View+10
	}, 30*time.Second, time.Second)
}
