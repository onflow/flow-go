package cohort2

import (
	"bytes"
	"context"
	"fmt"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
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

func TestObserverIndexerEnabledExtended(t *testing.T) {
	suite.Run(t, new(ObserverIndexerEnabledExtendedSuite))
}

// ObserverIndexerEnabledExtendedSuite tests the observer with the indexer enabled.
// It uses ObserverSuite as a base to reuse the test cases that need to be run for any observer variation.
type ObserverIndexerEnabledExtendedSuite struct {
	ObserverSuite
}

// SetupTest sets up the test suite by starting the network and preparing the observer client.
// By overriding this function, we can ensure that the observer is started with correct parameters and select
// the RPCs and REST endpoints that are tested.
func (s *ObserverIndexerEnabledExtendedSuite) SetupTest() {
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
		ContainerName: testnet.PrimaryON,
		LogLevel:      zerolog.InfoLevel,
		AdditionalFlags: []string{
			fmt.Sprintf("--execution-data-dir=%s", testnet.DefaultExecutionDataServiceDir),
			fmt.Sprintf("--execution-state-dir=%s", testnet.DefaultExecutionStateDir),
			"--execution-data-sync-enabled=true",
			"--execution-data-indexing-enabled=true",
			"--local-service-api-enabled=true",
			"--event-query-mode=execution-nodes-only",
		},
	},
		{
			ContainerName: "observer_2",
			LogLevel:      zerolog.InfoLevel,
		},
	}

	// prepare the network
	conf := testnet.NewNetworkConfig("observer_indexing_enabled_extended_test", nodeConfigs, testnet.WithObservers(observers...))
	s.net = testnet.PrepareFlowNetwork(s.T(), conf, flow.Localnet)

	// start the network
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel

	s.net.Start(ctx)
}

