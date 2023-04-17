package access

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	accessproto "github.com/onflow/flow/protobuf/go/flow/access"

	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/model/flow"
)

func TestObserver(t *testing.T) {
	suite.Run(t, new(ObserverSuite))
}

type ObserverSuite struct {
	suite.Suite
	net      *testnet.FlowNetwork
	teardown func()
	local    map[string]struct{}

	cancel context.CancelFunc
}

func (s *ObserverSuite) TearDownTest() {
	if s.net != nil {
		s.net.Remove()
		s.net = nil
	}
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
}

func (s *ObserverSuite) SetupTest() {
	s.local = map[string]struct{}{
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

	nodeConfigs := []testnet.NodeConfig{
		// access node with unstaked nodes supported
		testnet.NewNodeConfig(flow.RoleAccess, testnet.WithLogLevel(zerolog.InfoLevel), testnet.WithAdditionalFlag("--supports-observer=true")),

		// need one dummy execution node (unused ghost)
		testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.FatalLevel), testnet.AsGhost()),

		// need one dummy verification node (unused ghost)
		testnet.NewNodeConfig(flow.RoleVerification, testnet.WithLogLevel(zerolog.FatalLevel), testnet.AsGhost()),

		// need one controllable collection node (unused ghost)
		testnet.NewNodeConfig(flow.RoleCollection, testnet.WithLogLevel(zerolog.FatalLevel), testnet.AsGhost()),

		// need three consensus nodes (unused ghost)
		testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithLogLevel(zerolog.FatalLevel), testnet.AsGhost()),
		testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithLogLevel(zerolog.FatalLevel), testnet.AsGhost()),
		testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithLogLevel(zerolog.FatalLevel), testnet.AsGhost()),
	}

	observers := []testnet.ObserverConfig{{
		LogLevel: zerolog.InfoLevel,
	}}

	// prepare the network
	conf := testnet.NewNetworkConfig("observer_api_test", nodeConfigs, testnet.WithObservers(observers...))
	s.net = testnet.PrepareFlowNetwork(s.T(), conf, flow.Localnet)

	// start the network
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel

	s.net.Start(ctx)
}

// TestObserver runs the following tests:
// 1. CompareRPCs: verifies that the observer client returns the same errors as the access client for rpcs proxied to the upstream AN
// 2. HandledByUpstream: stops the upstream AN and verifies that the observer client returns errors for all rpcs handled by the upstream
// 3. HandledByObserver: stops the upstream AN and verifies that the observer client handles all other queries
func (s *ObserverSuite) TestObserver() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t := s.T()

	// get an observer client
	observer, err := s.getObserverClient()
	require.NoError(t, err)

	access, err := s.getAccessClient()
	require.NoError(t, err)

	t.Run("CompareRPCs", func(t *testing.T) {
		// verify that both clients return the same errors for proxied rpcs
		for _, rpc := range s.getRPCs() {
			// skip rpcs handled locally by observer
			if _, local := s.local[rpc.name]; local {
				continue
			}
			t.Run(rpc.name, func(t *testing.T) {
				accessErr := rpc.call(ctx, access)
				observerErr := rpc.call(ctx, observer)
				assert.Equal(t, accessErr, observerErr)
			})
		}
	})

	// stop the upstream access container
	err = s.net.StopContainerByName(ctx, testnet.PrimaryAN)
	require.NoError(t, err)

	t.Run("HandledByUpstream", func(t *testing.T) {
		// verify that we receive Unavailable errors from all rpcs handled upstream
		for _, rpc := range s.getRPCs() {
			if _, local := s.local[rpc.name]; local {
				continue
			}
			t.Run(rpc.name, func(t *testing.T) {
				err := rpc.call(ctx, observer)
				assert.Equal(t, codes.Unavailable, status.Code(err))
			})
		}
	})

	t.Run("HandledByObserver", func(t *testing.T) {
		// verify that we receive NotFound or no error from all rpcs handled locally
		for _, rpc := range s.getRPCs() {
			if _, local := s.local[rpc.name]; !local {
				continue
			}
			t.Run(rpc.name, func(t *testing.T) {
				err := rpc.call(ctx, observer)
				if err == nil {
					return
				}
				assert.Equal(t, codes.NotFound, status.Code(err))
			})
		}
	})

}

func (s *ObserverSuite) getAccessClient() (accessproto.AccessAPIClient, error) {
	return s.getClient(s.net.ContainerByName(testnet.PrimaryAN).Addr(testnet.GRPCPort))
}

func (s *ObserverSuite) getObserverClient() (accessproto.AccessAPIClient, error) {
	return s.getClient(s.net.ContainerByName("observer_1").Addr(testnet.GRPCPort))
}

func (s *ObserverSuite) getClient(address string) (accessproto.AccessAPIClient, error) {
	// helper func to create an access client
	conn, err := grpc.Dial(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	client := accessproto.NewAccessAPIClient(conn)
	return client, nil
}

type RPCTest struct {
	name string
	call func(ctx context.Context, client accessproto.AccessAPIClient) error
}

func (s *ObserverSuite) getRPCs() []RPCTest {
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
		{name: "GetEventsForHeightRange", call: func(ctx context.Context, client accessproto.AccessAPIClient) error {
			_, err := client.GetEventsForHeightRange(ctx, &accessproto.GetEventsForHeightRangeRequest{})
			return err
		}},
		{name: "GetEventsForBlockIDs", call: func(ctx context.Context, client accessproto.AccessAPIClient) error {
			_, err := client.GetEventsForBlockIDs(ctx, &accessproto.GetEventsForBlockIDsRequest{})
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
