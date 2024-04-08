package cohort3

import (
	"context"
	"fmt"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/engine/ghost/client"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/integration/tests/lib"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"

	accessproto "github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/entities"
)

func TestGrpcBlocksStream(t *testing.T) {
	suite.Run(t, new(GrpcBlocksStreamSuite))
}

type GrpcBlocksStreamSuite struct {
	suite.Suite
	lib.TestnetStateTracker

	log zerolog.Logger

	// root context for the current test
	ctx    context.Context
	cancel context.CancelFunc

	net *testnet.FlowNetwork

	// RPC methods to test
	testedRPCs func() []subscribeBlocksRPCTest

	ghostID flow.Identifier
}

func (s *GrpcBlocksStreamSuite) TearDownTest() {
	s.log.Info().Msg("================> Start TearDownTest")
	s.net.Remove()
	s.cancel()
	s.log.Info().Msg("================> Finish TearDownTest")
}

func (s *GrpcBlocksStreamSuite) SetupTest() {
	s.log = unittest.LoggerForTest(s.Suite.T(), zerolog.InfoLevel)
	s.log.Info().Msg("================> SetupTest")
	defer func() {
		s.log.Info().Msg("================> Finish SetupTest")
	}()

	// access node
	accessConfig := testnet.NewNodeConfig(
		flow.RoleAccess,
		testnet.WithLogLevel(zerolog.InfoLevel),
		testnet.WithAdditionalFlag("--execution-data-sync-enabled=true"),
		testnet.WithAdditionalFlagf("--execution-data-dir=%s", testnet.DefaultExecutionDataServiceDir),
		testnet.WithAdditionalFlag("--execution-data-retry-delay=1s"),
		testnet.WithAdditionalFlag("--execution-data-indexing-enabled=true"),
		testnet.WithAdditionalFlagf("--execution-state-dir=%s", testnet.DefaultExecutionStateDir),
		testnet.WithAdditionalFlag("--event-query-mode=local-only"),
		testnet.WithAdditionalFlag("--supports-observer=true"),
		testnet.WithAdditionalFlagf("--public-network-execution-data-sync-enabled=true"),
	)

	consensusConfigs := []func(config *testnet.NodeConfig){
		testnet.WithAdditionalFlag("--cruise-ctl-fallback-proposal-duration=400ms"),
		testnet.WithAdditionalFlag(fmt.Sprintf("--required-verification-seal-approvals=%d", 1)),
		testnet.WithAdditionalFlag(fmt.Sprintf("--required-construction-seal-approvals=%d", 1)),
		testnet.WithLogLevel(zerolog.FatalLevel),
	}

	// add the ghost (access) node config
	s.ghostID = unittest.IdentifierFixture()
	ghostNode := testnet.NewNodeConfig(
		flow.RoleAccess,
		testnet.WithID(s.ghostID),
		testnet.WithLogLevel(zerolog.FatalLevel),
		testnet.AsGhost())

	nodeConfigs := []testnet.NodeConfig{
		testnet.NewNodeConfig(flow.RoleCollection, testnet.WithLogLevel(zerolog.FatalLevel)),
		testnet.NewNodeConfig(flow.RoleCollection, testnet.WithLogLevel(zerolog.FatalLevel)),
		testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.FatalLevel)),
		testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.FatalLevel)),
		testnet.NewNodeConfig(flow.RoleConsensus, consensusConfigs...),
		testnet.NewNodeConfig(flow.RoleConsensus, consensusConfigs...),
		testnet.NewNodeConfig(flow.RoleConsensus, consensusConfigs...),
		testnet.NewNodeConfig(flow.RoleVerification, testnet.WithLogLevel(zerolog.FatalLevel)),
		accessConfig,
		ghostNode, // access ghost
	}

	// add the observer node config
	observers := []testnet.ObserverConfig{{
		ContainerName: testnet.PrimaryON,
		LogLevel:      zerolog.DebugLevel,
		AdditionalFlags: []string{
			fmt.Sprintf("--execution-data-dir=%s", testnet.DefaultExecutionDataServiceDir),
			fmt.Sprintf("--execution-state-dir=%s", testnet.DefaultExecutionStateDir),
			"--execution-data-sync-enabled=true",
			"--event-query-mode=execution-nodes-only",
			"--execution-data-indexing-enabled=true",
		},
	}}

	conf := testnet.NewNetworkConfig("access_blocks_streaming_test", nodeConfigs, testnet.WithObservers(observers...))
	s.net = testnet.PrepareFlowNetwork(s.T(), conf, flow.Localnet)

	// start the network
	s.T().Logf("starting flow network with docker containers")
	s.ctx, s.cancel = context.WithCancel(context.Background())

	s.testedRPCs = s.getRPCs

	s.net.Start(s.ctx)
	s.Track(s.T(), s.ctx, s.Ghost())
}

func (s *GrpcBlocksStreamSuite) Ghost() *client.GhostClient {
	client, err := s.net.ContainerByID(s.ghostID).GhostClient()
	require.NoError(s.T(), err, "could not get ghost client")
	return client
}