func (s *ObserverIndexerEnabledExtendedSuite) TestObserverIndexedRPCsHappyPath() {
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

	// now we can query events using observerLocal to data which has to be locally indexed

	// get an access node client
	accessNode, err := s.getClient(s.net.ContainerByName(testnet.PrimaryAN).Addr(testnet.GRPCPort))
	require.NoError(t, err)

	// get an observer with indexer enabled client
	observerLocal, err := s.getObserverClient()
	require.NoError(t, err)

	// get an upstream observer client
	observerUpstream, err := s.getClient(s.net.ContainerByName("observer_2").Addr(testnet.GRPCPort))
	require.NoError(t, err)

	// wait for data to be synced by observerLocal
	require.Eventually(t, func() bool {
		_, err := observerLocal.GetAccountAtBlockHeight(ctx, &accessproto.GetAccountAtBlockHeightRequest{
			Address:     newAccountAddress.Bytes(),
			BlockHeight: accountCreationTxRes.BlockHeight,
		})
		statusErr, ok := status.FromError(err)
		if !ok || err == nil {
			return true
		}
		return statusErr.Code() != codes.OutOfRange
	}, 30*time.Second, 1*time.Second)

	log := unittest.LoggerForTest(s.Suite.T(), zerolog.InfoLevel)
	log.Info().Msg("================> onverted.Payload.Results")

	blockWithAccount, err := observerLocal.GetBlockByID(ctx, &accessproto.GetBlockByIDRequest{
		Id:                accountCreationTxRes.BlockID[:],
		FullBlockResponse: true,
	})
	require.NoError(t, err)

	// GetEventsForBlockIDs
	eventsByBlockID := s.TestGetEventsForBlockIDsObserverRPC(ctx, observerLocal, observerUpstream, accessNode, [][]byte{blockWithAccount.Block.Id})

	// GetEventsForHeightRange
	eventsByHeight := s.TestGetEventsForHeightRangeObserverRPC(ctx, observerLocal, observerUpstream, accessNode, blockWithAccount.Block.Height, blockWithAccount.Block.Height)

	// validate that there is an event that we are looking for
	require.Equal(t, eventsByHeight.Results, eventsByBlockID.Results)

	var txIndex uint32
	found := false
	for _, eventsInBlock := range eventsByHeight.Results {
		for _, event := range eventsInBlock.Events {
			if event.Type == sdk.EventAccountCreated {
				if bytes.Equal(event.Payload, accountCreatedPayload) {
					found = true
					txIndex = event.TransactionIndex
				}
			}
		}
	}
	require.True(t, found)

	// GetSystemTransaction
	s.TestGetSystemTransactionObserverRPC(ctx, observerLocal, observerUpstream, accessNode, blockWithAccount.Block.Id)

	converted, err := convert.MessageToBlock(blockWithAccount.Block)
	require.NoError(t, err)

	resultId := converted.Payload.Results[0].ID()

	// GetExecutionResultByID
	s.TestGetExecutionResultByIDObserverRPC(ctx, observerLocal, observerUpstream, accessNode, convert.IdentifierToMessage(resultId))

	//GetTransaction
	s.TestGetTransactionObserverRPC(ctx, observerLocal, observerUpstream, accessNode, accountCreationTxRes.TransactionID.Bytes(), blockWithAccount.Block.Id, nil)

	// GetTransactionResult
	s.TestGetTransactionResultObserverRPC(ctx, observerLocal, observerUpstream, accessNode, accountCreationTxRes.TransactionID.Bytes(), blockWithAccount.Block.Id, accountCreationTxRes.CollectionID.Bytes())

	//GetTransactionResultByIndex
	s.TestGetTransactionResultsByIndexIDObserverRPC(ctx, observerLocal, observerUpstream, accessNode, blockWithAccount.Block.Id, txIndex)

	// GetTransactionResultsByBlockID
	s.TestGetTransactionResultsByBlockIDObserverRPC(ctx, observerLocal, observerUpstream, accessNode, blockWithAccount.Block.Id)

	// GetTransactionsByBlockID
	s.TestGetTransactionsByBlockIDObserverRPC(ctx, observerLocal, observerUpstream, accessNode, blockWithAccount.Block.Id)

	// GetCollectionByID
	s.TestGetCollectionByIDObserverRPC(ctx, observerLocal, observerUpstream, accessNode, accountCreationTxRes.CollectionID.Bytes())

	// ExecuteScriptAtBlockHeight
	s.TestExecuteScriptAtBlockHeightObserverRPC(ctx, observerLocal, observerUpstream, accessNode, blockWithAccount.Block.Height, []byte(simpleScript))

	// ExecuteScriptAtBlockID
	s.TestExecuteScriptAtBlockIDObserverRPC(ctx, observerLocal, observerUpstream, accessNode, blockWithAccount.Block.Id, []byte(simpleScript))

	// GetAccountAtBlockHeight
	s.TestGetAccountAtBlockHeightObserverRPC(ctx, observerLocal, observerUpstream, accessNode, newAccountAddress.Bytes(), accountCreationTxRes.BlockHeight)

	// GetAccount
	//getAccountObserver1Response, err := observerLocal.GetAccount(ctx, &accessproto.GetAccountRequest{
	//	Address: newAccountAddress.Bytes(),
	//})
	//require.NoError(t, err)
	//
	//getAccountObserver2Response, err := observerUpstream.GetAccount(ctx, &accessproto.GetAccountRequest{
	//	Address: newAccountAddress.Bytes(),
	//})
	//require.NoError(t, err)
	//
	//getAccountAccessResponse, err := accessNode.GetAccount(ctx, &accessproto.GetAccountRequest{
	//	Address: newAccountAddress.Bytes(),
	//})
	//require.NoError(t, err)
	//
	//require.Equal(t, getAccountAccessResponse.Account, getAccountObserver2Response.Account)
	//require.Equal(t, getAccountAccessResponse.Account, getAccountObserver1Response.Account)

	//GetAccountAtLatestBlock
	//getAccountAtLatestBlockObserver1Response, err := observerLocal.GetAccountAtLatestBlock(ctx, &accessproto.GetAccountAtLatestBlockRequest{
	//	Address: newAccountAddress.Bytes(),
	//})
	//require.NoError(t, err)
	//
	//getAccountAtLatestBlockObserver2Response, err := observerUpstream.GetAccountAtLatestBlock(ctx, &accessproto.GetAccountAtLatestBlockRequest{
	//	Address: newAccountAddress.Bytes(),
	//})
	//require.NoError(t, err)
	//
	//getAccountAtLatestBlockAccessResponse, err := accessNode.GetAccountAtLatestBlock(ctx, &accessproto.GetAccountAtLatestBlockRequest{
	//	Address: newAccountAddress.Bytes(),
	//})
	//require.NoError(t, err)
	//
	//require.Equal(t, getAccountAtLatestBlockObserver2Response.Account, getAccountAtLatestBlockAccessResponse.Account)
	//require.Equal(t, getAccountAtLatestBlockObserver1Response.Account, getAccountAtLatestBlockAccessResponse.Account)
}

