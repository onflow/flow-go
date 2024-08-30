package recover_epoch

import (
	"fmt"

	"github.com/onflow/cadence"
	"github.com/onflow/flow-core-contracts/lib/go/templates"

	sdk "github.com/onflow/flow-go-sdk"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/cmd/bootstrap/run"
	"github.com/onflow/flow-go/integration/tests/epochs"
	"github.com/onflow/flow-go/integration/utils"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
)

// Suite encapsulates common functionality for epoch integration tests.
type Suite struct {
	epochs.BaseSuite
}

func (s *Suite) SetupTest() {
	// use a shorter staking auction because we don't have staking operations in this case
	s.StakingAuctionLen = 2
	// to manually trigger EFM we assign very short dkg phase len ensuring the dkg will fail
	s.DKGPhaseLen = 10
	s.EpochLen = 80
	s.EpochCommitSafetyThreshold = 20
	s.NumOfCollectionClusters = 1

	// run the generic setup, which starts up the network
	s.BaseSuite.SetupTest()
}

// getNodeInfoDirs returns the internal node private info dir and the node config dir from a container with the specified role.
func (s *Suite) getNodeInfoDirs(role flow.Role) (string, string) {
	bootstrapPath := s.GetContainersByRole(role)[0].BootstrapPath()
	internalNodePrivInfoDir := fmt.Sprintf("%s/%s", bootstrapPath, bootstrap.DirPrivateRoot)
	nodeConfigJson := fmt.Sprintf("%s/%s", bootstrapPath, bootstrap.PathNodeInfosPub)
	return internalNodePrivInfoDir, nodeConfigJson
}

// executeEFMRecoverTXArgsCMD executes the efm-recover-tx-args CLI command to generate EpochRecover transaction arguments.
// Args:
//
//	collectionClusters: the number of collector clusters.
//	numViewsInEpoch: the number of views in the recovery epoch.
//	numViewsInStakingAuction: the number of views in the staking auction of the recovery epoch.
//	epochCounter: the epoch counter.
//	targetDuration: the target duration for the recover epoch.
//	targetEndTime: the target end time for the recover epoch.
//
// Returns:
//
//	[]cadence.Value: the transaction arguments.
func (s *Suite) executeEFMRecoverTXArgsCMD(
	collectionClusters int,
	numViewsInEpoch,
	numViewsInStakingAuction,
	recoveryEpochCounter,
	targetDuration uint64,
	unsafeAllowOverWrite bool) []cadence.Value {
	// read internal node info from one of the consensus nodes
	internalNodePrivInfoDir, nodeConfigJson := s.getNodeInfoDirs(flow.RoleConsensus)
	snapshot := s.GetLatestProtocolSnapshot(s.Ctx)
	txArgs, err := run.GenerateRecoverEpochTxArgs(
		s.Log,
		internalNodePrivInfoDir,
		nodeConfigJson,
		collectionClusters,
		recoveryEpochCounter,
		flow.Localnet,
		numViewsInStakingAuction,
		numViewsInEpoch,
		targetDuration,
		unsafeAllowOverWrite,
		snapshot,
	)
	require.NoError(s.T(), err)
	return txArgs
}

// recoverEpoch submits the recover epoch transaction to the network.
func (s *Suite) recoverEpoch(env templates.Environment, args []cadence.Value) *sdk.TransactionResult {
	latestBlockID, err := s.Client.GetLatestBlockID(s.Ctx)
	require.NoError(s.T(), err)

	tx, err := utils.MakeRecoverEpochTx(
		env,
		s.Client.Account(),
		0,
		sdk.Identifier(latestBlockID),
		args,
	)
	require.NoError(s.T(), err)

	err = s.Client.SignAndSendTransaction(s.Ctx, tx)
	require.NoError(s.T(), err)
	result, err := s.Client.WaitForSealed(s.Ctx, tx.ID())
	require.NoError(s.T(), err)
	s.Client.Account().Keys[0].SequenceNumber++
	require.NoError(s.T(), result.Error)

	return result
}
