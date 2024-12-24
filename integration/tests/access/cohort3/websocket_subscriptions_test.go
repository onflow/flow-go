package cohort3

import (
	"context"
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

	"github.com/onflow/flow-go/engine/access/rest/common/parser"
	"github.com/onflow/flow-go/engine/access/rest/websockets/data_providers"
	"github.com/onflow/flow-go/engine/access/rest/websockets/models"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"

	accessproto "github.com/onflow/flow/protobuf/go/flow/access"
)

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
		testnet.WithLogLevel(zerolog.InfoLevel),
		testnet.WithAdditionalFlag("--execution-data-sync-enabled=true"),
		testnet.WithAdditionalFlagf("--execution-data-dir=%s", testnet.DefaultExecutionDataServiceDir),
		testnet.WithAdditionalFlag("--execution-data-retry-delay=1s"),
		testnet.WithAdditionalFlag("--execution-data-indexing-enabled=true"),
		testnet.WithAdditionalFlagf("--execution-state-dir=%s", testnet.DefaultExecutionStateDir),
		testnet.WithAdditionalFlag("--websocket-inactivity-timeout=15s"),
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
}

// TestRestEventStreaming tests event streaming route on REST
func (s *WebsocketSubscriptionSuite) TestBlockHeaders() {
	accessUrl := fmt.Sprintf("localhost:%s", s.net.ContainerByName(testnet.PrimaryAN).Port(testnet.GRPCPort))
	grpcClient, err := getAccessAPIClient(accessUrl)
	s.Require().NoError(err)

	restAddr := s.net.ContainerByName(testnet.PrimaryAN).Addr(testnet.RESTPort)
	client, err := getWSClient(s.ctx, getWebsocketsUrl(restAddr))
	s.Require().NoError(err)

	//
	//s.testWebsocketSubscription(
	//	client,
	//	subscriptionRequest,
	//	func(receivedResponses []interface{}) {
	//		var blockHeaderResponses []*models.BlockHeaderMessageResponse
	//		for _, resp := range receivedResponses {
	//			if blockResp, ok := resp.(*models.BlockHeaderMessageResponse); ok {
	//				blockHeaderResponses = append(blockHeaderResponses, blockResp)
	//			}
	//		}
	//		s.validateBlockHeaders(grpcClient, blockHeaderResponses)
	//	})

	//TODO: move common to separate func to reuse for other subscriptions
	s.T().Run("block headers streaming", func(t *testing.T) {
		clientMessageID := uuid.New().String()

		subscriptionRequest := models.SubscribeMessageRequest{
			BaseMessageRequest: models.BaseMessageRequest{
				Action:          models.SubscribeAction,
				ClientMessageID: clientMessageID,
			},
			Topic:     data_providers.BlockHeadersTopic,
			Arguments: models.Arguments{"block_status": parser.Finalized},
		}

		go func() {
			err := client.WriteJSON(subscriptionRequest)
			require.NoError(s.T(), err)

			time.Sleep(10 * time.Second)
			// close connection after 15 seconds
			//TODO: add unsubscribe
			client.Close()
		}()

		var receivedResponse []interface{}
		for {
			var resp interface{}

			err = client.ReadJSON(&resp)
			if err != nil {
				s.T().Logf("websocket error: %v", err)

				var closeErr *websocket.CloseError
				if errors.As(err, &closeErr) {
					s.T().Logf("websocket close error: %v", closeErr)
					break
				}

				if strings.Contains(err.Error(), "use of closed network connection") {
					s.T().Logf("websocket connection already closed: %v", err)
					break
				}

				require.Fail(t, fmt.Sprintf("unexpected websocket error, %v", err))
			}
			receivedResponse = append(receivedResponse, resp)
			s.T().Logf("!!!! RESPONSE: %v", resp)
		}

		// Process received responses
		// collect received responses during 15 seconds
		var receivedHeadersResponse []models.BlockHeaderMessageResponse

		//TODO: add verify for success response message
		for _, response := range receivedResponse {
			switch resp := response.(type) {
			case models.BlockHeaderMessageResponse:
				receivedHeadersResponse = append(receivedHeadersResponse, resp)
				s.log.Info().Msgf("Received BlockHeaderMessageResponse: %v", resp)
			case *models.BlockMessageResponse:
				s.log.Info().Msgf("Received *BlocksMessageResponse: %v", resp)
			case *models.BlockDigestMessageResponse:
				s.log.Info().Msgf("Received *BlockDigestsMessageResponse: %v", resp)

			case *models.BaseMessageResponse:
				s.log.Info().Msgf("Received *BaseMessageResponse: %v", resp)
			default:
				s.T().Errorf("unexpected response type: %T", resp)
			}
		}

		// check block headers
		s.validateBlockHeaders(grpcClient, receivedHeadersResponse)
	})
}

func (s *WebsocketSubscriptionSuite) validateBlockHeaders(
	grpcClient accessproto.AccessAPIClient,
	receivedResponses []models.BlockHeaderMessageResponse,
) {
	//require.NotEmpty(s.T(), receivedResponses, "expected received block headers")

	for _, response := range receivedResponses {
		id, err := flow.HexStringToIdentifier(response.Header.Id)
		require.NoError(s.T(), err)

		grpcResponse, err := grpcClient.GetBlockHeaderByID(s.ctx, &accessproto.GetBlockHeaderByIDRequest{
			Id: convert.IdentifierToMessage(id),
		})
		require.NoError(s.T(), err)

		expected := grpcResponse.Block
		require.Equal(s.T(), expected.Height, response.Header.Height)
		require.Equal(s.T(), expected.Timestamp, response.Header.Timestamp)
		require.Equal(s.T(), expected.ParentId, response.Header.ParentId)
	}
}

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

func (s *WebsocketSubscriptionSuite) unsubscribeMessageRequest(clientMessageID string, subscriptionID string) interface{} {
	return models.UnsubscribeMessageRequest{
		BaseMessageRequest: models.BaseMessageRequest{
			Action:          models.UnsubscribeAction,
			ClientMessageID: clientMessageID,
		},
		SubscriptionID: subscriptionID,
	}
}

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

//func (s *WebsocketSubscriptionSuite) setupGRPCClient() accessproto.AccessAPIClient {
//	grpcAddr := s.net.ContainerByName(testnet.PrimaryAN).Addr(testnet.GRPCPort)
//	conn, err := grpc.Dial(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
//	require.NoError(s.T(), err)
//	s.T().Cleanup(func() { conn.Close() })
//	return accessproto.NewAccessAPIClient(conn)
//}

func (s *WebsocketSubscriptionSuite) testWebsocketSubscription(
	client *websocket.Conn,
	subscriptionRequest models.SubscribeMessageRequest,
	validate func([]interface{}),
) {
	go func() {
		require.NoError(s.T(), client.WriteJSON(subscriptionRequest))
		time.Sleep(15 * time.Second)
		client.Close()
	}()

	responseChan := make(chan interface{})
	s.listenForResponses(client, responseChan)

	var receivedResponses []interface{}
	for resp := range responseChan {
		receivedResponses = append(receivedResponses, resp)
	}

	validate(receivedResponses)
}

func (s *WebsocketSubscriptionSuite) listenForResponses(client *websocket.Conn, responseChan chan interface{}) {
	defer close(responseChan)
	for {
		var resp interface{}
		if err := client.ReadJSON(&resp); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
				s.T().Logf("unexpected close error: %v", err)
				require.NoError(s.T(), err)
			}
			return
		}
		responseChan <- resp
	}
}
