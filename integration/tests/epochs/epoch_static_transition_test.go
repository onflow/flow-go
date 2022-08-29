package epochs

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
)

func TestEpochStaticTransition(t *testing.T) {
	suite.Run(t, new(StaticEpochTransitionSuite))
}

// StaticEpochTransitionSuite is the suite used for epoch transition tests
// with a static identity table.
type StaticEpochTransitionSuite struct {
	Suite
}

func (s *StaticEpochTransitionSuite) SetupTest() {
	// use shorter epoch phases as no staking operations need to occur in the
	// staking phase for this test
	s.StakingAuctionLen = 10
	s.DKGPhaseLen = 50
	s.EpochLen = 300
	s.EpochCommitSafetyThreshold = 50

	// run the generic setup, which starts up the network
	s.Suite.SetupTest()
}

// TestStaticEpochTransition asserts epoch state transitions over full epoch
// without any nodes joining or leaving. In particular, we assert that we enter
// the EpochSetup phase, then successfully enter the second epoch (implying a
// successful DKG).
// This is equivalent to runTestEpochJoinAndLeave, without any committee changes.
func (s *StaticEpochTransitionSuite) TestStaticEpochTransition() {
	s.TimedLogf("waiting for EpochSetup phase of first epoch to begin")
	s.WaitForPhase(s.ctx, flow.EpochPhaseSetup)
	s.TimedLogf("successfully reached EpochSetup phase of first epoch")

	snapshot, err := s.client.GetLatestProtocolSnapshot(s.ctx)
	require.NoError(s.T(), err)

	header, err := snapshot.Head()
	require.NoError(s.T(), err)
	s.TimedLogf("retrieved header after entering EpochSetup phase: height=%d, view=%d", header.Height, header.View)

	epoch1FinalView, err := snapshot.Epochs().Current().FinalView()
	require.NoError(s.T(), err)
	epoch1Counter, err := snapshot.Epochs().Current().Counter()
	require.NoError(s.T(), err)

	// wait for the final view of the first epoch
	s.TimedLogf("waiting for the final view (%d) of epoch %d", epoch1FinalView, epoch1Counter)
	s.BlockState.WaitForSealedView(s.T(), epoch1FinalView+5)
	s.TimedLogf("sealed final view (%d) of epoch %d", epoch1FinalView, epoch1Counter)

	// assert transition to second epoch happened as expected
	// if counter is still 0, epoch emergency fallback was triggered and we can fail early
	s.assertEpochCounter(s.ctx, 1)

	// submit a smoke test transaction to verify the network can seal a transaction
	s.TimedLogf("sending smoke test transaction in second epoch")
	s.submitSmokeTestTransaction(s.ctx)
	s.TimedLogf("successfully submitted and observed sealing of smoke test transaction")
}
