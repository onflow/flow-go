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

	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/flow/protobuf/go/flow/entities"
	"github.com/onflow/flow/protobuf/go/flow/executiondata"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	sdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/test"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

var (
	jsonOptions = []jsoncdc.Option{jsoncdc.WithAllowUnstructuredStaticTypes(true)}
)

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

	s.net.Start(s.ctx)
}

// TestRestEventStreaming tests event streaming route on REST
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

	time.Sleep(20 * time.Second)

	// TODO: will be refactored after https://github.com/onflow/flow-go/pull/5602 merged
	req := &executiondata.SubscribeEventsRequest{
		StartBlockHeight:     0,
		EventEncodingVersion: entities.EventEncodingVersion_CCF_V0,
		Filter:               &executiondata.EventFilter{},
		HeartbeatInterval:    1,
	}

	testANStream, err := sdkClientTestAN.SubscribeEvents(s.ctx, req)
	s.Require().NoError(err)
	testANEvents, testANErrs, err := SubscribeHandler(s.ctx, testANStream.Recv, eventsResponseHandler)
	s.Require().NoError(err)

	controlANStream, err := sdkClientControlAN.SubscribeEvents(s.ctx, req)
	s.Require().NoError(err)
	controlANEvents, controlANErrs, err := SubscribeHandler(s.ctx, controlANStream.Recv, eventsResponseHandler)
	s.Require().NoError(err)

	testONStream, err := sdkClientTestON.SubscribeEvents(s.ctx, req)
	s.Require().NoError(err)
	testONEvents, testONErrs, err := SubscribeHandler(s.ctx, testONStream.Recv, eventsResponseHandler)
	s.Require().NoError(err)

	txCount := 10

	// generate events
	go func() {
		txGenerator, err := s.net.ContainerByName(testnet.PrimaryAN).TestnetClient()
		s.Require().NoError(err)

		refBlockID, err := txGenerator.GetLatestBlockID(s.ctx)
		s.Require().NoError(err)

		for i := 0; i < txCount; i++ {
			accountKey := test.AccountKeyGenerator().New()
			address, err := txGenerator.CreateAccount(s.ctx, accountKey, sdk.HexToID(refBlockID.String()))
			s.Require().NoError(err)
			s.T().Logf("created account: %s", address)
		}
	}()

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
	r := NewResponseTracker(compareEventsResponse)

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
				s.T().Logf("adding access test events: %d %d %v", event.BlockHeight, len(event.Events), event.Events)
				r.Add(s.T(), event.BlockHeight, "access_test", &event)
				foundANTxCount++
			}
		case event := <-controlANEvents:
			if has(event.Events, targetEvent) {
				s.T().Logf("adding control events: %d %d %v", event.BlockHeight, len(event.Events), event.Events)
				r.Add(s.T(), event.BlockHeight, "access_control", &event)
			}
		case event := <-testONEvents:
			if has(event.Events, targetEvent) {
				s.T().Logf("adding observer test events: %d %d %v", event.BlockHeight, len(event.Events), event.Events)
				r.Add(s.T(), event.BlockHeight, "observer_test", &event)
				foundONTxCount++
			}
		}

		if foundANTxCount >= txCount && foundONTxCount >= txCount {
			break
		}
	}
}

// ResponseTracker is a generic tracker for responses.
type ResponseTracker[T any] struct {
	r       map[uint64]map[string]T
	mu      sync.RWMutex
	compare func(t *testing.T, responses map[uint64]map[string]T, blockHeight uint64) error //func(control, test T) error
}

// NewResponseTracker creates a new ResponseTracker.
func NewResponseTracker[T any](
	compare func(t *testing.T, responses map[uint64]map[string]T, blockHeight uint64) error,
) *ResponseTracker[T] {
	return &ResponseTracker[T]{
		r:       make(map[uint64]map[string]T),
		compare: compare,
	}
}

func (r *ResponseTracker[T]) Add(t *testing.T, blockHeight uint64, name string, response T) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.r[blockHeight]; !ok {
		r.r[blockHeight] = make(map[string]T)
	}
	r.r[blockHeight][name] = response

	err := r.compare(t, r.r, blockHeight)
	if err != nil {
		log.Fatalf("comparison error at block height %d: %v", blockHeight, err)
	}

	delete(r.r, blockHeight)
}

func eventsResponseHandler(msg *executiondata.SubscribeEventsResponse) (flow.BlockEvents, error) {
	events := convert.MessagesToEvents(msg.GetEvents())

	return flow.BlockEvents{
		BlockHeight:    msg.GetBlockHeight(),
		BlockID:        convert.MessageToIdentifier(msg.GetBlockId()),
		Events:         events,
		BlockTimestamp: msg.GetBlockTimestamp().AsTime(),
	}, nil
}

func compareEventsResponse(t *testing.T, responses map[uint64]map[string]*flow.BlockEvents, blockHeight uint64) error {
	if len(responses[blockHeight]) != 3 {
		return nil
	}
	accessControlData := responses[blockHeight]["access_control"]
	accessTestData := responses[blockHeight]["access_test"]
	observerTestData := responses[blockHeight]["observer_test"]

	// Compare access_control with access_test
	err := compareEvents(t, accessControlData, accessTestData)
	if err != nil {
		return fmt.Errorf("failure comparing access and access data: %d: %v", blockHeight, err)
	}

	// Compare access_control with observer_test
	err = compareEvents(t, accessControlData, observerTestData)
	if err != nil {
		return fmt.Errorf("failure comparing access and observer data: %d: %v", blockHeight, err)
	}

	return nil
}

func compareEvents(t *testing.T, controlData, testData *flow.BlockEvents) error {
	require.Equal(t, controlData.BlockID, testData.BlockID)
	require.Equal(t, controlData.BlockHeight, testData.BlockHeight)
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
			t, err := recv()
			if err != nil {
				if err == io.EOF {
					return
				}

				sendErr(fmt.Errorf("error receiving block: %w", err))
				return
			}

			response, err := responseHandler(t)
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
