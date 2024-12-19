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
func (s *WebsocketSubscriptionSuite) TestBlockHeaders() {
	restAddr := s.net.ContainerByName(testnet.PrimaryAN).Addr(testnet.RESTPort)

	//TODO: move common to separate func to reuse for other subscriptions
	s.T().Run("block headers streaming", func(t *testing.T) {
		client, err := getWSClient(s.ctx, getWebsocketsUrl(restAddr))
		require.NoError(t, err)

		var receivedResponse []*models.BlockHeaderMessageResponse
		clientMessageID := uuid.New().String()

		go func() {
			err := client.WriteJSON(s.subscribeMessageRequest(clientMessageID, data_providers.BlockHeadersTopic, models.Arguments{"block_status": parser.Finalized}))
			require.NoError(s.T(), err)

			time.Sleep(15 * time.Second)
			// close connection after 15 seconds
			//TODO: add unsubscribe
			client.Close()
		}()

		responseChan := make(chan *models.BlockHeaderMessageResponse)
		go func() {
			for {
				resp := &models.BlockHeaderMessageResponse{}
				err := client.ReadJSON(resp)
				if err != nil {
					if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
						s.T().Logf("unexpected close error: %v", err)
						require.NoError(s.T(), err)
					}
					close(responseChan) // Close the response channel when the client connection is closed
					return
				}
				//TODO: add verify for success response message
				responseChan <- resp
			}
		}()

		// collect received responses during 15 seconds
		for response := range responseChan {
			receivedResponse = append(receivedResponse, response)
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

// getWebsocketsUrl is a helper function that creates websocket url
func getWebsocketsUrl(accessAddr string) string {
	u, _ := url.Parse("http://" + accessAddr + "/v1/ws")
	return u.String()
}
