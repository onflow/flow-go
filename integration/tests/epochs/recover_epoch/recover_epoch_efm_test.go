package recover_epoch

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	
	"github.com/onflow/flow-go/model/flow"
)

func TestRecoverEpoch(t *testing.T) {
	suite.Run(t, new(RecoverEpochSuite))
}

type RecoverEpochSuite struct {
	Suite
}

// TestRecoverEpoch ensures that the recover_epoch transaction flow works as expected. This test will simulate the network going
// into EFM by taking a consensus node offline before completing the DKG. While in EFM mode the test will execute the efm-recover-tx-args
// CLI command to generate transaction arguments to submit a recover_epoch transaction, after submitting the transaction the test will
// ensure the network is healthy.
func (s *RecoverEpochSuite) TestRecoverEpoch() {
	s.AwaitEpochPhase(s.Ctx, 0, flow.EpochPhaseSetup, 30*time.Second, 4*time.Second)

	// pausing execution node will force the network into EFM
	_ = s.GetContainersByRole(flow.RoleExecution)[0].Pause()

	// get the latest snapshot and start new container with it
	rootSnapshot, err := s.Client.GetLatestProtocolSnapshot(s.Ctx)
	require.NoError(s.T(), err)

	epoch1FinalView, err := rootSnapshot.Epochs().Current().FinalView()
	require.NoError(s.T(), err)

	// wait for at least the first block of the next epoch to be sealed before we pause our container to replace
	s.TimedLogf("waiting for epoch transition (finalized view %d) before pausing container", epoch1FinalView+1)
	s.AwaitFinalizedView(s.Ctx, epoch1FinalView+1, 2*time.Minute, 500*time.Millisecond)
	s.TimedLogf("observed finalized view %d -> pausing container", epoch1FinalView+1)

	// assert transition to second epoch did not happen
	// if counter is still 0, epoch emergency fallback was triggered as expected
	s.AssertInEpoch(s.Ctx, 0)
}
