package cohort1

import (
	"testing"
	"time"

	"github.com/onflow/flow-go/integration/tests/epochs"
	"github.com/onflow/flow-go/model/flow"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func TestEpochStaticTransition(t *testing.T) {
	s := new(StaticEpochTransitionSuite)
	suite.Run(t, &s.Suite)
}

// StaticEpochTransitionSuite is the suite used for epoch transition tests
// with a static identity table.
type StaticEpochTransitionSuite struct {
	epochs.Suite
}

func (s *StaticEpochTransitionSuite) SetupTest() {
	// use shorter epoch phases as no staking operations need to occur in the
	// staking phase for this test
	s.Suite.StakingAuctionLen = 10
	s.Suite.DKGPhaseLen = 50
	s.Suite.EpochLen = 300
	s.Suite.EpochCommitSafetyThreshold = 50

	// run the generic setup, which starts up the network
	s.Suite.SetupTest()
}

// TestStaticEpochTransition asserts epoch state transitions over full epoch
// without any nodes joining or leaving. In particular, we assert that we enter
// the EpochSetup phase, then successfully enter the second epoch (implying a
// successful DKG).
// This is equivalent to runTestEpochJoinAndLeave, without any committee changes.
func (s *StaticEpochTransitionSuite) TestStaticEpochTransition() {

	s.Suite.TimedLogf("waiting for EpochSetup phase of first epoch to begin")
	s.Suite.AwaitEpochPhase(s.Suite.Ctx, 0, flow.EpochPhaseSetup, time.Minute, 500*time.Millisecond)
	s.Suite.TimedLogf("successfully reached EpochSetup phase of first epoch")

	snapshot, err := s.Suite.Client.GetLatestProtocolSnapshot(s.Suite.Ctx)
	require.NoError(s.Suite.T(), err)

	header, err := snapshot.Head()
	require.NoError(s.Suite.T(), err)
	s.Suite.TimedLogf("retrieved header after entering EpochSetup phase: height=%d, view=%d", header.Height, header.View)

	epoch1FinalView, err := snapshot.Epochs().Current().FinalView()
	require.NoError(s.Suite.T(), err)
	epoch1Counter, err := snapshot.Epochs().Current().Counter()
	require.NoError(s.Suite.T(), err)

	// wait for the first view of the second epoch
	s.Suite.TimedLogf("waiting for the first view (%d) of second epoch %d", epoch1FinalView+1, epoch1Counter+1)
	s.Suite.AwaitFinalizedView(s.Suite.Ctx, epoch1FinalView+1, 4*time.Minute, 500*time.Millisecond)
	s.Suite.TimedLogf("finalized first view (%d) of second epoch %d", epoch1FinalView+1, epoch1Counter+1)

	// assert transition to second epoch happened as expected
	// if counter is still 0, epoch emergency fallback was triggered and we can fail early
	s.Suite.AssertInEpoch(s.Suite.Ctx, epoch1Counter+1)

	// submit a smoke test transaction to verify the network can seal a transaction
	s.Suite.TimedLogf("sending smoke test transaction in second epoch")
	s.Suite.SubmitSmokeTestTransaction(s.Suite.Ctx)
	s.Suite.TimedLogf("successfully submitted and observed sealing of smoke test transaction")
}
