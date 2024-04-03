package cohort3

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	jsoncdc "github.com/onflow/cadence/encoding/json"

	"github.com/onflow/flow-go-sdk/test"
	"github.com/onflow/flow-go/engine/access/state_stream/backend"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"

	sdk "github.com/onflow/flow-go-sdk"

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

func TestGrpcStateStream(t *testing.T) {
	suite.Run(t, new(GrpcStateStreamSuite))
}

type GrpcStateStreamSuite struct {
	suite.Suite

	log zerolog.Logger

	// root context for the current test
	ctx    context.Context
	cancel context.CancelFunc

	net *testnet.FlowNetwork

	// RPC methods to test
	testedRPCs func() []RPCTest
}

func (s *GrpcStateStreamSuite) TearDownTest() {
	s.log.Info().Msg("================> Start TearDownTest")
	s.net.Remove()
	s.cancel()
	s.log.Info().Msg("================> Finish TearDownTest")
}

func (s *GrpcStateStreamSuite) SetupTest() {
	s.log = unittest.LoggerForTest(s.Suite.T(), zerolog.InfoLevel)
	s.log.Info().Msg("================> SetupTest")
	defer func() {
		s.log.Info().Msg("================> Finish SetupTest")
	}()

	// access node
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

	consensusConfigs := []func(config *testnet.NodeConfig){
		testnet.WithAdditionalFlag("--cruise-ctl-fallback-proposal-duration=400ms"),
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

	conf := testnet.NewNetworkConfig("access_event_streaming_test", nodeConfigs, testnet.WithObservers(observers...))
	s.net = testnet.PrepareFlowNetwork(s.T(), conf, flow.Localnet)

	// start the network
	s.T().Logf("starting flow network with docker containers")
	s.ctx, s.cancel = context.WithCancel(context.Background())

	s.testedRPCs = s.getRPCs

	s.net.Start(s.ctx)
}

// TestRestEventStreaming tests gRPC event streaming
func (s *GrpcStateStreamSuite) TestHappyPath() {
	testANURL := fmt.Sprintf("localhost:%s", s.net.ContainerByName(testnet.PrimaryAN).Port(testnet.ExecutionStatePort))
	sdkClientTestAN, err := getClient(testANURL)
	s.Require().NoError(err)

	controlANURL := fmt.Sprintf("localhost:%s", s.net.ContainerByName("access_2").Port(testnet.ExecutionStatePort))
	sdkClientControlAN, err := getClient(controlANURL)
	s.Require().NoError(err)

	testONURL := fmt.Sprintf("localhost:%s", s.net.ContainerByName(testnet.PrimaryON).Port(testnet.ExecutionStatePort))
	sdkClientTestON, err := getClient(testONURL)
	s.Require().NoError(err)

	txGenerator, err := s.net.ContainerByName(testnet.PrimaryAN).TestnetClient()
	s.Require().NoError(err)
	header, err := txGenerator.GetLatestSealedBlockHeader(s.ctx)
	s.Require().NoError(err)

	time.Sleep(20 * time.Second)

	var startValue interface{}
	txCount := 10

	for _, rpc := range s.testedRPCs() {
		s.T().Run(rpc.name, func(t *testing.T) {
			if rpc.name == "SubscribeEventsFromStartBlockID" {
				startValue = header.ID.Bytes()
			} else {
				startValue = header.Height
			}

			testANStream, err := rpc.call(s.ctx, sdkClientTestAN, startValue, &executiondata.EventFilter{})
			s.Require().NoError(err)
			testANEvents, testANErrs, err := SubscribeEventsHandler(s.ctx, testANStream)
			s.Require().NoError(err)

			controlANStream, err := rpc.call(s.ctx, sdkClientControlAN, startValue, &executiondata.EventFilter{})
			s.Require().NoError(err)
			controlANEvents, controlANErrs, err := SubscribeEventsHandler(s.ctx, controlANStream)
			s.Require().NoError(err)

			testONStream, err := rpc.call(s.ctx, sdkClientTestON, startValue, &executiondata.EventFilter{})
			s.Require().NoError(err)
			testONEvents, testONErrs, err := SubscribeEventsHandler(s.ctx, testONStream)
			s.Require().NoError(err)

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

			foundANTxCount := 0
			foundONTxCount := 0
			r := newResponseTracker()

			for {
				select {
				case err := <-testANErrs:
					s.Require().NoErrorf(err, "unexpected test AN error")
				case err := <-controlANErrs:
					s.Require().NoErrorf(err, "unexpected control AN error")
				case err := <-testONErrs:
					s.Require().NoErrorf(err, "unexpected test ON error")
				case event := <-testANEvents:
					if has(event.Events, targetEvent) {
						s.T().Logf("adding access test events: %d %d %v", event.Height, len(event.Events), event.Events)
						r.Add(s.T(), event.Height, "access_test", &event)
						foundANTxCount++
					}
				case event := <-controlANEvents:
					if has(event.Events, targetEvent) {
						s.T().Logf("adding control events: %d %d %v", event.Height, len(event.Events), event.Events)
						r.Add(s.T(), event.Height, "access_control", &event)
					}
				case event := <-testONEvents:
					if has(event.Events, targetEvent) {
						s.T().Logf("adding observer test events: %d %d %v", event.Height, len(event.Events), event.Events)
						r.Add(s.T(), event.Height, "observer_test", &event)
						foundONTxCount++
					}
				}

				if foundANTxCount >= txCount && foundONTxCount >= txCount {
					break
				}
			}
		})
	}
}

// generateEvents is a helper function for generating AccountCreated events
func (s *GrpcStateStreamSuite) generateEvents(client *testnet.Client, txCount int) {
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

type RPCTest struct {
	name           string
	call           func(ctx context.Context, client executiondata.ExecutionDataAPIClient, startValue interface{}, filter *executiondata.EventFilter) (executiondata.ExecutionDataAPI_SubscribeEventsClient, error)
	generateEvents bool // add ability to integration test generate new events or use old events to decrease running test time
}

func (s *GrpcStateStreamSuite) getRPCs() []RPCTest {
	return []RPCTest{
		{
			name: "SubscribeEventsFromLatest",
			call: func(ctx context.Context, client executiondata.ExecutionDataAPIClient, _ interface{}, filter *executiondata.EventFilter) (executiondata.ExecutionDataAPI_SubscribeEventsClient, error) {
				return client.SubscribeEventsFromLatest(ctx, &executiondata.SubscribeEventsFromLatestRequest{
					EventEncodingVersion: entities.EventEncodingVersion_CCF_V0,
					Filter:               filter,
					HeartbeatInterval:    1,
				})
			},
			generateEvents: true,
		},
		{
			name: "SubscribeEvents",
			call: func(ctx context.Context, client executiondata.ExecutionDataAPIClient, _ interface{}, filter *executiondata.EventFilter) (executiondata.ExecutionDataAPI_SubscribeEventsClient, error) {
				//nolint: staticcheck
				return client.SubscribeEvents(ctx, &executiondata.SubscribeEventsRequest{
					StartBlockId:         convert.IdentifierToMessage(flow.ZeroID),
					StartBlockHeight:     0,
					EventEncodingVersion: entities.EventEncodingVersion_CCF_V0,
					Filter:               filter,
					HeartbeatInterval:    1,
				})
			},
			generateEvents: true,
		},
		{
			name: "SubscribeEventsFromStartBlockID",
			call: func(ctx context.Context, client executiondata.ExecutionDataAPIClient, startValue interface{}, filter *executiondata.EventFilter) (executiondata.ExecutionDataAPI_SubscribeEventsClient, error) {
				return client.SubscribeEventsFromStartBlockID(ctx, &executiondata.SubscribeEventsFromStartBlockIDRequest{
					StartBlockId:         startValue.([]byte),
					EventEncodingVersion: entities.EventEncodingVersion_CCF_V0,
					Filter:               filter,
					HeartbeatInterval:    1,
				})
			},
			generateEvents: false, // use previous events
		},
		{
			name: "SubscribeEventsFromStartHeight",
			call: func(ctx context.Context, client executiondata.ExecutionDataAPIClient, startValue interface{}, filter *executiondata.EventFilter) (executiondata.ExecutionDataAPI_SubscribeEventsClient, error) {
				return client.SubscribeEventsFromStartHeight(ctx, &executiondata.SubscribeEventsFromStartHeightRequest{
					StartBlockHeight:     startValue.(uint64),
					EventEncodingVersion: entities.EventEncodingVersion_CCF_V0,
					Filter:               filter,
					HeartbeatInterval:    1,
				})
			},
			generateEvents: false, // use previous events
		},
	}
}

type ResponseTracker struct {
	r  map[uint64]map[string]SubscribeEventsResponse
	mu sync.RWMutex
}

func newResponseTracker() *ResponseTracker {
	return &ResponseTracker{
		r: make(map[uint64]map[string]SubscribeEventsResponse),
	}
}

func (r *ResponseTracker) Add(t *testing.T, blockHeight uint64, name string, events *SubscribeEventsResponse) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.r[blockHeight]; !ok {
		r.r[blockHeight] = make(map[string]SubscribeEventsResponse)
	}
	r.r[blockHeight][name] = *events

	if len(r.r[blockHeight]) != 3 {
		return
	}

	err := r.compare(t, r.r[blockHeight]["access_control"], r.r[blockHeight]["access_test"])
	if err != nil {
		log.Fatalf("failure comparing access and access data %d: %v", blockHeight, err)
	}

	err = r.compare(t, r.r[blockHeight]["access_control"], r.r[blockHeight]["observer_test"])
	if err != nil {
		log.Fatalf("failure comparing access and observer data %d: %v", blockHeight, err)
	}

	delete(r.r, blockHeight)
}

func (r *ResponseTracker) compare(t *testing.T, controlData SubscribeEventsResponse, testData SubscribeEventsResponse) error {
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

	return nil
}

// TODO: switch to SDK versions once crypto library is fixed to support the latest SDK version

func getClient(address string) (executiondata.ExecutionDataAPIClient, error) {
	conn, err := grpc.Dial(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	return executiondata.NewExecutionDataAPIClient(conn), nil
}

func SubscribeEventsHandler(
	ctx context.Context,
	stream executiondata.ExecutionDataAPI_SubscribeEventsClient,
) (<-chan SubscribeEventsResponse, <-chan error, error) {
	sub := make(chan SubscribeEventsResponse)
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
			resp, err := stream.Recv()
			if err != nil {
				if err == io.EOF {
					return
				}

				sendErr(fmt.Errorf("error receiving event: %w", err))
				return
			}

			events := convert.MessagesToEvents(resp.GetEvents())

			response := SubscribeEventsResponse{
				EventsResponse: backend.EventsResponse{
					Height:         resp.GetBlockHeight(),
					BlockID:        convert.MessageToIdentifier(resp.GetBlockId()),
					Events:         events,
					BlockTimestamp: resp.GetBlockTimestamp().AsTime(),
				},
				MessageIndex: resp.MessageIndex,
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