func (s *ObserverIndexerEnabledExtendedSuite) TestGetEventsForBlockIDsObserverRPC(
	ctx context.Context,
	observerLocal accessproto.AccessAPIClient,
	observerUpstream accessproto.AccessAPIClient,
	accessNode accessproto.AccessAPIClient,
	blockIds [][]byte,
) *accessproto.EventsResponse {
	observerLocalResponse, err := observerLocal.GetEventsForBlockIDs(ctx, &accessproto.GetEventsForBlockIDsRequest{
		Type:                 sdk.EventAccountCreated,
		BlockIds:             blockIds,
		EventEncodingVersion: entities.EventEncodingVersion_JSON_CDC_V0,
	})
	require.NoError(s.T(), err)

	observerUpstreamResponse, err := observerUpstream.GetEventsForBlockIDs(ctx, &accessproto.GetEventsForBlockIDsRequest{
		Type:                 sdk.EventAccountCreated,
		BlockIds:             blockIds,
		EventEncodingVersion: entities.EventEncodingVersion_JSON_CDC_V0,
	})
	require.NoError(s.T(), err)

	accessNodeResponse, err := accessNode.GetEventsForBlockIDs(ctx, &accessproto.GetEventsForBlockIDsRequest{
		Type:                 sdk.EventAccountCreated,
		BlockIds:             blockIds,
		EventEncodingVersion: entities.EventEncodingVersion_JSON_CDC_V0,
	})
	require.NoError(s.T(), err)

	require.Equal(s.T(), accessNodeResponse.Results, observerLocalResponse.Results)
	require.Equal(s.T(), accessNodeResponse.Results, observerUpstreamResponse.Results)

	return observerLocalResponse
}

func (s *ObserverIndexerEnabledExtendedSuite) TestGetEventsForHeightRangeObserverRPC(
	ctx context.Context,
	observerLocal accessproto.AccessAPIClient,
	observerUpstream accessproto.AccessAPIClient,
	accessNode accessproto.AccessAPIClient,
	startHeight uint64,
	endHeight uint64,
) *accessproto.EventsResponse {
	observerLocalResponse, err := observerLocal.GetEventsForHeightRange(ctx, &accessproto.GetEventsForHeightRangeRequest{
		Type:                 sdk.EventAccountCreated,
		StartHeight:          startHeight,
		EndHeight:            endHeight,
		EventEncodingVersion: entities.EventEncodingVersion_JSON_CDC_V0,
	})
	require.NoError(s.T(), err)

	observerUpstreamResponse, err := observerUpstream.GetEventsForHeightRange(ctx, &accessproto.GetEventsForHeightRangeRequest{
		Type:                 sdk.EventAccountCreated,
		StartHeight:          startHeight,
		EndHeight:            endHeight,
		EventEncodingVersion: entities.EventEncodingVersion_JSON_CDC_V0,
	})
	require.NoError(s.T(), err)

	accessNodeResponse, err := accessNode.GetEventsForHeightRange(ctx, &accessproto.GetEventsForHeightRangeRequest{
		Type:                 sdk.EventAccountCreated,
		StartHeight:          startHeight,
		EndHeight:            endHeight,
		EventEncodingVersion: entities.EventEncodingVersion_JSON_CDC_V0,
	})
	require.NoError(s.T(), err)

	require.Equal(s.T(), accessNodeResponse.Results, observerLocalResponse.Results)
	require.Equal(s.T(), accessNodeResponse.Results, observerUpstreamResponse.Results)

	return observerLocalResponse
}

