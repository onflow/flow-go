package follower

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/cmd/access/node_builder"
	"github.com/onflow/flow-go/cmd/bootstrap/utils"
	"github.com/onflow/flow-go/crypto"
	channels "github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/local"
	mockmodule "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/test"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/utils/unittest"
)

type Suite struct {
	suite.Suite
	snapshot            protocol.Snapshot
	log                 zerolog.Logger
	bootDir             string
	stakedANNodeInfo    bootstrap.NodeInfo
	consensusNodeInfo   bootstrap.NodeInfo
	conensusNodeBuilder *cmd.FlowNodeBuilder
	allParticipants     flow.IdentityList
	stakedANIdentity    *flow.Identity
	consensusIdentity   *flow.Identity
	mockConsensus       *MockConsensus
	idProvider          *test.UpdatableIDProvider
	idTranslator        *p2p.FixedTableIdentityTranslator
	stakedAccessNode    *node_builder.StakedAccessNodeBuilder
	tmpDirs             []string
	follower            *consensusFollowerWithoutVerifer
	followerCancel      context.CancelFunc
}

func TestConsensusFollower(t *testing.T) {
	suite.Run(t, new(Suite))
}

func (suite *Suite) SetupTest() {
	suite.log = zerolog.New(os.Stderr)

	// generate node identities
	suite.generateNodeIdentities()
	// generate the root protocol state Json file containing the genesis data
	suite.generateGenesisSnapshot()
	// initialize the ID Providers used for the Access node and the consensus node
	suite.initIdentityProviders()

	// create the staked AN
	suite.createStakedAccessNode()

	// create the consensus node
	suite.createConsensusNode()

	// create the unstaked consensus follower
	suite.createFollower()

	for _, p := range suite.allParticipants {
		suite.log.Debug().Msg(p.String())
	}

	// update the ID provider with the  updated identities of the access node and consensus node which
	// now include the real port number they are running on
	suite.idProvider.SetIdentities(suite.allParticipants)

	head, err := suite.snapshot.Head()
	require.NoError(suite.T(), err)

	// create the mock consensus to generate test blocks
	suite.mockConsensus = &MockConsensus{
		consensusIdentity: suite.consensusIdentity,
		currentHeader:     head,
	}
}

func (suite *Suite) generateNodeIdentities() {
	// create one staked AN private node info and identity
	nodeInfos := unittest.PrivateNodeInfosFixture(1, unittest.WithRole(flow.RoleAccess))
	suite.stakedANNodeInfo = nodeInfos[0]
	suite.stakedANIdentity = suite.stakedANNodeInfo.Identity()

	// create one consensus node private node info and identity
	nodeInfos = unittest.PrivateNodeInfosFixture(1, unittest.WithRole(flow.RoleConsensus))
	suite.consensusNodeInfo = nodeInfos[0]
	suite.consensusIdentity = suite.consensusNodeInfo.Identity()

	// create identities for the rest of the nodes (node info not needed since only the access node and
	// consensus node will be run as real nodes)
	otherParticipants := unittest.IdentityListFixture(3, unittest.WithAllRolesExcept(flow.RoleAccess, flow.RoleConsensus), unittest.WithRandomPublicKeys())

	suite.allParticipants = append(otherParticipants, suite.stakedANIdentity, suite.consensusIdentity)
}

func (suite *Suite) generateGenesisSnapshot() {
	// include all identities in the root snapshot
	snapshot := unittest.RootSnapshotFixture(suite.allParticipants)
	qc, err := snapshot.QuorumCertificate()
	require.NoError(suite.T(), err)

	head, err := snapshot.Head()
	require.NoError(suite.T(), err)

	qc.View = head.View

	// create a boot directory shared by all nodes
	suite.bootDir = unittest.TempDir(suite.T())

	// write the root protocol Json file to the boot dir
	rootSnapshotPath := filepath.Join(suite.bootDir, bootstrap.PathRootProtocolStateSnapshot)
	err = unittest.WriteJSON(rootSnapshotPath, snapshot.Encodable())
	require.NoError(suite.T(), err)

	suite.snapshot = snapshot
}

func (suite *Suite) initIdentityProviders() {
	var err error
	suite.idProvider = test.NewUpdatableIDProvider(suite.allParticipants)
	suite.idTranslator, err = p2p.NewFixedTableIdentityTranslator(suite.allParticipants)
	require.NoError(suite.T(), err)
}

