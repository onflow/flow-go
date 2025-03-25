package cohort4

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	sdk "github.com/onflow/flow-go-sdk"
	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go-sdk/templates"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"

	restcommon "github.com/onflow/flow-go/engine/access/rest/common"
	commonmodels "github.com/onflow/flow-go/engine/access/rest/common/models"
	"github.com/onflow/flow-go/engine/access/rest/common/parser"
	"github.com/onflow/flow-go/engine/access/rest/util"
	"github.com/onflow/flow-go/engine/access/rest/websockets"
	"github.com/onflow/flow-go/engine/access/rest/websockets/data_providers"
	dpmodels "github.com/onflow/flow-go/engine/access/rest/websockets/data_providers/models"
	"github.com/onflow/flow-go/engine/access/rest/websockets/models"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/integration/tests/access/common"
	"github.com/onflow/flow-go/integration/tests/lib"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"

	accessproto "github.com/onflow/flow/protobuf/go/flow/access"
)

const InactivityTimeout = 20
const MaxSubscriptionsPerConnection = 5

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

	grpcClient        accessproto.AccessAPIClient
	serviceClient     *testnet.Client
	restAccessAddress string
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
		testnet.WithAdditionalFlagf("--websocket-max-subscriptions-per-connection=%d", MaxSubscriptionsPerConnection),
		testnet.WithAdditionalFlagf("--experimental-enable-websockets-stream-api=true"),
		testnet.WithMetricsServer(),
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

	sdkClient, err := s.net.ContainerByName(testnet.PrimaryAN).SDKClient()
	s.Require().NoError(err)

	s.grpcClient = sdkClient.RPCClient()

	s.serviceClient, err = s.net.ContainerByName(testnet.PrimaryAN).TestnetClient()
	s.Require().NoError(err)

	s.restAccessAddress = s.net.ContainerByName(testnet.PrimaryAN).Addr(testnet.RESTPort)

	// pause until the network is progressing
	var header *sdk.BlockHeader
	s.Require().Eventually(func() bool {
		header, err = s.serviceClient.GetLatestSealedBlockHeader(s.ctx)
		s.Require().NoError(err)

		return header.Height > 0
	}, 30*time.Second, 1*time.Second)
}

// TestWebsocketSubscriptions initializes a WebSocket client and runs a suite of WebSocket-related tests.
//
// This function ensures that all WebSocket tests run within a single setup to minimize system initialization time.
// New WebSocket-related tests should be added here to maintain efficiency.
//
// It executes the following test cases:
//   - Inactivity tracking
//   - Maximum subscriptions per connection
//   - Subscription error handling
//   - Unsubscription error handling
//   - Listing active subscriptions
//   - Valid subscription scenarios (happy cases)
//   - Subscription multiplexing
//
// The WebSocket client is properly closed after each sub-test execution to avoid resource leaks.
func (s *WebsocketSubscriptionSuite) TestWebsocketSubscriptions() {
	// NOTE: To minimize the system setup time for WebSocket tests,
	// the setup is performed once, and all tests run as sub-functions.
	// When adding a new WebSocket test, please include it here.
	s.testInactivityTracker()
	s.testMaxSubscriptionsPerConnection()
	s.testSubscriptionErrorCases()
	s.testUnsubscriptionErrorCases()
	s.testListOfSubscriptions()
	s.testHappyCases()
	s.testSubscriptionMultiplexing()
}

// TestInactivityTracker tests that the WebSocket connection closes due to inactivity
// after the specified timeout duration.
func (s *WebsocketSubscriptionSuite) testInactivityTracker() {
	// Steps:
	// 1. Establish a WebSocket connection to the server.
	// 2. Start a goroutine to listen for messages from the server.
	// 3. Wait for the server to close the connection due to inactivity.
	// 4. Validate that the actual inactivity duration is within the expected range.

	inactivityTickerPeriod := InactivityTimeout / 10 // determines the interval at which the inactivity ticker checks for inactivity
	expectedMinInactivityDuration := time.Duration(InactivityTimeout+inactivityTickerPeriod) * time.Second

	s.T().Run("no active subscription after connection creation", func(t *testing.T) {
		wsClient, err := common.GetWSClient(s.ctx, getWebsocketsUrl(s.restAccessAddress))
		s.Require().NoError(err)
		defer func() { s.Require().NoError(wsClient.Close()) }()

		actualInactivityDuration := monitorInactivity(t, wsClient, expectedMinInactivityDuration)
		// Verify that the connection does not close before the InactivityTimeout + inactivity ticker period.
		s.GreaterOrEqual(actualInactivityDuration, expectedMinInactivityDuration)
	})

	// Steps:
	// 1. Establish a WebSocket connection to the server.
	// 2. Subscribe to a topic and validate the subscription response.
	// 3. Unsubscribe from the topic and validate the unsubscription response.
	// 4. Wait for the server to close the connection due to inactivity.
	s.T().Run("all active subscriptions unsubscribed", func(t *testing.T) {
		// Step 1: Establish WebSocket connection
		wsClient, err := common.GetWSClient(s.ctx, getWebsocketsUrl(s.restAccessAddress))
		s.Require().NoError(err)
		defer func() { s.Require().NoError(wsClient.Close()) }()

		// Step 2: Subscribe to a topic
		subscriptionRequest := models.SubscribeMessageRequest{
			BaseMessageRequest: models.BaseMessageRequest{
				Action:         models.SubscribeAction,
				SubscriptionID: "events_id",
			},
			Topic: data_providers.EventsTopic,
		}

		s.Require().NoError(wsClient.WriteJSON(subscriptionRequest))

		_, baseResponses, _ := s.listenWebSocketResponses(
			wsClient,
			5*time.Second,
			subscriptionRequest.SubscriptionID,
		)
		s.Require().Equal(1, len(baseResponses))
		s.Require().Nil(baseResponses[0].Error)

		// Step 3: Unsubscribe from the topic
		unsubscribeRequest := models.UnsubscribeMessageRequest{
			BaseMessageRequest: models.BaseMessageRequest{
				Action:         models.UnsubscribeAction,
				SubscriptionID: subscriptionRequest.SubscriptionID,
			},
		}

		s.Require().NoError(wsClient.WriteJSON(unsubscribeRequest))

		var response models.BaseMessageResponse
		err = wsClient.ReadJSON(&response)
		s.Require().NoError(err, "failed to read unsubscribe response")
		s.Require().Nil(response.Error)

		// Step 4: Monitor inactivity after unsubscription
		actualInactivityDuration := monitorInactivity(s.T(), wsClient, expectedMinInactivityDuration)
		// Verify that the connection does not close before the InactivityTimeout + inactivity ticker period.
		s.GreaterOrEqual(actualInactivityDuration, expectedMinInactivityDuration)
	})
}

