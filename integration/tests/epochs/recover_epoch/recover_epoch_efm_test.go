package recover_epoch

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	sdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go/integration/utils"
	"github.com/onflow/flow-go/model/flow"
)

func TestRecoverEpoch(t *testing.T) {
	suite.Run(t, new(RecoverEpochSuite))
}

type RecoverEpochSuite struct {
	Suite
}

// TestRecoverEpoch ensures that the recover epoch governance transaction flow works as expected and a network that
// enters Epoch Fallback Mode can successfully recover. This test will do the following:
// 1. Manually triggers EFM by turning off a collection node before the end of the DKG forcing the DKG to fail.
// 2. Generates epoch recover transaction args using the epoch efm-recover-tx-args.
// 3. Submit recover epoch transaction.
// 4. Ensure expected EpochRecover event is emitted.
// TODO(EFM, #6164): Currently, this test does not test the processing of the EpochRecover event
func (s *RecoverEpochSuite) TestRecoverEpoch() {
	// 1. Manually trigger EFM
	// wait until the epoch setup phase to force network into EFM
	s.AwaitEpochPhase(s.Ctx, 0, flow.EpochPhaseSetup, 10*time.Second, 500*time.Millisecond)
	// pausing collection node will force the network into EFM
	ln := s.GetContainersByRole(flow.RoleCollection)[0]
	_ = ln.Pause()
	s.AwaitFinalizedView(s.Ctx, s.GetDKGEndView(), 2*time.Minute, 500*time.Millisecond)
	// start the paused collection node now that we are in EFM
	require.NoError(s.T(), ln.Start())

	// get final view form the latest snapshot
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

	// 2. Generate transaction arguments for epoch recover transaction.
	// generate epoch recover transaction args
	collectionClusters := s.NumOfCollectionClusters
	numViewsInRecoveryEpoch := s.EpochLen
	numViewsInStakingAuction := s.StakingAuctionLen
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

	// 3. Submit recover epoch transaction to the network.
	// submit the recover epoch transaction
	txArgs, err := utils.ParseJSON(b)
	require.NoError(s.T(), err)
	env := utils.LocalnetEnv()
	result := s.recoverEpoch(env, txArgs)
	require.NoError(s.T(), result.Error)
	require.Equal(s.T(), result.Status, sdk.TransactionStatusSealed)

	// 3. Ensure EpochRecover event was emitted.
	eventType := ""
	for _, evt := range result.Events {
		if strings.Contains(evt.Type, "FlowEpoch.EpochRecover") {
			eventType = evt.Type
			break
		}
	}
	require.NotEmpty(s.T(), eventType, "expected FlowEpoch.EpochRecover event type")
	events, err := s.Client.GetEventsForBlockIDs(s.Ctx, eventType, []sdk.Identifier{result.BlockID})
	require.NoError(s.T(), err)
	require.Equal(s.T(), events[0].Events[0].Type, eventType)

	// 4. @TODO ensure EpochRecover service event is processed by the fallback state machine and the network recovers.
}
