package cohort4

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/onflow/flow-go/engine/access/rest/http/request"
	"github.com/onflow/flow-go/engine/access/state_stream/backend"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/engine/ghost/client"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/integration/tests/access/common"
	"github.com/onflow/flow-go/integration/tests/lib"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"

	accessproto "github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/entities"
)

func TestStreamingAPIs(t *testing.T) {
	suite.Run(t, new(StreamingSuite))
}

// StreamingSuite tests both gRPC block streaming and REST event streaming APIs.
type StreamingSuite struct {
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

func (s *StreamingSuite) TearDownTest() {
	s.log.Info().Msg("================> Start TearDownTest")
	s.net.Remove()
	s.cancel()
	s.log.Info().Msg("================> Finish TearDownTest")
}

// SetupTest initializes the test suite with a network configuration for testing both GRPC block streaming and REST event streaming APIs.
//
// Network Configuration:
//   - 1 Access node (with execution data indexing and streaming support)
//   - 1 Ghost Access node (lightweight, for tracking block state)
//   - 1 Observer node with execution data indexing (event-query-mode=execution-nodes-only)
//   - 2 Collection nodes (standard configuration)
//   - 3 Consensus nodes (with custom timing: 100ms proposal duration, reduced seal approvals)
//   - 2 Execution nodes (standard configuration)
//   - 1 Verification node (standard configuration)
//
// This unified setup supports testing both GRPC block streaming APIs (SubscribeBlocksFromLatest,
// SubscribeBlocksFromStartBlockID, SubscribeBlocksFromStartHeight) and REST event streaming via websockets.
// The access node and observer both index execution data, allowing verification that streaming APIs
// return consistent results from both nodes.
func (s *StreamingSuite) SetupTest() {
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
		testnet.WithAdditionalFlag("--cruise-ctl-fallback-proposal-duration=100ms"),
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
		LogLevel:      zerolog.InfoLevel,
		AdditionalFlags: []string{
			fmt.Sprintf("--execution-data-dir=%s", testnet.DefaultExecutionDataServiceDir),
			fmt.Sprintf("--execution-state-dir=%s", testnet.DefaultExecutionStateDir),
			"--execution-data-sync-enabled=true",
			"--event-query-mode=execution-nodes-only",
			"--execution-data-indexing-enabled=true",
		},
	}}

	conf := testnet.NewNetworkConfig("streaming_test", nodeConfigs, testnet.WithObservers(observers...))
	s.net = testnet.PrepareFlowNetwork(s.T(), conf, flow.Localnet)

	// start the network
	s.T().Logf("starting flow network with docker containers")
	s.ctx, s.cancel = context.WithCancel(context.Background())

	s.testedRPCs = s.getRPCs

	s.net.Start(s.ctx)
	s.Track(s.T(), s.ctx, s.Ghost())
}

func (s *StreamingSuite) Ghost() *client.GhostClient {
	client, err := s.net.ContainerByID(s.ghostID).GhostClient()
	require.NoError(s.T(), err, "could not get ghost client")
	return client
}

// TestAllStreamingAPIs runs all streaming API tests with a shared network setup.
func (s *StreamingSuite) TestAllStreamingAPIs() {
	s.testGrpcBlocksStreaming()
	s.testRestEventStreaming()
}

