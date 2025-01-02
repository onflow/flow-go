package cohort3

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	commonmodels "github.com/onflow/flow-go/engine/access/rest/common/models"
	"github.com/onflow/flow-go/engine/access/rest/util"
	"github.com/onflow/flow-go/engine/access/rest/websockets/data_providers"
	"github.com/onflow/flow-go/engine/access/rest/websockets/models"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"

	accessproto "github.com/onflow/flow/protobuf/go/flow/access"
)

const InactivityTimeout = 20

func TestWebsocketSubscription(t *testing.T) {
	suite.Run(t, new(WebsocketSubscriptionSuite))
}

type WebsocketSubscriptionSuite struct {
	suite.Suite

	log zerolog.Logger

	// root context for the current test
	ctx    context.Context
	cancel context.CancelFunc

	net *testnet.FlowNetwork

	grpcClient accessproto.AccessAPIClient
}

func (s *WebsocketSubscriptionSuite) TearDownTest() {
	s.log.Info().Msg("================> Start TearDownTest")
	s.net.Remove()
	s.cancel()
	s.log.Info().Msg("================> Finish TearDownTest")
}

func (s *WebsocketSubscriptionSuite) SetupTest() {
	s.log = unittest.LoggerForTest(s.Suite.T(), zerolog.InfoLevel)
	s.log.Info().Msg("================> SetupTest")
	defer func() {
		s.log.Info().Msg("================> Finish SetupTest")
	}()

	// access node
	bridgeANConfig := testnet.NewNodeConfig(
		flow.RoleAccess,
		testnet.WithLogLevel(zerolog.ErrorLevel),
		testnet.WithAdditionalFlag("--execution-data-sync-enabled=true"),
		testnet.WithAdditionalFlagf("--execution-data-dir=%s", testnet.DefaultExecutionDataServiceDir),
		testnet.WithAdditionalFlag("--execution-data-retry-delay=1s"),
		testnet.WithAdditionalFlag("--execution-data-indexing-enabled=true"),
		testnet.WithAdditionalFlagf("--execution-state-dir=%s", testnet.DefaultExecutionStateDir),
		testnet.WithAdditionalFlagf("--websocket-inactivity-timeout=%ds", InactivityTimeout),
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

	conf := testnet.NewNetworkConfig("websockets_subscriptions_test", nodeConfigs)
	s.net = testnet.PrepareFlowNetwork(s.T(), conf, flow.Localnet)

	// start the network
	s.T().Logf("starting flow network with docker containers")
	s.ctx, s.cancel = context.WithCancel(context.Background())

	s.net.Start(s.ctx)

	accessUrl := fmt.Sprintf("localhost:%s", s.net.ContainerByName(testnet.PrimaryAN).Port(testnet.GRPCPort))
	var err error
	s.grpcClient, err = getAccessAPIClient(accessUrl)
	s.Require().NoError(err)
}

// TestInactivityHeaders tests that the WebSocket connection closes due to inactivity
// after the specified timeout duration.
func (s *WebsocketSubscriptionSuite) TestInactivityHeaders() {
	// Steps:
	// 1. Establish a WebSocket connection to the server.
	// 2. Start a goroutine to listen for messages from the server.
	// 3. Wait for the server to close the connection due to inactivity.
	// 4. Validate that the actual inactivity duration is within the expected range.
	s.T().Run("no active subscription after connection creation", func(t *testing.T) {
		restAddr := s.net.ContainerByName(testnet.PrimaryAN).Addr(testnet.RESTPort)
		wsClient, err := getWSClient(s.ctx, getWebsocketsUrl(restAddr))
		s.Require().NoError(err)
		defer func() { s.Require().NoError(wsClient.Close()) }()

		expectedInactivityDuration := InactivityTimeout * time.Second
		actualInactivityDuration := monitorInactivity(t, wsClient, expectedInactivityDuration)

		s.Assert().LessOrEqual(expectedInactivityDuration, actualInactivityDuration)
	})

	// Steps:
	// 1. Establish a WebSocket connection to the server.
	// 2. Subscribe to a topic and validate the subscription response.
	// 3. Unsubscribe from the topic and validate the unsubscription response.
	// 4. Wait for the server to close the connection due to inactivity.
	s.T().Run("all active subscriptions unsubscribed", func(t *testing.T) {
		// Step 1: Establish WebSocket connection
		restAddr := s.net.ContainerByName(testnet.PrimaryAN).Addr(testnet.RESTPort)
		wsClient, err := getWSClient(s.ctx, getWebsocketsUrl(restAddr))
		s.Require().NoError(err)
		defer func() { s.Require().NoError(wsClient.Close()) }()

		// Step 2: Subscribe to a topic
		subscriptionRequest := models.SubscribeMessageRequest{
			BaseMessageRequest: models.BaseMessageRequest{
				Action:          models.SubscribeAction,
				ClientMessageID: uuid.New().String(),
			},
			Topic: data_providers.EventsTopic,
		}

		s.Require().NoError(wsClient.WriteJSON(subscriptionRequest))

		_, baseResponses, _ := listenWebSocketResponses[models.EventResponse](
			s.T(),
			wsClient,
			5*time.Second,
			subscriptionRequest.ClientMessageID,
		)

		s.Require().Equal(1, len(baseResponses))
		subscribeResponse := baseResponses[0]
		s.Require().True(subscribeResponse.Success)

		// Step 3: Unsubscribe from the topic
		unsubscribeRequest := models.UnsubscribeMessageRequest{
			BaseMessageRequest: models.BaseMessageRequest{
				Action:          models.UnsubscribeAction,
				ClientMessageID: uuid.New().String(),
			},
			SubscriptionID: subscribeResponse.SubscriptionID,
		}

		s.Require().NoError(wsClient.WriteJSON(unsubscribeRequest))

		// TODO: Somehow unsubscription are not return properly, but result appended to subscriptions
		_, baseResponses, _ = listenWebSocketResponses[models.EventResponse](
			s.T(),
			wsClient,
			5*time.Second,
			unsubscribeRequest.ClientMessageID,
		)

		s.Require().Equal(1, len(baseResponses))
		unsubscribeResponse := baseResponses[0]
		s.Require().True(unsubscribeResponse.Success)

		// Step 4: Monitor inactivity after unsubscription
		expectedInactivityDuration := InactivityTimeout * time.Second
		actualInactivityDuration := monitorInactivity(s.T(), wsClient, expectedInactivityDuration)

		s.Assert().LessOrEqual(expectedInactivityDuration, actualInactivityDuration)
	})
}

// monitorInactivity monitors the WebSocket connection for inactivity.
func monitorInactivity(t *testing.T, client *websocket.Conn, timeout time.Duration) time.Duration {
	start := time.Now()
	errChan := make(chan error, 1)

	go func() {
		for {
			if _, _, err := client.ReadMessage(); err != nil {
				errChan <- err
				return
			}
		}
	}()

	select {
	case <-time.After(timeout * 2):
		t.Fatal("Test timed out waiting for WebSocket closure due to inactivity")
		return 0
	case <-errChan:
		return time.Since(start)
	}
}

func (s *WebsocketSubscriptionSuite) TestHappyCases() {
	restAddr := s.net.ContainerByName(testnet.PrimaryAN).Addr(testnet.RESTPort)
	wsClient, err := getWSClient(s.ctx, getWebsocketsUrl(restAddr))
	s.Require().NoError(err)

	defer wsClient.Close()

	// tests streaming blocks
	//s.T().Run("blocks streaming", func(t *testing.T) {
	//	clientMessageID := uuid.New().String()
	//
	//	subscriptionRequest := models.SubscribeMessageRequest{
	//		BaseMessageRequest: models.BaseMessageRequest{
	//			Action:          models.SubscribeAction,
	//			ClientMessageID: clientMessageID,
	//		},
	//		Topic:     data_providers.BlocksTopic,
	//		Arguments: models.Arguments{"block_status": parser.Finalized},
	//	}
	//
	//	testWebsocketSubscription[models.BlockMessageResponse](
	//		t,
	//		wsClient,
	//		subscriptionRequest,
	//		s.validateBlocks,
	//		5*time.Second,
	//	)
	//})

	//// tests streaming block headers
	//s.T().Run("block headers streaming", func(t *testing.T) {
	//	clientMessageID := uuid.New().String()
	//
	//	subscriptionRequest := models.SubscribeMessageRequest{
	//		BaseMessageRequest: models.BaseMessageRequest{
	//			Action:          models.SubscribeAction,
	//			ClientMessageID: clientMessageID,
	//		},
	//		Topic:     data_providers.BlockHeadersTopic,
	//		Arguments: models.Arguments{"block_status": parser.Finalized},
	//	}
	//
	//	testWebsocketSubscription[models.BlockHeaderMessageResponse](
	//		t,
	//		wsClient,
	//		subscriptionRequest,
	//		s.validateBlockHeaders,
	//		10*time.Second,
	//	)
	//})

	//// tests streaming block digests
	//s.T().Run("block digests streaming", func(t *testing.T) {
	//	clientMessageID := uuid.New().String()
	//
	//	subscriptionRequest := models.SubscribeMessageRequest{
	//		BaseMessageRequest: models.BaseMessageRequest{
	//			Action:          models.SubscribeAction,
	//			ClientMessageID: clientMessageID,
	//		},
	//		Topic:     data_providers.BlockDigestsTopic,
	//		Arguments: models.Arguments{"block_status": parser.Finalized},
	//	}
	//
	//	testWebsocketSubscription[models.BlockDigestMessageResponse](
	//		t,
	//		wsClient,
	//		subscriptionRequest,
	//		s.validateBlockDigests,
	//		5*time.Second,
	//	)
	//})

	// tests streaming events
	s.T().Run("events streaming", func(t *testing.T) {
		clientMessageID := uuid.New().String()

		subscriptionRequest := models.SubscribeMessageRequest{
			BaseMessageRequest: models.BaseMessageRequest{
				Action:          models.SubscribeAction,
				ClientMessageID: clientMessageID,
			},
			Topic:     data_providers.EventsTopic,
			Arguments: models.Arguments{},
		}

		testWebsocketSubscription[models.EventResponse](
			t,
			wsClient,
			subscriptionRequest,
			s.validateEvents,
			5*time.Second,
		)
	})
}

// validateBlocks validates the received block responses against gRPC responses.
func (s *WebsocketSubscriptionSuite) validateBlocks(
	receivedResponses []*models.BlockMessageResponse,
) {
	require.NotEmpty(s.T(), receivedResponses, "expected received block headers")

	for _, response := range receivedResponses {
		id, err := flow.HexStringToIdentifier(response.Block.Header.Id)
		require.NoError(s.T(), err)

		grpcResponse, err := s.grpcClient.GetBlockHeaderByID(s.ctx, &accessproto.GetBlockHeaderByIDRequest{
			Id: convert.IdentifierToMessage(id),
		})
		require.NoError(s.T(), err)

		grpcExpected := grpcResponse.Block
		actual := response.Block

		require.Equal(s.T(), convert.MessageToIdentifier(grpcExpected.Id).String(), actual.Header.Id)
		require.Equal(s.T(), util.FromUint(grpcExpected.Height), actual.Header.Height)
		require.Equal(s.T(), grpcExpected.Timestamp.AsTime(), actual.Header.Timestamp)
		require.Equal(s.T(), convert.MessageToIdentifier(grpcExpected.ParentId).String(), actual.Header.ParentId)
	}
}

// validateBlockHeaders validates the received block header responses against gRPC responses.
func (s *WebsocketSubscriptionSuite) validateBlockHeaders(
	receivedResponses []models.BlockHeaderMessageResponse,
) {
	require.NotEmpty(s.T(), receivedResponses, "expected received block headers")

	for _, response := range receivedResponses {
		id, err := flow.HexStringToIdentifier(response.Header.Id)
		require.NoError(s.T(), err)

		grpcResponse, err := s.grpcClient.GetBlockHeaderByID(s.ctx, &accessproto.GetBlockHeaderByIDRequest{
			Id: convert.IdentifierToMessage(id),
		})
		require.NoError(s.T(), err)

		grpcExpected := grpcResponse.Block
		actual := response.Header

		require.Equal(s.T(), convert.MessageToIdentifier(grpcExpected.Id).String(), actual.Id)
		require.Equal(s.T(), util.FromUint(grpcExpected.Height), actual.Height)
		require.Equal(s.T(), grpcExpected.Timestamp.AsTime(), actual.Timestamp)
		require.Equal(s.T(), convert.MessageToIdentifier(grpcExpected.ParentId).String(), actual.ParentId)
	}
}

// validateBlockDigests validates the received block digest responses against gRPC responses.
func (s *WebsocketSubscriptionSuite) validateBlockDigests(
	receivedResponses []*models.BlockDigestMessageResponse,
) {
	require.NotEmpty(s.T(), receivedResponses, "expected received block digests")

	for _, response := range receivedResponses {
		s.T().Logf("received response: %s", response.Block.Height)
		id, err := flow.HexStringToIdentifier(response.Block.BlockId)
		require.NoError(s.T(), err)

		grpcResponse, err := s.grpcClient.GetBlockHeaderByID(s.ctx, &accessproto.GetBlockHeaderByIDRequest{
			Id: convert.IdentifierToMessage(id),
		})
		require.NoError(s.T(), err)

		grpcExpected := grpcResponse.Block
		actual := response.Block

		require.Equal(s.T(), convert.MessageToIdentifier(grpcExpected.Id).String(), actual.BlockId)
		require.Equal(s.T(), util.FromUint(grpcExpected.Height), actual.Height)
		require.Equal(s.T(), grpcExpected.Timestamp.AsTime(), actual.Timestamp)
	}
}

// validateEvents is a helper function that encapsulates logic for comparing received events from rest state streaming and
// events which received from grpc api
func (s *WebsocketSubscriptionSuite) validateEvents(receivedEventsResponse []*models.EventResponse) {
	// make sure there are received events
	require.GreaterOrEqual(s.T(), len(receivedEventsResponse), 1, "expect received events")

	// Variable to keep track of non-empty event response count
	nonEmptyResponseCount := 0
	for _, receivedEventResponse := range receivedEventsResponse {
		// Create a map where key is event type and value is list of events with this event typ
		receivedEventMap := make(map[string][]commonmodels.Event)
		for _, event := range receivedEventResponse.Events {
			eventType := event.Type_
			receivedEventMap[eventType] = append(receivedEventMap[eventType], event)
		}

		for eventType, receivedEventList := range receivedEventMap {
			blockId, err := flow.HexStringToIdentifier(receivedEventResponse.BlockId)
			require.NoError(s.T(), err)

			// get events by block id and event type
			response, err := s.grpcClient.GetEventsForBlockIDs(
				s.ctx,
				&accessproto.GetEventsForBlockIDsRequest{
					BlockIds: [][]byte{convert.IdentifierToMessage(blockId)},
					Type:     eventType,
				},
			)
			require.NoError(s.T(), err)
			require.Equal(s.T(), 1, len(response.Results), "expect to get 1 result")

			expectedEventsResult := response.Results[0]
			require.Equal(s.T(), util.FromUint(expectedEventsResult.BlockHeight), receivedEventResponse.BlockHeight, "expect the same block height")
			require.Equal(s.T(), len(expectedEventsResult.Events), len(receivedEventList), "expect the same count of events: want: %+v, got: %+v", expectedEventsResult.Events, receivedEventList)

			for i, event := range receivedEventList {
				require.Equal(s.T(), util.FromUint(expectedEventsResult.Events[i].EventIndex), event.EventIndex, "expect the same event index")
				require.Equal(s.T(), convert.MessageToIdentifier(expectedEventsResult.Events[i].TransactionId).String(), event.TransactionId, "expect the same transaction id")
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

// subscribeMessageRequest creates a subscription message request.
func (s *WebsocketSubscriptionSuite) subscribeMessageRequest(clientMessageID string, topic string, arguments models.Arguments) interface{} {
	return models.SubscribeMessageRequest{
		BaseMessageRequest: models.BaseMessageRequest{
			Action:          models.SubscribeAction,
			ClientMessageID: clientMessageID,
		},
		Topic:     topic,
		Arguments: arguments,
	}
}

// unsubscribeMessageRequest creates an unsubscribe message request.
func (s *WebsocketSubscriptionSuite) unsubscribeMessageRequest(clientMessageID string, subscriptionID string) interface{} {
	return models.UnsubscribeMessageRequest{
		BaseMessageRequest: models.BaseMessageRequest{
			Action:          models.UnsubscribeAction,
			ClientMessageID: clientMessageID,
		},
		SubscriptionID: subscriptionID,
	}
}

// listSubscriptionsMessageRequest creates a list subscriptions message request.
func (s *WebsocketSubscriptionSuite) listSubscriptionsMessageRequest(clientMessageID string) interface{} {
	return models.ListSubscriptionsMessageRequest{
		BaseMessageRequest: models.BaseMessageRequest{
			Action:          models.ListSubscriptionsAction,
			ClientMessageID: clientMessageID,
		},
	}
}

// getWebsocketsUrl is a helper function that creates websocket url
func getWebsocketsUrl(accessAddr string) string {
	u, _ := url.Parse("http://" + accessAddr + "/v1/ws")
	return u.String()
}

// testWebsocketSubscription tests a websocket subscription and validates responses.
//
// This function handles the lifecycle of a websocket connection for a specific subscription,
// including sending a subscription request, listening for incoming responses, and validating
// them using a provided validation function. The websocket connection is closed automatically
// after a predefined time interval.
func testWebsocketSubscription[T any](
	t *testing.T,
	client *websocket.Conn,
	subscriptionRequest models.SubscribeMessageRequest,
	validate func([]*T),
	duration time.Duration,
) {
	// subscribe to specific topic
	require.NoError(t, client.WriteJSON(subscriptionRequest))

	responses, baseMessageResponses, _ := listenWebSocketResponses[T](t, client, duration, subscriptionRequest.ClientMessageID)

	// validate subscribe response
	require.Equal(t, 1, len(baseMessageResponses))

	subscribeMessageResponse := baseMessageResponses[0]
	require.Equal(t, subscriptionRequest.ClientMessageID, subscribeMessageResponse.ClientMessageID)

	// Use the provided validation function to ensure the received responses of type T are correct.
	validate(responses)

	// unsubscribe from specific topic
	unsubscriptionRequest := models.UnsubscribeMessageRequest{
		BaseMessageRequest: models.BaseMessageRequest{
			Action:          models.SubscribeAction,
			ClientMessageID: subscriptionRequest.ClientMessageID,
		},
		SubscriptionID: subscribeMessageResponse.SubscriptionID,
	}

	require.NoError(t, client.WriteJSON(unsubscriptionRequest))
}

// listenWebSocketResponses listens for websocket responses for a specified duration
// and unmarshalls them into expected types.
//
// Parameters:
//   - t: The *testing.T object used for managing test lifecycle and assertions.
//   - client: The websocket connection to read messages from.
//   - duration: The maximum time to listen for messages before stopping.
func listenWebSocketResponses[T any](
	t *testing.T,
	client *websocket.Conn,
	duration time.Duration,
	clientMessageID string,
) (
	[]*T,
	[]*models.BaseMessageResponse,
	[]*models.ListSubscriptionsMessageResponse,
) {
	var responses []*T
	var baseMessageResponses []*models.BaseMessageResponse
	var listSubscriptionsMessageResponses []*models.ListSubscriptionsMessageResponse

	timer := time.NewTimer(duration)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			t.Logf("stopping websocket response listener after %s", duration)
			return responses, baseMessageResponses, listSubscriptionsMessageResponses
		default:
			_, messageBytes, err := client.ReadMessage()
			if err != nil {
				t.Logf("websocket error: %v", err)

				var closeErr *websocket.CloseError
				if errors.As(err, &closeErr) || strings.Contains(err.Error(), "use of closed network connection") {
					t.Logf("websocket close error: %v", closeErr)
					return responses, baseMessageResponses, listSubscriptionsMessageResponses
				}

				require.FailNow(t, fmt.Sprintf("unexpected websocket error, %v", err))
			}

			// BaseMessageResponse and validate
			var baseResp models.BaseMessageResponse
			if err := json.Unmarshal(messageBytes, &baseResp); err == nil && baseResp.ClientMessageID == clientMessageID {
				baseMessageResponses = append(baseMessageResponses, &baseResp)
				continue
			}

			//// Try unmarshalling into ListSubscriptionsMessageResponse and validate
			//var listResp models.ListSubscriptionsMessageResponse
			//if err := json.Unmarshal(messageBytes, &listResp); err == nil &&
			//	listResp.ClientMessageID == clientMessageID {
			//	listSubscriptionsMessageResponses = append(listSubscriptionsMessageResponses, &listResp)
			//	continue
			//}

			//TODO: update Unmarshal according to changes for issue #6819
			var genericResp T
			if err := json.Unmarshal(messageBytes, &genericResp); err == nil && &genericResp != nil {
				responses = append(responses, &genericResp)
			}
		}
	}
}
