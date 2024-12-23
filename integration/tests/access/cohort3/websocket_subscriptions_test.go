package cohort3

import (
	"context"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

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
		testnet.WithLogLevel(zerolog.DebugLevel),
		testnet.WithAdditionalFlag("--execution-data-sync-enabled=true"),
		testnet.WithAdditionalFlagf("--execution-data-dir=%s", testnet.DefaultExecutionDataServiceDir),
		testnet.WithAdditionalFlag("--execution-data-retry-delay=1s"),
		testnet.WithAdditionalFlag("--execution-data-indexing-enabled=true"),
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

	conf := testnet.NewNetworkConfig("websockets_subscriptions_test", nodeConfigs)
	s.net = testnet.PrepareFlowNetwork(s.T(), conf, flow.Localnet)

	// start the network
	s.T().Logf("starting flow network with docker containers")
	s.ctx, s.cancel = context.WithCancel(context.Background())

	s.net.Start(s.ctx)
}

// TestRestEventStreaming tests event streaming route on REST
func (s *WebsocketSubscriptionSuite) TestBlockHeaders() {
	//accessUrl := fmt.Sprintf("localhost:%s", s.net.ContainerByName(testnet.PrimaryAN).Port(testnet.GRPCPort))
	//grpcClient, err := getAccessAPIClient(accessUrl)
	//s.Require().NoError(err)

	restAddr := s.net.ContainerByName(testnet.PrimaryAN).Addr(testnet.RESTPort)
	client, err := getWSClient(s.ctx, getWebsocketsUrl(restAddr))
	s.Require().NoError(err)

	//subscriptionRequest := models.SubscribeMessageRequest{
	//	BaseMessageRequest: models.BaseMessageRequest{
	//		Action:    models.SubscribeAction,
	//		MessageID: uuid.New().String(),
	//	},
	//	Topic:     data_providers.BlockHeadersTopic,
	//	Arguments: models.Arguments{"block_status": parser.Finalized},
	//}
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
		var receivedResponse []*models.BlockHeaderMessageResponse
		clientMessageID := uuid.New().String()

		go func() {
			err := client.WriteJSON(s.subscribeMessageRequest(
				clientMessageID,
				data_providers.BlockHeadersTopic,
				models.Arguments{"block_status": parser.Finalized},
			))
			require.NoError(s.T(), err)

			time.Sleep(15 * time.Second)
			// close connection after 15 seconds
			//TODO: add unsubscribe
			client.Close()
		}()

		responseChan := make(chan interface{}) // Channel to handle different response types
		go func() {
			for {
				var resp interface{}
				err := client.ReadJSON(&resp)
				if err != nil {
					if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
						s.T().Logf("unexpected close error: %v", err)
						require.NoError(s.T(), err)
					}
					close(responseChan) // Close the response channel when the client connection is closed
					return
				}

				responseChan <- resp
				s.T().Logf("!!!! RESPONSE: %v", resp)
			}
		}()

		// Process received responses
		// collect received responses during 15 seconds
		//TODO: add verify for success response message
		for response := range responseChan {
			switch resp := response.(type) {
			case *models.BlockHeaderMessageResponse:
				receivedResponse = append(receivedResponse, resp)
				s.log.Info().Msgf("Received *BlockHeaderMessageResponse: %v", resp)
			case *models.BlockMessageResponse:
				s.log.Info().Msgf("Received *BlocksMessageResponse: %v", resp)
			case *models.BlockDigestMessageResponse:
				s.log.Info().Msgf("Received *BlockDigestsMessageResponse: %v", resp)

			case *models.BaseMessageResponse:
				s.log.Info().Msgf("Received *BaseMessageResponse: %v", resp)
			default:
				s.T().Errorf("unexpected response type: %v", resp)
			}
		}

		// check block headers
		s.receivedResponse(receivedResponse)
	})
}

