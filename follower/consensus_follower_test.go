package follower

import (
	"os"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/cmd/access/node_builder"
	"github.com/onflow/flow-go/cmd/bootstrap/utils"
	"github.com/onflow/flow-go/crypto"
	channels "github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/id"
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
	net                 *module.Network
	bootDir             string
	stakedANNodeInfo    bootstrap.NodeInfo
	consensusNodeInfo   bootstrap.NodeInfo
	conensusNodeBuilder *cmd.FlowNodeBuilder
	allParticipants flow.IdentityList
	stakedANIdentity *flow.Identity
	consensusIdentity *flow.Identity

}

func TestConsensusFollower(t *testing.T) {
	suite.Run(t, new(Suite))
}

func (suite *Suite) SetupTest() {
	suite.log = zerolog.New(os.Stderr)

	suite.bootDir = unittest.TempDir(suite.T())

	nodeInfos := unittest.PrivateNodeInfosFixture(1, unittest.WithRole(flow.RoleAccess))
	suite.stakedANNodeInfo = nodeInfos[0]
	suite.stakedANIdentity = suite.stakedANNodeInfo.Identity()

	nodeInfos = unittest.PrivateNodeInfosFixture(1, unittest.WithRole(flow.RoleConsensus))
	suite.consensusNodeInfo = nodeInfos[0]
	suite.consensusIdentity = suite.consensusNodeInfo.Identity()

	otherParticipants := unittest.IdentityListFixture(3, unittest.WithAllRolesExcept(flow.RoleAccess, flow.RoleConsensus), unittest.WithRandomPublicKeys())
	suite.allParticipants = append(otherParticipants, suite.stakedANIdentity, suite.consensusIdentity)
	snapshot := unittest.RootSnapshotFixture(suite.allParticipants)
	qc, err := snapshot.QuorumCertificate()
	require.NoError(suite.T(), err)
	h, err := snapshot.Head()
	require.NoError(suite.T(), err)
	qc.View = h.View
	rootSnapshotPath := filepath.Join(suite.bootDir, bootstrap.PathRootProtocolStateSnapshot)
	err = unittest.WriteJSON(rootSnapshotPath, snapshot.Encodable())
	require.NoError(suite.T(), err)
	suite.snapshot = snapshot
}

