package cohort2

import (
	"bytes"
	"context"
	"fmt"
	"github.com/onflow/flow/protobuf/go/flow/entities"
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

	// now we can query events using observer to data which has to be locally indexed

	access, err := s.getClient(s.net.ContainerByName(testnet.PrimaryAN).Addr(testnet.GRPCPort))
	require.NoError(t, err)

	// get an observer client
	observer, err := s.getObserverClient()
	require.NoError(t, err)

	observer_2, err := s.getClient(s.net.ContainerByName("observer_2").Addr(testnet.GRPCPort))
	require.NoError(t, err)

	// wait for data to be synced by observer
	require.Eventually(t, func() bool {
		_, err := observer.GetAccount(ctx, &accessproto.GetAccountRequest{
			Address: newAccountAddress.Bytes(),
		})
		statusErr, ok := status.FromError(err)
		if !ok || err == nil {
			return true
		}
		return statusErr.Code() != codes.OutOfRange
	}, 30*time.Second, 1*time.Second)

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

	// wait for data to be synced by observer
	require.Eventually(t, func() bool {
		_, err := observer.GetAccountAtLatestBlock(ctx, &accessproto.GetAccountAtLatestBlockRequest{
			Address: newAccountAddress.Bytes(),
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

	blockWithAccount_2, err := observer_2.GetBlockHeaderByID(ctx, &accessproto.GetBlockHeaderByIDRequest{
		Id: accountCreationTxRes.BlockID[:],
	})
	require.NoError(t, err)

	accessEventsByBlockID, err := access.GetEventsForBlockIDs(ctx, &accessproto.GetEventsForBlockIDsRequest{
		Type:                 sdk.EventAccountCreated,
		BlockIds:             [][]byte{blockWithAccount.Block.Id},
		EventEncodingVersion: entities.EventEncodingVersion_JSON_CDC_V0,
	})
	require.NoError(t, err)

	eventsByBlockID, err := observer.GetEventsForBlockIDs(ctx, &accessproto.GetEventsForBlockIDsRequest{
		Type:                 sdk.EventAccountCreated,
		BlockIds:             [][]byte{blockWithAccount.Block.Id},
		EventEncodingVersion: entities.EventEncodingVersion_JSON_CDC_V0,
	})
	require.NoError(t, err)

	eventsByBlockID_2, err := observer_2.GetEventsForBlockIDs(ctx, &accessproto.GetEventsForBlockIDsRequest{
		Type:                 sdk.EventAccountCreated,
		BlockIds:             [][]byte{blockWithAccount_2.Block.Id},
		EventEncodingVersion: entities.EventEncodingVersion_JSON_CDC_V0,
	})
	require.NoError(t, err)

	require.Equal(t, eventsByBlockID.Results, accessEventsByBlockID.Results)
	require.Equal(t, eventsByBlockID_2.Results, accessEventsByBlockID.Results)

	// GetEventsForHeightRange
	accessEventsByHeight, err := access.GetEventsForHeightRange(ctx, &accessproto.GetEventsForHeightRangeRequest{
		Type:                 sdk.EventAccountCreated,
		StartHeight:          blockWithAccount.Block.Height,
		EndHeight:            blockWithAccount.Block.Height,
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

	eventsByHeight_2, err := observer_2.GetEventsForHeightRange(ctx, &accessproto.GetEventsForHeightRangeRequest{
		Type:                 sdk.EventAccountCreated,
		StartHeight:          blockWithAccount_2.Block.Height,
		EndHeight:            blockWithAccount_2.Block.Height,
		EventEncodingVersion: entities.EventEncodingVersion_JSON_CDC_V0,
	})
	require.NoError(t, err)

	require.Equal(t, eventsByHeight.Results, accessEventsByHeight.Results)
	require.Equal(t, eventsByHeight_2.Results, accessEventsByHeight.Results)

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

	// GetAccount
	getAccountObserver1Response, err := observer.GetAccount(ctx, &accessproto.GetAccountRequest{
		Address: newAccountAddress.Bytes(),
	})
	require.NoError(t, err)

	getAccountObserver2Response, err := observer_2.GetAccount(ctx, &accessproto.GetAccountRequest{
		Address: newAccountAddress.Bytes(),
	})
	require.NoError(t, err)

	getAccountAccessResponse, err := access.GetAccount(ctx, &accessproto.GetAccountRequest{
		Address: newAccountAddress.Bytes(),
	})
	require.NoError(t, err)

	require.Equal(t, getAccountAccessResponse.Account, getAccountObserver2Response.Account)
	require.Equal(t, getAccountAccessResponse.Account, getAccountObserver1Response.Account)

	// GetAccountAtBlockHeight
	getAccountAtBlockHeightObserver1Response, err := observer.GetAccountAtBlockHeight(ctx, &accessproto.GetAccountAtBlockHeightRequest{
		Address:     newAccountAddress.Bytes(),
		BlockHeight: accountCreationTxRes.BlockHeight,
	})
	require.NoError(t, err)

	getAccountAtBlockHeightObserver2Response, err := observer_2.GetAccountAtBlockHeight(ctx, &accessproto.GetAccountAtBlockHeightRequest{
		Address:     newAccountAddress.Bytes(),
		BlockHeight: accountCreationTxRes.BlockHeight,
	})
	require.NoError(t, err)

	getAccountAtBlockHeightAccessResponse, err := access.GetAccountAtBlockHeight(ctx, &accessproto.GetAccountAtBlockHeightRequest{
		Address:     newAccountAddress.Bytes(),
		BlockHeight: accountCreationTxRes.BlockHeight,
	})
	require.NoError(t, err)

	require.Equal(t, getAccountAtBlockHeightObserver2Response.Account, getAccountAtBlockHeightAccessResponse.Account)
	require.Equal(t, getAccountAtBlockHeightObserver1Response.Account, getAccountAtBlockHeightAccessResponse.Account)

	//GetAccountAtLatestBlock
	getAccountAtLatestBlockObserver1Response, err := observer.GetAccountAtLatestBlock(ctx, &accessproto.GetAccountAtLatestBlockRequest{
		Address: newAccountAddress.Bytes(),
	})
	require.NoError(t, err)

	getAccountAtLatestBlockObserver2Response, err := observer_2.GetAccountAtLatestBlock(ctx, &accessproto.GetAccountAtLatestBlockRequest{
		Address: newAccountAddress.Bytes(),
	})
	require.NoError(t, err)

	getAccountAtLatestBlockAccessResponse, err := access.GetAccountAtLatestBlock(ctx, &accessproto.GetAccountAtLatestBlockRequest{
		Address: newAccountAddress.Bytes(),
	})
	require.NoError(t, err)

	require.Equal(t, getAccountAtLatestBlockObserver2Response.Account, getAccountAtLatestBlockAccessResponse.Account)
	require.Equal(t, getAccountAtLatestBlockObserver1Response.Account, getAccountAtLatestBlockAccessResponse.Account)

	// GetSystemTransaction
	getSystemTransactionObserver1Response, err := observer.GetSystemTransaction(ctx, &accessproto.GetSystemTransactionRequest{
		BlockId: blockWithAccount.Block.Id,
	})
	require.NoError(t, err)

	getSystemTransactionObserver2Response, err := observer_2.GetSystemTransaction(ctx, &accessproto.GetSystemTransactionRequest{
		BlockId: blockWithAccount.Block.Id,
	})
	require.NoError(t, err)

	getSystemTransactionAccessResponse, err := access.GetSystemTransaction(ctx, &accessproto.GetSystemTransactionRequest{
		BlockId: blockWithAccount.Block.Id,
	})
	require.NoError(t, err)

	require.Equal(t, getSystemTransactionObserver2Response.Transaction, getSystemTransactionAccessResponse.Transaction)
	require.Equal(t, getSystemTransactionObserver1Response.Transaction, getSystemTransactionAccessResponse.Transaction)

	// GetSystemTransactionResult
	getSystemTransactionResultObserver2Response, err := observer_2.GetSystemTransactionResult(ctx, &accessproto.GetSystemTransactionResultRequest{
		BlockId:              blockWithAccount.Block.Id,
		EventEncodingVersion: entities.EventEncodingVersion_JSON_CDC_V0,
	})
	require.NoError(t, err)

	getSystemTransactionResultAccessResponse, err := access.GetSystemTransactionResult(ctx, &accessproto.GetSystemTransactionResultRequest{
		BlockId:              blockWithAccount.Block.Id,
		EventEncodingVersion: entities.EventEncodingVersion_JSON_CDC_V0,
	})
	require.NoError(t, err)

	//getSystemTransactionResultObserver1Response, err := observer.GetSystemTransactionResult(ctx, &accessproto.GetSystemTransactionResultRequest{
	//	BlockId:              blockWithAccount.Block.Id,
	//	EventEncodingVersion: entities.EventEncodingVersion_JSON_CDC_V0,
	//})
	//require.NoError(t, err)

	require.Equal(t, getSystemTransactionResultObserver2Response.Events, getSystemTransactionResultAccessResponse.Events)
	//require.Equal(t, getSystemTransactionResultObserver1Response.Events, getSystemTransactionResultAccessResponse.Events)

	// GetExecutionResultByID
	getExecutionResultByIDObserver1Response, err := observer.GetExecutionResultByID(ctx, &accessproto.GetExecutionResultByIDRequest{
		Id: blockWithAccount.Block.Id,
	})
	require.NoError(t, err)

	getExecutionResultByIDObserver2Response, err := observer_2.GetExecutionResultByID(ctx, &accessproto.GetExecutionResultByIDRequest{
		Id: blockWithAccount.Block.Id,
	})
	require.NoError(t, err)

	getExecutionResultByIDAccessResponse, err := access.GetExecutionResultByID(ctx, &accessproto.GetExecutionResultByIDRequest{
		Id: blockWithAccount.Block.Id,
	})
	require.NoError(t, err)

	require.Equal(t, getExecutionResultByIDAccessResponse.ExecutionResult, getExecutionResultByIDObserver2Response.ExecutionResult)
	require.Equal(t, getExecutionResultByIDAccessResponse.ExecutionResult, getExecutionResultByIDObserver1Response.ExecutionResult)

}