func (suite *Suite) createStakedAccessNode() {
	baseOptions := node_builder.WithBaseOptions(suite.nodeOptions())
	accessNodeOptions := node_builder.SupportsUnstakedNode(true)
	nodeBuilder := node_builder.NewStakedAccessNodeBuilder(node_builder.FlowAccessNode(baseOptions, accessNodeOptions))
	nodeInfoPriv, err := suite.stakedANNodeInfo.Private()
	require.NoError(suite.T(), err)
	nodeBuilder.Me, err = local.New(suite.stakedANNodeInfo.Identity(), nodeInfoPriv.StakingPrivKey)
	require.NoError(suite.T(), err)
	nodeBuilder.NodeConfig.NodeID = suite.stakedANNodeInfo.NodeID
	nodeBuilder.NodeID = suite.stakedANNodeInfo.NodeID
	nodeBuilder.NodeConfig.NetworkKey = nodeInfoPriv.NetworkPrivKey
	nodeBuilder.NodeConfig.StakingKey = nodeInfoPriv.StakingPrivKey
	nodeBuilder.Initialize()
	nodeBuilder.Build()
	consensusFollowerWithMockVerifier(nodeBuilder.FlowAccessNodeBuilder) // use the consensus follower without the verifier
	unittest.RequireCloseBefore(suite.T(), nodeBuilder.Ready(), 5*time.Second, "failed to start the access node")
	suite.stakedAccessNode = nodeBuilder
	// now that the node is running, the node builder should have the real port that was chosen to bind by the network
	host, port := suite.getAddress(nodeBuilder.FlowNodeBuilder)
	// update the identity of the node with the real address
	suite.stakedANIdentity.Address = fmt.Sprintf("%s:%s", host, port)
}

// createConsensusNode creates a consensus node minus all the engines
func (suite *Suite) createConsensusNode() {
	options := suite.nodeOptions()
	nodeBuilder := cmd.FlowNode(flow.RoleConsensus.String(), options...)
	nodeInfoPriv, err := suite.consensusNodeInfo.Private()
	require.NoError(suite.T(), err)
	nodeBuilder.Me, err = local.New(suite.consensusIdentity, nodeInfoPriv.StakingPrivKey)
	require.NoError(suite.T(), err)
	nodeBuilder.NodeConfig.NodeID = suite.consensusIdentity.NodeID
	nodeBuilder.NodeID = suite.consensusIdentity.NodeID
	nodeBuilder.NodeConfig.NetworkKey = nodeInfoPriv.NetworkPrivKey
	nodeBuilder.NodeConfig.StakingKey = nodeInfoPriv.StakingPrivKey
	nodeBuilder.InitIDProviders()
	nodeBuilder.EnqueueNetworkInit(context.Background())
	nodeBuilder.EnqueueTracer()
	unittest.RequireCloseBefore(suite.T(), nodeBuilder.Ready(), 5*time.Second, "failed to start the consensus node")
	suite.conensusNodeBuilder = nodeBuilder
	//go func() {
	//	nodeBuilder.Run()
	//}()
	// now that the node is running, the node builder should have the real port that was chosen to bind by the network
	host, port := suite.getAddress(nodeBuilder)
	// update the identity of the node with the real address
	suite.consensusIdentity.Address = fmt.Sprintf("%s:%s", host, port)
}

// createFollower creates the unstaked follower
func (suite *Suite) createFollower() ConsensusFollower {
	//accessNodekey, err := p2p.LibP2PPublicKeyFromFlow(suite.stakedANIdentity.NetworkPubKey)
	//require.NoError(suite.T(), err)
	//id, err := peer.IDFromPublicKey(accessNodekey)
	//require.NoError(suite.T(), err)

	followerDB, tmpDir := unittest.TempBadgerDB(suite.T())
	suite.tmpDirs = append(suite.tmpDirs, tmpDir)

	// TODO: use keyutils
	followerKey, err := UnstakedNetworkingKey()
	require.NoError(suite.T(), err)

	host, portStr := suite.getAddress(suite.stakedAccessNode.FlowNodeBuilder)
	port, err := strconv.Atoi(portStr)
	require.NoError(suite.T(), err)
	bootstrapNodeInfo := BootstrapNodeInfo{
		Host:             host,
		Port:             uint(port),
		NetworkPublicKey: suite.stakedAccessNode.NodeConfig.NetworkKey.PublicKey(),
	}

	follower, err := newConsensusFollowerWithoutVerifier(followerKey, "0.0.0.0:0", []BootstrapNodeInfo{bootstrapNodeInfo},
		WithBootstrapDir(suite.bootDir), WithDB(followerDB))
	follower.NodeBuilder.MetricsEnabled = false
	require.NoError(suite.T(), err)

	follower.AddOnBlockFinalizedConsumer(suite.OnBlockFinalizedConsumer)

	// get the underlying node builder
	node := follower.NodeBuilder
	// wait for the follower to have completely started
	unittest.RequireCloseBefore(suite.T(), node.Ready(), 10*time.Second,
		"timed out while waiting for consensus follower to start")

	//go func() {
	//	follower.Run(context.Background())
	//}()
	//
	return follower
}

