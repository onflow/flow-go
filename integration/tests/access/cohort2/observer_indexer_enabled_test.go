package cohort2

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	sdk "github.com/onflow/flow-go-sdk"
	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go-sdk/templates"
	"github.com/onflow/flow-go/engine/access/rpc/backend"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/integration/tests/lib"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"

	accessproto "github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/entities"
)

func TestObserverIndexerEnabled(t *testing.T) {
	suite.Run(t, new(ObserverIndexerEnabledSuite))
}

// ObserverIndexerEnabledSuite tests the observer with the indexer enabled.
// It uses ObserverSuite as a base to reuse the test cases that need to be run for any observer variation.
type ObserverIndexerEnabledSuite struct {
	ObserverSuite
}

// SetupTest sets up the test suite by starting the network and preparing the observer client.
// By overriding this function, we can ensure that the observer is started with correct parameters and select
// the RPCs and REST endpoints that are tested.
func (s *ObserverIndexerEnabledSuite) SetupTest() {
	s.localRpc = map[string]struct{}{
		"Ping":                           {},
		"GetLatestBlockHeader":           {},
		"GetBlockHeaderByID":             {},
		"GetBlockHeaderByHeight":         {},
		"GetLatestBlock":                 {},
		"GetBlockByID":                   {},
		"GetBlockByHeight":               {},
		"GetLatestProtocolStateSnapshot": {},
		"GetNetworkParameters":           {},
	}

	s.localRest = map[string]struct{}{
		"getBlocksByIDs":       {},
		"getBlocksByHeight":    {},
		"getBlockPayloadByID":  {},
		"getNetworkParameters": {},
		"getNodeVersionInfo":   {},
	}

	s.testedRPCs = s.getRPCs
	s.testedRestEndpoints = s.getRestEndpoints

	consensusConfigs := []func(config *testnet.NodeConfig){
		// `cruise-ctl-fallback-proposal-duration` is set to 250ms instead to of 100ms
		// to purposely slow down the block rate. This is needed since the crypto module
		// update providing faster BLS operations.
		// TODO: fix the access integration test logic to function without slowing down
		// the block rate
		testnet.WithAdditionalFlag("--cruise-ctl-fallback-proposal-duration=250ms"),
		testnet.WithAdditionalFlagf("--required-verification-seal-approvals=%d", 1),
		testnet.WithAdditionalFlagf("--required-construction-seal-approvals=%d", 1),
		testnet.WithLogLevel(zerolog.FatalLevel),
	}

	nodeConfigs := []testnet.NodeConfig{
		// access node with unstaked nodes supported
		testnet.NewNodeConfig(flow.RoleAccess, testnet.WithLogLevel(zerolog.InfoLevel),
			testnet.WithAdditionalFlag("--supports-observer=true"),
			testnet.WithAdditionalFlagf("--public-network-execution-data-sync-enabled=true"),
			testnet.WithAdditionalFlagf("--script-execution-mode=%s", backend.IndexQueryModeExecutionNodesOnly),
			testnet.WithAdditionalFlagf("--tx-result-query-mode=%s", backend.IndexQueryModeExecutionNodesOnly),
			testnet.WithAdditionalFlag("--event-query-mode=execution-nodes-only"),
		),

		testnet.NewNodeConfig(flow.RoleCollection, testnet.WithLogLevel(zerolog.FatalLevel)),
		testnet.NewNodeConfig(flow.RoleCollection, testnet.WithLogLevel(zerolog.FatalLevel)),
		testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.FatalLevel)),
		testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.FatalLevel)),
		testnet.NewNodeConfig(flow.RoleConsensus, consensusConfigs...),
		testnet.NewNodeConfig(flow.RoleConsensus, consensusConfigs...),
		testnet.NewNodeConfig(flow.RoleConsensus, consensusConfigs...),
		testnet.NewNodeConfig(flow.RoleVerification, testnet.WithLogLevel(zerolog.FatalLevel)),
	}

	observers := []testnet.ObserverConfig{{
		LogLevel: zerolog.InfoLevel,
		AdditionalFlags: []string{
			fmt.Sprintf("--execution-data-dir=%s", testnet.DefaultExecutionDataServiceDir),
			fmt.Sprintf("--execution-state-dir=%s", testnet.DefaultExecutionStateDir),
			"--execution-data-sync-enabled=true",
			"--execution-data-indexing-enabled=true",
			"--local-service-api-enabled=true",
			"--event-query-mode=execution-nodes-only",
		},
	}}

	// prepare the network
	conf := testnet.NewNetworkConfig("observer_indexing_enabled_test", nodeConfigs, testnet.WithObservers(observers...))
	s.net = testnet.PrepareFlowNetwork(s.T(), conf, flow.Localnet)

	// start the network
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel

	s.net.Start(ctx)
}