func (s *ObserverIndexerEnabledExtendedSuite) TestGetAccountAtBlockHeightObserverRPC(
	ctx context.Context,
	observerLocal accessproto.AccessAPIClient,
	observerUpstream accessproto.AccessAPIClient,
	accessNode accessproto.AccessAPIClient,
	accountAddress []byte,
	blockHeight uint64,
) {

	observerLocalResponse, err := observerLocal.GetAccountAtBlockHeight(ctx, &accessproto.GetAccountAtBlockHeightRequest{
		Address:     accountAddress,
		BlockHeight: blockHeight,
	})
	require.NoError(s.T(), err)

	observerUpstreamResponse, err := observerUpstream.GetAccountAtBlockHeight(ctx, &accessproto.GetAccountAtBlockHeightRequest{
		Address:     accountAddress,
		BlockHeight: blockHeight,
	})
	require.NoError(s.T(), err)

	accessNodeResponse, err := accessNode.GetAccountAtBlockHeight(ctx, &accessproto.GetAccountAtBlockHeightRequest{
		Address:     accountAddress,
		BlockHeight: blockHeight,
	})
	require.NoError(s.T(), err)

	require.Equal(s.T(), accessNodeResponse.Account, observerLocalResponse.Account)
	require.Equal(s.T(), accessNodeResponse.Account, observerUpstreamResponse.Account)
}

func (s *ObserverIndexerEnabledExtendedSuite) TestGetSystemTransactionObserverRPC(
	ctx context.Context,
	observerLocal accessproto.AccessAPIClient,
	observerUpstream accessproto.AccessAPIClient,
	accessNode accessproto.AccessAPIClient,
	blockId []byte,
) {

	observerLocalResponse, err := observerLocal.GetSystemTransaction(ctx, &accessproto.GetSystemTransactionRequest{
		BlockId: blockId,
	})
	require.NoError(s.T(), err)

	observerUpstreamResponse, err := observerUpstream.GetSystemTransaction(ctx, &accessproto.GetSystemTransactionRequest{
		BlockId: blockId,
	})
	require.NoError(s.T(), err)

	accessNodeResponse, err := accessNode.GetSystemTransaction(ctx, &accessproto.GetSystemTransactionRequest{
		BlockId: blockId,
	})
	require.NoError(s.T(), err)

	require.Equal(s.T(), accessNodeResponse.Transaction, observerLocalResponse.Transaction)
	require.Equal(s.T(), accessNodeResponse.Transaction, observerUpstreamResponse.Transaction)
}

