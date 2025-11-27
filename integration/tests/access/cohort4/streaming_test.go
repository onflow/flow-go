package cohort4

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	sdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/test"

	"github.com/onflow/flow-go/engine/access/rest/http/request"
	"github.com/onflow/flow-go/engine/access/state_stream/backend"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/engine/ghost/client"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/integration/tests/access/common"
	"github.com/onflow/flow-go/integration/tests/lib"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/utils/unittest"

	accessproto "github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/entities"
	"github.com/onflow/flow/protobuf/go/flow/executiondata"
)

var (
	jsonOptions = []jsoncdc.Option{jsoncdc.WithAllowUnstructuredStaticTypes(true)}
)

// SubscribeEventsResponse represents the subscription response containing events for a specific block and messageIndex
type SubscribeEventsResponse struct {
	backend.EventsResponse
	MessageIndex uint64
}

func TestStreamingCombined(t *testing.T) {
	suite.Run(t, new(StreamingSuite))
}

// StreamingSuite tests gRPC event streaming, gRPC block streaming, and REST event streaming APIs
// with a single unified network setup to optimize test runtime.
type StreamingSuite struct {
	suite.Suite
	lib.TestnetStateTracker

	log zerolog.Logger

	// root context for the current test
	ctx    context.Context
	cancel context.CancelFunc

	net *testnet.FlowNetwork

	ghostID flow.Identifier
}

func (s *StreamingSuite) TearDownTest() {
	s.log.Info().Msg("================> Start TearDownTest")
	s.net.Remove()
	s.cancel()
	s.log.Info().Msg("================> Finish TearDownTest")
}

// SetupTest initializes the test suite with a network configuration for testing all streaming APIs.
//
// Network Configuration:
//   - 2 Access nodes with different event query modes:
//     testAN (access_1): event-query-mode=local-only, with execution data indexing
//     controlAN (access_2): event-query-mode=execution-nodes-only, with execution data indexing
//     Both have supports-observer=true and public-network-execution-data-sync-enabled=true
//   - 1 Ghost Access node (lightweight, for tracking block state)
//   - 1 Observer node (with execution data indexing: event-query-mode=execution-nodes-only)
//   - 2 Collection nodes (standard configuration)
//   - 3 Consensus nodes (with custom timing: 100ms proposal duration, reduced seal approvals)
//   - 2 Execution nodes (standard configuration)
//   - 1 Verification node (standard configuration)
//
// This unified setup supports testing gRPC event streaming, gRPC block streaming, and REST event streaming,
// allowing verification that all streaming APIs return consistent results across different nodes and query modes.
func (s *StreamingSuite) SetupTest() {
	s.log = unittest.LoggerForTest(s.Suite.T(), zerolog.InfoLevel)
	s.log.Info().Msg("================> SetupTest")
	defer func() {
		s.log.Info().Msg("================> Finish SetupTest")
	}()

	// access node with local-only event query mode
	testANConfig := testnet.NewNodeConfig(
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

	// access node with execution-nodes-only event query mode
	controlANConfig := testnet.NewNodeConfig(
		flow.RoleAccess,
		testnet.WithLogLevel(zerolog.InfoLevel),
		testnet.WithAdditionalFlag("--execution-data-sync-enabled=true"),
		testnet.WithAdditionalFlagf("--execution-data-dir=%s", testnet.DefaultExecutionDataServiceDir),
		testnet.WithAdditionalFlag("--execution-data-retry-delay=1s"),
		testnet.WithAdditionalFlag("--execution-data-indexing-enabled=true"),
		testnet.WithAdditionalFlagf("--execution-state-dir=%s", testnet.DefaultExecutionStateDir),
		testnet.WithAdditionalFlag("--event-query-mode=execution-nodes-only"),
	)

	// add the ghost (access) node config
	s.ghostID = unittest.IdentifierFixture()
	ghostNode := testnet.NewNodeConfig(
		flow.RoleAccess,
		testnet.WithID(s.ghostID),
		testnet.WithLogLevel(zerolog.FatalLevel),
		testnet.AsGhost())

	consensusConfigs := []func(config *testnet.NodeConfig){
		testnet.WithAdditionalFlag("--cruise-ctl-fallback-proposal-duration=100ms"),
		testnet.WithAdditionalFlag(fmt.Sprintf("--required-verification-seal-approvals=%d", 1)),
		testnet.WithAdditionalFlag(fmt.Sprintf("--required-construction-seal-approvals=%d", 1)),
		testnet.WithLogLevel(zerolog.FatalLevel),
	}

	nodeConfigs := []testnet.NodeConfig{
		testnet.NewNodeConfig(flow.RoleCollection, testnet.WithLogLevel(zerolog.FatalLevel)),
		testnet.NewNodeConfig(flow.RoleCollection, testnet.WithLogLevel(zerolog.FatalLevel)),
		testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.FatalLevel)),
		testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.FatalLevel)),
		testnet.NewNodeConfig(flow.RoleConsensus, consensusConfigs...),
		testnet.NewNodeConfig(flow.RoleConsensus, consensusConfigs...),
		testnet.NewNodeConfig(flow.RoleConsensus, consensusConfigs...),
		testnet.NewNodeConfig(flow.RoleVerification, testnet.WithLogLevel(zerolog.FatalLevel)),
		testANConfig,    // access_1
		controlANConfig, // access_2
		ghostNode,       // access ghost
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

	conf := testnet.NewNetworkConfig("streaming_combined_test", nodeConfigs, testnet.WithObservers(observers...))
	s.net = testnet.PrepareFlowNetwork(s.T(), conf, flow.Localnet)

	// start the network
	s.T().Logf("starting flow network with docker containers")
	s.ctx, s.cancel = context.WithCancel(context.Background())

	s.net.Start(s.ctx)
	s.Track(s.T(), s.ctx, s.Ghost())
}