// TestObserverIndexedRPCsHappyPath tests RPCs that are handled by the observer by using a dedicated indexer for the events.
// For now the observer only supports the following RPCs:
// - GetEventsForHeightRange
// - GetEventsForBlockIDs
// To ensure that the observer is handling these RPCs, we stop the upstream access node and verify that the observer client
// returns success for valid requests and errors for invalid ones.
func (s *ObserverIndexerEnabledSuite) TestObserverIndexedRPCsHappyPath() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t := s.T()

	// prepare environment to create a new account
	serviceAccountClient, err := s.net.ContainerByName(testnet.PrimaryAN).TestnetClient()
	require.NoError(t, err)

	latestBlockID, err := serviceAccountClient.GetLatestBlockID(ctx)
	require.NoError(t, err)

	// create new account to deploy Counter to
	accountPrivateKey := lib.RandomPrivateKey()

	accountKey := sdk.NewAccountKey().
		FromPrivateKey(accountPrivateKey).
		SetHashAlgo(sdkcrypto.SHA3_256).
		SetWeight(sdk.AccountKeyWeightThreshold)

	serviceAddress := sdk.Address(serviceAccountClient.Chain.ServiceAddress())

	// Generate the account creation transaction
	createAccountTx, err := templates.CreateAccount(
		[]*sdk.AccountKey{accountKey},
		[]templates.Contract{
			{
				Name:   lib.CounterContract.Name,
				Source: lib.CounterContract.ToCadence(),
			},
		}, serviceAddress)
	require.NoError(t, err)
	createAccountTx.
		SetReferenceBlockID(sdk.Identifier(latestBlockID)).
		SetProposalKey(serviceAddress, 0, serviceAccountClient.GetSeqNumber()).
		SetPayer(serviceAddress).
		SetComputeLimit(9999)

	// send the create account tx
	childCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	err = serviceAccountClient.SignAndSendTransaction(childCtx, createAccountTx)
	require.NoError(t, err)

	cancel()

	// wait for account to be created
	var accountCreationTxRes *sdk.TransactionResult
	unittest.RequireReturnsBefore(t, func() {
		accountCreationTxRes, err = serviceAccountClient.WaitForSealed(context.Background(), createAccountTx.ID())
		require.NoError(t, err)
	}, 20*time.Second, "has to seal before timeout")

	// obtain the account address
	var accountCreatedPayload []byte
	var newAccountAddress sdk.Address
	for _, event := range accountCreationTxRes.Events {
		if event.Type == sdk.EventAccountCreated {
			accountCreatedEvent := sdk.AccountCreatedEvent(event)
			accountCreatedPayload = accountCreatedEvent.Payload
			newAccountAddress = accountCreatedEvent.Address()
			break
		}
	}
	require.NotEqual(t, sdk.EmptyAddress, newAccountAddress)

	// now we can query events using observer to data which has to be locally indexed

	// get an observer client
	observer, err := s.getObserverClient()
	require.NoError(t, err)

	// wait for data to be synced by observer
	require.Eventually(t, func() bool {
		_, err := observer.GetAccountAtBlockHeight(ctx, &accessproto.GetAccountAtBlockHeightRequest{
			Address:     newAccountAddress.Bytes(),
			BlockHeight: accountCreationTxRes.BlockHeight,
		})
		statusErr, ok := status.FromError(err)
		if !ok || err == nil {
			return true
		}
		return statusErr.Code() != codes.OutOfRange
	}, 30*time.Second, 1*time.Second)

	blockWithAccount, err := observer.GetBlockHeaderByID(ctx, &accessproto.GetBlockHeaderByIDRequest{
		Id: accountCreationTxRes.BlockID[:],
	})
	require.NoError(t, err)

	// stop the upstream access container
	err = s.net.StopContainerByName(ctx, testnet.PrimaryAN)
	require.NoError(t, err)

	eventsByBlockID, err := observer.GetEventsForBlockIDs(ctx, &accessproto.GetEventsForBlockIDsRequest{
		Type:                 sdk.EventAccountCreated,
		BlockIds:             [][]byte{blockWithAccount.Block.Id},
		EventEncodingVersion: entities.EventEncodingVersion_JSON_CDC_V0,
	})
	require.NoError(t, err)

	eventsByHeight, err := observer.GetEventsForHeightRange(ctx, &accessproto.GetEventsForHeightRangeRequest{
		Type:                 sdk.EventAccountCreated,
		StartHeight:          blockWithAccount.Block.Height,
		EndHeight:            blockWithAccount.Block.Height,
		EventEncodingVersion: entities.EventEncodingVersion_JSON_CDC_V0,
	})
	require.NoError(t, err)

	// validate that there is an event that we are looking for
	require.Equal(t, eventsByHeight.Results, eventsByBlockID.Results)
	found := false
	for _, eventsInBlock := range eventsByHeight.Results {
		for _, event := range eventsInBlock.Events {
			if event.Type == sdk.EventAccountCreated {
				if bytes.Equal(event.Payload, accountCreatedPayload) {
					found = true
				}
			}
		}
	}
	require.True(t, found)

}

