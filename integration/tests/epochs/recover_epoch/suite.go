package recover_epoch

import (
	"fmt"
	"os"

	"github.com/onflow/cadence"
	"github.com/onflow/flow-core-contracts/lib/go/templates"
	sdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go/cmd/util/cmd/epochs/cmd"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/integration/tests/epochs"
	"github.com/onflow/flow-go/integration/utils"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/stretchr/testify/require"
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
	internalNodePrivInfoDir := fmt.Sprintf("%s/%s", s.GetContainersByRole(role)[0].BootstrapPath(), bootstrap.DirPrivateRoot)
	nodeConfigJson := fmt.Sprintf("%s/%s", s.GetContainersByRole(role)[0].BootstrapPath(), bootstrap.PathNodeInfosPub)
	return internalNodePrivInfoDir, nodeConfigJson
}

// executeEFMRecoverTXArgsCMD executes the efm-recover-tx-args CLI command to generate EpochRecover transaction arguments.
// Args:
//
//	role: the container role that will be used to read internal node private info and the node config json.
//	snapshot: the protocol state snapshot.
//	collectionClusters: the number of collector clusters.
//	numViewsInEpoch: the number of views in the recovery epoch.
//	numViewsInStakingAuction: the number of views in the staking auction of the recovery epoch.
//	epochCounter: the epoch counter.
//	targetDuration: the target duration for the recover epoch.
//	targetEndTime: the target end time for the recover epoch.
//	randomSource: the random source of the recover epoch.
//	out: the tx args output file full path.
func (s *Suite) executeEFMRecoverTXArgsCMD(collectionClusters, numViewsInEpoch, numViewsInStakingAuction, epochCounter, targetDuration, targetEndTime uint64, out string) {
	// read internal node info from one of the consensus nodes
	internalNodePrivInfoDir, nodeConfigJson := s.getNodeInfoDirs(flow.RoleConsensus)
	an1 := s.GetContainersByRole(flow.RoleAccess)[0]
	anAddress := an1.Addr(testnet.GRPCPort)
	// set command line arguments
	os.Args = []string{
		"epochs", "efm-recover-tx-args",
		"--insecure=true",
		fmt.Sprintf("--root-chain-id=%s", flow.Localnet),
		fmt.Sprintf("--out=%s", out),
		fmt.Sprintf("--access-address=%s", anAddress),
		fmt.Sprintf("--collection-clusters=%d", collectionClusters),
		fmt.Sprintf("--config=%s", nodeConfigJson),
		fmt.Sprintf("--internal-priv-dir=%s", internalNodePrivInfoDir),
		fmt.Sprintf("--epoch-length=%d", numViewsInEpoch),
		fmt.Sprintf("--epoch-staking-phase-length=%d", numViewsInStakingAuction),
		fmt.Sprintf("--epoch-counter=%d", epochCounter),
		fmt.Sprintf("--epoch-timing-duration=%d", targetDuration),
		fmt.Sprintf("--epoch-timing-end-time=%d", targetEndTime),
	}

	// execute the root command
	rootCmd := cmd.RootCmd
	rootCmd.SetArgs(os.Args[1:])
	if err := rootCmd.Execute(); err != nil {
		s.T().Fatalf("Failed to execute epochs efm-recover-tx-args command: %v", err)
	}
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
