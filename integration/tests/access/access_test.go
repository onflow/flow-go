package access

import (
	"context"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/onflow/flow-go/consensus/hotstuff/committees"
	"github.com/onflow/flow-go/consensus/hotstuff/signature"
	"github.com/onflow/flow-go/engine/access/rest/request"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/engine/common/state_stream"
	"net"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	accessproto "github.com/onflow/flow/protobuf/go/flow/access"

	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestAccess(t *testing.T) {
	suite.Run(t, new(AccessSuite))
}

type AccessSuite struct {
	suite.Suite

	log zerolog.Logger

	// root context for the current test
	ctx    context.Context
	cancel context.CancelFunc

	net *testnet.FlowNetwork
}

func (s *AccessSuite) TearDownTest() {
	s.log.Info().Msg("================> Start TearDownTest")
	s.net.Remove()
	s.cancel()
	s.log.Info().Msg("================> Finish TearDownTest")
}

func (s *AccessSuite) SetupTest() {
	s.log = unittest.LoggerForTest(s.Suite.T(), zerolog.InfoLevel)
	s.log.Info().Msg("================> SetupTest")
	defer func() {
		s.log.Info().Msg("================> Finish SetupTest")
	}()

	nodeConfigs := []testnet.NodeConfig{
		testnet.NewNodeConfig(flow.RoleAccess, testnet.WithLogLevel(zerolog.InfoLevel), testnet.WithAdditionalFlag(fmt.Sprintf("--state-stream-addr=%s", testnet.ExecutionStatePort))),
	}

	// need one dummy execution node (unused ghost)
	exeConfig := testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.FatalLevel))
	nodeConfigs = append(nodeConfigs, exeConfig)

	// need one dummy verification node (unused ghost)
	verConfig := testnet.NewNodeConfig(flow.RoleVerification, testnet.WithLogLevel(zerolog.FatalLevel))
	nodeConfigs = append(nodeConfigs, verConfig)

	// need one controllable collection node (unused ghost)
	collConfig := testnet.NewNodeConfig(flow.RoleCollection, testnet.WithLogLevel(zerolog.FatalLevel))
	nodeConfigs = append(nodeConfigs, collConfig)

	// need three consensus nodes (unused ghost)
	for n := 0; n < 3; n++ {
		conID := unittest.IdentifierFixture()
		nodeConfig := testnet.NewNodeConfig(flow.RoleConsensus,
			testnet.WithLogLevel(zerolog.FatalLevel),
			testnet.WithID(conID))
		nodeConfigs = append(nodeConfigs, nodeConfig)
	}

	conf := testnet.NewNetworkConfig("access_api_test", nodeConfigs)
	s.net = testnet.PrepareFlowNetwork(s.T(), conf, flow.Localnet)

	// start the network
	s.T().Logf("starting flow network with docker containers")
	s.ctx, s.cancel = context.WithCancel(context.Background())

	s.net.Start(s.ctx)
}

func (s *AccessSuite) TestAPIsAvailable() {

	s.T().Run("TestHTTPProxyPortOpen", func(t *testing.T) {
		httpProxyAddress := s.net.ContainerByName(testnet.PrimaryAN).Addr(testnet.GRPCWebPort)

		conn, err := net.DialTimeout("tcp", httpProxyAddress, 1*time.Second)
		require.NoError(s.T(), err, "http proxy port not open on the access node")

		conn.Close()
	})

	s.T().Run("TestAccessConnection", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(s.ctx, 1*time.Second)
		defer cancel()

		grpcAddress := s.net.ContainerByName(testnet.PrimaryAN).Addr(testnet.GRPCPort)
		conn, err := grpc.DialContext(ctx, grpcAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
		require.NoError(t, err, "failed to connect to access node")
		defer conn.Close()

		client := accessproto.NewAccessAPIClient(conn)

		_, err = client.Ping(s.ctx, &accessproto.PingRequest{})
		assert.NoError(t, err, "failed to ping access node")
	})
}