// testGrpcBlocksStreaming tests gRPC block streaming APIs.
func (s *StreamingSuite) testGrpcBlocksStreaming() {
	accessUrl := fmt.Sprintf("localhost:%s", s.net.ContainerByName(testnet.PrimaryAN).Port(testnet.GRPCPort))
	accessClient, err := common.GetAccessAPIClient(accessUrl)
	s.Require().NoError(err)

	observerURL := fmt.Sprintf("localhost:%s", s.net.ContainerByName(testnet.PrimaryON).Port(testnet.GRPCPort))
	observerClient, err := common.GetAccessAPIClient(observerURL)
	s.Require().NoError(err)

	// get the first block height
	currentFinalized := s.BlockState.HighestFinalizedHeight()
	blockA := s.BlockState.WaitForHighestFinalizedProgress(s.T(), currentFinalized)

	var startValue interface{}
	txCount := 10

	for _, rpc := range s.testedRPCs() {
		s.T().Run(rpc.name, func(t *testing.T) {
			if rpc.name == "SubscribeBlocksFromStartBlockID" {
				startValue = convert.IdentifierToMessage(blockA.ID())
			} else {
				startValue = blockA.Height
			}

			accessRecv := rpc.call(s.ctx, accessClient, startValue)
			accessBlocks, accessBlockErrs, err := SubscribeHandler(s.ctx, accessRecv, blockResponseHandler)
			s.Require().NoError(err)

			observerRecv := rpc.call(s.ctx, observerClient, startValue)
			observerBlocks, observerBlockErrs, err := SubscribeHandler(s.ctx, observerRecv, blockResponseHandler)
			s.Require().NoError(err)

			foundANTxCount := 0
			foundONTxCount := 0

			r := NewResponseTracker(compareBlocksResponse, 2)

			for {
				select {
				case err := <-accessBlockErrs:
					s.Require().NoErrorf(err, "unexpected AN error")
				case err := <-observerBlockErrs:
					s.Require().NoErrorf(err, "unexpected ON error")
				case block := <-accessBlocks:
					s.T().Logf("AN block received: height: %d", block.Height)
					r.Add(s.T(), block.Height, "access", block)
					foundANTxCount++
				case block := <-observerBlocks:
					s.T().Logf("ON block received: height: %d", block.Height)
					s.addObserverBlock(block, r, rpc.name, &foundONTxCount)
				}

				if foundANTxCount >= txCount && foundONTxCount >= txCount {
					break
				}
			}

			r.AssertAllResponsesHandled(t, txCount)
		})
	}
}

// testRestEventStreaming tests REST event streaming via WebSocket.
func (s *StreamingSuite) testRestEventStreaming() {
	restAddr := s.net.ContainerByName(testnet.PrimaryAN).Addr(testnet.RESTPort)

	s.T().Run("subscribe events", func(t *testing.T) {
		startBlockId := flow.ZeroID
		startHeight := uint64(0)
		url := getSubscribeEventsRequest(restAddr, startBlockId, startHeight, nil, nil, nil)

		client, err := common.GetWSClient(s.ctx, url)
		require.NoError(t, err)

		var receivedEventsResponse []*backend.EventsResponse

		go func() {
			time.Sleep(10 * time.Second)
			// close connection after 10 seconds
			client.Close()
		}()

		eventChan := make(chan *backend.EventsResponse)
		go func() {
			for {
				resp := &backend.EventsResponse{}
				err := client.ReadJSON(resp)
				if err != nil {
					if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
						s.T().Logf("unexpected close error: %v", err)
						require.NoError(s.T(), err)
					}
					close(eventChan) // Close the event channel when the client connection is closed
					return
				}
				eventChan <- resp
			}
		}()

		// collect received events during 10 seconds
		for eventResponse := range eventChan {
			receivedEventsResponse = append(receivedEventsResponse, eventResponse)
		}

		// check events
		s.requireEvents(receivedEventsResponse)
	})
}

// addObserverBlock adds a block received from the observer node to the response tracker
// and increments the transaction count for that node.
//
// Parameters:
//   - block: The block received from the node.
//   - responseTracker: The response tracker to which the block should be added.
//   - rpcCallName: The name of the rpc subscription call which is testing.
//   - txCount: A pointer to an integer that tracks the number of transactions received from the node.
func (s *StreamingSuite) addObserverBlock(
	block *flow.Block,
	responseTracker *ResponseTracker[*flow.Block],
	rpcCallName string,
	txCount *int,
) {
	// the response tracker expects to receive data for the same heights from each node.
	// when subscribing to the latest block, the specific start height depends on the node's
	// local sealed height, so it may vary.
	// check only the responses for ON that are also tracked by AN and compare them
	isANResponseExist := len(responseTracker.r[block.Height]) > 0
	if rpcCallName == "SubscribeBlocksFromLatest" && !isANResponseExist {
		return
	}

	responseTracker.Add(s.T(), block.Height, "observer", block)
	*txCount++
}

