package cohort3

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

	"github.com/onflow/flow-go/engine/access/rest/request"
	"github.com/onflow/flow-go/engine/access/state_stream/backend"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"

	accessproto "github.com/onflow/flow/protobuf/go/flow/access"
)

func TestRestStateStream(t *testing.T) {
	suite.Run(t, new(RestStateStreamSuite))
}

type RestStateStreamSuite struct {
	suite.Suite

	log zerolog.Logger

	// root context for the current test
	ctx    context.Context
	cancel context.CancelFunc

	net *testnet.FlowNetwork
}

func (s *RestStateStreamSuite) TearDownTest() {
	s.log.Info().Msg("================> Start TearDownTest")
	s.net.Remove()
	s.cancel()
	s.log.Info().Msg("================> Finish TearDownTest")
}

func (s *RestStateStreamSuite) SetupTest() {
	s.log = unittest.LoggerForTest(s.Suite.T(), zerolog.InfoLevel)
	s.log.Info().Msg("================> SetupTest")
	defer func() {
		s.log.Info().Msg("================> Finish SetupTest")
	}()

	// access node
	bridgeANConfig := testnet.NewNodeConfig(
		flow.RoleAccess,
		testnet.WithLogLevel(zerolog.InfoLevel),
		testnet.WithAdditionalFlag("--execution-data-sync-enabled=true"),
		testnet.WithAdditionalFlagf("--execution-data-dir=%s", testnet.DefaultExecutionDataServiceDir),
		testnet.WithAdditionalFlag("--execution-data-retry-delay=1s"),
		testnet.WithAdditionalFlag("--execution-data-indexer-enabled=true"),
		testnet.WithAdditionalFlagf("--execution-state-dir=%s", testnet.DefaultExecutionStateDir),
	)

	// add the ghost (access) node config
	ghostNode := testnet.NewNodeConfig(
		flow.RoleAccess,
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
		bridgeANConfig,
		ghostNode,
	}

	conf := testnet.NewNetworkConfig("access_api_test", nodeConfigs)
	s.net = testnet.PrepareFlowNetwork(s.T(), conf, flow.Localnet)

	// start the network
	s.T().Logf("starting flow network with docker containers")
	s.ctx, s.cancel = context.WithCancel(context.Background())

	s.net.Start(s.ctx)
}

// TestRestEventStreaming tests event streaming route on REST
func (s *RestStateStreamSuite) TestRestEventStreaming() {
	restAddr := s.net.ContainerByName(testnet.PrimaryAN).Addr(testnet.RESTPort)

	s.T().Run("subscribe events", func(t *testing.T) {
		startBlockId := flow.ZeroID
		startHeight := uint64(0)
		url := getSubscribeEventsRequest(restAddr, startBlockId, startHeight, nil, nil, nil)

		client, err := getWSClient(s.ctx, url)
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

// requireEvents is a helper function that encapsulates logic for comparing received events from rest state streaming and
// events which received from grpc api
func (s *RestStateStreamSuite) requireEvents(receivedEventsResponse []*backend.EventsResponse) {
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

// getWSClient is a helper function that creates a websocket client
func getWSClient(ctx context.Context, address string) (*websocket.Conn, error) {
	// helper func to create WebSocket client
	client, _, err := websocket.DefaultDialer.DialContext(ctx, strings.Replace(address, "http", "ws", 1), nil)
	if err != nil {
		return nil, err
	}
	return client, nil
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
