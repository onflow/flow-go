package recover_epoch

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/integration/utils"
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
	// wait until the epoch setup phase to force network into EFM
	s.AwaitEpochPhase(s.Ctx, 0, flow.EpochPhaseSetup, 10*time.Second, 500*time.Millisecond)

	// pausing consensus node will force the network into EFM
	enContainer := s.GetContainersByRole(flow.RoleCollection)[0]
	_ = enContainer.Pause()

	s.AwaitFinalizedView(s.Ctx, 32, 2*time.Minute, 500*time.Millisecond)

	// start the paused execution node now that we are in EFM
	require.NoError(s.T(), enContainer.Start())

	// get the latest snapshot and start new container with it
	epoch1FinalView, err := s.Net.BootstrapSnapshot.Epochs().Current().FinalView()
	require.NoError(s.T(), err)

	// wait for at least the first block of the next epoch to be sealed so that we can
	// ensure that we are still in the same epoch after the final view of that epoch indicating we are in EFM
	s.TimedLogf("waiting for epoch transition (finalized view %d) before pausing container", epoch1FinalView+1)
	s.AwaitFinalizedView(s.Ctx, epoch1FinalView+1, 2*time.Minute, 500*time.Millisecond)
	s.TimedLogf("observed finalized view %d -> pausing container", epoch1FinalView+1)

	//assert transition to second epoch did not happen
	//if counter is still 0, epoch emergency fallback was triggered as expected
	s.AssertInEpoch(s.Ctx, 0)

	// generate epoch recover transaction args
	collectionClusters := uint64(1)
	numViewsInRecoveryEpoch := uint64(80)
	numViewsInStakingAuction := uint64(2)
	epochCounter := uint64(1)
	targetDuration := uint64(3000)
	targetEndTime := uint64(4000)
	out := fmt.Sprintf("%s/recover-epoch-tx-args.json", s.Net.BootstrapDir)
	s.executeEFMRecoverTXArgsCMD(
		collectionClusters,
		numViewsInRecoveryEpoch,
		numViewsInStakingAuction,
		epochCounter,
		targetDuration,
		targetEndTime,
		out,
	)
	b, err := os.ReadFile(out)
	require.NoError(s.T(), err)

	txArgs, err := utils.ParseJSON(b)
	require.NoError(s.T(), err)
	env := utils.LocalnetEnv()
	result := s.recoverEpoch(env, txArgs)

	latestFinalizedHeader := s.GetLatestFinalizedHeader(s.Ctx)
	// wait for at-least 2 epoch transitions to ensure successful recovery
	waitForView := latestFinalizedHeader.View + numViewsInRecoveryEpoch
	s.TimedLogf("waiting for at-least 2 epoch transitions (finalized view %d)", waitForView)
	s.AwaitFinalizedView(s.Ctx, waitForView, 2*time.Minute, 500*time.Millisecond)
	s.TimedLogf("observed finalized view %d", waitForView)

	snap := s.GetLatestProtocolSnapshot(s.Ctx)
	c, _ := snap.Epochs().Next().Counter()
	fmt.Println("NEXT EPOCH COUNTER", c)
	currentEpoch := s.CurrentEpoch(s.Ctx)
	// assert we have transitioned out of Epoch 0 indicating a successful recovery.
	require.Greater(s.T(), currentEpoch, uint64(0))

	fmt.Println("TX RESULT STATUS: ", result)
}
