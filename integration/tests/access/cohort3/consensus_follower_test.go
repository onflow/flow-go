package cohort3

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/cmd/bootstrap/utils"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/crypto"
	consensus_follower "github.com/onflow/flow-go/follower"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

const blockCount = 5 // number of finalized blocks to wait for

func TestConsensusFollower(t *testing.T) {
	suite.Run(t, new(ConsensusFollowerSuite))
}

type ConsensusFollowerSuite struct {
	suite.Suite

	log zerolog.Logger

	// root context for the current test
	ctx    context.Context
	cancel context.CancelFunc

	net          *testnet.FlowNetwork
	stakedID     flow.Identifier
	conID        flow.Identifier
	followerMgr1 *followerManager
	followerMgr2 *followerManager
}

func (s *ConsensusFollowerSuite) TearDownTest() {
	s.log.Info().Msg("================> Start TearDownTest")
	s.net.Remove()
	s.cancel()
	s.log.Info().Msgf("================> Finish TearDownTest")
}

func (s *ConsensusFollowerSuite) SetupTest() {
	s.log = unittest.LoggerForTest(s.Suite.T(), zerolog.InfoLevel)
	s.log.Info().Msg("================> SetupTest")
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.buildNetworkConfig()
	// start the network
	s.net.Start(s.ctx)
}

// TestReceiveBlocks tests the following
// 1. The consensus follower follows the chain and persists blocks in storage.
// 2. The consensus follower can catch up if it is started after the chain has started producing blocks.
func (s *ConsensusFollowerSuite) TestReceiveBlocks() {
	ctx, cancel := context.WithCancel(s.ctx)
	defer cancel()

	receivedBlocks := make(map[flow.Identifier]struct{}, blockCount)

	s.Run("consensus follower follows the chain", func() {
		// kick off the first follower
		s.followerMgr1.startFollower(ctx)
		var err error
		receiveBlocks := func() {
			for i := 0; i < blockCount; i++ {
				blockID := <-s.followerMgr1.blockIDChan
				receivedBlocks[blockID] = struct{}{}
				_, err = s.followerMgr1.getBlock(blockID)
				if err != nil {
					return
				}
			}
		}

		// wait for finalized blocks
		unittest.AssertReturnsBefore(s.T(), receiveBlocks, 2*time.Minute) // waiting 2 minute for 5 blocks

		// all blocks were found in the storage
		require.NoError(s.T(), err, "finalized block not found in storage")

		// assert that blockCount number of blocks were received
		require.Len(s.T(), receivedBlocks, blockCount)
	})

	s.Run("consensus follower sync up with the chain", func() {
		// kick off the second follower
		s.followerMgr2.startFollower(ctx)

		// the second follower is now atleast blockCount blocks behind and should sync up and get all the missed blocks
		receiveBlocks := func() {
			for {
				select {
				case <-ctx.Done():
					return
				case blockID := <-s.followerMgr2.blockIDChan:
					delete(receivedBlocks, blockID)
					if len(receivedBlocks) == 0 {
						return
					}
				}
			}
		}
		// wait for finalized blocks
		unittest.AssertReturnsBefore(s.T(), receiveBlocks, 2*time.Minute) // waiting 2 minute for the missing 5 blocks
	})
}

