package cohort1

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/integration/tests/epochs"
	"github.com/onflow/flow-go/model/flow"
)

func TestEpochStaticTransition(t *testing.T) {
	suite.Run(t, new(StaticEpochTransitionSuite))
}

// StaticEpochTransitionSuite is the suite used for epoch transition tests
// with a static identity table.
type StaticEpochTransitionSuite struct {
	epochs.Suite
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
	s.AwaitEpochPhase(s.Ctx, 0, flow.EpochPhaseSetup, time.Minute, 500*time.Millisecond)
	s.TimedLogf("successfully reached EpochSetup phase of first epoch")

	snapshot, err := s.Client.GetLatestProtocolSnapshot(s.Ctx)
	require.NoError(s.T(), err)

	header, err := snapshot.Head()
	require.NoError(s.T(), err)
	s.TimedLogf("retrieved header after entering EpochSetup phase: height=%d, view=%d", header.Height, header.View)

	epoch1FinalView, err := snapshot.Epochs().Current().FinalView()
	require.NoError(s.T(), err)
	epoch1Counter, err := snapshot.Epochs().Current().Counter()
	require.NoError(s.T(), err)

	// wait for the first view of the second epoch
	s.TimedLogf("waiting for the first view (%d) of second epoch %d", epoch1FinalView+1, epoch1Counter+1)
	s.AwaitFinalizedView(s.Ctx, epoch1FinalView+1, 4*time.Minute, 500*time.Millisecond)
	s.TimedLogf("finalized first view (%d) of second epoch %d", epoch1FinalView+1, epoch1Counter+1)

	// assert transition to second epoch happened as expected
	// if counter is still 0, epoch emergency fallback was triggered and we can fail early
	s.AssertInEpoch(s.Ctx, epoch1Counter+1)

	// submit a smoke test transaction to verify the network can seal a transaction
	s.TimedLogf("sending smoke test transaction in second epoch")
	s.SubmitSmokeTestTransaction(s.Ctx)
	s.TimedLogf("successfully submitted and observed sealing of smoke test transaction")
}