// requireEvents is a helper function that encapsulates logic for comparing received events from rest state streaming and
// events which received from grpc api
func (s *StreamingSuite) requireEvents(receivedEventsResponse []*backend.EventsResponse) {
	// make sure there are received events
	require.GreaterOrEqual(s.T(), len(receivedEventsResponse), 1, "expect received events")

	grpcCtx, grpcCancel := context.WithCancel(s.ctx)
	defer grpcCancel()

	grpcAddr := s.net.ContainerByName(testnet.PrimaryAN).Addr(testnet.GRPCPort)

	grpcConn, err := grpc.DialContext(grpcCtx, grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(s.T(), err, "failed to connect to access node")
	defer grpcConn.Close()

	grpcClient := accessproto.NewAccessAPIClient(grpcConn)

	// Variable to keep track of non-empty event response count
	nonEmptyResponseCount := 0
	for _, receivedEventResponse := range receivedEventsResponse {
		// Create a map where key is EventType and value is list of events with this EventType
		receivedEventMap := make(map[flow.EventType][]flow.Event)
		for _, event := range receivedEventResponse.Events {
			eventType := event.Type
			receivedEventMap[eventType] = append(receivedEventMap[eventType], event)
		}

		for eventType, receivedEventList := range receivedEventMap {
			// get events by block id and event type
			response, err := MakeApiRequest(
				grpcClient.GetEventsForBlockIDs,
				grpcCtx,
				&accessproto.GetEventsForBlockIDsRequest{
					BlockIds: [][]byte{convert.IdentifierToMessage(receivedEventResponse.BlockID)},
					Type:     string(eventType),
				},
			)
			require.NoError(s.T(), err)
			require.Equal(s.T(), 1, len(response.Results), "expect to get 1 result")

			expectedEventsResult := response.Results[0]
			require.Equal(s.T(), expectedEventsResult.BlockHeight, receivedEventResponse.Height, "expect the same block height")
			require.Equal(s.T(), len(expectedEventsResult.Events), len(receivedEventList), "expect the same count of events: want: %+v, got: %+v", expectedEventsResult.Events, receivedEventList)

			for i, event := range receivedEventList {
				require.Equal(s.T(), expectedEventsResult.Events[i].EventIndex, event.EventIndex, "expect the same event index")
				require.Equal(s.T(), convert.MessageToIdentifier(expectedEventsResult.Events[i].TransactionId), event.TransactionID, "expect the same transaction id")
			}

			// Check if the current response has non-empty events
			if len(receivedEventResponse.Events) > 0 {
				nonEmptyResponseCount++
			}
		}
	}
	// Ensure that at least one response had non-empty events
	require.GreaterOrEqual(s.T(), nonEmptyResponseCount, 1, "expect at least one response with non-empty events")
}

func blockResponseHandler(msg *accessproto.SubscribeBlocksResponse) (*flow.Block, error) {
	return convert.MessageToBlock(msg.GetBlock())
}

func compareBlocksResponse(t *testing.T, responses map[uint64]map[string]*flow.Block, blockHeight uint64) error {
	accessData := responses[blockHeight]["access"]
	observerData := responses[blockHeight]["observer"]

	// Compare access with observer
	compareBlocks(t, accessData, observerData)

	return nil
}

func compareBlocks(t *testing.T, accessBlock *flow.Block, observerBlock *flow.Block) {
	require.Equal(t, accessBlock.ID(), observerBlock.ID())
	require.Equal(t, accessBlock.Height, observerBlock.Height)
	require.Equal(t, accessBlock.Timestamp, observerBlock.Timestamp)
	require.Equal(t, accessBlock.Payload.Hash(), observerBlock.Payload.Hash())
}

// getSubscribeEventsRequest is a helper function that creates SubscribeEventsRequest
func getSubscribeEventsRequest(accessAddr string, startBlockId flow.Identifier, startHeight uint64, eventTypes []string, addresses []string, contracts []string) string {
	u, _ := url.Parse("http://" + accessAddr + "/v1/subscribe_events")
	q := u.Query()

	if startBlockId != flow.ZeroID {
		q.Add("start_block_id", startBlockId.String())
	}

	if startHeight != request.EmptyHeight {
		q.Add("start_height", fmt.Sprintf("%d", startHeight))
	}

	if len(eventTypes) > 0 {
		q.Add("event_types", strings.Join(eventTypes, ","))
	}
	if len(addresses) > 0 {
		q.Add("addresses", strings.Join(addresses, ","))
	}
	if len(contracts) > 0 {
		q.Add("contracts", strings.Join(contracts, ","))
	}

	u.RawQuery = q.Encode()
	return u.String()
}

type subscribeBlocksRPCTest struct {
	name string
	call func(ctx context.Context, client accessproto.AccessAPIClient, startValue interface{}) func() (*accessproto.SubscribeBlocksResponse, error)
}

func (s *StreamingSuite) getRPCs() []subscribeBlocksRPCTest {
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
