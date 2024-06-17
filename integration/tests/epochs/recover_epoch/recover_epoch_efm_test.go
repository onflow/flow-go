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
	s.AwaitEpochPhase(s.Ctx, 0, flow.EpochPhaseSetup, 30*time.Second, 4*time.Second)

	// pausing execution node will force the network into EFM
	_ = s.GetContainersByRole(flow.RoleExecution)[0].Pause()

	// get the latest snapshot and start new container with it
	epoch1FinalView, err := s.Net.BootstrapSnapshot.Epochs().Current().FinalView()
	require.NoError(s.T(), err)

	// wait for at least the first block of the next epoch to be sealed so that we can
	// ensure that we are still in the same epoch after the final view of that epoch indicating we are in EFM
	s.TimedLogf("waiting for epoch transition (finalized view %d) before pausing container", epoch1FinalView+1)
	s.AwaitFinalizedView(s.Ctx, epoch1FinalView+1, 2*time.Minute, 500*time.Millisecond)
	s.TimedLogf("observed finalized view %d -> pausing container", epoch1FinalView+1)

	// assert transition to second epoch did not happen
	// if counter is still 0, epoch emergency fallback was triggered as expected
	s.AssertInEpoch(s.Ctx, 0)

	// generate epoch recover transaction args
	collectionClusters := uint64(1)
	numViewsInEpoch := uint64(4000)
	numViewsInStakingAuction := uint64(100)
	epochCounter := uint64(2)
	randomSource := "ohsorandom"
	targetDuration := uint64(3000)
	targetEndTime := uint64(4000)
	out := fmt.Sprintf("%s/recover-epoch-tx-args.josn", s.Net.BootstrapDir)

	s.executeEFMRecoverTXArgsCMD(
		flow.RoleConsensus,
		collectionClusters,
		numViewsInEpoch,
		numViewsInStakingAuction,
		epochCounter,
		targetDuration,
		targetEndTime,
		randomSource,
		out,
	)
	b, err := os.ReadFile(out)
	require.NoError(s.T(), err)

	txArgs, err := utils.ParseJSON(b)
	require.NoError(s.T(), err)

	env := utils.LocalnetEnv()
	result := s.recoverEpoch(env, txArgs)
	fmt.Println("TX RESULT: ", result)
	// submit recover epoch transaction to recover the network
}