func (s *ObserverIndexerEnabledExtendedSuite) TestGetExecutionResultByIDObserverRPC(
	ctx context.Context,
	observerLocal accessproto.AccessAPIClient,
	observerUpstream accessproto.AccessAPIClient,
	accessNode accessproto.AccessAPIClient,
	id []byte,
) {

	observerLocalResponse, err := observerLocal.GetExecutionResultByID(ctx, &accessproto.GetExecutionResultByIDRequest{
		Id: id,
	})
	require.NoError(s.T(), err)

	observerUpstreamResponse, err := observerUpstream.GetExecutionResultByID(ctx, &accessproto.GetExecutionResultByIDRequest{
		Id: id,
	})
	require.NoError(s.T(), err)

	accessNodeResponse, err := accessNode.GetExecutionResultByID(ctx, &accessproto.GetExecutionResultByIDRequest{
		Id: id,
	})
	require.NoError(s.T(), err)

	require.Equal(s.T(), accessNodeResponse.ExecutionResult, observerLocalResponse.ExecutionResult)
	require.Equal(s.T(), accessNodeResponse.ExecutionResult, observerUpstreamResponse.ExecutionResult)
}

func (s *ObserverIndexerEnabledExtendedSuite) TestGetTransactionObserverRPC(
	ctx context.Context,
	observerLocal accessproto.AccessAPIClient,
	observerUpstream accessproto.AccessAPIClient,
	accessNode accessproto.AccessAPIClient,
	id []byte,
	blockId []byte,
	collectionId []byte,
) {

	observerLocalResponse, err := observerLocal.GetTransaction(ctx, &accessproto.GetTransactionRequest{
		Id:           id,
		BlockId:      blockId,
		CollectionId: collectionId,
	})
	require.NoError(s.T(), err)

	observerUpstreamResponse, err := observerUpstream.GetTransaction(ctx, &accessproto.GetTransactionRequest{
		Id:           id,
		BlockId:      blockId,
		CollectionId: collectionId,
	})
	require.NoError(s.T(), err)

	accessNodeResponse, err := accessNode.GetTransaction(ctx, &accessproto.GetTransactionRequest{
		Id:           id,
		BlockId:      blockId,
		CollectionId: collectionId,
	})
	require.NoError(s.T(), err)

	require.Equal(s.T(), accessNodeResponse.Transaction, observerLocalResponse.Transaction)
	require.Equal(s.T(), accessNodeResponse.Transaction, observerUpstreamResponse.Transaction)
}

func (s *ObserverIndexerEnabledExtendedSuite) TestGetTransactionResultObserverRPC(
	ctx context.Context,
	observerLocal accessproto.AccessAPIClient,
	observerUpstream accessproto.AccessAPIClient,
	accessNode accessproto.AccessAPIClient,
	id []byte,
	blockId []byte,
	collectionId []byte,
) {

	observerLocalResponse, err := observerLocal.GetTransactionResult(ctx, &accessproto.GetTransactionRequest{
		Id:           id,
		BlockId:      blockId,
		CollectionId: collectionId,
	})
	require.NoError(s.T(), err)

	observerUpstreamResponse, err := observerUpstream.GetTransactionResult(ctx, &accessproto.GetTransactionRequest{
		Id:           id,
		BlockId:      blockId,
		CollectionId: collectionId,
	})
	require.NoError(s.T(), err)

	accessNodeResponse, err := accessNode.GetTransactionResult(ctx, &accessproto.GetTransactionRequest{
		Id:           id,
		BlockId:      blockId,
		CollectionId: collectionId,
	})
	require.NoError(s.T(), err)

	require.Equal(s.T(), accessNodeResponse.Events, observerLocalResponse.Events)
	require.Equal(s.T(), accessNodeResponse.Events, observerUpstreamResponse.Events)
}

