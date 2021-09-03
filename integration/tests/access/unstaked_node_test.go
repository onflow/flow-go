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

	net                    *testnet.FlowNetwork
	stakedID               flow.Identifier
	conID                  flow.Identifier
	followers              []consensus_follower.ConsensusFollower
	finalizedBlockIDsChans []chan flow.Identifier
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
	suite.finalizedBlockIDsChans = []chan flow.Identifier{
		make(chan flow.Identifier, blockCount),
		make(chan flow.Identifier, blockCount),
	}
	suite.buildNetworkConfig()
	// start the network
	suite.ctx, suite.cancel = context.WithCancel(context.Background())
	suite.net.Start(suite.ctx)
}

// TestReceiveBlocks tests that consensus follower follows the chain and persists blocks in storage
func (suite *UnstakedAccessSuite) TestReceiveBlocks() {
	ctx, cancel := context.WithCancel(suite.ctx)
	defer cancel()
	// kick off the first follower
	go suite.followers[0].Run(ctx)

	followerImpl1, ok := suite.followers[0].(*consensus_follower.ConsensusFollowerImpl)
	if !ok {
		suite.Fail("unexpected consensus follower implementation")
		return
	}

	// get the underlying node builder
	node1 := followerImpl1.NodeBuilder
	// wait for the follower to have completely started
	unittest.RequireCloseBefore(suite.T(), node1.Ready(), 10*time.Second,
		"timed out while waiting for consensus follower to start")

	// get the underlying storage that the follower is using
	storage := node1.Storage
	require.NotNil(suite.T(), storage)
	blocks := storage.Blocks
	require.NotNil(suite.T(), blocks)

	rcvdBlockCnt := 0
	var err error
	receivedBlocks := make([]flow.Identifier, blockCount)
	receiveBlocks := func() {
		for ; rcvdBlockCnt < blockCount; rcvdBlockCnt++ {
			select {
			case blockID := <-suite.finalizedBlockIDsChans[0]:
				receivedBlocks[rcvdBlockCnt] = blockID
				_, err = blocks.ByID(blockID)
				if err != nil {
					return
				}
			}
		}
	}

	// wait for finalized blocks
	unittest.AssertReturnsBefore(suite.T(), receiveBlocks, 2*time.Minute) // waiting 1 minute for 5 blocks

	// all blocks were found in the storage
	require.NoError(suite.T(), err, "finalized block not found in storage")

	// assert that blockCount number of blocks were received
	require.Equal(suite.T(), blockCount, rcvdBlockCnt)

	// kick off the second follower
	go suite.followers[1].Run(ctx)

	followerImpl2, ok := suite.followers[1].(*consensus_follower.ConsensusFollowerImpl)
	if !ok {
		suite.Fail("unexpected consensus follower implementation")
		return
	}

	// get the underlying node builder
	node2 := followerImpl2.NodeBuilder
	// wait for the follower to have completely started
	unittest.RequireCloseBefore(suite.T(), node2.Ready(), 10*time.Second,
		"timed out while waiting for consensus follower to start")

	receiveBlocks2 := func() {
		for _, blockID := range receivedBlocks {
			select {
			case receivedBlockID := <-suite.finalizedBlockIDsChans[1]:
				suite.Assert().Equal(blockID, receivedBlockID)
			}
		}
	}

	// wait for finalized blocks
	unittest.AssertReturnsBefore(suite.T(), receiveBlocks2, 2*time.Minute) // waiting 1 minute for 5 blocks
}

func (suite *UnstakedAccessSuite) OnBlockFinalizedConsumer(index int) func(flow.Identifier) {
	return func(finalizedBlockID flow.Identifier) {
		// push the finalized block ID to the finalizedBlockIDsChan channel
		suite.finalizedBlockIDsChans[index] <- finalizedBlockID
	}
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
		testnet.WithAdditionalFlag(fmt.Sprintf("--required-verification-seal-approvals=%d", 1)),
		testnet.WithAdditionalFlag(fmt.Sprintf("--required-construction-seal-approvals=%d", 1)),
		testnet.WithLogLevel(zerolog.DebugLevel),
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

	unstakedKey1, err := UnstakedNetworkingKey()
	require.NoError(suite.T(), err)
	unstakedKey2, err := UnstakedNetworkingKey()
	require.NoError(suite.T(), err)

	followerConfigs := []testnet.ConsensusFollowerConfig{
		testnet.NewConsensusFollowerConfig(suite.T(), unstakedKey1, suite.stakedID, consensus_follower.WithLogLevel("debug")),
		testnet.NewConsensusFollowerConfig(suite.T(), unstakedKey2, suite.stakedID, consensus_follower.WithLogLevel("debug")),
	}

	// consensus follower
	conf := testnet.NewNetworkConfig("consensus follower test", net, testnet.WithConsensusFollowers(followerConfigs...))
	suite.net = testnet.PrepareFlowNetwork(suite.T(), conf)

	follower1 := suite.net.ConsensusFollowerByID(followerConfigs[0].NodeID)
	follower1.AddOnBlockFinalizedConsumer(suite.OnBlockFinalizedConsumer(0))

	follower2 := suite.net.ConsensusFollowerByID(followerConfigs[1].NodeID)
	follower2.AddOnBlockFinalizedConsumer(suite.OnBlockFinalizedConsumer(1))

	suite.followers = []consensus_follower.ConsensusFollower{follower1, follower2}
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