func (s *ConsensusFollowerSuite) buildNetworkConfig() {

	// staked access node
	s.stakedID = unittest.IdentifierFixture()
	stakedConfig := testnet.NewNodeConfig(
		flow.RoleAccess,
		testnet.WithID(s.stakedID),
		testnet.WithAdditionalFlag("--supports-observer=true"),
		testnet.WithLogLevel(zerolog.WarnLevel),
	)

	collectionConfigs := []func(*testnet.NodeConfig){
		testnet.WithLogLevel(zerolog.FatalLevel),
		testnet.AsGhost(),
	}

	consensusConfigs := []func(config *testnet.NodeConfig){
		testnet.WithAdditionalFlag("--cruise-ctl-fallback-proposal-duration=100ms"),
		testnet.WithAdditionalFlag(fmt.Sprintf("--required-verification-seal-approvals=%d", 1)),
		testnet.WithAdditionalFlag(fmt.Sprintf("--required-construction-seal-approvals=%d", 1)),
		testnet.WithLogLevel(zerolog.FatalLevel),
	}

	net := []testnet.NodeConfig{
		testnet.NewNodeConfig(flow.RoleCollection, collectionConfigs...),
		testnet.NewNodeConfig(flow.RoleCollection, collectionConfigs...),
		testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.FatalLevel)),
		testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.FatalLevel)),
		testnet.NewNodeConfig(flow.RoleConsensus, consensusConfigs...),
		testnet.NewNodeConfig(flow.RoleConsensus, consensusConfigs...),
		testnet.NewNodeConfig(flow.RoleConsensus, consensusConfigs...),
		testnet.NewNodeConfig(flow.RoleVerification, testnet.WithLogLevel(zerolog.FatalLevel)),
		testnet.NewNodeConfig(flow.RoleAccess, testnet.WithLogLevel(zerolog.FatalLevel)),
		stakedConfig,
	}

	unstakedKey1, err := UnstakedNetworkingKey()
	require.NoError(s.T(), err)
	unstakedKey2, err := UnstakedNetworkingKey()
	require.NoError(s.T(), err)

	followerConfigs := []testnet.ConsensusFollowerConfig{
		testnet.NewConsensusFollowerConfig(s.T(), unstakedKey1, s.stakedID, consensus_follower.WithLogLevel("warn")),
		testnet.NewConsensusFollowerConfig(s.T(), unstakedKey2, s.stakedID, consensus_follower.WithLogLevel("warn")),
	}

	// consensus followers
	conf := testnet.NewNetworkConfig("consensus follower test", net, testnet.WithConsensusFollowers(followerConfigs...))
	s.net = testnet.PrepareFlowNetwork(s.T(), conf, flow.Localnet)

	follower1 := s.net.ConsensusFollowerByID(followerConfigs[0].NodeID)
	s.followerMgr1, err = newFollowerManager(s.T(), follower1)
	require.NoError(s.T(), err)

	follower2 := s.net.ConsensusFollowerByID(followerConfigs[1].NodeID)
	s.followerMgr2, err = newFollowerManager(s.T(), follower2)
	require.NoError(s.T(), err)
}

// TODO: Move this to unittest and resolve the circular dependency issue
func UnstakedNetworkingKey() (crypto.PrivateKey, error) {
	return utils.GeneratePublicNetworkingKey(unittest.SeedFixture(crypto.KeyGenSeedMinLen))
}

// followerManager is a convenience wrapper around the consensus follower
type followerManager struct {
	follower    *consensus_follower.ConsensusFollowerImpl
	blockIDChan chan flow.Identifier
	t           *testing.T
}

func newFollowerManager(t *testing.T, follower consensus_follower.ConsensusFollower) (*followerManager, error) {
	followerImpl, ok := follower.(*consensus_follower.ConsensusFollowerImpl)
	if !ok {
		return nil, fmt.Errorf("unexpected consensus follower implementation")
	}
	fm := &followerManager{
		follower:    followerImpl,
		blockIDChan: make(chan flow.Identifier, blockCount),
		t:           t,
	}
	follower.AddOnBlockFinalizedConsumer(fm.onBlockFinalizedConsumer)
	return fm, nil
}

func (fm *followerManager) startFollower(ctx context.Context) {
	go func() {
		fm.follower.Run(ctx)
	}()
	// wait for the follower to have completely started
	unittest.RequireCloseBefore(fm.t, fm.follower.Ready(), 10*time.Second,
		"timed out while waiting for consensus follower to start")
}

func (fm *followerManager) onBlockFinalizedConsumer(block *model.Block) {
	// push the finalized block ID to the blockIDChannel channel
	fm.blockIDChan <- block.BlockID
}

// getBlock checks if the underlying storage of the consensus follower has a block
func (fm *followerManager) getBlock(blockID flow.Identifier) (*flow.Block, error) {
	// get the underlying storage that the follower is using
	store := fm.follower.Storage
	require.NotNil(fm.t, store)
	blocks := store.Blocks
	require.NotNil(fm.t, blocks)
	return blocks.ByID(blockID)
}