func (s *ObserverIndexerEnabledExtendedSuite) TestGetTransactionResultsByBlockIDObserverRPC(
	ctx context.Context,
	observerLocal accessproto.AccessAPIClient,
	observerUpstream accessproto.AccessAPIClient,
	accessNode accessproto.AccessAPIClient,
	blockId []byte,
) {

	observerLocalResponse, err := observerLocal.GetTransactionResultsByBlockID(ctx, &accessproto.GetTransactionsByBlockIDRequest{
		BlockId:              blockId,
		EventEncodingVersion: entities.EventEncodingVersion_JSON_CDC_V0,
	})
	require.NoError(s.T(), err)

	observerUpstreamResponse, err := observerUpstream.GetTransactionResultsByBlockID(ctx, &accessproto.GetTransactionsByBlockIDRequest{
		BlockId:              blockId,
		EventEncodingVersion: entities.EventEncodingVersion_JSON_CDC_V0,
	})
	require.NoError(s.T(), err)

	accessNodeResponse, err := accessNode.GetTransactionResultsByBlockID(ctx, &accessproto.GetTransactionsByBlockIDRequest{
		BlockId:              blockId,
		EventEncodingVersion: entities.EventEncodingVersion_JSON_CDC_V0,
	})
	require.NoError(s.T(), err)

	require.Equal(s.T(), accessNodeResponse.TransactionResults, observerLocalResponse.TransactionResults)
	require.Equal(s.T(), accessNodeResponse.TransactionResults, observerUpstreamResponse.TransactionResults)
}

func (s *ObserverIndexerEnabledExtendedSuite) TestGetTransactionResultsByIndexIDObserverRPC(
	ctx context.Context,
	observerLocal accessproto.AccessAPIClient,
	observerUpstream accessproto.AccessAPIClient,
	accessNode accessproto.AccessAPIClient,
	blockId []byte,
	index uint32,
) {
	observerLocalResponse, err := observerLocal.GetTransactionResultByIndex(ctx, &accessproto.GetTransactionByIndexRequest{
		BlockId:              blockId,
		Index:                index,
		EventEncodingVersion: entities.EventEncodingVersion_JSON_CDC_V0,
	})
	require.NoError(s.T(), err)

	observerUpstreamResponse, err := observerUpstream.GetTransactionResultByIndex(ctx, &accessproto.GetTransactionByIndexRequest{
		BlockId:              blockId,
		Index:                index,
		EventEncodingVersion: entities.EventEncodingVersion_JSON_CDC_V0,
	})
	require.NoError(s.T(), err)

	accessNodeResponse, err := accessNode.GetTransactionResultByIndex(ctx, &accessproto.GetTransactionByIndexRequest{
		BlockId:              blockId,
		Index:                index,
		EventEncodingVersion: entities.EventEncodingVersion_JSON_CDC_V0,
	})
	require.NoError(s.T(), err)

	require.Equal(s.T(), accessNodeResponse.Events, observerLocalResponse.Events)
	require.Equal(s.T(), accessNodeResponse.Events, observerUpstreamResponse.Events)
}

func (s *ObserverIndexerEnabledExtendedSuite) TestGetTransactionsByBlockIDObserverRPC(
	ctx context.Context,
	observerLocal accessproto.AccessAPIClient,
	observerUpstream accessproto.AccessAPIClient,
	accessNode accessproto.AccessAPIClient,
	blockId []byte,
) {

	observerLocalResponse, err := observerLocal.GetTransactionsByBlockID(ctx, &accessproto.GetTransactionsByBlockIDRequest{
		BlockId: blockId,
	})
	require.NoError(s.T(), err)

	observerUpstreamResponse, err := observerUpstream.GetTransactionsByBlockID(ctx, &accessproto.GetTransactionsByBlockIDRequest{
		BlockId: blockId,
	})
	require.NoError(s.T(), err)

	accessNodeResponse, err := accessNode.GetTransactionsByBlockID(ctx, &accessproto.GetTransactionsByBlockIDRequest{
		BlockId: blockId,
	})
	require.NoError(s.T(), err)

	require.Equal(s.T(), accessNodeResponse.Transactions, observerLocalResponse.Transactions)
	require.Equal(s.T(), accessNodeResponse.Transactions, observerUpstreamResponse.Transactions)
}