func (s *StreamingSuite) Ghost() *client.GhostClient {
	client, err := s.net.ContainerByID(s.ghostID).GhostClient()
	require.NoError(s.T(), err, "could not get ghost client")
	return client
}

// TestStreamingAPIs runs all streaming API tests with a shared network setup to minimize initialization time.
func (s *StreamingSuite) TestStreamingAPIs() {
	s.testGrpcEventStreaming()
	s.testGrpcBlockStreaming()
	s.testRestEventStreaming()
}

// testGrpcEventStreaming tests gRPC event streaming APIs (ExecutionDataAPI).
func (s *StreamingSuite) testGrpcEventStreaming() {
	testAN := s.net.ContainerByName(testnet.PrimaryAN)
	sdkClientTestAN := getExecutionDataClient(s.T(), testAN)

	controlAN := s.net.ContainerByName("access_2")
	sdkClientControlAN := getExecutionDataClient(s.T(), controlAN)

	testON := s.net.ContainerByName(testnet.PrimaryON)
	sdkClientTestON := getExecutionDataClient(s.T(), testON)

	// get the first block height
	currentFinalized := s.BlockState.HighestFinalizedHeight()
	blockA := s.BlockState.WaitForHighestFinalizedProgress(s.T(), currentFinalized)

	// Let the network run for this many blocks
	blockCount := uint64(5)
	// wait for the requested number of sealed blocks
	s.BlockState.WaitForSealedHeight(s.T(), blockA.Height+blockCount)

	txGenerator, err := s.net.ContainerByName(testnet.PrimaryAN).TestnetClient()
	s.Require().NoError(err)

	var startValue interface{}
	txCount := 10

	for _, rpc := range getEventStreamingRPCs(s) {
		s.T().Run(rpc.name, func(t *testing.T) {
			if rpc.name == "SubscribeEventsFromStartBlockID" {
				startValue = convert.IdentifierToMessage(blockA.ID())
			} else {
				startValue = blockA.Height
			}

			testANRecv := rpc.call(s.ctx, sdkClientTestAN, startValue, &executiondata.EventFilter{})
			testANEvents, testANErrs, err := SubscribeHandler(s.ctx, testANRecv, eventsResponseHandler)
			require.NoError(t, err)

			controlANRecv := rpc.call(s.ctx, sdkClientControlAN, startValue, &executiondata.EventFilter{})
			controlANEvents, controlANErrs, err := SubscribeHandler(s.ctx, controlANRecv, eventsResponseHandler)
			require.NoError(t, err)

			testONRecv := rpc.call(s.ctx, sdkClientTestON, startValue, &executiondata.EventFilter{})
			testONEvents, testONErrs, err := SubscribeHandler(s.ctx, testONRecv, eventsResponseHandler)
			require.NoError(t, err)

			if rpc.generateEvents {
				// generate events
				go func() {
					s.generateEvents(txGenerator, txCount)
				}()
			}

			has := func(events []flow.Event, eventType flow.EventType) bool {
				for _, event := range events {
					if event.Type == eventType {
						return true
					}
				}
				return false
			}

			targetEvent := flow.EventType("flow.AccountCreated")

			foundANTxTestCount := 0
			foundANTxControlCount := 0
			foundONTxCount := 0

			messageIndex := counters.NewMonotonicCounter(0)

			r := NewResponseTracker(compareEventsResponse, 3)

			for {
				select {
				case err := <-testANErrs:
					require.NoErrorf(t, err, "unexpected test AN error")
				case err := <-controlANErrs:
					require.NoErrorf(t, err, "unexpected control AN error")
				case err := <-testONErrs:
					require.NoErrorf(t, err, "unexpected test ON error")
				case event := <-testANEvents:
					if has(event.Events, targetEvent) {
						t.Logf("adding access test events: %d %d %v", event.Height, len(event.Events), event.Events)
						r.Add(t, event.Height, "access_test", event)
						foundANTxTestCount++
					}
				case event := <-controlANEvents:
					if has(event.Events, targetEvent) {
						if ok := messageIndex.Set(event.MessageIndex); !ok {
							require.NoErrorf(t, err, "messageIndex isn`t sequential")
						}
						t.Logf("adding control events: %d %d %v", event.Height, len(event.Events), event.Events)
						r.Add(t, event.Height, "access_control", event)
						foundANTxControlCount++
					}
				case event := <-testONEvents:
					if has(event.Events, targetEvent) {
						t.Logf("adding observer test events: %d %d %v", event.Height, len(event.Events), event.Events)
						r.Add(t, event.Height, "observer_test", event)
						foundONTxCount++
					}
				}

				if foundANTxTestCount >= txCount &&
					foundONTxCount >= txCount &&
					foundANTxControlCount >= txCount {
					break
				}
			}

			r.AssertAllResponsesHandled(t, txCount)
		})
	}
}