func (suite *Suite) TestFollowerReceivesBlocks() {

	idProvider := test.NewUpdatableIDProvider(flow.IdentityList{})

	db, _ := unittest.TempBadgerDB(suite.T())
	baseOptions := node_builder.WithBaseOptions([]cmd.Option{cmd.WithDB(db),
		cmd.WithBindAddress("0.0.0.0:0"),
		cmd.WithBootstrapDir(suite.bootDir),
		cmd.WithMetricsEnabled(false),
	cmd.WithIDProvider(idProvider)})
	accessNodeOptions := node_builder.SupportsUnstakedNode(true)
	nodeBuilder := node_builder.NewStakedAccessNodeBuilder(node_builder.FlowAccessNode(baseOptions, accessNodeOptions))
	nodeInfoPriv, err := suite.stakedANNodeInfo.Private()
	require.NoError(suite.T(), err)
	nodeBuilder.Me, err = local.New(suite.stakedANNodeInfo.Identity(), nodeInfoPriv.StakingPrivKey)
	nodeBuilder.NodeConfig.NodeID = suite.stakedANNodeInfo.NodeID
	nodeBuilder.NodeID = suite.stakedANNodeInfo.NodeID
	nodeBuilder.NodeConfig.NetworkKey = nodeInfoPriv.NetworkPrivKey
	nodeBuilder.NodeConfig.StakingKey = nodeInfoPriv.StakingPrivKey
	nodeBuilder.Initialize()
	nodeBuilder.Build()
	<-nodeBuilder.Ready()
	h, err := nodeBuilder.State.Sealed().Head()
	require.NoError(suite.T(), err)
	fmt.Println(h.Height)
	host, portStr, err := nodeBuilder.LibP2PNode.GetIPPort()
	require.NoError(suite.T(), err)
	port, err := strconv.Atoi(portStr)
	require.NoError(suite.T(), err)


	suite.stakedANIdentity.Address = suite.getAddress(nodeBuilder.FlowNodeBuilder)

	followerDB, _ := unittest.TempBadgerDB(suite.T())

	followerKey, err := UnstakedNetworkingKey()
	require.NoError(suite.T(), err)

	bootstrapNodeInfo := BootstrapNodeInfo{
		Host:             host,
		Port:             uint(port),
		NetworkPublicKey: nodeBuilder.NodeConfig.NetworkKey.PublicKey(),
	}

	follower, err := NewConsensusFollower(followerKey, "0.0.0.0:0", []BootstrapNodeInfo{bootstrapNodeInfo},
		WithBootstrapDir(suite.bootDir), WithDB(followerDB))
	follower.NodeBuilder.MetricsEnabled = false

	require.NoError(suite.T(), err)

	follower.AddOnBlockFinalizedConsumer(suite.OnBlockFinalizedConsumer)

	// get the underlying node builder
	node := follower.NodeBuilder
	// wait for the follower to have completely started
	unittest.RequireCloseBefore(suite.T(), node.Ready(), 10*time.Second,
		"timed out while waiting for consensus follower to start")

	go func() {
		follower.Run(context.Background())
	}()

	suite.conensusNodeBuilder = suite.createConsensusNode(idProvider)
	suite.consensusIdentity.Address = suite.getAddress(suite.conensusNodeBuilder)

	for _, i := range suite.allParticipants {
		fmt.Println(i.String())
	}

	conduit, err := suite.conensusNodeBuilder.Network.Register(channels.PushBlocks, new(mockmodule.Engine))
	require.NoError(suite.T(), err)

	// determine the nodes we should send the block to
	recipients, err := suite.snapshot.Identities(filter.And(
		filter.Not(filter.Ejected),
		filter.Not(filter.HasRole(flow.RoleConsensus)),
	))
	require.NoError(suite.T(), err)

	blockProposal := unittest.ProposalFixture()
	err = conduit.Publish(blockProposal, recipients.NodeIDs()...)
	require.NoError(suite.T(), err)
	time.Sleep(1 * time.Minute)

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

func (suite *Suite) createConsensusNode(idProvider id.IdentityProvider) *cmd.FlowNodeBuilder {
	db, _ := unittest.TempBadgerDB(suite.T())
	options := []cmd.Option{cmd.WithDB(db),
		cmd.WithBindAddress("0.0.0.0:0"),
		cmd.WithBootstrapDir(suite.bootDir),
		cmd.WithMetricsEnabled(false),
		cmd.WithIDProvider(idProvider)}
	nodeBuilder := cmd.FlowNode(flow.RoleConsensus.String(), options...)
	nodeInfoPriv, err := suite.consensusNodeInfo.Private()
	require.NoError(suite.T(), err)
	nodeBuilder.Me, err = local.New(suite.stakedANNodeInfo.Identity(), nodeInfoPriv.StakingPrivKey)
	require.NoError(suite.T(), err)
	nodeBuilder.NodeConfig.NodeID = suite.stakedANNodeInfo.NodeID
	nodeBuilder.NodeID = suite.stakedANNodeInfo.NodeID
	nodeBuilder.NodeConfig.NetworkKey = nodeInfoPriv.NetworkPrivKey
	nodeBuilder.NodeConfig.StakingKey = nodeInfoPriv.StakingPrivKey
	nodeBuilder.InitIDProviders()
	nodeBuilder.EnqueueNetworkInit(context.Background())
	nodeBuilder.EnqueueTracer()
	<-nodeBuilder.Ready()
	go func() {
		nodeBuilder.Run()
	}()
	return nodeBuilder
}


func (suite *Suite) getAddress(nodeBuilder *cmd.FlowNodeBuilder) string {
	middleware, ok := nodeBuilder.Middleware.(*p2p.Middleware)
	if !ok {
		suite.Fail("unexpected type of middleware")
	}
	ip, port, err := middleware.GetIPPort()
	require.NoError(suite.T(), err)
	return fmt.Sprintf("%s:%s",ip, port)
}