func (s *ObserverIndexerEnabledSuite) getRPCs() []RPCTest {
	return []RPCTest{
		{name: "Ping", call: func(ctx context.Context, client accessproto.AccessAPIClient) error {
			_, err := client.Ping(ctx, &accessproto.PingRequest{})
			return err
		}},
		{name: "GetLatestBlockHeader", call: func(ctx context.Context, client accessproto.AccessAPIClient) error {
			_, err := client.GetLatestBlockHeader(ctx, &accessproto.GetLatestBlockHeaderRequest{})
			return err
		}},
		{name: "GetBlockHeaderByID", call: func(ctx context.Context, client accessproto.AccessAPIClient) error {
			_, err := client.GetBlockHeaderByID(ctx, &accessproto.GetBlockHeaderByIDRequest{
				Id: make([]byte, 32),
			})
			return err
		}},
		{name: "GetBlockHeaderByHeight", call: func(ctx context.Context, client accessproto.AccessAPIClient) error {
			_, err := client.GetBlockHeaderByHeight(ctx, &accessproto.GetBlockHeaderByHeightRequest{})
			return err
		}},
		{name: "GetLatestBlock", call: func(ctx context.Context, client accessproto.AccessAPIClient) error {
			_, err := client.GetLatestBlock(ctx, &accessproto.GetLatestBlockRequest{})
			return err
		}},
		{name: "GetBlockByID", call: func(ctx context.Context, client accessproto.AccessAPIClient) error {
			_, err := client.GetBlockByID(ctx, &accessproto.GetBlockByIDRequest{Id: make([]byte, 32)})
			return err
		}},
		{name: "GetBlockByHeight", call: func(ctx context.Context, client accessproto.AccessAPIClient) error {
			_, err := client.GetBlockByHeight(ctx, &accessproto.GetBlockByHeightRequest{})
			return err
		}},
		{name: "GetCollectionByID", call: func(ctx context.Context, client accessproto.AccessAPIClient) error {
			_, err := client.GetCollectionByID(ctx, &accessproto.GetCollectionByIDRequest{Id: make([]byte, 32)})
			return err
		}},
		{name: "SendTransaction", call: func(ctx context.Context, client accessproto.AccessAPIClient) error {
			_, err := client.SendTransaction(ctx, &accessproto.SendTransactionRequest{})
			return err
		}},
		{name: "GetTransaction", call: func(ctx context.Context, client accessproto.AccessAPIClient) error {
			_, err := client.GetTransaction(ctx, &accessproto.GetTransactionRequest{})
			return err
		}},
		{name: "GetTransactionResult", call: func(ctx context.Context, client accessproto.AccessAPIClient) error {
			_, err := client.GetTransactionResult(ctx, &accessproto.GetTransactionRequest{})
			return err
		}},
		{name: "GetTransactionResultByIndex", call: func(ctx context.Context, client accessproto.AccessAPIClient) error {
			_, err := client.GetTransactionResultByIndex(ctx, &accessproto.GetTransactionByIndexRequest{})
			return err
		}},
		{name: "GetTransactionResultsByBlockID", call: func(ctx context.Context, client accessproto.AccessAPIClient) error {
			_, err := client.GetTransactionResultsByBlockID(ctx, &accessproto.GetTransactionsByBlockIDRequest{})
			return err
		}},
		{name: "GetTransactionsByBlockID", call: func(ctx context.Context, client accessproto.AccessAPIClient) error {
			_, err := client.GetTransactionsByBlockID(ctx, &accessproto.GetTransactionsByBlockIDRequest{})
			return err
		}},
		{name: "GetAccount", call: func(ctx context.Context, client accessproto.AccessAPIClient) error {
			_, err := client.GetAccount(ctx, &accessproto.GetAccountRequest{})
			return err
		}},
		{name: "GetAccountAtLatestBlock", call: func(ctx context.Context, client accessproto.AccessAPIClient) error {
			_, err := client.GetAccountAtLatestBlock(ctx, &accessproto.GetAccountAtLatestBlockRequest{})
			return err
		}},
		{name: "GetAccountAtBlockHeight", call: func(ctx context.Context, client accessproto.AccessAPIClient) error {
			_, err := client.GetAccountAtBlockHeight(ctx, &accessproto.GetAccountAtBlockHeightRequest{})
			return err
		}},
		{name: "ExecuteScriptAtLatestBlock", call: func(ctx context.Context, client accessproto.AccessAPIClient) error {
			_, err := client.ExecuteScriptAtLatestBlock(ctx, &accessproto.ExecuteScriptAtLatestBlockRequest{})
			return err
		}},
		{name: "ExecuteScriptAtBlockID", call: func(ctx context.Context, client accessproto.AccessAPIClient) error {
			_, err := client.ExecuteScriptAtBlockID(ctx, &accessproto.ExecuteScriptAtBlockIDRequest{})
			return err
		}},
		{name: "ExecuteScriptAtBlockHeight", call: func(ctx context.Context, client accessproto.AccessAPIClient) error {
			_, err := client.ExecuteScriptAtBlockHeight(ctx, &accessproto.ExecuteScriptAtBlockHeightRequest{})
			return err
		}},
		{name: "GetNetworkParameters", call: func(ctx context.Context, client accessproto.AccessAPIClient) error {
			_, err := client.GetNetworkParameters(ctx, &accessproto.GetNetworkParametersRequest{})
			return err
		}},
		{name: "GetLatestProtocolStateSnapshot", call: func(ctx context.Context, client accessproto.AccessAPIClient) error {
			_, err := client.GetLatestProtocolStateSnapshot(ctx, &accessproto.GetLatestProtocolStateSnapshotRequest{})
			return err
		}},
		{name: "GetExecutionResultForBlockID", call: func(ctx context.Context, client accessproto.AccessAPIClient) error {
			_, err := client.GetExecutionResultForBlockID(ctx, &accessproto.GetExecutionResultForBlockIDRequest{})
			return err
		}},
	}
}

