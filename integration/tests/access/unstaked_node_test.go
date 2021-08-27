package access

import (
	"context"
	"crypto/rand"
	"fmt"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/cmd/bootstrap/utils"
	"github.com/onflow/flow-go/crypto"
	consensus_follower "github.com/onflow/flow-go/follower"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

const blockCount = 5 // number of finalized blocks to wait for

type UnstakedAccessSuite struct {
	suite.Suite

	// root context for the current test
	ctx    context.Context
	cancel context.CancelFunc

	net                   *testnet.FlowNetwork
	stakedID              flow.Identifier
	unstakedID            flow.Identifier
	conID                 flow.Identifier
	follower              consensus_follower.ConsensusFollower
	finalizedBlockIDsChan chan flow.Identifier
}

func TestUnstakedAccessSuite(t *testing.T) {
	suite.Run(t, new(UnstakedAccessSuite))
}

func (suite *UnstakedAccessSuite) TearDownTest() {
	// avoid nil pointer errors for skipped tests
	if suite.cancel != nil {
		defer suite.cancel()
	}
	if suite.net != nil {
		suite.net.Remove()
	}
}

func (suite *UnstakedAccessSuite) SetupTest() {
	suite.finalizedBlockIDsChan = make(chan flow.Identifier, blockCount)
	suite.buildNetworkConfig()
	// start the network
	suite.ctx, suite.cancel = context.WithCancel(context.Background())
	suite.net.Start(suite.ctx)
}

// TestReceiveBlocks tests that consensus follower follows the chain and persists blocks in storage
func (suite *UnstakedAccessSuite) TestReceiveBlocks() {
	ctx, cancel := context.WithCancel(suite.ctx)
	defer cancel()
	// kick off the follower
	go suite.follower.Run(ctx)

	followerImpl, ok := suite.follower.(*consensus_follower.ConsensusFollowerImpl)
	if !ok {
		suite.Fail("unexpected consensus follower implementation")
		return
	}

	// get the underlying node builder
	node := followerImpl.NodeBuilder
	// wait for the follower to have completely started
	unittest.RequireCloseBefore(suite.T(), node.Ready(), 10*time.Second,
		"timed out while waiting for consensus follower to start")

	// get the underlying storage that the follower is using
	storage := node.Storage
	require.NotNil(suite.T(), storage)
	blocks := storage.Blocks
	require.NotNil(suite.T(), blocks)

	rcvdBlockCnt := 0
	var err error
	receiveBlocks := func() {
		for ; rcvdBlockCnt < blockCount; rcvdBlockCnt++ {
			select {
			case blockID := <-suite.finalizedBlockIDsChan:
				_, err = blocks.ByID(blockID)
				if err != nil {
					return
				}
			}
		}
	}

	// wait for finalized blocks
	unittest.AssertReturnsBefore(suite.T(), receiveBlocks, 1*time.Minute) // waiting 1 minute for 5 blocks

	// all blocks were found in the storage
	require.NoError(suite.T(), err, "finalized block not found in storage")

	// assert that blockCount number of blocks were received
	require.Equal(suite.T(), blockCount, rcvdBlockCnt)

}

func (suite *UnstakedAccessSuite) OnBlockFinalizedConsumer(finalizedBlockID flow.Identifier) {
	// push the finalized block ID to the finalizedBlockIDsChan channel
	suite.finalizedBlockIDsChan <- finalizedBlockID
}

func (suite *UnstakedAccessSuite) buildNetworkConfig() {

	// staked access node
	suite.stakedID = unittest.IdentifierFixture()
	stakedConfig := testnet.NewNodeConfig(
		flow.RoleAccess,
		testnet.WithID(suite.stakedID),
		testnet.SupportsUnstakedNodes(),
		testnet.WithLogLevel(zerolog.TraceLevel),
	)

	collectionConfigs := []func(*testnet.NodeConfig){
		testnet.WithAdditionalFlag("--hotstuff-timeout=12s"),
		testnet.WithAdditionalFlag("--block-rate-delay=100ms"),
		testnet.WithLogLevel(zerolog.WarnLevel),
	}

	consensusConfigs := []func(config *testnet.NodeConfig){
		testnet.WithAdditionalFlag("--hotstuff-timeout=12s"),
		testnet.WithAdditionalFlag("--block-rate-delay=100ms"),
		testnet.WithAdditionalFlag(fmt.Sprintf("--required-verification-seal-approvals=%d", 0)),
		testnet.WithAdditionalFlag(fmt.Sprintf("--required-construction-seal-approvals=%d", 0)),
		testnet.WithLogLevel(zerolog.WarnLevel),
	}

	net := []testnet.NodeConfig{
		testnet.NewNodeConfig(flow.RoleCollection, collectionConfigs...),
		testnet.NewNodeConfig(flow.RoleCollection, collectionConfigs...),
		testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.WarnLevel)),
		testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.WarnLevel)),
		testnet.NewNodeConfig(flow.RoleConsensus, consensusConfigs...),
		testnet.NewNodeConfig(flow.RoleConsensus, consensusConfigs...),
		testnet.NewNodeConfig(flow.RoleConsensus, consensusConfigs...),
		testnet.NewNodeConfig(flow.RoleVerification, testnet.WithLogLevel(zerolog.WarnLevel), testnet.WithDebugImage(false)),
		stakedConfig,
	}

	unstakedKey, err := UnstakedNetworkingKey()
	require.NoError(suite.T(), err)

	followerConfigs := []testnet.ConsensusFollowerConfig{
		testnet.NewConsensusFollowerConfig(suite.T(), unstakedKey, suite.stakedID),
	}

	suite.unstakedID = followerConfigs[0].NodeID

	// consensus follower
	conf := testnet.NewNetworkConfig("consensus follower test", net, testnet.WithConsensusFollowers(followerConfigs...))
	suite.net = testnet.PrepareFlowNetwork(suite.T(), conf)

	suite.follower = suite.net.ConsensusFollowerByID(suite.unstakedID)
	suite.follower.AddOnBlockFinalizedConsumer(suite.OnBlockFinalizedConsumer)
}

// TODO: Move this to unittest and resolve the circular dependency issue
func UnstakedNetworkingKey() (crypto.PrivateKey, error) {
	seed := make([]byte, crypto.KeyGenSeedMinLenECDSASecp256k1)
	n, err := rand.Read(seed)
	if err != nil || n != crypto.KeyGenSeedMinLenECDSASecp256k1 {
		return nil, err
	}
	return utils.GenerateUnstakedNetworkingKey(unittest.SeedFixture(n))
}