func (suite *Suite) nodeOptions() []cmd.Option {
	// create a temp db for each node
	db, tmpDir := unittest.TempBadgerDB(suite.T())
	suite.tmpDirs = append(suite.tmpDirs, tmpDir)
	return []cmd.Option{cmd.WithDB(db), // // create a temp db for each node
		cmd.WithBindAddress("0.0.0.0:0"),     // choose a local port
		cmd.WithBootstrapDir(suite.bootDir),  // use the shared bootstrap directory
		cmd.WithMetricsEnabled(false),        // disable metrics
		cmd.WithIDProvider(suite.idProvider), // use the shared ID provider and translator
		cmd.WithIDTranslator(suite.idTranslator)}
}

func (suite *Suite) TeardownTest() {

	// stop the follower
	if suite.followerCancel != nil {
		suite.followerCancel()
	}

	// stop the staked nodes
	unittest.RequireCloseBefore(suite.T(), suite.conensusNodeBuilder.Done(), 5*time.Second, "failed to stop the consensus node")
	unittest.RequireCloseBefore(suite.T(), suite.stakedAccessNode.Done(), 5*time.Second, "failed to stop the access node")

	// delete all temp dirs
	for _, d := range suite.tmpDirs {
		os.RemoveAll(d)
	}
	os.RemoveAll(suite.bootDir)
}