// testMaxSubscriptionsPerConnection validates the behavior of the WebSocket server
// when the number of subscriptions exceeds the configured maximum limit.
//
// Expected behavior:
// - For the first `MaxSubscriptionsPerConnection` requests, the server should respond with successful subscription messages.
// - On exceeding the subscription limit, the server should return an error response with a message.
func (s *WebsocketSubscriptionSuite) testMaxSubscriptionsPerConnection() {
	websocketsUrl := getWebsocketsUrl(s.restAccessAddress)
	wsClient, err := common.GetWSClient(s.ctx, websocketsUrl)
	s.Require().NoError(err)

	defer func() { s.Require().NoError(wsClient.Close()) }()

	blocksSubscriptionArguments := models.Arguments{"block_status": parser.Finalized}
	// Expected error message when exceeding the maximum subscription limit.
	expectedErrorMessage := fmt.Sprintf("error creating new subscription: %s", websockets.ErrMaxSubscriptionsReached.Error())

	// Loop to send subscription requests, including one request exceeding the limit.
	for i := 1; i <= MaxSubscriptionsPerConnection+1; i++ {
		// Create a subscription message request with a unique ID.
		subscriptionToBlocksRequest := s.subscribeMessageRequest(
			strconv.Itoa(i),
			data_providers.BlocksTopic,
			blocksSubscriptionArguments,
		)

		// send blocks subscription message
		err := wsClient.WriteJSON(subscriptionToBlocksRequest)
		s.Require().NoError(err, "failed to send subscription message")

		// Receive response
		_, baseResponses, _ := s.listenWebSocketResponses(wsClient, 2*time.Second, subscriptionToBlocksRequest.SubscriptionID)
		s.Require().Equal(1, len(baseResponses))
		subscribeResponse := baseResponses[0]

		if i <= MaxSubscriptionsPerConnection {
			s.Require().Nil(subscribeResponse.Error)
		} else {
			// Validate error response for exceeding the subscription limit.
			s.Require().Equal(expectedErrorMessage, subscribeResponse.Error.Message)
			s.Require().Equal(http.StatusTooManyRequests, subscribeResponse.Error.Code)
		}
	}
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

// testSubscriptionErrorCases tests error cases for subscriptions.
func (s *WebsocketSubscriptionSuite) testSubscriptionErrorCases() {
	tests := []struct {
		name            string
		message         models.SubscribeMessageRequest
		expectedErrMsg  string
		expectedErrCode int
	}{
		{
			name:            "Invalid Subscription ID",
			message:         s.subscribeMessageRequest("invalid_subscription_id", data_providers.BlocksTopic, models.Arguments{}), // id length > 20 symbols
			expectedErrMsg:  "error parsing subscription id: subscription ID provided by the client must not exceed 20 characters",
			expectedErrCode: http.StatusBadRequest,
		},
		{
			name:            "Invalid Topic",
			message:         s.subscribeMessageRequest("", "invalid_topic", models.Arguments{}),
			expectedErrMsg:  "error creating data provider", // Update based on expected error message
			expectedErrCode: http.StatusBadRequest,
		},
		{
			name:            "Invalid Arguments",
			message:         s.subscribeMessageRequest("", data_providers.BlocksTopic, models.Arguments{"invalid_arg": 42}),
			expectedErrMsg:  "error creating data provider",
			expectedErrCode: http.StatusBadRequest,
		},
		{
			name:            "Empty Topic",
			message:         s.subscribeMessageRequest("", "", models.Arguments{}),
			expectedErrMsg:  "error creating data provider",
			expectedErrCode: http.StatusBadRequest,
		},
	}

	wsClient, err := common.GetWSClient(s.ctx, getWebsocketsUrl(s.restAccessAddress))
	s.Require().NoError(err)
	defer func() { s.Require().NoError(wsClient.Close()) }()

	for _, tt := range tests {
		s.Run(tt.name, func() {
			// Send subscription message
			err := wsClient.WriteJSON(tt.message)
			s.Require().NoError(err, "failed to send subscription message")

			// Receive response
			var response models.BaseMessageResponse
			err = wsClient.ReadJSON(&response)
			s.Require().NoError(err, "failed to read subscription response")

			// Validate response
			s.Require().Equal(models.SubscribeAction, response.Action)
			s.Require().NotNil(response.Error)
			s.Contains(response.Error.Message, tt.expectedErrMsg)
			s.Require().Equal(tt.expectedErrCode, response.Error.Code)
		})
	}
}

// testUnsubscriptionErrorCases tests error cases for unsubscriptions.
func (s *WebsocketSubscriptionSuite) testUnsubscriptionErrorCases() {
	tests := []struct {
		name            string
		message         models.UnsubscribeMessageRequest
		expectedErrMsg  string
		expectedErrCode int
	}{
		{
			name:            "Invalid Subscription ID",
			message:         s.unsubscribeMessageRequest("invalid_subscription_id"),
			expectedErrMsg:  "error parsing subscription id: subscription ID provided by the client must not exceed 20 characters", // id length > 20 symbols
			expectedErrCode: http.StatusBadRequest,
		},
		{
			name:            "Non-Existent Subscription ID",
			message:         s.unsubscribeMessageRequest("non_existent_id"),
			expectedErrMsg:  "subscription not found", // not associated with an active subscription
			expectedErrCode: http.StatusNotFound,
		},
		{
			name:            "Empty Subscription ID",
			message:         s.unsubscribeMessageRequest(""),
			expectedErrMsg:  "error parsing subscription id: subscription ID provided by the client must not be empty",
			expectedErrCode: http.StatusBadRequest,
		},
	}

	wsClient, err := common.GetWSClient(s.ctx, getWebsocketsUrl(s.restAccessAddress))
	s.Require().NoError(err)
	defer func() { s.Require().NoError(wsClient.Close()) }()

	for _, tt := range tests {
		s.Run(tt.name, func() {
			// Send unsubscription message
			err := wsClient.WriteJSON(tt.message)
			s.Require().NoError(err, "failed to send unsubscription message")

			// Receive response
			var response models.BaseMessageResponse
			err = wsClient.ReadJSON(&response)
			s.Require().NoError(err, "failed to read unsubscription response")

			// Validate response
			s.Require().Equal(models.UnsubscribeAction, response.Action)
			s.Require().NotNil(response.Error)
			s.Contains(response.Error.Message, tt.expectedErrMsg)
			s.Require().Equal(tt.expectedErrCode, response.Error.Code)
		})
	}
}

// testListOfSubscriptions tests the websocket request for the list of active subscription and its response.
func (s *WebsocketSubscriptionSuite) testListOfSubscriptions() {
	wsClient, err := common.GetWSClient(s.ctx, getWebsocketsUrl(s.restAccessAddress))
	s.Require().NoError(err)
	defer func() { s.Require().NoError(wsClient.Close()) }()

	// 1. Create blocks subscription request message
	blocksSubscriptionID := "blocks_id"
	blocksSubscriptionArguments := models.Arguments{"block_status": parser.Finalized}
	subscriptionToBlocksRequest := s.subscribeMessageRequest(
		blocksSubscriptionID,
		data_providers.BlocksTopic,
		blocksSubscriptionArguments,
	)
	// send blocks subscription message
	s.Require().NoError(wsClient.WriteJSON(subscriptionToBlocksRequest))

	// verify success subscribe response
	_, baseResponses, _ := s.listenWebSocketResponses(wsClient, 1*time.Second, blocksSubscriptionID)
	s.Require().Equal(1, len(baseResponses))
	s.Require().Nil(baseResponses[0].Error)

	// 2. Create block headers subscription request message
	blockHeadersSubscriptionID := "block_headers_id"
	blockHeadersSubscriptionArguments := models.Arguments{"block_status": parser.Finalized}
	subscriptionToBlockHeadersRequest := s.subscribeMessageRequest(
		blockHeadersSubscriptionID,
		data_providers.BlockHeadersTopic,
		blockHeadersSubscriptionArguments,
	)
	// send block headers subscription message
	s.Require().NoError(wsClient.WriteJSON(subscriptionToBlockHeadersRequest))

	// verify success subscribe response
	_, baseResponses, _ = s.listenWebSocketResponses(wsClient, 1*time.Second, blockHeadersSubscriptionID)
	s.Require().Equal(1, len(baseResponses))
	s.Require().Nil(baseResponses[0].Error)

	// 3. Create list of subscription request message
	listOfSubscriptionRequest := models.ListSubscriptionsMessageRequest{
		BaseMessageRequest: models.BaseMessageRequest{
			Action: models.ListSubscriptionsAction,
		},
	}
	// send list of subscription message
	s.Require().NoError(wsClient.WriteJSON(listOfSubscriptionRequest))

	_, _, responses := s.listenWebSocketResponses(wsClient, 1*time.Second, "")

	// validate list of active subscriptions response
	s.Require().Equal(1, len(responses))
	listOfSubscriptionResponse := responses[0]
	expectedSubscriptions := []*models.SubscriptionEntry{
		{
			SubscriptionID: blocksSubscriptionID,
			Topic:          data_providers.BlocksTopic,
			Arguments:      blocksSubscriptionArguments,
		},
		{
			SubscriptionID: blockHeadersSubscriptionID,
			Topic:          data_providers.BlockHeadersTopic,
			Arguments:      blockHeadersSubscriptionArguments,
		},
	}
	s.Require().Len(listOfSubscriptionResponse.Subscriptions, len(expectedSubscriptions))

	for i, expected := range expectedSubscriptions {
		actual := listOfSubscriptionResponse.Subscriptions[i]
		s.Require().Equal(expected.SubscriptionID, actual.SubscriptionID)
		for key, value := range expected.Arguments {
			s.Require().Equal(value, actual.Arguments[key])
		}
	}
}

// testHappyCases tests various scenarios for websocket subscriptions including
// streaming blocks, block headers, block digests, events, account statuses,
// and transaction statuses.
func (s *WebsocketSubscriptionSuite) testHappyCases() {
	tests := []struct {
		name                               string
		topic                              string
		prepareArguments                   func() models.Arguments
		listenSubscriptionResponseDuration time.Duration
		testUnsubscribe                    bool
	}{
		{
			name:  "Blocks streaming",
			topic: data_providers.BlocksTopic,
			prepareArguments: func() models.Arguments {
				return models.Arguments{"block_status": parser.Finalized}
			},
			listenSubscriptionResponseDuration: 5 * time.Second,
			testUnsubscribe:                    true,
		},
		{
			name:  "Block headers streaming",
			topic: data_providers.BlockHeadersTopic,
			prepareArguments: func() models.Arguments {
				return models.Arguments{"block_status": parser.Finalized}
			},
			listenSubscriptionResponseDuration: 5 * time.Second,
			testUnsubscribe:                    true,
		},
		{
			name:  "Block digests streaming",
			topic: data_providers.BlockDigestsTopic,
			prepareArguments: func() models.Arguments {
				return models.Arguments{"block_status": parser.Finalized}
			},
			listenSubscriptionResponseDuration: 5 * time.Second,
			testUnsubscribe:                    true,
		},
		{
			name:  "Events streaming",
			topic: data_providers.EventsTopic,
			prepareArguments: func() models.Arguments {
				return models.Arguments{}
			},
			listenSubscriptionResponseDuration: 5 * time.Second,
			testUnsubscribe:                    true,
		},
		{
			name:  "Account statuses streaming",
			topic: data_providers.AccountStatusesTopic,
			prepareArguments: func() models.Arguments {
				tx := s.createAccountTx()
				err := s.serviceClient.SendTransaction(s.ctx, tx)
				s.Require().NoError(err)
				s.T().Logf("txId %v", flow.Identifier(tx.ID()))

				return models.Arguments{
					"event_types": []string{"flow.AccountCreated", "flow.AccountKeyAdded"},
				}
			},
			listenSubscriptionResponseDuration: 10 * time.Second,
			testUnsubscribe:                    true,
		},
		{
			name:  "Transaction statuses streaming",
			topic: data_providers.TransactionStatusesTopic,
			prepareArguments: func() models.Arguments {
				tx := s.createAccountTx()

				// Send the transaction
				err := s.serviceClient.SendTransaction(s.ctx, tx)
				s.Require().NoError(err)
				s.T().Logf("txId %v", flow.Identifier(tx.ID()))

				return models.Arguments{
					"tx_id": tx.ID().String(),
				}
			},
			listenSubscriptionResponseDuration: 10 * time.Second,
			testUnsubscribe:                    false,
		},
		{
			name:  "Send and subscribe to transaction statuses",
			topic: data_providers.SendAndGetTransactionStatusesTopic,
			prepareArguments: func() models.Arguments {
				tx := s.createAccountTx()

				convertToProposalKey := func(key sdk.ProposalKey) commonmodels.ProposalKey {
					return commonmodels.ProposalKey{
						Address:        flow.Address(key.Address).String(),
						KeyIndex:       strconv.FormatUint(uint64(key.KeyIndex), 10),
						SequenceNumber: strconv.FormatUint(key.SequenceNumber, 10),
					}
				}

				convertToArguments := func(arguments [][]byte) []string {
					wsArguments := make([]string, len(arguments))
					for i, arg := range arguments {
						wsArguments[i] = util.ToBase64(arg)
					}

					return wsArguments
				}

				convertToAuthorizers := func(authorizers []sdk.Address) []string {
					wsAuthorizers := make([]string, len(authorizers))
					for i, authorizer := range authorizers {
						wsAuthorizers[i] = authorizer.String()
					}

					return wsAuthorizers
				}

				convertToSig := func(sigs []sdk.TransactionSignature) []commonmodels.TransactionSignature {
					wsSigs := make([]commonmodels.TransactionSignature, len(sigs))
					for i, sig := range sigs {
						wsSigs[i] = commonmodels.TransactionSignature{
							Address:   sig.Address.String(),
							KeyIndex:  strconv.FormatUint(uint64(sig.KeyIndex), 10),
							Signature: util.ToBase64(sig.Signature),
						}
					}

					return wsSigs
				}
				return models.Arguments{
					"script":              util.ToBase64(tx.Script),
					"arguments":           convertToArguments(tx.Arguments),
					"reference_block_id":  tx.ReferenceBlockID.String(),
					"gas_limit":           strconv.FormatUint(tx.GasLimit, 10),
					"payer":               tx.Payer.String(),
					"proposal_key":        convertToProposalKey(tx.ProposalKey),
					"authorizers":         convertToAuthorizers(tx.Authorizers),
					"payload_signatures":  convertToSig(tx.PayloadSignatures),
					"envelope_signatures": convertToSig(tx.EnvelopeSignatures),
				}
			},
			listenSubscriptionResponseDuration: 10 * time.Second,
			testUnsubscribe:                    false,
		},
	}

	for _, tt := range tests {
		// This test cases handles the lifecycle of a websocket connection for a specific subscription,
		// including sending a subscription and unsubscription requests, listening for incoming responses, and validating
		// them using a provided validation function.
		s.Run(tt.name, func() {
			// Step 1: Establish a WebSocket connection
			wsClient, err := common.GetWSClient(s.ctx, getWebsocketsUrl(s.restAccessAddress))
			s.Require().NoError(err)
			defer func() { s.Require().NoError(wsClient.Close()) }()

			// Step 2: Create and send the subscription request
			subscriptionRequest := s.subscribeMessageRequest(
				"dummy_id",
				tt.topic,
				tt.prepareArguments(),
			)
			s.Require().NoError(wsClient.WriteJSON(subscriptionRequest))

			// Step 3: Listen for WebSocket responses for the specified duration
			responses, baseMessageResponses, _ := s.listenWebSocketResponses(
				wsClient,
				tt.listenSubscriptionResponseDuration,
				subscriptionRequest.SubscriptionID,
			)

			// Step 4: Validate the subscription response
			s.Require().Equal(1, len(baseMessageResponses), "expected one subscription response")
			s.Require().Nil(baseMessageResponses[0].Error)

			// Step 5: Use the provided validation function to check received responses
			s.validate(
				subscriptionRequest.SubscriptionID,
				subscriptionRequest.Topic,
				responses,
			)

			// Step 6: Optionally unsubscribe from the topic
			if tt.testUnsubscribe {
				// Create an unsubscription request
				unsubscriptionRequest := s.unsubscribeMessageRequest(subscriptionRequest.SubscriptionID)

				// Send the unsubscription request to the WebSocket server
				s.Require().NoError(wsClient.WriteJSON(unsubscriptionRequest))

				// Step 6.1: Read and validate the unsubscription response
				var response models.BaseMessageResponse
				err := wsClient.ReadJSON(&response)
				s.Require().NoError(err, "failed to read unsubscription response")
				s.Require().Nil(response.Error)
			}
		})
	}
}

// testSubscriptionMultiplexing verifies that when subscribing to multiple channels simultaneously,
// all expected messages are received correctly, ensuring subscription multiplexing works as expected.
func (s *WebsocketSubscriptionSuite) testSubscriptionMultiplexing() {
	// Define the list of subscriptions with topic names and arguments required for each topic.
	subscriptions := []struct {
		name             string
		topic            string
		prepareArguments func() models.Arguments
	}{
		{
			name:  "Blocks streaming",
			topic: data_providers.BlocksTopic,
			prepareArguments: func() models.Arguments {
				return models.Arguments{"block_status": parser.Finalized}
			},
		},
		{
			name:  "Block headers streaming",
			topic: data_providers.BlockHeadersTopic,
			prepareArguments: func() models.Arguments {
				return models.Arguments{"block_status": parser.Finalized}
			},
		},
		{
			name:  "Block digests streaming",
			topic: data_providers.BlockDigestsTopic,
			prepareArguments: func() models.Arguments {
				return models.Arguments{"block_status": parser.Finalized}
			},
		},
	}

	// Step 1: Establish a WebSocket connection to the server.
	wsClient, err := common.GetWSClient(s.ctx, getWebsocketsUrl(s.restAccessAddress))
	s.Require().NoError(err)
	defer func() { s.Require().NoError(wsClient.Close()) }()

	// Step 2: Subscribe to all topics and handle the responses
	subscriptionRequests := make(map[string]models.SubscribeMessageRequest)
	subscribeResponses := make([]models.SubscribeMessageResponse, 0)
	unsubscribeResponses := make([]models.UnsubscribeMessageResponse, 0)
	messageBuckets := make(map[string][]dpmodels.BaseDataProvidersResponse)

	parseResponse := func(t *testing.T, msg json.RawMessage) (string, interface{}) {
		var message models.BaseMessageResponse
		err := json.Unmarshal(msg, &message)
		s.Require().NoError(err, "failed to unmarshal message")

		switch message.Action {
		case models.SubscribeAction:
			var m models.SubscribeMessageResponse
			err = json.Unmarshal(msg, &m)
			s.Require().NoError(err, "failed to unmarshal subscribe message")
			return message.SubscriptionID, m

		case models.UnsubscribeAction:
			var m models.UnsubscribeMessageResponse
			err = json.Unmarshal(msg, &m)
			s.Require().NoError(err, "failed to unmarshal unsubscribe message")
			return message.SubscriptionID, m

		default:
			var m dpmodels.BaseDataProvidersResponse
			err = json.Unmarshal(msg, &m)
			s.Require().NoError(err, "failed to unmarshal unsubscribe message")
			return message.SubscriptionID, m
		}
	}

	// Step 2. Launch a router to collect messages from the WebSocket server into per subscription buckets
	// will shutdown once all subscriptions are done
	routerStopped := make(chan struct{})
	go func() {
		defer close(routerStopped)

		for {
			var rawMessage json.RawMessage
			err := wsClient.ReadJSON(&rawMessage)
			s.Require().NoError(err)

			subID, message := parseResponse(s.T(), rawMessage)

			switch v := message.(type) {
			case models.SubscribeMessageResponse:
				subscribeResponses = append(subscribeResponses, v)
			case models.UnsubscribeMessageResponse:
				unsubscribeResponses = append(unsubscribeResponses, v)
			case dpmodels.BaseDataProvidersResponse:
				messageBuckets[subID] = append(messageBuckets[subID], v)
			default:
				s.Failf("unexpected message type", "got type: %T: %+v", message, message)
			}

			// break out of router once the last expected unsubscribe message is received
			if len(unsubscribeResponses) == len(subscriptions) {
				return
			}
		}
	}()

	// Step 3: Subscribe to all topics and handle the responses
	for i, sub := range subscriptions {
		subId := fmt.Sprintf("sub_%d", i+1) // Generate a unique subscription ID for each subscription.
		subscriptionRequest := s.subscribeMessageRequest(subId, sub.topic, sub.prepareArguments())

		// Send the subscription request.
		s.Require().NoError(wsClient.WriteJSON(subscriptionRequest))
		subscriptionRequests[subId] = subscriptionRequest
	}

	// Step 4. Unsubscribe from all topics after a short delay.
	time.Sleep(time.Second)
	for _, sub := range subscriptionRequests {
		// Send the unsubscription request.
		unsubscriptionRequest := s.unsubscribeMessageRequest(sub.SubscriptionID)
		s.Require().NoError(wsClient.WriteJSON(unsubscriptionRequest))
	}

	unittest.RequireCloseBefore(s.T(), routerStopped, 5*time.Second, "timed out waiting for router to stop")

	// Step 5: Validate the collected messages to ensure they are received for all active subscriptions.

	s.Require().Len(subscribeResponses, len(subscriptions), "Missing subscribe messages: have: %+v", subscribeResponses)
	s.Require().Len(unsubscribeResponses, len(subscriptions), "Missing unsubscribe messages: have: %+v", unsubscribeResponses)

	blockResponses := make(map[string]int)

	for subID, responses := range messageBuckets {
		s.Require().NotEmpty(responses, "Expected at least 1 messages for subscription ID: %s", subID)
		s.validate(subID, subscriptionRequests[subID].Topic, responses)

		for _, response := range responses {
			payloadRaw := s.validateBaseDataProvidersResponse(response.SubscriptionID, response.Topic, response)

			switch response.Topic {
			case data_providers.BlockDigestsTopic:
				var payload dpmodels.BlockDigest
				err := restcommon.ParseBody(bytes.NewReader(payloadRaw), &payload)
				s.Require().NoError(err)
				blockResponses[payload.Height]++
			case data_providers.BlockHeadersTopic:
				var payload commonmodels.BlockHeader
				err := restcommon.ParseBody(bytes.NewReader(payloadRaw), &payload)
				s.Require().NoError(err)
				blockResponses[payload.Height]++
			case data_providers.BlocksTopic:
				var payload commonmodels.Block
				err := restcommon.ParseBody(bytes.NewReader(payloadRaw), &payload)
				s.Require().NoError(err)
				blockResponses[payload.Header.Height]++
			default:
				s.Failf("unexpected message topic", "got: %s", response.Topic)
			}
		}
	}

	for height, count := range blockResponses {
		s.Assert().Equalf(len(subscriptions), count, "Expected %d responses for block height %s, but got: %d", len(subscriptions), height, count)
	}
}

// validate checks if the received responses for a given subscription ID and topic
// match the expected data format and correctness.
//
// It dispatches validation to specific topic handlers based on the topic type.
//
// Parameters:
//   - subscriptionId: The unique identifier of the WebSocket subscription.
//   - topic: The topic associated with the subscription (e.g., blocks, events, transactions).
//   - responses: A slice of BaseDataProvidersResponse containing the received data.
//
// If the topic is invalid or unsupported, it logs a warning instead of failing the test.
func (s *WebsocketSubscriptionSuite) validate(subscriptionId string, topic string, responses []dpmodels.BaseDataProvidersResponse) {
	switch topic {
	case data_providers.BlocksTopic:
		s.validateBlocks(subscriptionId, topic, responses)
	case data_providers.BlockHeadersTopic:
		s.validateBlockHeaders(subscriptionId, topic, responses)
	case data_providers.BlockDigestsTopic:
		s.validateBlockDigests(subscriptionId, topic, responses)
	case data_providers.EventsTopic:
		s.validateEvents(subscriptionId, topic, responses)
	case data_providers.AccountStatusesTopic:
		s.validateAccountStatuses(subscriptionId, topic, responses)
	case data_providers.TransactionStatusesTopic, data_providers.SendAndGetTransactionStatusesTopic:
		s.validateTransactionStatuses(subscriptionId, topic, responses)
	default:
		s.T().Logf("invalid topic to validate %s", topic)
	}
}

// validateBlocks validates the received block responses against gRPC responses.
func (s *WebsocketSubscriptionSuite) validateBlocks(
	expectedSubscriptionID string,
	expectedTopic string,
	receivedResponses []dpmodels.BaseDataProvidersResponse,
) {
	s.Require().NotEmpty(receivedResponses, "expected received block headers")

	for _, response := range receivedResponses {
		payloadRaw := s.validateBaseDataProvidersResponse(expectedSubscriptionID, expectedTopic, response)

		var payload commonmodels.Block
		err := restcommon.ParseBody(bytes.NewReader(payloadRaw), &payload)
		s.Require().NoError(err)

		id, err := flow.HexStringToIdentifier(payload.Header.Id)
		s.Require().NoError(err)

		grpcResponse, err := s.grpcClient.GetBlockByID(s.ctx, &accessproto.GetBlockByIDRequest{
			Id: convert.IdentifierToMessage(id),
		})
		s.Require().NoError(err)

		grpcExpected := grpcResponse.Block
		s.Require().Equal(convert.MessageToIdentifier(grpcExpected.Id).String(), payload.Header.Id)
		s.Require().Equal(util.FromUint(grpcExpected.Height), payload.Header.Height)
		s.Require().Equal(grpcExpected.Timestamp.AsTime(), payload.Header.Timestamp)
		s.Require().Equal(convert.MessageToIdentifier(grpcExpected.ParentId).String(), payload.Header.ParentId)
	}
}

// validateBlockHeaders validates the received block header responses against gRPC responses.
func (s *WebsocketSubscriptionSuite) validateBlockHeaders(
	expectedSubscriptionID string,
	expectedTopic string,
	receivedResponses []dpmodels.BaseDataProvidersResponse,
) {
	s.Require().NotEmpty(receivedResponses, "expected received block headers")

	for _, response := range receivedResponses {
		payloadRaw := s.validateBaseDataProvidersResponse(expectedSubscriptionID, expectedTopic, response)

		var payload commonmodels.BlockHeader
		err := restcommon.ParseBody(bytes.NewReader(payloadRaw), &payload)
		s.Require().NoError(err)

		id, err := flow.HexStringToIdentifier(payload.Id)
		s.Require().NoError(err)

		grpcResponse, err := s.grpcClient.GetBlockHeaderByID(s.ctx, &accessproto.GetBlockHeaderByIDRequest{
			Id: convert.IdentifierToMessage(id),
		})
		s.Require().NoError(err)

		grpcExpected := grpcResponse.Block

		s.Require().Equal(convert.MessageToIdentifier(grpcExpected.Id).String(), payload.Id)
		s.Require().Equal(util.FromUint(grpcExpected.Height), payload.Height)
		s.Require().Equal(grpcExpected.Timestamp.AsTime(), payload.Timestamp)
		s.Require().Equal(convert.MessageToIdentifier(grpcExpected.ParentId).String(), payload.ParentId)
	}
}

// validateBlockDigests validates the received block digest responses against gRPC responses.
func (s *WebsocketSubscriptionSuite) validateBlockDigests(
	expectedSubscriptionID string,
	expectedTopic string,
	receivedResponses []dpmodels.BaseDataProvidersResponse,
) {
	s.Require().NotEmpty(receivedResponses, "expected received block digests")

	for _, response := range receivedResponses {
		payloadRaw := s.validateBaseDataProvidersResponse(expectedSubscriptionID, expectedTopic, response)

		var payload dpmodels.BlockDigest
		err := restcommon.ParseBody(bytes.NewReader(payloadRaw), &payload)
		s.Require().NoError(err)

		id, err := flow.HexStringToIdentifier(payload.BlockId)
		s.Require().NoError(err)

		grpcResponse, err := s.grpcClient.GetBlockHeaderByID(s.ctx, &accessproto.GetBlockHeaderByIDRequest{
			Id: convert.IdentifierToMessage(id),
		})
		s.Require().NoError(err)

		grpcExpected := grpcResponse.Block

		s.Require().Equal(convert.MessageToIdentifier(grpcExpected.Id).String(), payload.BlockId)
		s.Require().Equal(util.FromUint(grpcExpected.Height), payload.Height)
		s.Require().Equal(grpcExpected.Timestamp.AsTime(), payload.Timestamp)
	}
}

// validateEvents is a helper function that encapsulates logic for comparing received events from rest state streaming and
// events which received from grpc api.
func (s *WebsocketSubscriptionSuite) validateEvents(
	expectedSubscriptionID string,
	expectedTopic string,
	receivedResponses []dpmodels.BaseDataProvidersResponse,
) {
	// make sure there are received events
	s.Require().NotEmpty(receivedResponses, "expect received events")

	expectedCounter := uint64(0)
	for _, response := range receivedResponses {
		payloadRaw := s.validateBaseDataProvidersResponse(expectedSubscriptionID, expectedTopic, response)

		var payload dpmodels.EventResponse
		err := restcommon.ParseBody(bytes.NewReader(payloadRaw), &payload)
		s.Require().NoError(err)

		s.Require().Equal(expectedCounter, payload.MessageIndex)
		expectedCounter++

		blockId, err := flow.HexStringToIdentifier(payload.BlockId)
		s.Require().NoError(err)

		s.validateEventsForBlock(
			payload.BlockHeight,
			payload.Events,
			blockId,
		)
	}
}

// validateAccountStatuses is a helper function that encapsulates logic for comparing received account statuses.
func (s *WebsocketSubscriptionSuite) validateAccountStatuses(
	expectedSubscriptionID string,
	expectedTopic string,
	receivedResponses []dpmodels.BaseDataProvidersResponse,
) {
	s.Require().NotEmpty(receivedResponses, "expected received block digests")

	expectedCounter := uint64(0)
	for _, response := range receivedResponses {
		payloadRaw := s.validateBaseDataProvidersResponse(expectedSubscriptionID, expectedTopic, response)

		var payload dpmodels.AccountStatusesResponse
		err := restcommon.ParseBody(bytes.NewReader(payloadRaw), &payload)
		s.Require().NoError(err)

		s.Require().Equal(expectedCounter, payload.MessageIndex)
		expectedCounter++

		blockId, err := flow.HexStringToIdentifier(payload.BlockID)
		s.Require().NoError(err)

		for _, events := range payload.AccountEvents {
			s.validateEventsForBlock(payload.Height, events, blockId)
		}
	}
}

// groupEventsByType groups events by their type.
func groupEventsByType(events commonmodels.Events) map[string]commonmodels.Events {
	eventMap := make(map[string]commonmodels.Events)
	for _, event := range events {
		eventType := event.Type_
		eventMap[eventType] = append(eventMap[eventType], event)
	}

	return eventMap
}

// validateEventsForBlock validates events against the gRPC response for a specific block.
func (s *WebsocketSubscriptionSuite) validateEventsForBlock(blockHeight string, events []commonmodels.Event, blockID flow.Identifier) {
	receivedEventMap := groupEventsByType(events)

	for eventType, receivedEventList := range receivedEventMap {
		// Get events by block ID and event type
		response, err := s.grpcClient.GetEventsForBlockIDs(
			s.ctx,
			&accessproto.GetEventsForBlockIDsRequest{
				BlockIds: [][]byte{convert.IdentifierToMessage(blockID)},
				Type:     eventType,
			},
		)
		s.Require().NoError(err)
		s.Require().Equal(1, len(response.Results), "expect to get 1 result")

		expectedEventsResult := response.Results[0]
		s.Require().Equal(util.FromUint(expectedEventsResult.BlockHeight), blockHeight, "expect the same block height")
		s.Require().Equal(len(expectedEventsResult.Events), len(receivedEventList), "expect the same count of events: want: %+v, got: %+v", expectedEventsResult.Events, receivedEventList)

		for i, event := range receivedEventList {
			expectedEvent := expectedEventsResult.Events[i]

			s.Require().Equal(util.FromUint(expectedEvent.EventIndex), event.EventIndex, "expect the same event index")
			s.Require().Equal(convert.MessageToIdentifier(expectedEvent.TransactionId).String(), event.TransactionId, "expect the same transaction id")
			s.Require().Equal(util.FromUint(expectedEvent.TransactionIndex), event.TransactionIndex, "expect the same transaction index")
		}
	}
}

// validateTransactionStatuses is a helper function that encapsulates logic for comparing received transaction statuses.
func (s *WebsocketSubscriptionSuite) validateTransactionStatuses(
	expectedSubscriptionID string,
	expectedTopic string,
	receivedResponses []dpmodels.BaseDataProvidersResponse,
) {
	expectedCount := 4 // pending, finalized, executed, sealed
	s.Require().Equal(expectedCount, len(receivedResponses), fmt.Sprintf("expected %d transaction statuses", expectedCount))

	expectedCounter := uint64(0)
	lastReportedTxStatus := commonmodels.PENDING_TransactionStatus

	// Define the expected sequence of statuses
	// Expected order: pending(0) -> finalized(1) -> executed(2) -> sealed(3)
	expectedStatuses := []commonmodels.TransactionStatus{
		commonmodels.PENDING_TransactionStatus,
		commonmodels.FINALIZED_TransactionStatus,
		commonmodels.EXECUTED_TransactionStatus,
		commonmodels.SEALED_TransactionStatus,
	}

	for _, response := range receivedResponses {
		payloadRaw := s.validateBaseDataProvidersResponse(expectedSubscriptionID, expectedTopic, response)

		var payload dpmodels.TransactionStatusesResponse
		err := restcommon.ParseBody(bytes.NewReader(payloadRaw), &payload)
		s.Require().NoError(err)

		s.Require().Equal(expectedCounter, payload.MessageIndex)

		payloadStatus := *payload.TransactionResult.Status

		// Check if all statuses received one by one. The subscription should send responses for each of the statuses,
		// and the message should be sent in the order of transaction statuses.
		s.Require().Equal(expectedStatuses[expectedCounter], payloadStatus)

		expectedCounter++
		lastReportedTxStatus = payloadStatus
	}
	// Check, if the last transaction status is sealed.
	s.Require().Equal(commonmodels.SEALED_TransactionStatus, lastReportedTxStatus)
}

// validateBaseDataProvidersResponse validates the subscription ID, topic, and converts the payload to JSON.
func (s *WebsocketSubscriptionSuite) validateBaseDataProvidersResponse(
	expectedSubscriptionID string,
	expectedTopic string,
	response dpmodels.BaseDataProvidersResponse,
) []byte {
	// Step 1: Validate Subscription ID and Topic
	s.Require().Equal(expectedSubscriptionID, response.SubscriptionID)
	s.Require().Equal(expectedTopic, response.Topic)

	// Step 2: Convert the payload map to JSON
	payloadRaw, err := json.Marshal(response.Payload)
	s.Require().NoError(err, "failed to marshal payload: %w", err)

	return payloadRaw
}

// subscribeMessageRequest creates a subscription message request.
func (s *WebsocketSubscriptionSuite) subscribeMessageRequest(
	subscriptionID string,
	topic string,
	arguments models.Arguments,
) models.SubscribeMessageRequest {
	return models.SubscribeMessageRequest{
		BaseMessageRequest: models.BaseMessageRequest{
			Action:         models.SubscribeAction,
			SubscriptionID: subscriptionID,
		},
		Topic:     topic,
		Arguments: arguments,
	}
}

// unsubscribeMessageRequest creates an unsubscribe message request.
func (s *WebsocketSubscriptionSuite) unsubscribeMessageRequest(subscriptionID string) models.UnsubscribeMessageRequest {
	return models.UnsubscribeMessageRequest{
		BaseMessageRequest: models.BaseMessageRequest{
			Action:         models.UnsubscribeAction,
			SubscriptionID: subscriptionID,
		},
	}
}

// getWebsocketsUrl is a helper function that creates websocket url.
func getWebsocketsUrl(accessAddr string) string {
	u, _ := url.Parse("http://" + accessAddr + "/v1/ws")
	return u.String()
}

// listenWebSocketResponses listens for websocket responses for a specified duration
// and unmarshalls them into expected types.
//
// Parameters:
//   - client: The websocket connection to read messages from.
//   - duration: The maximum time to listen for messages before stopping.
//   - subscriptionID: The subscription ID used to filter relevant responses.
func (s *WebsocketSubscriptionSuite) listenWebSocketResponses(
	client *websocket.Conn,
	duration time.Duration,
	subscriptionID string,
) (
	baseDataProvidersResponses []dpmodels.BaseDataProvidersResponse,
	baseMessageResponses []models.BaseMessageResponse,
	listSubscriptionsMessageResponses []models.ListSubscriptionsMessageResponse,
) {
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			s.T().Logf("stopping websocket response listener after %s", duration)
			return baseDataProvidersResponses, baseMessageResponses, listSubscriptionsMessageResponses
		default:
			_, messageBytes, err := client.ReadMessage()
			if err != nil {
				s.T().Logf("websocket error: %v", err)

				var closeErr *websocket.CloseError
				if errors.As(err, &closeErr) {
					return baseDataProvidersResponses, baseMessageResponses, listSubscriptionsMessageResponses
				}

				s.Require().FailNow(fmt.Sprintf("unexpected websocket error, %v", err))
			}

			var baseResp models.BaseMessageResponse
			err = restcommon.ParseBody(bytes.NewReader(messageBytes), &baseResp)
			if err == nil && baseResp.SubscriptionID == subscriptionID {
				baseMessageResponses = append(baseMessageResponses, baseResp)
				continue
			}

			var listResp models.ListSubscriptionsMessageResponse
			err = restcommon.ParseBody(bytes.NewReader(messageBytes), &listResp)
			if err == nil && listResp.Action == models.ListSubscriptionsAction {
				listSubscriptionsMessageResponses = append(listSubscriptionsMessageResponses, listResp)
				continue
			}

			var baseDataProvidersResponse dpmodels.BaseDataProvidersResponse
			err = restcommon.ParseBody(bytes.NewReader(messageBytes), &baseDataProvidersResponse)
			if err == nil && baseDataProvidersResponse.SubscriptionID == subscriptionID {
				baseDataProvidersResponses = append(baseDataProvidersResponses, baseDataProvidersResponse)
			}
		}
	}
}