// TestSignerIndicesDecoding tests that access node uses signer indices' decoder to correctly parse encoded data in blocks.
// This test receives blocks from consensus follower and then requests same blocks from access API and checks if returned data
// matches.
func (s *AccessSuite) TestSignerIndicesDecoding() {

	container := s.net.ContainerByName(testnet.PrimaryAN)

	ctx, cancel := context.WithCancel(s.ctx)
	defer cancel()

	// create access API
	grpcAddress := container.Addr(testnet.GRPCPort)
	conn, err := grpc.DialContext(ctx, grpcAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(s.T(), err, "failed to connect to access node")
	defer conn.Close()

	client := accessproto.NewAccessAPIClient(conn)

	// query latest finalized block
	latestFinalizedBlock, err := makeApiRequest(client.GetLatestBlockHeader, ctx, &accessproto.GetLatestBlockHeaderRequest{
		IsSealed: false,
	})
	require.NoError(s.T(), err)

	blockByID, err := makeApiRequest(client.GetBlockHeaderByID, ctx, &accessproto.GetBlockHeaderByIDRequest{Id: latestFinalizedBlock.Block.Id})
	require.NoError(s.T(), err)

	require.Equal(s.T(), latestFinalizedBlock, blockByID, "expect to receive same block by ID")

	blockByHeight, err := makeApiRequest(client.GetBlockHeaderByHeight, ctx,
		&accessproto.GetBlockHeaderByHeightRequest{Height: latestFinalizedBlock.Block.Height})
	require.NoError(s.T(), err)

	require.Equal(s.T(), blockByID, blockByHeight, "expect to receive same block by height")

	// stop container, so we can access it's state and perform assertions
	err = s.net.StopContainerByName(ctx, testnet.PrimaryAN)
	require.NoError(s.T(), err)

	err = container.WaitForContainerStopped(5 * time.Second)
	require.NoError(s.T(), err)

	// open state to build a block singer decoder
	state, err := container.OpenState()
	require.NoError(s.T(), err)

	// create committee so we can create decoder to assert validity of data
	committee, err := committees.NewConsensusCommittee(state, container.Config.NodeID)
	require.NoError(s.T(), err)
	blockSignerDecoder := signature.NewBlockSignerDecoder(committee)

	expectedFinalizedBlock, err := state.AtBlockID(flow.HashToID(latestFinalizedBlock.Block.Id)).Head()
	require.NoError(s.T(), err)

	// since all blocks should be equal we will execute just check on one of them
	require.Equal(s.T(), latestFinalizedBlock.Block.ParentVoterIndices, expectedFinalizedBlock.ParentVoterIndices)

	// check if the response contains valid encoded signer IDs.
	msg := latestFinalizedBlock.Block
	block, err := convert.MessageToBlockHeader(msg)
	require.NoError(s.T(), err)
	decodedIdentities, err := blockSignerDecoder.DecodeSignerIDs(block)
	require.NoError(s.T(), err)
	// transform to assert
	var transformed [][]byte
	for _, identity := range decodedIdentities {
		identity := identity
		transformed = append(transformed, identity[:])
	}
	assert.ElementsMatch(s.T(), transformed, msg.ParentVoterIds, "response must contain correctly encoded signer IDs")
}

// TestRestSubscribeEvents tests event streaming on REST
func (s *AccessSuite) TestRestSubscribeEvents() {
	time.Sleep(5 * time.Second)
	t := s.T()

	ctx, cancel := context.WithTimeout(s.ctx, 30*time.Second)
	defer cancel()

	accessAddr := s.net.ContainerByName(testnet.PrimaryAN).Addr(testnet.RESTPort)

	t.Run("subscribe events", func(t *testing.T) {
		startBlockId := flow.ZeroID
		startHeight := uint64(0)
		url := getSubscribeEventsRequest(accessAddr, startBlockId, startHeight, nil, nil, nil)

		s.log.Info().Msg("================> resp.Request.URL.String()" + url)
		client, err := s.getWSClient(ctx, url)
		require.NoError(t, err)
		var receivedEvents []*state_stream.EventsResponse
		eventChan := make(chan *state_stream.EventsResponse)

		go func() {
			for {
				resp := &state_stream.EventsResponse{}
				err := client.ReadJSON(resp)
				if err != nil {
					if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
						t.Logf("unexpected close error: %v", err)
					}
					t.Log(fmt.Sprintf("______ReadJSON error %v", err))
					close(eventChan) // Close the event channel when the client connection is closed
					return
				}
				t.Log(fmt.Sprintf("______response %v", resp))
				eventChan <- resp
			}
		}()

		// Wait for events or timeout
		select {
		case <-time.After(10 * time.Second):
			// Handle the timeout, close the client connection, and break the loop
			t.Log("______receivedEvents")
			t.Log(receivedEvents)

			client.Close()
			t.Log("Client closed")
			return
		case event := <-eventChan:
			receivedEvents = append(receivedEvents, event)
			t.Log(fmt.Sprintf("______received events %v", event))
		}

	})
}

func (s *AccessSuite) getWSClient(ctx context.Context, address string) (*websocket.Conn, error) {
	// helper func to create WebSocket client
	client, _, err := websocket.DefaultDialer.DialContext(ctx, strings.Replace(address, "http", "ws", 1), nil)
	if err != nil {
		return nil, err
	}
	return client, nil
}

//
//// Assert that the received events match the expected events
//assert.Equal(s.T(), len(expectedEvents), len(receivedEvents))
//for i, expected := range expectedEvents {
//	received := receivedEvents[i]
//	s.T().Logf("expected" + expected.String())
//	s.T().Logf("received: BlockID" + received.BlockID.String() + ", height: " + fmt.Sprint(received.Height))
//	//assert.Equal(s.T(), expected.Height, received.Height)
//	//assert.Equal(s.T(), expected.BlockID, received.BlockID)
//	//assert.Equal(s.T(), len(expected.Events), len(received.Events))
//	//for j, expectedEvent := range expected.Events {
//	//	receivedEvent := received.Events[j]
//	//	// Perform further assertions on each event if needed
//	//	assert.Equal(s.T(), expectedEvent.Type, receivedEvent.Type)
//	//	assert.Equal(s.T(), expectedEvent.Data, receivedEvent.Data)
//	//}
//}

//resp, err := makeSubscribeEventsCall(accessAddr, startBlockId, startHeight, nil, nil, nil)
//assert.NoError(t, err)
//assert.Contains(t, [...]int{
//	http.StatusOK,
//}, resp.StatusCode)
//s.log.Info().Msg(fmt.Sprintf("================> %s %d", resp.Status, resp.StatusCode))

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

//func makeSubscribeEventsCall(accessAddr string, startBlockId flow.Identifier, startHeight uint64, eventTypes []string, addresses []string, contracts []string) (*http.Response, error) {
//	httpClient := http.DefaultClient
//	url := getSubscribeEventsRequest(accessAddr, startBlockId, startHeight, eventTypes, addresses, contracts)
//	return httpClient.Get(url)
//}

// makeApiRequest is a helper function that encapsulates context creation for grpc client call, used to avoid repeated creation
// of new context for each call.
func makeApiRequest[Func func(context.Context, *Req, ...grpc.CallOption) (*Resp, error), Req any, Resp any](apiCall Func, ctx context.Context, req *Req) (*Resp, error) {
	clientCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
	resp, err := apiCall(clientCtx, req)
	cancel()
	return resp, err
}
