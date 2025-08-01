package cohort4

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

	accessproto "github.com/onflow/flow/protobuf/go/flow/access"

	"github.com/onflow/flow-go/consensus/hotstuff/committees"
	"github.com/onflow/flow-go/consensus/hotstuff/signature"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
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
		testnet.NewNodeConfig(flow.RoleCollection, testnet.WithLogLevel(zerolog.FatalLevel)),
		testnet.NewNodeConfig(flow.RoleCollection, testnet.WithLogLevel(zerolog.FatalLevel)),
		testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.FatalLevel)),
		testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.FatalLevel)),
		testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithLogLevel(zerolog.FatalLevel)),
		testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithLogLevel(zerolog.FatalLevel)),
		testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithLogLevel(zerolog.FatalLevel)),
		testnet.NewNodeConfig(flow.RoleVerification, testnet.WithLogLevel(zerolog.FatalLevel)),
	}

	conf := testnet.NewNetworkConfig("access_api_test", nodeConfigs)
	s.net = testnet.PrepareFlowNetwork(s.T(), conf, flow.Localnet)

	// start the network
	s.T().Logf("starting flow network with docker containers")
	s.ctx, s.cancel = context.WithCancel(context.Background())

	s.net.Start(s.ctx)
}

func (s *AccessSuite) TestAllTheThings() {
	s.runTestAPIsAvailable()

	// run this test last because it stops the container
	s.runTestSignerIndicesDecoding()
}

func (s *AccessSuite) runTestAPIsAvailable() {
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
		conn, err := grpc.NewClient(grpcAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
		require.NoError(t, err, "failed to connect to access node")
		defer conn.Close()

		client := accessproto.NewAccessAPIClient(conn)

		_, err = client.Ping(ctx, &accessproto.PingRequest{})
		assert.NoError(t, err, "failed to ping access node")
	})
}

// runTestSignerIndicesDecoding tests that access node uses signer indices' decoder to correctly parse encoded data in blocks.
// This test receives blocks from consensus follower and then requests same blocks from access API and checks if returned data
// matches.
// CAUTION: must be run last if running multiple tests using the same network since it stops the containers.
func (s *AccessSuite) runTestSignerIndicesDecoding() {
	container := s.net.ContainerByName(testnet.PrimaryAN)

	ctx, cancel := context.WithCancel(s.ctx)
	defer cancel()

	// create access API
	grpcAddress := container.Addr(testnet.GRPCPort)
	conn, err := grpc.NewClient(grpcAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(s.T(), err, "failed to connect to access node")
	defer conn.Close()

	client := accessproto.NewAccessAPIClient(conn)

	// query latest finalized block. wait until at least two blocks have been finalized.
	// otherwise, we may get the root block which does not have any voter indices or its
	// immediate child who's parent voter indices are empty.
	var latestFinalizedBlock *accessproto.BlockHeaderResponse
	require.Eventually(s.T(), func() bool {
		latestFinalizedBlock, err = MakeApiRequest(client.GetLatestBlockHeader, ctx, &accessproto.GetLatestBlockHeaderRequest{
			IsSealed: false,
		})
		require.NoError(s.T(), err)
		return latestFinalizedBlock.GetBlock().Height > 1
	}, 30*time.Second, 100*time.Millisecond)

	// verify we get the same block when querying by ID and height
	blockByID, err := MakeApiRequest(client.GetBlockHeaderByID, ctx, &accessproto.GetBlockHeaderByIDRequest{Id: latestFinalizedBlock.Block.Id})
	require.NoError(s.T(), err)
	require.Equal(s.T(), latestFinalizedBlock, blockByID, "expect to receive same block by ID")

	blockByHeight, err := MakeApiRequest(client.GetBlockHeaderByHeight, ctx, &accessproto.GetBlockHeaderByHeightRequest{Height: latestFinalizedBlock.Block.Height})
	require.NoError(s.T(), err)
	require.Equal(s.T(), latestFinalizedBlock, blockByHeight, "expect to receive same block by height")

	// stop container, so we can access it's state and perform assertions
	err = s.net.StopContainerByName(ctx, testnet.PrimaryAN)
	require.NoError(s.T(), err)

	err = container.WaitForContainerStopped(5 * time.Second)
	require.NoError(s.T(), err)

	// open state to build a block singer decoder
	state, err := container.OpenState()
	require.NoError(s.T(), err)

	// create committee so we can create decoder to assert validity of data
	committee, err := committees.NewConsensusCommittee(state, container.Config.NodeID)
	require.NoError(s.T(), err)
	blockSignerDecoder := signature.NewBlockSignerDecoder(committee)

	expectedFinalizedBlock, err := state.AtBlockID(flow.HashToID(latestFinalizedBlock.Block.Id)).Head()
	require.NoError(s.T(), err)

	// since all blocks should be equal we will execute just check on one of them
	require.Equal(s.T(), latestFinalizedBlock.Block.ParentVoterIndices, expectedFinalizedBlock.ParentVoterIndices)

	// check if the response contains valid encoded signer IDs.
	msg := latestFinalizedBlock.Block
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

// MakeApiRequest is a helper function that encapsulates context creation for grpc client call, used to avoid repeated creation
// of new context for each call.
func MakeApiRequest[Func func(context.Context, *Req, ...grpc.CallOption) (*Resp, error), Req any, Resp any](apiCall Func, ctx context.Context, req *Req) (*Resp, error) {
	clientCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
	resp, err := apiCall(clientCtx, req)
	cancel()
	return resp, err
}