func (suite *Suite) TestFollowerReceivesBlocks() {

	//k, err := p2p.LibP2PPublicKeyFromFlow(suite.stakedANIdentity.NetworkPubKey)
	//require.NoError(suite.T(), err)
	//id, err := peer.IDFromPublicKey(k)
	//require.NoError(suite.T(), err)
	//fmt.Println(id.String())

	follower := suite.createFollower()
	//ctx, cancel := context.WithCancel(context.Background())
	go func() {
		follower.Run(context.Background())
	}()

	conduit, err := suite.conensusNodeBuilder.Network.Register(channels.PushBlocks, new(mockmodule.Engine))
	require.NoError(suite.T(), err)

	// TODO: remove the below lines since the staked AN should automatically connect to the consensus node,
	// without the need of an explicit AddPeer. Should work after the PeerManager is re-introduced back into the
	// staked AN
	pInfo, err := p2p.PeerAddressInfo(*suite.consensusIdentity)
	require.NoError(suite.T(), err)
	err = suite.stakedAccessNode.LibP2PNode.AddPeer(context.Background(), pInfo)
	require.NoError(suite.T(), err)
	middleware, ok := suite.stakedAccessNode.Middleware.(*p2p.Middleware)
	if !ok {
		suite.Fail("unexpected type of middleware")
	}
	connected, err := middleware.IsConnected(suite.consensusIdentity.NodeID)
	require.NoError(suite.T(), err)
	require.True(suite.T(), connected)

	head, err := suite.snapshot.Head()
	require.NoError(suite.T(), err)
	topic := channels.TopicFromChannel(channels.PushBlocks, head.ID().String())
	require.Eventually(suite.T(),
		func() bool {
			fmt.Println(suite.stakedAccessNode.LibP2PNode.PubSub().ListPeers(topic.String()))
			return len(suite.stakedAccessNode.LibP2PNode.PubSub().ListPeers(topic.String())) >= 2 },
	time.Second * 5, time.Millisecond * 100)


	//connected, err = middleware.IsConnected(suite.follower.NodeBuilder.NodeID)
	//require.NoError(suite.T(), err)
	//require.True(suite.T(), connected)
	//time.Sleep(5 * time.Second)

	// determine the nodes we should send the block to
	//recipients, err := suite.snapshot.Identities(filter.And(
	//	filter.Not(filter.Ejected),
	//	filter.Not(filter.HasRole(flow.RoleConsensus)),
	//))
	//require.NoError(suite.T(), err)

	//head, err := suite.snapshot.Head()
	//require.NoError(suite.T(), err)
	//extend := unittest.BlockWithParentFixture(head)
	//extend.Payload.Guarantees = nil
	//extend.Header.PayloadHash = extend.Payload.Hash()

	block := suite.mockConsensus.extendBlock(suite.mockConsensus.currentHeader.View + 1)
	blockProposal := unittest.ProposalFromBlock(block)

	//h, _ := suite.snapshot.Head()
	//blk := model.BlockFromFlow(block.Header, h.View)
	//suite.stakedAccessNode.FlowAccessNodeBuilder.IngestEng.OnFinalizedBlock(blk)
	err = conduit.Publish(blockProposal, suite.stakedANIdentity.NodeID)
	require.NoError(suite.T(), err)

	block = suite.mockConsensus.extendBlock(suite.mockConsensus.currentHeader.View + 1)
	blockProposal = unittest.ProposalFromBlock(block)
	err = conduit.Publish(blockProposal, suite.stakedANIdentity.NodeID)
	require.NoError(suite.T(), err)

	require.Eventually(suite.T(), func() bool {
		_, err := suite.stakedAccessNode.Storage.Blocks.ByID(block.Header.ID())
		return err == nil
	}, 2 * time.Second, 100 * time.Millisecond)

	block = suite.mockConsensus.extendBlock(suite.mockConsensus.currentHeader.View + 1)
	blockProposal = unittest.ProposalFromBlock(block)
	err = conduit.Publish(blockProposal, suite.stakedANIdentity.NodeID)
	require.NoError(suite.T(), err)

	require.Eventually(suite.T(), func() bool {
		_, err := suite.stakedAccessNode.Storage.Blocks.ByID(block.Header.ID())
		return err == nil
	}, 2 * time.Second, 100 * time.Millisecond)

	block = suite.mockConsensus.extendBlock(suite.mockConsensus.currentHeader.View + 1)
	blockProposal = unittest.ProposalFromBlock(block)
	err = conduit.Publish(blockProposal, suite.stakedANIdentity.NodeID)
	require.NoError(suite.T(), err)

	require.Eventually(suite.T(), func() bool {
		_, err := suite.stakedAccessNode.Storage.Blocks.ByID(block.Header.ID())
		return err == nil
	}, 2 * time.Second, 100 * time.Millisecond)

	block = suite.mockConsensus.extendBlock(suite.mockConsensus.currentHeader.View + 5)
	blockProposal = unittest.ProposalFromBlock(block)
	err = conduit.Publish(blockProposal, suite.stakedANIdentity.NodeID)
	require.NoError(suite.T(), err)
	time.Sleep(5 * time.Second)

	//block1, err := suite.stakedAccessNode.Storage.Blocks.ByID(block.ID())
	//fmt.Println(err)
	//fmt.Println(block1.ID())
	time.Sleep(time.Hour)
	//blk, err := nodeBuilder.Storage.Blocks.ByID(blockProposal.Header.ID())
	//require.NoError(suite.T(), err)
	//fmt.Println(blk.ID())

}

func (suite *Suite) OnBlockFinalizedConsumer(finalizedBlockID flow.Identifier) {
	fmt.Println(" >>>>>> " + finalizedBlockID.String())
}

func UnstakedNetworkingKey() (crypto.PrivateKey, error) {
	seed := make([]byte, crypto.KeyGenSeedMinLenECDSASecp256k1)
	n, err := rand.Read(seed)
	if err != nil || n != crypto.KeyGenSeedMinLenECDSASecp256k1 {
		return nil, err
	}
	return utils.GenerateUnstakedNetworkingKey(unittest.SeedFixture(n))
}

func (suite *Suite) getAddress(nodeBuilder *cmd.FlowNodeBuilder) (string, string) {
	middleware, ok := nodeBuilder.Middleware.(*p2p.Middleware)
	if !ok {
		suite.Fail("unexpected type of middleware")
	}
	ip, port, err := middleware.GetIPPort()
	require.NoError(suite.T(), err)
	return ip, port
}

type MockConsensus struct {
	consensusIdentity *flow.Identity
	currentHeader     *flow.Header
}

func (mc *MockConsensus) extendBlock(blockView uint64) *flow.Block {
	nextBlock := unittest.BlockWithParentFixture(mc.currentHeader)
	nextBlock.Header.View = blockView
	nextBlock.Header.ProposerID = mc.consensusIdentity.NodeID
	nextBlock.Header.ParentVoterIDs = flow.IdentifierList{mc.consensusIdentity.NodeID}
	mc.currentHeader = nextBlock.Header
	return &nextBlock
}
