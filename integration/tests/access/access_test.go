package access

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/onflow/flow-go/consensus/hotstuff/committees"
	"github.com/onflow/flow-go/consensus/hotstuff/signature"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
	accessproto "github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/entities"
)

func TestAccess(t *testing.T) {
	suite.Run(t, new(AccessSuite))
}

type AccessSuite struct {
	suite.Suite

	log zerolog.Logger

	// root context for the current test
	ctx    context.Context
	cancel context.CancelFunc

	net *testnet.FlowNetwork
}

func (s *AccessSuite) TearDownTest() {
	s.log.Info().Msg("================> Start TearDownTest")
	s.net.Remove()
	s.cancel()
	s.log.Info().Msg("================> Finish TearDownTest")
}

func (s *AccessSuite) SetupTest() {
	s.log = unittest.LoggerForTest(s.Suite.T(), zerolog.InfoLevel)
	s.log.Info().Msg("================> SetupTest")
	defer func() {
		s.log.Info().Msg("================> Finish SetupTest")
	}()

	nodeConfigs := []testnet.NodeConfig{
		testnet.NewNodeConfig(flow.RoleAccess, testnet.WithLogLevel(zerolog.InfoLevel)),
	}

	// need one dummy execution node (unused ghost)
	exeConfig := testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.FatalLevel), testnet.AsGhost())
	nodeConfigs = append(nodeConfigs, exeConfig)

	// need one dummy verification node (unused ghost)
	verConfig := testnet.NewNodeConfig(flow.RoleVerification, testnet.WithLogLevel(zerolog.FatalLevel), testnet.AsGhost())
	nodeConfigs = append(nodeConfigs, verConfig)

	// need one controllable collection node (unused ghost)
	collConfig := testnet.NewNodeConfig(flow.RoleCollection, testnet.WithLogLevel(zerolog.FatalLevel), testnet.AsGhost())
	nodeConfigs = append(nodeConfigs, collConfig)

	// need three consensus nodes (unused ghost)
	for n := 0; n < 3; n++ {
		conID := unittest.IdentifierFixture()
		nodeConfig := testnet.NewNodeConfig(flow.RoleConsensus,
			testnet.WithLogLevel(zerolog.FatalLevel),
			testnet.WithID(conID),
			testnet.AsGhost())
		nodeConfigs = append(nodeConfigs, nodeConfig)
	}

	conf := testnet.NewNetworkConfig("access_api_test", nodeConfigs)
	s.net = testnet.PrepareFlowNetwork(s.T(), conf, flow.Localnet)

	// start the network
	s.T().Logf("starting flow network with docker containers")
	s.ctx, s.cancel = context.WithCancel(context.Background())

	s.net.Start(s.ctx)
}

func (s *AccessSuite) TestAPIsAvailable() {

	s.T().Run("TestHTTPProxyPortOpen", func(t *testing.T) {
		httpProxyAddress := s.net.ContainerByName(testnet.PrimaryAN).Addr(testnet.GRPCWebPort)

		conn, err := net.DialTimeout("tcp", httpProxyAddress, 1*time.Second)
		require.NoError(s.T(), err, "http proxy port not open on the access node")

		conn.Close()
	})

	s.T().Run("TestAccessConnection", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(s.ctx, 1*time.Second)
		defer cancel()

		grpcAddress := s.net.ContainerByName(testnet.PrimaryAN).Addr(testnet.GRPCPort)
		conn, err := grpc.DialContext(ctx, grpcAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
		require.NoError(t, err, "failed to connect to access node")
		defer conn.Close()

		client := accessproto.NewAccessAPIClient(conn)

		_, err = client.Ping(s.ctx, &accessproto.PingRequest{})
		assert.NoError(t, err, "failed to ping access node")
	})
}

// TestSignerIndicesDecoding tests that access node uses signer indices' decoder to correctly parse encoded data in blocks.
// This test receives blocks from consensus follower and then requests same blocks from access API and checks if returned data
// matches.
func (s *ConsensusFollowerSuite) TestSignerIndicesDecoding() {
	// create committee so we can create decoder to assert validity of data
	committee, err := committees.NewConsensusCommittee(s.followerMgr1.follower.State, s.stakedID)
	require.NoError(s.T(), err)
	blockSignerDecoder := signature.NewBlockSignerDecoder(committee)
	assertSignerIndicesValidity := func(msg *entities.BlockHeader) {
		block, err := convert.MessageToBlockHeader(msg)
		require.NoError(s.T(), err)
		decodedIdentities, err := blockSignerDecoder.DecodeSignerIDs(block)
		require.NoError(s.T(), err)
		// transform to assert
		var transformed [][]byte
		for _, identity := range decodedIdentities {
			identity := identity
			transformed = append(transformed, identity[:])
		}
		assert.ElementsMatch(s.T(), transformed, msg.ParentVoterIds, "response must contain correctly encoded signer IDs")
	}

	ctx, cancel := context.WithCancel(s.ctx)
	defer cancel()

	// kick off the first follower
	s.followerMgr1.startFollower(ctx)

	var finalizedBlockID flow.Identifier
	select {
	case <-time.After(30 * time.Second):
		require.Fail(s.T(), "expect to receive finalized block before timeout")
	case finalizedBlockID = <-s.followerMgr1.blockIDChan:
	}

	finalizedBlock, err := s.followerMgr1.getBlock(finalizedBlockID)
	require.NoError(s.T(), err)

	// create access API

	grpcAddress := s.net.ContainerByID(s.stakedID).Addr(testnet.GRPCPort)
	conn, err := grpc.DialContext(ctx, grpcAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(s.T(), err, "failed to connect to access node")
	defer conn.Close()

	client := accessproto.NewAccessAPIClient(conn)

	blockByID, err, cancel := makeApiRequest(client.GetBlockHeaderByID, ctx, &accessproto.GetBlockHeaderByIDRequest{Id: finalizedBlockID[:]})
	require.NoError(s.T(), err)
	defer cancel()

	blockByHeight, err, cancel := makeApiRequest(client.GetBlockHeaderByHeight, ctx,
		&accessproto.GetBlockHeaderByHeightRequest{Height: finalizedBlock.Header.Height})
	defer cancel()

	if err != nil {
		// could be that access node didn't process finalized block yet, add a small delay and try again
		time.Sleep(time.Second)
		blockByHeight, err, cancel = makeApiRequest(client.GetBlockHeaderByHeight, ctx,
			&accessproto.GetBlockHeaderByHeightRequest{Height: finalizedBlock.Header.Height})
		require.NoError(s.T(), err)
		defer cancel()
	}

	require.Equal(s.T(), blockByID, blockByHeight)
	require.Equal(s.T(), blockByID.Block.ParentVoterIndices, finalizedBlock.Header.ParentVoterIndices)
	assertSignerIndicesValidity(blockByID.Block)

	latestBlockHeader, err, cancel := makeApiRequest(client.GetLatestBlockHeader, ctx, &accessproto.GetLatestBlockHeaderRequest{})
	require.NoError(s.T(), err)
	defer cancel()
	assertSignerIndicesValidity(latestBlockHeader.Block)
}

// makeApiRequest is a helper function that encapsulates context creation for grpc client call, used to avoid repeated creation
// of new context for each call.
func makeApiRequest[Func func(context.Context, *Req, ...grpc.CallOption) (*Resp, error), Req any, Resp any](apiCall Func, ctx context.Context, req *Req) (*Resp, error, context.CancelFunc) {
	clientCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
	resp, err := apiCall(clientCtx, req)
	return resp, err, cancel
}
