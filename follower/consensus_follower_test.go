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
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/local"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/utils/unittest"
)

type Suite struct {
	suite.Suite
	state            *protocol.State
	snapshot         *protocol.Snapshot
	log              zerolog.Logger
	net              *module.Network
	bootDir          string
	stakedANNodeInfo bootstrap.NodeInfo
}

func TestConsensusFollower(t *testing.T) {
	suite.Run(t, new(Suite))
}

func (suite *Suite) SetupTest() {
	suite.log = zerolog.New(os.Stderr)

	suite.bootDir = unittest.TempDir(suite.T())

	nodeInfos := unittest.PrivateNodeInfosFixture(1, unittest.WithRole(flow.RoleAccess))
	suite.stakedANNodeInfo = nodeInfos[0]
	identity := suite.stakedANNodeInfo.Identity()

	otherParticipants := unittest.IdentityListFixture(4, unittest.WithAllRolesExcept(flow.RoleAccess), unittest.WithRandomPublicKeys())
	allParticipants := append(otherParticipants, identity)
	snapshot := unittest.RootSnapshotFixture(allParticipants)
	qc, err := snapshot.QuorumCertificate()
	require.NoError(suite.T(), err)
	h, err := snapshot.Head()
	require.NoError(suite.T(), err)
	qc.View = h.View

	rootSnapshotPath := filepath.Join(suite.bootDir, bootstrap.PathRootProtocolStateSnapshot)
	err = unittest.WriteJSON(rootSnapshotPath, snapshot.Encodable())
	require.NoError(suite.T(), err)
}

func (suite *Suite) TeardownTest() {
	defer os.RemoveAll(suite.bootDir)
}

func (suite *Suite) TestFollowerReceivesBlocks() {
	db, _ := unittest.TempBadgerDB(suite.T())
	baseOptions := node_builder.WithBaseOptions([]cmd.Option{cmd.WithDB(db),
		cmd.WithBindAddress("0.0.0.0:0"),
		cmd.WithBootstrapDir(suite.bootDir),
		cmd.WithMetricsEnabled(false)})
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