func (s *ObserverIndexerEnabledExtendedSuite) TestGetCollectionByIDObserverRPC(
	ctx context.Context,
	observerLocal accessproto.AccessAPIClient,
	observerUpstream accessproto.AccessAPIClient,
	accessNode accessproto.AccessAPIClient,
	collectionId []byte,
) {

	observerLocalResponse, err := observerLocal.GetCollectionByID(ctx, &accessproto.GetCollectionByIDRequest{
		Id: collectionId,
	})
	require.NoError(s.T(), err)

	observerUpstreamResponse, err := observerUpstream.GetCollectionByID(ctx, &accessproto.GetCollectionByIDRequest{
		Id: collectionId,
	})
	require.NoError(s.T(), err)

	accessNodeResponse, err := accessNode.GetCollectionByID(ctx, &accessproto.GetCollectionByIDRequest{
		Id: collectionId,
	})
	require.NoError(s.T(), err)

	require.Equal(s.T(), accessNodeResponse.Collection, observerLocalResponse.Collection)
	require.Equal(s.T(), accessNodeResponse.Collection, observerUpstreamResponse.Collection)
}

func (s *ObserverIndexerEnabledExtendedSuite) TestExecuteScriptAtBlockHeightObserverRPC(
	ctx context.Context,
	observerLocal accessproto.AccessAPIClient,
	observerUpstream accessproto.AccessAPIClient,
	accessNode accessproto.AccessAPIClient,
	blockHeight uint64,
	script []byte,
) {

	observerLocalResponse, err := observerLocal.ExecuteScriptAtBlockHeight(ctx, &accessproto.ExecuteScriptAtBlockHeightRequest{
		BlockHeight: blockHeight,
		Script:      script,
		Arguments:   make([][]byte, 0),
	})
	require.NoError(s.T(), err)

	observerUpstreamResponse, err := observerUpstream.ExecuteScriptAtBlockHeight(ctx, &accessproto.ExecuteScriptAtBlockHeightRequest{
		BlockHeight: blockHeight,
		Script:      script,
		Arguments:   make([][]byte, 0),
	})
	require.NoError(s.T(), err)

	accessNodeResponse, err := accessNode.ExecuteScriptAtBlockHeight(ctx, &accessproto.ExecuteScriptAtBlockHeightRequest{
		BlockHeight: blockHeight,
		Script:      script,
		Arguments:   make([][]byte, 0),
	})
	require.NoError(s.T(), err)

	require.Equal(s.T(), accessNodeResponse.Value, observerLocalResponse.Value)
	require.Equal(s.T(), accessNodeResponse.Value, observerUpstreamResponse.Value)
}

func (s *ObserverIndexerEnabledExtendedSuite) TestExecuteScriptAtBlockIDObserverRPC(
	ctx context.Context,
	observerLocal accessproto.AccessAPIClient,
	observerUpstream accessproto.AccessAPIClient,
	accessNode accessproto.AccessAPIClient,
	blockId []byte,
	script []byte,
) {

	observerLocalResponse, err := observerLocal.ExecuteScriptAtBlockID(ctx, &accessproto.ExecuteScriptAtBlockIDRequest{
		BlockId:   blockId,
		Script:    script,
		Arguments: make([][]byte, 0),
	})
	require.NoError(s.T(), err)

	observerUpstreamResponse, err := observerUpstream.ExecuteScriptAtBlockID(ctx, &accessproto.ExecuteScriptAtBlockIDRequest{
		BlockId:   blockId,
		Script:    script,
		Arguments: make([][]byte, 0),
	})
	require.NoError(s.T(), err)

	accessNodeResponse, err := accessNode.ExecuteScriptAtBlockID(ctx, &accessproto.ExecuteScriptAtBlockIDRequest{
		BlockId:   blockId,
		Script:    script,
		Arguments: make([][]byte, 0),
	})
	require.NoError(s.T(), err)

	require.Equal(s.T(), accessNodeResponse.Value, observerLocalResponse.Value)
	require.Equal(s.T(), accessNodeResponse.Value, observerUpstreamResponse.Value)
}