func (s *WebsocketSubscriptionSuite) receivedResponse(receivedResponse []*models.BlockHeaderMessageResponse) {
	// make sure there are received block headers
	s.Require().GreaterOrEqual(len(receivedResponse), 1, "expect received block headers")

	// TODO: move creating grpc connection from receivedResponse
	grpcCtx, grpcCancel := context.WithCancel(s.ctx)
	defer grpcCancel()

	grpcAddr := s.net.ContainerByName(testnet.PrimaryAN).Addr(testnet.GRPCPort)
	grpcConn, err := grpc.DialContext(grpcCtx, grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(s.T(), err, "failed to connect to access node")
	defer grpcConn.Close()

	grpcClient := accessproto.NewAccessAPIClient(grpcConn)

	for _, receivedBlockHeaderResponse := range receivedResponse {
		receivedBlockHeader := receivedBlockHeaderResponse.Header
		// get block header by block id
		grpcResponse, err := MakeApiRequest(
			grpcClient.GetBlockHeaderByID,
			grpcCtx,
			&accessproto.GetBlockHeaderByIDRequest{
				Id: convert.IdentifierToMessage(receivedBlockHeader.ID()),
			},
		)
		require.NoError(s.T(), err)

		expectedBlockHeader := grpcResponse.Block
		s.Require().Equal(expectedBlockHeader.Height, receivedBlockHeader.Height, "expect the same block height")
		s.Require().Equal(expectedBlockHeader.Timestamp, receivedBlockHeader.Timestamp, "expect the same block timestamp")
		s.Require().Equal(expectedBlockHeader.ParentId, receivedBlockHeader.ParentID, "expect the same block parent id")
		s.Require().Equal(expectedBlockHeader.PayloadHash, receivedBlockHeader.PayloadHash, "expect the same block parent id")
	}
}

func (s *WebsocketSubscriptionSuite) validateBlockHeaders(
	grpcClient accessproto.AccessAPIClient,
	receivedResponses []*models.BlockHeaderMessageResponse,
) {
	require.NotEmpty(s.T(), receivedResponses, "expected received block headers")
	for _, response := range receivedResponses {
		grpcResponse, err := grpcClient.GetBlockHeaderByID(s.ctx, &accessproto.GetBlockHeaderByIDRequest{
			Id: convert.IdentifierToMessage(response.Header.ID()),
		})
		require.NoError(s.T(), err)

		expected := grpcResponse.Block
		require.Equal(s.T(), expected.Height, response.Header.Height)
		require.Equal(s.T(), expected.Timestamp, response.Header.Timestamp)
		require.Equal(s.T(), expected.ParentId, response.Header.ParentID)
		require.Equal(s.T(), expected.PayloadHash, response.Header.PayloadHash)
	}
}

func (s *WebsocketSubscriptionSuite) subscribeMessageRequest(clientMessageID string, topic string, arguments models.Arguments) interface{} {
	return models.SubscribeMessageRequest{
		BaseMessageRequest: models.BaseMessageRequest{
			Action:    models.SubscribeAction,
			MessageID: clientMessageID,
		},
		Topic:     topic,
		Arguments: arguments,
	}
}

func (s *WebsocketSubscriptionSuite) unsubscribeMessageRequest(clientMessageID string, subscriptionID string) interface{} {
	return models.UnsubscribeMessageRequest{
		BaseMessageRequest: models.BaseMessageRequest{
			Action:    models.UnsubscribeAction,
			MessageID: clientMessageID,
		},
		SubscriptionID: subscriptionID,
	}
}

func (s *WebsocketSubscriptionSuite) listSubscriptionsMessageRequest(clientMessageID string) interface{} {
	return models.ListSubscriptionsMessageRequest{
		BaseMessageRequest: models.BaseMessageRequest{
			Action:    models.ListSubscriptionsAction,
			MessageID: clientMessageID,
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
	go s.listenForResponses(client, responseChan)

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