// testGrpcBlockStreaming tests gRPC block streaming APIs (AccessAPI).
func (s *StreamingSuite) testGrpcBlockStreaming() {
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

	for _, rpc := range getBlockStreamingRPCs(s) {
		s.T().Run(rpc.name, func(t *testing.T) {
			if rpc.name == "SubscribeBlocksFromStartBlockID" {
				startValue = convert.IdentifierToMessage(blockA.ID())
			} else {
				startValue = blockA.Height
			}

			accessRecv := rpc.call(s.ctx, accessClient, startValue)
			accessBlocks, accessBlockErrs, err := SubscribeHandler(s.ctx, accessRecv, blockResponseHandler)
			require.NoError(t, err)

			observerRecv := rpc.call(s.ctx, observerClient, startValue)
			observerBlocks, observerBlockErrs, err := SubscribeHandler(s.ctx, observerRecv, blockResponseHandler)
			require.NoError(t, err)

			foundANTxCount := 0
			foundONTxCount := 0

			r := NewResponseTracker(compareBlocksResponse, 2)

			for {
				select {
				case err := <-accessBlockErrs:
					require.NoErrorf(t, err, "unexpected AN error")
				case err := <-observerBlockErrs:
					require.NoErrorf(t, err, "unexpected ON error")
				case block := <-accessBlocks:
					t.Logf("AN block received: height: %d", block.Height)
					r.Add(t, block.Height, "access", block)
					foundANTxCount++
				case block := <-observerBlocks:
					t.Logf("ON block received: height: %d", block.Height)
					s.addObserverBlock(t, block, r, rpc.name, &foundONTxCount)
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
						t.Logf("unexpected close error: %v", err)
						require.NoError(t, err)
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
//   - t: The testing context from the subtest.
//   - block: The block received from the node.
//   - responseTracker: The response tracker to which the block should be added.
//   - rpcCallName: The name of the rpc subscription call which is testing.
//   - txCount: A pointer to an integer that tracks the number of transactions received from the node.
func (s *StreamingSuite) addObserverBlock(
	t *testing.T,
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

	responseTracker.Add(t, block.Height, "observer", block)
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

// generateEvents is a helper function for generating AccountCreated events
func (s *StreamingSuite) generateEvents(client *testnet.Client, txCount int) {
	refBlockID, err := client.GetLatestBlockID(s.ctx)
	s.Require().NoError(err)

	for i := 0; i < txCount; i++ {
		accountKey := test.AccountKeyGenerator().New()
		address, err := client.CreateAccount(s.ctx, accountKey, sdk.HexToID(refBlockID.String()))
		if err != nil {
			i--
			continue
		}
		s.T().Logf("created account: %s", address)
	}
}

type subscribeEventsRPCTest struct {
	name           string
	call           func(ctx context.Context, client executiondata.ExecutionDataAPIClient, startValue interface{}, filter *executiondata.EventFilter) func() (*executiondata.SubscribeEventsResponse, error)
	generateEvents bool // add ability to integration test generate new events or use old events to decrease running test time
}

func getEventStreamingRPCs(s *StreamingSuite) []subscribeEventsRPCTest {
	return []subscribeEventsRPCTest{
		{
			name: "SubscribeEventsFromLatest",
			call: func(ctx context.Context, client executiondata.ExecutionDataAPIClient, _ interface{}, filter *executiondata.EventFilter) func() (*executiondata.SubscribeEventsResponse, error) {
				stream, err := client.SubscribeEventsFromLatest(ctx, &executiondata.SubscribeEventsFromLatestRequest{
					EventEncodingVersion: entities.EventEncodingVersion_CCF_V0,
					Filter:               filter,
					HeartbeatInterval:    1,
				})
				s.Require().NoError(err)
				return stream.Recv
			},
			generateEvents: true,
		},
		{
			name: "SubscribeEvents",
			call: func(ctx context.Context, client executiondata.ExecutionDataAPIClient, _ interface{}, filter *executiondata.EventFilter) func() (*executiondata.SubscribeEventsResponse, error) {
				// Ignore deprecation warning. keeping these tests until endpoint is removed
				//nolint:staticcheck
				stream, err := client.SubscribeEvents(ctx, &executiondata.SubscribeEventsRequest{
					StartBlockId:         convert.IdentifierToMessage(flow.ZeroID),
					StartBlockHeight:     0,
					EventEncodingVersion: entities.EventEncodingVersion_CCF_V0,
					Filter:               filter,
					HeartbeatInterval:    1,
				})
				s.Require().NoError(err)
				return stream.Recv
			},
			generateEvents: true,
		},
		{
			name: "SubscribeEventsFromStartBlockID",
			call: func(ctx context.Context, client executiondata.ExecutionDataAPIClient, startValue interface{}, filter *executiondata.EventFilter) func() (*executiondata.SubscribeEventsResponse, error) {
				stream, err := client.SubscribeEventsFromStartBlockID(ctx, &executiondata.SubscribeEventsFromStartBlockIDRequest{
					StartBlockId:         startValue.([]byte),
					EventEncodingVersion: entities.EventEncodingVersion_CCF_V0,
					Filter:               filter,
					HeartbeatInterval:    1,
				})
				s.Require().NoError(err)
				return stream.Recv
			},
			generateEvents: false, // use previous events
		},
		{
			name: "SubscribeEventsFromStartHeight",
			call: func(ctx context.Context, client executiondata.ExecutionDataAPIClient, startValue interface{}, filter *executiondata.EventFilter) func() (*executiondata.SubscribeEventsResponse, error) {
				stream, err := client.SubscribeEventsFromStartHeight(ctx, &executiondata.SubscribeEventsFromStartHeightRequest{
					StartBlockHeight:     startValue.(uint64),
					EventEncodingVersion: entities.EventEncodingVersion_CCF_V0,
					Filter:               filter,
					HeartbeatInterval:    1,
				})
				s.Require().NoError(err)
				return stream.Recv
			},
			generateEvents: false, // use previous events
		},
	}
}

type subscribeBlocksRPCTest struct {
	name string
	call func(ctx context.Context, client accessproto.AccessAPIClient, startValue interface{}) func() (*accessproto.SubscribeBlocksResponse, error)
}

func getBlockStreamingRPCs(s *StreamingSuite) []subscribeBlocksRPCTest {
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

// ResponseTracker is a generic tracker for responses.
type ResponseTracker[T any] struct {
	r                       map[uint64]map[string]T
	mu                      sync.RWMutex
	compare                 func(t *testing.T, responses map[uint64]map[string]T, blockHeight uint64) error
	checkCount              int // actual common count of responses we want to check
	responsesCountToCompare int // count of responses that we want to compare with each other
}

// NewResponseTracker creates a new ResponseTracker.
func NewResponseTracker[T any](
	compare func(t *testing.T, responses map[uint64]map[string]T, blockHeight uint64) error,
	responsesCountToCompare int,
) *ResponseTracker[T] {
	return &ResponseTracker[T]{
		r:                       make(map[uint64]map[string]T),
		compare:                 compare,
		responsesCountToCompare: responsesCountToCompare,
	}
}

func (r *ResponseTracker[T]) AssertAllResponsesHandled(t *testing.T, expectedCheckCount int) {
	assert.Equal(t, expectedCheckCount, r.checkCount)

	// we check if response tracker has some responses which were not checked, but should be checked
	hasNotComparedResponses := false
	for _, valueMap := range r.r {
		if len(valueMap) == r.responsesCountToCompare {
			hasNotComparedResponses = true
			break
		}
	}
	assert.False(t, hasNotComparedResponses)
}

func (r *ResponseTracker[T]) Add(t *testing.T, blockHeight uint64, name string, response T) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.r[blockHeight]; !ok {
		r.r[blockHeight] = make(map[string]T)
	}
	r.r[blockHeight][name] = response

	if len(r.r[blockHeight]) != r.responsesCountToCompare {
		return
	}

	r.checkCount += 1
	err := r.compare(t, r.r, blockHeight)
	if err != nil {
		log.Fatalf("comparison error at block height %d: %v", blockHeight, err)
	}

	delete(r.r, blockHeight)
}

func eventsResponseHandler(msg *executiondata.SubscribeEventsResponse) (*SubscribeEventsResponse, error) {
	events, err := convert.MessagesToEvents(msg.GetEvents())
	if err != nil {
		return nil, err
	}

	return &SubscribeEventsResponse{
		EventsResponse: backend.EventsResponse{
			Height:         msg.GetBlockHeight(),
			BlockID:        convert.MessageToIdentifier(msg.GetBlockId()),
			Events:         events,
			BlockTimestamp: msg.GetBlockTimestamp().AsTime(),
		},
		MessageIndex: msg.MessageIndex,
	}, nil
}

func compareEventsResponse(t *testing.T, responses map[uint64]map[string]*SubscribeEventsResponse, blockHeight uint64) error {

	accessControlData := responses[blockHeight]["access_control"]
	accessTestData := responses[blockHeight]["access_test"]
	observerTestData := responses[blockHeight]["observer_test"]

	// Compare access_control with access_test
	compareEvents(t, accessControlData, accessTestData)

	// Compare access_control with observer_test
	compareEvents(t, accessControlData, observerTestData)

	return nil
}

func compareEvents(t *testing.T, controlData, testData *SubscribeEventsResponse) {
	require.Equal(t, controlData.BlockID, testData.BlockID)
	require.Equal(t, controlData.Height, testData.Height)
	require.Equal(t, controlData.BlockTimestamp, testData.BlockTimestamp)
	require.Equal(t, controlData.MessageIndex, testData.MessageIndex)
	require.Equal(t, len(controlData.Events), len(testData.Events))

	for i := range controlData.Events {
		require.Equal(t, controlData.Events[i].Type, testData.Events[i].Type)
		require.Equal(t, controlData.Events[i].TransactionID, testData.Events[i].TransactionID)
		require.Equal(t, controlData.Events[i].TransactionIndex, testData.Events[i].TransactionIndex)
		require.Equal(t, controlData.Events[i].EventIndex, testData.Events[i].EventIndex)
		require.True(t, bytes.Equal(controlData.Events[i].Payload, testData.Events[i].Payload))
	}
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

func getExecutionDataClient(t *testing.T, node *testnet.Container) executiondata.ExecutionDataAPIClient {
	accessClient, err := node.SDKClient()
	require.NoError(t, err, "could not get access client")
	return accessClient.ExecutionDataRPCClient()
}

func SubscribeHandler[T any, V any](
	ctx context.Context,
	recv func() (T, error),
	responseHandler func(T) (V, error),
) (<-chan V, <-chan error, error) {
	sub := make(chan V)
	errChan := make(chan error)

	sendErr := func(err error) {
		select {
		case <-ctx.Done():
		case errChan <- err:
		}
	}

	go func() {
		defer close(sub)
		defer close(errChan)

		for {
			resp, err := recv()
			if err != nil {
				if err == io.EOF {
					return
				}

				sendErr(fmt.Errorf("error receiving response: %w", err))
				return
			}

			response, err := responseHandler(resp)
			if err != nil {
				sendErr(fmt.Errorf("error converting response: %w", err))
				return
			}

			select {
			case <-ctx.Done():
				return
			case sub <- response:
			}
		}
	}()

	return sub, errChan, nil
}