// createAndSendTx creates a new account transaction.
func (s *WebsocketSubscriptionSuite) createAccountTx() *sdk.Transaction {
	latestBlockID, err := s.serviceClient.GetLatestBlockID(s.ctx)
	s.Require().NoError(err)

	// create new account to deploy Counter to
	accountPrivateKey := lib.RandomPrivateKey()

	accountKey := sdk.NewAccountKey().
		FromPrivateKey(accountPrivateKey).
		SetHashAlgo(sdkcrypto.SHA3_256).
		SetWeight(sdk.AccountKeyWeightThreshold)

	serviceAddress := sdk.Address(s.serviceClient.Chain.ServiceAddress())

	// Generate the account creation transaction
	createAccountTx, err := templates.CreateAccount(
		[]*sdk.AccountKey{accountKey},
		nil, serviceAddress)
	s.Require().NoError(err)

	// Generate the account creation transaction
	createAccountTx.
		SetReferenceBlockID(sdk.Identifier(latestBlockID)).
		SetProposalKey(serviceAddress, 0, s.serviceClient.GetAndIncrementSeqNumber()).
		SetPayer(serviceAddress).
		SetComputeLimit(flow.DefaultMaxTransactionGasLimit)

	createAccountTx, err = s.serviceClient.SignTransaction(createAccountTx)
	s.Require().NoError(err)

	return createAccountTx
}