// TestRestEventStreaming tests gRPC event streaming
func (s *GrpcBlocksStreamSuite) TestHappyPath() {
	accessUrl := fmt.Sprintf("localhost:%s", s.net.ContainerByName(testnet.PrimaryAN).Port(testnet.GRPCPort))
	accessClient, err := getAccessAPIClient(accessUrl)
	s.Require().NoError(err)

	observerURL := fmt.Sprintf("localhost:%s", s.net.ContainerByName(testnet.PrimaryON).Port(testnet.GRPCPort))
	observerClient, err := getAccessAPIClient(observerURL)
	s.Require().NoError(err)

	// get the first block height
	currentFinalized := s.BlockState.HighestFinalizedHeight()
	blockA := s.BlockState.WaitForHighestFinalizedProgress(s.T(), currentFinalized)

	// Let the network run for this many blocks
	blockCount := uint64(5)
	// wait for the requested number of sealed blocks
	s.BlockState.WaitForSealed(s.T(), blockA.Header.Height+blockCount)

	txGenerator, err := s.net.ContainerByName(testnet.PrimaryAN).TestnetClient()
	s.Require().NoError(err)
	header, err := txGenerator.GetLatestSealedBlockHeader(s.ctx)
	s.Require().NoError(err)

	var startValue interface{}
	txCount := 10

	for _, rpc := range s.testedRPCs() {
		s.T().Run(rpc.name, func(t *testing.T) {
			if rpc.name == "SubscribeBlocksFromStartBlockID" {
				startValue = header.ID.Bytes()
			} else {
				startValue = header.Height
			}

			accessRecv := rpc.call(s.ctx, accessClient, startValue)
			accessBlocks, accessBlockErrs, err := SubscribeHandler(s.ctx, accessRecv, blockResponseHandler)
			s.Require().NoError(err)

			observerRecv := rpc.call(s.ctx, observerClient, startValue)
			observerBlocks, observerBlockErrs, err := SubscribeHandler(s.ctx, observerRecv, blockResponseHandler)
			s.Require().NoError(err)

			foundANTxCount := 0
			foundONTxCount := 0

			r := NewResponseTracker(compareBlocksResponse)

			for {
				select {
				case err := <-accessBlockErrs:
					s.Require().NoErrorf(err, "unexpected AN error")
				case err := <-observerBlockErrs:
					s.Require().NoErrorf(err, "unexpected ON error")
				case block := <-accessBlocks:
					s.T().Logf("AN block received: height: %d", block.Header.Height)
					r.Add(s.T(), block.Header.Height, "access", block)
					foundANTxCount++
				case block := <-observerBlocks:
					s.T().Logf("ON block received: height: %d", block.Header.Height)
					r.Add(s.T(), block.Header.Height, "observer", block)
					foundONTxCount++
				}

				if foundANTxCount >= txCount && foundONTxCount >= txCount {
					break
				}
			}
		})
	}
}

func blockResponseHandler(msg *accessproto.SubscribeBlocksResponse) (*flow.Block, error) {
	return convert.MessageToBlock(msg.GetBlock())
}

func compareBlocksResponse(t *testing.T, responses map[uint64]map[string]*flow.Block, blockHeight uint64) error {
	if len(responses[blockHeight]) != 2 {
		return nil
	}
	accessData := responses[blockHeight]["access"]
	observerData := responses[blockHeight]["observer"]

	// Compare access with observer
	err := compareBlocks(t, accessData, observerData)
	if err != nil {
		return fmt.Errorf("failure comparing access and observer data: %d: %v", blockHeight, err)
	}

	return nil
}

func compareBlocks(t *testing.T, accessBlock *flow.Block, observerBlock *flow.Block) error {
	require.Equal(t, accessBlock.ID(), observerBlock.ID())
	require.Equal(t, accessBlock.Header.Height, observerBlock.Header.Height)
	require.Equal(t, accessBlock.Header.Timestamp, observerBlock.Header.Timestamp)
	require.Equal(t, accessBlock.Payload.Hash(), observerBlock.Payload.Hash())

	return nil
}

type subscribeBlocksRPCTest struct {
	name string
	call func(ctx context.Context, client accessproto.AccessAPIClient, startValue interface{}) func() (*accessproto.SubscribeBlocksResponse, error)
}

func (s *GrpcBlocksStreamSuite) getRPCs() []subscribeBlocksRPCTest {
	return []subscribeBlocksRPCTest{
		{
			name: "SubscribeBlocksFromLatest",
			call: func(ctx context.Context, client accessproto.AccessAPIClient, _ interface{}) func() (*accessproto.SubscribeBlocksResponse, error) {
				stream, err := client.SubscribeBlocksFromLatest(ctx, &accessproto.SubscribeBlocksFromLatestRequest{
					BlockStatus:       entities.BlockStatus_BLOCK_FINALIZED,
					FullBlockResponse: true,
				})
				s.Require().NoError(err)
				return stream.Recv
			},
		},
		{
			name: "SubscribeBlocksFromStartBlockID",
			call: func(ctx context.Context, client accessproto.AccessAPIClient, startValue interface{}) func() (*accessproto.SubscribeBlocksResponse, error) {
				stream, err := client.SubscribeBlocksFromStartBlockID(ctx, &accessproto.SubscribeBlocksFromStartBlockIDRequest{
					StartBlockId:      startValue.([]byte),
					BlockStatus:       entities.BlockStatus_BLOCK_FINALIZED,
					FullBlockResponse: true,
				})
				s.Require().NoError(err)
				return stream.Recv
			},
		},
		{
			name: "SubscribeBlocksFromStartHeight",
			call: func(ctx context.Context, client accessproto.AccessAPIClient, startValue interface{}) func() (*accessproto.SubscribeBlocksResponse, error) {
				stream, err := client.SubscribeBlocksFromStartHeight(ctx, &accessproto.SubscribeBlocksFromStartHeightRequest{
					StartBlockHeight:  startValue.(uint64),
					BlockStatus:       entities.BlockStatus_BLOCK_FINALIZED,
					FullBlockResponse: true,
				})
				s.Require().NoError(err)
				return stream.Recv
			},
		},
	}
}

func getAccessAPIClient(address string) (accessproto.AccessAPIClient, error) {
	conn, err := grpc.Dial(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	return accessproto.NewAccessAPIClient(conn), nil
}