func (s *ObserverIndexerEnabledSuite) getRestEndpoints() []RestEndpointTest {
	transactionId := unittest.IdentifierFixture().String()
	account := flow.Localnet.Chain().ServiceAddress().String()
	block := unittest.BlockFixture()
	executionResult := unittest.ExecutionResultFixture()
	collection := unittest.CollectionFixture(2)
	eventType := unittest.EventTypeFixture(flow.Localnet)

	return []RestEndpointTest{
		{
			name:   "getTransactionByID",
			method: http.MethodGet,
			path:   "/transactions/" + transactionId,
		},
		{
			name:   "createTransaction",
			method: http.MethodPost,
			path:   "/transactions",
			body:   createTx(s.net),
		},
		{
			name:   "getTransactionResultByID",
			method: http.MethodGet,
			path:   fmt.Sprintf("/transaction_results/%s?block_id=%s&collection_id=%s", transactionId, block.ID().String(), collection.ID().String()),
		},
		{
			name:   "getBlocksByIDs",
			method: http.MethodGet,
			path:   "/blocks/" + block.ID().String(),
		},
		{
			name:   "getBlocksByHeight",
			method: http.MethodGet,
			path:   "/blocks?height=1",
		},
		{
			name:   "getBlockPayloadByID",
			method: http.MethodGet,
			path:   "/blocks/" + block.ID().String() + "/payload",
		},
		{
			name:   "getExecutionResultByID",
			method: http.MethodGet,
			path:   "/execution_results/" + executionResult.ID().String(),
		},
		{
			name:   "getExecutionResultByBlockID",
			method: http.MethodGet,
			path:   "/execution_results?block_id=" + block.ID().String(),
		},
		{
			name:   "getCollectionByID",
			method: http.MethodGet,
			path:   "/collections/" + collection.ID().String(),
		},
		{
			name:   "executeScript",
			method: http.MethodPost,
			path:   "/scripts",
			body:   createScript(),
		},
		{
			name:   "getAccount",
			method: http.MethodGet,
			path:   "/accounts/" + account + "?block_height=1",
		},
		{
			name:   "getEvents",
			method: http.MethodGet,
			path:   fmt.Sprintf("/events?type=%s&start_height=%d&end_height=%d", eventType, 0, 3),
		},
		{
			name:   "getNetworkParameters",
			method: http.MethodGet,
			path:   "/network/parameters",
		},
		{
			name:   "getNodeVersionInfo",
			method: http.MethodGet,
			path:   "/node_version_info",
		},
	}
}
